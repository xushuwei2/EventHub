function buildFeedback() {
  const payload = {
    feedbackId: uuid(),
    submittedAt: isoNow(),
    release: val("release"),
    env: val("env"),
    category: val("feedback-category"),
    content: val("feedback-content"),
  };

  const contact = val("feedback-contact");
  const route = val("feedback-route");
  const scene = val("feedback-scene");
  const lang = val("language");
  const runtime = val("runtime");
  const platform = val("device-platform");
  const screenshot = val("feedback-screenshot");

  if (contact) payload.contact = contact;
  if (route) payload.route = route;
  if (scene) payload.scene = scene;
  if (lang) payload.language = lang;
  if (runtime) payload.runtime = runtime;
  if (platform) payload.devicePlatform = platform;
  if (screenshot) payload.extra = { screenshotUrl: screenshot };

  return payload;
}

async function sendFeedback() {
  const content = val("feedback-content");
  if (!content) {
    showResponse({ error: "请填写反馈内容" }, true);
    return;
  }

  const token = await ensureToken();
  if (!token) return;

  const payload = buildFeedback();
  $("request-preview").textContent = JSON.stringify(payload, null, 2);

  try {
    const res = await fetch("/reporting/v1/feedback", {
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
  $("btn-send-feedback")?.addEventListener("click", () => sendFeedback());
});
