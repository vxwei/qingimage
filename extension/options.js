"use strict";
const $ = s => document.querySelector(s);

(async () => {
  const c = await qGetConfig();
  $("#server").value = c.server;
  $("#token").value = c.token;
  $("#format").value = c.format;
})();

$("#save").onclick = async () => {
  const server = $("#server").value.trim().replace(/\/+$/, "") || Q_DEFAULT_SERVER;
  const token = $("#token").value.trim();
  const format = $("#format").value;
  // Ask for access to a custom server origin while still in the click gesture.
  if (!server.startsWith(Q_DEFAULT_SERVER)) {
    try { await chrome.permissions.request({ origins: [server + "/*"] }); } catch (e) { /* user may decline */ }
  }
  await chrome.storage.local.set({ server, token, format });
  $("#msg").textContent = "已保存 ✓";
  setTimeout(() => { $("#msg").textContent = ""; }, 2000);
};
