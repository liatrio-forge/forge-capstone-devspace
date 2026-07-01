const fs = require("fs");
const path = require("path");
const { marked } = require("marked");

const root = process.cwd();
const docs = [
  "README.md",
  "docs/release-readiness.md",
  "docs/capstone/README.md",
  "docs/capstone/spec.md",
  "docs/capstone/proof-artifacts.md",
  "docs/capstone/case-study.md",
  "docs/capstone/demo-script.md",
  "docs/capstone/playbook-contribution.md",
  "ops/wave-ship/README.md",
];

function read(rel) {
  return fs.readFileSync(path.join(root, rel), "utf8");
}

function escapeHtml(value) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function attr(value) {
  return escapeHtml(String(value));
}

function slug(value) {
  return String(value)
    .toLowerCase()
    .replace(/`([^`]+)`/g, "$1")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80);
}

const renderer = new marked.Renderer();
let codeCounter = 0;
let headingCounter = 0;

renderer.heading = ({ tokens, depth }) => {
  const text = marked.Parser.parseInline(tokens);
  const plain = tokens.map((token) => token.raw || token.text || "").join("");
  const id = `${slug(plain) || "section"}-${++headingCounter}`;
  return `<h${depth} id="${attr(id)}"><a class="heading-link" href="#${attr(id)}" aria-label="Link to ${attr(plain)}">#</a>${text}</h${depth}>`;
};

renderer.code = ({ text, lang }) => {
  const id = `code-${++codeCounter}`;
  const language = (lang || "text").split(/\s+/)[0] || "text";
  const encoded = Buffer.from(text, "utf8").toString("base64");
  return `<figure class="code-block" data-lang="${attr(language)}">
    <figcaption>
      <span>${attr(language)}</span>
      <button class="copy-btn" type="button" data-copy-code="${attr(id)}">Copy</button>
    </figcaption>
    <div id="${attr(id)}" class="diff-render" data-code-b64="${encoded}" data-lang="${attr(language)}"></div>
    <noscript><pre><code>${escapeHtml(text)}</code></pre></noscript>
  </figure>`;
};

marked.setOptions({
  gfm: true,
  breaks: false,
  renderer,
});

const sections = docs.map((rel, index) => {
  const markdown = read(rel);
  const html = marked.parse(markdown);
  const titleMatch = markdown.match(/^#\s+(.+)$/m);
  const title = titleMatch ? titleMatch[1].replace(/`/g, "") : rel;
  const words = markdown.trim().split(/\s+/).filter(Boolean).length;
  const codeBlocks = (markdown.match(/```/g) || []).length / 2;
  const tables = (markdown.match(/\n\|.+\|\n\|[\s:-]+\|/g) || []).length;
  return {
    index: index + 1,
    rel,
    id: `doc-${index + 1}`,
    title,
    words,
    codeBlocks,
    tables,
    html,
    markdown,
  };
});

const totalWords = sections.reduce((sum, doc) => sum + doc.words, 0);
const totalCode = sections.reduce((sum, doc) => sum + doc.codeBlocks, 0);
const totalTables = sections.reduce((sum, doc) => sum + doc.tables, 0);

function jsonScript(value) {
  return JSON.stringify(value).replace(/</g, "\\u003c");
}

const html = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>DevDrop Markdown Reader</title>
  <meta name="description" content="Interactive HTML reader generated from every Markdown file in the DevDrop repository.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
  <style>
    :root {
      color-scheme: light;
      --ink: #17201a;
      --muted: #5d6b62;
      --line: #d9e1da;
      --line-strong: #b8c6bd;
      --paper: #f8faf7;
      --panel: #ffffff;
      --soft: #eef4f0;
      --mint: #0f766e;
      --mint-dark: #0b4f4a;
      --amber: #b7791f;
      --red: #b42318;
      --green: #16803b;
      --blue: #1d4f8f;
      --mono: "JetBrains Mono", "SFMono-Regular", Consolas, monospace;
      --sans: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      --radius: 8px;
      --shadow: 0 1px 2px rgba(15, 23, 42, 0.08), 0 12px 32px rgba(15, 23, 42, 0.06);
      --focus: 0 0 0 3px rgba(15, 118, 110, 0.22);
    }

    @media (prefers-color-scheme: dark) {
      :root {
        color-scheme: dark;
        --ink: #eff7f1;
        --muted: #b7c7bd;
        --line: #2a3a31;
        --line-strong: #496257;
        --paper: #101612;
        --panel: #161f19;
        --soft: #1d2a22;
        --mint: #5ee0d0;
        --mint-dark: #8df1e4;
        --amber: #f4bf62;
        --red: #ff8a7d;
        --green: #67d391;
        --blue: #8fb8ff;
        --shadow: 0 1px 2px rgba(0, 0, 0, 0.35), 0 12px 32px rgba(0, 0, 0, 0.28);
      }
    }

    * { box-sizing: border-box; }
    html { scroll-behavior: smooth; }
    body {
      margin: 0;
      min-width: 320px;
      background: var(--paper);
      color: var(--ink);
      font-family: var(--sans);
      line-height: 1.55;
      letter-spacing: 0;
    }
    button, input { font: inherit; }
    button, a { touch-action: manipulation; }
    button:focus-visible, a:focus-visible, input:focus-visible {
      outline: none;
      box-shadow: var(--focus);
    }
    .app {
      display: grid;
      grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
      min-height: 100vh;
    }
    .sidebar {
      position: sticky;
      top: 0;
      align-self: start;
      height: 100vh;
      overflow: auto;
      border-right: 1px solid var(--line);
      background: color-mix(in srgb, var(--panel) 92%, var(--soft));
      padding: 20px;
    }
    .brand {
      display: grid;
      gap: 10px;
      padding-bottom: 18px;
      border-bottom: 1px solid var(--line);
    }
    .brand-row {
      display: flex;
      gap: 12px;
      align-items: center;
      min-width: 0;
    }
    .brand-mark {
      display: inline-grid;
      place-items: center;
      flex: 0 0 auto;
      width: 40px;
      height: 40px;
      border: 1px solid var(--line-strong);
      border-radius: 8px;
      background: var(--ink);
      color: var(--paper);
      font-weight: 800;
    }
    .brand h1 {
      margin: 0;
      font-size: 1.05rem;
      line-height: 1.1;
    }
    .brand p {
      margin: 0;
      color: var(--muted);
      font-size: 0.9rem;
    }
    .stats {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 8px;
      margin-top: 14px;
    }
    .stat {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
      padding: 10px;
      min-width: 0;
    }
    .stat strong {
      display: block;
      font-size: 1.05rem;
      line-height: 1;
    }
    .stat span {
      display: block;
      margin-top: 4px;
      color: var(--muted);
      font-size: 0.74rem;
    }
    .controls {
      display: grid;
      gap: 10px;
      margin: 18px 0;
    }
    .search {
      width: 100%;
      min-height: 42px;
      border: 1px solid var(--line-strong);
      border-radius: 8px;
      background: var(--panel);
      color: var(--ink);
      padding: 10px 12px;
    }
    .segmented {
      display: grid;
      grid-template-columns: 1fr 1fr;
      border: 1px solid var(--line);
      border-radius: 8px;
      overflow: hidden;
      background: var(--panel);
    }
    .segmented button {
      border: 0;
      background: transparent;
      color: var(--muted);
      min-height: 38px;
      cursor: pointer;
    }
    .segmented button[aria-pressed="true"] {
      background: var(--ink);
      color: var(--paper);
      font-weight: 700;
    }
    .nav {
      display: grid;
      gap: 6px;
      margin-top: 18px;
    }
    .nav button {
      min-height: 48px;
      width: 100%;
      border: 1px solid transparent;
      border-radius: 8px;
      padding: 10px 12px;
      background: transparent;
      color: var(--muted);
      text-align: left;
      cursor: pointer;
      display: grid;
      grid-template-columns: 28px minmax(0, 1fr);
      gap: 9px;
      align-items: center;
    }
    .nav button[aria-selected="true"] {
      background: var(--soft);
      border-color: var(--line);
      color: var(--ink);
      font-weight: 700;
    }
    .nav .num {
      display: inline-grid;
      place-items: center;
      width: 26px;
      height: 26px;
      border-radius: 999px;
      background: var(--panel);
      border: 1px solid var(--line);
      font-family: var(--mono);
      font-size: 0.72rem;
      color: var(--muted);
    }
    .nav .label {
      display: grid;
      gap: 2px;
      min-width: 0;
    }
    .nav .title {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .nav .path {
      color: var(--muted);
      font-family: var(--mono);
      font-size: 0.72rem;
      overflow-wrap: anywhere;
    }
    .main {
      min-width: 0;
      padding: 28px;
    }
    .hero {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 20px;
      align-items: end;
      padding: 28px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
      box-shadow: var(--shadow);
      margin-bottom: 18px;
    }
    .hero > * {
      min-width: 0;
    }
    .hero h2 {
      margin: 0;
      max-width: 100%;
      font-size: clamp(1.55rem, 2.7vw, 2.8rem);
      line-height: 1.02;
      letter-spacing: 0;
      overflow-wrap: anywhere;
    }
    .hero p {
      margin: 10px 0 0;
      color: var(--muted);
      max-width: 72ch;
    }
    .badge-row {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      justify-content: flex-end;
      min-width: 0;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      min-height: 28px;
      border: 1px solid var(--line);
      border-radius: 999px;
      padding: 4px 10px;
      background: var(--soft);
      color: var(--muted);
      font-size: 0.82rem;
      overflow-wrap: anywhere;
    }
    .doc-grid {
      display: grid;
      gap: 18px;
    }
    .doc {
      display: none;
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
      box-shadow: var(--shadow);
      overflow: hidden;
    }
    .doc.active {
      display: block;
    }
    .doc-header {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 14px;
      padding: 22px 24px;
      border-bottom: 1px solid var(--line);
      background: color-mix(in srgb, var(--soft) 62%, transparent);
    }
    .doc-header h2 {
      margin: 0;
      font-size: 1.35rem;
      line-height: 1.15;
    }
    .doc-path {
      margin-top: 6px;
      color: var(--muted);
      font-family: var(--mono);
      font-size: 0.82rem;
      overflow-wrap: anywhere;
    }
    .doc-meta {
      display: flex;
      flex-wrap: wrap;
      align-content: start;
      justify-content: flex-end;
      gap: 8px;
      min-width: 0;
    }
    .doc-body {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
    }
    .rendered, .source {
      min-width: 0;
      padding: 24px;
    }
    .source {
      display: none;
      border-top: 1px solid var(--line);
      background: color-mix(in srgb, var(--soft) 45%, transparent);
    }
    body.show-source .source {
      display: block;
    }
    body.show-source .rendered {
      border-bottom: 1px solid var(--line);
    }
    .markdown {
      min-width: 0;
      max-width: 980px;
    }
    .markdown h1, .markdown h2, .markdown h3, .markdown h4 {
      position: relative;
      margin: 1.5em 0 0.45em;
      line-height: 1.18;
      letter-spacing: 0;
    }
    .markdown h1:first-child, .markdown h2:first-child, .markdown h3:first-child {
      margin-top: 0;
    }
    .markdown h1 { font-size: 2rem; }
    .markdown h2 { font-size: 1.45rem; }
    .markdown h3 { font-size: 1.12rem; }
    .markdown p, .markdown li {
      color: color-mix(in srgb, var(--ink) 88%, var(--muted));
    }
    .markdown a {
      color: var(--mint-dark);
      font-weight: 650;
      text-decoration-thickness: 1px;
      text-underline-offset: 3px;
    }
    .heading-link {
      position: absolute;
      left: -1.1em;
      color: var(--muted);
      opacity: 0;
      text-decoration: none;
      font-weight: 700;
    }
    .markdown h1:hover .heading-link,
    .markdown h2:hover .heading-link,
    .markdown h3:hover .heading-link,
    .markdown h4:hover .heading-link {
      opacity: 1;
    }
    .markdown blockquote {
      margin: 18px 0;
      padding: 12px 16px;
      border-left: 4px solid var(--mint);
      background: var(--soft);
      border-radius: 0 8px 8px 0;
    }
    .markdown table {
      width: 100%;
      border-collapse: collapse;
      margin: 18px 0;
      overflow-wrap: anywhere;
      font-size: 0.93rem;
    }
    .markdown th, .markdown td {
      border: 1px solid var(--line);
      padding: 10px 12px;
      vertical-align: top;
    }
    .markdown th {
      background: var(--soft);
      text-align: left;
    }
    .markdown :not(pre) > code {
      border: 1px solid var(--line);
      border-radius: 6px;
      background: var(--soft);
      padding: 0.08em 0.34em;
      font-family: var(--mono);
      font-size: 0.88em;
      overflow-wrap: anywhere;
    }
    .code-block {
      margin: 18px 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      overflow: hidden;
      background: #0d1117;
      min-width: 0;
    }
    .code-block figcaption {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      min-height: 40px;
      padding: 8px 10px;
      background: color-mix(in srgb, #0d1117 86%, var(--mint));
      color: #e6edf3;
      font-family: var(--mono);
      font-size: 0.78rem;
    }
    .copy-btn {
      border: 1px solid rgba(255,255,255,0.18);
      border-radius: 6px;
      background: rgba(255,255,255,0.08);
      color: #e6edf3;
      min-height: 28px;
      padding: 4px 9px;
      cursor: pointer;
    }
    .diff-render {
      min-width: 0;
    }
    .fallback-code {
      margin: 0;
      padding: 14px;
      color: #e6edf3;
      overflow-x: auto;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
      font-family: var(--mono);
      font-size: 0.86rem;
    }
    .source pre {
      margin: 0;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
      font-family: var(--mono);
      font-size: 0.86rem;
      color: var(--ink);
    }
    .empty-state {
      display: none;
      border: 1px dashed var(--line-strong);
      border-radius: 8px;
      background: var(--panel);
      padding: 24px;
      color: var(--muted);
    }
    .empty-state.visible {
      display: block;
    }
    .mobile-toggle {
      display: none;
      width: 100%;
      min-height: 42px;
      border: 1px solid var(--line-strong);
      border-radius: 8px;
      background: var(--panel);
      color: var(--ink);
      margin-bottom: 14px;
      cursor: pointer;
    }
    @media (max-width: 860px) {
      .app {
        display: block;
      }
      .mobile-toggle {
        display: block;
      }
      .sidebar {
        position: static;
        display: none;
        height: auto;
        max-height: none;
        border-right: 0;
        border-bottom: 1px solid var(--line);
      }
      body.nav-open .sidebar {
        display: block;
      }
      .main {
        padding: 16px;
      }
      .hero {
        grid-template-columns: minmax(0, 1fr);
        padding: 20px;
      }
      .badge-row {
        justify-content: flex-start;
      }
      .doc-header {
        grid-template-columns: minmax(0, 1fr);
      }
      .doc-meta {
        justify-content: flex-start;
      }
      .rendered, .source {
        padding: 18px;
      }
      .heading-link {
        position: static;
        opacity: 1;
        margin-right: 0.35em;
      }
    }
  </style>
</head>
<body>
  <div class="app">
    <aside class="sidebar" id="sidebar">
      <div class="brand">
        <div class="brand-row">
          <div class="brand-mark">DD</div>
          <div>
            <h1>DevDrop Markdown</h1>
            <p>Interactive HTML reader for every Markdown file in this repo.</p>
          </div>
        </div>
        <div class="stats" aria-label="Document stats">
          <div class="stat"><strong>${sections.length}</strong><span>files</span></div>
          <div class="stat"><strong>${totalWords.toLocaleString()}</strong><span>words</span></div>
          <div class="stat"><strong>${totalCode}</strong><span>snippets</span></div>
        </div>
      </div>

      <div class="controls">
        <input id="search" class="search" type="search" placeholder="Search docs..." autocomplete="off">
        <div class="segmented" aria-label="View mode">
          <button id="renderedBtn" type="button" aria-pressed="true">HTML</button>
          <button id="sourceBtn" type="button" aria-pressed="false">HTML + MD</button>
        </div>
      </div>

      <nav class="nav" id="nav" aria-label="Markdown files">
        ${sections.map((doc) => `<button type="button" data-doc="${attr(doc.id)}" aria-selected="${doc.index === 1 ? "true" : "false"}">
          <span class="num">${doc.index}</span>
          <span class="label"><span class="title">${escapeHtml(doc.title)}</span><span class="path">${escapeHtml(doc.rel)}</span></span>
        </button>`).join("\n")}
      </nav>
    </aside>

    <main class="main">
      <button class="mobile-toggle" type="button" id="mobileToggle">Toggle document menu</button>
      <section class="hero" aria-labelledby="page-title">
        <div>
          <h2 id="page-title">Markdown converted into a reviewable HTML surface.</h2>
          <p>Use the left navigation to switch files, search across converted content, and toggle raw Markdown under each rendered document when you need source-level review.</p>
        </div>
        <div class="badge-row">
          <span class="badge">${sections.length} Markdown files</span>
          <span class="badge">${totalTables} tables</span>
          <span class="badge">${totalCode} code blocks</span>
        </div>
      </section>

      <div id="emptyState" class="empty-state">No Markdown files match the current search.</div>
      <section class="doc-grid" id="docs">
        ${sections.map((doc) => `<article class="doc${doc.index === 1 ? " active" : ""}" id="${attr(doc.id)}" data-title="${attr(doc.title)}" data-path="${attr(doc.rel)}" data-search="${attr((doc.title + " " + doc.rel + " " + doc.markdown).toLowerCase())}">
          <header class="doc-header">
            <div>
              <h2>${escapeHtml(doc.title)}</h2>
              <div class="doc-path">${escapeHtml(doc.rel)}</div>
            </div>
            <div class="doc-meta">
              <span class="badge">${doc.words.toLocaleString()} words</span>
              <span class="badge">${doc.codeBlocks} code</span>
              <span class="badge">${doc.tables} tables</span>
            </div>
          </header>
          <div class="doc-body">
            <div class="rendered markdown">${doc.html}</div>
            <div class="source"><pre>${escapeHtml(doc.markdown)}</pre></div>
          </div>
        </article>`).join("\n")}
      </section>
    </main>
  </div>

  <script id="doc-data" type="application/json">${jsonScript(sections.map(({ id, title, rel }) => ({ id, title, rel })))}</script>
  <script type="module">
    import { File } from "https://esm.sh/@pierre/diffs@1.2.10?bundle";

    const docs = Array.from(document.querySelectorAll(".doc"));
    const navButtons = Array.from(document.querySelectorAll("#nav button"));
    const search = document.querySelector("#search");
    const emptyState = document.querySelector("#emptyState");
    const renderedBtn = document.querySelector("#renderedBtn");
    const sourceBtn = document.querySelector("#sourceBtn");
    const mobileToggle = document.querySelector("#mobileToggle");

    function decodeBase64(value) {
      const bytes = Uint8Array.from(atob(value), (char) => char.charCodeAt(0));
      return new TextDecoder().decode(bytes);
    }

    function renderCodeBlocks() {
      const theme = { light: "github-light", dark: "github-dark" };
      const prefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
      const options = { theme, themeType: prefersDark ? "dark" : "light", overflow: "wrap" };
      document.querySelectorAll(".diff-render").forEach((container, index) => {
        const code = decodeBase64(container.dataset.codeB64 || "");
        const lang = container.dataset.lang || "text";
        try {
          new File(options).render({
            containerWrapper: container,
            file: { name: \`snippet-\${index + 1}.\${lang}\`, contents: code.endsWith("\\n") ? code : code + "\\n" },
          });
        } catch (error) {
          container.innerHTML = "";
          const pre = document.createElement("pre");
          pre.className = "fallback-code";
          pre.textContent = code;
          container.appendChild(pre);
        }
      });
    }

    function setActive(id) {
      docs.forEach((doc) => doc.classList.toggle("active", doc.id === id));
      navButtons.forEach((button) => button.setAttribute("aria-selected", String(button.dataset.doc === id)));
      document.body.classList.remove("nav-open");
      history.replaceState(null, "", "#" + id);
      window.scrollTo({ top: 0, behavior: "smooth" });
    }

    function firstVisibleDoc() {
      return docs.find((doc) => !doc.hidden);
    }

    function filterDocs() {
      const query = search.value.trim().toLowerCase();
      let visible = 0;
      docs.forEach((doc) => {
        const match = query === "" || doc.dataset.search.includes(query);
        doc.hidden = !match;
        if (match) visible += 1;
      });
      navButtons.forEach((button) => {
        const doc = document.getElementById(button.dataset.doc);
        button.hidden = doc ? doc.hidden : false;
      });
      emptyState.classList.toggle("visible", visible === 0);
      const active = docs.find((doc) => doc.classList.contains("active"));
      if (visible > 0 && (!active || active.hidden)) {
        setActive(firstVisibleDoc().id);
      }
    }

    navButtons.forEach((button) => {
      button.addEventListener("click", () => setActive(button.dataset.doc));
    });

    search.addEventListener("input", filterDocs);
    renderedBtn.addEventListener("click", () => {
      document.body.classList.remove("show-source");
      renderedBtn.setAttribute("aria-pressed", "true");
      sourceBtn.setAttribute("aria-pressed", "false");
    });
    sourceBtn.addEventListener("click", () => {
      document.body.classList.add("show-source");
      renderedBtn.setAttribute("aria-pressed", "false");
      sourceBtn.setAttribute("aria-pressed", "true");
    });
    mobileToggle.addEventListener("click", () => {
      document.body.classList.toggle("nav-open");
    });

    document.addEventListener("click", async (event) => {
      const button = event.target.closest("[data-copy-code]");
      if (!button) return;
      const target = document.getElementById(button.dataset.copyCode);
      if (!target) return;
      const code = decodeBase64(target.dataset.codeB64 || "");
      await navigator.clipboard.writeText(code);
      const previous = button.textContent;
      button.textContent = "Copied";
      setTimeout(() => { button.textContent = previous; }, 1000);
    });

    const hashId = location.hash.slice(1);
    if (hashId && document.getElementById(hashId)?.classList.contains("doc")) {
      setActive(hashId);
    }
    renderCodeBlocks();
  </script>
</body>
</html>
`;

const outputs = [
  path.join(root, ".lavish", "devdrop-markdown.html"),
  path.join(root, "docs", "capstone", "index.html"),
];

for (const output of outputs) {
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, html);
  console.log(`wrote ${path.relative(root, output)}`);
}
