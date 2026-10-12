package kb

// searchHTML is the independent retrieval page at GET /search.
// It lists bound CONTEXT entities and the indexed data under each one.
const searchHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>knowledge-base search</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 14px/1.4 -apple-system, Segoe UI, Roboto, sans-serif; margin: 0; padding: 20px; background: #f6f7f9; color: #1c1e21; }
  h1 { font-size: 18px; margin: 0 0 4px; }
  h2 { font-size: 14px; margin: 18px 0 8px; color: #344054; }
  .sub { color: #667085; margin-bottom: 14px; }
  .nav { display: flex; gap: 14px; margin: 0 0 14px; }
  .nav a { color: #1c4ed8; text-decoration: none; font-size: 13px; font-weight: 600; }
  .nav a.on { color: #1c1e21; }
  .msg { margin: 10px 0; padding: 8px 10px; border-radius: 6px; display: none; white-space: pre-wrap; }
  .msg.ok { display: block; background: #e6f4ea; color: #1a7f37; }
  .msg.err { display: block; background: #fde8e8; color: #b42318; }
  .hint { color: #667085; font-size: 12px; }
  .layout { display: flex; gap: 16px; align-items: flex-start; }
  aside { flex: 0 0 280px; background: #fff; box-shadow: 0 1px 2px rgba(16,24,40,.06); }
  main { flex: 1; min-width: 0; }
  .formtitle { font-size: 13px; font-weight: 600; color: #344054; padding: 12px 12px 6px; }
  .ctx { padding: 8px 12px; border-top: 1px solid #eaecf0; cursor: pointer; }
  .ctx:hover, .ctx.on { background: #eef2ff; }
  .ctx .meta { color: #667085; font-size: 12px; }
  .panel { background: #fff; padding: 14px; margin-bottom: 16px; box-shadow: 0 1px 2px rgba(16,24,40,.06); }
  .row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
  .row input[type=text] { flex: 1; min-width: 160px; padding: 5px 7px; border: 1px solid #d0d5dd; border-radius: 6px; }
  button { font: inherit; padding: 3px 9px; border: 1px solid #d0d5dd; background: #fff; border-radius: 6px; cursor: pointer; }
  button.primary { background: #1c4ed8; border-color: #1c4ed8; color: #fff; }
  .pill { display: inline-block; padding: 1px 8px; border-radius: 999px; font-size: 12px; font-weight: 600; background: #eef2ff; color: #3538cd; margin-right: 4px; }
  code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
  .result { padding: 8px 10px; border-bottom: 1px solid #eaecf0; cursor: pointer; }
  .result:hover { background: #f9fafb; }
  .result .score { float: right; color: #667085; font-size: 12px; }
  .tree-k { margin: 12px 0 4px; font-weight: 600; }
  .tree-f { margin: 6px 0 2px 8px; }
  .tree-s { margin-left: 22px; padding: 2px 0; cursor: pointer; color: #1c4ed8; }
  .tree-s:hover { text-decoration: underline; }
  pre.content { white-space: pre-wrap; background: #f9fafb; padding: 10px; border-radius: 6px; margin: 8px 0 0; font-size: 12px; }
  @media (max-width: 720px) { .layout { flex-direction: column; } aside { flex: auto; width: 100%; } }
</style>
</head>
<body>
<nav class="nav">
  <a href="/">Maintain</a>
  <a href="/search" class="on">Search</a>
</nav>
<h1>Context search</h1>
<div class="sub">
  Bound CONTEXT entities and the documents indexed under them.
  Pick one to browse headings or search. Empty list means nothing is bound yet.
</div>
<div id="msg" class="msg"></div>
<div class="layout">
  <aside>
    <div class="formtitle">Bound contexts</div>
    <div id="ctx-list"><div class="hint" style="padding:8px 12px">loading…</div></div>
  </aside>
  <main>
    <div id="ctx-empty" class="panel hint">
      Select a context on the left. If none are listed, register a source and bind it on Maintain.
    </div>
    <div id="ctx-main" hidden>
      <div class="panel">
        <div class="formtitle" id="ctx-title">context</div>
        <div class="hint" id="ctx-meta"></div>
        <form id="search" autocomplete="off" style="margin-top:10px">
          <div class="row">
            <input type="text" id="f-query" placeholder="search terms in this context">
            <button type="submit" class="primary">Search</button>
          </div>
        </form>
        <div id="results"></div>
        <div id="section"></div>
      </div>
      <div class="panel">
        <div class="formtitle">Indexed data</div>
        <div id="tree" class="hint">loading…</div>
      </div>
    </div>
  </main>
</div>
<script>
const $ = (id) => document.getElementById(id);

function say(text, kind) {
  const box = $("msg");
  box.className = "msg " + (kind || "ok");
  box.textContent = text;
}

async function api(method, path, body) {
  const res = await fetch(path, {
    method: method,
    headers: body ? { "content-type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let parsed = null;
  try { parsed = text ? JSON.parse(text) : null; } catch (e) { parsed = { raw: text }; }
  if (!res.ok) {
    throw new Error(res.status + " " + (parsed && parsed.code ? parsed.code + ": " : "") +
      ((parsed && (parsed.message || parsed.error || parsed.raw)) || res.statusText));
  }
  return parsed;
}

function esc(value) {
  const map = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" };
  return String(value == null ? "" : value).replace(/[&<>"]/g, function (c) { return map[c]; });
}

function parseHash() {
  const h = (location.hash || "").replace(/^#/, "");
  const i = h.indexOf("/");
  if (i < 1) return null;
  return { type: decodeURIComponent(h.slice(0, i)), id: decodeURIComponent(h.slice(i + 1)) };
}

let contexts = [];
let current = null;

function labelOf(c) {
  if (c.name) return c.name + " · " + c.type + "/" + c.id;
  return c.type + "/" + c.id;
}

function renderList() {
  const box = $("ctx-list");
  if (!contexts.length) {
    box.innerHTML = '<div class="hint" style="padding:8px 12px">No bound contexts yet.</div>';
    return;
  }
  box.innerHTML = contexts.map(function (c, i) {
    const on = current && current.type === c.type && current.id === c.id ? " on" : "";
    return '<div class="ctx' + on + '" data-i="' + i + '">' +
      '<div>' + esc(labelOf(c)) + "</div>" +
      '<div class="meta">' + c.knowledge + " sources · " + c.documents + " docs · " + c.sections + " sections</div>" +
      "</div>";
  }).join("");
}

async function selectContext(c) {
  current = c;
  location.hash = encodeURIComponent(c.type) + "/" + encodeURIComponent(c.id);
  renderList();
  $("ctx-empty").hidden = true;
  $("ctx-main").hidden = false;
  $("ctx-title").textContent = labelOf(c);
  $("ctx-meta").textContent = c.knowledge + " sources, " + c.documents + " documents, " + c.sections + " sections";
  $("results").innerHTML = "";
  $("section").innerHTML = "";
  $("tree").textContent = "loading…";
  try {
    const out = await api("GET", "/api/contexts/" + encodeURIComponent(c.type) + "/" +
      encodeURIComponent(c.id) + "/browse");
    const items = (out && out.knowledge) || [];
    if (!items.length) {
      $("tree").innerHTML = '<div class="hint">This context has no indexed files yet. Sync a bound source on Maintain.</div>';
      return;
    }
    $("tree").innerHTML = items.map(function (k) {
      const files = (k.files || []).map(function (f) {
        const secs = (f.sections || []).map(function (s) {
          return '<div class="tree-s" data-section="' + esc(s.id) + '">' + esc(s.heading || "(untitled)") + "</div>";
        }).join("");
        return '<div class="tree-f"><code>' + esc(f.path || f.name) + "</code> · " +
          (f.sections || []).length + " sections" + secs + "</div>";
      }).join("");
      return '<div class="tree-k">' + esc(k.name || k.id) +
        ' <span class="pill">' + esc(k.kind) + "/" + esc(k.provider) + "</span> " +
        '<span class="hint"><code>' + esc(k.revision || "") + "</code></span></div>" + files;
    }).join("");
  } catch (e) {
    $("tree").innerHTML = "";
    say(e.message, "err");
  }
}

async function loadContexts() {
  try {
    const out = await api("GET", "/api/contexts");
    contexts = (out && out.contexts) || [];
    renderList();
    const want = parseHash();
    let pick = null;
    if (want) {
      pick = contexts.filter(function (c) { return c.type === want.type && c.id === want.id; })[0];
    }
    if (!pick && contexts.length) pick = contexts[0];
    if (pick) await selectContext(pick);
  } catch (e) {
    $("ctx-list").innerHTML = '<div class="hint" style="padding:8px 12px">could not load contexts</div>';
    say(e.message, "err");
  }
}

$("ctx-list").addEventListener("click", function (event) {
  const row = event.target.closest(".ctx");
  if (!row) return;
  const c = contexts[Number(row.dataset.i)];
  if (c) selectContext(c);
});

$("search").addEventListener("submit", async function (event) {
  event.preventDefault();
  $("section").innerHTML = "";
  if (!current) { say("select a context first", "err"); return; }
  const q = $("f-query").value.trim();
  if (!q) { say("type something to search for", "err"); return; }
  try {
    const out = await api("GET", "/api/contexts/" + encodeURIComponent(current.type) + "/" +
      encodeURIComponent(current.id) + "/search?q=" + encodeURIComponent(q));
    const results = (out && out.results) || [];
    $("results").innerHTML = results.length ? results.map(function (r) {
      return '<div class="result" data-section="' + esc(r.section_id) + '">' +
        '<span class="score">' + esc(Number(r.score).toFixed(3)) + "</span>" +
        "<strong>" + esc(r.heading || "(untitled)") + "</strong> " +
        '<span class="hint">' + esc(r.document_name || r.knowledge_id) + "</span>" +
        '<div class="hint">' + esc(r.snippet) + "</div>" +
      "</div>";
    }).join("") : '<div class="hint" style="padding:8px 0">no matching sections</div>';
  } catch (e) {
    $("results").innerHTML = "";
    say(e.message, "err");
  }
});

async function openSection(id) {
  try {
    const s = await api("GET", "/api/sections/" + encodeURIComponent(id));
    $("section").innerHTML = '<div style="margin-top:10px">' +
      "<strong>" + esc(s.heading || "(untitled)") + "</strong> " +
      '<span class="hint">' + esc(s.path || "") +
      (s.revision ? " · rev <code>" + esc(s.revision) + "</code>" : "") + "</span>" +
      '<pre class="content">' + esc(s.content) + "</pre></div>";
  } catch (e) {
    say(e.message, "err");
  }
}

$("results").addEventListener("click", function (event) {
  const hit = event.target.closest("[data-section]");
  if (hit) openSection(hit.dataset.section);
});
$("tree").addEventListener("click", function (event) {
  const hit = event.target.closest("[data-section]");
  if (hit) openSection(hit.dataset.section);
});

window.addEventListener("hashchange", function () {
  const want = parseHash();
  if (!want) return;
  const pick = contexts.filter(function (c) { return c.type === want.type && c.id === want.id; })[0];
  if (pick && (!current || current.type !== pick.type || current.id !== pick.id)) selectContext(pick);
});

loadContexts();
</script>
</body>
</html>
`
