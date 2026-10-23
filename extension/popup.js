"use strict";
const $ = s => document.querySelector(s);
let format = "url";

function setStatus(msg, ok) {
  const el = $("#status");
  el.textContent = msg || "";
  el.className = ok === true ? "ok" : ok === false ? "err" : "";
}

function addResult(text) {
  const d = document.createElement("div");
  d.className = "result";
  d.textContent = text;
  d.title = "点击复制";
  d.onclick = () => navigator.clipboard.writeText(text).then(() => setStatus("已复制 ✓", true));
  $("#results").prepend(d);
}

async function handleFiles(files) {
  const imgs = [...files].filter(f => f.type.startsWith("image/") || /\.(png|jpe?g|gif|webp|bmp|avif)$/i.test(f.name || ""));
  if (!imgs.length) { setStatus("没有可上传的图片", false); return; }
  const done = [];
  for (const f of imgs) {
    setStatus("正在上传 " + (f.name || "粘贴的图片") + " …");
    try {
      const text = await qUploadBlob(f, f.name && f.name !== "image.png" ? f.name : undefined);
      done.push(text);
      addResult(text);
    } catch (e) {
      setStatus(e.message, false);
    }
  }
  if (done.length) {
    await navigator.clipboard.writeText(done.join("\n")).catch(() => {});
    setStatus(done.length > 1 ? `已上传 ${done.length} 张，链接已全部复制 ✓` : "已上传，链接已复制 ✓", true);
  }
}

const drop = $("#drop"), fileInput = $("#file");
drop.onclick = () => fileInput.click();
fileInput.onchange = () => { handleFiles(fileInput.files); fileInput.value = ""; };
["dragenter", "dragover"].forEach(ev => drop.addEventListener(ev, e => { e.preventDefault(); drop.classList.add("drag"); }));
["dragleave", "drop"].forEach(ev => drop.addEventListener(ev, e => { e.preventDefault(); drop.classList.remove("drag"); }));
drop.addEventListener("drop", e => handleFiles(e.dataTransfer.files));
document.addEventListener("paste", e => { if (e.clipboardData && e.clipboardData.files.length) handleFiles(e.clipboardData.files); });

$("#fmtSeg").querySelectorAll("button").forEach(b => {
  b.onclick = async () => {
    format = b.dataset.fmt;
    $("#fmtSeg").querySelectorAll("button").forEach(x => x.classList.toggle("on", x === b));
    await chrome.storage.local.set({ format });
  };
});
$("#openOptions").onclick = () => chrome.runtime.openOptionsPage();
$("#openSite").onclick = async () => { const c = await qGetConfig(); chrome.tabs.create({ url: c.server }); };

(async () => {
  const c = await qGetConfig();
  format = c.format;
  $("#fmtSeg").querySelectorAll("button").forEach(x => x.classList.toggle("on", x.dataset.fmt === format));
  if (!c.token) setStatus("还没有设置 token：点右上角「设置」填入图床后台的 PicGo token", false);
})();
