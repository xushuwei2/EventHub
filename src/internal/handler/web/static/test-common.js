const IDENTITY_KEYS = [
  "project-id", "release", "env", "user-id", "session-id", "room-id",
  "language", "runtime", "device-platform", "report-token",
];

function uuid() {
  if (crypto.randomUUID) return crypto.randomUUID();
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

function isoNow(offsetSec = 0) {
  const d = new Date(Date.now() + offsetSec * 1000);
  return d.toISOString();
}

function $(id) {
  return document.getElementById(id);
}

function val(id) {
  const el = $(id);
  return el ? el.value.trim() : "";
}

function setVal(id, v) {
  const el = $(id);
  if (el) el.value = v ?? "";
}

function saveIdentity() {
  const data = {};
  for (const id of IDENTITY_KEYS) {
    const el = $(id);
    if (el) data[id] = el.value;
  }
  try {
    localStorage.setItem("eventhub_test_identity", JSON.stringify(data));
  } catch (_) { /* ignore */ }
}

function restoreIdentity() {
  try {
    const raw = localStorage.getItem("eventhub_test_identity");
    if (!raw) return;
    const data = JSON.parse(raw);
    for (const id of IDENTITY_KEYS) {
      if (data[id] != null) setVal(id, data[id]);
    }
    if (data["report-token"]) {
      $("token-result")?.classList.remove("hidden");
    }
  } catch (_) { /* ignore */ }
}

function bindIdentityPersistence() {
  for (const id of IDENTITY_KEYS) {
    const el = $(id);
    if (el) el.addEventListener("change", saveIdentity);
    if (el && el.tagName === "INPUT") el.addEventListener("input", saveIdentity);
  }
}

async function signToken() {
  const projectId = parseInt(val("project-id"), 10);
  if (!projectId) {
    showResponse({ error: "请先创建项目" }, true);
    return null;
  }
  const opt = $("project-id").selectedOptions[0];
  if (opt && opt.dataset.status === "disabled") {
    showResponse({ error: "所选项目已停用" }, true);
    return null;
  }

  const body = {
    projectId,
    userId: val("user-id"),
    sessionId: val("session-id"),
    roomId: val("room-id"),
    release: val("release"),
  };

  const res = await fetch("/reporting/admin/test/sign-token", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    showResponse(data, true);
    return null;
  }
  $("report-token").value = data.reportToken;
  $("token-result").classList.remove("hidden");
  saveIdentity();
  return data.reportToken;
}

function showResponse(data, isError) {
  const el = $("response-preview");
  if (!el) return;
  const accepted = data.accepted;
  const rejected = data.rejected;
  const feedbackId = data.feedbackId;
  let summary = "";
  if (feedbackId) {
    summary = `成功：feedbackId=${feedbackId}\n\n`;
    isError = false;
  } else if (accepted != null) {
    const ok = accepted > 0 && (!rejected || rejected === 0);
    summary = ok
      ? `成功：accepted=${accepted}\n\n`
      : `失败：accepted=${accepted}, rejected=${rejected}\n（事件未入库，请重启服务后重试或查看服务端日志）\n\n`;
    isError = isError || !ok;
  }
  el.textContent = summary + JSON.stringify(data, null, 2);
  el.classList.toggle("response-error", !!isError);
  el.classList.remove("response-idle");
}

async function ensureToken() {
  let token = val("report-token");
  if (!token) {
    token = await signToken();
  }
  return token;
}

document.addEventListener("DOMContentLoaded", () => {
  restoreIdentity();
  bindIdentityPersistence();
  const btnSign = $("btn-sign-token");
  if (btnSign) btnSign.addEventListener("click", () => signToken());
});
