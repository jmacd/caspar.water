const SCHEMA_VERSION = 4;
const STALE_AFTER_MS = 6 * 60 * 60 * 1000;
const STATES = new Set(["healthy", "alarm", "unknown"]);
const TONES = new Set(["good", "watch", "concern", "alarm"]);

function append(parent, tag, text, className) {
  const child = document.createElement(tag);
  if (text !== undefined) {
    child.textContent = text;
  }
  if (className) {
    child.className = className;
  }
  parent.append(child);
  return child;
}

function validateStatus(status) {
  if (!status || status.schema_version !== SCHEMA_VERSION) {
    throw new Error(`unsupported monitor schema (expected ${SCHEMA_VERSION})`);
  }
  if (
    typeof status.pond !== "string" ||
    typeof status.title !== "string" ||
    typeof status.generated_at !== "string" ||
    !Number.isInteger(status.transaction_sequence) ||
    !STATES.has(status.state) ||
    !Array.isArray(status.checks)
  ) {
    throw new Error("invalid monitor status");
  }
  for (const check of status.checks) {
    if (
      !check ||
      typeof check.label !== "string" ||
      typeof check.description !== "string" ||
      typeof check.href !== "string" ||
      !check.href.startsWith("/") ||
      check.href.startsWith("//") ||
      !STATES.has(check.state) ||
      (check.tone !== undefined && !TONES.has(check.tone))
    ) {
      throw new Error("invalid monitor check");
    }
  }
  const generatedAt = Date.parse(status.generated_at);
  if (!Number.isFinite(generatedAt)) {
    throw new Error("invalid monitor generation time");
  }
  return generatedAt;
}

async function fetchStatus(url) {
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  const status = await response.json();
  const generatedAt = validateStatus(status);
  return { status, generatedAt };
}

function stateCard(title, state, headingLevel = "h2", tag = "article") {
  const card = append(
    document.createDocumentFragment(),
    tag,
    undefined,
    `monitor-card monitor-state-${state}`,
  );
  const header = append(card, "header");
  append(header, headingLevel, title);
  append(header, "strong", state, "monitor-state");
  return card;
}

function formatTime(timestamp) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(timestamp);
}

function renderFailure(title, error) {
  const card = stateCard(title, "unavailable");
  append(card, "p", error.message, "monitor-error");
  return card;
}

function renderCheck(check, stale) {
  const state = stale ? "stale" : (check.tone ?? check.state);
  const card = stateCard(check.label, state, "h2", "a");
  card.href = check.href;
  append(card, "p", check.description);
  return card;
}

async function renderMonitor(container) {
  const url = container.dataset.statusUrl;
  const fallbackTitle = container.dataset.title || "Monitoring";
  try {
    const { status, generatedAt } = await fetchStatus(url);
    const stale = Date.now() - generatedAt > STALE_AFTER_MS;
    const grid = append(container, "div", undefined, "monitor-grid");
    for (const check of status.checks) {
      grid.append(renderCheck(check, stale));
    }
    append(
      container,
      "p",
      `Status generated ${formatTime(generatedAt)} from transaction ${status.transaction_sequence}.`,
      "monitor-meta",
    );
  } catch (error) {
    container.append(renderFailure(fallbackTitle, error));
  }
  container.querySelector("[data-loading]")?.remove();
}

for (const container of document.querySelectorAll("[data-monitor]")) {
  renderMonitor(container);
}
