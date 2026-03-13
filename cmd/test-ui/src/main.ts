import {
  AfterthalkClient,
  AftertalkError,
  type Minutes,
  type Session,
  type Template,
  type WebRTCConnection,
} from '@aftertalk/sdk';

// ─── DOM helpers ─────────────────────────────────────────────────────────────

function el<T extends HTMLElement>(id: string): T {
  const e = document.getElementById(id);
  if (!e) throw new Error(`Element #${id} not found`);
  return e as T;
}

function show(id: string) { el(id).style.display = ''; }
function hide(id: string) { el(id).style.display = 'none'; }
function setText(id: string, text: string) { el(id).textContent = text; }
function setHtml(id: string, html: string) { el(id).innerHTML = html; }

function log(msg: string) {
  const container = el('log');
  const line = document.createElement('div');
  line.textContent = `[${new Date().toLocaleTimeString()}] ${msg}`;
  container.appendChild(line);
  container.scrollTop = container.scrollHeight;
}

// ─── App state ────────────────────────────────────────────────────────────────

let client: AfterthalkClient | null = null;
let currentSession: Session | null = null;
let currentConnection: WebRTCConnection | null = null;
let templates: Template[] = [];

// ─── Init ─────────────────────────────────────────────────────────────────────

async function init() {
  const apiKey = localStorage.getItem('aftertalk_api_key') ?? '';
  el<HTMLInputElement>('api-key').value = apiKey;

  el('api-key').addEventListener('change', (e) => {
    const key = (e.target as HTMLInputElement).value.trim();
    localStorage.setItem('aftertalk_api_key', key);
    setupClient(key);
  });

  el('connect-btn').addEventListener('click', onConnect);
  el('end-btn').addEventListener('click', onEnd);
  el('mute-btn').addEventListener('click', onMute);
  el('template-select').addEventListener('change', onTemplateChange);

  if (apiKey) setupClient(apiKey);
}

function setupClient(apiKey: string) {
  const baseUrl = window.location.origin;
  client = new AfterthalkClient({ baseUrl, apiKey });
  loadConfig();
}

async function loadConfig() {
  if (!client) return;
  try {
    const cfg = await client.config.getServerConfig();
    templates = cfg.templates;
    renderTemplateSelector(templates, cfg.defaultTemplateId);
    log(`Loaded ${templates.length} templates`);
  } catch (err) {
    log(`Config load error: ${formatError(err)}`);
  }
}

// ─── Template selector ────────────────────────────────────────────────────────

function renderTemplateSelector(tpls: Template[], defaultId: string) {
  const select = el<HTMLSelectElement>('template-select');
  select.innerHTML = tpls.map(t =>
    `<option value="${t.id}"${t.id === defaultId ? ' selected' : ''}>${t.name}</option>`
  ).join('');
  updateRoleSelectors(defaultId);
}

function onTemplateChange() {
  const templateId = el<HTMLSelectElement>('template-select').value;
  updateRoleSelectors(templateId);
}

function updateRoleSelectors(templateId: string) {
  const template = templates.find(t => t.id === templateId);
  if (!template) return;

  const roles = template.roles;
  const render = (selectId: string, excludeIdx?: number) => {
    const select = el<HTMLSelectElement>(selectId);
    select.innerHTML = roles
      .filter((_, i) => i !== excludeIdx)
      .map(r => `<option value="${r.key}">${r.label}</option>`)
      .join('');
  };

  render('role1-select');
  render('role2-select', 0);
}

// ─── Connect / End ────────────────────────────────────────────────────────────

async function onConnect() {
  if (!client) { log('Set API key first'); return; }

  const templateId = el<HTMLSelectElement>('template-select').value;
  const userId1 = el<HTMLInputElement>('user1').value.trim() || 'user-1';
  const userId2 = el<HTMLInputElement>('user2').value.trim() || 'user-2';
  const role1 = el<HTMLSelectElement>('role1-select').value;
  const role2 = el<HTMLSelectElement>('role2-select').value;

  try {
    log('Creating session...');
    currentSession = await client.sessions.create({
      participantCount: 2,
      templateId,
      participants: [
        { userId: userId1, role: role1 },
        { userId: userId2, role: role2 },
      ],
    });

    const token = currentSession.participants[0]?.token;
    if (!token) throw new Error('No participant token received');

    setText('session-id', currentSession.sessionId);
    log(`Session created: ${currentSession.sessionId}`);

    log('Connecting WebRTC...');
    currentConnection = await client.connectWebRTC({
      sessionId: currentSession.sessionId,
      token,
    });

    currentConnection.on('connected', (sid) => {
      log(`WebRTC connected (${sid})`);
      show('end-btn');
      show('mute-btn');
      hide('connect-btn');
    });

    currentConnection.on('ice-state-changed', (state) => log(`ICE: ${state}`));
    currentConnection.on('disconnected', (reason) => log(`Disconnected: ${reason}`));
    currentConnection.on('error', (err) => log(`WebRTC error: ${err.message}`));

  } catch (err) {
    log(`Connect error: ${formatError(err)}`);
  }
}

async function onEnd() {
  if (!client || !currentSession) return;

  try {
    log('Ending session...');
    await currentConnection?.disconnect();
    await client.sessions.end(currentSession.sessionId);
    log('Session ended. Waiting for minutes...');

    hide('end-btn');
    hide('mute-btn');
    show('connect-btn');
    show('minutes-section');
    setText('minutes-status', 'Generating...');

    const minutes = await client.waitForMinutes(currentSession.sessionId, {
      timeout: 120_000,
      minInterval: 3_000,
    });

    log('Minutes ready!');
    renderMinutes(minutes);

  } catch (err) {
    log(`End error: ${formatError(err)}`);
    setText('minutes-status', `Error: ${formatError(err)}`);
  }
}

function onMute() {
  if (!currentConnection) return;
  const muted = !currentConnection.muted;
  currentConnection.setMuted(muted);
  setText('mute-btn', muted ? 'Unmute' : 'Mute');
}

// ─── Minutes rendering ────────────────────────────────────────────────────────

function renderMinutes(minutes: Minutes) {
  setText('minutes-status', `v${minutes.version} — ${minutes.status}`);

  const template = templates.find(t => t.id === minutes.templateId);
  const sections = minutes.sections;

  let html = '';
  if (template) {
    for (const section of template.sections) {
      const data = sections[section.key];
      html += `<div class="section"><h3>${section.label}</h3>`;
      if (Array.isArray(data)) {
        html += '<ul>' + (data as string[]).map(item => `<li>${escHtml(String(item))}</li>`).join('') + '</ul>';
      } else if (data !== null && data !== undefined) {
        html += `<p>${escHtml(String(data))}</p>`;
      }
      html += '</div>';
    }
  } else {
    html = `<pre>${escHtml(JSON.stringify(sections, null, 2))}</pre>`;
  }

  if (minutes.citations.length > 0) {
    html += '<div class="section"><h3>Citations</h3><ul>';
    for (const c of minutes.citations) {
      const ts = formatMs(c.timestampMs);
      html += `<li><em>${escHtml(c.role)}</em> [${ts}]: ${escHtml(c.text)}</li>`;
    }
    html += '</ul></div>';
  }

  setHtml('minutes-content', html);
}

// ─── Utils ────────────────────────────────────────────────────────────────────

function formatError(err: unknown): string {
  if (err instanceof AftertalkError) return `[${err.code}] ${err.message}`;
  if (err instanceof Error) return err.message;
  return String(err);
}

function escHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function formatMs(ms: number): string {
  const s = Math.floor(ms / 1000);
  const m = Math.floor(s / 60);
  return `${String(m).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`;
}

// ─── Boot ─────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', init);
