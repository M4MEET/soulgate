import { useState, useEffect, useRef } from 'react';
import { Search, RefreshCw, Play, Square, Activity } from 'lucide-react';
import { fetchAuditEvents, type AuditEvent } from '../lib/api';
import { formatRelativeTime, truncate } from '../lib/utils';
import toast from 'react-hot-toast';

const TYPE_COLORS: Record<string, string> = {
  file_read:       'text-sky-400 bg-sky-500/10',
  file_write:      'text-amber-400 bg-amber-500/10',
  tool_call:       'text-violet-400 bg-violet-500/10',
  policy_decision: 'text-orange-400 bg-orange-500/10',
  session_start:   'text-emerald-400 bg-emerald-500/10',
  session_end:     'text-zinc-400 bg-zinc-700/40',
  error:           'text-red-400 bg-red-500/10',
  auth:            'text-pink-400 bg-pink-500/10',
  default:         'text-zinc-400 bg-zinc-700/40',
};

const STATUS_COLORS: Record<string, string> = {
  success: 'text-emerald-400',
  denied:  'text-red-400',
  error:   'text-red-400',
  pending: 'text-amber-400',
  default: 'text-zinc-500',
};

const DEMO_EVENTS: AuditEvent[] = [
  { id: '1', type: 'session_start',   category: 'session', status: 'success', metadata: { channel: 'web' },           created_at: new Date(Date.now() - 10000).toISOString() },
  { id: '2', type: 'file_read',       category: 'file',    status: 'success', metadata: { path: './README.md' },      created_at: new Date(Date.now() - 9000).toISOString() },
  { id: '3', type: 'policy_decision', category: 'policy',  status: 'success', metadata: { action: 'files.read', decision: 'allow' }, created_at: new Date(Date.now() - 8000).toISOString() },
  { id: '4', type: 'tool_call',       category: 'tool',    status: 'success', metadata: { tool: 'files.list', duration: '12ms' },     created_at: new Date(Date.now() - 6000).toISOString() },
  { id: '5', type: 'file_write',      category: 'file',    status: 'denied',  metadata: { path: '../etc/passwd' },    created_at: new Date(Date.now() - 4000).toISOString(), session_id: 'abc123' },
  { id: '6', type: 'policy_decision', category: 'policy',  status: 'denied',  metadata: { action: 'files.write', decision: 'deny', reason: 'path traversal' }, created_at: new Date(Date.now() - 3900).toISOString() },
  { id: '7', type: 'error',           category: 'system',  status: 'error',   metadata: { message: 'context deadline exceeded' }, created_at: new Date(Date.now() - 1000).toISOString() },
];

const EVENT_TYPES = ['all', 'file_read', 'file_write', 'tool_call', 'policy_decision', 'session_start', 'session_end', 'error', 'auth'];
const CATEGORIES = ['all', 'session', 'file', 'policy', 'tool', 'system', 'auth'];
const STATUSES = ['all', 'success', 'denied', 'error', 'pending'];

export default function AuditView() {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [catFilter, setCatFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [limit, setLimit] = useState(50);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = async () => {
    try {
      const data = await fetchAuditEvents(limit);
      setEvents(data.length > 0 ? data : DEMO_EVENTS);
    } catch {
      toast.error('Failed to load audit events');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [limit]);

  useEffect(() => {
    if (autoRefresh) {
      intervalRef.current = setInterval(load, 3000);
    } else {
      if (intervalRef.current) clearInterval(intervalRef.current);
    }
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [autoRefresh, limit]);

  const filtered = events.filter(e => {
    if (typeFilter !== 'all' && e.type !== typeFilter) return false;
    if (catFilter !== 'all' && e.category !== catFilter) return false;
    if (statusFilter !== 'all' && e.status !== statusFilter) return false;
    if (query) {
      const q = query.toLowerCase();
      return (
        e.type.includes(q) ||
        e.category.includes(q) ||
        (e.session_id || '').includes(q) ||
        JSON.stringify(e.metadata).toLowerCase().includes(q)
      );
    }
    return true;
  });

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Audit Log</h2>
          <p className="text-sm text-zinc-500">{events.length} events · showing {filtered.length}</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => { setAutoRefresh(a => !a); toast(autoRefresh ? 'Auto-refresh off' : 'Auto-refresh on', 'info' as never); }}
            className={`flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm transition-all ${
              autoRefresh
                ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                : 'border border-zinc-700 text-zinc-400 hover:text-zinc-200 hover:border-zinc-600'
            }`}
          >
            {autoRefresh ? <Square size={13} /> : <Play size={13} />}
            {autoRefresh ? 'Live' : 'Auto-refresh'}
          </button>
          <button
            onClick={() => { setLoading(true); load(); }}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            Refresh
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="space-y-3 mb-5">
        <div className="relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search events…"
            className="w-full pl-9 pr-3 py-2.5 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
          />
        </div>

        <div className="flex flex-wrap gap-3">
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Type:</span>
            <div className="flex gap-1 flex-wrap">
              {EVENT_TYPES.map(t => (
                <button
                  key={t}
                  onClick={() => setTypeFilter(t)}
                  className={`px-2 py-1 rounded text-xs transition-all ${
                    typeFilter === t ? 'bg-indigo-500/15 text-indigo-400' : 'text-zinc-600 hover:text-zinc-400'
                  }`}
                >
                  {t}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-4">
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Category:</span>
            <select
              value={catFilter}
              onChange={e => setCatFilter(e.target.value)}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60"
            >
              {CATEGORIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Status:</span>
            <select
              value={statusFilter}
              onChange={e => setStatusFilter(e.target.value)}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60"
            >
              {STATUSES.map(s => <option key={s} value={s}>{s}</option>)}
            </select>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Limit:</span>
            <select
              value={limit}
              onChange={e => setLimit(Number(e.target.value))}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60"
            >
              {[25, 50, 100, 500].map(n => <option key={n} value={n}>{n}</option>)}
            </select>
          </div>
        </div>
      </div>

      {/* Events table */}
      <div className="rounded-xl border border-zinc-700/40 overflow-hidden">
        {loading ? (
          <div className="flex items-center gap-2 text-zinc-500 p-6"><Activity size={16} className="animate-spin" />Loading…</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-700/50 bg-zinc-800/60">
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Time</th>
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Type</th>
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Category</th>
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Status</th>
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Session</th>
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Details</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr><td colSpan={6} className="text-center text-zinc-500 text-sm py-8">No events match filters</td></tr>
              ) : (
                filtered.map(event => {
                  const typeColor = TYPE_COLORS[event.type] || TYPE_COLORS.default;
                  const statusColor = STATUS_COLORS[event.status] || STATUS_COLORS.default;
                  return (
                    <tr key={event.id} className="border-b border-zinc-800/40 hover:bg-zinc-800/20 transition-colors">
                      <td className="px-4 py-2.5 text-xs text-zinc-600 whitespace-nowrap">{formatRelativeTime(event.created_at)}</td>
                      <td className="px-4 py-2.5">
                        <span className={`text-xs px-2 py-0.5 rounded-full ${typeColor}`}>{event.type}</span>
                      </td>
                      <td className="px-4 py-2.5 text-xs text-zinc-500">{event.category}</td>
                      <td className={`px-4 py-2.5 text-xs font-medium ${statusColor}`}>{event.status}</td>
                      <td className="px-4 py-2.5">
                        {event.session_id && (
                          <span className="font-mono text-xs text-zinc-600">{event.session_id.slice(0, 8)}…</span>
                        )}
                      </td>
                      <td className="px-4 py-2.5 text-xs text-zinc-500 font-mono max-w-xs truncate">
                        {truncate(JSON.stringify(event.metadata), 80)}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
