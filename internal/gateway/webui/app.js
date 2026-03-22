'use strict';

/**
 * SoulGate Web UI
 * Vanilla JS — no framework, no build step, no CDN dependencies.
 * Works offline/airgapped.  Embeddable via Go embed.FS.
 */

// ── State ──────────────────────────────────────────────────────────────────

const state = {
  view: 'chat',
  connected: false,
  health: null,
  status: null,
  sessions: [],
  chatHistory: [],    // { role, content, time, tokens }
  isSending: false,
  streamingEl: null,  // current assistant bubble being streamed
};

// ── Utilities ──────────────────────────────────────────────────────────────

function esc(text) {
  const d = document.createElement('div');
  d.appendChild(document.createTextNode(String(text ?? '')));
  return d.innerHTML;
}

function $(id) { return document.getElementById(id); }

function q(sel, root) { return (root || document).querySelector(sel); }

function formatTime(iso) {
  if (!iso) return '';
  return new Date(iso).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

function relTime(iso) {
  if (!iso) return '';
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 5000)    return 'just now';
  if (diff < 60000)   return `${Math.floor(diff / 1000)}s ago`;
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  return `${Math.floor(diff / 3600000)}h ago`;
}

function formatUptime(seconds) {
  if (seconds == null) return '--';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function formatBytes(mb) {
  if (mb == null) return '--';
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}

function shortID(id, len) {
  len = len || 12;
  if (!id) return '--';
  return id.length > len ? id.slice(0, len) + '…' : id;
}

// ── Markdown renderer ──────────────────────────────────────────────────────

function renderMarkdown(text) {
  if (!text) return '';

  const lines = text.split('\n');
  const out = [];
  let i = 0;
  let inList = null;     // 'ul' or 'ol'
  let listBuf = [];

  function flushList() {
    if (listBuf.length > 0) {
      out.push(`<${inList}>${listBuf.join('')}</${inList}>`);
      listBuf = [];
      inList = null;
    }
  }

  while (i < lines.length) {
    let line = lines[i];

    // Fenced code block
    if (/^```/.test(line)) {
      flushList();
      const lang = line.slice(3).trim().toLowerCase();
      const codeLines = [];
      i++;
      while (i < lines.length && !/^```/.test(lines[i])) {
        codeLines.push(lines[i]);
        i++;
      }
      i++; // skip closing ```
      const rawCode = codeLines.join('\n');
      const highlighted = syntaxHighlight(rawCode, lang);
      const langTag = lang ? `<span class="code-lang">${esc(lang)}</span>` : '';
      // Detect diagram/flowchart blocks (contain arrows → ↓ ← ↑ or box-drawing chars)
      const isDiagram = !lang && /[→←↑↓│├└┌┐┘┤┬┴┼╭╮╰╯─═║▼▲◆●]/.test(rawCode);
      const preClass = isDiagram ? ' class="diagram"' : '';
      out.push(`<pre${preClass}>${langTag}<code>${highlighted}</code></pre>`);
      continue;
    }

    // ATX headings
    const hm = line.match(/^(#{1,3}) (.+)/);
    if (hm) {
      flushList();
      const level = hm[1].length;
      out.push(`<h${level}>${inlineMarkdown(hm[2])}</h${level}>`);
      i++; continue;
    }

    // Markdown table (header row with | separators, followed by separator row)
    if (/^\|(.+)\|$/.test(line) && i + 1 < lines.length && /^\|[-:\s|]+\|$/.test(lines[i + 1])) {
      flushList();
      // Parse header
      const headers = line.split('|').slice(1, -1).map(c => c.trim());
      i += 2; // skip header + separator
      const rows = [];
      while (i < lines.length && /^\|(.+)\|$/.test(lines[i])) {
        rows.push(lines[i].split('|').slice(1, -1).map(c => c.trim()));
        i++;
      }
      let tableHTML = '<table><thead><tr>';
      for (const h of headers) tableHTML += `<th>${inlineMarkdown(h)}</th>`;
      tableHTML += '</tr></thead><tbody>';
      for (const row of rows) {
        tableHTML += '<tr>';
        for (const cell of row) tableHTML += `<td>${inlineMarkdown(cell)}</td>`;
        tableHTML += '</tr>';
      }
      tableHTML += '</tbody></table>';
      out.push(tableHTML);
      continue;
    }

    // Horizontal rule
    if (/^(-{3,}|\*{3,}|_{3,})$/.test(line.trim())) {
      flushList();
      out.push('<hr>');
      i++; continue;
    }

    // Blockquote
    const bq = line.match(/^> (.+)/);
    if (bq) {
      flushList();
      out.push(`<blockquote>${inlineMarkdown(bq[1])}</blockquote>`);
      i++; continue;
    }

    // Unordered list
    const ul = line.match(/^[ \t]*[-*+] (.+)/);
    if (ul) {
      if (inList === 'ol') flushList();
      inList = 'ul';
      listBuf.push(`<li>${inlineMarkdown(ul[1])}</li>`);
      i++; continue;
    }

    // Ordered list
    const ol = line.match(/^[ \t]*\d+\. (.+)/);
    if (ol) {
      if (inList === 'ul') flushList();
      inList = 'ol';
      listBuf.push(`<li>${inlineMarkdown(ol[1])}</li>`);
      i++; continue;
    }

    flushList();

    // Empty line → paragraph break
    if (line.trim() === '') {
      out.push('<br>');
      i++; continue;
    }

    out.push(`<p>${inlineMarkdown(line)}</p>`);
    i++;
  }

  flushList();
  return out.join('');
}

function inlineMarkdown(text) {
  // Escape HTML first, then apply inline patterns to escaped result
  // Actually we need to handle code first before escaping
  // Split on inline code to preserve it
  const parts = text.split(/(`[^`]+`)/g);
  return parts.map((part, idx) => {
    if (idx % 2 === 1) {
      // inline code
      const code = part.slice(1, -1);
      return `<code>${esc(code)}</code>`;
    }
    let s = esc(part);
    // Bold+italic ***
    s = s.replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>');
    // Bold **
    s = s.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    // Italic *  (not adjacent to spaces)
    s = s.replace(/(?<!\s)\*(?!\s)(.+?)(?<!\s)\*(?!\s)/g, '<em>$1</em>');
    // Strikethrough ~~
    s = s.replace(/~~(.+?)~~/g, '<del>$1</del>');
    // Links [text](url)
    s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
    return s;
  }).join('');
}

// ── Syntax highlighter (CSS token classes, no library) ────────────────────

function syntaxHighlight(code, lang) {
  const escaped = esc(code);
  if (!lang || !['go', 'js', 'javascript', 'ts', 'typescript', 'python', 'py', 'bash', 'sh', 'json', 'yaml', 'yml'].includes(lang)) {
    return escaped;
  }

  if (lang === 'json') return highlightJSON(escaped);
  if (lang === 'yaml' || lang === 'yml') return highlightYAML(escaped);
  if (lang === 'bash' || lang === 'sh') return highlightBash(escaped);
  return highlightGeneric(escaped, lang);
}

function highlightJSON(code) {
  return code
    .replace(/("(?:[^"\\]|\\.)*")\s*:/g, '<span class="tok-function">$1</span>:')
    .replace(/:(\s*)("(?:[^"\\]|\\.)*")/g, ':$1<span class="tok-string">$2</span>')
    .replace(/\b(true|false|null)\b/g, '<span class="tok-keyword">$1</span>')
    .replace(/\b(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\b/g, '<span class="tok-number">$1</span>');
}

function highlightYAML(code) {
  return code
    .replace(/^([ \t]*)([\w-]+):/gm, '$1<span class="tok-function">$2</span>:')
    .replace(/:\s*("(?:[^"\\]|\\.)*")/g, ': <span class="tok-string">$1</span>')
    .replace(/:\s*('(?:[^'\\]|\\.)*')/g, ': <span class="tok-string">$1</span>')
    .replace(/\b(true|false|null)\b/g, '<span class="tok-keyword">$1</span>')
    .replace(/(#.*)$/gm, '<span class="tok-comment">$1</span>');
}

function highlightBash(code) {
  return code
    .replace(/(#[^!].*)$/gm, '<span class="tok-comment">$1</span>')
    .replace(/\b(if|then|else|elif|fi|for|while|do|done|case|esac|function|return|export|local|readonly|declare)\b/g, '<span class="tok-keyword">$1</span>')
    .replace(/("(?:[^"\\]|\\.)*")/g, '<span class="tok-string">$1</span>')
    .replace(/('(?:[^'\\]|\\.)*')/g, '<span class="tok-string">$1</span>')
    .replace(/\b(\d+)\b/g, '<span class="tok-number">$1</span>');
}

function highlightGeneric(code, lang) {
  const goKw = 'package|import|func|var|const|type|struct|interface|map|chan|go|defer|return|if|else|for|range|switch|case|default|select|break|continue|fallthrough|goto|make|new|append|len|cap|close|delete|copy|panic|recover|nil|true|false|iota';
  const jsKw = 'function|var|let|const|return|if|else|for|while|do|switch|case|default|break|continue|new|this|typeof|instanceof|in|of|try|catch|finally|throw|async|await|class|extends|import|export|from|null|undefined|true|false';
  const pyKw = 'def|class|return|if|elif|else|for|while|import|from|as|with|try|except|finally|raise|pass|break|continue|lambda|yield|async|await|True|False|None';

  const kwMap = {
    go: goKw, js: jsKw, javascript: jsKw, ts: jsKw, typescript: jsKw,
    python: pyKw, py: pyKw,
  };

  const kw = kwMap[lang] || goKw;

  return code
    .replace(new RegExp(`\\b(${kw})\\b`, 'g'), '<span class="tok-keyword">$1</span>')
    .replace(/(\/\/[^\n]*)/g, '<span class="tok-comment">$1</span>')
    .replace(/(\/\*[\s\S]*?\*\/)/g, '<span class="tok-comment">$1</span>')
    .replace(/(#[^\n]*)/g, '<span class="tok-comment">$1</span>')
    .replace(/("(?:[^"\\]|\\.)*")/g, '<span class="tok-string">$1</span>')
    .replace(/('(?:[^'\\]|\\.)*')/g, '<span class="tok-string">$1</span>')
    .replace(/(`(?:[^`\\]|\\.)*`)/g, '<span class="tok-string">$1</span>')
    .replace(/\b(\d+(?:\.\d+)?)\b/g, '<span class="tok-number">$1</span>');
}

// ── API client ─────────────────────────────────────────────────────────────

const api = {
  async get(path) {
    const res = await fetch(path, { signal: AbortSignal.timeout(5000) });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  },

  async health() {
    return this.get('/api/health');
  },

  async status() {
    return this.get('/api/status');
  },

  async sessions() {
    return this.get('/api/sessions');
  },

  async chat(message) {
    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      throw new Error(err.error || `HTTP ${res.status}`);
    }
    return res.json();
  },

  async chatStream(message, onChunk, onDone, onError) {
    // Try streaming first; fall back to plain JSON if endpoint doesn't stream.
    let res;
    try {
      res = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, stream: true }),
      });
    } catch (err) {
      onError(err.message);
      return;
    }

    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      onError(err.error || `HTTP ${res.status}`);
      return;
    }

    const contentType = res.headers.get('content-type') || '';

    // SSE / streaming
    if (contentType.includes('text/event-stream') || contentType.includes('application/x-ndjson')) {
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const parts = buf.split('\n');
        buf = parts.pop();
        for (const part of parts) {
          const line = part.trim();
          if (!line || line === ':') continue;
          const data = line.startsWith('data: ') ? line.slice(6) : line;
          if (data === '[DONE]') { onDone(); return; }
          try {
            const obj = JSON.parse(data);
            const chunk = obj.delta || obj.content || obj.text || obj.response || '';
            if (chunk) onChunk(chunk);
          } catch (_) {
            if (data) onChunk(data);
          }
        }
      }
      onDone();
    } else {
      // Plain JSON response (current gateway behaviour)
      const data = await res.json();
      const full = data.response || '(empty response)';
      // Simulate streaming for a nicer UX: character-by-character reveal
      const chunkSize = 4;
      for (let i = 0; i < full.length; i += chunkSize) {
        onChunk(full.slice(i, i + chunkSize));
        await sleep(8);
      }
      onDone(data.tokens);
    }
  },
};

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

// ── Connection polling ─────────────────────────────────────────────────────

async function refresh() {
  try {
    const [health, sessions] = await Promise.allSettled([
      api.health(),
      api.sessions(),
    ]);

    if (health.status === 'fulfilled') {
      state.health = health.value;
      state.connected = true;
      setConnectionStatus('online', health.value.status);
    } else {
      state.connected = false;
      setConnectionStatus('offline', 'offline');
    }

    if (sessions.status === 'fulfilled') {
      state.sessions = sessions.value.sessions || [];
    }

    // Also fetch status for provider/model info
    try {
      state.status = await api.status();
    } catch (_) {}

    if (state.view === 'dashboard') renderDashboard();
    if (state.view === 'connectors') renderConnectors();
    if (state.view === 'settings') renderSettings();
  } catch (_) {
    setConnectionStatus('offline', 'offline');
  }
}

function setConnectionStatus(cls, label) {
  const dot = $('statusDot');
  const lbl = $('statusLabel');
  if (!dot || !lbl) return;
  dot.className = `status-dot ${cls}`;
  dot.title = label;
  lbl.textContent = label === 'healthy' ? 'Online' : label === 'degraded' ? 'Degraded' : label === 'offline' ? 'Offline' : label;
}

// ── Router ─────────────────────────────────────────────────────────────────

function navigate(viewName) {
  if (state.view === viewName) return;
  state.view = viewName;

  // Update tab indicators
  document.querySelectorAll('.nav-tab').forEach(btn => {
    const active = btn.dataset.view === viewName;
    btn.classList.toggle('active', active);
    btn.setAttribute('aria-selected', String(active));
  });

  // Swap views
  document.querySelectorAll('.view').forEach(el => {
    el.classList.toggle('active', el.id === `view-${viewName}`);
  });

  // Render the incoming view
  switch (viewName) {
    case 'chat':       break; // persistent
    case 'dashboard':  renderDashboard(); break;
    case 'connectors': renderConnectors(); break;
    case 'settings':   renderSettings(); break;
  }
}

// ── Chat view ──────────────────────────────────────────────────────────────

function buildChatView() {
  return `
    <div class="view active chat-view" id="view-chat">
      <div class="chat-messages" id="chatMessages">
        <div class="msg msg-system">
          <div class="msg-bubble">Connected to SoulGate. Type a message to start.</div>
        </div>
        <div class="typing-indicator" id="typingIndicator">
          <div class="typing-dot"></div>
          <div class="typing-dot"></div>
          <div class="typing-dot"></div>
        </div>
      </div>
      <div class="chat-input-area">
        <div class="chat-input-row">
          <textarea
            class="chat-input"
            id="chatInput"
            rows="1"
            placeholder="Message SoulGate… (Enter to send, Shift+Enter for newline)"
            aria-label="Chat input"
          ></textarea>
          <button class="send-btn" id="sendBtn" title="Send (Enter)" aria-label="Send">
            <svg viewBox="0 0 20 20" fill="none">
              <path d="M4 10H16M10 4l6 6-6 6" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>
        </div>
        <div class="chat-hint">Enter to send &nbsp;·&nbsp; Shift+Enter for newline</div>
      </div>
    </div>`;
}

function buildOtherViews() {
  return `
    <div class="view" id="view-dashboard"></div>
    <div class="view" id="view-connectors"></div>
    <div class="view" id="view-settings"></div>
  `;
}

function appendMessage(role, contentHTML, time, tokens) {
  const container = $('chatMessages');
  if (!container) return null;

  const typingEl = $('typingIndicator');
  const el = document.createElement('div');
  el.className = `msg msg-${role}`;

  const timeStr = time || formatTime(new Date().toISOString());
  const label = role === 'user' ? 'You' : role === 'assistant' ? 'SoulGate' : '';

  if (role === 'system') {
    el.innerHTML = `<div class="msg-bubble">${contentHTML}</div>`;
  } else {
    const metaTokens = tokens ? `<span class="msg-meta-tokens">${tokens} tokens</span>` : '';
    el.innerHTML = `
      <div class="msg-meta">${esc(label)} <span style="opacity:0.5">·</span> ${esc(timeStr)}${metaTokens}</div>
      <div class="msg-bubble">${contentHTML}</div>`;
  }

  // Insert before typing indicator
  if (typingEl) {
    container.insertBefore(el, typingEl);
  } else {
    container.appendChild(el);
  }

  scrollChatToBottom();
  return el;
}

function appendStreamingMessage() {
  const container = $('chatMessages');
  if (!container) return null;

  const typingEl = $('typingIndicator');
  const el = document.createElement('div');
  el.className = 'msg msg-assistant';
  el.innerHTML = `
    <div class="msg-meta">SoulGate <span style="opacity:0.5">·</span> ${esc(formatTime(new Date().toISOString()))}</div>
    <div class="msg-bubble"><span class="stream-cursor"></span></div>`;

  if (typingEl) {
    container.insertBefore(el, typingEl);
  } else {
    container.appendChild(el);
  }
  scrollChatToBottom();
  return el;
}

let streamBuffer = '';

function appendChunk(el, chunk) {
  if (!el) return;
  streamBuffer += chunk;
  const bubble = el.querySelector('.msg-bubble');
  if (!bubble) return;
  bubble.innerHTML = renderMarkdown(streamBuffer) + '<span class="stream-cursor"></span>';
  scrollChatToBottom();
}

function finalizeStreaming(el, tokens) {
  if (!el) return;
  const bubble = el.querySelector('.msg-bubble');
  if (!bubble) return;
  bubble.innerHTML = renderMarkdown(streamBuffer);
  if (tokens) {
    const meta = el.querySelector('.msg-meta');
    if (meta) {
      meta.insertAdjacentHTML('beforeend', `<span class="msg-meta-tokens">${tokens} tokens</span>`);
    }
  }
  streamBuffer = '';
  state.streamingEl = null;
  bindToolCards(bubble);
}

function bindToolCards(root) {
  if (!root) return;
  root.querySelectorAll('.tool-card-header').forEach(header => {
    header.addEventListener('click', () => {
      header.closest('.tool-card').classList.toggle('open');
    });
  });
}

function scrollChatToBottom() {
  const el = $('chatMessages');
  if (el) el.scrollTop = el.scrollHeight;
}

function setTyping(visible) {
  const el = $('typingIndicator');
  if (el) el.classList.toggle('visible', visible);
  scrollChatToBottom();
}

function autoResize(textarea) {
  textarea.style.height = '';
  textarea.style.height = Math.min(textarea.scrollHeight, 160) + 'px';
}

async function handleSend() {
  const input = $('chatInput');
  const btn = $('sendBtn');
  if (!input || state.isSending) return;

  const text = input.value.trim();
  if (!text) return;

  input.value = '';
  autoResize(input);
  state.isSending = true;
  if (btn) btn.disabled = true;

  appendMessage('user', esc(text).replace(/\n/g, '<br>'));

  // Start streaming assistant bubble
  streamBuffer = '';
  state.streamingEl = appendStreamingMessage();
  setTyping(false);

  await api.chatStream(
    text,
    (chunk) => appendChunk(state.streamingEl, chunk),
    (tokens) => {
      finalizeStreaming(state.streamingEl, tokens);
      state.isSending = false;
      if (btn) btn.disabled = false;
      if (input) input.focus();
      setTimeout(refresh, 800);
    },
    (errMsg) => {
      finalizeStreaming(state.streamingEl, null);
      appendMessage('system', `Error: ${esc(errMsg)}`);
      state.isSending = false;
      if (btn) btn.disabled = false;
      if (input) input.focus();
    }
  );
}

function initChat() {
  const input = $('chatInput');
  const btn = $('sendBtn');

  if (input) {
    input.addEventListener('keydown', e => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    });
    input.addEventListener('input', () => autoResize(input));
    input.focus();
  }

  if (btn) {
    btn.addEventListener('click', handleSend);
  }
}

// ── Dashboard view ─────────────────────────────────────────────────────────

function renderDashboard() {
  const el = $('view-dashboard');
  if (!el) return;

  const s = state.status || {};
  const h = state.health || {};
  const mem = h.memory || {};
  const checks = h.checks || [];

  const totalClients = (s.agents || 0) + (s.channels || 0) + (s.uis || 0) + (s.nodes || 0);
  const uptimeStr = formatUptime(s.uptime_seconds);

  // Build memory bars
  const maxMB = Math.max(mem.sys_mb || 1, 1);
  function bar(val, max) {
    const pct = Math.min(100, Math.round((val / max) * 100));
    const cls = pct > 80 ? 'crit' : pct > 60 ? 'warn' : '';
    return `<div class="memory-track"><div class="memory-fill ${cls}" style="width:${pct}%"></div></div>`;
  }

  // Sessions
  const sessionRows = state.sessions.length === 0
    ? `<div class="empty-state"><div class="empty-icon">○</div><div class="empty-title">No active sessions</div></div>`
    : state.sessions.map(sess => {
        const st = (sess.state || 'active').toLowerCase();
        const stBadge = st === 'active' ? 'badge-success' : st === 'idle' ? 'badge-warning' : 'badge-muted';
        return `
          <div class="session-item">
            <div class="session-top">
              <span class="session-id" title="${esc(sess.id)}">${esc(shortID(sess.id, 14))}</span>
              <span class="badge ${stBadge}">${esc(st)}</span>
            </div>
            <div class="session-meta">
              <span>${esc(sess.channel || '--')}</span>
              <span>${sess.message_count ?? 0} msgs</span>
            </div>
          </div>`;
      }).join('');

  // Health checks
  const checkRows = checks.length === 0
    ? `<div class="empty-state"><div class="empty-title">No checks available</div></div>`
    : checks.map(c => `
        <div class="check-item">
          <div class="check-dot ${c.status}"></div>
          <div class="check-name">${esc(c.name.replace(/_/g, ' '))}</div>
          <div class="check-detail" title="${esc(c.detail)}">${esc(c.detail || '')}</div>
        </div>`).join('');

  el.innerHTML = `
    <div class="dashboard-view">

      <div>
        <div class="section-header">
          <div class="section-title">System Overview</div>
          <span class="badge ${h.status === 'healthy' ? 'badge-success' : h.status === 'degraded' ? 'badge-warning' : 'badge-error'}">${esc(h.status || '--')}</span>
        </div>
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-label">Provider</div>
            <div class="stat-value" style="font-size:18px">${esc(s.provider || '--')}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">Model</div>
            <div class="stat-value c-accent" style="font-size:13px;padding-top:6px">${esc(s.model || '--')}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">Uptime</div>
            <div class="stat-value c-success">${esc(uptimeStr)}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">Clients</div>
            <div class="stat-value c-info">${totalClients}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">Sessions</div>
            <div class="stat-value">${s.sessions ?? '--'}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">Goroutines</div>
            <div class="stat-value ${(mem.goroutines || 0) > 500 ? 'c-warning' : ''}">${mem.goroutines ?? '--'}</div>
          </div>
        </div>
      </div>

      <div class="dashboard-cols">
        <div class="dashboard-col">
          <div class="panel-header">
            <div class="panel-title">Active Sessions</div>
            <span class="badge badge-muted">${state.sessions.length}</span>
          </div>
          <div class="panel-body">${sessionRows}</div>
        </div>
        <div class="dashboard-col">
          <div class="panel-header">
            <div class="panel-title">Health Checks</div>
          </div>
          <div class="panel-body">${checkRows}</div>
        </div>
      </div>

      <div>
        <div class="section-header">
          <div class="section-title">Memory Usage</div>
          <span class="badge badge-muted">${esc(h.uptime || '--')}</span>
        </div>
        <div class="card" style="padding:12px 14px;gap:8px;display:flex;flex-direction:column;">
          <div class="memory-row">
            <div class="memory-label">Allocated</div>
            ${bar(mem.alloc_mb, maxMB)}
            <div class="memory-val">${formatBytes(mem.alloc_mb)}</div>
          </div>
          <div class="memory-row">
            <div class="memory-label">System</div>
            ${bar(mem.sys_mb, maxMB)}
            <div class="memory-val">${formatBytes(mem.sys_mb)}</div>
          </div>
          <div class="memory-row">
            <div class="memory-label">Total Alloc</div>
            ${bar(mem.total_alloc_mb, maxMB)}
            <div class="memory-val">${formatBytes(mem.total_alloc_mb)}</div>
          </div>
        </div>
      </div>

    </div>`;
}

// ── Connectors view ────────────────────────────────────────────────────────

function renderConnectors() {
  const el = $('view-connectors');
  if (!el) return;

  const s = state.status || {};

  const roles = [
    { key: 'agent_clients',   role: 'agent',   label: 'Agent',   icon: '🤖', cls: 'icon-agent' },
    { key: 'channel_clients', role: 'channel', label: 'Channel', icon: '📡', cls: 'icon-channel' },
    { key: 'ui_clients',      role: 'ui',      label: 'UI',      icon: '🖥',  cls: 'icon-ui' },
    { key: 'node_clients',    role: 'node',    label: 'Node',    icon: '⬡',  cls: 'icon-node' },
  ];

  const allClients = [];
  for (const r of roles) {
    const map = s[r.key] || {};
    Object.keys(map).forEach(id => {
      allClients.push({ id, ...r });
    });
  }

  const roleCount = (role) => (s[role + '_clients'] ? Object.keys(s[role + '_clients']).length : 0);

  const summaryCards = roles.map(r => {
    const cnt = roleCount(r.role);
    const active = cnt > 0;
    return `
      <div class="stat-card" style="display:flex;align-items:center;gap:12px;padding:14px 16px;">
        <div class="connector-icon ${r.cls}" style="width:36px;height:36px;">${r.icon}</div>
        <div>
          <div class="stat-label">${r.label}s</div>
          <div class="stat-value ${active ? 'c-success' : ''}" style="font-size:22px">${cnt}</div>
        </div>
        <div style="margin-left:auto">
          <span class="badge ${active ? 'badge-success' : 'badge-muted'}">${active ? 'connected' : 'none'}</span>
        </div>
      </div>`;
  }).join('');

  const cards = allClients.length === 0
    ? `<div class="empty-state" style="grid-column:1/-1">
         <div class="empty-icon">◌</div>
         <div class="empty-title">No connected clients</div>
         <div class="empty-desc">Connect an agent, channel, or UI via WebSocket at <code>ws://localhost:${s.port || 8080}/ws</code></div>
       </div>`
    : allClients.map(c => `
        <div class="connector-card">
          <div class="connector-header">
            <div class="connector-icon ${c.cls}">${c.icon}</div>
            <div class="connector-info">
              <div class="connector-id" title="${esc(c.id)}">${esc(shortID(c.id, 18))}</div>
              <div class="connector-role">${esc(c.label)}</div>
            </div>
            <span class="badge badge-success">live</span>
          </div>
          <div class="connector-row">
            <span class="connector-row-key">Role</span>
            <span class="connector-row-val">${esc(c.role)}</span>
          </div>
          <div class="connector-row">
            <span class="connector-row-key">Full ID</span>
            <span class="connector-row-val" style="font-size:10px">${esc(c.id)}</span>
          </div>
        </div>`).join('');

  el.innerHTML = `
    <div class="connectors-view">
      <div>
        <div class="section-header">
          <div class="section-title">Connected Clients</div>
          <span class="badge badge-muted">${allClients.length} total</span>
        </div>
        <div class="stats-grid">${summaryCards}</div>
      </div>

      <div>
        <div class="section-header">
          <div class="section-title">Client Details</div>
        </div>
        <div class="connector-grid">${cards}</div>
      </div>

      <div class="card" style="background:var(--bg-secondary);">
        <div class="label" style="margin-bottom:10px">WebSocket Endpoint</div>
        <code style="font-family:var(--font-mono);font-size:12px;color:var(--accent-hover);">
          ws://localhost:${esc(s.port || 8080)}/ws
        </code>
        <div style="margin-top:8px;font-size:12px;color:var(--text-muted);">
          Send a <code>connect</code> frame with <code>role</code> set to agent, channel, ui, or node.
        </div>
      </div>
    </div>`;
}

// ── Settings view ──────────────────────────────────────────────────────────

function renderSettings() {
  const el = $('view-settings');
  if (!el) return;

  const s = state.status || {};
  const h = state.health || {};

  function row(key, val, muted) {
    return `
      <div class="settings-row">
        <span class="settings-key">${esc(key)}</span>
        <span class="settings-val ${muted ? 'muted' : ''}">${esc(val || '--')}</span>
      </div>`;
  }

  el.innerHTML = `
    <div class="settings-view">
      <div class="settings-section">
        <div class="settings-section-header">Provider Configuration</div>
        ${row('Provider', s.provider)}
        ${row('Model', s.model)}
        ${row('Port', s.port)}
        ${row('Status', h.status)}
        ${row('Started At', s.started_at ? new Date(s.started_at).toLocaleString() : '--')}
        ${row('Uptime', h.uptime)}
      </div>

      <div class="settings-section">
        <div class="settings-section-header">Connected Clients</div>
        ${row('Agents', s.agents ?? 0)}
        ${row('Channels', s.channels ?? 0)}
        ${row('UIs', s.uis ?? 0)}
        ${row('Nodes', s.nodes ?? 0)}
        ${row('Active Sessions', s.sessions ?? 0)}
      </div>

      <div class="settings-section">
        <div class="settings-section-header">Runtime</div>
        ${row('Goroutines', h.memory ? h.memory.goroutines : '--')}
        ${row('Allocated Memory', h.memory ? formatBytes(h.memory.alloc_mb) : '--')}
        ${row('System Memory', h.memory ? formatBytes(h.memory.sys_mb) : '--')}
        ${row('GC Cycles', h.memory ? h.memory.num_gc : '--')}
      </div>

      <div class="settings-section">
        <div class="settings-section-header">API Endpoints</div>
        ${row('Health', '/api/health')}
        ${row('Status', '/api/status')}
        ${row('Sessions', '/api/sessions')}
        ${row('Chat', 'POST /api/chat')}
        ${row('WebSocket', '/ws')}
      </div>

      <div class="settings-section">
        <div class="settings-section-header">Documentation</div>
        <div class="settings-row">
          <span class="settings-key">GitHub</span>
          <a class="settings-link" href="https://github.com/M4MEET/soulgate" target="_blank" rel="noopener">github.com/M4MEET/soulgate</a>
        </div>
        <div class="settings-row">
          <span class="settings-key">Config</span>
          <span class="settings-val muted">.soulgate/config.yml</span>
        </div>
        <div class="settings-row">
          <span class="settings-key">Policy</span>
          <span class="settings-val muted">.soulgate/policy.yml</span>
        </div>
      </div>
    </div>`;
}

// ── Bootstrap ──────────────────────────────────────────────────────────────

function init() {
  const app = $('app');
  if (!app) return;

  // Inject view containers
  app.innerHTML = buildChatView() + buildOtherViews();

  // Wire up tab navigation
  document.querySelectorAll('.nav-tab').forEach(btn => {
    btn.addEventListener('click', () => navigate(btn.dataset.view));
  });

  // Wire up chat
  initChat();

  // Initial data load
  refresh();

  // Poll every 5 seconds
  setInterval(refresh, 5000);
}

document.addEventListener('DOMContentLoaded', init);
