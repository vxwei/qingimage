"use strict";
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg && msg.type === "qing-copy") {
    (async () => {
      let ok = false;
      try { await navigator.clipboard.writeText(msg.text); ok = true; } catch (e) {}
      if (!ok) {
        try {
          const ta = document.getElementById("ta");
          ta.value = msg.text;
          ta.focus();
          ta.select();
          ok = document.execCommand("copy");
        } catch (e) {}
      }
      sendResponse({ ok });
    })();
    return true;
  }
  return false;
});
