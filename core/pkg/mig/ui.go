package mig

const migConsoleHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>MIG Console</title>
  <style>
    :root {
      --bg: #f5f7fb;
      --panel: #ffffff;
      --text: #172033;
      --muted: #5f6c86;
      --accent: #0666d4;
      --border: #d5deea;
      --ok: #137333;
      --warn: #b06d00;
    }
    body {
      margin: 0;
      font-family: "IBM Plex Sans", "Segoe UI", sans-serif;
      color: var(--text);
      background:
        radial-gradient(circle at 10% -10%, #dce9ff 0, transparent 45%),
        radial-gradient(circle at 85% 0%, #ffe7c2 0, transparent 35%),
        var(--bg);
    }
    .shell {
      max-width: 1200px;
      margin: 0 auto;
      padding: 24px 16px 40px;
    }
    .title {
      font-size: 28px;
      margin: 0 0 4px;
      letter-spacing: -0.02em;
    }
    .subtitle {
      margin: 0 0 18px;
      color: var(--muted);
    }
    .toolbar {
      display: grid;
      gap: 10px;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      margin-bottom: 16px;
    }
    .panel {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 12px;
      box-shadow: 0 6px 20px rgba(17, 38, 73, 0.07);
      padding: 14px;
    }
    .label {
      display: block;
      font-size: 12px;
      color: var(--muted);
      margin-bottom: 6px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    input, button, select {
      width: 100%;
      font: inherit;
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 8px 10px;
      box-sizing: border-box;
      background: #fff;
      color: var(--text);
    }
    button {
      cursor: pointer;
      background: var(--accent);
      color: #fff;
      border-color: var(--accent);
      font-weight: 600;
    }
    .metrics {
      display: grid;
      gap: 10px;
      grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
      margin-bottom: 12px;
    }
    .kpi {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 12px;
    }
    .kpi .name {
      margin: 0;
      font-size: 12px;
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.06em;
    }
    .kpi .value {
      margin: 6px 0 0;
      font-size: 24px;
      font-weight: 700;
    }
    .grid {
      display: grid;
      gap: 12px;
      grid-template-columns: 2fr 1fr;
    }
    @media (max-width: 900px) {
      .grid {
        grid-template-columns: 1fr;
      }
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 13px;
    }
    th, td {
      border-bottom: 1px solid var(--border);
      padding: 8px 6px;
      text-align: left;
      vertical-align: top;
    }
    th {
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .mono {
      font-family: "SFMono-Regular", Menlo, Monaco, monospace;
      font-size: 12px;
      overflow-wrap: anywhere;
    }
    .row {
      display: grid;
      gap: 8px;
      grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
      margin-bottom: 8px;
    }
    pre {
      background: #0f172a;
      color: #d7e4ff;
      border-radius: 10px;
      padding: 10px;
      overflow: auto;
      min-height: 120px;
      margin: 0;
      font-size: 12px;
    }
    .status {
      font-size: 12px;
      margin-top: 8px;
      color: var(--muted);
    }
    .ok { color: var(--ok); }
    .warn { color: var(--warn); }
  </style>
</head>
<body>
  <div class="shell">
    <h1 class="title">MIG Console</h1>
    <p class="subtitle">Open-source runtime visibility for connections, capabilities, usage, and quick invoke testing.</p>

    <div class="toolbar">
      <div class="panel">
        <label class="label" for="tenant">Tenant</label>
        <input id="tenant" value="acme" />
      </div>
      <div class="panel">
        <label class="label" for="token">Bearer Token (optional)</label>
        <input id="token" placeholder="JWT for auth mode=jwt" />
      </div>
      <div class="panel">
        <label class="label" for="interval">Refresh</label>
        <select id="interval">
          <option value="1000">1s</option>
          <option value="2000" selected>2s</option>
          <option value="5000">5s</option>
          <option value="0">paused</option>
        </select>
      </div>
      <div class="panel">
        <label class="label">Actions</label>
        <button id="refresh">Refresh now</button>
      </div>
    </div>

    <div class="metrics">
      <div class="kpi"><p class="name">Active Connections</p><p class="value" id="kpi-connections">0</p></div>
      <div class="kpi"><p class="name">Invocations</p><p class="value" id="kpi-invocations">0</p></div>
      <div class="kpi"><p class="name">Capabilities</p><p class="value" id="kpi-capabilities">0</p></div>
      <div class="kpi"><p class="name">Conformance</p><p class="value" id="kpi-conformance">-</p></div>
    </div>

    <div class="grid">
      <div class="panel">
        <h3>Connections</h3>
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Protocol</th>
              <th>Kind</th>
              <th>Tenant</th>
              <th>Actor</th>
              <th>Remote</th>
              <th>Started</th>
            </tr>
          </thead>
          <tbody id="connections"></tbody>
        </table>
      </div>

      <div class="panel">
        <h3>Quick Actions</h3>
        <div class="row">
          <button id="btn-hello">HELLO</button>
          <button id="btn-discover">DISCOVER</button>
          <button id="btn-invoke">INVOKE</button>
        </div>
        <label class="label" for="invoke-capability">Capability</label>
        <input id="invoke-capability" value="observatory.models.infer" />
        <label class="label" for="invoke-payload">Payload (JSON)</label>
        <input id="invoke-payload" value='{"input":"hello"}' />
        <p class="status" id="action-status">Ready.</p>
        <pre id="action-output">{}</pre>
      </div>
    </div>
  </div>

  <script>
    const state = {
      timer: null,
      tenant: localStorage.getItem("mig.console.tenant") || "acme",
      token: localStorage.getItem("mig.console.token") || "",
      interval: localStorage.getItem("mig.console.interval") || "2000",
    };

    const el = (id) => document.getElementById(id);
    const tenantInput = el("tenant");
    const tokenInput = el("token");
    const intervalInput = el("interval");
    const statusEl = el("action-status");
    const outputEl = el("action-output");

    tenantInput.value = state.tenant;
    tokenInput.value = state.token;
    intervalInput.value = state.interval;

    function headers() {
      const h = {"content-type": "application/json", "x-tenant-id": state.tenant || "acme"};
      if (state.token) h["authorization"] = "Bearer " + state.token;
      return h;
    }

    function setStatus(message, cls) {
      statusEl.textContent = message;
      statusEl.className = "status" + (cls ? " " + cls : "");
    }

    async function getJSON(url) {
      const resp = await fetch(url, {headers: headers()});
      const text = await resp.text();
      let data;
      try { data = JSON.parse(text || "{}"); } catch { data = {raw: text}; }
      if (!resp.ok) throw new Error(text || resp.statusText);
      return data;
    }

    async function postJSON(url, body) {
      const resp = await fetch(url, {method: "POST", headers: headers(), body: JSON.stringify(body)});
      const text = await resp.text();
      let data;
      try { data = JSON.parse(text || "{}"); } catch { data = {raw: text}; }
      if (!resp.ok) throw new Error(text || resp.statusText);
      return data;
    }

    function renderConnections(connections) {
      const tbody = el("connections");
      if (!connections.length) {
        tbody.innerHTML = '<tr><td colspan="7" class="mono">No active long-lived connections.</td></tr>';
        return;
      }
      tbody.innerHTML = connections.map((conn) =>
        '<tr>' +
          '<td class="mono">' + (conn.id || "") + '</td>' +
          '<td>' + (conn.protocol || "") + '</td>' +
          '<td>' + (conn.kind || "") + '</td>' +
          '<td>' + (conn.tenant_id || "") + '</td>' +
          '<td>' + (conn.actor || "") + '</td>' +
          '<td class="mono">' + (conn.remote_addr || "") + '</td>' +
          '<td class="mono">' + (conn.started_at || "") + '</td>' +
        '</tr>'
      ).join("");
    }

    async function refreshDashboard() {
      try {
        const [connections, usage, caps, conf] = await Promise.all([
          getJSON("/admin/v0.1/connections?tenant_id=" + encodeURIComponent(state.tenant || "")),
          getJSON("/cloud/v0.1/usage"),
          getJSON("/admin/v0.1/capabilities"),
          getJSON("/admin/v0.1/health/conformance"),
        ]);

        el("kpi-connections").textContent = String(connections.summary?.total || 0);
        el("kpi-invocations").textContent = String(usage.total_invocations || 0);
        el("kpi-capabilities").textContent = String((caps.capabilities || []).length);
        const confText = conf.full ? "FULL" : (conf.core ? "CORE" : "NO");
        el("kpi-conformance").textContent = confText;
        renderConnections(connections.connections || []);
      } catch (err) {
        setStatus("Dashboard refresh failed: " + err.message, "warn");
      }
    }

    async function runHello() {
      const body = {
        header: {tenant_id: state.tenant || "acme"},
        supported_versions: ["0.1"],
        requested_bindings: ["http"],
      };
      const result = await postJSON("/mig/v0.1/hello", body);
      outputEl.textContent = JSON.stringify(result, null, 2);
      setStatus("HELLO ok", "ok");
    }

    async function runDiscover() {
      const body = {header: {tenant_id: state.tenant || "acme"}};
      const result = await postJSON("/mig/v0.1/discover", body);
      outputEl.textContent = JSON.stringify(result, null, 2);
      setStatus("DISCOVER ok", "ok");
    }

    async function runInvoke() {
      const capability = el("invoke-capability").value.trim();
      let payload;
      try {
        payload = JSON.parse(el("invoke-payload").value || "{}");
      } catch (err) {
        throw new Error("Invalid invoke payload JSON");
      }
      const body = {header: {tenant_id: state.tenant || "acme"}, payload};
      const result = await postJSON("/mig/v0.1/invoke/" + encodeURIComponent(capability), body);
      outputEl.textContent = JSON.stringify(result, null, 2);
      setStatus("INVOKE ok", "ok");
    }

    function resetTimer() {
      if (state.timer) clearInterval(state.timer);
      const ms = Number(state.interval);
      if (ms > 0) state.timer = setInterval(refreshDashboard, ms);
    }

    tenantInput.addEventListener("input", () => {
      state.tenant = tenantInput.value.trim();
      localStorage.setItem("mig.console.tenant", state.tenant);
      refreshDashboard();
    });

    tokenInput.addEventListener("input", () => {
      state.token = tokenInput.value.trim();
      localStorage.setItem("mig.console.token", state.token);
      refreshDashboard();
    });

    intervalInput.addEventListener("change", () => {
      state.interval = intervalInput.value;
      localStorage.setItem("mig.console.interval", state.interval);
      resetTimer();
    });

    el("refresh").addEventListener("click", refreshDashboard);
    el("btn-hello").addEventListener("click", () => runHello().catch((err) => setStatus(err.message, "warn")));
    el("btn-discover").addEventListener("click", () => runDiscover().catch((err) => setStatus(err.message, "warn")));
    el("btn-invoke").addEventListener("click", () => runInvoke().catch((err) => setStatus(err.message, "warn")));

    refreshDashboard();
    resetTimer();
  </script>
</body>
</html>
`
