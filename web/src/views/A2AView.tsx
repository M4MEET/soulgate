import { useState, useEffect, useCallback } from 'react';
import { toast } from 'react-hot-toast';

const API = '/api/a2a';

interface AgentSkill {
  id: string;
  name: string;
  description: string;
  tags: string[];
}

interface RemoteAgent {
  url: string;
  card: {
    name: string;
    description: string;
    version: string;
    skills: AgentSkill[];
    capabilities: { streaming: boolean; pushNotifications: boolean };
  };
  added_at: string;
  last_seen: string;
  status: string;
  task_count: number;
}

interface TaskPart {
  type: string;
  text?: string;
  data?: unknown;
}

interface TaskMessage {
  messageId: string;
  role: string;
  parts: TaskPart[];
}

interface A2ATask {
  id: string;
  contextId: string;
  status: {
    state: string;
    message?: TaskMessage;
    timestamp: string;
  };
  artifacts: { artifactId: string; name: string; parts: TaskPart[] }[];
  history: TaskMessage[];
}

const stateColors: Record<string, string> = {
  submitted: 'bg-blue-500/20 text-blue-400',
  working: 'bg-yellow-500/20 text-yellow-400',
  completed: 'bg-green-500/20 text-green-400',
  failed: 'bg-red-500/20 text-red-400',
  canceled: 'bg-gray-500/20 text-gray-400',
  'input-required': 'bg-purple-500/20 text-purple-400',
  rejected: 'bg-red-500/20 text-red-400',
};

export default function A2AView() {
  const [tab, setTab] = useState<'agents' | 'tasks' | 'card'>('agents');
  const [agents, setAgents] = useState<RemoteAgent[]>([]);
  const [tasks, setTasks] = useState<A2ATask[]>([]);
  const [card, setCard] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(false);
  const [showAddAgent, setShowAddAgent] = useState(false);
  const [newAgentURL, setNewAgentURL] = useState('');
  const [showSendMessage, setShowSendMessage] = useState(false);
  const [sendTarget, setSendTarget] = useState('');
  const [sendMessage, setSendMessage] = useState('');
  const [selectedTask, setSelectedTask] = useState<A2ATask | null>(null);

  const fetchAgents = useCallback(async () => {
    try {
      const res = await fetch(`${API}/agents`);
      const data = await res.json();
      setAgents(data.agents || []);
    } catch { /* ignore */ }
  }, []);

  const fetchTasks = useCallback(async () => {
    try {
      const res = await fetch(`${API}/tasks`);
      const data = await res.json();
      setTasks(data.tasks || []);
    } catch { /* ignore */ }
  }, []);

  const fetchCard = useCallback(async () => {
    try {
      const res = await fetch(`${API}/card`);
      const data = await res.json();
      setCard(data);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    fetchAgents();
    fetchTasks();
    fetchCard();
    const interval = setInterval(() => {
      if (tab === 'agents') fetchAgents();
      if (tab === 'tasks') fetchTasks();
    }, 5000);
    return () => clearInterval(interval);
  }, [tab, fetchAgents, fetchTasks, fetchCard]);

  const addAgent = async () => {
    if (!newAgentURL) return;
    setLoading(true);
    try {
      const res = await fetch(`${API}/agents`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: newAgentURL }),
      });
      if (!res.ok) {
        const err = await res.json();
        toast.error(err.error || 'Failed to discover agent');
        return;
      }
      toast.success('Agent discovered and added');
      setNewAgentURL('');
      setShowAddAgent(false);
      fetchAgents();
    } catch {
      toast.error('Network error');
    } finally {
      setLoading(false);
    }
  };

  const removeAgent = async (url: string) => {
    await fetch(`${API}/agents/${encodeURIComponent(url)}`, { method: 'DELETE' });
    toast.success('Agent removed');
    fetchAgents();
  };

  const sendToAgent = async () => {
    if (!sendTarget || !sendMessage) return;
    setLoading(true);
    try {
      const res = await fetch(`${API}/send`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agentUrl: sendTarget, message: sendMessage }),
      });
      if (!res.ok) {
        const err = await res.json();
        toast.error(err.error || 'Failed to send');
        return;
      }
      toast.success('Message sent to remote agent');
      setSendMessage('');
      setShowSendMessage(false);
      fetchTasks();
    } catch {
      toast.error('Network error');
    } finally {
      setLoading(false);
    }
  };

  const cancelTask = async (taskId: string) => {
    await fetch(`${API}/tasks/${taskId}/cancel`, { method: 'POST' });
    toast.success('Task canceled');
    fetchTasks();
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">A2A Protocol</h1>
          <p className="text-sm text-zinc-400 mt-1">
            Agent-to-Agent communication — discover, connect, and delegate to remote AI agents
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowAddAgent(true)}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors"
          >
            + Add Agent
          </button>
          <button
            onClick={() => setShowSendMessage(true)}
            className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg text-sm font-medium transition-colors"
          >
            Send Message
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-zinc-800/50 p-1 rounded-lg w-fit">
        {(['agents', 'tasks', 'card'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
              tab === t ? 'bg-zinc-700 text-white' : 'text-zinc-400 hover:text-white'
            }`}
          >
            {t === 'agents' ? `Remote Agents (${agents.length})` : t === 'tasks' ? `Tasks (${tasks.length})` : 'My Agent Card'}
          </button>
        ))}
      </div>

      {/* Agents Tab */}
      {tab === 'agents' && (
        <div className="space-y-4">
          {agents.length === 0 ? (
            <div className="text-center py-12 text-zinc-500">
              <p className="text-lg">No remote agents connected</p>
              <p className="text-sm mt-1">Click "Add Agent" to discover and connect to a remote A2A agent</p>
            </div>
          ) : (
            agents.map((agent) => (
              <div key={agent.url} className="bg-zinc-800/50 border border-zinc-700 rounded-lg p-5">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3">
                      <h3 className="text-lg font-semibold text-white">{agent.card.name}</h3>
                      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                        agent.status === 'online' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                      }`}>
                        {agent.status}
                      </span>
                      <span className="text-xs text-zinc-500">v{agent.card.version}</span>
                    </div>
                    <p className="text-sm text-zinc-400 mt-1">{agent.card.description}</p>
                    <p className="text-xs text-zinc-500 mt-2 font-mono">{agent.url}</p>

                    {/* Capabilities */}
                    <div className="flex gap-2 mt-3">
                      {agent.card.capabilities?.streaming && (
                        <span className="px-2 py-0.5 bg-blue-500/10 text-blue-400 text-xs rounded">Streaming</span>
                      )}
                      {agent.card.capabilities?.pushNotifications && (
                        <span className="px-2 py-0.5 bg-purple-500/10 text-purple-400 text-xs rounded">Push</span>
                      )}
                    </div>

                    {/* Skills */}
                    {agent.card.skills?.length > 0 && (
                      <div className="mt-3">
                        <p className="text-xs text-zinc-500 mb-1">Skills:</p>
                        <div className="flex flex-wrap gap-1">
                          {agent.card.skills.map((skill) => (
                            <span key={skill.id} className="px-2 py-0.5 bg-zinc-700 text-zinc-300 text-xs rounded" title={skill.description}>
                              {skill.name}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => { setSendTarget(agent.url); setShowSendMessage(true); }}
                      className="px-3 py-1.5 bg-purple-600/20 text-purple-400 hover:bg-purple-600/30 rounded text-sm transition-colors"
                    >
                      Send
                    </button>
                    <button
                      onClick={() => removeAgent(agent.url)}
                      className="px-3 py-1.5 bg-red-600/20 text-red-400 hover:bg-red-600/30 rounded text-sm transition-colors"
                    >
                      Remove
                    </button>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {/* Tasks Tab */}
      {tab === 'tasks' && (
        <div className="space-y-3">
          {tasks.length === 0 ? (
            <div className="text-center py-12 text-zinc-500">
              <p className="text-lg">No A2A tasks</p>
              <p className="text-sm mt-1">Tasks appear when you send messages to remote agents or receive them</p>
            </div>
          ) : (
            tasks.map((task) => (
              <div
                key={task.id}
                onClick={() => setSelectedTask(selectedTask?.id === task.id ? null : task)}
                className={`bg-zinc-800/50 border rounded-lg p-4 cursor-pointer transition-colors ${
                  selectedTask?.id === task.id ? 'border-blue-500' : 'border-zinc-700 hover:border-zinc-600'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${stateColors[task.status.state] || 'bg-zinc-700 text-zinc-300'}`}>
                      {task.status.state}
                    </span>
                    <span className="text-sm text-white font-mono">{task.id.slice(0, 8)}...</span>
                    <span className="text-xs text-zinc-500">{new Date(task.status.timestamp).toLocaleString()}</span>
                  </div>
                  <div className="flex gap-2">
                    {!['completed', 'failed', 'canceled', 'rejected'].includes(task.status.state) && (
                      <button
                        onClick={(e) => { e.stopPropagation(); cancelTask(task.id); }}
                        className="px-3 py-1 bg-red-600/20 text-red-400 hover:bg-red-600/30 rounded text-xs"
                      >
                        Cancel
                      </button>
                    )}
                  </div>
                </div>

                {/* First message preview */}
                {task.history?.[0] && (
                  <p className="text-sm text-zinc-400 mt-2 truncate">
                    {task.history[0].parts?.map(p => p.text).filter(Boolean).join(' ')}
                  </p>
                )}

                {/* Expanded detail */}
                {selectedTask?.id === task.id && (
                  <div className="mt-4 border-t border-zinc-700 pt-4 space-y-3">
                    <div className="grid grid-cols-2 gap-2 text-xs">
                      <div><span className="text-zinc-500">Task ID:</span> <span className="text-zinc-300 font-mono">{task.id}</span></div>
                      <div><span className="text-zinc-500">Context:</span> <span className="text-zinc-300 font-mono">{task.contextId?.slice(0, 8)}...</span></div>
                    </div>

                    {/* History */}
                    {task.history?.length > 0 && (
                      <div>
                        <p className="text-xs text-zinc-500 mb-2">History ({task.history.length} messages):</p>
                        <div className="space-y-2 max-h-60 overflow-y-auto">
                          {task.history.map((msg, i) => (
                            <div key={i} className={`p-2 rounded text-sm ${
                              msg.role === 'user' ? 'bg-blue-900/20 border-l-2 border-blue-500' : 'bg-zinc-700/50 border-l-2 border-green-500'
                            }`}>
                              <span className="text-xs text-zinc-500">{msg.role}</span>
                              <p className="text-zinc-300 mt-0.5">{msg.parts?.map(p => p.text).filter(Boolean).join(' ')}</p>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Artifacts */}
                    {task.artifacts?.length > 0 && (
                      <div>
                        <p className="text-xs text-zinc-500 mb-2">Artifacts ({task.artifacts.length}):</p>
                        {task.artifacts.map((art) => (
                          <div key={art.artifactId} className="bg-zinc-700/30 p-2 rounded text-sm">
                            <span className="text-zinc-400">{art.name || art.artifactId}</span>
                            <pre className="text-zinc-300 mt-1 text-xs whitespace-pre-wrap">
                              {art.parts?.map(p => p.text).filter(Boolean).join('\n')}
                            </pre>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      )}

      {/* Card Tab */}
      {tab === 'card' && card && (
        <div className="bg-zinc-800/50 border border-zinc-700 rounded-lg p-5">
          <h3 className="text-lg font-semibold text-white mb-4">Your Agent Card</h3>
          <p className="text-xs text-zinc-500 mb-3">
            Served at <code className="bg-zinc-700 px-1.5 py-0.5 rounded">/.well-known/agent.json</code> for other agents to discover you
          </p>
          <pre className="bg-zinc-900 p-4 rounded-lg text-sm text-zinc-300 overflow-x-auto">
            {JSON.stringify(card, null, 2)}
          </pre>
        </div>
      )}

      {/* Add Agent Modal */}
      {showAddAgent && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowAddAgent(false)}>
          <div className="bg-zinc-800 border border-zinc-700 rounded-xl p-6 w-full max-w-md" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-white mb-4">Discover Remote Agent</h3>
            <p className="text-sm text-zinc-400 mb-4">
              Enter the base URL of an A2A-compatible agent. We'll fetch its agent card to discover its capabilities.
            </p>
            <input
              type="text"
              value={newAgentURL}
              onChange={(e) => setNewAgentURL(e.target.value)}
              placeholder="https://agent.example.com"
              className="w-full bg-zinc-900 border border-zinc-600 rounded-lg px-4 py-2.5 text-white placeholder-zinc-500 focus:border-blue-500 focus:outline-none"
              onKeyDown={(e) => e.key === 'Enter' && addAgent()}
            />
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setShowAddAgent(false)} className="px-4 py-2 text-zinc-400 hover:text-white text-sm">Cancel</button>
              <button onClick={addAgent} disabled={loading} className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm disabled:opacity-50">
                {loading ? 'Discovering...' : 'Discover'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Send Message Modal */}
      {showSendMessage && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowSendMessage(false)}>
          <div className="bg-zinc-800 border border-zinc-700 rounded-xl p-6 w-full max-w-lg" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-white mb-4">Send Message to Remote Agent</h3>
            <div className="space-y-3">
              <div>
                <label className="text-xs text-zinc-400 mb-1 block">Agent URL</label>
                {agents.length > 0 ? (
                  <select
                    value={sendTarget}
                    onChange={(e) => setSendTarget(e.target.value)}
                    className="w-full bg-zinc-900 border border-zinc-600 rounded-lg px-4 py-2.5 text-white focus:border-blue-500 focus:outline-none"
                  >
                    <option value="">Select an agent...</option>
                    {agents.map(a => (
                      <option key={a.url} value={a.url}>{a.card.name} ({a.url})</option>
                    ))}
                  </select>
                ) : (
                  <input
                    type="text"
                    value={sendTarget}
                    onChange={(e) => setSendTarget(e.target.value)}
                    placeholder="https://agent.example.com"
                    className="w-full bg-zinc-900 border border-zinc-600 rounded-lg px-4 py-2.5 text-white placeholder-zinc-500 focus:border-blue-500 focus:outline-none"
                  />
                )}
              </div>
              <div>
                <label className="text-xs text-zinc-400 mb-1 block">Message</label>
                <textarea
                  value={sendMessage}
                  onChange={(e) => setSendMessage(e.target.value)}
                  placeholder="What do you want the remote agent to do?"
                  rows={4}
                  className="w-full bg-zinc-900 border border-zinc-600 rounded-lg px-4 py-2.5 text-white placeholder-zinc-500 focus:border-blue-500 focus:outline-none resize-none"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setShowSendMessage(false)} className="px-4 py-2 text-zinc-400 hover:text-white text-sm">Cancel</button>
              <button onClick={sendToAgent} disabled={loading || !sendTarget || !sendMessage} className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg text-sm disabled:opacity-50">
                {loading ? 'Sending...' : 'Send'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
