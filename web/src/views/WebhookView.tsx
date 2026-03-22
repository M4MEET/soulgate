import { useState, useEffect, useCallback } from 'react';
import { Webhook, Play, Clock, CheckCircle, XCircle, RefreshCw, ChevronDown, ChevronRight } from 'lucide-react';
import { fetchConfig, type ConfigData } from '../lib/api';
import toast from 'react-hot-toast';

// ── Types ────────────────────────────────────────────────────────────────────

interface WebhookConfig {
  name: string;
  url: string;
  events: string[];
}

interface TestResult {
  id: string;
  webhookName: string;
  timestamp: Date;
  status: number | null;
  body: string;
  durationMs: number;
  error?: string;
}

type PayloadFormat = 'json' | 'github-push' | 'github-pr' | 'gitlab-push' | 'plain';

// ── Pre-built payloads ────────────────────────────────────────────────────────

const PRESET_PAYLOADS: Record<PayloadFormat, { label: string; payload: string }> = {
  json: {
    label: 'Custom JSON',
    payload: JSON.stringify({ event: 'test', message: 'Hello from SoulGate', timestamp: new Date().toISOString() }, null, 2),
  },
  'github-push': {
    label: 'GitHub Push',
    payload: JSON.stringify({
      ref: 'refs/heads/main',
      before: '0000000000000000000000000000000000000000',
      after: 'abc123def456',
      repository: { id: 12345, name: 'my-repo', full_name: 'user/my-repo', private: false },
      pusher: { name: 'octocat', email: 'octocat@github.com' },
      commits: [{ id: 'abc123def456', message: 'Fix bug in webhook handler', author: { name: 'Octocat' }, added: [], removed: [], modified: ['src/webhook.go'] }],
    }, null, 2),
  },
  'github-pr': {
    label: 'GitHub PR Opened',
    payload: JSON.stringify({
      action: 'opened',
      number: 42,
      pull_request: {
        id: 999,
        number: 42,
        title: 'Add webhook testing feature',
        state: 'open',
        body: 'This PR adds a webhook tester to the UI.',
        user: { login: 'octocat' },
        head: { ref: 'feature/webhook-tester', sha: 'abc123' },
        base: { ref: 'main', sha: 'def456' },
      },
      repository: { name: 'my-repo', full_name: 'user/my-repo' },
    }, null, 2),
  },
  'gitlab-push': {
    label: 'GitLab Pipeline',
    payload: JSON.stringify({
      object_kind: 'pipeline',
      object_attributes: { id: 101, ref: 'main', sha: 'abc123', status: 'success', duration: 42, stages: ['build', 'test', 'deploy'] },
      project: { id: 1, name: 'my-project', web_url: 'https://gitlab.com/user/my-project' },
      user: { name: 'GitLab User', username: 'user' },
      commit: { id: 'abc123', message: 'Deploy webhook tester', author: { name: 'GitLab User' } },
    }, null, 2),
  },
  'plain': {
    label: 'Plain Text',
    payload: 'Hello from webhook tester!',
  },
};

// ── Sub-components ────────────────────────────────────────────────────────────

function ResultBadge({ status, error }: { status: number | null; error?: string }) {
  if (error) {
    return (
      <span className="flex items-center gap-1 text-xs font-mono text-red-400">
        <XCircle size={12} /> ERR
      </span>
    );
  }
  if (status === null) return null;
  const ok = status >= 200 && status < 300;
  return (
    <span className={`flex items-center gap-1 text-xs font-mono ${ok ? 'text-emerald-400' : 'text-red-400'}`}>
      {ok ? <CheckCircle size={12} /> : <XCircle size={12} />}
      {status}
    </span>
  );
}

function HistoryEntry({ result }: { result: TestResult }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="rounded-lg border border-zinc-800/60 overflow-hidden">
      <button
        onClick={() => setOpen(o => !o)}
        className="flex items-center gap-3 w-full px-3 py-2 text-left hover:bg-zinc-800/40 transition-colors"
      >
        {open ? <ChevronDown size={13} className="text-zinc-600" /> : <ChevronRight size={13} className="text-zinc-600" />}
        <span className="text-xs text-zinc-500 font-mono flex-shrink-0">
          {result.timestamp.toLocaleTimeString()}
        </span>
        <span className="text-xs text-zinc-400 flex-1 truncate">{result.webhookName}</span>
        <span className="text-xs text-zinc-600 font-mono mr-2">{result.durationMs}ms</span>
        <ResultBadge status={result.status} error={result.error} />
      </button>
      {open && (
        <div className="px-3 pb-3 border-t border-zinc-800/60 pt-2">
          {result.error ? (
            <pre className="text-xs text-red-400 font-mono whitespace-pre-wrap break-all">{result.error}</pre>
          ) : (
            <pre className="text-xs text-zinc-400 font-mono whitespace-pre-wrap break-all max-h-48 overflow-y-auto" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
              {result.body || '(empty response)'}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

function WebhookTestPanel({ webhook }: { webhook: WebhookConfig }) {
  const [format, setFormat] = useState<PayloadFormat>('json');
  const [payload, setPayload] = useState(PRESET_PAYLOADS['json'].payload);
  const [sending, setSending] = useState(false);
  const [history, setHistory] = useState<TestResult[]>([]);

  const loadPreset = (f: PayloadFormat) => {
    setFormat(f);
    setPayload(PRESET_PAYLOADS[f].payload);
  };

  const handleSend = async () => {
    setSending(true);
    const start = performance.now();
    const id = `${Date.now()}-${Math.random()}`;

    try {
      const contentType = format === 'plain' ? 'text/plain' : 'application/json';
      const res = await fetch(`/webhook/${encodeURIComponent(webhook.name)}`, {
        method: 'POST',
        headers: { 'Content-Type': contentType },
        body: payload,
      });

      const durationMs = Math.round(performance.now() - start);
      let body = '';
      try {
        const text = await res.text();
        // Try to pretty-print if JSON
        try {
          body = JSON.stringify(JSON.parse(text), null, 2);
        } catch {
          body = text;
        }
      } catch { /* ignore */ }

      const result: TestResult = {
        id,
        webhookName: webhook.name,
        timestamp: new Date(),
        status: res.status,
        body,
        durationMs,
      };
      setHistory(h => [result, ...h].slice(0, 10));

      if (res.ok) {
        toast.success(`Webhook responded ${res.status}`);
      } else {
        toast.error(`Webhook returned ${res.status}`);
      }
    } catch (err: unknown) {
      const durationMs = Math.round(performance.now() - start);
      const error = (err as Error).message || String(err);
      const result: TestResult = {
        id,
        webhookName: webhook.name,
        timestamp: new Date(),
        status: null,
        body: '',
        durationMs,
        error,
      };
      setHistory(h => [result, ...h].slice(0, 10));
      toast.error(`Request failed: ${error}`);
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Endpoint info */}
      <div className="flex items-center gap-2 p-3 rounded-lg bg-zinc-900/60 border border-zinc-800/60">
        <span className="text-xs font-mono text-zinc-600 flex-shrink-0">POST</span>
        <code className="text-xs font-mono text-indigo-300 flex-1 break-all">
          /webhook/{webhook.name}
        </code>
        {webhook.url && (
          <span className="text-xs text-zinc-600 truncate max-w-48" title={webhook.url}>
            → {webhook.url}
          </span>
        )}
      </div>

      {/* Format selector */}
      <div>
        <p className="text-xs text-zinc-500 mb-2">Pre-built payloads</p>
        <div className="flex flex-wrap gap-2">
          {(Object.keys(PRESET_PAYLOADS) as PayloadFormat[]).map(f => (
            <button
              key={f}
              onClick={() => loadPreset(f)}
              className={`px-3 py-1 rounded-full text-xs transition-all border ${
                format === f
                  ? 'bg-indigo-500/20 border-indigo-500/50 text-indigo-300'
                  : 'bg-zinc-800/60 border-zinc-700/40 text-zinc-400 hover:text-zinc-200 hover:border-zinc-600'
              }`}
            >
              {PRESET_PAYLOADS[f].label}
            </button>
          ))}
        </div>
      </div>

      {/* Payload editor */}
      <div>
        <p className="text-xs text-zinc-500 mb-2">Payload</p>
        <textarea
          value={payload}
          onChange={e => setPayload(e.target.value)}
          rows={10}
          spellCheck={false}
          className="w-full px-3 py-2.5 rounded-lg bg-zinc-900/60 border border-zinc-700/40 text-xs font-mono text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/50 resize-y"
          style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
        />
      </div>

      {/* Send button */}
      <button
        onClick={handleSend}
        disabled={sending}
        className="flex items-center gap-2 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-60 text-white text-sm transition-colors"
      >
        {sending ? <RefreshCw size={14} className="animate-spin" /> : <Play size={14} />}
        {sending ? 'Sending…' : 'Send Test'}
      </button>

      {/* Request history */}
      {history.length > 0 && (
        <div>
          <p className="text-xs text-zinc-500 mb-2 flex items-center gap-1.5">
            <Clock size={12} />
            Request history ({history.length})
          </p>
          <div className="space-y-1.5">
            {history.map(r => <HistoryEntry key={r.id} result={r} />)}
          </div>
        </div>
      )}
    </div>
  );
}

// ── Main view ─────────────────────────────────────────────────────────────────

export default function WebhookView() {
  const [webhooks, setWebhooks] = useState<WebhookConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const config: ConfigData | null = await fetchConfig();
      const whs = config?.webhooks || [];
      setWebhooks(whs);
      // Auto-expand the first webhook
      if (whs.length > 0 && expanded === null) {
        setExpanded(whs[0].name);
      }
    } finally {
      setLoading(false);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { load(); }, [load]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-500 gap-2">
        <RefreshCw size={16} className="animate-spin" />
        Loading webhooks…
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Webhook Tester</h2>
          <p className="text-sm text-zinc-500">Send test requests to configured webhooks</p>
        </div>
        <button
          onClick={load}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 border border-zinc-700/40 transition-all"
        >
          <RefreshCw size={13} />
          Reload
        </button>
      </div>

      {webhooks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 gap-4">
          <div className="w-14 h-14 rounded-xl bg-zinc-800/60 flex items-center justify-center">
            <Webhook size={28} className="text-zinc-600" />
          </div>
          <div className="text-center">
            <p className="text-zinc-400 font-medium">No webhooks configured</p>
            <p className="text-zinc-600 text-sm mt-1">
              Add webhooks in{' '}
              <span className="text-indigo-400">Settings → Webhooks</span>
              {' '}to test them here.
            </p>
          </div>
        </div>
      ) : (
        <div className="space-y-4 max-w-3xl">
          {webhooks.map(wh => {
            const isExpanded = expanded === wh.name;
            return (
              <div
                key={wh.name}
                className="rounded-xl border border-zinc-700/40 bg-zinc-800/30 overflow-hidden"
              >
                {/* Webhook header */}
                <button
                  onClick={() => setExpanded(isExpanded ? null : wh.name)}
                  className="flex items-center gap-3 w-full px-5 py-3.5 text-left hover:bg-zinc-800/40 transition-colors border-b border-zinc-700/40 bg-zinc-800/60"
                >
                  <Webhook size={15} className="text-zinc-400 flex-shrink-0" />
                  <span className="text-sm font-semibold text-zinc-300 flex-1">{wh.name || '(unnamed)'}</span>
                  {wh.events.length > 0 && (
                    <div className="flex gap-1 mr-2">
                      {wh.events.slice(0, 3).map(ev => (
                        <span key={ev} className="px-1.5 py-0.5 rounded text-xs bg-zinc-700/60 text-zinc-400 font-mono">{ev}</span>
                      ))}
                      {wh.events.length > 3 && (
                        <span className="px-1.5 py-0.5 rounded text-xs bg-zinc-700/60 text-zinc-500">+{wh.events.length - 3}</span>
                      )}
                    </div>
                  )}
                  {isExpanded
                    ? <ChevronDown size={14} className="text-zinc-500 flex-shrink-0" />
                    : <ChevronRight size={14} className="text-zinc-500 flex-shrink-0" />
                  }
                </button>

                {/* Test panel */}
                {isExpanded && (
                  <div className="p-5">
                    <WebhookTestPanel webhook={wh} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
