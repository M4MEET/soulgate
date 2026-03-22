import { useState, useEffect } from 'react';
import {
  Bot, Plus, Play, Square, RefreshCw, Activity, AlertCircle, CheckCircle,
  Clock, Sparkles, Code, Search, Shield, FileText, Terminal, Globe,
  GitBranch, Database, Zap, ChevronRight, MessageSquare,
  Trash2, Pause, Send,
} from 'lucide-react';
import { fetchAgents, createAgent, type AgentData } from '../lib/api';
import { formatRelativeTime } from '../lib/utils';
import toast from 'react-hot-toast';

const STATUS_CONFIG: Record<AgentData['status'], { label: string; color: string; icon: React.ElementType }> = {
  running:   { label: 'Running',   color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20', icon: Activity },
  completed: { label: 'Completed', color: 'text-sky-400 bg-sky-500/10 border-sky-500/20',            icon: CheckCircle },
  stopped:   { label: 'Stopped',   color: 'text-zinc-400 bg-zinc-500/10 border-zinc-500/20',         icon: Square },
  error:     { label: 'Error',     color: 'text-red-400 bg-red-500/10 border-red-500/20',            icon: AlertCircle },
};

// ── Agent Templates & Suggestions ────────────────────────────────────────────

interface AgentTemplate {
  name: string;
  role: string;
  task: string;
  icon: React.ElementType;
  color: string;
  description: string;
  capabilities: string[];
  suggestedTasks: string[];
}

const AGENT_TEMPLATES: AgentTemplate[] = [
  {
    name: 'Code Assistant',
    role: 'coder',
    task: 'Help with coding tasks — write, review, debug, and refactor code',
    icon: Code,
    color: 'text-blue-400 bg-blue-500/10 border-blue-500/20',
    description: 'Expert programmer that writes, reviews, and fixes code',
    capabilities: ['Write code in any language', 'Debug errors', 'Refactor & optimize', 'Create tests', 'Apply patches'],
    suggestedTasks: [
      'Review the codebase and suggest improvements',
      'Write unit tests for untested functions',
      'Refactor duplicated code into shared utilities',
      'Fix all TODO comments in the project',
      'Add error handling to all API endpoints',
    ],
  },
  {
    name: 'Research Agent',
    role: 'research',
    task: 'Research topics, summarize findings, and compile reports',
    icon: Search,
    color: 'text-violet-400 bg-violet-500/10 border-violet-500/20',
    description: 'Deep researcher that finds, analyzes, and summarizes information',
    capabilities: ['Web search', 'Read documents & PDFs', 'Summarize findings', 'Compare options', 'Write reports'],
    suggestedTasks: [
      'Research competitors and create a comparison table',
      'Find the best practices for our tech stack',
      'Summarize the latest changes in our dependency licenses',
      'Research and recommend a CI/CD pipeline setup',
      'Find security vulnerabilities in our dependencies',
    ],
  },
  {
    name: 'DevOps Agent',
    role: 'ops',
    task: 'Manage infrastructure, deployments, and system health',
    icon: Terminal,
    color: 'text-amber-400 bg-amber-500/10 border-amber-500/20',
    description: 'Operations expert for infrastructure, deployment, and monitoring',
    capabilities: ['Run shell commands', 'Manage processes', 'Monitor system health', 'Docker operations', 'Git workflows'],
    suggestedTasks: [
      'Check system health and report any issues',
      'Clean up old Docker images and containers',
      'Set up automated backups for the database',
      'Monitor disk space and alert if below 10%',
      'Update all npm/go dependencies to latest versions',
    ],
  },
  {
    name: 'Content Writer',
    role: 'general',
    task: 'Write documentation, blog posts, emails, and other content',
    icon: FileText,
    color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
    description: 'Skilled writer for docs, blogs, emails, and marketing copy',
    capabilities: ['Write documentation', 'Create blog posts', 'Draft emails', 'Edit & proofread', 'Generate README files'],
    suggestedTasks: [
      'Write a getting-started guide for new developers',
      'Create API documentation for all endpoints',
      'Draft a changelog for the latest release',
      'Write a blog post about our architecture',
      'Create onboarding documentation for new team members',
    ],
  },
  {
    name: 'Security Auditor',
    role: 'research',
    task: 'Audit code and infrastructure for security vulnerabilities',
    icon: Shield,
    color: 'text-red-400 bg-red-500/10 border-red-500/20',
    description: 'Security expert that finds vulnerabilities and recommends fixes',
    capabilities: ['Code scanning', 'Dependency audit', 'Config review', 'Permission analysis', 'Compliance checks'],
    suggestedTasks: [
      'Scan the codebase for hardcoded secrets',
      'Audit file permissions on sensitive configs',
      'Check all API endpoints for authentication gaps',
      'Review policy rules for overly permissive access',
      'Verify all user inputs are properly sanitized',
    ],
  },
  {
    name: 'Data Pipeline',
    role: 'ops',
    task: 'Process, transform, and analyze data files',
    icon: Database,
    color: 'text-cyan-400 bg-cyan-500/10 border-cyan-500/20',
    description: 'Data engineer for ETL, analysis, and reporting',
    capabilities: ['Read/write CSV & JSON', 'Data transformation', 'Statistical analysis', 'Generate charts', 'Automate reports'],
    suggestedTasks: [
      'Parse audit logs and generate a daily summary report',
      'Analyze API response times and find slow endpoints',
      'Export session data to CSV for analysis',
      'Calculate cost trends and project next month spend',
      'Clean and deduplicate memory entries',
    ],
  },
  {
    name: 'Git Workflow',
    role: 'coder',
    task: 'Manage git operations — branches, commits, PRs, reviews',
    icon: GitBranch,
    color: 'text-orange-400 bg-orange-500/10 border-orange-500/20',
    description: 'Git expert for branch management, PRs, and code review',
    capabilities: ['Git status & diff', 'Branch management', 'Commit & push', 'Stash operations', 'Merge conflict resolution'],
    suggestedTasks: [
      'Create a feature branch and commit current changes',
      'Review git history for the last week of changes',
      'Find and clean up stale branches',
      'Generate a changelog from recent commits',
      'Check for uncommitted changes across all repos',
    ],
  },
  {
    name: 'Web Scout',
    role: 'research',
    task: 'Browse the web, fetch pages, and extract information',
    icon: Globe,
    color: 'text-teal-400 bg-teal-500/10 border-teal-500/20',
    description: 'Web crawler that fetches, parses, and summarizes web content',
    capabilities: ['Web search', 'Fetch & parse URLs', 'Browser automation', 'Screenshot pages', 'Extract structured data'],
    suggestedTasks: [
      'Search for the latest news about our industry',
      'Monitor a competitor website for changes',
      'Fetch and summarize a research paper',
      'Check if our website is responding correctly',
      'Find and list all broken links on our docs site',
    ],
  },
];

const ROLES = ['general', 'coder', 'research', 'ops'];

// ── Components ───────────────────────────────────────────────────────────────

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

function TemplateCard({ template, onUse }: { template: AgentTemplate; onUse: (t: AgentTemplate) => void }) {
  const [expanded, setExpanded] = useState(false);
  const Icon = template.icon;

  return (
    <div className="rounded-xl bg-zinc-800/30 border border-zinc-700/40 overflow-hidden transition-all hover:border-zinc-600/60">
      <div
        className="flex items-center gap-3 p-4 cursor-pointer"
        onClick={() => setExpanded(!expanded)}
      >
        <div className={`w-9 h-9 rounded-xl flex items-center justify-center border ${template.color}`}>
          <Icon size={18} />
        </div>
        <div className="flex-1 min-w-0">
          <div className="font-semibold text-sm text-zinc-200">{template.name}</div>
          <div className="text-xs text-zinc-500">{template.description}</div>
        </div>
        <ChevronRight size={14} className={`text-zinc-600 transition-transform ${expanded ? 'rotate-90' : ''}`} />
      </div>

      {expanded && (
        <div className="px-4 pb-4 space-y-3 border-t border-zinc-800/60 pt-3">
          {/* Capabilities */}
          <div>
            <div className="text-xs font-medium text-zinc-500 mb-1.5">Capabilities</div>
            <div className="flex flex-wrap gap-1.5">
              {template.capabilities.map(cap => (
                <span key={cap} className="text-xs px-2 py-0.5 rounded-full bg-zinc-700/50 text-zinc-400">
                  {cap}
                </span>
              ))}
            </div>
          </div>

          {/* Suggested Tasks */}
          <div>
            <div className="text-xs font-medium text-zinc-500 mb-1.5">Suggested Tasks</div>
            <div className="space-y-1">
              {template.suggestedTasks.map(task => (
                <button
                  key={task}
                  onClick={(e) => {
                    e.stopPropagation();
                    onUse({ ...template, task });
                  }}
                  className="flex items-center gap-2 w-full text-left px-2.5 py-1.5 rounded-lg text-xs text-zinc-400 hover:text-zinc-200 hover:bg-zinc-700/40 transition-all group"
                >
                  <Zap size={11} className="text-zinc-600 group-hover:text-amber-400 flex-shrink-0" />
                  <span className="flex-1">{task}</span>
                  <Play size={10} className="text-zinc-700 group-hover:text-indigo-400 flex-shrink-0" />
                </button>
              ))}
            </div>
          </div>

          {/* Use Template Button */}
          <button
            onClick={(e) => { e.stopPropagation(); onUse(template); }}
            className="w-full py-2 rounded-lg bg-indigo-600/20 border border-indigo-500/30 text-indigo-400 text-xs font-medium hover:bg-indigo-600/30 transition-all flex items-center justify-center gap-1.5"
          >
            <Plus size={13} />
            Create {template.name}
          </button>
        </div>
      )}
    </div>
  );
}

function AgentDetail({ agent, onClose, onRefresh }: { agent: AgentData; onClose: () => void; onRefresh: () => void }) {
  const cfg = STATUS_CONFIG[agent.status];
  const Icon = cfg.icon;
  const [message, setMessage] = useState('');

  // Find matching template for suggestions
  const matchingTemplate = AGENT_TEMPLATES.find(t =>
    t.role === agent.role || t.name.toLowerCase().includes(agent.role)
  );

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
        {/* Header */}
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
          <button onClick={onClose} className="text-zinc-600 hover:text-zinc-300 transition-colors text-lg">×</button>
        </div>

        {/* Info */}
        <div className="px-6 py-4 border-b border-zinc-800/50 space-y-2">
          <div className="flex gap-2 text-xs"><span className="text-zinc-500 w-16">Role:</span><span className="text-zinc-300 capitalize">{agent.role}</span></div>
          <div className="flex gap-2 text-xs"><span className="text-zinc-500 w-16">Task:</span><span className="text-zinc-300">{agent.task}</span></div>
          <div className="flex gap-2 text-xs"><span className="text-zinc-500 w-16">Created:</span><span className="text-zinc-300">{new Date(agent.created_at).toLocaleString()}</span></div>
          <div className="flex gap-2 text-xs"><span className="text-zinc-500 w-16">Messages:</span><span className="text-zinc-300">{agent.message_count ?? 0}</span></div>
        </div>

        {/* Suggested next actions */}
        {matchingTemplate && (
          <div className="px-6 py-3 border-b border-zinc-800/50">
            <div className="text-xs font-medium text-zinc-500 mb-2 flex items-center gap-1.5">
              <Sparkles size={11} className="text-amber-400" />
              Suggested next actions
            </div>
            <div className="space-y-1">
              {matchingTemplate.suggestedTasks.slice(0, 3).map(task => (
                <button
                  key={task}
                  onClick={() => { setMessage(task); }}
                  className="flex items-center gap-2 w-full text-left px-2 py-1.5 rounded-lg text-xs text-zinc-400 hover:text-zinc-200 hover:bg-zinc-700/40 transition-all"
                >
                  <ChevronRight size={10} className="text-zinc-600 flex-shrink-0" />
                  <span className="flex-1 line-clamp-1">{task}</span>
                </button>
              ))}
            </div>
          </div>
        )}

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

        {/* Actions + message */}
        <div className="px-6 py-4 border-t border-zinc-800 space-y-2">
          <div className="flex gap-2">
            <input
              value={message}
              onChange={e => setMessage(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && send()}
              placeholder="Send message or task to agent..."
              className="flex-1 px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
            <button onClick={send} className="px-3 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white transition-colors">
              <Send size={14} />
            </button>
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => toast.success(`Delegated to ${agent.name}`)}
              className="flex-1 py-1.5 rounded-lg border border-zinc-700 text-xs text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all flex items-center justify-center gap-1.5"
            >
              <MessageSquare size={11} /> Delegate
            </button>
            <button
              onClick={() => toast.success('Agent paused')}
              className="flex-1 py-1.5 rounded-lg border border-zinc-700 text-xs text-zinc-400 hover:text-amber-400 hover:border-amber-500/40 transition-all flex items-center justify-center gap-1.5"
            >
              <Pause size={11} /> Pause
            </button>
            <button
              onClick={() => { toast.success('Agent stopped'); onClose(); onRefresh(); }}
              className="flex-1 py-1.5 rounded-lg border border-zinc-700 text-xs text-zinc-400 hover:text-red-400 hover:border-red-500/40 transition-all flex items-center justify-center gap-1.5"
            >
              <Trash2 size={11} /> Stop
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ── Main View ────────────────────────────────────────────────────────────────

export default function AgentsView() {
  const [agents, setAgents] = useState<AgentData[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<AgentData | null>(null);
  const [creating, setCreating] = useState(false);
  const [showTemplates, setShowTemplates] = useState(true);
  const [form, setForm] = useState({ name: '', task: '', role: 'general' });

  const load = async () => {
    setLoading(true);
    try {
      const data = await fetchAgents();
      setAgents(data);
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
      setForm({ name: '', task: '', role: 'general' });
      toast.success(`Agent "${form.name}" created`);
    } catch {
      toast.error('Failed to create agent');
    }
  };

  const useTemplate = (template: AgentTemplate) => {
    setForm({ name: template.name, task: template.task, role: template.role });
    setCreating(true);
    setShowTemplates(false);
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
            onClick={() => setShowTemplates(s => !s)}
            className={`flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm transition-all ${
              showTemplates
                ? 'bg-amber-500/15 text-amber-400 border border-amber-500/30'
                : 'border border-zinc-700 text-zinc-400 hover:text-zinc-200 hover:border-zinc-600'
            }`}
          >
            <Sparkles size={14} />
            Templates
          </button>
          <button
            onClick={load}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
          >
            <RefreshCw size={14} />
          </button>
          <button
            onClick={() => { setCreating(true); setShowTemplates(false); }}
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
              <button onClick={handleCreate} className="flex-1 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors flex items-center justify-center gap-1.5">
                <Play size={13} /> Create
              </button>
              <button onClick={() => setCreating(false)} className="px-4 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 transition-all">
                Cancel
              </button>
            </div>
          </div>
          <textarea
            placeholder="Describe the agent's task..."
            value={form.task}
            onChange={e => setForm(f => ({ ...f, task: e.target.value }))}
            rows={2}
            className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 resize-none"
          />
        </div>
      )}

      {/* Agent Templates */}
      {showTemplates && !creating && (
        <div className="mb-6">
          <div className="flex items-center gap-2 mb-3">
            <Sparkles size={14} className="text-amber-400" />
            <h3 className="text-sm font-semibold text-zinc-300">Agent Templates</h3>
            <span className="text-xs text-zinc-600">Click to expand, then pick a task or create</span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {AGENT_TEMPLATES.map(t => (
              <TemplateCard key={t.name} template={t} onUse={useTemplate} />
            ))}
          </div>
        </div>
      )}

      {/* Active Agents */}
      {agents.length > 0 && (
        <div className="mb-4">
          <h3 className="text-sm font-semibold text-zinc-400 mb-3">Active Agents ({agents.length})</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {agents.map(agent => (
              <AgentCard key={agent.id} agent={agent} onClick={() => setSelected(agent)} />
            ))}
          </div>
        </div>
      )}

      {/* Empty state */}
      {!loading && agents.length === 0 && !showTemplates && !creating && (
        <div className="text-center py-16 text-zinc-500">
          <Bot size={40} className="mx-auto mb-3 opacity-30" />
          <p className="mb-2">No agents running.</p>
          <button
            onClick={() => setShowTemplates(true)}
            className="text-indigo-400 hover:text-indigo-300 text-sm transition-colors"
          >
            Browse templates to get started
          </button>
        </div>
      )}

      {loading && (
        <div className="flex items-center gap-2 text-zinc-500 py-8">
          <Activity size={16} className="animate-spin" /> Loading agents...
        </div>
      )}

      {selected && <AgentDetail agent={selected} onClose={() => setSelected(null)} onRefresh={load} />}
    </div>
  );
}
