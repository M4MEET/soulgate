import { useState } from 'react';
import { BookOpen, Search, Download, ExternalLink, Clock, MessageSquare, Activity } from 'lucide-react';
import type { SessionData } from '../lib/api';
import { formatRelativeTime } from '../lib/utils';
import toast from 'react-hot-toast';

interface Props {
  sessions: SessionData[];
}

const CHANNEL_COLORS: Record<string, string> = {
  telegram:   'text-sky-400 bg-sky-500/10',
  discord:    'text-indigo-400 bg-indigo-500/10',
  slack:      'text-emerald-400 bg-emerald-500/10',
  web:        'text-violet-400 bg-violet-500/10',
  api:        'text-orange-400 bg-orange-500/10',
  default:    'text-zinc-400 bg-zinc-700/40',
};

function exportJSON(session: SessionData) {
  const blob = new Blob([JSON.stringify(session, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `session-${session.id.slice(0, 8)}.json`;
  a.click();
  URL.revokeObjectURL(url);
  toast.success('Exported as JSON');
}

function exportMarkdown(session: SessionData) {
  const md = `# Session ${session.id}\n\n**Channel:** ${session.channel}\n**Messages:** ${session.message_count}\n**Created:** ${session.created_at}\n`;
  const blob = new Blob([md], { type: 'text/markdown' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `session-${session.id.slice(0, 8)}.md`;
  a.click();
  URL.revokeObjectURL(url);
  toast.success('Exported as Markdown');
}

export default function SessionsView({ sessions }: Props) {
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<SessionData | null>(null);

  const filtered = sessions.filter(s =>
    s.id?.toLowerCase().includes(query.toLowerCase()) ||
    s.channel?.toLowerCase().includes(query.toLowerCase()) ||
    s.conversation_id?.toLowerCase().includes(query.toLowerCase())
  );

  // Demo sessions if none
  const display = filtered.length === 0 && query === '' && sessions.length === 0
    ? [
        { id: 'abc123def456', conversation_id: 'conv_01', channel: 'web',      message_count: 14, created_at: new Date(Date.now() - 7200000).toISOString(), last_activity: new Date(Date.now() - 300000).toISOString() },
        { id: 'xyz789uvw012', conversation_id: 'conv_02', channel: 'telegram', message_count: 6,  created_at: new Date(Date.now() - 86400000).toISOString(), last_activity: new Date(Date.now() - 3600000).toISOString() },
        { id: 'pqr345stu678', conversation_id: 'conv_03', channel: 'api',      message_count: 3,  created_at: new Date(Date.now() - 172800000).toISOString(), last_activity: new Date(Date.now() - 86400000).toISOString() },
      ] as SessionData[]
    : filtered;

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Sessions</h2>
          <p className="text-sm text-zinc-500">{sessions.length} total</p>
        </div>
      </div>

      {/* Search */}
      <div className="relative mb-5">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
        <input
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Search sessions by ID, channel…"
          className="w-full pl-9 pr-3 py-2.5 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
        />
      </div>

      {/* Session list */}
      {display.length === 0 ? (
        <div className="text-center py-16 text-zinc-500">
          <BookOpen size={40} className="mx-auto mb-3 opacity-30" />
          <p>No sessions found.</p>
        </div>
      ) : (
        <div className="space-y-2">
          {display.map(session => {
            const channelClass = CHANNEL_COLORS[session.channel] || CHANNEL_COLORS.default;
            return (
              <div
                key={session.id}
                className="flex items-center gap-4 p-4 rounded-xl bg-zinc-800/40 border border-zinc-700/40 hover:border-zinc-600/60 hover:bg-zinc-800/60 transition-all cursor-pointer group"
                onClick={() => setSelected(s => s?.id === session.id ? null : session)}
              >
                <div className="flex-shrink-0 w-8 h-8 rounded-lg bg-zinc-800 border border-zinc-700/50 flex items-center justify-center">
                  <MessageSquare size={14} className="text-zinc-500" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="font-mono text-sm text-zinc-200 truncate">{session.id.slice(0, 16)}…</span>
                    <span className={`text-xs px-2 py-0.5 rounded-full ${channelClass}`}>{session.channel}</span>
                  </div>
                  <div className="flex items-center gap-3 text-xs text-zinc-600">
                    <span className="flex items-center gap-1"><MessageSquare size={10} />{session.message_count} msgs</span>
                    <span className="flex items-center gap-1"><Clock size={10} />{formatRelativeTime(session.created_at)}</span>
                    {session.last_activity && <span>Active {formatRelativeTime(session.last_activity)}</span>}
                  </div>
                </div>
                <div className="flex items-center gap-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    onClick={e => { e.stopPropagation(); exportJSON(session); }}
                    title="Export JSON"
                    className="p-1.5 rounded-lg hover:bg-zinc-700 text-zinc-500 hover:text-zinc-300 transition-all"
                  >
                    <Download size={13} />
                  </button>
                  <button
                    onClick={e => { e.stopPropagation(); exportMarkdown(session); }}
                    title="Export Markdown"
                    className="p-1.5 rounded-lg hover:bg-zinc-700 text-zinc-500 hover:text-zinc-300 transition-all"
                  >
                    <ExternalLink size={13} />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Session detail panel */}
      {selected && (
        <div className="mt-5 p-5 rounded-xl border border-indigo-500/20 bg-indigo-500/5">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-semibold text-zinc-200">Session Details</h3>
            <button onClick={() => setSelected(null)} className="text-zinc-600 hover:text-zinc-300 transition-colors text-lg leading-none">×</button>
          </div>
          <div className="grid grid-cols-2 gap-3 text-sm mb-4">
            <div><span className="text-zinc-500">ID:</span> <span className="font-mono text-indigo-300 ml-2">{selected.id}</span></div>
            <div><span className="text-zinc-500">Channel:</span> <span className="text-zinc-300 ml-2">{selected.channel}</span></div>
            <div><span className="text-zinc-500">Messages:</span> <span className="text-zinc-300 ml-2">{selected.message_count}</span></div>
            <div><span className="text-zinc-500">Created:</span> <span className="text-zinc-300 ml-2">{new Date(selected.created_at).toLocaleString()}</span></div>
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => exportJSON(selected)}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
            >
              <Download size={13} />
              Export JSON
            </button>
            <button
              onClick={() => exportMarkdown(selected)}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
            >
              <ExternalLink size={13} />
              Export MD
            </button>
            <button
              onClick={() => {
                const html = `<html><body><h1>Session ${selected.id}</h1><p>Channel: ${selected.channel}</p></body></html>`;
                const blob = new Blob([html], { type: 'text/html' });
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = `session-${selected.id.slice(0, 8)}.html`;
                a.click();
                URL.revokeObjectURL(url);
                toast.success('Exported as HTML');
              }}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
            >
              <Activity size={13} />
              Export HTML
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
