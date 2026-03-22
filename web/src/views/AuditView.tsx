import { useState, useEffect, useRef } from 'react';
import { Search, RefreshCw, Play, Square, Activity } from 'lucide-react';
import { fetchAuditEvents, type AuditEvent } from '../lib/api';
import { formatRelativeTime, truncate } from '../lib/utils';
import toast from 'react-hot-toast';

// The API returns event types with dots (e.g. "session.start", "file.read").
// Map them to display-friendly colors.
const TYPE_COLORS: Record<string, string> = {
  'session.start':   'text-emerald-400 bg-emerald-500/10',
  'session.end':     'text-zinc-400 bg-zinc-700/40',
  'run.start':       'text-blue-400 bg-blue-500/10',
  'run.complete':    'text-blue-400 bg-blue-500/10',
  'run.error':       'text-red-400 bg-red-500/10',
  'model.call':      'text-violet-400 bg-violet-500/10',
  'model.response':  'text-violet-400 bg-violet-500/10',
  'model.error':     'text-red-400 bg-red-500/10',
  'tool.execute':    'text-amber-400 bg-amber-500/10',
  'tool.success':    'text-emerald-400 bg-emerald-500/10',
  'tool.error':      'text-red-400 bg-red-500/10',
  'policy.evaluate': 'text-orange-400 bg-orange-500/10',
  'policy.allow':    'text-emerald-400 bg-emerald-500/10',
  'policy.deny':     'text-red-400 bg-red-500/10',
  'file.read':       'text-sky-400 bg-sky-500/10',
  'file.write':      'text-amber-400 bg-amber-500/10',
  'file.list':       'text-sky-400 bg-sky-500/10',
  'file.delete':     'text-red-400 bg-red-500/10',
  'exec.command':    'text-amber-400 bg-amber-500/10',
  'net.request':     'text-cyan-400 bg-cyan-500/10',
};

const STATUS_COLORS: Record<string, string> = {
  success: 'text-emerald-400',
  denied:  'text-red-400',
  error:   'text-red-400',
  pending: 'text-amber-400',
};

function getTypeColor(type: string): string {
  return TYPE_COLORS[type] || 'text-zinc-400 bg-zinc-700/40';
}

function getStatusColor(status: string): string {
  return STATUS_COLORS[status] || 'text-zinc-500';
}

// Format metadata into a readable string
function formatMetadata(meta: Record<string, unknown> | null | undefined): string {
  if (!meta || typeof meta !== 'object') return '';
  const entries = Object.entries(meta);
  if (entries.length === 0) return '';
  // Show key=value pairs, truncated
  return entries
    .map(([k, v]) => `${k}=${typeof v === 'string' ? v : JSON.stringify(v)}`)
    .join(' ')
    .slice(0, 120);
}

// Get the timestamp from an audit event — API may use "timestamp" or "created_at"
function getEventTime(event: AuditEvent): string {
  return (event as unknown as Record<string, string>).timestamp || event.created_at || '';
}

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
      setEvents(data);
    } catch {
      toast.error('Failed to load audit events');
      setEvents([]);
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

  // Collect unique types from actual data for filter buttons
  const uniqueTypes = ['all', ...Array.from(new Set(events.map(e => e.type).filter(Boolean)))];
  const uniqueCategories = ['all', ...Array.from(new Set(events.map(e => e.category).filter(Boolean)))];

  const filtered = events.filter(e => {
    if (typeFilter !== 'all' && e.type !== typeFilter) return false;
    if (catFilter !== 'all' && e.category !== catFilter) return false;
    if (statusFilter !== 'all' && e.status !== statusFilter) return false;
    if (query) {
      const q = query.toLowerCase();
      return (
        (e.type || '').toLowerCase().includes(q) ||
        (e.category || '').toLowerCase().includes(q) ||
        (e.session_id || '').toLowerCase().includes(q) ||
        (e.run_id || '').toLowerCase().includes(q) ||
        formatMetadata(e.metadata).toLowerCase().includes(q)
      );
    }
    return true;
  });

  const toggleAutoRefresh = () => {
    setAutoRefresh(prev => {
      toast.success(!prev ? 'Auto-refresh enabled' : 'Auto-refresh disabled');
      return !prev;
    });
  };

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
            onClick={toggleAutoRefresh}
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
            placeholder="Search events..."
            className="w-full pl-9 pr-3 py-2.5 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
          />
        </div>

        <div className="flex flex-wrap gap-3">
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Type:</span>
            <div className="flex gap-1 flex-wrap">
              {uniqueTypes.slice(0, 12).map(t => (
                <button
                  key={t}
                  onClick={() => setTypeFilter(t)}
                  className={`px-2 py-1 rounded text-xs transition-all ${
                    typeFilter === t ? 'bg-indigo-500/15 text-indigo-400' : 'text-zinc-600 hover:text-zinc-400'
                  }`}
                >
                  {t === 'all' ? 'all' : t.replace(/\./g, ' ')}
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
              {uniqueCategories.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Status:</span>
            <select
              value={statusFilter}
              onChange={e => setStatusFilter(e.target.value)}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60"
            >
              {['all', 'success', 'denied', 'error'].map(s => <option key={s} value={s}>{s}</option>)}
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
          <div className="flex items-center gap-2 text-zinc-500 p-6"><Activity size={16} className="animate-spin" /> Loading...</div>
        ) : filtered.length === 0 ? (
          <div className="text-center text-zinc-500 text-sm py-12">
            {events.length === 0 ? 'No audit events yet. Events appear as you use SoulGate.' : 'No events match filters.'}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-zinc-700/50 bg-zinc-800/60">
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-24">Time</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Type</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-20">Category</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-20">Status</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-24">Session</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Details</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((event, idx) => (
                  <tr key={event.id || idx} className="border-b border-zinc-800/40 hover:bg-zinc-800/20 transition-colors">
                    <td className="px-4 py-2.5 text-xs text-zinc-600 whitespace-nowrap">
                      {getEventTime(event) ? formatRelativeTime(getEventTime(event)) : '—'}
                    </td>
                    <td className="px-4 py-2.5">
                      <span className={`text-xs px-2 py-0.5 rounded-full ${getTypeColor(event.type)}`}>
                        {(event.type || '').replace(/\./g, ' ')}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-xs text-zinc-500">{event.category || '—'}</td>
                    <td className={`px-4 py-2.5 text-xs font-medium ${getStatusColor(event.status)}`}>
                      {event.status || '—'}
                    </td>
                    <td className="px-4 py-2.5">
                      {event.session_id ? (
                        <span className="font-mono text-xs text-zinc-600">{event.session_id.slice(0, 12)}...</span>
                      ) : (
                        <span className="text-xs text-zinc-700">—</span>
                      )}
                    </td>
                    <td className="px-4 py-2.5 text-xs text-zinc-500 font-mono max-w-xs">
                      <span title={JSON.stringify(event.metadata)}>
                        {truncate(formatMetadata(event.metadata), 80) || '—'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
