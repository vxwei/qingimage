"use strict";
importScripts("lib.js");

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "qing-upload-image",
    title: "上传图片到图床",
    contexts: ["image"],
  });
});

function notify(title, message) {
  chrome.notifications.create({
    type: "basic",
    iconUrl: "icons/icon128.png",
    title,
    message,
  });
}

async function copyText(text) {
  return qCopyText(text);
}

chrome.contextMenus.onClicked.addListener(async (info) => {
  if (info.menuItemId !== "qing-upload-image" || !info.srcUrl) return;
  const cfg = await qGetConfig();
  if (!cfg.token) {
    notify("轻图床助手", "还没有设置 token，先打开扩展设置填入 PicGo token");
    chrome.runtime.openOptionsPage();
    return;
  }
  try {
    const resp = await fetch(info.srcUrl);
    if (!resp.ok) throw new Error("取图失败 HTTP " + resp.status);
    const blob = await resp.blob();
    if (!blob.type.startsWith("image/")) throw new Error("这不是图片");
    const text = await qUploadBlob(blob);
    if (await copyText(text)) notify("上传成功，链接已复制 ✓", text);
    else notify("上传成功，但复制失败（链接在下面）", text);
  } catch (e) {
    notify("上传失败", e.message);
  }
});
