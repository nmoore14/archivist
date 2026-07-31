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
  sourceReturnFocus?.focus();
}

function openSourceDrawer(citation) {
  if (!sourceDrawer) return;
  sourceReturnFocus = citation;
  const page = Number(citation.dataset.sourcePage || 0);
  const url = citation.dataset.sourceUrl;
  sourceTitle.textContent = citation.dataset.sourceName || "Course source";
  sourceLocation.textContent = page ? `Cited location · page ${page}` : "Original document";
  sourceOpenNew.href = url;
  sourceFrame.src = url;
  document.body.classList.add("source-drawer-open");
  sourceDrawer.setAttribute("aria-hidden", "false");
  sourceDrawer.querySelector(".drawer-close")?.focus();
}

async function copyMessage(button) {
  const text = button.closest(".message")?.querySelector(".message-content")?.innerText.trim();
  if (!text) return;
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
    button.textContent = "Copy";
  }, 1800);
}

document.addEventListener("click", (event) => {
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

document.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && event.shiftKey && event.target?.matches?.('.composer textarea[name="question"]')) {
    event.preventDefault();
    const composer = event.target.closest(".composer");
    if (!composer.classList.contains("htmx-request") && event.target.value.trim()) {
      composer.requestSubmit();
    }
    return;
  }
  if (event.key === "Escape" && document.body.classList.contains("source-drawer-open")) {
    closeSourceDrawer();
  }
});
