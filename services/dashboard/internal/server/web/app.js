'use strict';
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const esc = (s) => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

async function api(path, opts = {}) {
  const r = await fetch(path, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, ...opts });
  if (r.status === 401) { showLogin(); throw new Error('unauthorized'); }
  const t = await r.text();
  let j = null; try { j = t ? JSON.parse(t) : null; } catch { j = t; }
  if (!r.ok) throw Object.assign(new Error((j && j.error) || r.statusText), { status: r.status, body: j });
  return j;
}

let RUNTIME_OK = false;

/* ---------- boot / auth ---------- */
async function boot() {
  try {
    const me = await fetch('/api/me', { credentials: 'same-origin' }).then(r => r.json());
    RUNTIME_OK = !!me.runtime_available;
    if (me.authed) { enterApp(me); } else { showLogin(); }
  } catch { showLogin(); }
}
function showLogin() { $('#app').classList.add('hidden'); $('#login').classList.remove('hidden'); $('#login-token').focus(); }
function enterApp(me) {
  $('#login').classList.add('hidden'); $('#app').classList.remove('hidden');
  if (me.dev) $('#devbadge').classList.remove('hidden');
  $('#rt-dot').className = 'dot ' + (RUNTIME_OK ? 'ok' : 'bad');
  $('#db-dot').className = 'dot ok';
  startClock(); refreshAll(); startLoops();
}
$('#login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  try {
    await api('/api/login', { method: 'POST', body: JSON.stringify({ token: $('#login-token').value }) });
    const me = await fetch('/api/me', { credentials: 'same-origin' }).then(r => r.json());
    RUNTIME_OK = !!me.runtime_available; enterApp(me);
  } catch { $('#login-err').textContent = '✕ ACCESS DENIED'; }
});

/* ---------- clock ---------- */
function startClock() {
  const tick = () => { const d = new Date(); $('#clock').textContent = d.toTimeString().slice(0, 8); };
  tick(); setInterval(tick, 1000);
}

/* ---------- nav ---------- */
$$('.nav').forEach(b => b.addEventListener('click', () => setView(b.dataset.view)));
function setView(v) {
  $$('.nav').forEach(b => b.classList.toggle('active', b.dataset.view === v));
  $$('.view').forEach(s => s.classList.toggle('active', s.id === 'view-' + v));
  if (v === 'registry') loadRegistry();
  if (v === 'approvals') loadApprovals();
  if (v === 'audit') loadAudit(200);
}

/* ---------- loops ---------- */
function startLoops() {
  setInterval(loadStats, 5000);
  setInterval(() => { loadApprovals(true); }, 6000);
  setInterval(() => { if ($('#view-audit').classList.contains('active')) loadAudit(200); else loadStats(); }, 4500);
}
function refreshAll() { loadStats(); loadApprovals(true); }

/* ---------- overview ---------- */
function spark(seed) {
  let h = ''; let x = seed % 17 + 3;
  for (let i = 0; i < 16; i++) { x = (x * 31 + 7) % 23; h += `<i style="height:${20 + x * 3.2}%"></i>`; }
  return `<div class="spark">${h}</div>`;
}
async function loadStats() {
  let s; try { s = await api('/api/stats'); } catch { return; }
  $('#rt-dot').className = 'dot ' + (s.runtime_available ? 'ok' : 'bad');
  const err = Math.round(s.error_rate * 100);
  const sys = $('#sys-status');
  if (s.pending_approvals > 0) { sys.className = 'sys warn'; sys.innerHTML = `<i class="dot ok"></i> ${s.pending_approvals} AWAITING APPROVAL`; }
  else { sys.className = 'sys ok'; sys.innerHTML = `<i class="dot ok"></i> SYSTEM NOMINAL`; }

  const cards = [
    { lbl: 'MCP SERVERS', val: s.servers_total, cls: '', sub: `${s.servers_approved} approved · ${s.servers_proposed} proposed`, sp: true },
    { lbl: 'TOOLS EXPOSED', val: s.tools_total, cls: 'amber', sub: `${s.high_impact_tools} high-impact`, sp: true },
    { lbl: 'PENDING APPROVALS', val: s.pending_approvals, cls: s.pending_approvals ? 'red' : 'green', sub: 'human-in-the-loop', sp: false },
    { lbl: 'ACTIVITY · 24H', val: s.audit_24h, cls: '', sub: 'audited calls', sp: true },
    { lbl: 'ERROR / DENY RATE', val: err + '%', cls: err > 20 ? 'red' : 'green', sub: 'of recent calls', sp: false },
    { lbl: 'AVG LATENCY', val: s.avg_latency_ms, cls: 'amber', sub: 'milliseconds', sp: true },
  ];
  $('#telemetry').innerHTML = cards.map((c, i) => `
    <div class="tcard" style="animation-delay:${i * 55}ms">
      <div class="lbl">${c.lbl}</div>
      <div class="val ${c.cls}">${esc(c.val)}</div>
      <div class="sub">${esc(c.sub)}</div>
      ${c.sp ? spark(s[Object.keys(s)[i]] || i + 5) : ''}
    </div>`).join('');

  $('#sysgrid').innerHTML = [
    ['CAPABILITY GATEWAY', s.servers_total > 0 ? 'ok' : 'off', s.servers_total > 0 ? 'ONLINE' : 'IDLE'],
    ['HITL GATING', 'ok', 'ENFORCED'],
    ['AUDIT TRAIL', s.audit_24h > 0 ? 'ok' : 'off', s.audit_24h > 0 ? 'RECORDING' : 'QUIET'],
    ['AGENT RUNTIME', s.runtime_available ? 'ok' : 'off', s.runtime_available ? 'REACHABLE' : 'NOT CONFIGURED'],
    ['FAIL-CLOSED DEFAULTS', 'ok', 'ACTIVE'],
  ].map(([n, st, lbl]) => `<div class="sysrow"><span class="name"><i class="dot ${st === 'ok' ? 'ok' : ''}"></i>${n}</span><span class="state ${st}">${lbl}</span></div>`).join('');

  loadAudit(8, '#ov-audit', true);
}

/* ---------- audit ---------- */
const RES = (r) => r.startsWith('ok') ? 'r-ok' : r.startsWith('denied') ? 'r-deny' : r.startsWith('gated') ? 'r-gate' : r.startsWith('error') ? 'r-err' : 'r-other';
const hhmmss = (iso) => { try { return new Date(iso).toTimeString().slice(0, 8); } catch { return '--'; } };
async function loadAudit(limit = 200, sel = '#audit', mini = false) {
  let d; try { d = await api('/api/audit?limit=' + limit); } catch { return; }
  const rows = d.audit || [];
  const el = $(sel); if (!el) return;
  if (!rows.length) { el.innerHTML = `<div class="empty">no audited calls yet — <b>route a tool through the gateway</b></div>`; return; }
  if (mini) {
    el.innerHTML = rows.map(e => `<div class="mrow"><span class="t">${hhmmss(e.created_at)}</span><span class="tool">${esc(e.tool || e.actor)}</span><span class="res ${RES(e.result)}">${esc((e.result || '').split(':')[0])}</span></div>`).join('');
  } else {
    el.innerHTML = rows.map(e => `<div class="lrow"><span class="t">${hhmmss(e.created_at)}</span><span class="ac">${esc(e.actor || '—')}</span><span class="tl">${esc(e.tool || '—')}</span><span class="${RES(e.result)}">${esc(e.result || '')}</span><span class="ms">${e.latency_ms || 0}</span></div>`).join('');
  }
}

/* ---------- registry ---------- */
async function loadRegistry() {
  let d; try { d = await api('/api/registry'); } catch { return; }
  const servers = d.servers || [];
  const el = $('#registry');
  if (!servers.length) { el.innerHTML = `<div class="empty">registry empty — run <b>gateway sync</b> to register manifests</div>`; return; }
  el.innerHTML = servers.map((s, i) => {
    const tools = (s.tools || []).map(t => `<div class="toolrow"><span>${esc(t.name)}</span><span class="scope">${esc(t.scope)}</span><span class="imp ${t.impact}">${t.impact.toUpperCase()}</span></div>`).join('');
    const promote = s.lifecycle === 'proposed' ? `<button class="mini go" data-act="promote" data-name="${esc(s.name)}">PROMOTE →</button>` : '';
    return `<div class="srv" style="animation-delay:${i * 40}ms">
      <div class="srv-top" data-toggle>
        <div><div class="srv-name">${esc(s.name)}</div><div class="srv-owner">${esc(s.owner)}</div></div>
        <span class="lc ${s.lifecycle}">${s.lifecycle.toUpperCase()}</span>
        <div class="srv-spacer"></div>
        <span class="chip">${(s.tools || []).length} TOOLS</span>
        <span class="chip">${(s.committees || []).join(' · ') || 'no committees'}</span>
        <div class="srv-actions">
          ${promote}
          <div class="toggle ${s.enabled ? 'on' : ''}" data-act="toggle" data-name="${esc(s.name)}" data-en="${s.enabled}" title="enable/disable"></div>
        </div>
      </div>
      <div class="srv-tools">${tools || '<div class="toolrow"><span>no tools</span></div>'}</div>
    </div>`;
  }).join('');
  $$('#registry [data-toggle]').forEach(t => t.addEventListener('click', e => { if (e.target.closest('[data-act]')) return; t.closest('.srv').classList.toggle('open'); }));
  $$('#registry [data-act="promote"]').forEach(b => b.addEventListener('click', () => act('promote', b.dataset.name)));
  $$('#registry [data-act="toggle"]').forEach(b => b.addEventListener('click', () => act('toggle', b.dataset.name, b.dataset.en === 'true')));
}
async function act(kind, name, enabled) {
  try {
    if (kind === 'promote') { await api(`/api/registry/${encodeURIComponent(name)}/lifecycle`, { method: 'POST', body: JSON.stringify({ lifecycle: 'approved' }) }); toast(`${name} → APPROVED`, 'good'); }
    if (kind === 'toggle') { await api(`/api/registry/${encodeURIComponent(name)}/enabled`, { method: 'POST', body: JSON.stringify({ enabled: !enabled }) }); toast(`${name} ${enabled ? 'DISABLED' : 'ENABLED'}`, 'good'); }
    loadRegistry(); loadStats();
  } catch (e) { toast(e.message, 'bad'); }
}

/* ---------- approvals ---------- */
async function loadApprovals(badgeOnly = false) {
  let d; try { d = await api('/api/approvals'); } catch { return; }
  const items = d.approvals || [];
  const badge = $('#rail-appr');
  badge.textContent = items.length; badge.classList.toggle('hidden', items.length === 0);
  if (badgeOnly && !$('#view-approvals').classList.contains('active')) return;
  const el = $('#approvals');
  if (!items.length) { el.innerHTML = `<div class="empty">no pending approvals — <b>nothing is waiting on a human</b></div>`; return; }
  el.innerHTML = items.map(a => `<div class="appr">
      <div class="flag">⚑ HIGH-IMPACT · AWAITING DECISION</div>
      <div class="tool">${esc(a.tool)}</div>
      <div class="meta">session ${esc((a.session_id || '—').slice(0, 8))} · ${hhmmss(a.created_at)}</div>
      <div class="args">${esc(JSON.stringify(a.args_redacted))}</div>
      <div class="appr-btns">
        <button class="btn-deny" data-id="${a.id}" data-s="denied">DENY</button>
        <button class="btn-approve" data-id="${a.id}" data-s="approved">APPROVE</button>
      </div></div>`).join('');
  $$('#approvals [data-id]').forEach(b => b.addEventListener('click', () => decide(b.dataset.id, b.dataset.s)));
}
async function decide(id, status) {
  try {
    await api(`/api/approvals/${id}/decide`, { method: 'POST', body: JSON.stringify({ status, decided_by: 'dashboard-maintainer' }) });
    toast(`approval ${status.toUpperCase()}`, status === 'approved' ? 'good' : 'bad');
    loadApprovals(); loadStats();
  } catch (e) { toast(e.message, 'bad'); }
}

/* ---------- console ---------- */
let MODE = 'goal', TASK = null, POLL = null, SEEN = 0;
$('#mode-goal').addEventListener('click', () => setMode('goal'));
$('#mode-recipe').addEventListener('click', () => setMode('recipe'));
function setMode(m) { MODE = m; $('#mode-goal').classList.toggle('seg-on', m === 'goal'); $('#mode-recipe').classList.toggle('seg-on', m === 'recipe'); $('#f-goal').classList.toggle('hidden', m !== 'goal'); $('#f-recipe').classList.toggle('hidden', m !== 'recipe'); }
$('#submit').addEventListener('click', submitTask);

async function submitTask() {
  if (!RUNTIME_OK) { toast('runtime not configured', 'bad'); return; }
  const body = { identity: $('#in-identity').value || 'dashboard', committee: $('#in-committee').value || '', hard_task: $('#in-hard').checked };
  if (MODE === 'goal') body.inline_goal = $('#in-goal').value.trim(); else body.recipe_id = $('#in-recipe').value.trim();
  if (!body.inline_goal && !body.recipe_id) { toast('enter a goal or recipe', 'bad'); return; }
  $('#submit').disabled = true; SEEN = 0; $('#term-approve').classList.add('hidden');
  $('#stream').innerHTML = `<div class="evt status"><span class="pre">❯</span><span class="txt term-cursor">submitting task to runtime…</span></div>`;
  setTermState('run', `agent://${MODE === 'goal' ? 'goal' : body.recipe_id}`);
  try {
    const r = await api('/api/tasks', { method: 'POST', body: JSON.stringify(body) });
    TASK = r.task_id; pushEvt('status', '❯', `task ${TASK} accepted`);
    POLL = setInterval(pollTask, 700); pollTask();
  } catch (e) { pushEvt('error', '✕', e.message); setTermState('', 'agent://error'); $('#submit').disabled = false; }
}
async function pollTask() {
  if (!TASK) return;
  let s; try { s = await api('/api/tasks/' + TASK); } catch { return; }
  (s.events || []).slice(SEEN).forEach(e => renderEvt(e));
  SEEN = (s.events || []).length;
  if (s.status === 'awaiting_approval' && s.pending_approval) {
    setTermState('wait', `agent://${TASK}`); $('#ta-tool').textContent = s.pending_approval.tool;
    $('#term-approve').dataset.appr = s.pending_approval.approval_id; $('#term-approve').classList.remove('hidden');
  } else { $('#term-approve').classList.add('hidden'); }
  if (s.status === 'done' || s.status === 'error') {
    clearInterval(POLL); POLL = null; $('#submit').disabled = false;
    setTermState(s.status === 'done' ? 'done' : '', `agent://${s.status}`);
    if (s.result && s.result.output) pushEvt('output', '◀', s.result.output);
    pushEvt('done', '■', `task ${s.status} · ${SEEN} events`);
  }
}
$('#ta-approve').addEventListener('click', () => resolveAppr(true));
$('#ta-deny').addEventListener('click', () => resolveAppr(false));
async function resolveAppr(granted) {
  const id = $('#term-approve').dataset.appr;
  try {
    await api(`/api/tasks/${TASK}/approve`, { method: 'POST', body: JSON.stringify({ approval_id: id, granted, decided_by: 'dashboard-maintainer' }) });
    $('#term-approve').classList.add('hidden'); pushEvt('approval', granted ? '✓' : '✕', `${granted ? 'approved' : 'denied'} ${id}`);
    setTermState('run', `agent://${TASK}`);
  } catch (e) { toast(e.message, 'bad'); }
}
function renderEvt(e) {
  const map = { tool_call: ['tool', '⚙', `call ${e.tool}`], approval_required: ['approval', '⚑', `approval required · ${e.tool}`], output: ['output', '◀', e.text], error: ['error', '✕', e.text], done: ['done', '■', e.text || 'done'], status: ['status', '·', e.text] };
  const [cls, pre, txt] = map[e.kind] || ['status', '·', e.text || e.kind];
  pushEvt(cls, pre, txt);
}
function pushEvt(cls, pre, txt) {
  const st = $('#stream'); if ($('.stream-empty')) st.innerHTML = '';
  const d = document.createElement('div'); d.className = 'evt ' + cls;
  d.innerHTML = `<span class="pre">${esc(pre)}</span><span class="txt">${esc(txt)}</span>`;
  st.appendChild(d); st.scrollTop = st.scrollHeight;
}
function setTermState(cls, title) { const e = $('#term-state'); e.className = 'term-state ' + cls; e.textContent = cls === 'run' ? 'RUNNING' : cls === 'wait' ? 'AWAITING APPROVAL' : cls === 'done' ? 'COMPLETE' : 'READY'; $('#term-title').textContent = title; }

/* ---------- toast ---------- */
let toastT;
function toast(msg, kind = '') { const t = $('#toast'); t.textContent = msg; t.className = 'toast ' + kind; clearTimeout(toastT); toastT = setTimeout(() => t.classList.add('hidden'), 3200); }

boot();
