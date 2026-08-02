const mathOptions = {
  delimiters: [
    { left: "$$", right: "$$", display: true },
    { left: "\\[", right: "\\]", display: true },
    { left: "\\(", right: "\\)", display: false },
    { left: "$", right: "$", display: false },
  ],
  throwOnError: false,
  strict: "warn",
};

function renderMessageMath(root = document) {
  if (typeof window.renderMathInElement !== "function") return;
  root.querySelectorAll(".message.assistant .message-content:not([data-math-rendered])").forEach((message) => {
    window.renderMathInElement(message, mathOptions);
    message.dataset.mathRendered = "true";
  });
}

function scrollToNewestMessage(behavior = "auto") {
  const messages = document.querySelector("#messages");
  if (!messages || !messages.querySelector(".message")) return;
  messages.scrollTo({ top: messages.scrollHeight, behavior });
}

document.addEventListener("DOMContentLoaded", () => {
  renderMessageMath();
  updateSelectedFiles();
  scheduleDocumentPoll();
  requestAnimationFrame(() => requestAnimationFrame(() => scrollToNewestMessage()));
});

window.addEventListener("load", () => scrollToNewestMessage(), { once: true });

function appendOptimisticMessage(composer) {
  const question = composer.querySelector('textarea[name="question"]')?.value.trim();
  const messages = document.querySelector("#messages");
  if (!question || !messages || messages.querySelector("[data-optimistic]")) return;

  messages.querySelector(".chat-empty")?.remove();
  const message = document.createElement("article");
  message.className = "message user message-enter";
  message.dataset.optimistic = "true";

  const role = document.createElement("div");
  role.className = "message-role";
  role.textContent = "You";

  const content = document.createElement("div");
  const paragraph = document.createElement("p");
  paragraph.textContent = question;
  content.append(paragraph);
  message.append(role, content);
  messages.append(message);
  scrollToNewestMessage("smooth");
}

document.addEventListener("htmx:beforeRequest", (event) => {
  const composer = event.detail?.elt?.closest(".composer");
  if (!composer) return;

  appendOptimisticMessage(composer);
  const question = composer.querySelector('textarea[name="question"]');
  const pendingQuestion = question?.value || "";
  composer.dataset.pendingQuestion = pendingQuestion;
  // Let HTMX finish serializing the form before changing the successful
  // control that supplies the question. This still clears before the browser
  // paints the next frame, so the composer feels immediate.
  setTimeout(() => {
    if (question?.value === pendingQuestion) question.value = "";
  }, 0);
  composer.closest(".chat-shell")?.querySelector(".chat-body")?.classList.add("is-thinking");
});

document.addEventListener("htmx:afterRequest", (event) => {
  if (event.detail?.elt?.querySelector?.(".batch-dropzone")) {
    requestAnimationFrame(updateSelectedFiles);
  }
  const composer = event.detail?.elt?.closest(".composer");
  if (!composer) return;

  composer.closest(".chat-shell")?.querySelector(".chat-body")?.classList.remove("is-thinking");
  if (!event.detail?.successful) {
    document.querySelector("[data-optimistic]")?.remove();
    const question = composer.querySelector('textarea[name="question"]');
    if (question) question.value = composer.dataset.pendingQuestion || "";
  }
  if (event.detail?.successful) composer.reset();
  delete composer.dataset.pendingQuestion;
  renderMessageMath(document);
});

document.addEventListener("htmx:afterSwap", (event) => {
  if (event.detail?.target?.id === "document-list") {
    scheduleDocumentPoll();
  }
  if (event.detail?.target?.id !== "messages") return;

  const messages = document.querySelector("#messages");
  renderMessageMath(messages);
  [...messages.querySelectorAll(".message")].slice(-2).forEach((message) => {
    message.classList.add("message-enter");
  });
  scrollToNewestMessage("smooth");
  document.querySelector(".composer textarea")?.focus();
});

let documentPollTimer = null;

function scheduleDocumentPoll() {
  clearTimeout(documentPollTimer);
  const list = document.querySelector("#document-list");
  if (!list?.querySelector("[data-index-pending]")) return;
  documentPollTimer = setTimeout(refreshDocumentProgress, 1200);
}

async function refreshDocumentProgress() {
  const list = document.querySelector("#document-list");
  const url = list?.dataset.indexStatusUrl;
  if (!list || !url) return;
  try {
    const response = await fetch(url, { headers: { "HX-Request": "true" } });
    if (!response.ok) throw new Error(`Status ${response.status}`);
    list.outerHTML = await response.text();
  } catch {
    // A brief network interruption should not stop progress updates.
  }
  scheduleDocumentPoll();
}

function updateSelectedFiles() {
  const input = document.querySelector('.batch-dropzone input[type="file"]');
  const output = document.querySelector(".batch-dropzone .file-selection");
  if (!input || !output) return;
  const count = input.files?.length || 0;
  output.textContent = count ? `${count} ${count === 1 ? "file" : "files"} selected` : "";
}

document.addEventListener("change", (event) => {
  if (event.target.matches?.('.batch-dropzone input[type="file"]')) updateSelectedFiles();
});

const sourceDrawer = document.querySelector("#source-drawer");
const sourceFrame = document.querySelector("#source-frame");
const sourceTitle = document.querySelector("#source-drawer-title");
const sourceLocation = document.querySelector("#source-drawer-location");
const sourceOpenNew = document.querySelector("#source-open-new");
let sourceReturnFocus = null;

function closeSourceDrawer() {
  if (!sourceDrawer) return;
  document.body.classList.remove("source-drawer-open");
  sourceDrawer.setAttribute("aria-hidden", "true");
  sourceFrame.removeAttribute("src");
  sourceFrame.setAttribute("sandbox", "");
  sourceReturnFocus?.focus();
}

function openSourceDrawer(citation) {
  if (!sourceDrawer) return;
  sourceReturnFocus = citation;
  const page = Number(citation.dataset.sourcePage || 0);
  const url = citation.dataset.sourceUrl;
  const sourceName = citation.dataset.sourceName || "Course source";
  sourceTitle.textContent = sourceName;
  sourceLocation.textContent = page ? `Cited location · page ${page}` : "Original document";
  sourceOpenNew.href = url;
  // Browser PDF viewers cannot initialize inside a fully sandboxed iframe.
  // Other source types remain sandboxed because HTML uploads are untrusted.
  if (/\.pdf$/i.test(sourceName)) {
    sourceFrame.removeAttribute("sandbox");
  } else {
    sourceFrame.setAttribute("sandbox", "");
  }
  sourceFrame.src = url;
  document.body.classList.add("source-drawer-open");
  sourceDrawer.setAttribute("aria-hidden", "false");
  sourceDrawer.querySelector(".drawer-close")?.focus();
}

async function copyMessage(button) {
  const text = button.closest(".message")?.querySelector(".message-content")?.innerText.trim();
  if (!text) return;
  await copyText(text, button);
}

async function copyText(text, button) {
  const originalHTML = button.innerHTML;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const helper = document.createElement("textarea");
      helper.value = text;
      helper.setAttribute("readonly", "");
      helper.style.position = "fixed";
      helper.style.opacity = "0";
      document.body.append(helper);
      helper.select();
      document.execCommand("copy");
      helper.remove();
    }
    button.textContent = "Copied";
  } catch {
    button.textContent = "Copy failed";
  }
  setTimeout(() => {
    button.innerHTML = originalHTML;
  }, 1800);
}

function focusComposer() {
  const question = document.querySelector('.composer textarea[name="question"]');
  if (!question) return false;
  question.focus();
  question.scrollIntoView({ block: "nearest", behavior: "smooth" });
  return true;
}

async function copyLatestAnswer(button) {
  const answers = document.querySelectorAll(".message.assistant .message-content");
  const latest = answers[answers.length - 1]?.innerText.trim();
  if (!latest) {
    button.textContent = "No answer yet";
    setTimeout(() => { button.innerHTML = '<span>Copy latest answer</span><kbd>Alt C</kbd>'; }, 1800);
    return;
  }
  await copyText(latest, button);
}

const globalSearchModal = document.querySelector(".global-search-modal");
const globalSearchInput = globalSearchModal?.querySelector("[data-global-search]");
const globalSearchResults = globalSearchModal?.querySelector("[data-global-search-results]");
let globalSearchReturnFocus = null;
const userSettingsMenu = document.querySelector("[data-user-menu]");
const userMenuButton = document.querySelector("[data-user-menu-open]");
const systemTheme = window.matchMedia("(prefers-color-scheme: dark)");

function savedThemePreference() {
  try {
    return localStorage.getItem("archivist-theme") || "system";
  } catch {
    return "system";
  }
}

function applyTheme(preference, persist = true) {
  if (!["light", "dark", "system"].includes(preference)) preference = "system";
  const resolved = preference === "system" ? (systemTheme.matches ? "dark" : "light") : preference;
  document.documentElement.dataset.theme = resolved;
  document.documentElement.dataset.themePreference = preference;
  document.querySelectorAll("[data-theme-choice]").forEach((choice) => {
    choice.checked = choice.value === preference;
  });
  if (persist) {
    try { localStorage.setItem("archivist-theme", preference); } catch { /* Storage may be disabled. */ }
  }
}

function openUserSettings() {
  if (!userSettingsMenu) return;
  applyTheme(savedThemePreference(), false);
  userSettingsMenu.setAttribute("aria-hidden", "false");
  userMenuButton?.setAttribute("aria-expanded", "true");
  userSettingsMenu.querySelector("input:checked")?.focus();
}

function closeUserSettings(returnFocus = true) {
  if (!userSettingsMenu) return;
  userSettingsMenu.setAttribute("aria-hidden", "true");
  userMenuButton?.setAttribute("aria-expanded", "false");
  if (returnFocus) userMenuButton?.focus();
}

systemTheme.addEventListener?.("change", () => {
  if (savedThemePreference() === "system") applyTheme("system", false);
});

function openGlobalSearch(trigger) {
  if (!globalSearchModal) return false;
  globalSearchReturnFocus = trigger || document.activeElement;
  document.body.classList.add("global-search-open");
  globalSearchModal.setAttribute("aria-hidden", "false");
  requestAnimationFrame(() => {
    globalSearchInput?.focus();
    globalSearchInput?.select();
  });
  return true;
}

function closeGlobalSearch() {
  if (!globalSearchModal) return;
  document.body.classList.remove("global-search-open");
  globalSearchModal.setAttribute("aria-hidden", "true");
  globalSearchReturnFocus?.focus?.();
}

async function submitGlobalSearch(form) {
  const query = new FormData(form).get("q")?.toString().trim();
  if (!query || !globalSearchResults) return;
  globalSearchResults.innerHTML = '<div class="search-loading" role="status">Searching your private index…</div>';
  try {
    const response = await fetch(`/search?partial=1&q=${encodeURIComponent(query)}`, {
      headers: { "X-Archivist-Partial": "search" },
    });
    if (!response.ok) throw new Error(`Status ${response.status}`);
    globalSearchResults.innerHTML = await response.text();
  } catch {
    globalSearchResults.innerHTML = '<div class="alert error">Search could not be completed. Please try again.</div>';
  }
}

document.addEventListener("click", (event) => {
  if (event.target.closest("[data-user-menu-open]")) {
    if (userSettingsMenu?.getAttribute("aria-hidden") === "false") closeUserSettings();
    else openUserSettings();
    return;
  }
  if (event.target.closest("[data-user-menu-close]")) {
    closeUserSettings();
    return;
  }
  if (userSettingsMenu?.getAttribute("aria-hidden") === "false" && !event.target.closest("[data-user-menu]")) {
    closeUserSettings(false);
  }
  const globalSearchButton = event.target.closest("[data-global-search-open]");
  if (globalSearchButton && openGlobalSearch(globalSearchButton)) {
    event.preventDefault();
    return;
  }
  if (event.target.closest("[data-global-search-close]")) {
    closeGlobalSearch();
    return;
  }
  const focusButton = event.target.closest("[data-focus-composer]");
  if (focusButton) {
    if (focusComposer()) event.preventDefault();
    return;
  }
  const latestButton = event.target.closest("[data-copy-latest]");
  if (latestButton) {
    copyLatestAnswer(latestButton);
    return;
  }
  const copyButton = event.target.closest("[data-copy-message]");
  if (copyButton) {
    copyMessage(copyButton);
    return;
  }
  const citation = event.target.closest(".source-citation");
  if (citation) {
    openSourceDrawer(citation);
    return;
  }
  if (event.target.closest("[data-source-close]")) closeSourceDrawer();
});

document.addEventListener("submit", (event) => {
  const form = event.target.closest?.("[data-global-search-form]");
  if (!form) return;
  event.preventDefault();
  submitGlobalSearch(form);
});

document.addEventListener("change", (event) => {
  const choice = event.target.closest?.("[data-theme-choice]");
  if (choice) applyTheme(choice.value);
});

document.addEventListener("keydown", (event) => {
  const isEditing = event.target?.matches?.("input, textarea, select, [contenteditable='true']");
  if ((event.metaKey || event.ctrlKey) && event.code === "KeyK") {
    event.preventDefault();
    openGlobalSearch();
    return;
  }
  if ((event.key === "/" || event.code === "Slash") && !isEditing && !event.altKey && !event.ctrlKey && !event.metaKey) {
    event.preventDefault();
    if (!focusComposer()) {
      const chat = document.querySelector('[data-course-shortcut="1"]');
      if (chat) window.location.assign(chat.href);
    }
    return;
  }
  const courseShortcut = event.altKey ? event.code?.match(/^Digit([1-5])$/)?.[1] : null;
  if (courseShortcut) {
    const destination = document.querySelector(`[data-course-shortcut="${courseShortcut}"]`);
    if (destination) {
      event.preventDefault();
      window.location.assign(destination.href);
    }
    return;
  }
  if (event.altKey && event.code === "KeyC" && !isEditing) {
    const latestButton = document.querySelector("[data-copy-latest]");
    if (latestButton) {
      event.preventDefault();
      copyLatestAnswer(latestButton);
    }
    return;
  }
  if (event.key === "Enter" && (event.metaKey || event.ctrlKey) && event.target?.matches?.('.composer textarea[name="question"]')) {
    event.preventDefault();
    const composer = event.target.closest(".composer");
    if (!composer.classList.contains("htmx-request") && event.target.value.trim()) {
      composer.requestSubmit();
    }
    return;
  }
  if (event.key === "Escape") {
    if (userSettingsMenu?.getAttribute("aria-hidden") === "false") closeUserSettings();
    else if (document.body.classList.contains("global-search-open")) closeGlobalSearch();
    else if (document.body.classList.contains("source-drawer-open")) closeSourceDrawer();
  }
});
