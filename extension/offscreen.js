"use strict";
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg && msg.type === "qing-copy") {
    const ta = document.getElementById("ta");
    ta.value = msg.text;
    ta.select();
    const ok = document.execCommand("copy");
    sendResponse({ ok });
  }
  return false;
});
