function generateTokenSecret() {
  const arr = new Uint8Array(24);
  crypto.getRandomValues(arr);
  return Array.from(arr, (b) => b.toString(16).padStart(2, "0")).join("");
}

function copyText(text) {
  if (!text) return;
  navigator.clipboard.writeText(text).then(() => {
    showToast("已复制");
  }).catch(() => {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.left = "-9999px";
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    document.body.removeChild(ta);
    showToast("已复制");
  });
}

function showToast(msg) {
  let el = document.getElementById("copy-toast");
  if (!el) {
    el = document.createElement("div");
    el.id = "copy-toast";
    el.className = "copy-toast";
    document.body.appendChild(el);
  }
  el.textContent = msg;
  el.classList.add("show");
  clearTimeout(el._timer);
  el._timer = setTimeout(() => el.classList.remove("show"), 1500);
}

document.addEventListener("click", (e) => {
  const randomBtn = e.target.closest("[data-random-secret]");
  if (randomBtn) {
    const input = document.getElementById(randomBtn.dataset.randomSecret);
    if (input) input.value = generateTokenSecret();
    return;
  }
  const copyBtn = e.target.closest("[data-copy]");
  if (copyBtn) {
    const fromId = copyBtn.dataset.copyFrom;
    const text = fromId
      ? (document.getElementById(fromId) || {}).value
      : copyBtn.dataset.copy;
    copyText(text || "");
  }
});
