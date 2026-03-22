import { useState, useEffect } from 'react';
import { Bot, Plus, Play, Square, RefreshCw, Activity, AlertCircle, CheckCircle, Clock } from 'lucide-react';
import { fetchAgents, createAgent, type AgentData } from '../lib/api';
import { formatRelativeTime } from '../lib/utils';
import toast from 'react-hot-toast';

const STATUS_CONFIG: Record<AgentData['status'], { label: string; color: string; icon: React.ElementType }> = {
  running:   { label: 'Running',   color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20', icon: Activity },
  completed: { label: 'Completed', color: 'text-sky-400 bg-sky-500/10 border-sky-500/20',            icon: CheckCircle },
  stopped:   { label: 'Stopped',   color: 'text-zinc-400 bg-zinc-500/10 border-zinc-500/20',         icon: Square },
  error:     { label: 'Error',     color: 'text-red-400 bg-red-500/10 border-red-500/20',            icon: AlertCircle },
};

const ROLES = ['assistant', 'researcher', 'coder', 'analyst', 'writer', 'reviewer'];

function AgentCard({ agent, onClick }: { agent: AgentData; onClick: () => void }) {
  const cfg = STATUS_CONFIG[agent.status];
  const Icon = cfg.icon;

  return (
    <div
      onClick={onClick}
      className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 hover:border-zinc-600/60 cursor-pointer transition-all group"
    >
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-xl bg-indigo-500/20 border border-indigo-500/30 flex items-center justify-center">
            <Bot size={18} className="text-indigo-400" />
          </div>
          <div>
            <div className="font-semibold text-zinc-100 text-sm">{agent.name}</div>
            <div className="text-xs text-zinc-500 capitalize">{agent.role}</div>
          </div>
        </div>
        <span className={`flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-full border ${cfg.color}`}>
          <Icon size={11} />
          {cfg.label}
        </span>
      </div>
      <p className="text-sm text-zinc-400 mb-4 line-clamp-2">{agent.task}</p>
      <div className="flex items-center justify-between text-xs text-zinc-600">
        <span>{agent.message_count ?? 0} messages</span>
        <span>{agent.last_activity ? formatRelativeTime(agent.last_activity) : 'just created'}</span>
      </div>
    </div>
  );
}

function AgentDetail({ agent, onClose }: { agent: AgentData; onClose: () => void }) {
  const cfg = STATUS_CONFIG[agent.status];
  const Icon = cfg.icon;
  const [message, setMessage] = useState('');

  const send = () => {
    if (!message.trim()) return;
    toast.success(`Message sent to ${agent.name}`);
    setMessage('');
  };

  return (
    <div className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm flex justify-end" onClick={onClose}>
      <div
        className="w-full max-w-lg bg-zinc-900 border-l border-zinc-800 flex flex-col h-full overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800">
          <div className="flex items-center gap-3">
            <Bot size={18} className="text-indigo-400" />
            <div>
              <div className="font-semibold text-zinc-100">{agent.name}</div>
              <span className={`flex items-center gap-1 text-xs ${cfg.color.split(' ')[0]}`}>
                <Icon size={10} />{cfg.label}
              </span>
            </div>
          </div>
          <button onClick={onClose} className="text-zinc-600 hover:text-zinc-300 transition-colors text-lg leading-none">×</button>
        </div>

        <div className="px-6 py-4 border-b border-zinc-800/50 space-y-2">
          <div className="flex gap-2 text-xs">
            <span className="text-zinc-500">Role:</span>
            <span className="text-zinc-300 capitalize">{agent.role}</span>
          </div>
          <div className="flex gap-2 text-xs">
            <span className="text-zinc-500">Task:</span>
            <span className="text-zinc-300">{agent.task}</span>
          </div>
          <div className="flex gap-2 text-xs">
            <span className="text-zinc-500">Created:</span>
            <span className="text-zinc-300">{new Date(agent.created_at).toLocaleString()}</span>
          </div>
        </div>

        {/* Activity log */}
        <div className="flex-1 overflow-y-auto px-6 py-4" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
          <h4 className="text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-3">Activity Log</h4>
          {agent.log && agent.log.length > 0 ? (
            <div className="space-y-2">
              {agent.log.map((entry, i) => (
                <div key={i} className="flex gap-3 text-sm">
                  <Clock size={13} className="text-zinc-600 flex-shrink-0 mt-0.5" />
                  <span className="text-zinc-400">{entry}</span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-zinc-600">No activity yet.</p>
          )}
        </div>

        {/* Message input */}
        <div className="px-6 py-4 border-t border-zinc-800">
          <div className="flex gap-2">
            <input
              value={message}
              onChange={e => setMessage(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && send()}
              placeholder="Send message to agent…"
              className="flex-1 px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
            <button
              onClick={send}
              className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors"
            >
              Send
            </button>
          </div>
          <div className="flex gap-2 mt-2">
            <button
              onClick={() => { toast.success(`Delegated to ${agent.name}`); }}
              className="flex-1 py-1.5 rounded-lg border border-zinc-700 text-xs text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
            >
              Delegate Task
            </button>
            <button
              onClick={() => { toast.success('Agent stopped'); onClose(); }}
              className="flex-1 py-1.5 rounded-lg border border-zinc-700 text-xs text-zinc-400 hover:text-red-400 hover:border-red-500/40 transition-all"
            >
              Stop Agent
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function AgentsView() {
  const [agents, setAgents] = useState<AgentData[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<AgentData | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ name: '', task: '', role: 'assistant' });

  const load = async () => {
    setLoading(true);
    try {
      const data = await fetchAgents();
      if (data.length === 0) {
        // Demo data
        setAgents([
          { id: '1', name: 'Research Agent', role: 'researcher', task: 'Gather information on AI safety', status: 'running', created_at: new Date().toISOString(), last_activity: new Date().toISOString(), message_count: 12, log: ['Started research task', 'Found 3 relevant papers', 'Summarizing findings…'] },
          { id: '2', name: 'Code Reviewer', role: 'reviewer', task: 'Review pull request #42', status: 'completed', created_at: new Date(Date.now() - 3600000).toISOString(), last_activity: new Date(Date.now() - 1800000).toISOString(), message_count: 8, log: ['Reviewed 5 files', 'Found 2 issues', 'Posted comments'] },
          { id: '3', name: 'Data Analyst', role: 'analyst', task: 'Analyze Q4 metrics', status: 'stopped', created_at: new Date(Date.now() - 86400000).toISOString(), message_count: 3, log: [] },
        ]);
      } else {
        setAgents(data);
      }
    } catch {
      toast.error('Failed to load agents');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async () => {
    if (!form.name.trim() || !form.task.trim()) {
      toast.error('Name and task are required');
      return;
    }
    try {
      const agent = await createAgent(form);
      setAgents(prev => [...prev, agent]);
      setCreating(false);
      setForm({ name: '', task: '', role: 'assistant' });
      toast.success(`Agent "${agent.name}" created`);
    } catch {
      toast.error('Failed to create agent');
    }
  };

  const statusCounts = agents.reduce((acc, a) => {
    acc[a.status] = (acc[a.status] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Agents</h2>
          <p className="text-sm text-zinc-500">{agents.length} total · {statusCounts.running || 0} running</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={load}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
          >
            <RefreshCw size={14} />
            Refresh
          </button>
          <button
            onClick={() => setCreating(true)}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors"
          >
            <Plus size={14} />
            New Agent
          </button>
        </div>
      </div>

      {/* Create form */}
      {creating && (
        <div className="mb-6 p-5 rounded-xl border border-indigo-500/30 bg-indigo-500/5">
          <h3 className="text-sm font-semibold text-zinc-200 mb-4">Create New Agent</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-3">
            <input
              placeholder="Agent name"
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
              className="px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
            <select
              value={form.role}
              onChange={e => setForm(f => ({ ...f, role: e.target.value }))}
              className="px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 focus:outline-none focus:border-indigo-500/60"
            >
              {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
            </select>
            <div className="flex gap-2">
              <button
                onClick={handleCreate}
                className="flex-1 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors flex items-center justify-center gap-1.5"
              >
                <Play size={13} />
                Create
              </button>
              <button
                onClick={() => setCreating(false)}
                className="px-4 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 transition-all"
              >
                Cancel
              </button>
            </div>
          </div>
          <textarea
            placeholder="Describe the agent's task…"
            value={form.task}
            onChange={e => setForm(f => ({ ...f, task: e.target.value }))}
            rows={2}
            className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 resize-none"
          />
        </div>
      )}

      {/* Agents grid */}
      {loading ? (
        <div className="flex items-center gap-2 text-zinc-500">
          <Activity size={16} className="animate-spin" />
          Loading agents…
        </div>
      ) : agents.length === 0 ? (
        <div className="text-center py-16 text-zinc-500">
          <Bot size={40} className="mx-auto mb-3 opacity-30" />
          <p>No agents yet. Create your first agent.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {agents.map(agent => (
            <AgentCard key={agent.id} agent={agent} onClick={() => setSelected(agent)} />
          ))}
        </div>
      )}

      {selected && <AgentDetail agent={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}
