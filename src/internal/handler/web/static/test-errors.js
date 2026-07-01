const CATEGORY_FIELDS = {
  api_failure: `
    <label>apiMethod</label>
    <input type="text" id="api-method" value="POST">
    <label>apiPath</label>
    <input type="text" id="api-path" value="/api/orders">
    <label>httpStatus</label>
    <input type="number" id="http-status" value="500">
  `,
  ws_failure: `
    <label>wsPhase</label>
    <select id="ws-phase">
      <option value="connect" selected>connect</option>
      <option value="handshake">handshake</option>
      <option value="message">message</option>
      <option value="close">close</option>
    </select>
    <label>wsCode / wsReason <span class="label-hint">（可空）</span></label>
    <div class="inline-fields">
      <input type="number" id="ws-code" placeholder="1006">
      <input type="text" id="ws-reason" placeholder="connection refused">
    </div>
  `,
  asset_failure: `
    <label>assetType</label>
    <select id="asset-type">
      <option value="image" selected>image</option>
      <option value="audio">audio</option>
      <option value="svg">svg</option>
      <option value="manifest">manifest</option>
    </select>
    <label>assetPath / assetUrl <span class="label-hint">（可空）</span></label>
    <div class="inline-fields">
      <input type="text" id="asset-path" placeholder="assets/logo.png">
      <input type="text" id="asset-url" placeholder="https://cdn.example.com/logo.png">
    </div>
  `,
  biz_error: `
    <label>bizCode</label>
    <input type="text" id="biz-code" value="room_result_retry_exhausted">
  `,
};

const PRESETS = {
  uncaught_js: {
    category: "uncaught_js",
    severity: "error",
    message: "Uncaught TypeError: Cannot read property 'foo' of undefined",
    route: "home",
    scene: "boot",
    module: "app/main",
    stack: "TypeError: Cannot read property 'foo' of undefined\n    at init (main.js:42:10)\n    at boot (app.js:8:3)",
  },
  unhandled_promise: {
    category: "unhandled_promise",
    severity: "error",
    message: "Unhandled Promise rejection: network timeout",
    route: "login",
    scene: "auth",
    module: "platform/api/auth",
    stack: "Error: network timeout\n    at fetchWithTimeout (http.js:88:11)",
  },
  api_failure: {
    category: "api_failure",
    severity: "error",
    message: "提交订单失败：HTTP 500",
    route: "checkout",
    scene: "checkout",
    module: "platform/api/order",
    apiMethod: "POST",
    apiPath: "/api/orders",
    httpStatus: 500,
  },
  ws_failure: {
    category: "ws_failure",
    severity: "error",
    message: "WebSocket 建连失败",
    route: "game",
    scene: "room",
    module: "platform/ws",
    wsPhase: "connect",
    wsCode: 1006,
    wsReason: "connection refused",
  },
  asset_failure: {
    category: "asset_failure",
    severity: "warn",
    message: "图片资源加载失败",
    route: "home",
    scene: "splash",
    module: "assets/loader",
    assetType: "image",
    assetPath: "assets/splash.png",
    assetUrl: "https://cdn.example.com/assets/splash.png",
  },
  biz_error: {
    category: "biz_error",
    severity: "error",
    message: "房间结算重试次数耗尽",
    route: "room",
    scene: "result",
    module: "game/room",
    bizCode: "room_result_retry_exhausted",
  },
};

function renderCategoryFields() {
  const cat = val("category");
  const container = $("category-fields");
  if (!container) return;
  container.innerHTML = CATEGORY_FIELDS[cat] || "";
}

function applyPreset(name) {
  const p = PRESETS[name];
  if (!p) return;
  setVal("category", p.category);
  setVal("severity", p.severity);
  setVal("message", p.message);
  setVal("route", p.route || "");
  setVal("scene", p.scene || "");
  setVal("module", p.module || "");
  setVal("stack", p.stack || "");
  renderCategoryFields();
  if (p.apiMethod) setVal("api-method", p.apiMethod);
  if (p.apiPath) setVal("api-path", p.apiPath);
  if (p.httpStatus != null) setVal("http-status", String(p.httpStatus));
  if (p.wsPhase) setVal("ws-phase", p.wsPhase);
  if (p.wsCode != null) setVal("ws-code", String(p.wsCode));
  if (p.wsReason) setVal("ws-reason", p.wsReason);
  if (p.assetType) setVal("asset-type", p.assetType);
  if (p.assetPath) setVal("asset-path", p.assetPath);
  if (p.assetUrl) setVal("asset-url", p.assetUrl);
  if (p.bizCode) setVal("biz-code", p.bizCode);
}

function buildEvent() {
  const cat = val("category");
  const evt = {
    eventId: uuid(),
    occurredAt: isoNow(-2),
    release: val("release"),
    env: val("env"),
    category: cat,
    severity: val("severity"),
    message: val("message"),
  };

  const route = val("route");
  const scene = val("scene");
  const mod = val("module");
  const stack = val("stack");
  const lang = val("language");
  const runtime = val("runtime");
  const platform = val("device-platform");
  const userId = val("user-id");
  const sessionId = val("session-id");
  const roomId = val("room-id");

  if (route) evt.route = route;
  if (scene) evt.scene = scene;
  if (mod) evt.module = mod;
  if (stack) evt.stack = stack;
  if (lang) evt.language = lang;
  if (runtime) evt.runtime = runtime;
  if (platform) evt.devicePlatform = platform;
  if (userId) evt.userId = userId;
  if (sessionId) evt.sessionId = sessionId;
  if (roomId) evt.roomId = roomId;

  if (cat === "api_failure") {
    evt.apiMethod = val("api-method");
    evt.apiPath = val("api-path");
    const status = parseInt(val("http-status"), 10);
    if (!Number.isNaN(status) && status > 0) evt.httpStatus = status;
  } else if (cat === "ws_failure") {
    evt.wsPhase = val("ws-phase");
    const code = parseInt(val("ws-code"), 10);
    if (!Number.isNaN(code)) evt.wsCode = code;
    const reason = val("ws-reason");
    if (reason) evt.wsReason = reason;
  } else if (cat === "asset_failure") {
    evt.assetType = val("asset-type");
    const path = val("asset-path");
    const url = val("asset-url");
    if (path) evt.assetPath = path;
    if (url) evt.assetUrl = url;
  } else if (cat === "biz_error") {
    evt.bizCode = val("biz-code");
  }

  return evt;
}

function buildBatch(count) {
  const events = [];
  for (let i = 0; i < count; i++) {
    events.push(buildEvent());
  }
  return {
    clientSentAt: isoNow(),
    events,
  };
}

async function sendReport(count) {
  const token = await ensureToken();
  if (!token) return;

  const payload = buildBatch(count);
  $("request-preview").textContent = JSON.stringify(payload, null, 2);

  try {
    const res = await fetch("/reporting/v1/events/batch", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + token,
      },
      body: JSON.stringify(payload),
    });
    const data = await res.json();
    showResponse({ status: res.status, ...data }, !res.ok);
  } catch (err) {
    showResponse({ error: String(err) }, true);
  }
}

document.addEventListener("DOMContentLoaded", () => {
  renderCategoryFields();
  applyPreset("uncaught_js");

  $("category")?.addEventListener("change", renderCategoryFields);

  document.querySelectorAll("[data-preset]").forEach((btn) => {
    btn.addEventListener("click", () => applyPreset(btn.dataset.preset));
  });

  $("btn-send")?.addEventListener("click", () => sendReport(1));
  $("btn-send-batch")?.addEventListener("click", () => sendReport(3));
});
