import { useState, useEffect, useRef } from 'react';
import { Search, RefreshCw, Play, Square, Activity, X, Copy, Check, Clock, Tag, Layers, User, Hash, Download, Calendar } from 'lucide-react';
import { fetchAuditEvents, type AuditEvent } from '../lib/api';
import { truncate } from '../lib/utils';
import toast from 'react-hot-toast';

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

function formatMetadata(meta: Record<string, unknown> | null | undefined): string {
  if (!meta || typeof meta !== 'object') return '';
  const entries = Object.entries(meta);
  if (entries.length === 0) return '';
  return entries
    .map(([k, v]) => `${k}=${typeof v === 'string' ? v : JSON.stringify(v)}`)
    .join(' ')
    .slice(0, 120);
}

function getEventTime(event: AuditEvent): string {
  return (event as unknown as Record<string, string>).timestamp || event.created_at || '';
}

function EventDetailPanel({ event, onClose }: { event: AuditEvent; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const time = (event as unknown as Record<string, string>).timestamp || event.created_at || '';
  const meta = event.metadata || {};
  const metaEntries = Object.entries(meta).filter(([, v]) => v !== null && v !== undefined);

  const copyJson = () => {
    navigator.clipboard.writeText(JSON.stringify(event, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm flex justify-end" onClick={onClose}>
      <div
        className="w-full max-w-xl bg-zinc-900 border-l border-zinc-800 flex flex-col h-full overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800 flex-shrink-0">
          <div className="flex items-center gap-3">
            <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${getTypeColor(event.type)}`}>
              <Activity size={15} />
            </div>
            <div>
              <div className="font-semibold text-zinc-100 text-sm">{(event.type || '').replace(/\./g, ' ')}</div>
              <div className="text-xs text-zinc-500">{event.category}</div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={copyJson}
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-xs text-zinc-400 hover:text-zinc-200 transition-all"
            >
              {copied ? <Check size={12} /> : <Copy size={12} />}
              {copied ? 'Copied' : 'Copy JSON'}
            </button>
            <button onClick={onClose} className="text-zinc-600 hover:text-zinc-300 transition-colors text-lg">
              <X size={18} />
            </button>
          </div>
        </div>

        <div className="px-6 py-4 border-b border-zinc-800/50 grid grid-cols-2 gap-3 flex-shrink-0">
          <div className="flex items-center gap-2">
            <Tag size={12} className="text-zinc-600" />
            <span className="text-xs text-zinc-500">Type</span>
            <span className={`text-xs px-2 py-0.5 rounded-full ml-auto ${getTypeColor(event.type)}`}>{event.type}</span>
          </div>
          <div className="flex items-center gap-2">
            <Layers size={12} className="text-zinc-600" />
            <span className="text-xs text-zinc-500">Category</span>
            <span className="text-xs text-zinc-300 ml-auto">{event.category || '---'}</span>
          </div>
          <div className="flex items-center gap-2">
            <Activity size={12} className="text-zinc-600" />
            <span className="text-xs text-zinc-500">Status</span>
            <span className={`text-xs font-medium ml-auto ${getStatusColor(event.status)}`}>{event.status}</span>
          </div>
          <div className="flex items-center gap-2">
            <Clock size={12} className="text-zinc-600" />
            <span className="text-xs text-zinc-500">Time</span>
            <span className="text-xs text-zinc-300 ml-auto">{time ? new Date(time).toLocaleString() : '---'}</span>
          </div>
          {event.session_id && (
            <div className="flex items-center gap-2 col-span-2">
              <User size={12} className="text-zinc-600" />
              <span className="text-xs text-zinc-500">Session</span>
              <span className="text-xs text-indigo-400 font-mono ml-auto">{event.session_id}</span>
            </div>
          )}
          {event.run_id && (
            <div className="flex items-center gap-2 col-span-2">
              <Hash size={12} className="text-zinc-600" />
              <span className="text-xs text-zinc-500">Run</span>
              <span className="text-xs text-zinc-400 font-mono ml-auto">{event.run_id}</span>
            </div>
          )}
          <div className="flex items-center gap-2 col-span-2">
            <Hash size={12} className="text-zinc-600" />
            <span className="text-xs text-zinc-500">Event ID</span>
            <span className="text-xs text-zinc-500 font-mono ml-auto">{event.id}</span>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
          <h4 className="text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-3">Metadata</h4>
          {metaEntries.length === 0 ? (
            <p className="text-sm text-zinc-600">No metadata</p>
          ) : (
            <div className="space-y-2">
              {metaEntries.map(([key, value]) => (
                <div key={key} className="rounded-lg bg-zinc-800/40 border border-zinc-700/40 overflow-hidden">
                  <div className="flex items-center justify-between px-3 py-2 bg-zinc-800/60">
                    <span className="text-xs font-medium text-zinc-400">{key}</span>
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(typeof value === 'string' ? value : JSON.stringify(value, null, 2));
                        toast.success(`Copied "${key}"`);
                      }}
                      className="text-zinc-600 hover:text-zinc-400 transition-colors"
                    >
                      <Copy size={11} />
                    </button>
                  </div>
                  <div className="px-3 py-2">
                    {typeof value === 'string' ? (
                      value.length > 200 ? (
                        <pre className="text-xs text-zinc-300 font-mono whitespace-pre-wrap break-all max-h-48 overflow-y-auto">{value}</pre>
                      ) : (
                        <span className="text-sm text-zinc-200">{value}</span>
                      )
                    ) : (
                      <pre className="text-xs text-zinc-300 font-mono whitespace-pre-wrap break-all max-h-48 overflow-y-auto">
                        {JSON.stringify(value, null, 2)}
                      </pre>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}

          <h4 className="text-xs font-semibold text-zinc-500 uppercase tracking-wide mt-6 mb-3">Raw Event</h4>
          <pre className="text-xs text-zinc-400 font-mono bg-zinc-800/40 border border-zinc-700/40 rounded-lg p-3 whitespace-pre-wrap break-all max-h-64 overflow-y-auto">
            {JSON.stringify(event, null, 2)}
          </pre>
        </div>
      </div>
    </div>
  );
}

export default function AuditView() {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [catFilter, setCatFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [limit, setLimit] = useState(50);
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);
  const [showExport, setShowExport] = useState(false);
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

  const uniqueTypes = ['all', ...Array.from(new Set(events.map(e => e.type).filter(Boolean)))];
  const uniqueCategories = ['all', ...Array.from(new Set(events.map(e => e.category).filter(Boolean)))];

  const filtered = events.filter(e => {
    if (typeFilter !== 'all' && e.type !== typeFilter) return false;
    if (catFilter !== 'all' && e.category !== catFilter) return false;
    if (statusFilter !== 'all' && e.status !== statusFilter) return false;
    const eventTime = getEventTime(e);
    if (dateFrom && eventTime) {
      const from = new Date(dateFrom); from.setHours(0, 0, 0, 0);
      if (new Date(eventTime) < from) return false;
    }
    if (dateTo && eventTime) {
      const to = new Date(dateTo); to.setHours(23, 59, 59, 999);
      if (new Date(eventTime) > to) return false;
    }
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

  const exportJSON = () => {
    const blob = new Blob([JSON.stringify(filtered, null, 2)], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `soulgate-audit-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(a.href);
    toast.success(`Exported ${filtered.length} events as JSON`);
    setShowExport(false);
  };

  const exportCSV = () => {
    const headers = ['timestamp', 'type', 'category', 'status', 'session_id', 'run_id', 'metadata'];
    const rows = filtered.map(e => [
      getEventTime(e), e.type, e.category, e.status, e.session_id || '', e.run_id || '', formatMetadata(e.metadata),
    ]);
    const csv = [headers.join(','), ...rows.map(r => r.map(v => `"${(v || '').replace(/"/g, '""')}"`).join(','))].join('\n');
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([csv], { type: 'text/csv' }));
    a.download = `soulgate-audit-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(a.href);
    toast.success(`Exported ${filtered.length} events as CSV`);
    setShowExport(false);
  };

  const exportMarkdown = () => {
    let md = `# SoulGate Audit Log\n\nExported: ${new Date().toLocaleString()}\nEvents: ${filtered.length}\n\n`;
    md += `| Time | Type | Category | Status | Session | Details |\n|------|------|----------|--------|---------|--------|\n`;
    for (const e of filtered) {
      const t = getEventTime(e) ? new Date(getEventTime(e)).toLocaleString() : '-';
      md += `| ${t} | ${e.type} | ${e.category || '-'} | ${e.status} | ${e.session_id?.slice(0, 12) || '-'} | ${truncate(formatMetadata(e.metadata), 50) || '-'} |\n`;
    }
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([md], { type: 'text/markdown' }));
    a.download = `soulgate-audit-${new Date().toISOString().slice(0, 10)}.md`;
    a.click();
    URL.revokeObjectURL(a.href);
    toast.success(`Exported ${filtered.length} events as Markdown`);
    setShowExport(false);
  };

  const hasDateFilter = dateFrom || dateTo;
  const hasAnyFilter = typeFilter !== 'all' || catFilter !== 'all' || statusFilter !== 'all' || hasDateFilter || query;

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Audit Log</h2>
          <p className="text-sm text-zinc-500">
            {events.length} events{hasAnyFilter ? ` / ${filtered.length} matching` : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {/* Export */}
          <div className="relative">
            <button
              onClick={() => setShowExport(s => !s)}
              disabled={filtered.length === 0}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all disabled:opacity-40"
            >
              <Download size={13} />
              Export
            </button>
            {showExport && (
              <div className="absolute top-full mt-1 right-0 z-30 bg-zinc-900 border border-zinc-700 rounded-lg shadow-xl overflow-hidden w-48">
                <button onClick={exportJSON} className="w-full flex items-center gap-2 px-3 py-2.5 text-xs text-zinc-300 hover:bg-zinc-800 transition-colors">
                  <span className="text-amber-400">{'{ }'}</span> Export as JSON
                </button>
                <button onClick={exportCSV} className="w-full flex items-center gap-2 px-3 py-2.5 text-xs text-zinc-300 hover:bg-zinc-800 transition-colors">
                  <span className="text-emerald-400">CSV</span> Export as CSV
                </button>
                <button onClick={exportMarkdown} className="w-full flex items-center gap-2 px-3 py-2.5 text-xs text-zinc-300 hover:bg-zinc-800 transition-colors">
                  <span className="text-violet-400">MD</span> Export as Markdown
                </button>
              </div>
            )}
          </div>

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
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="space-y-3 mb-5">
        {/* Search */}
        <div className="relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search events by type, session, metadata..."
            className="w-full pl-9 pr-3 py-2.5 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
          />
        </div>

        {/* Date range */}
        <div className="flex items-center gap-3 flex-wrap">
          <Calendar size={13} className="text-zinc-500" />
          <div className="flex items-center gap-1.5">
            <span className="text-xs text-zinc-600">From</span>
            <input
              type="date"
              value={dateFrom}
              onChange={e => setDateFrom(e.target.value)}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60 [color-scheme:dark]"
            />
          </div>
          <div className="flex items-center gap-1.5">
            <span className="text-xs text-zinc-600">To</span>
            <input
              type="date"
              value={dateTo}
              onChange={e => setDateTo(e.target.value)}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60 [color-scheme:dark]"
            />
          </div>
          <div className="flex gap-1">
            <button onClick={() => { const d = new Date().toISOString().slice(0, 10); setDateFrom(d); setDateTo(d); }}
              className="px-2 py-1 rounded text-[10px] text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors">Today</button>
            <button onClick={() => { const t = new Date(); const f = new Date(t.getTime() - 7 * 86400000); setDateFrom(f.toISOString().slice(0, 10)); setDateTo(t.toISOString().slice(0, 10)); }}
              className="px-2 py-1 rounded text-[10px] text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors">7 days</button>
            <button onClick={() => { const t = new Date(); const f = new Date(t.getTime() - 30 * 86400000); setDateFrom(f.toISOString().slice(0, 10)); setDateTo(t.toISOString().slice(0, 10)); }}
              className="px-2 py-1 rounded text-[10px] text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors">30 days</button>
            {hasDateFilter && (
              <button onClick={() => { setDateFrom(''); setDateTo(''); }}
                className="px-2 py-1 rounded text-[10px] text-red-400/60 hover:text-red-400 hover:bg-red-500/10 transition-colors">Clear</button>
            )}
          </div>
        </div>

        {/* Type filter pills */}
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

        {/* Category, status, limit */}
        <div className="flex flex-wrap gap-4">
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Category:</span>
            <select value={catFilter} onChange={e => setCatFilter(e.target.value)}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60">
              {uniqueCategories.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Status:</span>
            <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60">
              {['all', 'success', 'denied', 'error'].map(s => <option key={s} value={s}>{s}</option>)}
            </select>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-zinc-600">Limit:</span>
            <select value={limit} onChange={e => setLimit(Number(e.target.value))}
              className="px-2 py-1 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60">
              {[25, 50, 100, 200, 500].map(n => <option key={n} value={n}>{n}</option>)}
            </select>
          </div>
          {hasAnyFilter && (
            <button
              onClick={() => { setTypeFilter('all'); setCatFilter('all'); setStatusFilter('all'); setQuery(''); setDateFrom(''); setDateTo(''); }}
              className="flex items-center gap-1 px-2 py-1 rounded text-xs text-red-400/60 hover:text-red-400 hover:bg-red-500/10 transition-colors"
            >
              <X size={11} /> Clear all
            </button>
          )}
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
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-36">Time</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Type</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-20">Category</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-20">Status</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-24">Session</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Details</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((event, idx) => {
                  const time = getEventTime(event);
                  return (
                    <tr key={event.id || idx} onClick={() => setSelectedEvent(event)} className="border-b border-zinc-800/40 hover:bg-indigo-500/5 transition-colors cursor-pointer">
                      <td className="px-4 py-2.5 text-xs text-zinc-500 whitespace-nowrap" title={time ? new Date(time).toLocaleString() : ''}>
                        <div>{time ? new Date(time).toLocaleDateString([], { day: '2-digit', month: 'short' }) : '---'}</div>
                        <div className="text-zinc-600">{time ? new Date(time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : ''}</div>
                      </td>
                      <td className="px-4 py-2.5">
                        <span className={`text-xs px-2 py-0.5 rounded-full ${getTypeColor(event.type)}`}>
                          {(event.type || '').replace(/\./g, ' ')}
                        </span>
                      </td>
                      <td className="px-4 py-2.5 text-xs text-zinc-500">{event.category || '---'}</td>
                      <td className={`px-4 py-2.5 text-xs font-medium ${getStatusColor(event.status)}`}>
                        {event.status || '---'}
                      </td>
                      <td className="px-4 py-2.5">
                        {event.session_id ? (
                          <span className="font-mono text-xs text-zinc-600">{event.session_id.slice(0, 12)}...</span>
                        ) : (
                          <span className="text-xs text-zinc-700">---</span>
                        )}
                      </td>
                      <td className="px-4 py-2.5 text-xs text-zinc-500 font-mono max-w-xs">
                        <span title={JSON.stringify(event.metadata)}>
                          {truncate(formatMetadata(event.metadata), 80) || '---'}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {selectedEvent && (
        <EventDetailPanel event={selectedEvent} onClose={() => setSelectedEvent(null)} />
      )}
    </div>
  );
}
