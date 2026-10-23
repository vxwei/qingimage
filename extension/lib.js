"use strict";
// Shared config + upload logic for the QingImage browser extension.
// Loaded by popup, options and the background service worker (importScripts).

const Q_DEFAULT_SERVER = "https://pic.vcdn.win";

async function qGetConfig() {
  const d = await chrome.storage.local.get(["server", "token", "format"]);
  return {
    server: (d.server || Q_DEFAULT_SERVER).replace(/\/+$/, ""),
    token: d.token || "",
    format: d.format || "url",
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
async function qUploadBlob(blob, filename) {
  const cfg = await qGetConfig();
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
