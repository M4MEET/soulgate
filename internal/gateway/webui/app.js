/**
 * SoulGate Web Dashboard
 *
 * Vanilla JS — no framework dependencies.
 * Polls /api/status and /api/sessions every 5 seconds.
 * Sends chat via POST /api/chat.
 */

'use strict';

// ─── Utility ──────────────────────────────────────────────────────────────────

const $ = id => document.getElementById(id);

function escapeHtml(text) {
  const d = document.createElement('div');
  d.appendChild(document.createTextNode(text));
  return d.innerHTML;
}

/**
 * Minimal markdown renderer: bold, inline code, fenced code blocks, and lists.
 * Does not rely on any external library.
 */
function renderMarkdown(text) {
  if (!text) return '';

  // Fenced code blocks  ```lang\n...\n```
  text = text.replace(/```(\w*)\n([\s\S]*?)```/g, (_, lang, code) => {
    const escaped = escapeHtml(code.trimEnd());
    const langAttr = lang ? ` data-lang="${escapeHtml(lang)}"` : '';
    return `<pre${langAttr}><code>${escaped}</code></pre>`;
  });

  // Inline code `...`
  text = text.replace(/`([^`\n]+)`/g, (_, code) => `<code>${escapeHtml(code)}</code>`);

  // Bold **...**
  text = text.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');

  // Unordered list items  - item  or  * item
  text = text.replace(/^[ \t]*[-*] (.+)$/gm, '<li>$1</li>');
  text = text.replace(/(<li>.*<\/li>(\n|$))+/g, match => `<ul>${match}</ul>`);

  // Ordered list items  1. item
  text = text.replace(/^[ \t]*\d+\. (.+)$/gm, '<li>$1</li>');

  // Paragraphs — double newline becomes <br><br>
  text = text.replace(/\n{2,}/g, '<br><br>');

  // Single newlines
  text = text.replace(/\n/g, '<br>');

  return text;
}

function formatTime(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function relativeTime(iso) {
  if (!iso) return '';
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 60000)  return `${Math.floor(diff / 1000)}s ago`;
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  return `${Math.floor(diff / 3600000)}h ago`;
}

function formatUptime(seconds) {
  if (seconds == null) return '--';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

// ─── State ───────────────────────────────────────────────────────────────────

const state = {
  connected: false,
  status: null,      // last /api/status response
  sessions: [],      // last /api/sessions response
  auditEvents: [],   // last /api/audit response (may be empty if endpoint absent)
  startedAt: null,
};

// ─── DOM refs ────────────────────────────────────────────────────────────────

const refs = {
  statusDot:      $('statusDot'),
  statusLabel:    $('statusLabel'),
  uptimeDisplay:  $('uptimeDisplay'),
  refreshPulse:   $('refreshPulse'),

  // Stat cards
  statClients:    $('statClients'),
  statSessions:   $('statSessions'),
  statAgents:     $('statAgents'),
  statChannels:   $('statChannels'),

  // Provider info
  providerName:   $('providerName'),
  providerModel:  $('providerModel'),
  providerPort:   $('providerPort'),

  // Client list
  clientList:     $('clientList'),

  // Chat
  chatMessages:   $('chatMessages'),
  typingIndicator: $('typingIndicator'),
  chatInput:      $('chatInput'),
  sendBtn:        $('sendBtn'),

  // Aside panels
  activityList:   $('activityList'),
  activityBadge:  $('activityBadge'),
  sessionList:    $('sessionList'),
  sessionBadge:   $('sessionBadge'),
};

// ─── Connectivity check ───────────────────────────────────────────────────────

function setConnected(online) {
  state.connected = online;
  refs.statusDot.className   = `status-dot ${online ? 'online' : 'offline'}`;
  refs.statusLabel.textContent = online ? 'Online' : 'Offline';
}

// ─── Status refresh ───────────────────────────────────────────────────────────

async function fetchStatus() {
  try {
    const res = await fetch('/api/status', { signal: AbortSignal.timeout(4000) });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    state.status = await res.json();
    setConnected(true);
    applyStatus(state.status);
  } catch {
    setConnected(false);
  }
}

async function fetchSessions() {
  try {
    const res = await fetch('/api/sessions', { signal: AbortSignal.timeout(4000) });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    state.sessions = Array.isArray(data.sessions) ? data.sessions : [];
    renderSessions(state.sessions);
  } catch {
    // non-fatal — sessions panel will just show stale data
  }
}

async function fetchAudit() {
  try {
    const res = await fetch('/api/audit?limit=20', { signal: AbortSignal.timeout(4000) });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    state.auditEvents = Array.isArray(data.events) ? data.events : [];
    renderActivity(state.auditEvents);
  } catch {
    // /api/audit is optional — silently skip
  }
}

function pulseRefresh() {
  refs.refreshPulse.classList.add('active');
  setTimeout(() => refs.refreshPulse.classList.remove('active'), 600);
}

async function refresh() {
  pulseRefresh();
  await Promise.allSettled([fetchStatus(), fetchSessions(), fetchAudit()]);
}

function applyStatus(s) {
  // Counts
  const totalClients = (s.agents || 0) + (s.channels || 0) + (s.uis || 0) + (s.nodes || 0);
  refs.statClients.textContent  = totalClients;
  refs.statSessions.textContent = s.sessions ?? '--';
  refs.statAgents.textContent   = s.agents   ?? '--';
  refs.statChannels.textContent = s.channels  ?? '--';

  // Provider
  refs.providerName.textContent  = s.provider  || '--';
  refs.providerModel.textContent = s.model     || '--';
  refs.providerPort.textContent  = s.port      || window.location.port || '8080';

  // Uptime
  refs.uptimeDisplay.textContent = formatUptime(s.uptime_seconds);

  // Connected clients breakdown
  renderClientList(s);
}

function renderClientList(s) {
  const items = [];

  function makeClients(map, role) {
    if (!map) return;
    Object.entries(map).forEach(([id, meta]) => {
      items.push({ id, role, meta: meta || {} });
    });
  }

  makeClients(s.agent_clients,   'agent');
  makeClients(s.channel_clients, 'channel');
  makeClients(s.ui_clients,      'ui');
  makeClients(s.node_clients,    'node');

  if (items.length === 0) {
    refs.clientList.innerHTML = '<div class="empty-state">No connected clients</div>';
    return;
  }

  refs.clientList.innerHTML = items.map(({ id, role }) => {
    const shortID = id.length > 12 ? id.slice(0, 8) + '…' : id;
    return `
      <div class="client-item">
        <span class="client-role-badge role-${role}">${escapeHtml(role)}</span>
        <span class="client-id" title="${escapeHtml(id)}">${escapeHtml(shortID)}</span>
      </div>`;
  }).join('');
}

// ─── Sessions ─────────────────────────────────────────────────────────────────

function renderSessions(sessions) {
  refs.sessionBadge.textContent = sessions.length;

  if (sessions.length === 0) {
    refs.sessionList.innerHTML = '<div class="empty-state">No active sessions</div>';
    return;
  }

  refs.sessionList.innerHTML = sessions.map(s => {
    const stateClass = `state-${(s.state || 'active').toLowerCase()}`;
    const shortID = s.id ? s.id.slice(0, 12) + '…' : '--';
    return `
      <div class="session-item">
        <div class="session-top">
          <span class="session-id" title="${escapeHtml(s.id || '')}">${escapeHtml(shortID)}</span>
          <span class="session-state ${stateClass}">${escapeHtml(s.state || 'active')}</span>
        </div>
        <div class="session-meta">
          <span class="session-channel">${escapeHtml(s.channel || '--')}</span>
          <span class="session-msgs">${s.message_count ?? 0} msgs</span>
        </div>
      </div>`;
  }).join('');
}

// ─── Activity ─────────────────────────────────────────────────────────────────

function renderActivity(events) {
  refs.activityBadge.textContent = events.length;

  if (events.length === 0) {
    refs.activityList.innerHTML = '<div class="empty-state">No recent activity</div>';
    return;
  }

  refs.activityList.innerHTML = events.map(ev => {
    let typeClass = 'type-default';
    if (ev.status === 'success') typeClass = 'type-success';
    else if (ev.status === 'error') typeClass = 'type-error';
    else if (ev.status === 'denied') typeClass = 'type-denied';

    const resource = ev.resource ? `<div class="activity-resource">${escapeHtml(ev.resource)}</div>` : '';

    return `
      <div class="activity-item">
        <div class="activity-top">
          <span class="activity-type ${typeClass}">${escapeHtml(ev.status || 'info')}</span>
          <span class="activity-event">${escapeHtml(ev.type || ev.category || '--')}</span>
          <span class="activity-time">${relativeTime(ev.timestamp)}</span>
        </div>
        ${resource}
      </div>`;
  }).join('');
}

// ─── Chat ─────────────────────────────────────────────────────────────────────

function addSystemMessage(text) {
  const el = document.createElement('div');
  el.className = 'message system-message';
  el.innerHTML = `<div class="message-bubble">${escapeHtml(text)}</div>`;
  refs.chatMessages.appendChild(el);
  scrollToBottom();
}

function addMessage(role, content, time) {
  const el = document.createElement('div');
  el.className = `message ${role === 'user' ? 'user-message' : 'assistant-message'}`;

  const label  = role === 'user' ? 'You' : 'SoulGate';
  const timeStr = time || new Date().toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  const html   = role === 'assistant' ? renderMarkdown(content) : escapeHtml(content);

  el.innerHTML = `
    <div class="message-meta">${escapeHtml(label)} · ${timeStr}</div>
    <div class="message-bubble">${html}</div>`;

  refs.chatMessages.appendChild(el);
  scrollToBottom();
}

function scrollToBottom() {
  refs.chatMessages.scrollTop = refs.chatMessages.scrollHeight;
}

function setTyping(visible) {
  refs.typingIndicator.classList.toggle('visible', visible);
  scrollToBottom();
}

async function sendChat() {
  const message = refs.chatInput.value.trim();
  if (!message) return;

  refs.chatInput.value = '';
  refs.chatInput.style.height = '';
  refs.sendBtn.disabled = true;

  addMessage('user', message);
  setTyping(true);

  try {
    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message }),
    });

    setTyping(false);

    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      addSystemMessage(`Error: ${err.error || 'Unknown error'}`);
      return;
    }

    const data = await res.json();
    addMessage('assistant', data.response || '(empty response)');

    // Refresh status after a chat — session count may have changed
    setTimeout(refresh, 500);

  } catch (err) {
    setTyping(false);
    addSystemMessage(`Network error: ${err.message}`);
  } finally {
    refs.sendBtn.disabled = false;
    refs.chatInput.focus();
  }
}

// Auto-resize textarea
function autoResize(el) {
  el.style.height = '';
  el.style.height = Math.min(el.scrollHeight, 120) + 'px';
}

// ─── Init ────────────────────────────────────────────────────────────────────

function init() {
  // Chat events
  refs.sendBtn.addEventListener('click', sendChat);
  refs.chatInput.addEventListener('keydown', e => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendChat();
    }
  });
  refs.chatInput.addEventListener('input', () => autoResize(refs.chatInput));

  // Show /api/chat availability message
  addSystemMessage('Connected to SoulGate gateway. Type a message to interact with the AI.');

  // Initial load
  refresh();

  // Poll every 5 seconds
  setInterval(refresh, 5000);

  // Focus input
  refs.chatInput.focus();
}

document.addEventListener('DOMContentLoaded', init);
