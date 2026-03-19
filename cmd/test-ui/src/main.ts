/**
 * Aftertalk Test UI — reference frontend implementation
 *
 * Architecture (two distinct layers):
 *
 *   ┌─ Middleware (privileged) ─────────────────────────────────────────────┐
 *   │  PHP server-side layer. Holds the API key. The browser forwards its   │
 *   │  local key via X-API-Key ONLY for test convenience — in production     │
 *   │  the key is stored server-side only and never leaves PHP.             │
 *   └──────────────────────────────────────────────────────────────────────┘
 *        ↕  POST /rooms/join   POST /sessions/end   GET /minutes
 *
 *   ┌─ AftertalkClient (SDK, no API key) ──────────────────────────────────┐
 *   │  Used directly from the browser for:                                 │
 *   │  • GET /v1/config     — public, no auth                              │
 *   │  • GET /v1/rtc-config — public, no auth                              │
 *   │  • WS  /signaling     — authenticated via JWT session token          │
 *   └──────────────────────────────────────────────────────────────────────┘
 */

import {
  AftertalkClient,
  AftertalkError,
  type Template,
  type WebRTCConnection,
} from '@aftertalk/sdk';

// ─── Middleware client ────────────────────────────────────────────────────────

// Wraps calls to the PHP middleware. The API key is sent in a request header
// here ONLY because this is a local test UI — in production it must stay server-side.
class Middleware {
  constructor(
    private readonly url: string,
    private readonly apiKey: string,
  ) {}

  async joinRoom(params: {
    code: string; name: string; role: string;
    templateId: string; sttProfile?: string; llmProfile?: string;
  }): Promise<{ session_id: string; token: string }> {
    return this.call('POST', '/rooms/join', {
      code:        params.code,
      name:        params.name,
      role:        params.role,
      template_id: params.templateId,
      stt_profile: params.sttProfile,
      llm_profile: params.llmProfile,
    });
  }

  async endSession(sessionId: string): Promise<void> {
    await this.call('POST', '/sessions/end', { session_id: sessionId });
  }

  async getMinutes(sessionId: string): Promise<PhpMinutes | null> {
    try {
      return await this.call<PhpMinutes>('GET', `/minutes?session_id=${sessionId}`);
    } catch (e) {
      if (e instanceof Error && e.message.includes('404')) return null;
      throw e;
    }
  }

  private async call<T = unknown>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await fetch(`${this.url}${path}`, {
      method,
      headers: {
        'Content-Type': 'application/json',
        // TEST ONLY: API key forwarded from browser. Remove in production.
        'X-API-Key': this.apiKey,
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    if (res.status === 204) return undefined as T;
    const data = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    if (!res.ok) throw new Error(data.error ?? `HTTP ${res.status}`);
    return data as T;
  }
}

// Minutes shape returned by the PHP middleware (camelCase from PHP DTO).
interface PhpMinutes {
  status: string;
  sections: Record<string, unknown>;
  citations: Array<{ role: string; text: string; timestampMs: number }>;
  templateId?: string;
  version: number;
  generatedAt: string;
  provider?: string;
}

// ─── App state ────────────────────────────────────────────────────────────────

let sdk:        AftertalkClient | null = null;
let middleware: Middleware       | null = null;
let templates:  Template[]             = [];
let connection: WebRTCConnection | null = null;
let sessionId = '';

// ─── Init ─────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', async () => {
  // API key from URL param (?key=...) takes priority — saves to localStorage for next visits.
  const urlKey = new URLSearchParams(window.location.search).get('token');
  if (urlKey) {
    localStorage.setItem('at_api_key', urlKey);
    // Clean the key from the URL bar without reloading.
    history.replaceState(null, '', window.location.pathname);
  }

  // Restore saved config
  el<HTMLInputElement>('apiKey').value       = ls('at_api_key');
  // Default middleware URL: same-origin path for HTTPS deployments (e.g. /aftertalk-middleware),
  // or localhost:8081 when running at the root path (local dev).
  const pagePath = window.location.pathname.replace(/\/$/, '');
  const defaultMiddleware = pagePath
    ? window.location.origin + pagePath + '-middleware'
    : 'http://localhost:8081';
  const savedMiddleware = ls('at_middleware_url');
  // Discard a saved HTTP middleware URL when the page itself is HTTPS (mixed-content block).
  const middlewareOk = savedMiddleware && !(window.location.protocol === 'https:' && savedMiddleware.startsWith('http:'));
  el<HTMLInputElement>('middlewareUrl').value = middlewareOk ? savedMiddleware : defaultMiddleware;

  el('apiKey').addEventListener('change',       onConfigChange);
  el('middlewareUrl').addEventListener('change', onConfigChange);
  el('btnJoin').addEventListener('click',        onJoin);
  el('btnMute').addEventListener('click',        onMute);
  el('btnEnd').addEventListener('click',         onEnd);

  onConfigChange();
});

function onConfigChange() {
  const apiKey        = el<HTMLInputElement>('apiKey').value.trim();
  const middlewareUrl = el<HTMLInputElement>('middlewareUrl').value.trim();

  localStorage.setItem('at_api_key',        apiKey);
  localStorage.setItem('at_middleware_url', middlewareUrl);

  // SDK client: no API key — only for public endpoints and WebRTC (JWT).
  // Derive base URL from current page to handle subpath deployments (e.g. /aftertalk/).
  const baseUrl = window.location.origin + window.location.pathname.replace(/\/[^/]*$/, '');
  sdk        = new AftertalkClient({ baseUrl });
  middleware = new Middleware(middlewareUrl, apiKey);

  loadConfig();
}

async function loadConfig() {
  if (!sdk) return;
  try {
    const cfg = await sdk.config.getConfig(); // public endpoint, no auth
    templates = cfg.templates;
    populateTemplates(cfg.templates, cfg.defaultTemplateId);
    populateProfiles(cfg.sttProfiles, cfg.llmProfiles, cfg.sttDefaultProfile, cfg.llmDefaultProfile);
    log('Config loaded');
  } catch (e) {
    log(`Config error: ${fmt(e)}`);
  }
}

// ─── UI population ────────────────────────────────────────────────────────────

function populateTemplates(tpls: Template[], defaultId?: string) {
  const sel = el<HTMLSelectElement>('templateSelect');
  sel.innerHTML = tpls.map(t =>
    `<option value="${t.id}"${t.id === defaultId ? ' selected' : ''}>${t.name}</option>`
  ).join('');
  onTemplateChange();
  sel.addEventListener('change', onTemplateChange);
}

function onTemplateChange() {
  const tmpl = templates.find(t => t.id === el<HTMLSelectElement>('templateSelect').value);
  if (!tmpl) return;
  el<HTMLSelectElement>('roleSelect').innerHTML = tmpl.roles
    .map(r => `<option value="${r.key}">${r.label}</option>`).join('');
}

function populateProfiles(
  sttProfiles?: string[], llmProfiles?: string[],
  defaultStt?: string,   defaultLlm?: string,
) {
  const hasStt = sttProfiles && sttProfiles.length > 0;
  const hasLlm = llmProfiles && llmProfiles.length > 0;
  el('profileRow').hidden = !hasStt && !hasLlm;

  if (hasStt) {
    el<HTMLSelectElement>('sttSelect').innerHTML = sttProfiles!
      .map(p => `<option value="${p}"${p === defaultStt ? ' selected' : ''}>${profileLabel(p)}</option>`).join('');
  }
  if (hasLlm) {
    el<HTMLSelectElement>('llmSelect').innerHTML = llmProfiles!
      .map(p => `<option value="${p}"${p === defaultLlm ? ' selected' : ''}>${profileLabel(p)}</option>`).join('');
  }
}

const profileLabel = (p: string) =>
  p === 'local' ? '🏠 Local' : p === 'cloud' ? '☁️ Cloud' : p;

// ─── Join room ────────────────────────────────────────────────────────────────

async function onJoin() {
  if (!sdk || !middleware) return;

  const code       = el<HTMLInputElement>('roomCode').value.trim();
  const name       = el<HTMLInputElement>('userName').value.trim();
  const role       = el<HTMLSelectElement>('roleSelect').value;
  const templateId = el<HTMLSelectElement>('templateSelect').value;
  const sttProfile = el<HTMLSelectElement>('sttSelect').value || undefined;
  const llmProfile = el<HTMLSelectElement>('llmSelect').value || undefined;

  if (!code || !name) { alert('Enter room code and name.'); return; }

  setJoinLoading(true);
  log(`Joining room "${code}" as ${role}…`);

  try {
    // ① PHP middleware creates/joins the room with the API key.
    //    The browser never touches the API key directly for this call.
    const room = await middleware.joinRoom({ code, name, role, templateId, sttProfile, llmProfile });
    sessionId = room.session_id;
    log(`Session: ${sessionId.slice(0, 8)}…`);

    showSection('sectionSession');
    el('sessionMeta').textContent = `${code} · ${role} · ${templateId}`;

    // ② SDK connects WebRTC using the JWT token — no API key needed.
    log('Connecting WebRTC…');
    connection = await sdk.connectWebRTC({ sessionId, token: room.token });

    connection.on('connected',          ()  => { setConnected(true);  log('WebRTC connected'); });
    connection.on('disconnected',       (r) => { setConnected(false); log(`Disconnected: ${r}`); });
    connection.on('ice-state-changed',  (s) => log(`ICE: ${s}`));
    connection.on('error',              (e) => log(`WebRTC error: ${(e as Error).message}`));

  } catch (e) {
    log(`Error: ${fmt(e)}`);
    setJoinLoading(false);
  }
}

// ─── End session ──────────────────────────────────────────────────────────────

async function onEnd() {
  if (!middleware || !sessionId) return;

  setConnected(false);
  connection?.disconnect();
  connection = null;

  log('Ending session…');
  el('btnEnd').setAttribute('disabled', '');
  el('btnMute').setAttribute('disabled', '');

  try {
    // PHP middleware ends the session (API key required server-side).
    await middleware.endSession(sessionId);
    log('Session ended — waiting for minutes…');
    showSection('sectionMinutes');
    el('minutesStatus').textContent = 'Generating…';
    pollMinutes();
  } catch (e) {
    log(`End error: ${fmt(e)}`);
  }
}

// ─── Mute ─────────────────────────────────────────────────────────────────────

function onMute() {
  if (!connection) return;
  const muted = !connection.muted;
  connection.setMuted(muted);
  el('btnMute').textContent = muted ? '🔇 Unmute' : '🎙️ Mute';
}

// ─── Minutes polling ─────────────────────────────────────────────────────────

async function pollMinutes() {
  if (!middleware || !sessionId) return;

  try {
    const m = await middleware.getMinutes(sessionId);
    if (!m) {
      el('minutesStatus').textContent = 'Not ready yet, retrying…';
      setTimeout(pollMinutes, 5_000);
      return;
    }

    if (m.status === 'error') {
      el('minutesStatus').textContent = '✗ Generation failed';
      return;
    }

    if (m.status !== 'ready' && m.status !== 'delivered') {
      el('minutesStatus').textContent = `Status: ${m.status}, retrying…`;
      setTimeout(pollMinutes, 5_000);
      return;
    }

    el('minutesStatus').textContent = `v${m.version} · ${m.provider ?? ''}`;
    el('minutesContent').innerHTML  = renderMinutes(m);
    log('Minutes ready!');
  } catch (e) {
    log(`Minutes error: ${fmt(e)}`);
    setTimeout(pollMinutes, 8_000);
  }
}

// ─── Minutes renderer ─────────────────────────────────────────────────────────

function renderMinutes(m: PhpMinutes): string {
  const tmpl     = templates.find(t => t.id === m.templateId);
  const sections = m.sections ?? {};
  const defs     = tmpl?.sections ?? [];

  // Render sections in template order, then any extra keys.
  const keys = [
    ...defs.map(d => d.key),
    ...Object.keys(sections).filter(k => !defs.find(d => d.key === k)),
  ];

  const labelOf = (key: string) => defs.find(d => d.key === key)?.label ?? key.replace(/_/g, ' ');
  const msToTs  = (ms: number)  => {
    const s = Math.floor(ms / 1000);
    return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`;
  };

  let html = '';

  for (const key of keys) {
    const val = sections[key];
    if (val == null) continue;
    const label = labelOf(key);

    if (Array.isArray(val) && val.length > 0) {
      html += `<div class="sec"><strong>${label}</strong><ul>`;
      for (const item of val as unknown[]) {
        html += `<li>${typeof item === 'string' ? esc(item) : esc(JSON.stringify(item))}</li>`;
      }
      html += '</ul></div>';
    } else if (val && typeof val === 'object' && ('progress' in val || 'issues' in val)) {
      const v = val as { progress?: string[]; issues?: string[] };
      html += `<div class="sec"><strong>${label}</strong><ul>`;
      for (const x of v.progress ?? []) html += `<li>✅ ${esc(x)}</li>`;
      for (const x of v.issues   ?? []) html += `<li>⚠️ ${esc(x)}</li>`;
      html += '</ul></div>';
    }
  }

  if (m.citations.length > 0) {
    const roleLabel = (key: string) => tmpl?.roles.find(r => r.key === key)?.label ?? key;
    html += `<div class="sec"><strong>💬 Citations</strong><ul>`;
    for (const c of m.citations) {
      html += `<li><code>${msToTs(c.timestampMs)}</code> <em>${esc(c.text)}</em> [${roleLabel(c.role)}]</li>`;
    }
    html += '</ul></div>';
  }

  return html || '<p class="muted">No content extracted.</p>';
}

// ─── UI helpers ───────────────────────────────────────────────────────────────

function showSection(id: string) {
  ['sectionJoin', 'sectionSession', 'sectionMinutes'].forEach(s => {
    el(s).hidden = s !== id;
  });
  if (id === 'sectionJoin') el('sectionSession').hidden = true;
}

function setJoinLoading(loading: boolean) {
  el('btnJoin').toggleAttribute('disabled', loading);
  el('btnJoin').textContent = loading ? 'Connecting…' : '🚀 Join Room';
}

function setConnected(connected: boolean) {
  el('btnEnd').toggleAttribute('disabled', !connected);
  el('btnMute').toggleAttribute('disabled', !connected);
  el('connStatus').textContent = connected ? '🟢 Connected — audio streaming' : '⚫ Disconnected';
}

function log(msg: string) {
  const time = new Date().toLocaleTimeString();
  el('log').insertAdjacentHTML('beforeend', `<div>[${time}] ${esc(msg)}</div>`);
  el('log').scrollTop = el('log').scrollHeight;
}

function fmt(e: unknown): string {
  if (e instanceof AftertalkError) return `[${e.code}] ${e.message}`;
  if (e instanceof Error) return e.message;
  return String(e);
}

function el<T extends HTMLElement = HTMLElement>(id: string): T {
  return document.getElementById(id) as T;
}

function ls(key: string): string {
  return localStorage.getItem(key) ?? '';
}

function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
