"use strict";
// scans.anonshield.org dashboard — fetches /api/v1/* and renders KPIs, hosts,
// scans-over-time bar chart, severity pie, and the top containers table.

const API = (location.pathname.endsWith("/") ? "" : "/") + "api/v1";
const REFRESH_MS = 30_000;
const PAGE_SIZE = 100;

const fmt = (n) => n == null ? "–" : Number(n).toLocaleString("pt-BR");
const fmtBig = (n) => {
  if (n == null) return "–";
  const v = Number(n);
  if (v >= 1e9) return (v/1e9).toFixed(1) + " bi";
  if (v >= 1e6) return (v/1e6).toFixed(1) + " mi";
  if (v >= 1e3) return (v/1e3).toFixed(1) + " k";
  return v.toString();
};
const fmtPct = (a, b) => (b > 0 ? ((a/b)*100).toFixed(2) + "%" : "–");
const fmtTs = (iso) => {
  if (!iso) return "–";
  const d = new Date(iso);
  return d.toLocaleString("pt-BR", { hour12: false });
};
const ago = (iso) => {
  if (!iso) return "";
  const s = (Date.now() - new Date(iso).getTime()) / 1000;
  if (s < 60) return Math.floor(s) + " s atrás";
  if (s < 3600) return Math.floor(s/60) + " min atrás";
  if (s < 86400) return Math.floor(s/3600) + " h atrás";
  return Math.floor(s/86400) + " d atrás";
};

let containersState = { all: [], filtered: [], offset: 0, n_total: 0, generated_at: null };
let charts = { timeline: null, severity: null, scannerSev: null, scannerCov: null, hist: null, histVuln: null, topVuln: null };

const SCANNERS = ["syft", "trivy", "grype", "osv", "dockle", "trufflehog"];
const SEV_COLORS = { c: "#eb5757", h: "#f28b30", m: "#f2c94c", l: "#56a8ff", i: "#8b95a8", u: "#555" };
const SEV_LABELS = { c: "crítica", h: "alta", m: "média", l: "baixa", i: "info", u: "desc." };

async function api(path) {
  const r = await fetch(API + path, { cache: "no-store" });
  if (!r.ok) throw new Error(`${path} → HTTP ${r.status}`);
  return r.json();
}

// ── KPIs ────────────────────────────────────────────────────────────────────
async function renderKpis() {
  const live = await api("/dit-live").catch(() => null);
  const q = live?.queue || {};
  const total = q.total || 0;
  const done = q.done || 0;
  const pending = q.pending || 0;
  const skipped = q.skipped || 0;
  const failed = q.failed || 0;
  const findings = q.findings || 0;
  const rate = live?.rate_per_min;

  const cards = [
    { lbl: "containers escaneados", val: fmt(done), sub: fmtPct(done, total) + " de " + fmt(total), klass: "pct" },
    { lbl: "pendentes na fila", val: fmt(pending), sub: skipped > 0 ? `${fmt(skipped)} já feitos · ${fmt(failed)} falhas` : `${fmt(failed)} falhas` },
    { lbl: "findings consolidados", val: fmtBig(findings), sub: "soma bruta dos relatórios" },
    { lbl: "ritmo atual", val: (rate == null ? "–" : rate.toFixed(2)) + "/min", sub: "últimas amostras de 15 min", klass: "rate" },
  ];
  document.getElementById("kpis").innerHTML = cards.map(c =>
    `<div class="kpi"><div class="lbl">${c.lbl}</div><div class="val ${c.klass||''}">${c.val}</div><div class="sub">${c.sub||''}</div></div>`
  ).join("");
}

// ── máquinas ────────────────────────────────────────────────────────────────
async function renderHosts() {
  const data = await api("/machines").catch(() => ({ hosts: [] }));
  const hosts = (data.hosts || []).slice().sort((a, b) => (a.host||'').localeCompare(b.host||''));
  document.getElementById("hosts").innerHTML = hosts.map(h => {
    const onlineCls = h.online ? "online" : "";
    const role = h.role || "worker";
    const wk = h.workers_alive ?? "–";
    const load = h.load1 == null ? "–" : Number(h.load1).toFixed(2);
    const ramGb = h.ram_avail_mb == null ? "–" : (h.ram_avail_mb/1024).toFixed(1) + " GB";
    return `<div class="host ${onlineCls}">
      <div class="dot"></div>
      <h3>${h.host || '?'}</h3>
      <div class="role">${role}</div>
      <div class="stats">
        <div><span>workers</span><b>${wk}</b></div>
        <div><span>load 1m</span><b>${load}</b></div>
        <div><span>ram livre</span><b>${ramGb}</b></div>
        <div><span>status</span><b style="color:${h.online ? 'var(--ok)' : 'var(--err)'}">${h.online ? 'online' : 'offline'}</b></div>
      </div>
    </div>`;
  }).join("");
}

// ── gráfico de timeline ─────────────────────────────────────────────────────
async function renderTimeline() {
  const data = await api("/queue/timeline?bucket_minutes=60&hours=48").catch(() => null);
  if (!data) return;
  const fullLabels = data.buckets.map(b => {
    const d = new Date(b.ts);
    const pad = (n) => String(n).padStart(2, "0");
    return `${d.getUTCFullYear()}-${pad(d.getUTCMonth()+1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:00 UTC`;
  });
  const labels = data.buckets.map(b => {
    const d = new Date(b.ts);
    return d.getUTCHours() + "h " + (d.getUTCMonth()+1) + "/" + d.getUTCDate();
  });
  const done = data.buckets.map(b => b.done);
  const ctx = document.getElementById("chart-timeline").getContext("2d");
  if (charts.timeline) charts.timeline.destroy();
  charts.timeline = new Chart(ctx, {
    type: "bar",
    data: { labels, datasets: [{ label: "scans/h", data: done, backgroundColor: "rgba(93,200,255,.7)", borderColor: "rgba(93,200,255,1)", borderWidth: 1 }] },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            title: (items) => fullLabels[items[0].dataIndex],
            label: (c) => `${fmt(c.parsed.y)} relatórios fechados`,
          },
        },
      },
      scales: {
        x: {
          title: { display: true, text: "hora (UTC)", color: "#8b95a8", font: { size: 11 } },
          ticks: { color: "#8b95a8", maxRotation: 0, autoSkipPadding: 24 },
          grid: { display: false },
        },
        y: {
          title: { display: true, text: "relatórios fechados", color: "#8b95a8", font: { size: 11 } },
          ticks: { color: "#8b95a8" },
          grid: { color: "#222a3a" },
        },
      },
    },
  });
}

// ── gráficos baseados em containersState.all ────────────────────────────────
async function renderScannerCharts() {
  // findings por scanner empilhados por severidade.
  //
  // Preferimos o endpoint pre-computado /api/v1/scanner-stats (que agrega
  // findings_by_severity de TODOS os reports via cron na gpu1, totalizando
  // ~49M findings). O fallback usa containersState.all, que só traz
  // by_scanner para o top-500 (~1.3M findings) — útil quando o arquivo
  // pré-computado ainda não foi gerado.
  const perScanner = {}; // {scanner: {c,h,m,l,i,u, total, ok, runs}}
  const stats = await api("/scanner-stats").catch(() => null);
  if (stats && stats.scanners && Object.keys(stats.scanners).length) {
    for (const sc of SCANNERS) {
      const s = stats.scanners[sc] || {};
      perScanner[sc] = {
        c: s.c|0, h: s.h|0, m: s.m|0, l: s.l|0, i: s.i|0, u: s.u|0,
        total: s.n_findings|0, runs: s.n_runs|0, ok: s.n_ok|0,
      };
    }
  } else {
    for (const sc of SCANNERS) perScanner[sc] = { c:0,h:0,m:0,l:0,i:0,u:0, total:0, ok:0, runs:0 };
    for (const ct of containersState.all) {
      for (const sc of SCANNERS) {
        const v = ct.by_scanner?.[sc];
        if (!v) continue;
        perScanner[sc].runs += 1;
        if ((v.status||"").startsWith("ok")) perScanner[sc].ok += 1;
        for (const k of ["c","h","m","l","i","u"]) perScanner[sc][k] += v[k]||0;
        perScanner[sc].total += v.n||0;
      }
    }
  }
  const ctx = document.getElementById("chart-scanner-sev")?.getContext("2d");
  if (!ctx) return;
  if (charts.scannerSev) charts.scannerSev.destroy();
  charts.scannerSev = new Chart(ctx, {
    type: "bar",
    data: {
      labels: SCANNERS,
      datasets: ["c","h","m","l","i","u"].map(k => ({
        label: SEV_LABELS[k],
        data: SCANNERS.map(s => perScanner[s][k]),
        backgroundColor: SEV_COLORS[k],
      })),
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: { legend: { position: "bottom", labels: { color: "#e6e8ee", boxWidth: 11, font:{size:11} } } },
      scales: {
        x: {
          stacked: true,
          title: { display: true, text: "scanner", color: "#8b95a8", font: { size: 11 } },
          ticks: { color: "#8b95a8" },
          grid: { display:false },
        },
        y: {
          stacked: true,
          title: { display: true, text: "findings", color: "#8b95a8", font: { size: 11 } },
          ticks: { color: "#8b95a8" },
          grid: { color: "#222a3a" },
        },
      },
    },
  });

  // cobertura: % de runs com status ok* para cada scanner
  const ctx2 = document.getElementById("chart-scanner-cov")?.getContext("2d");
  if (!ctx2) return;
  if (charts.scannerCov) charts.scannerCov.destroy();
  const pct = SCANNERS.map(s => perScanner[s].runs ? (perScanner[s].ok/perScanner[s].runs)*100 : 0);
  charts.scannerCov = new Chart(ctx2, {
    type: "bar",
    data: { labels: SCANNERS, datasets: [{ label: "% ok", data: pct, backgroundColor: "rgba(111,207,151,.65)", borderColor: "#6fcf97", borderWidth: 1 }] },
    options: {
      indexAxis: "y",
      responsive: true, maintainAspectRatio: false,
      plugins: { legend: { display:false }, tooltip:{ callbacks:{ label:(c)=>`${c.parsed.x.toFixed(1)}%` } } },
      scales: {
        x: {
          min:0, max:100,
          title: { display: true, text: "% containers com status=ok", color: "#8b95a8", font: { size: 11 } },
          ticks: { color:"#8b95a8", callback:(v)=>v+"%" },
          grid:{ color:"#222a3a" },
        },
        y: {
          title: { display: true, text: "scanner", color: "#8b95a8", font: { size: 11 } },
          ticks: { color: "#8b95a8" },
          grid: { display:false },
        },
      },
    },
  });
}

function renderDistributions() {
  // histograma de findings (merged) por container
  const bins = [
    { l: "0", min: 0, max: 0 },
    { l: "1–10", min: 1, max: 10 },
    { l: "11–100", min: 11, max: 100 },
    { l: "101–500", min: 101, max: 500 },
    { l: "501–2k", min: 501, max: 2000 },
    { l: "2k–10k", min: 2001, max: 10000 },
    { l: ">10k", min: 10001, max: Infinity },
  ];
  const counts = bins.map(()=>0);
  for (const ct of containersState.all) {
    const n = ct.merged || 0;
    for (let i = 0; i < bins.length; i++) if (n >= bins[i].min && n <= bins[i].max) { counts[i]++; break; }
  }
  const ctx = document.getElementById("chart-hist")?.getContext("2d");
  if (ctx) {
    if (charts.hist) charts.hist.destroy();
    charts.hist = new Chart(ctx, {
      type: "bar",
      data: { labels: bins.map(b=>b.l), datasets: [{ data: counts, backgroundColor: "rgba(93,200,255,.65)", borderColor: "#5dc8ff", borderWidth: 1 }] },
      options: { responsive:true, maintainAspectRatio:false,
        plugins:{
          legend:{display:false},
          tooltip:{ callbacks:{ label:(c)=>`${fmt(c.parsed.y)} containers` } },
        },
        scales:{
          x:{
            title: { display: true, text: "faixa de findings", color: "#8b95a8", font: { size: 11 } },
            ticks:{color:"#8b95a8"}, grid:{display:false},
          },
          y:{
            title: { display: true, text: "número de containers", color: "#8b95a8", font: { size: 11 } },
            ticks:{color:"#8b95a8"}, grid:{color:"#222a3a"},
          },
        } },
    });
  }

  // histograma de vulnerabilidades (crit+high+med+low) por container — exclui só info/unknown
  const vbins = [
    { l: "0", min: 0, max: 0 },
    { l: "1–10", min: 1, max: 10 },
    { l: "11–50", min: 11, max: 50 },
    { l: "51–200", min: 51, max: 200 },
    { l: "201–1k", min: 201, max: 1000 },
    { l: ">1k", min: 1001, max: Infinity },
  ];
  const vcounts = vbins.map(()=>0);
  for (const ct of containersState.all) {
    let chml = 0;
    for (const v of Object.values(ct.by_scanner||{})) chml += (v.c||0) + (v.h||0) + (v.m||0) + (v.l||0);
    for (let i = 0; i < vbins.length; i++) if (chml >= vbins[i].min && chml <= vbins[i].max) { vcounts[i]++; break; }
  }
  const ctxV = document.getElementById("chart-hist-vuln")?.getContext("2d");
  if (ctxV) {
    if (charts.histVuln) charts.histVuln.destroy();
    charts.histVuln = new Chart(ctxV, {
      type: "bar",
      data: { labels: vbins.map(b=>b.l), datasets: [{ data: vcounts, backgroundColor: "rgba(235,87,87,.55)", borderColor: "#eb5757", borderWidth: 1 }] },
      options: { responsive:true, maintainAspectRatio:false, plugins:{ legend:{display:false},
        tooltip:{ callbacks:{ label:(c)=>`${fmt(c.parsed.y)} containers (crit+alta+média+baixa)` } } },
        scales:{
          x:{
            title: { display: true, text: "faixa de findings (crítica + alta + média + baixa)", color: "#8b95a8", font: { size: 11 } },
            ticks:{color:"#8b95a8"}, grid:{display:false},
          },
          y:{
            title: { display: true, text: "número de containers", color: "#8b95a8", font: { size: 11 } },
            ticks:{color:"#8b95a8"}, grid:{color:"#222a3a"},
          },
        } },
    });
  }

  // top 15 mais vulneráveis (críticos + altos)
  const ranked = containersState.all.map(ct => {
    let ch = 0;
    for (const v of Object.values(ct.by_scanner||{})) ch += (v.c||0) + (v.h||0);
    return { image: ct.image, ch };
  }).filter(x => x.ch > 0).sort((a,b)=>b.ch-a.ch).slice(0,15);
  const ctx2 = document.getElementById("chart-top-vuln")?.getContext("2d");
  if (ctx2) {
    if (charts.topVuln) charts.topVuln.destroy();
    const labels = ranked.map(r => {
      // mostra "ns/repo:tag" sem o "@sha256:...". Se ja for curto, usa inteiro.
      const noDigest = r.image.split("@")[0];
      return noDigest.length > 50 ? noDigest.slice(0, 48) + "…" : noDigest;
    });
    charts.topVuln = new Chart(ctx2, {
      type: "bar",
      data: { labels, datasets: [{ data: ranked.map(r=>r.ch), backgroundColor: "rgba(235,87,87,.65)", borderColor: "#eb5757", borderWidth: 1 }] },
      options: { indexAxis:"y", responsive:true, maintainAspectRatio:false,
        plugins:{
          legend:{display:false},
          tooltip:{ callbacks:{ label:(c)=>`${fmt(c.parsed.x)} findings (crit+alta)` } },
        },
        scales:{
          x:{
            title: { display: true, text: "findings (crítica + alta)", color: "#8b95a8", font: { size: 11 } },
            ticks:{color:"#8b95a8"}, grid:{color:"#222a3a"},
          },
          y:{
            title: { display: true, text: "imagem", color: "#8b95a8", font: { size: 11 } },
            ticks:{color:"#8b95a8", font:{size:10, family:"ui-monospace,monospace"}},
            grid:{display:false},
          },
        } },
    });
  }
}

// ── pie de severidade ───────────────────────────────────────────────────────
function renderSeverity() {
  const sums = { c: 0, h: 0, m: 0, l: 0, i: 0, u: 0 };
  for (const ct of containersState.all) {
    for (const sc of Object.values(ct.by_scanner || {})) {
      sums.c += sc.c||0; sums.h += sc.h||0; sums.m += sc.m||0;
      sums.l += sc.l||0; sums.i += sc.i||0; sums.u += sc.u||0;
    }
  }
  const ctx = document.getElementById("chart-severity").getContext("2d");
  if (charts.severity) charts.severity.destroy();
  charts.severity = new Chart(ctx, {
    type: "doughnut",
    data: {
      labels: ["crítica","alta","média","baixa","info","desconhecida"],
      datasets: [{ data: [sums.c, sums.h, sums.m, sums.l, sums.i, sums.u],
                   backgroundColor: ["#eb5757","#f28b30","#f2c94c","#56a8ff","#8b95a8","#555"],
                   borderColor: "#161a22", borderWidth: 2 }]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: { legend: { position: "right", labels: { color: "#e6e8ee", boxWidth: 12, font: { size: 11 } } } },
      cutout: "55%"
    }
  });
}

// ── tabela ──────────────────────────────────────────────────────────────────
function applyFilter(qstr) {
  const q = (qstr || "").trim().toLowerCase();
  containersState.filtered = q
    ? containersState.all.filter(c => c.image.toLowerCase().includes(q))
    : containersState.all;
  containersState.offset = 0;
  renderTable();
}

function renderTable() {
  const items = containersState.filtered.slice(containersState.offset, containersState.offset + PAGE_SIZE);
  const tbody = document.querySelector("#tbl tbody");
  tbody.innerHTML = items.map(ct => {
    const sc = (name) => {
      const v = ct.by_scanner?.[name];
      if (!v) return `<td class="num">–</td>`;
      const sev = `<span class="sev">${["c","h","m","l","i","u"].map(k => v[k] > 0 ? `<span class="${k}" title="${k}:${v[k]}"></span>` : "").join("")}</span>`;
      return `<td class="num"><b>${fmt(v.n)}</b> ${sev}</td>`;
    };
    return `<tr>
      <td class="num">${ct.rank}</td>
      <td class="num">${fmtBig(ct.exposure)}</td>
      <td class="img" title="${ct.image}">${ct.image}</td>
      <td class="num"><b>${fmt(ct.merged)}</b></td>
      ${sc("syft")}${sc("trivy")}${sc("grype")}${sc("osv")}${sc("dockle")}${sc("trufflehog")}
    </tr>`;
  }).join("");
  document.getElementById("count").textContent =
    `${fmt(containersState.filtered.length)} de ${fmt(containersState.all.length)} containers`;
  const start = containersState.offset + 1;
  const end = Math.min(containersState.offset + PAGE_SIZE, containersState.filtered.length);
  document.getElementById("page-info").textContent =
    containersState.filtered.length ? `mostrando ${fmt(start)}–${fmt(end)}` : "sem resultados";
  document.getElementById("prev").disabled = containersState.offset === 0;
  document.getElementById("next").disabled = end >= containersState.filtered.length;
}

async function loadContainers() {
  // pega o snapshot inteiro de uma vez (cache no servidor faz isso rapido)
  const data = await api("/containers?limit=100000").catch(() => null);
  if (!data) return;
  containersState.all = data.containers;
  containersState.n_total = data.n_total_scanned;
  containersState.generated_at = data.generated_at;
  applyFilter(document.getElementById("q").value);
  renderSeverity();
  renderScannerCharts();
  renderDistributions();
}

// ── eventos ─────────────────────────────────────────────────────────────────
document.getElementById("q").addEventListener("input", (e) => applyFilter(e.target.value));
document.getElementById("prev").addEventListener("click", () => { containersState.offset = Math.max(0, containersState.offset - PAGE_SIZE); renderTable(); });
document.getElementById("next").addEventListener("click", () => { containersState.offset += PAGE_SIZE; renderTable(); });

async function renderRecent() {
  const recent = await api("/queue/recent?limit=30").catch(() => []);
  document.getElementById("recent-list").innerHTML = recent.map(r => {
    const when = r.finished_at ? ago(new Date(r.finished_at * 1000).toISOString()) : "—";
    return `<div class="item">
      <div class="img" title="${r.image}">${r.image}</div>
      <div class="meta"><span class="findings">${fmt(r.findings)}</span> findings · <span>${when}</span></div>
    </div>`;
  }).join("");
}

function esc(s){ return String(s == null ? '' : s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]); }

// 3 listas separadas: pending (próximos na fila), skipped (pulados), failed (falhas)
async function renderStatusList(status, rootId, emptyMsg) {
  const root = document.getElementById(rootId);
  if (!root) return;
  const list = await api(`/queue/top?limit=25&status=${status}`).catch(() => []);
  if (!list.length) {
    root.innerHTML = `<div class="item"><div class="img">${esc(emptyMsg)}</div></div>`;
    return;
  }
  root.innerHTML = list.map((r, idx) => {
    const err = r.error ? ` · <span title="${esc(r.error)}" style="color:var(--err);font-size:10px">${esc(String(r.error).slice(0,60))}</span>` : "";
    return `<div class="item">
      <div class="img" title="${esc(r.image)}"><span class="pill">#${idx+1}</span> ${esc(r.image)}</div>
      <div class="meta"><span class="findings">${fmtBig(r.weight)}</span> exposição · <span>tent. ${r.attempts || 0}</span>${err}</div>
    </div>`;
  }).join("");
}

async function renderQueuePending() { return renderStatusList("pending", "queue-pending-list", "nada pendente — fila esvaziada"); }
async function renderQueueSkipped() { return renderStatusList("skipped", "queue-skipped-list", "nenhum alvo pulado"); }
async function renderQueueFailed()  { return renderStatusList("failed",  "queue-failed-list",  "nenhuma falha registrada"); }

// ── fila paginada (todos os pendentes/skipped/failed) ──────────────────────
let queueState = { all: [], filtered: [], offset: 0, q: "" };
const QPAGE_SIZE = 100;
const STATUS_BADGE = {
  pending: { txt: "pendente", col: "#56a8ff" },
  skipped: { txt: "pulado",   col: "#f2c94c" },
  failed:  { txt: "falhou",   col: "#eb5757" },
  running: { txt: "rodando",  col: "#6fcf97" },
};

async function loadQueueAll() {
  const list = await api(`/queue/top?limit=10000&status=pending`).catch(() => []);
  queueState.all = list.map(r => ({...r, _st: "pending"}));
  applyQueueFilter(document.getElementById("qq")?.value || "");
}

function applyQueueFilter(qstr) {
  const q = (qstr || "").trim().toLowerCase();
  queueState.q = q;
  queueState.filtered = q ? queueState.all.filter(r => (r.image||'').toLowerCase().includes(q)) : queueState.all;
  queueState.offset = 0;
  renderQueueTable();
}

function renderQueueTable() {
  const items = queueState.filtered.slice(queueState.offset, queueState.offset + QPAGE_SIZE);
  const tbody = document.querySelector("#qtbl tbody");
  if (!tbody) return;
  tbody.innerHTML = items.map((r, i) => {
    const meta = STATUS_BADGE[r._st] || { txt: r._st, col: "#8b95a8" };
    const err = r.error ? `<span title="${esc(r.error)}" style="color:var(--err)">${esc(String(r.error).slice(0, 80))}</span>` : '<span style="color:var(--mut)">—</span>';
    return `<tr>
      <td class="num">${queueState.offset + i + 1}</td>
      <td class="num">${fmtBig(r.weight)}</td>
      <td class="img" title="${esc(r.image)}">${esc(r.image)}</td>
      <td><span class="pill" style="color:${meta.col}">${meta.txt}</span></td>
      <td class="num">${r.attempts || 0}</td>
      <td class="img" style="max-width:380px">${err}</td>
    </tr>`;
  }).join("");
  document.getElementById("qcount").textContent =
    `${fmt(queueState.filtered.length)} de ${fmt(queueState.all.length)} na fila não-concluídos`;
  const start = queueState.offset + 1;
  const end = Math.min(queueState.offset + QPAGE_SIZE, queueState.filtered.length);
  document.getElementById("qpage-info").textContent =
    queueState.filtered.length ? `mostrando ${fmt(start)}–${fmt(end)}` : "sem resultados";
  document.getElementById("qprev").disabled = queueState.offset === 0;
  document.getElementById("qnext").disabled = end >= queueState.filtered.length;
}

document.getElementById("qq")?.addEventListener("input", (e) => applyQueueFilter(e.target.value));
document.getElementById("qprev")?.addEventListener("click", () => { queueState.offset = Math.max(0, queueState.offset - QPAGE_SIZE); renderQueueTable(); });
document.getElementById("qnext")?.addEventListener("click", () => { queueState.offset += QPAGE_SIZE; renderQueueTable(); });

async function refreshAll() {
  document.getElementById("last-update").textContent = "atualizando…";
  await Promise.allSettled([renderKpis(), renderHosts(), renderTimeline(), renderRecent(), renderQueuePending(), renderQueueSkipped(), renderQueueFailed(), loadQueueAll(), loadContainers()]);
  document.getElementById("last-update").textContent = "última atualização: agora · F5 recarrega";
}

refreshAll();
setInterval(refreshAll, REFRESH_MS);
