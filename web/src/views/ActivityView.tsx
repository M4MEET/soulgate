import { useState, useEffect, useCallback, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import {
  Activity, MessageSquare, Bot, Radio, RefreshCw,
  ChevronRight, Clock, User, Zap, Send,
  Globe, Filter, Play, Pause, Square, CheckCircle, AlertCircle, Cpu
} from 'lucide-react';
import {
  fetchSessions, fetchConnectors, fetchActivity, fetchSessionDetail, replyToSession, fetchAgents,
  fetchAgentLog, sendAgentMessage,
  type SessionData, type ConnectorsData, type ActivityEntry, type SessionMessage, type AgentData,
  type AgentLogEntry,
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
                <div className={`rounded-xl px-3 py-2 text-sm ${
                  isIncoming
                    ? 'max-w-[75%] bg-blue-600/20 border border-blue-500/20 text-blue-100'
                    : isResponse
                    ? 'max-w-[90%] bg-zinc-800/60 border border-zinc-700/40 text-zinc-200'
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
                  <div className="whitespace-pre-wrap break-words prose prose-invert prose-sm max-w-none prose-p:my-1 prose-pre:my-1 prose-ul:my-1 prose-li:my-0.5 prose-code:text-emerald-300 prose-code:bg-zinc-800 prose-code:px-1 prose-code:py-0.5 prose-code:rounded prose-pre:bg-zinc-900 prose-pre:rounded-lg prose-strong:text-zinc-100">
                    {isResponse ? (
                      <ReactMarkdown>{String(data.text || data.result || data.error || '')}</ReactMarkdown>
                    ) : (
                      String(data.text || data.result || data.error || JSON.stringify(data).slice(0, 300))
                    )}
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

// ── Agent Live Control Panel (embedded in Activity Hub) ──────────────────────

const LOG_ICONS: Record<string, { color: string; bg: string; icon: React.ElementType }> = {
  model_call:       { color: 'text-violet-400',  bg: 'bg-violet-500/10', icon: Cpu },
  model_done:       { color: 'text-violet-400',  bg: 'bg-violet-500/10', icon: CheckCircle },
  tool_start:       { color: 'text-amber-400',   bg: 'bg-amber-500/10',  icon: Play },
  tool_done:        { color: 'text-emerald-400', bg: 'bg-emerald-500/10',icon: CheckCircle },
  error:            { color: 'text-red-400',     bg: 'bg-red-500/10',    icon: AlertCircle },
  text:             { color: 'text-zinc-400',    bg: 'bg-zinc-500/10',   icon: MessageSquare },
  iteration:        { color: 'text-zinc-500',    bg: 'bg-zinc-500/10',   icon: Clock },
  status:           { color: 'text-blue-400',    bg: 'bg-blue-500/10',   icon: Activity },
  info:             { color: 'text-blue-400',    bg: 'bg-blue-500/10',   icon: Activity },
  message_received: { color: 'text-blue-400',    bg: 'bg-blue-500/10',   icon: MessageSquare },
};

// Extended log entry with source agent info
interface MergedLogEntry extends AgentLogEntry {
  agentId: string;
  agentName: string;
}

function AgentLivePanel({ agentId, agents, onClose }: { agentId: string; agents: AgentData[]; onClose: () => void }) {
  const [entries, setEntries] = useState<MergedLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [paused, setPaused] = useState(false);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const agent = agents.find(a => a.id === agentId);
  const isStandby = agent?.status === 'standby';
  const isRunning = agent?.status === 'running';

  // Get all alive agents for merged view
  const aliveAgents = agents.filter(a => a.status === 'running' || a.status === 'standby');

  const loadLog = useCallback(async () => {
    // Fetch logs from ALL alive agents and merge into unified timeline
    const agentsToFetch = aliveAgents.length > 1 ? aliveAgents : [agent].filter(Boolean);
    const allLogs: MergedLogEntry[] = [];

    await Promise.all(
      agentsToFetch.map(async (a) => {
        if (!a) return;
        try {
          const data = await fetchAgentLog(a.id, 50);
          for (const entry of data) {
            allLogs.push({ ...entry, agentId: a.id, agentName: a.name });
          }
        } catch { /* ignore */ }
      })
    );

    // Sort by timestamp
    allLogs.sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());
    setEntries(allLogs);
  }, [agentId, aliveAgents.length]);

  useEffect(() => {
    setLoading(true);
    loadLog().finally(() => setLoading(false));
  }, [loadLog]);

  useEffect(() => {
    if (paused) return;
    const id = setInterval(() => loadLog(), 2000);
    return () => clearInterval(id);
  }, [paused, loadLog]);

  useEffect(() => {
    if (!paused && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [entries, paused]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || sending) return;
    setSending(true);
    setInput('');
    try {
      await sendAgentMessage(agentId, text);
      toast.success('Sent to agent');
      await loadLog();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Send failed');
    } finally {
      setSending(false);
      inputRef.current?.focus();
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className={`flex items-center gap-3 px-4 py-3 border-b border-zinc-700/40 ${isStandby ? 'bg-violet-500/5' : 'bg-emerald-500/5'}`}>
        <button onClick={onClose} className="text-zinc-400 hover:text-zinc-200">
          <ChevronRight size={16} className="rotate-180" />
        </button>
        <div className={`w-2 h-2 rounded-full animate-pulse ${isStandby ? 'bg-violet-400' : 'bg-emerald-400'}`} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className={`text-sm font-semibold ${isStandby ? 'text-violet-400' : 'text-emerald-400'}`}>{agent?.name || agentId}</span>
            <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${
              isStandby ? 'bg-violet-500/15 text-violet-400' : isRunning ? 'bg-emerald-500/15 text-emerald-400' : 'bg-zinc-500/15 text-zinc-400'
            }`}>{agent?.status}</span>
          </div>
          <div className="text-[10px] text-zinc-500 truncate">{agent?.role} &middot; {agent?.id}</div>
        </div>
        <div className="flex items-center gap-1">
          <span className="text-xs text-zinc-600">{entries.length} entries</span>
          <button
            onClick={() => setPaused(p => !p)}
            className={`flex items-center gap-1 px-2 py-1 rounded-md text-[10px] border transition-all ${
              paused ? 'bg-amber-500/15 border-amber-500/30 text-amber-400' : 'border-zinc-700 text-zinc-500 hover:text-zinc-300'
            }`}
          >
            {paused ? <Play size={10} /> : <Pause size={10} />}
            {paused ? 'Resume' : 'Pause'}
          </button>
        </div>
      </div>

      {/* Activity log — chat-style with collapsible thinking */}
      <div className="flex-1 overflow-y-auto p-3 space-y-1.5" style={{ scrollbarWidth: 'thin' }}>
        {loading && (
          <div className="flex items-center gap-2 text-zinc-500 text-sm py-4">
            <Activity size={14} className="animate-spin" /> Loading...
          </div>
        )}
        {!loading && entries.length === 0 && (
          <div className="text-center py-8 text-zinc-600 text-sm">No activity yet</div>
        )}
        {(() => {
          // Group entries into: visible messages + collapsible thinking blocks
          const groups: { type: 'message' | 'thinking'; entries: typeof entries }[] = [];
          let thinkingBuf: typeof entries = [];

          const flushThinking = () => {
            if (thinkingBuf.length > 0) {
              groups.push({ type: 'thinking', entries: [...thinkingBuf] });
              thinkingBuf = [];
            }
          };

          for (const entry of entries) {
            const isVisible = entry.type === 'text' || entry.type === 'message_received' || entry.type === 'status' || entry.type === 'error';
            if (isVisible) {
              flushThinking();
              groups.push({ type: 'message', entries: [entry] });
            } else {
              thinkingBuf.push(entry);
            }
          }
          flushThinking();

          return groups.map((group, gi) => {
            if (group.type === 'thinking') {
              // Minimal thinking indicator — just a tiny dot, expandable
              return (
                <details key={gi} className="group flex justify-center">
                  <summary className="flex items-center justify-center gap-1 py-0.5 cursor-pointer list-none text-zinc-700 hover:text-zinc-500">
                    <span className="flex gap-0.5">
                      <span className="w-1 h-1 rounded-full bg-zinc-700" />
                      <span className="w-1 h-1 rounded-full bg-zinc-700" />
                      <span className="w-1 h-1 rounded-full bg-zinc-700" />
                    </span>
                  </summary>
                  <div className="ml-4 pl-2 border-l border-zinc-800 space-y-0.5 mt-1 mb-1">
                    {group.entries.map((entry, ei) => {
                      const cfg = LOG_ICONS[entry.type] || { color: 'text-zinc-500', bg: 'bg-zinc-500/10', icon: Clock };
                      const Icon = cfg.icon;
                      return (
                        <div key={ei} className="flex items-start gap-1.5 py-0.5 px-1 text-[10px]">
                          <Icon size={9} className={`${cfg.color} mt-0.5 flex-shrink-0 opacity-60`} />
                          <span className={`${cfg.color} opacity-60`}>{entry.type}</span>
                          <span className="text-zinc-600">{new Date(entry.timestamp).toLocaleTimeString()}</span>
                          <span className="text-zinc-500 truncate flex-1">{entry.message}</span>
                        </div>
                      );
                    })}
                  </div>
                </details>
              );
            }

            // Visible message entry
            const entry = group.entries[0];
            const isMsg = entry.type === 'message_received';
            const isText = entry.type === 'text';
            const isError = entry.type === 'error';
            const isStatus = entry.type === 'status';
            const isApproval = entry.message?.toLowerCase().includes('approval') || entry.message?.toLowerCase().includes('permission');

            // Filter noise
            if (isText && (
              entry.message === 'agent finished' ||
              entry.message?.startsWith('activation complete:') ||
              entry.message?.match(/^(Got it|I understand|I'll be ready|I'm ready|Let me|Here's how)/)
            )) {
              return null;
            }
            if (isStatus) return null; // hide all status lines
            // Hide standalone errors (they're in thinking blocks)
            if (isError && entry.message?.includes('standby activation error')) return null;

            // Agent identity
            const me = (entry as MergedLogEntry);
            const entryAgentId = me.agentId || agentId;
            const entryAgentName = me.agentName || agent?.name || 'Agent';

            // Color per agent
            const agentColors = [
              { text: 'text-emerald-400', bubble: 'bg-emerald-900/15 border-emerald-500/20' },
              { text: 'text-pink-400', bubble: 'bg-pink-900/15 border-pink-500/20' },
              { text: 'text-cyan-400', bubble: 'bg-cyan-900/15 border-cyan-500/20' },
              { text: 'text-amber-400', bubble: 'bg-amber-900/15 border-amber-500/20' },
            ];
            const agentIdx = aliveAgents.findIndex(a => a.id === entryAgentId);
            const aColor = agentColors[agentIdx >= 0 ? agentIdx % agentColors.length : 0];

            // Clean message text — strip "Summary:" boilerplate and meta prefixes
            const cleanMessage = (msg: string) => {
              let cleaned = msg;
              // Remove "Summary:\n- ..." sections
              cleaned = cleaned.replace(/\n\nSummary:[\s\S]*$/, '').replace(/\nSummary of actions[\s\S]*$/, '');
              // Remove meta prefixes like "[Roast to Boyfriend Nice]" or "Message to X (via agent_message):"
              cleaned = cleaned.replace(/^\[.*?\]\n?/, '').replace(/^Message to .+?\(via agent_message\):\n?/, '');
              return cleaned.trim();
            };

            if (isMsg) {
              // Incoming message to this agent — show on right (like user messages in chat)
              const sender = entry.message?.match(/\[([^\]]+)\]/)?.[1] || 'User';
              const msgText = entry.message?.replace(/^\[[^\]]+\]:\s*/, '') || entry.message;

              return (
                <div key={gi} className="flex justify-end">
                  <div className="max-w-[80%] rounded-2xl px-3.5 py-2.5 border bg-blue-600/15 border-blue-500/20">
                    <div className="flex items-center gap-1.5 mb-1">
                      <User size={10} className="text-blue-400" />
                      <span className="text-[10px] font-medium text-blue-400">{sender}</span>
                      <span className="text-[9px] text-zinc-600 ml-auto">{new Date(entry.timestamp).toLocaleTimeString()}</span>
                    </div>
                    <div className="text-[13px] text-zinc-100 break-words leading-relaxed">{msgText}</div>
                  </div>
                </div>
              );
            }

            if (isText) {
              const cleaned = cleanMessage(entry.message || '');
              if (!cleaned) return null;

              // Current agent → left, other agents → right (like a conversation)
              const isOtherAgent = entryAgentId !== agentId;

              return (
                <div key={gi} className={`flex ${isOtherAgent ? 'justify-end' : 'justify-start'}`}>
                  <div className={`max-w-[80%] rounded-2xl px-3.5 py-2.5 border ${aColor.bubble}`}>
                    <div className="flex items-center gap-1.5 mb-1">
                      <Bot size={10} className={aColor.text} />
                      <span className={`text-[10px] font-medium ${aColor.text}`}>{entryAgentName}</span>
                      <span className="text-[9px] text-zinc-600 ml-auto">{new Date(entry.timestamp).toLocaleTimeString()}</span>
                    </div>
                    <div className="text-[13px] text-zinc-100 break-words whitespace-pre-wrap leading-relaxed">{cleaned}</div>
                  </div>
                </div>
              );
            }

            if (isError) {
              // Compact collapsible error
              const shortErr = entry.message?.split('\n')[0]?.slice(0, 80) || 'Error';
              return (
                <details key={gi} className="group">
                  <summary className="flex items-center gap-1.5 px-2 py-1 rounded-lg cursor-pointer hover:bg-red-500/5 text-[10px] text-red-400/70 list-none">
                    <AlertCircle size={10} className="flex-shrink-0" />
                    <span className="truncate">{shortErr}</span>
                    <span className="text-[9px] text-zinc-700 ml-auto">{new Date(entry.timestamp).toLocaleTimeString()}</span>
                  </summary>
                  <div className="ml-4 px-2 py-1.5 mt-0.5 rounded bg-red-500/5 border border-red-500/10 text-[10px] text-red-300/80 break-words font-mono whitespace-pre-wrap max-h-32 overflow-y-auto">
                    {entry.message}
                  </div>
                </details>
              );
            }

            if (isStatus) {
              return (
                <div key={gi} className="flex items-center justify-center gap-1.5 py-0.5 text-[9px] text-zinc-700">
                  <span>{entry.message}</span>
                </div>
              );
            }

            if (isApproval) {
              return (
                <div key={gi} className="px-3 py-2 rounded-lg bg-amber-500/5 border border-amber-500/20">
                  <div className="text-xs text-amber-200 break-words mb-2">{entry.message}</div>
                  <div className="flex gap-2">
                    <button className="flex items-center gap-1 px-2.5 py-1 rounded bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-400 text-[10px] font-medium">
                      <CheckCircle size={10} /> Approve
                    </button>
                    <button className="flex items-center gap-1 px-2.5 py-1 rounded bg-red-500/20 hover:bg-red-500/30 text-red-400 text-[10px] font-medium">
                      <Square size={10} /> Deny
                    </button>
                  </div>
                </div>
              );
            }

            return null;
          });
        })()}
        <div ref={bottomRef} />
      </div>

      {/* Command input */}
      <div className="flex-shrink-0 border-t border-zinc-700/40 px-3 py-2.5">
        <div className="flex items-center gap-2">
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSend()}
            placeholder={isStandby ? "Send task to wake this agent..." : "Send message to agent..."}
            disabled={sending}
            className="flex-1 px-3 py-1.5 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-violet-500/60 disabled:opacity-50"
          />
          <button
            onClick={handleSend}
            disabled={sending || !input.trim()}
            className="p-2 rounded-lg bg-violet-600 hover:bg-violet-500 text-white transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <Send size={14} />
          </button>
        </div>
        <div className="text-[10px] text-zinc-600 mt-1">
          {isStandby ? 'Agent is in standby — send a task to wake it up' : 'Send commands to steer the running agent'}
        </div>
      </div>
    </div>
  );
}

export default function ActivityView() {
  const [sessions, setSessions] = useState<SessionData[]>([]);
  const [connectors, setConnectors] = useState<ConnectorsData | null>(null);
  const [activity, setActivity] = useState<ActivityEntry[]>([]);
  const [selectedSession, setSelectedSession] = useState<string | null>(null);
  const [selectedAgent, setSelectedAgent] = useState<string | null>(null);
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
    <div className="flex flex-1 h-full overflow-hidden">
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

        {/* Scrollable content */}
        <div className="flex-1 overflow-y-auto" style={{ scrollbarWidth: 'thin' }}>

          {/* ── Connectors ── */}
          {connectors && (() => {
            const wsChannels = connectors.channels;
            const wsTypes = new Set(wsChannels.map(c => c.channel || c.metadata?.channel));
            const httpConnectors = (connectors.spawned || []).filter(s => s.status === 'running' && !wsTypes.has(s.type));
            const allConnectors = [
              ...wsChannels.map(c => ({ type: c.channel || c.metadata?.channel || 'unknown', name: c.metadata?.bot_username ? `@${c.metadata.bot_username}` : '', source: 'ws' as const })),
              ...httpConnectors.map(s => ({ type: s.type, name: '', source: 'http' as const })),
            ];
            if (allConnectors.length === 0) return null;
            return (
              <div className="px-3 py-2">
                <div className="flex items-center gap-1.5 mb-2">
                  <Radio size={10} className="text-emerald-400" />
                  <span className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider">Connectors</span>
                  <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400 font-medium ml-auto">{allConnectors.length}</span>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {allConnectors.map((c, i) => {
                    const style = getChannelStyle(c.type);
                    const label = c.type.charAt(0).toUpperCase() + c.type.slice(1);
                    return (
                      <span key={i} className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-lg text-[10px] ${style.bg} ${style.text} border ${style.border}`}>
                        <span className={`w-1.5 h-1.5 rounded-full ${style.dot} animate-pulse`} />
                        <span className="font-medium">{label}</span>
                        {c.name && <span className="opacity-60">{c.name}</span>}
                      </span>
                    );
                  })}
                </div>
              </div>
            );
          })()}

          {/* ── Agents (compact badges) ── */}
          {(() => {
            const alive = agents.filter(a => a.status === 'running' || a.status === 'standby');
            if (alive.length === 0) return null;
            return (
              <div className="px-3 py-2 border-t border-zinc-700/20">
                <div className="flex items-center gap-1.5 mb-2">
                  <Bot size={10} className="text-violet-400" />
                  <span className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider">Agents</span>
                  <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-violet-500/15 text-violet-400 font-medium ml-auto">{alive.length}</span>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {alive.map(a => {
                    const isStandby = a.status === 'standby';
                    return (
                      <span key={a.id} className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-lg text-[10px] border ${
                        isStandby
                          ? 'bg-violet-500/10 text-violet-400 border-violet-500/20'
                          : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                      }`}>
                        <span className={`w-1.5 h-1.5 rounded-full animate-pulse ${isStandby ? 'bg-violet-400' : 'bg-emerald-400'}`} />
                        <span className="font-medium">{a.name}</span>
                        <span className="opacity-60">{a.status}</span>
                      </span>
                    );
                  })}
                </div>
              </div>
            );
          })()}

          {/* ── Conversations (connector sessions + agent chats unified) ── */}
          {(() => {
            const alive = agents.filter(a => a.status === 'running' || a.status === 'standby');
            const totalConversations = filteredSessions.length + alive.length;
            return (
              <>
                <div className="px-3 py-2 border-t border-zinc-700/20">
                  <div className="flex items-center gap-1.5">
                    <MessageSquare size={10} className="text-zinc-500" />
                    <span className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider">Conversations</span>
                    <span className="text-[10px] text-zinc-600 ml-auto">{totalConversations}</span>
                  </div>
                </div>

                {totalConversations === 0 ? (
                  <div className="flex flex-col items-center justify-center py-8 text-zinc-600 text-xs gap-1.5">
                    <Globe size={18} className="text-zinc-700" />
                    <span>{loading ? 'Loading...' : 'No conversations yet'}</span>
                  </div>
                ) : (
                  <div className="px-2 space-y-0.5">
                    {/* Agent conversations */}
                    {alive.map(a => {
                      const isSelected = selectedAgent === a.id;
                      const isStandby = a.status === 'standby';
                      return (
                        <button
                          key={`agent-${a.id}`}
                          onClick={() => { setSelectedAgent(a.id); setSelectedSession(null); }}
                          className={`w-full text-left px-2.5 py-2 rounded-lg transition-all ${
                            isSelected ? 'bg-violet-500/10 ring-1 ring-violet-500/30' : 'hover:bg-zinc-800/50'
                          }`}
                        >
                          <div className="flex items-center gap-2">
                            <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 animate-pulse ${isStandby ? 'bg-violet-400' : 'bg-emerald-400'}`} />
                            <span className={`text-xs font-medium ${isStandby ? 'text-violet-400' : 'text-emerald-400'}`}>{a.name}</span>
                            <span className={`ml-auto text-[9px] px-1.5 py-0.5 rounded-full ${isStandby ? 'bg-violet-500/15 text-violet-400' : 'bg-emerald-500/15 text-emerald-400'}`}>{a.status}</span>
                          </div>
                          <div className="pl-3.5 mt-0.5 text-[10px] text-zinc-600 truncate">{a.id}</div>
                        </button>
                      );
                    })}

                    {/* Connector sessions */}
                    {filteredSessions.map(s => {
                      const ch = s.channel || s.id.split(':')[0] || 'unknown';
                      const style = getChannelStyle(ch);
                      const isSelected = selectedSession === s.id;

                      return (
                        <button
                          key={s.id}
                          onClick={() => { setSelectedSession(s.id); setSelectedAgent(null); }}
                          className={`w-full text-left px-2.5 py-2 rounded-lg transition-all ${
                            isSelected ? 'bg-indigo-500/10 ring-1 ring-indigo-500/30' : 'hover:bg-zinc-800/50'
                          }`}
                        >
                          <div className="flex items-center gap-2">
                            <span className={`w-1.5 h-1.5 rounded-full ${style.dot} flex-shrink-0`} />
                            <span className={`text-xs font-medium capitalize ${style.text}`}>{ch}</span>
                            <span className="text-[10px] text-zinc-600 flex items-center gap-1 ml-auto">
                              <MessageSquare size={8} /> {s.message_count}
                            </span>
                          </div>
                          <div className="pl-3.5 mt-0.5 flex items-center gap-2">
                            <span className="text-[10px] text-zinc-500 truncate flex-1">{s.conversation_id || s.id}</span>
                            <span className="text-[9px] text-zinc-700">{s.last_activity ? relativeTime(s.last_activity) : ''}</span>
                          </div>
                        </button>
                      );
                    })}
                  </div>
                )}
              </>
            );
          })()}
        </div>
      </div>

      {/* Center panel -- Session chat, Agent live control, or Activity feed */}
      <div className="flex-1 flex flex-col min-w-0 h-full overflow-hidden">
        {selectedAgent ? (
          <AgentLivePanel
            agentId={selectedAgent}
            agents={agents}
            onClose={() => setSelectedAgent(null)}
          />
        ) : selectedSession ? (
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

            {/* Activity feed — chat-style layout */}
            <div className="flex-1 overflow-y-auto px-4 py-3" style={{ scrollbarWidth: 'thin' }}>
              {activity.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-48 text-zinc-500 text-sm gap-3">
                  <Activity size={28} className="text-zinc-600" />
                  <span>{loading ? 'Loading activity...' : 'No activity yet'}</span>
                  <span className="text-xs text-zinc-600">Messages from all connectors will appear here in real-time</span>
                </div>
              ) : (
                <div className="space-y-1">
                  {activity.map((entry, i) => {
                    const style = getChannelStyle(entry.channel);
                    const data = entry.data as Record<string, unknown>;
                    const isMessage = entry.type === 'event.message' || entry.type === 'message';
                    const isResponse = entry.type === 'cmd.channel.send' || entry.type === 'response';
                    const text = String(data.text || data.result || data.error || '').slice(0, 200);
                    const senderName = String((data.sender as any)?.username || (data.sender as any)?.name || '');
                    const channelLabel = entry.channel?.charAt(0).toUpperCase() + entry.channel?.slice(1);

                    // Show date separator when date changes
                    const prevEntry = i > 0 ? activity[i - 1] : null;
                    const curDate = new Date(entry.ts * 1000).toLocaleDateString();
                    const prevDate = prevEntry ? new Date(prevEntry.ts * 1000).toLocaleDateString() : '';
                    const showDateSep = curDate !== prevDate;

                    return (
                      <div key={`${entry.ts}-${i}`}>
                        {showDateSep && (
                          <div className="flex items-center gap-3 my-3">
                            <div className="flex-1 h-px bg-zinc-800" />
                            <span className="text-[10px] text-zinc-600 font-medium">{curDate}</span>
                            <div className="flex-1 h-px bg-zinc-800" />
                          </div>
                        )}
                        <button
                          onClick={() => setSelectedSession(entry.session_id)}
                          className={`w-full text-left flex gap-3 py-2 px-3 rounded-lg hover:bg-zinc-800/30 transition-all group ${
                            isResponse ? '' : ''
                          }`}
                        >
                          {/* Avatar / icon */}
                          <div className={`flex-shrink-0 w-8 h-8 rounded-lg flex items-center justify-center text-[10px] font-bold ${
                            isResponse
                              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                              : isMessage
                              ? `${style.bg} ${style.text} border ${style.border}`
                              : 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                          }`}>
                            {isResponse ? <Bot size={14} /> : isMessage ? <User size={14} /> : <Zap size={14} />}
                          </div>

                          {/* Content */}
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-0.5">
                              <span className={`text-xs font-semibold ${isResponse ? 'text-emerald-400' : isMessage ? 'text-zinc-200' : 'text-amber-400'}`}>
                                {isResponse ? 'SoulGate' : isMessage ? (senderName || 'User') : 'Tool'}
                              </span>
                              <span className={`text-[9px] px-1.5 py-0.5 rounded ${style.bg} ${style.text} font-medium`}>
                                {channelLabel}
                              </span>
                              <span className="ml-auto text-[10px] text-zinc-600 tabular-nums">{formatTs(entry.ts)}</span>
                            </div>
                            {text && (
                              <div className={`text-[13px] leading-relaxed ${isResponse ? 'text-zinc-400' : 'text-zinc-300'} line-clamp-2`}>
                                {text}
                              </div>
                            )}
                          </div>

                          <ChevronRight size={12} className="text-zinc-800 group-hover:text-zinc-500 mt-2.5 flex-shrink-0 transition-colors" />
                        </button>
                      </div>
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
