package kb

// dashboardHTML is the operator UI at GET / and GET /dashboard.
// Self-contained: no bundler, no external asset. It calls this service's
// own relative APIs. Binding is always a CONTEXT entity {type, id}.
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>knowledge-base</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 14px/1.4 -apple-system, Segoe UI, Roboto, sans-serif; margin: 0; padding: 20px; background: #f6f7f9; color: #1c1e21; }
  h1 { font-size: 18px; margin: 0 0 4px; }
  h2 { font-size: 14px; margin: 18px 0 8px; color: #344054; }
  .sub { color: #667085; margin-bottom: 14px; }
  table { border-collapse: collapse; width: 100%; background: #fff; box-shadow: 0 1px 2px rgba(16,24,40,.06); }
  th, td { text-align: left; padding: 8px 10px; border-bottom: 1px solid #eaecf0; vertical-align: middle; }
  th { background: #f9fafb; font-size: 12px; text-transform: uppercase; letter-spacing: .03em; color: #475467; }
  tr.group td { background: #f2f4f7; font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: .03em; color: #344054; }
  code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
  .pill { display: inline-block; padding: 1px 8px; border-radius: 999px; font-size: 12px; font-weight: 600; background: #eef2ff; color: #3538cd; margin: 1px 2px 1px 0; }
  button { font: inherit; padding: 3px 9px; border: 1px solid #d0d5dd; background: #fff; border-radius: 6px; cursor: pointer; }
  button:hover { background: #f9fafb; }
  button.primary { background: #1c4ed8; border-color: #1c4ed8; color: #fff; }
  button.danger { color: #b42318; border-color: #fda29b; }
  button:disabled { opacity: .5; cursor: not-allowed; }
  form, .panel { background: #fff; padding: 14px; margin-bottom: 16px; box-shadow: 0 1px 2px rgba(16,24,40,.06); }
  .grid > div { padding: 9px 0; border-bottom: 1px dashed #eaecf0; }
  .grid > div:last-child { border-bottom: none; }
  .formtitle { font-size: 13px; font-weight: 600; color: #344054; margin-bottom: 2px; }
  .actions { border-top: 1px solid #eaecf0; padding-top: 12px; margin-top: 4px; }
  label { display: block; font-size: 12px; color: #475467; margin-bottom: 3px; }
  input[type=text], select { width: 100%; padding: 5px 7px; border: 1px solid #d0d5dd; border-radius: 6px; box-sizing: border-box; }
  .row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
  .row input[type=text], .row select { flex: 1; width: auto; min-width: 140px; }
  .check { display: flex; gap: 6px; align-items: center; font-size: 13px; color: #344054; }
  .shape { display: flex; gap: 10px; flex-wrap: wrap; }
  .shape label.check { padding: 8px 10px; border: 1px solid #d0d5dd; border-radius: 8px; background: #f9fafb; cursor: pointer; }
  .shape input { width: auto; margin: 0; }
  form.dir .only-file, form.file .only-dir { display: none; }
  form.no-provider .needs-provider { opacity: .5; pointer-events: none; }
  .custom-type { display: none; }
  .show-custom .custom-type { display: block; }
  .msg { margin: 10px 0; padding: 8px 10px; border-radius: 6px; display: none; white-space: pre-wrap; }
  .msg.ok { display: block; background: #e6f4ea; color: #1a7f37; }
  .msg.err { display: block; background: #fde8e8; color: #b42318; }
  .hint { color: #667085; font-size: 12px; }
  .result { padding: 8px 10px; border-bottom: 1px solid #eaecf0; cursor: pointer; }
  .result:hover { background: #f9fafb; }
  .result .score { float: right; color: #667085; font-size: 12px; }
  pre.content { white-space: pre-wrap; background: #f9fafb; padding: 10px; border-radius: 6px; margin: 8px 0 0; font-size: 12px; }
</style>
</head>
<body>
<h1>knowledge-base</h1>
<div class="sub">
  Register sources by kind and provider, bind them to a CONTEXT entity
  ({type, id}), sync the index, search sections and read one.
  project is one entity type among others.
</div>

<div id="msg" class="msg"></div>

<div class="panel" id="entity-bar">
  <div class="formtitle">CONTEXT entity</div>
  <div class="row">
    <label for="f-type" style="margin:0">type</label>
    <select id="f-type">
      <option value="project">project</option>
      <option value="organization">organization</option>
      <option value="service">service</option>
      <option value="environment">environment</option>
      <option value="task">task</option>
      <option value="_other">other</option>
    </select>
    <input type="text" id="f-type-custom" class="custom-type" placeholder="custom type">
    <label for="f-id" style="margin:0">id</label>
    <input type="text" id="f-id" placeholder="entity id">
    <button type="button" id="reload">Load</button>
  </div>
  <div class="shape" id="f-scope" style="margin-top:10px">
    <label class="check"><input type="radio" name="f-scope" value="all" checked> all sources</label>
    <label class="check"><input type="radio" name="f-scope" value="entity"> bound to this entity</label>
  </div>
</div>

<h2>Sources</h2>
<div class="shape" id="f-filter" style="margin-bottom:10px">
  <label class="check"><input type="radio" name="f-filter" value="" checked> all</label>
  <label class="check"><input type="radio" name="f-filter" value="filesystem"> filesystem</label>
  <label class="check"><input type="radio" name="f-filter" value="source_code_repository"> source code repository</label>
  <label class="check"><input type="radio" name="f-filter" value="cloud"> cloud storage</label>
</div>
<table>
  <thead><tr><th>name</th><th>location</th><th>bindings</th><th>revision</th><th></th></tr></thead>
  <tbody id="rows"><tr><td colspan="5" class="hint">loading…</td></tr></tbody>
</table>

<h2>Register</h2>
<form id="register" class="file" autocomplete="off">
  <div class="grid">
    <div>
      <div class="formtitle">knowledge base</div>
      <div class="shape" id="f-kind">
        <label class="check"><input type="radio" name="f-kind" value="filesystem" checked> filesystem</label>
        <label class="check"><input type="radio" name="f-kind" value="source_code_repository"> source code repository</label>
        <label class="check"><input type="radio" name="f-kind" value="cloud"> cloud storage</label>
      </div>
    </div>
    <div>
      <label for="f-provider">provider</label>
      <select id="f-provider"></select>
      <div class="hint" id="f-provider-hint"></div>
    </div>
    <div class="needs-provider">
      <div class="formtitle">shape</div>
      <div class="shape" id="f-shape">
        <label class="check"><input type="radio" name="f-shape" value="file" checked> single file</label>
        <label class="check"><input type="radio" name="f-shape" value="directory"> recursive collection</label>
      </div>
      <div class="hint only-file" id="f-file-hint">One document on this provider.</div>
      <div class="hint only-dir" id="f-dir-hint">Recursively. Suffixes empty = all document types.</div>
    </div>
    <div class="only-file needs-provider"><label for="f-name">name</label><input type="text" id="f-name" placeholder="runtime.md"></div>
    <div class="needs-provider">
      <label for="f-location" id="f-location-label">path</label>
      <input type="text" id="f-location" placeholder="/path/to/doc.md">
    </div>
    <div class="only-dir needs-provider">
      <label for="f-suffixes">suffixes <span class="hint">(optional; empty = all)</span></label>
      <input type="text" id="f-suffixes" placeholder="md, markdown">
    </div>
    <div class="only-file needs-provider" id="f-path-wrap"><label for="f-path">path <span class="hint">(inside the location, optional)</span></label><input type="text" id="f-path"></div>
    <div class="needs-provider"><label class="check"><input type="checkbox" id="f-bind" checked> bind to the entity above</label></div>
    <div class="needs-provider"><label class="check"><input type="checkbox" id="f-sync" checked> sync after register</label></div>
  </div>
  <div class="actions"><button type="submit" class="primary" id="f-register">Register</button></div>
</form>

<h2>Search</h2>
<form id="search" autocomplete="off">
  <div class="hint">Searches sources bound to the CONTEXT entity above.</div>
  <div class="row">
    <input type="text" id="f-query" placeholder="search terms">
    <button type="submit">Search</button>
  </div>
  <div id="results"></div>
  <div id="section"></div>
</form>

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

function entityType() {
  const picked = $("f-type").value;
  if (picked === "_other") return $("f-type-custom").value.trim();
  return picked;
}

function entityId() { return $("f-id").value.trim(); }

function currentEntity() {
  const typ = entityType();
  const id = entityId();
  if (!typ || !id) return null;
  return { type: typ, id: id };
}

function scope() {
  const picked = document.querySelector('input[name="f-scope"]:checked');
  return picked ? picked.value : "all";
}

function applyType() {
  $("entity-bar").classList.toggle("show-custom", $("f-type").value === "_other");
}

const KIND_ORDER = ["filesystem", "source_code_repository", "cloud"];
const KIND_LABEL = {
  filesystem: "filesystem",
  source_code_repository: "source code repository",
  cloud: "cloud storage",
};
const KIND_HINT = {
  filesystem: {
    file: "One Markdown file on the runtime host.",
    dir: "Every matching file under the directory, recursively.",
    fileLoc: "file path",
    dirLoc: "directory path",
    filePh: "/path/to/doc.md",
    dirPh: "/path/to/docs",
  },
  source_code_repository: {
    file: "One file in a local git checkout. Revision is the commit SHA.",
    dir: "Walk a local git checkout. Must be inside a work tree.",
    fileLoc: "file path",
    dirLoc: "checkout or subtree",
    filePh: "/path/to/repo/docs/runtime.md",
    dirPh: "/path/to/checkout",
  },
  cloud: {
    file: "No cloud provider is implemented yet.",
    dir: "No cloud provider is implemented yet (s3, gcs, azure, oss).",
    fileLoc: "object key",
    dirLoc: "prefix",
    filePh: "",
    dirPh: "",
  },
};

let backends = {
  filesystem: ["local"],
  source_code_repository: ["git-local"],
  cloud: [],
};

function knowledgeKind() {
  const picked = document.querySelector('input[name="f-kind"]:checked');
  return picked ? picked.value : "filesystem";
}

function shape() {
  const picked = document.querySelector('input[name="f-shape"]:checked');
  return picked ? picked.value : "file";
}

function listFilter() {
  const picked = document.querySelector('input[name="f-filter"]:checked');
  return picked ? picked.value : "";
}

function applyKind() {
  const kind = knowledgeKind();
  const list = backends[kind] || [];
  const sel = $("f-provider");
  sel.innerHTML = list.length
    ? list.map(function (id) { return '<option value="' + esc(id) + '">' + esc(id) + "</option>"; }).join("")
    : '<option value="">none implemented</option>';
  sel.disabled = !list.length;
  const copy = KIND_HINT[kind] || KIND_HINT.filesystem;
  $("f-provider-hint").textContent = list.length
    ? (kind === "source_code_repository"
        ? "Implemented: git-local (a checkout on the runtime host)."
        : "A path on the runtime host.")
    : copy.dir;
  $("register").classList.toggle("no-provider", !list.length);
  $("f-register").disabled = !list.length;
  applyShape();
}

function applyShape() {
  const form = $("register");
  const kind = shape();
  form.classList.toggle("file", kind === "file");
  form.classList.toggle("dir", kind === "directory");
  const copy = KIND_HINT[knowledgeKind()] || KIND_HINT.filesystem;
  $("f-file-hint").textContent = copy.file;
  $("f-dir-hint").textContent = copy.dir;
  $("f-location-label").textContent = kind === "directory" ? copy.dirLoc : copy.fileLoc;
  $("f-location").placeholder = kind === "directory" ? copy.dirPh : copy.filePh;
}

document.querySelectorAll('input[name="f-kind"]').forEach(function (el) { el.addEventListener("change", applyKind); });
document.querySelectorAll('input[name="f-shape"]').forEach(function (el) { el.addEventListener("change", applyShape); });
document.querySelectorAll('input[name="f-filter"]').forEach(function (el) { el.addEventListener("change", render); });
document.querySelectorAll('input[name="f-scope"]').forEach(function (el) { el.addEventListener("change", load); });
$("f-type").addEventListener("change", applyType);
applyKind();
applyType();

function sourceText(s) {
  if (!s) return "";
  return [s.location, s.repository, s.path].filter(Boolean).join(" · ");
}

function bindingPills(list) {
  if (!list || !list.length) return '<span class="hint">unbound</span>';
  return list.map(function (e) {
    return '<span class="pill">' + esc(e.type) + "/" + esc(e.id) + "</span>";
  }).join("");
}

let sources = [];

async function loadBackends() {
  try {
    const out = await api("GET", "/api/backends");
    const next = { filesystem: [], source_code_repository: [], cloud: [] };
    for (const b of (out && out.backends) || []) {
      if (!next[b.kind]) next[b.kind] = [];
      if (b.provider) next[b.kind].push(b.provider);
    }
    backends = next;
  } catch (e) {}
  applyKind();
}

async function load() {
  try {
    let path = "/api/knowledge";
    if (scope() === "entity") {
      const ent = currentEntity();
      if (!ent) { say("entity type and id are required to list bindings", "err"); return; }
      path = "/api/knowledge?entity_type=" + encodeURIComponent(ent.type) +
        "&entity_id=" + encodeURIComponent(ent.id);
    }
    const out = await api("GET", path);
    sources = (out && out.knowledge) || [];
    render();
  } catch (e) {
    sources = [];
    $("rows").innerHTML = '<tr><td colspan="5" class="hint">could not load sources</td></tr>';
    say(e.message, "err");
  }
}

function render() {
  const rows = $("rows");
  const filter = listFilter();
  const visible = sources.filter(function (r) { return !filter || r.kind === filter; });
  if (!visible.length) {
    rows.innerHTML = '<tr><td colspan="5" class="hint">' +
      (sources.length ? "no sources in this knowledge base" : "no sources registered yet — register one below") +
      "</td></tr>";
    return;
  }
  const groups = {};
  for (const r of visible) {
    const key = (r.kind || "") + "/" + (r.provider || "(none)");
    if (!groups[key]) groups[key] = { kind: r.kind, provider: r.provider || "(none)", items: [] };
    groups[key].items.push(r);
  }
  const keys = Object.keys(groups).sort(function (a, b) {
    const ia = KIND_ORDER.indexOf(groups[a].kind);
    const ib = KIND_ORDER.indexOf(groups[b].kind);
    if (ia !== ib) return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib);
    return a.localeCompare(b);
  });
  let html = "";
  for (const key of keys) {
    const g = groups[key];
    html += '<tr class="group"><td colspan="5">' +
      esc(KIND_LABEL[g.kind] || g.kind) + " / " + esc(g.provider) +
      " · " + g.items.length + "</td></tr>";
    html += g.items.map(function (r) {
      return "<tr>" +
        "<td>" + esc(r.name || r.id) + '<div class="hint"><code>' + esc(r.id) + "</code></div></td>" +
        "<td><code>" + esc(sourceText(r.source)) + "</code></td>" +
        "<td>" + bindingPills(r.bindings) + "</td>" +
        "<td><code>" + esc(r.revision || (r.checksum ? r.checksum.slice(0, 12) : "—")) + "</code></td>" +
        '<td><button type="button" data-sync="' + esc(r.id) + '">Sync</button> ' +
        '<button type="button" data-bind="' + esc(r.id) + '">Bind</button> ' +
        '<button type="button" data-unbind="' + esc(r.id) + '">Unbind</button> ' +
        '<button type="button" class="danger" data-delete="' + esc(r.id) + '">Delete</button></td>' +
      "</tr>";
    }).join("");
  }
  rows.innerHTML = html;
}

$("rows").addEventListener("click", async function (event) {
  const syncBtn = event.target.closest("button[data-sync]");
  const bindBtn = event.target.closest("button[data-bind]");
  const unbindBtn = event.target.closest("button[data-unbind]");
  const delBtn = event.target.closest("button[data-delete]");
  try {
    if (syncBtn) {
      const out = await api("POST", "/api/knowledge/" + encodeURIComponent(syncBtn.dataset.sync) + "/sync");
      say("synced " + syncBtn.dataset.sync + ": " + (out.status || "ok") +
        (out.sections != null ? " (" + out.sections + " sections)" : ""));
      await load();
    } else if (bindBtn || unbindBtn) {
      const ent = currentEntity();
      if (!ent) { say("set entity type and id first", "err"); return; }
      const id = (bindBtn || unbindBtn).dataset.bind || (unbindBtn && unbindBtn.dataset.unbind);
      const path = "/api/knowledge/" + encodeURIComponent(id) + "/bindings/" +
        encodeURIComponent(ent.type) + "/" + encodeURIComponent(ent.id);
      if (bindBtn) {
        await api("PUT", path);
        say("bound " + id + " to " + ent.type + "/" + ent.id);
      } else {
        await api("DELETE", path);
        say("unbound " + id + " from " + ent.type + "/" + ent.id);
      }
      await load();
    } else if (delBtn) {
      const id = delBtn.dataset.delete;
      if (!confirm("Delete source " + id + " and its indexed sections? The files themselves are not touched.")) return;
      await api("DELETE", "/api/knowledge/" + encodeURIComponent(id));
      say("deleted " + id);
      await load();
    }
  } catch (e) {
    say(e.message, "err");
  }
});

$("register").addEventListener("submit", async function (event) {
  event.preventDefault();
  const kb = knowledgeKind();
  const provider = $("f-provider").value.trim();
  if (!provider) { say("this knowledge base has no implemented provider yet", "err"); return; }
  const kind = shape();
  const location = $("f-location").value.trim();
  if (!location) { say("location is required", "err"); return; }
  const body = {
    kind: kb,
    provider: provider,
    shape: kind,
    source: { location: location },
  };
  if (kind === "file") {
    body.name = $("f-name").value.trim();
    body.source.path = $("f-path").value.trim();
  } else {
    const suffixes = $("f-suffixes").value.split(/[\s,]+/).map(function (s) { return s.trim(); }).filter(Boolean);
    if (suffixes.length) body.suffixes = suffixes;
  }
  if ($("f-bind").checked) {
    const ent = currentEntity();
    if (!ent) { say("entity type and id are required to bind", "err"); return; }
    body.bind = ent;
  }
  try {
    const created = await api("POST", "/api/knowledge", body);
    let text = "registered " + created.id + " via " + kb + "/" + provider;
    if ($("f-sync").checked) {
      try {
        const out = await api("POST", "/api/knowledge/" + encodeURIComponent(created.id) + "/sync");
        text += ", synced " + (out.status || "ok") + (out.sections != null ? " (" + out.sections + " sections)" : "");
      } catch (e) {
        await load();
        say(text + ", but sync failed: " + e.message, "err");
        return;
      }
    }
    say(text);
    $("register").reset();
    applyKind();
    await load();
  } catch (e) {
    say(e.message, "err");
  }
});

$("search").addEventListener("submit", async function (event) {
  event.preventDefault();
  $("section").innerHTML = "";
  const ent = currentEntity();
  if (!ent) { say("entity type and id are required to search", "err"); return; }
  const q = $("f-query").value.trim();
  if (!q) { say("type something to search for", "err"); return; }
  try {
    const out = await api("GET", "/api/contexts/" + encodeURIComponent(ent.type) + "/" +
      encodeURIComponent(ent.id) + "/search?q=" + encodeURIComponent(q));
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

$("results").addEventListener("click", async function (event) {
  const hit = event.target.closest("[data-section]");
  if (!hit) return;
  try {
    const s = await api("GET", "/api/sections/" + encodeURIComponent(hit.dataset.section));
    $("section").innerHTML = '<div class="panel" style="margin-top:10px">' +
      "<strong>" + esc(s.heading || "(untitled)") + "</strong> " +
      '<span class="hint">' + esc(s.path || "") +
      (s.revision ? " · rev <code>" + esc(s.revision) + "</code>" : "") + "</span>" +
      '<pre class="content">' + esc(s.content) + "</pre></div>";
  } catch (e) {
    say(e.message, "err");
  }
});

$("reload").addEventListener("click", load);
$("f-id").addEventListener("keydown", function (event) { if (event.key === "Enter") load(); });
loadBackends().then(load);
</script>
</body>
</html>
`
