"use strict";
// Shared config + upload logic for the QingImage browser extension.
// Loaded by popup, options and the background service worker (importScripts).

const Q_DEFAULT_SERVER = "";

async function qGetConfig() {
  const d = await chrome.storage.local.get(["server", "token", "format"]);
  return {
    server: (d.server || Q_DEFAULT_SERVER).replace(/\/+$/, ""),
    token: d.token || "",
    format: d.format || "markdown",
  };
}

function qFormatLink(links, format) {
  if (format === "markdown") return links.markdown || links.url;
  if (format === "html") return links.html || links.url;
  return links.url;
}

function qExtForType(type) {
  if (type === "image/jpeg") return ".jpg";
  if (type === "image/gif") return ".gif";
  if (type === "image/webp") return ".webp";
  if (type === "image/bmp") return ".bmp";
  if (type === "image/avif") return ".avif";
  return ".png";
}

// Upload one Blob/File. Resolves with the formatted link text.
// Copy text to the clipboard, honestly: resolves true only when a copy
// actually succeeded. Primary route is the offscreen document (the
// reliable MV3 channel); local fallbacks run in page contexts.
async function qCopyText(text) {
  try {
    await chrome.offscreen.createDocument({
      url: "offscreen.html",
      reasons: ["CLIPBOARD"],
      justification: "把上传后的图片链接复制到剪贴板",
    });
  } catch (e) { /* already open */ }
  try {
    const r = await chrome.runtime.sendMessage({ type: "qing-copy", text });
    if (r && r.ok) return true;
  } catch (e) { /* no receiver */ }
  if (typeof document !== "undefined") {
    try { await navigator.clipboard.writeText(text); return true; } catch (e) {}
    try {
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed"; ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      const ok = document.execCommand("copy");
      ta.remove();
      if (ok) return true;
    } catch (e) {}
  }
  return false;
}

async function qUploadBlob(blob, filename) {
  const cfg = await qGetConfig();
  if (!cfg.server) throw new Error("还没有设置图床地址：请先打开扩展的「设置」填入你自己的图床网址和 token");
  if (!cfg.token) throw new Error("还没有设置 token：请先打开扩展的「设置」填入图床后台的 PicGo token");
  const name = filename || ("image-" + Date.now() + qExtForType(blob.type));
  const fd = new FormData();
  fd.append("file", blob, name);
  let resp;
  try {
    resp = await fetch(cfg.server + "/api/v1/upload", {
      method: "POST",
      headers: { Authorization: "Bearer " + cfg.token },
      body: fd,
    });
  } catch (e) {
    throw new Error("连不上图床服务器：" + e.message);
  }
  const j = await resp.json().catch(() => null);
  if (!resp.ok || !j || j.status !== true || !j.data || !j.data.links) {
    throw new Error("上传失败：" + ((j && j.message) || ("HTTP " + resp.status)));
  }
  return qFormatLink(j.data.links, cfg.format);
}
