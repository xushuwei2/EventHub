function buildTrackEvent() {
  const evt = {
    eventId: uuid(),
    occurredAt: isoNow(-1),
    release: val("release"),
    env: val("env"),
    eventName: val("track-event-name"),
  };
  const route = val("route");
  if (route) evt.route = route;
  const funnelKey = val("track-funnel-key");
  const stepKey = val("track-step-key");
  if (funnelKey && stepKey) {
    evt.funnelKey = funnelKey;
    evt.stepKey = stepKey;
  }
  return evt;
}

async function sendTrack() {
  const token = await ensureToken();
  if (!token) return;

  const payload = {
    clientSentAt: isoNow(),
    events: [buildTrackEvent()],
  };
  $("request-preview").textContent = JSON.stringify(payload, null, 2);

  try {
    const res = await fetch("/reporting/v1/track/batch", {
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
  $("btn-send-track")?.addEventListener("click", () => sendTrack());
});
