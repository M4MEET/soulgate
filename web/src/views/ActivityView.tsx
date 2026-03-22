import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Activity, MessageSquare, Bot, Radio, RefreshCw,
  ChevronRight, Clock, Hash, User, ArrowRight, Zap, Send,
  Globe, Filter
} from 'lucide-react';
import {
  fetchSessions, fetchConnectors, fetchActivity, fetchSessionDetail, replyToSession, fetchAgents,
  type SessionData, type ConnectorsData, type ActivityEntry, type SessionMessage, type AgentData,
} from '../lib/api';
import toast from 'react-hot-toast';

// Channel color map
const channelColors: Record<string, { bg: string; text: string; dot: string; border: string }> = {
  telegram:  { bg: 'bg-sky-500/10',     text: 'text-sky-400',     dot: 'bg-sky-400',     border: 'border-sky-500/20' },
  discord:   { bg: 'bg-indigo-500/10',  text: 'text-indigo-400',  dot: 'bg-indigo-400',  border: 'border-indigo-500/20' },
  slack:     { bg: 'bg-emerald-500/10', text: 'text-emerald-400', dot: 'bg-emerald-400', border: 'border-emerald-500/20' },
  whatsapp:  { bg: 'bg-green-500/10',   text: 'text-green-400',   dot: 'bg-green-400',   border: 'border-green-500/20' },
  web:       { bg: 'bg-violet-500/10',  text: 'text-violet-400',  dot: 'bg-violet-400',  border: 'border-violet-500/20' },
  api:       { bg: 'bg-orange-500/10',  text: 'text-orange-400',  dot: 'bg-orange-400',  border: 'border-orange-500/20' },
  matrix:    { bg: 'bg-teal-500/10',    text: 'text-teal-400',    dot: 'bg-teal-400',    border: 'border-teal-500/20' },
  irc:       { bg: 'bg-amber-500/10',   text: 'text-amber-400',   dot: 'bg-amber-400',   border: 'border-amber-500/20' },
  signal:    { bg: 'bg-blue-500/10',    text: 'text-blue-400',    dot: 'bg-blue-400',    border: 'border-blue-500/20' },
  twitch:    { bg: 'bg-purple-500/10',  text: 'text-purple-400',  dot: 'bg-purple-400',  border: 'border-purple-500/20' },
};
const defaultChannel = { bg: 'bg-zinc-500/10', text: 'text-zinc-400', dot: 'bg-zinc-400', border: 'border-zinc-500/20' };

function getChannelStyle(channel: string) {
  return channelColors[channel?.toLowerCase()] || defaultChannel;
}

function formatTs(ts: number) {
  return new Date(ts * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function relativeTime(iso: string) {
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 60000) return 'just now';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  return `${Math.floor(diff / 86400000)}d ago`;
}

// ── Session Chat Panel ───────────────────────────────────────────────────────

function SessionChat({ sessionId, onClose }: { sessionId: string; onClose: () => void }) {
  const [messages, setMessages] = useState<SessionMessage[]>([]);
  const [meta, setMeta] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(true);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const loadMessages = useCallback(async () => {
    const d = await fetchSessionDetail(sessionId);
    setMessages(d.messages);
    setMeta(d.meta);
    setLoading(false);
  }, [sessionId]);

  useEffect(() => {
    setLoading(true);
    loadMessages();
    // Auto-refresh session chat every 2 seconds
    const t = setInterval(loadMessages, 2000);
    return () => clearInterval(t);
  }, [loadMessages]);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages.length]);

  // Focus input on mount
  useEffect(() => {
    inputRef.current?.focus();
  }, [sessionId]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || sending) return;

    setSending(true);
    setInput('');
    try {
      const result = await replyToSession(sessionId, text);
      if (result.error) {
        toast.error(result.error);
      }
      // Refresh messages immediately to show the reply
      await loadMessages();
    } catch (err) {
      toast.error(`Failed to send: ${err}`);
    } finally {
      setSending(false);
      inputRef.current?.focus();
    }
  };

  const channel = String((meta as any)?.channel || sessionId.split(':')[0] || 'unknown');
  const style = getChannelStyle(channel);

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className={`flex items-center gap-3 px-4 py-3 border-b border-zinc-700/40 ${style.bg}`}>
        <button onClick={onClose} className="text-zinc-400 hover:text-zinc-200">
          <ChevronRight size={16} className="rotate-180" />
        </button>
        <div className={`w-2 h-2 rounded-full ${style.dot} animate-pulse`} />
        <div className="flex-1 min-w-0">
          <div className={`text-sm font-semibold ${style.text} capitalize`}>{channel}</div>
          <div className="text-xs text-zinc-500 truncate">{sessionId}</div>
        </div>
        <span className="text-xs text-zinc-500">{messages.length} entries</span>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-2" style={{ scrollbarWidth: 'thin' }}>
        {loading ? (
          <div className="flex items-center justify-center h-32 text-zinc-500 text-sm">
            <RefreshCw size={16} className="animate-spin mr-2" />Loading...
          </div>
        ) : messages.length === 0 ? (
          <div className="flex items-center justify-center h-32 text-zinc-500 text-sm">No messages</div>
        ) : (
          messages.map((msg, i) => {
            const data = msg.data as Record<string, unknown>;
            const isIncoming = msg.type === 'event.message' || msg.type === 'message';
            const isResponse = msg.type === 'cmd.channel.send' || msg.type === 'response';
            const isTool = msg.type.includes('tool');

            return (
              <div key={i} className={`flex ${isResponse ? 'justify-start' : isIncoming ? 'justify-end' : 'justify-center'}`}>
                <div className={`max-w-[85%] rounded-xl px-3 py-2 text-sm ${
                  isIncoming
                    ? 'bg-blue-600/20 border border-blue-500/20 text-blue-100'
                    : isResponse
                    ? 'bg-zinc-800/60 border border-zinc-700/40 text-zinc-200'
                    : isTool
                    ? 'bg-amber-500/5 border border-amber-500/10 text-amber-200 text-xs font-mono w-full'
                    : 'bg-zinc-800/30 border border-zinc-700/20 text-zinc-400 text-xs w-full'
                }`}>
                  {isIncoming && !!data.sender && (
                    <div className="flex items-center gap-1.5 mb-1">
                      <User size={10} className="text-blue-400" />
                      <span className="text-[10px] text-blue-400 font-medium">
                        {(data.sender as any)?.username || (data.sender as any)?.name || 'User'}
                      </span>
                    </div>
                  )}
                  {isResponse && (
                    <div className="flex items-center gap-1.5 mb-1">
                      <Bot size={10} className="text-emerald-400" />
                      <span className="text-[10px] text-emerald-400 font-medium">SoulGate</span>
                    </div>
                  )}
                  {isTool && (
                    <div className="flex items-center gap-1.5 mb-1">
                      <Zap size={10} className="text-amber-400" />
                      <span className="text-[10px] text-amber-400">{msg.type}: {String(data.tool_name || data.tool || '')}</span>
                    </div>
                  )}
                  <div className="whitespace-pre-wrap break-words">
                    {String(data.text || data.result || data.error || JSON.stringify(data).slice(0, 300))}
                  </div>
                  <div className="text-[10px] text-zinc-600 mt-1 text-right">{formatTs(msg.ts)}</div>
                </div>
              </div>
            );
          })
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Chat input */}
      <div className="border-t border-zinc-700/40 px-4 py-3">
        <div className="flex items-center gap-2">
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); } }}
            placeholder={`Reply to ${channel} conversation...`}
            disabled={sending}
            className="flex-1 px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 disabled:opacity-50"
          />
          <button
            onClick={handleSend}
            disabled={sending || !input.trim()}
            className="p-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white transition-colors disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center"
          >
            {sending ? (
              <RefreshCw size={16} className="animate-spin" />
            ) : (
              <Send size={16} />
            )}
          </button>
        </div>
        <div className="text-[10px] text-zinc-600 mt-1">
          Your message will be processed by SoulGate AI and the reply sent to the {channel} user
        </div>
      </div>
    </div>
  );
}

// ── Main Activity View ───────────────────────────────────────────────────────

export default function ActivityView() {
  const [sessions, setSessions] = useState<SessionData[]>([]);
  const [connectors, setConnectors] = useState<ConnectorsData | null>(null);
  const [activity, setActivity] = useState<ActivityEntry[]>([]);
  const [selectedSession, setSelectedSession] = useState<string | null>(null);
  const [channelFilter, setChannelFilter] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [agents, setAgents] = useState<AgentData[]>([]);
  const prevActivityLen = useRef(0);

  const refresh = useCallback(async () => {
    const [s, c, a, ag] = await Promise.all([
      fetchSessions(),
      fetchConnectors(),
      fetchActivity(100, channelFilter || undefined),
      fetchAgents(),
    ]);
    setSessions(s);
    setConnectors(c);
    setActivity(a);
    setAgents(ag);
    setLoading(false);

    // Flash indicator when new activity arrives
    if (a.length > prevActivityLen.current && prevActivityLen.current > 0) {
      document.title = `(${a.length - prevActivityLen.current} new) SoulGate`;
      setTimeout(() => { document.title = 'SoulGate'; }, 3000);
    }
    prevActivityLen.current = a.length;
  }, [channelFilter]);

  useEffect(() => {
    setLoading(true);
    refresh();
  }, [refresh]);

  // Auto-refresh every 2 seconds for real-time feel
  useEffect(() => {
    const t = setInterval(refresh, 2000);
    return () => clearInterval(t);
  }, [refresh]);

  // Unique channels from sessions
  const channels = [...new Set(sessions.map(s => s.channel).filter(Boolean))];

  // Sessions filtered by channel, sorted by last activity (most recent first)
  const filteredSessions = (channelFilter
    ? sessions.filter(s => s.channel === channelFilter)
    : sessions
  ).sort((a, b) => new Date(b.last_activity).getTime() - new Date(a.last_activity).getTime());

  return (
    <div className="flex h-full overflow-hidden">
      {/* Left panel -- Connectors & Sessions */}
      <div className="w-80 flex-shrink-0 border-r border-zinc-700/40 flex flex-col bg-zinc-900/30 overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-700/40">
          <div className="flex items-center gap-2">
            <Activity size={16} className="text-indigo-400" />
            <span className="text-sm font-semibold text-zinc-200">Activity Hub</span>
            {activity.length > 0 && (
              <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-indigo-500/20 text-indigo-400 font-medium">
                {activity.length}
              </span>
            )}
          </div>
          <button onClick={refresh} className="p-1.5 rounded-lg hover:bg-zinc-700/40 text-zinc-500 hover:text-zinc-300 transition-colors">
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>

        {/* Connected clients summary — count WS channels + HTTP spawned connectors */}
        {connectors && (() => {
          const wsChannels = connectors.channels.length;
          // Deduplicate: if a spawned connector's type matches a WS channel, don't double count
          const wsTypes = new Set(connectors.channels.map(c => c.channel || c.metadata?.channel));
          const uniqueHTTP = (connectors.spawned || []).filter(s => s.status === 'running' && !wsTypes.has(s.type)).length;
          const channelCount = wsChannels + uniqueHTTP;

          return (
            <div className="px-4 py-2.5 border-b border-zinc-700/30">
              <div className="flex gap-3">
                <div className="flex items-center gap-1.5">
                  <Radio size={11} className={channelCount > 0 ? 'text-emerald-400 animate-pulse' : 'text-zinc-600'} />
                  <span className="text-[11px] text-zinc-400">{channelCount} channel{channelCount !== 1 ? 's' : ''} connected</span>
                </div>
                {agents.length > 0 && (() => {
                  const running = agents.filter(a => a.status === 'running').length;
                  return (
                    <div className="flex items-center gap-1.5">
                      <Bot size={11} className={running > 0 ? 'text-indigo-400 animate-pulse' : 'text-indigo-400'} />
                      <span className="text-[11px] text-zinc-400">
                        {running > 0 ? `${running} running` : `${agents.length} agent${agents.length !== 1 ? 's' : ''}`}
                      </span>
                    </div>
                  );
                })()}
              </div>
              {/* Show active connectors inline */}
              {channelCount > 0 && (
                <div className="flex flex-wrap gap-1.5 mt-2">
                  {connectors.channels.map(c => {
                    const ch = c.channel || c.metadata?.channel || 'unknown';
                    const style = getChannelStyle(ch);
                    const label = ch.charAt(0).toUpperCase() + ch.slice(1);
                    const botName = c.metadata?.bot_username ? `@${c.metadata.bot_username}` : '';
                    return (
                      <span key={c.client_id} className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] ${style.bg} ${style.text} border ${style.border}`}>
                        <span className={`w-1.5 h-1.5 rounded-full ${style.dot} animate-pulse`} />
                        <span className="font-semibold">{label}</span>
                        {botName && <span className="opacity-70">{botName}</span>}
                      </span>
                    );
                  })}
                  {(connectors.spawned || []).filter(s => s.status === 'running' && !wsTypes.has(s.type)).map(s => {
                    const style = getChannelStyle(s.type);
                    const label = s.type.charAt(0).toUpperCase() + s.type.slice(1);
                    // Try to find a bot name from sessions for this channel type
                    const channelSession = sessions.find(sess => sess.channel === s.type);
                    const botHint = channelSession ? 'connected' : 'active';
                    return (
                      <span key={`spawned-${s.type}`} className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] ${style.bg} ${style.text} border ${style.border}`}>
                        <span className={`w-1.5 h-1.5 rounded-full ${style.dot} animate-pulse`} />
                        <span className="font-semibold">{label}</span>
                        <span className="opacity-70">{botHint}</span>
                      </span>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })()}

        {/* Channel filter */}
        {channels.length > 0 && (
          <div className="px-3 py-2 border-b border-zinc-700/30">
            <div className="flex items-center gap-2">
              <Filter size={12} className="text-zinc-500" />
              <select
                value={channelFilter}
                onChange={e => setChannelFilter(e.target.value)}
                className="flex-1 bg-transparent text-xs text-zinc-300 border-none outline-none cursor-pointer"
              >
                <option value="">All channels</option>
                {channels.map(ch => (
                  <option key={ch} value={ch}>{ch}</option>
                ))}
              </select>
            </div>
          </div>
        )}

        {/* Sessions list */}
        <div className="flex-1 overflow-y-auto" style={{ scrollbarWidth: 'thin' }}>
          {filteredSessions.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-32 text-zinc-500 text-xs gap-2">
              <Globe size={20} className="text-zinc-600" />
              <span>{loading ? 'Loading...' : 'No sessions yet'}</span>
              {!loading && <span className="text-zinc-600">Send a message to any connector</span>}
            </div>
          ) : (
            filteredSessions.map(s => {
              const ch = s.channel || s.id.split(':')[0] || 'unknown';
              const style = getChannelStyle(ch);
              const isSelected = selectedSession === s.id;

              return (
                <button
                  key={s.id}
                  onClick={() => setSelectedSession(s.id)}
                  className={`w-full text-left px-3 py-2.5 border-b border-zinc-800/50 transition-all ${
                    isSelected ? 'bg-indigo-500/10 border-l-2 border-l-indigo-500' : 'hover:bg-zinc-800/40 border-l-2 border-l-transparent'
                  }`}
                >
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className={`w-1.5 h-1.5 rounded-full ${style.dot}`} />
                    <span className={`text-xs font-semibold capitalize ${style.text}`}>{ch}</span>
                    <span className="ml-auto text-[10px] text-zinc-600">
                      {s.last_activity ? relativeTime(s.last_activity) : ''}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 pl-3.5">
                    <span className="text-[11px] text-zinc-400 truncate flex-1">{s.conversation_id || s.id}</span>
                    <span className="text-[10px] text-zinc-600 flex items-center gap-1">
                      <MessageSquare size={9} /> {s.message_count}
                    </span>
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>

      {/* Center panel -- Session chat or Activity feed */}
      <div className="flex-1 flex flex-col min-w-0 h-full overflow-hidden">
        {selectedSession ? (
          <SessionChat
            sessionId={selectedSession}
            onClose={() => setSelectedSession(null)}
          />
        ) : (
          <>
            {/* Activity feed header */}
            <div className="flex items-center gap-3 px-4 py-3 border-b border-zinc-700/40">
              <Clock size={16} className="text-zinc-400" />
              <span className="text-sm font-semibold text-zinc-200">Live Activity Feed</span>
              <span className="text-xs text-zinc-500">{activity.length} events</span>
              {activity.length > 0 && (
                <div className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse ml-1" title="Live" />
              )}
            </div>

            {/* Activity feed */}
            <div className="flex-1 overflow-y-auto p-4" style={{ scrollbarWidth: 'thin' }}>
              {activity.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-48 text-zinc-500 text-sm gap-3">
                  <Activity size={28} className="text-zinc-600" />
                  <span>{loading ? 'Loading activity...' : 'No activity yet'}</span>
                  <span className="text-xs text-zinc-600">Messages from all connectors will appear here in real-time</span>
                  {!loading && connectors && connectors.channels.length === 0 && (
                    <span className="text-xs text-amber-500/70 mt-1">No connectors active -- connect one first</span>
                  )}
                </div>
              ) : (
                <div className="space-y-1">
                  {activity.map((entry, i) => {
                    const style = getChannelStyle(entry.channel);
                    const data = entry.data as Record<string, unknown>;
                    const isMessage = entry.type === 'event.message' || entry.type === 'message';
                    const isResponse = entry.type === 'cmd.channel.send' || entry.type === 'response';
                    const isTool = entry.type.includes('tool');

                    let icon = <Hash size={12} />;
                    let label = entry.type;
                    let preview = '';

                    if (isMessage) {
                      icon = <ArrowRight size={12} className="text-blue-400" />;
                      label = String((data.sender as any)?.username || (data.sender as any)?.name || 'user');
                      preview = String(data.text || '').slice(0, 120);
                    } else if (isResponse) {
                      icon = <Send size={12} className="text-emerald-400" />;
                      label = 'SoulGate';
                      preview = String(data.text || '').slice(0, 120);
                    } else if (isTool) {
                      icon = <Zap size={12} className="text-amber-400" />;
                      label = String(data.tool_name || entry.type);
                      preview = String(data.result || data.error || '').slice(0, 80);
                    }

                    return (
                      <button
                        key={`${entry.ts}-${i}`}
                        onClick={() => setSelectedSession(entry.session_id)}
                        className={`w-full text-left flex items-start gap-2.5 px-3 py-2 rounded-lg hover:bg-zinc-800/40 transition-all group ${
                          i === 0 ? 'bg-zinc-800/20' : ''
                        }`}
                      >
                        <div className="mt-0.5">{icon}</div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <span className={`text-[10px] px-1.5 py-0.5 rounded ${style.bg} ${style.text} capitalize font-medium`}>
                              {entry.channel}
                            </span>
                            <span className="text-xs text-zinc-300 font-medium">{label}</span>
                            <span className="ml-auto text-[10px] text-zinc-600">{formatTs(entry.ts)}</span>
                          </div>
                          {preview && (
                            <div className="text-xs text-zinc-500 mt-0.5 truncate">{preview}</div>
                          )}
                        </div>
                        <ChevronRight size={12} className="text-zinc-700 group-hover:text-zinc-500 mt-1.5 flex-shrink-0" />
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
