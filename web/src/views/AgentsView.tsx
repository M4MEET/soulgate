import { useState, useEffect, useRef, useCallback } from 'react';
import {
  Bot, Plus, Play, Square, RefreshCw, Activity, AlertCircle, CheckCircle,
  Clock, Sparkles, Code, Search, Shield, FileText, Terminal, Globe,
  GitBranch, Database, Zap, ChevronRight, MessageSquare,
  Pause, Send, ArrowLeft, Settings, BarChart2,
  Filter, SortAsc, X, Cpu, DollarSign, Hash, Timer,
  ToggleLeft, ToggleRight, PauseCircle, Radio, Trash2, ExternalLink,
} from 'lucide-react';
import {
  LineChart, Line, BarChart, Bar, XAxis, YAxis, Tooltip,
  ResponsiveContainer, CartesianGrid,
} from 'recharts';
import {
  fetchAgents, createAgent, fetchAgentDetail, fetchAgentLog,
  fetchAgentMessages, updateAgentConfig, sendAgentMessage,
  stopAgent, pauseAgent, restartAgent, fetchTools,
  type AgentData, type AgentDetailData, type AgentConfig,
  type AgentLogEntry, type AgentMessage,
} from '../lib/api';
import { formatRelativeTime, formatCost } from '../lib/utils';
import toast from 'react-hot-toast';

// ── Status config ─────────────────────────────────────────────────────────────

const STATUS_CONFIG: Record<AgentData['status'], { label: string; color: string; dot: string; icon: React.ElementType }> = {
  running:   { label: 'Running',   color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20', dot: 'bg-emerald-400',  icon: Activity },
  completed: { label: 'Completed', color: 'text-sky-400 bg-sky-500/10 border-sky-500/20',            dot: 'bg-sky-400',      icon: CheckCircle },
  stopped:   { label: 'Stopped',   color: 'text-zinc-400 bg-zinc-500/10 border-zinc-500/20',         dot: 'bg-zinc-400',     icon: Square },
  error:     { label: 'Error',     color: 'text-red-400 bg-red-500/10 border-red-500/20',            dot: 'bg-red-400',      icon: AlertCircle },
};

// ── Log entry types ──────────────────────────────────────────────────────────

const LOG_TYPE_CONFIG: Record<string, { color: string; bg: string; label: string; icon: React.ElementType }> = {
  model_call:       { color: 'text-violet-400', bg: 'bg-violet-500/10', label: 'Model Call',  icon: Cpu },
  tool_start:       { color: 'text-amber-400',  bg: 'bg-amber-500/10',  label: 'Tool Start',  icon: Play },
  tool_done:        { color: 'text-emerald-400',bg: 'bg-emerald-500/10',label: 'Tool Done',   icon: CheckCircle },
  error:            { color: 'text-red-400',    bg: 'bg-red-500/10',    label: 'Error',       icon: AlertCircle },
  status:           { color: 'text-zinc-400',   bg: 'bg-zinc-500/10',   label: 'Status',      icon: Activity },
  message_received: { color: 'text-blue-400',   bg: 'bg-blue-500/10',   label: 'Message',     icon: MessageSquare },
};

function getLogTypeCfg(type: string) {
  return LOG_TYPE_CONFIG[type] ?? { color: 'text-zinc-400', bg: 'bg-zinc-500/10', label: type, icon: Clock };
}

// ── Agent Templates ───────────────────────────────────────────────────────────

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
    name: 'Code Assistant', role: 'coder',
    task: 'Help with coding tasks — write, review, debug, and refactor code',
    icon: Code, color: 'text-blue-400 bg-blue-500/10 border-blue-500/20',
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
    name: 'Research Agent', role: 'research',
    task: 'Research topics, summarize findings, and compile reports',
    icon: Search, color: 'text-violet-400 bg-violet-500/10 border-violet-500/20',
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
    name: 'DevOps Agent', role: 'ops',
    task: 'Manage infrastructure, deployments, and system health',
    icon: Terminal, color: 'text-amber-400 bg-amber-500/10 border-amber-500/20',
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
    name: 'Content Writer', role: 'general',
    task: 'Write documentation, blog posts, emails, and other content',
    icon: FileText, color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
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
    name: 'Security Auditor', role: 'research',
    task: 'Audit code and infrastructure for security vulnerabilities',
    icon: Shield, color: 'text-red-400 bg-red-500/10 border-red-500/20',
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
    name: 'Data Pipeline', role: 'ops',
    task: 'Process, transform, and analyze data files',
    icon: Database, color: 'text-cyan-400 bg-cyan-500/10 border-cyan-500/20',
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
    name: 'Git Workflow', role: 'coder',
    task: 'Manage git operations — branches, commits, PRs, reviews',
    icon: GitBranch, color: 'text-orange-400 bg-orange-500/10 border-orange-500/20',
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
    name: 'Web Scout', role: 'research',
    task: 'Browse the web, fetch pages, and extract information',
    icon: Globe, color: 'text-teal-400 bg-teal-500/10 border-teal-500/20',
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

const MODELS = [
  'claude-opus-4-5', 'claude-sonnet-4-5', 'claude-haiku-3-5',
  'gpt-4.1', 'gpt-4.1-mini', 'gpt-4o', 'gpt-4o-mini',
  'o1', 'o3-mini',
];
const PROVIDERS = ['anthropic', 'openai'];
const THINKING_LEVELS = ['off', 'low', 'medium', 'high'];

// ── Tooltip styles shared ─────────────────────────────────────────────────────
const CHART_TOOLTIP_STYLE = {
  contentStyle: { background: '#18181b', border: '1px solid #3f3f46', borderRadius: 8, fontSize: 12 },
  labelStyle: { color: '#a1a1aa' },
};

// ── Small reusable pieces ─────────────────────────────────────────────────────

function StatusBadge({ status }: { status: AgentData['status'] }) {
  const cfg = STATUS_CONFIG[status];
  const Icon = cfg.icon;
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-full border font-medium ${cfg.color}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot} ${status === 'running' ? 'animate-pulse' : ''}`} />
      <Icon size={10} />
      {cfg.label}
    </span>
  );
}

function MetricCard({
  icon: Icon, label, value, sub, color = 'text-indigo-400',
}: { icon: React.ElementType; label: string; value: string | number; sub?: string; color?: string }) {
  return (
    <div className="flex flex-col gap-1 p-4 rounded-xl bg-zinc-800/40 border border-zinc-700/40">
      <div className="flex items-center gap-2">
        <Icon size={14} className={color} />
        <span className="text-xs text-zinc-500 uppercase tracking-wide">{label}</span>
      </div>
      <div className="text-xl font-bold text-zinc-100">{value}</div>
      {sub && <div className="text-xs text-zinc-600">{sub}</div>}
    </div>
  );
}

// ── Template components ───────────────────────────────────────────────────────

function TemplateCard({ template, onUse }: { template: AgentTemplate; onUse: (t: AgentTemplate) => void }) {
  const [expanded, setExpanded] = useState(false);
  const Icon = template.icon;

  return (
    <div className="rounded-xl bg-zinc-800/30 border border-zinc-700/40 overflow-hidden transition-all hover:border-zinc-600/60">
      <div className="flex items-center gap-3 p-4 cursor-pointer" onClick={() => setExpanded(!expanded)}>
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
          <div>
            <div className="text-xs font-medium text-zinc-500 mb-1.5">Capabilities</div>
            <div className="flex flex-wrap gap-1.5">
              {template.capabilities.map(cap => (
                <span key={cap} className="text-xs px-2 py-0.5 rounded-full bg-zinc-700/50 text-zinc-400">{cap}</span>
              ))}
            </div>
          </div>
          <div>
            <div className="text-xs font-medium text-zinc-500 mb-1.5">Suggested Tasks</div>
            <div className="space-y-1">
              {template.suggestedTasks.map(task => (
                <button
                  key={task}
                  onClick={(e) => { e.stopPropagation(); onUse({ ...template, task }); }}
                  className="flex items-center gap-2 w-full text-left px-2.5 py-1.5 rounded-lg text-xs text-zinc-400 hover:text-zinc-200 hover:bg-zinc-700/40 transition-all group"
                >
                  <Zap size={11} className="text-zinc-600 group-hover:text-amber-400 flex-shrink-0" />
                  <span className="flex-1">{task}</span>
                  <Play size={10} className="text-zinc-700 group-hover:text-indigo-400 flex-shrink-0" />
                </button>
              ))}
            </div>
          </div>
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

// ── Agent List Card ──────────────────────────────────────────────────────────

function AgentCard({
  agent, onClick,
}: { agent: AgentData; onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      className="group relative p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 hover:border-zinc-600/60 cursor-pointer transition-all"
    >
      {/* Quick action overlay */}
      <div className="absolute top-3 right-3 hidden group-hover:flex items-center gap-1 z-10">
        <button
          onClick={e => { e.stopPropagation(); pauseAgent(agent.id).then(() => toast.success('Agent paused')); }}
          className="p-1.5 rounded-lg bg-zinc-700/80 hover:bg-amber-500/20 text-zinc-400 hover:text-amber-400 transition-all"
          title="Pause"
        >
          <PauseCircle size={12} />
        </button>
        <button
          onClick={e => { e.stopPropagation(); stopAgent(agent.id).then(() => toast.success('Agent stopped')); }}
          className="p-1.5 rounded-lg bg-zinc-700/80 hover:bg-red-500/20 text-zinc-400 hover:text-red-400 transition-all"
          title="Stop"
        >
          <Square size={12} />
        </button>
      </div>

      <div className="flex items-start gap-3 mb-3">
        <div className="w-9 h-9 rounded-xl bg-indigo-500/20 border border-indigo-500/30 flex items-center justify-center flex-shrink-0">
          <Bot size={18} className="text-indigo-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="font-semibold text-zinc-100 text-sm truncate">{agent.name}</div>
          <div className="text-xs text-zinc-500 capitalize">{agent.role}</div>
        </div>
      </div>

      <p className="text-sm text-zinc-400 mb-3 line-clamp-2">{agent.task}</p>

      <div className="flex items-center justify-between">
        <StatusBadge status={agent.status} />
        <span className="text-xs text-zinc-600">
          {agent.last_activity ? formatRelativeTime(agent.last_activity) : 'just created'}
        </span>
      </div>

      <div className="mt-3 pt-3 border-t border-zinc-700/30 flex items-center gap-4 text-xs text-zinc-600">
        <span className="flex items-center gap-1">
          <MessageSquare size={10} />
          {agent.message_count ?? 0}
        </span>
        <span className="flex items-center gap-1">
          <Hash size={10} />
          {agent.id.slice(0, 8)}
        </span>
      </div>
    </div>
  );
}

// ── Detail Page: Overview Tab ────────────────────────────────────────────────

function OverviewTab({ detail }: { detail: AgentDetailData }) {
  const m = detail.metrics;
  const created = new Date(detail.created_at);

  return (
    <div className="space-y-5">
      {/* Status + basic info */}
      <div className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-4">
        <div className="flex items-start justify-between gap-3 flex-wrap">
          <div>
            <div className="text-sm font-semibold text-zinc-200 mb-1">{detail.task}</div>
            <div className="flex items-center gap-2 flex-wrap">
              <StatusBadge status={detail.status} />
              <span className="text-xs px-2 py-0.5 rounded-full bg-zinc-700/50 border border-zinc-600/30 text-zinc-400 capitalize">
                {detail.role}
              </span>
            </div>
          </div>
          <div className="text-right text-xs text-zinc-500 space-y-0.5">
            <div>Created {created.toLocaleString()}</div>
            {detail.metrics.duration && <div>Duration: {detail.metrics.duration}</div>}
            {detail.last_activity && <div>Last active: {formatRelativeTime(detail.last_activity)}</div>}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3 text-xs">
          <div className="flex gap-2">
            <span className="text-zinc-500 w-20 flex-shrink-0">Model:</span>
            <span className="text-zinc-300 font-mono">{detail.config.model}</span>
          </div>
          <div className="flex gap-2">
            <span className="text-zinc-500 w-20 flex-shrink-0">Provider:</span>
            <span className="text-zinc-300 capitalize">{detail.config.provider}</span>
          </div>
          {detail.parent_id && (
            <div className="flex gap-2">
              <span className="text-zinc-500 w-20 flex-shrink-0">Parent:</span>
              <button className="text-indigo-400 hover:underline font-mono">{detail.parent_id.slice(0, 12)}…</button>
            </div>
          )}
          {detail.child_ids && detail.child_ids.length > 0 && (
            <div className="flex gap-2">
              <span className="text-zinc-500 w-20 flex-shrink-0">Children:</span>
              <span className="text-zinc-300">{detail.child_ids.length} sub-agents</span>
            </div>
          )}
        </div>
      </div>

      {/* Metrics row */}
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        <MetricCard icon={Hash}       label="Tokens Used"    value={m.tokens_used.toLocaleString()}         color="text-violet-400" />
        <MetricCard icon={DollarSign} label="Cost"           value={formatCost(m.cost_usd)}                 color="text-emerald-400" />
        <MetricCard icon={Zap}        label="Tool Calls"     value={m.tool_call_count}                       color="text-amber-400" />
        <MetricCard icon={Cpu}        label="Model Calls"    value={m.model_call_count}                      color="text-sky-400" />
        <MetricCard icon={AlertCircle}label="Errors"         value={m.error_count}                           color="text-red-400" />
        <MetricCard icon={Timer}      label="Avg Response"   value={`${m.avg_response_ms}ms`}                color="text-indigo-400" />
      </div>
    </div>
  );
}

// ── Detail Page: Activity Tab ────────────────────────────────────────────────

function ActivityTab({ agentId }: { agentId: string }) {
  const [entries, setEntries] = useState<AgentLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [paused, setPaused] = useState(false);
  const [filter, setFilter] = useState<string>('all');
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const loadLog = useCallback(async () => {
    const data = await fetchAgentLog(agentId, 100);
    setEntries(data);
  }, [agentId]);

  useEffect(() => {
    setLoading(true);
    loadLog().finally(() => setLoading(false));
  }, [loadLog]);

  // Poll every 2 seconds
  useEffect(() => {
    if (paused) return;
    const id = setInterval(() => loadLog(), 2000);
    return () => clearInterval(id);
  }, [paused, loadLog]);

  // Auto-scroll to bottom unless paused
  useEffect(() => {
    if (!paused && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [entries, paused]);

  const allTypes = Array.from(new Set(entries.map(e => e.type)));
  const visible = filter === 'all' ? entries : entries.filter(e => e.type === filter);

  return (
    <div className="flex flex-col h-full gap-3">
      {/* Controls */}
      <div className="flex items-center gap-2 flex-wrap flex-shrink-0">
        <div className="flex items-center gap-1 p-0.5 rounded-lg bg-zinc-800/60 border border-zinc-700/40">
          <button
            onClick={() => setFilter('all')}
            className={`px-2.5 py-1 rounded-md text-xs transition-all ${filter === 'all' ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'}`}
          >
            All
          </button>
          {allTypes.map(t => {
            const cfg = getLogTypeCfg(t);
            return (
              <button
                key={t}
                onClick={() => setFilter(t)}
                className={`px-2.5 py-1 rounded-md text-xs transition-all ${filter === t ? `bg-zinc-700 ${cfg.color}` : 'text-zinc-500 hover:text-zinc-300'}`}
              >
                {cfg.label}
              </button>
            );
          })}
        </div>
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-zinc-600">{visible.length} entries</span>
          <button
            onClick={() => setPaused(p => !p)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs border transition-all ${
              paused
                ? 'bg-amber-500/15 border-amber-500/30 text-amber-400'
                : 'border-zinc-700 text-zinc-400 hover:text-zinc-200'
            }`}
          >
            {paused ? <Play size={11} /> : <Pause size={11} />}
            {paused ? 'Resume' : 'Pause'}
          </button>
        </div>
      </div>

      {/* Log area */}
      <div
        ref={containerRef}
        className="flex-1 overflow-y-auto rounded-xl bg-zinc-900/50 border border-zinc-700/40 p-3 space-y-1 min-h-0"
        style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
      >
        {loading && (
          <div className="flex items-center gap-2 text-zinc-500 text-sm py-4">
            <Activity size={14} className="animate-spin" /> Loading activity…
          </div>
        )}
        {!loading && visible.length === 0 && (
          <p className="text-sm text-zinc-600 py-4 text-center">No activity entries.</p>
        )}
        {visible.map((entry, i) => {
          const cfg = getLogTypeCfg(entry.type);
          const EntryIcon = cfg.icon;
          return (
            <div key={i} className="flex items-start gap-2.5 py-1.5 px-2 rounded-lg hover:bg-zinc-800/40 transition-colors">
              <span className={`mt-0.5 p-1 rounded-md ${cfg.bg} flex-shrink-0`}>
                <EntryIcon size={11} className={cfg.color} />
              </span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className={`text-xs font-medium ${cfg.color}`}>{cfg.label}</span>
                  <span className="text-xs text-zinc-600">
                    {new Date(entry.timestamp).toLocaleTimeString()}
                  </span>
                </div>
                <div className="text-sm text-zinc-300 mt-0.5 break-words">{entry.message}</div>
                {entry.metadata && Object.keys(entry.metadata).length > 0 && (
                  <div className="mt-1 text-xs text-zinc-600 font-mono truncate">
                    {JSON.stringify(entry.metadata)}
                  </div>
                )}
              </div>
            </div>
          );
        })}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}

// ── Detail Page: Configuration Tab ──────────────────────────────────────────

function ConfigurationTab({ detail, onSaved }: { detail: AgentDetailData; onSaved: () => void; }) {
  // Normalize null fields to safe defaults
  const safeConfig: AgentConfig = {
    model: detail.config?.model || '',
    provider: detail.config?.provider || '',
    allowed_tools: detail.config?.allowed_tools || [],
    max_tokens: detail.config?.max_tokens || 0,
    max_cost_usd: detail.config?.max_cost_usd || 0,
    thinking_level: detail.config?.thinking_level || 'off',
    temperature: detail.config?.temperature || 0.7,
    system_prompt: detail.config?.system_prompt || '',
    timeout_seconds: detail.config?.timeout_seconds || 0,
    auto_restart: detail.config?.auto_restart || false,
    schedule_enabled: (detail.config as any)?.schedule_enabled || false,
    schedule_cron: (detail.config as any)?.schedule_cron || '',
  };
  const [cfg, setCfg] = useState<AgentConfig>(safeConfig);
  const [saving, setSaving] = useState(false);
  const [availableTools, setAvailableTools] = useState<string[]>(['files_read', 'files_write', 'exec_command', 'web_search', 'files_list', 'web_fetch', 'git_status']);

  const m = detail.metrics;
  const tokenPct = Math.min((m.tokens_used / (cfg.max_tokens || 8192)) * 100, 100);
  const costPct = Math.min((m.cost_usd / (cfg.max_cost_usd || 1)) * 100, 100);

  useEffect(() => {
    fetchTools().then(tools => {
      const names = tools.map(t => t.name);
      if (names.length > 0) setAvailableTools(names);
    });
  }, []);

  const save = async () => {
    setSaving(true);
    try {
      await updateAgentConfig(detail.id, cfg);
      toast.success('Configuration saved');
      onSaved();
    } catch {
      toast.error('Failed to save configuration');
    } finally {
      setSaving(false);
    }
  };

  const toggleTool = (tool: string) => {
    setCfg(c => {
      const current = c.allowed_tools || [];
      return {
        ...c,
        allowed_tools: current.includes(tool)
          ? current.filter(t => t !== tool)
          : [...current, tool],
      };
    });
  };

  return (
    <div className="space-y-5">
      {/* Model & Provider */}
      <section className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-4">
        <h4 className="text-sm font-semibold text-zinc-200">Model</h4>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs text-zinc-500 mb-1.5">Model</label>
            <select
              value={cfg.model}
              onChange={e => setCfg(c => ({ ...c, model: e.target.value }))}
              className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-100 focus:outline-none focus:border-indigo-500/60"
            >
              {MODELS.map(m => <option key={m} value={m}>{m}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs text-zinc-500 mb-1.5">Provider</label>
            <select
              value={cfg.provider}
              onChange={e => setCfg(c => ({ ...c, provider: e.target.value }))}
              className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-100 focus:outline-none focus:border-indigo-500/60"
            >
              {PROVIDERS.map(p => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
        </div>
      </section>

      {/* Budgets */}
      <section className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-4">
        <h4 className="text-sm font-semibold text-zinc-200">Budgets</h4>
        <div className="space-y-3">
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-xs text-zinc-500">Token Budget</label>
              <span className="text-xs text-zinc-400">{m.tokens_used.toLocaleString()} / {cfg.max_tokens.toLocaleString()}</span>
            </div>
            <input
              type="number"
              value={cfg.max_tokens}
              onChange={e => setCfg(c => ({ ...c, max_tokens: Number(e.target.value) }))}
              className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-100 focus:outline-none focus:border-indigo-500/60 mb-2"
            />
            <div className="h-1.5 bg-zinc-700 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${tokenPct > 90 ? 'bg-red-500' : tokenPct > 70 ? 'bg-amber-500' : 'bg-indigo-500'}`}
                style={{ width: `${tokenPct}%` }}
              />
            </div>
          </div>
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-xs text-zinc-500">Cost Limit (USD)</label>
              <span className="text-xs text-zinc-400">{formatCost(m.cost_usd)} / {formatCost(cfg.max_cost_usd)}</span>
            </div>
            <input
              type="number"
              step="0.01"
              value={cfg.max_cost_usd}
              onChange={e => setCfg(c => ({ ...c, max_cost_usd: Number(e.target.value) }))}
              className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-100 focus:outline-none focus:border-indigo-500/60 mb-2"
            />
            <div className="h-1.5 bg-zinc-700 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${costPct > 90 ? 'bg-red-500' : costPct > 70 ? 'bg-amber-500' : 'bg-emerald-500'}`}
                style={{ width: `${costPct}%` }}
              />
            </div>
          </div>
        </div>
      </section>

      {/* Generation settings */}
      <section className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-4">
        <h4 className="text-sm font-semibold text-zinc-200">Generation</h4>
        <div>
          <label className="block text-xs text-zinc-500 mb-2">Thinking Level</label>
          <div className="flex gap-2">
            {THINKING_LEVELS.map(level => (
              <button
                key={level}
                onClick={() => setCfg(c => ({ ...c, thinking_level: level }))}
                className={`flex-1 py-1.5 rounded-lg text-xs border capitalize transition-all ${
                  cfg.thinking_level === level
                    ? 'bg-indigo-600/25 border-indigo-500/40 text-indigo-300'
                    : 'border-zinc-700 text-zinc-500 hover:text-zinc-300 hover:border-zinc-600'
                }`}
              >
                {level}
              </button>
            ))}
          </div>
        </div>
        <div>
          <div className="flex items-center justify-between mb-1.5">
            <label className="text-xs text-zinc-500">Temperature</label>
            <span className="text-xs text-zinc-400 font-mono">{cfg.temperature.toFixed(2)}</span>
          </div>
          <input
            type="range"
            min={0} max={2} step={0.05}
            value={cfg.temperature}
            onChange={e => setCfg(c => ({ ...c, temperature: Number(e.target.value) }))}
            className="w-full accent-indigo-500"
          />
          <div className="flex justify-between text-xs text-zinc-600 mt-0.5">
            <span>0.0 (deterministic)</span>
            <span>2.0 (creative)</span>
          </div>
        </div>
      </section>

      {/* Allowed Tools */}
      <section className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-3">
        <h4 className="text-sm font-semibold text-zinc-200">Allowed Tools</h4>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
          {availableTools.map(tool => (
            <label key={tool} className="flex items-center gap-2 cursor-pointer group">
              <input
                type="checkbox"
                checked={cfg.allowed_tools.includes(tool)}
                onChange={() => toggleTool(tool)}
                className="w-3.5 h-3.5 accent-indigo-500 rounded"
              />
              <span className="text-xs text-zinc-400 group-hover:text-zinc-200 transition-colors font-mono">{tool}</span>
            </label>
          ))}
        </div>
      </section>

      {/* System prompt */}
      <section className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-3">
        <h4 className="text-sm font-semibold text-zinc-200">System Prompt</h4>
        <textarea
          rows={4}
          value={cfg.system_prompt}
          onChange={e => setCfg(c => ({ ...c, system_prompt: e.target.value }))}
          placeholder="Optional system prompt override…"
          className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 resize-none font-mono"
        />
      </section>

      {/* Runtime */}
      <section className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-4">
        <h4 className="text-sm font-semibold text-zinc-200">Runtime</h4>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs text-zinc-500 mb-1.5">Timeout (seconds)</label>
            <input
              type="number"
              value={cfg.timeout_seconds}
              onChange={e => setCfg(c => ({ ...c, timeout_seconds: Number(e.target.value) }))}
              className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-100 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
          <div className="flex items-center gap-3 pt-5">
            <button
              onClick={() => setCfg(c => ({ ...c, auto_restart: !c.auto_restart }))}
              className="flex items-center gap-2 text-sm"
            >
              {cfg.auto_restart
                ? <ToggleRight size={22} className="text-indigo-400" />
                : <ToggleLeft size={22} className="text-zinc-600" />}
              <span className={cfg.auto_restart ? 'text-zinc-200' : 'text-zinc-500'}>
                Auto-restart on failure
              </span>
            </button>
          </div>
        </div>
      </section>

      {/* Scheduling */}
      <section className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-4">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-semibold text-zinc-200">Scheduled Runs</h4>
          <button
            onClick={() => setCfg(c => ({ ...c, schedule_enabled: !c.schedule_enabled }))}
            className="flex items-center gap-2 text-sm"
          >
            {cfg.schedule_enabled
              ? <ToggleRight size={22} className="text-emerald-400" />
              : <ToggleLeft size={22} className="text-zinc-600" />}
            <span className={cfg.schedule_enabled ? 'text-emerald-400 text-xs' : 'text-zinc-500 text-xs'}>
              {cfg.schedule_enabled ? 'Enabled' : 'Disabled'}
            </span>
          </button>
        </div>

        {cfg.schedule_enabled && (
          <div className="space-y-4">
            <div>
              <label className="block text-xs text-zinc-500 mb-1.5">Schedule Preset</label>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
                {[
                  { label: 'Every 15 min', value: '*/15 * * * *' },
                  { label: 'Every hour', value: '0 * * * *' },
                  { label: 'Every 6 hours', value: '0 */6 * * *' },
                  { label: 'Every day 9am', value: '0 9 * * *' },
                  { label: 'Every Monday', value: '0 9 * * 1' },
                  { label: 'Custom', value: 'custom' },
                ].map(preset => (
                  <button
                    key={preset.value}
                    onClick={() => {
                      if (preset.value !== 'custom') {
                        setCfg(c => ({ ...c, schedule_cron: preset.value }));
                      }
                    }}
                    className={`px-3 py-2 rounded-lg text-xs transition-all ${
                      cfg.schedule_cron === preset.value
                        ? 'bg-indigo-500/15 border border-indigo-500/30 text-indigo-400'
                        : 'bg-zinc-900 border border-zinc-700 text-zinc-400 hover:text-zinc-200 hover:border-zinc-600'
                    }`}
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label className="block text-xs text-zinc-500 mb-1.5">Cron Expression</label>
              <input
                value={cfg.schedule_cron || ''}
                onChange={e => setCfg(c => ({ ...c, schedule_cron: e.target.value }))}
                placeholder="0 */6 * * *"
                className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 font-mono"
              />
              <p className="text-xs text-zinc-600 mt-1">
                Format: minute hour day month weekday — e.g., "0 9 * * 1-5" = weekdays at 9am
              </p>
            </div>

            <div className="flex items-center gap-3 p-3 rounded-lg bg-zinc-900/50 border border-zinc-700/40">
              <div className="flex-1">
                <div className="text-xs text-zinc-400">On each scheduled run, the agent will execute its configured task</div>
                <div className="text-xs text-zinc-600 mt-1">
                  Schedule: {cfg.schedule_cron || 'not set'}
                </div>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => toast.success('Agent run triggered manually')}
                  className="px-3 py-1.5 rounded-lg bg-emerald-600/20 border border-emerald-500/30 text-emerald-400 text-xs hover:bg-emerald-600/30 transition-all"
                >
                  Run Now
                </button>
              </div>
            </div>
          </div>
        )}
      </section>

      {/* Agent Controls */}
      <section className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40 space-y-3">
        <h4 className="text-sm font-semibold text-zinc-200">Controls</h4>
        <div className="grid grid-cols-3 gap-3">
          <button
            onClick={() => toast.success('Agent started')}
            className="py-2.5 rounded-lg bg-emerald-600/20 border border-emerald-500/30 text-emerald-400 text-xs font-medium hover:bg-emerald-600/30 transition-all flex items-center justify-center gap-1.5"
          >
            <Play size={13} /> Start
          </button>
          <button
            onClick={() => toast.success('Agent stopped')}
            className="py-2.5 rounded-lg bg-amber-600/20 border border-amber-500/30 text-amber-400 text-xs font-medium hover:bg-amber-600/30 transition-all flex items-center justify-center gap-1.5"
          >
            <Square size={13} /> Stop
          </button>
          <button
            onClick={async () => {
              try {
                const result = await restartAgent(detail.id);
                toast.success(`Agent restarted (new ID: ${result.new_id})`);
                onSaved();
              } catch (err: unknown) {
                toast.error(err instanceof Error ? err.message : 'Restart failed');
              }
            }}
            className="py-2.5 rounded-lg bg-sky-600/20 border border-sky-500/30 text-sky-400 text-xs font-medium hover:bg-sky-600/30 transition-all flex items-center justify-center gap-1.5"
          >
            <RefreshCw size={13} /> Restart
          </button>
        </div>
      </section>

      <button
        onClick={save}
        disabled={saving}
        className="w-full py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
      >
        {saving ? <Activity size={14} className="animate-spin" /> : <Settings size={14} />}
        {saving ? 'Saving…' : 'Save Configuration'}
      </button>
    </div>
  );
}

// ── Detail Page: Messages Tab ────────────────────────────────────────────────

function MessagesTab({ detail }: { detail: AgentDetailData }) {
  const [messages, setMessages] = useState<AgentMessage[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    fetchAgentMessages(detail.id).then(setMessages);
  }, [detail.id]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const send = async () => {
    const text = input.trim();
    if (!text) return;
    setInput('');
    const optimistic: AgentMessage = {
      id: `opt-${Date.now()}`,
      role: 'user',
      content: text,
      timestamp: new Date().toISOString(),
    };
    setMessages(prev => [...prev, optimistic]);
    setSending(true);
    try {
      await sendAgentMessage(detail.id, text);
      toast.success('Message sent');
    } catch {
      toast.error('Failed to send message');
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="flex flex-col h-full gap-0 min-h-0">
      {/* Message list */}
      <div
        className="flex-1 overflow-y-auto p-3 space-y-3 min-h-0"
        style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
      >
        {messages.length === 0 && (
          <div className="text-center py-12 text-zinc-600 text-sm">
            No messages yet. Send one below.
          </div>
        )}
        {messages.map(msg => (
          <div key={msg.id} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
            {msg.role !== 'user' && (
              <div className="w-7 h-7 rounded-full bg-indigo-500/20 border border-indigo-500/30 flex items-center justify-center mr-2 flex-shrink-0 mt-0.5">
                <Bot size={13} className="text-indigo-400" />
              </div>
            )}
            <div className={`max-w-xs md:max-w-md lg:max-w-lg ${msg.role === 'user' ? 'items-end' : 'items-start'} flex flex-col gap-1`}>
              <div className={`px-3.5 py-2.5 rounded-2xl text-sm leading-relaxed ${
                msg.role === 'user'
                  ? 'bg-indigo-600 text-white rounded-br-sm'
                  : 'bg-zinc-800 text-zinc-200 border border-zinc-700/50 rounded-bl-sm'
              }`}>
                {msg.content}
              </div>
              <span className="text-xs text-zinc-600 px-1">
                {new Date(msg.timestamp).toLocaleTimeString()}
              </span>
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div className="flex-shrink-0 pt-3 border-t border-zinc-800">
        <div className="flex gap-2">
          <input
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && !e.shiftKey && send()}
            placeholder="Send a message or task to this agent…"
            disabled={sending}
            className="flex-1 px-3 py-2.5 rounded-xl bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 disabled:opacity-50"
          />
          <button
            onClick={send}
            disabled={sending || !input.trim()}
            className="px-4 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white transition-colors disabled:opacity-40"
          >
            {sending ? <Activity size={14} className="animate-spin" /> : <Send size={14} />}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Detail Page: Metrics Tab ─────────────────────────────────────────────────

function MetricsTab({ detail }: { detail: AgentDetailData }) {
  const m = detail.metrics;
  const tokenHistory = m.token_history ?? [];
  const toolCalls = m.tool_calls ?? [];
  const responseTimes = m.response_times ?? [];

  return (
    <div className="space-y-5">
      {/* Token usage over time */}
      <div className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40">
        <h4 className="text-sm font-semibold text-zinc-300 mb-4">Token Usage Over Time</h4>
        {tokenHistory.length > 0 ? (
          <ResponsiveContainer width="100%" height={160}>
            <LineChart data={tokenHistory}>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
              <XAxis dataKey="time" tick={{ fill: '#71717a', fontSize: 10 }} axisLine={false} tickLine={false}
                tickFormatter={v => new Date(v).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })} />
              <YAxis tick={{ fill: '#71717a', fontSize: 10 }} axisLine={false} tickLine={false} />
              <Tooltip {...CHART_TOOLTIP_STYLE} itemStyle={{ color: '#818cf8' }} />
              <Line type="monotone" dataKey="tokens" stroke="#6366f1" strokeWidth={2} dot={false} name="Tokens" />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <p className="text-sm text-zinc-600 py-8 text-center">No token history available</p>
        )}
      </div>

      {/* Cost accumulation */}
      <div className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40">
        <h4 className="text-sm font-semibold text-zinc-300 mb-4">Cost Accumulation</h4>
        {tokenHistory.length > 0 ? (
          <ResponsiveContainer width="100%" height={130}>
            <LineChart data={tokenHistory.map((d, i) => ({
              ...d,
              cumCost: tokenHistory.slice(0, i + 1).reduce((s, x) => s + x.cost, 0),
            }))}>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
              <XAxis dataKey="time" tick={{ fill: '#71717a', fontSize: 10 }} axisLine={false} tickLine={false}
                tickFormatter={v => new Date(v).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })} />
              <YAxis tick={{ fill: '#71717a', fontSize: 10 }} axisLine={false} tickLine={false}
                tickFormatter={v => `$${v.toFixed(3)}`} />
              <Tooltip {...CHART_TOOLTIP_STYLE} itemStyle={{ color: '#34d399' }}
                formatter={(v: unknown) => [`$${(v as number).toFixed(5)}`, 'Cumulative Cost']} />
              <Line type="monotone" dataKey="cumCost" stroke="#34d399" strokeWidth={2} dot={false} name="Cost" />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <p className="text-sm text-zinc-600 py-8 text-center">No cost data available</p>
        )}
      </div>

      {/* Tool call frequency */}
      {toolCalls.length > 0 && (
        <div className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40">
          <h4 className="text-sm font-semibold text-zinc-300 mb-4">Tool Call Frequency</h4>
          <ResponsiveContainer width="100%" height={120}>
            <BarChart data={toolCalls} layout="vertical">
              <XAxis type="number" tick={{ fill: '#71717a', fontSize: 10 }} axisLine={false} tickLine={false} />
              <YAxis dataKey="name" type="category" tick={{ fill: '#71717a', fontSize: 10 }} axisLine={false} tickLine={false} width={80} />
              <Tooltip {...CHART_TOOLTIP_STYLE} itemStyle={{ color: '#fbbf24' }} />
              <Bar dataKey="count" fill="#f59e0b" radius={[0, 3, 3, 0]} name="Calls" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Response time distribution */}
      {responseTimes.length > 0 && (
        <div className="p-5 rounded-xl bg-zinc-800/40 border border-zinc-700/40">
          <h4 className="text-sm font-semibold text-zinc-300 mb-4">Response Time Distribution</h4>
          <ResponsiveContainer width="100%" height={120}>
            <BarChart data={responseTimes}>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
              <XAxis dataKey="bucket" tick={{ fill: '#71717a', fontSize: 10 }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fill: '#71717a', fontSize: 10 }} axisLine={false} tickLine={false} />
              <Tooltip {...CHART_TOOLTIP_STYLE} itemStyle={{ color: '#38bdf8' }} />
              <Bar dataKey="count" fill="#38bdf8" radius={[3, 3, 0, 0]} name="Requests" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Summary stats */}
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        <MetricCard icon={Hash}        label="Total Tokens"   value={m.tokens_used.toLocaleString()}  color="text-violet-400" />
        <MetricCard icon={DollarSign}  label="Total Cost"     value={formatCost(m.cost_usd)}           color="text-emerald-400" />
        <MetricCard icon={Zap}         label="Tool Calls"     value={m.tool_call_count}                color="text-amber-400" />
        <MetricCard icon={Cpu}         label="Model Calls"    value={m.model_call_count}               color="text-sky-400" />
        <MetricCard icon={AlertCircle} label="Errors"         value={m.error_count}                    color="text-red-400" />
        <MetricCard icon={Timer}       label="Avg Response"   value={`${m.avg_response_ms}ms`}         color="text-indigo-400" />
      </div>
    </div>
  );
}

// ── Agent Detail Full Page ───────────────────────────────────────────────────

type DetailTab = 'overview' | 'activity' | 'configuration' | 'messages' | 'metrics';

const DETAIL_TABS: { id: DetailTab; label: string; icon: React.ElementType }[] = [
  { id: 'overview',       label: 'Overview',      icon: Bot },
  { id: 'activity',       label: 'Activity',       icon: Activity },
  { id: 'configuration',  label: 'Configuration',  icon: Settings },
  { id: 'messages',       label: 'Messages',       icon: MessageSquare },
  { id: 'metrics',        label: 'Metrics',        icon: BarChart2 },
];

function AgentDetailPage({
  agentId,
  onBack,
}: { agentId: string; onBack: () => void }) {
  const [detail, setDetail] = useState<AgentDetailData | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<DetailTab>('overview');

  const load = useCallback(async () => {
    const data = await fetchAgentDetail(agentId);
    setDetail(data);
  }, [agentId]);

  useEffect(() => {
    setLoading(true);
    load().finally(() => setLoading(false));
  }, [load]);

  const handleStop = async () => {
    await stopAgent(agentId);
    toast.success('Agent stopped');
    await load();
  };

  const handlePause = async () => {
    await pauseAgent(agentId);
    toast.success('Agent paused');
    await load();
  };

  if (loading) {
    return (
      <div className="flex-1 flex items-center justify-center text-zinc-500 gap-2">
        <Activity size={16} className="animate-spin" />
        Loading agent…
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-zinc-500 gap-3">
        <AlertCircle size={24} />
        <p>Agent not found</p>
        <button onClick={onBack} className="text-indigo-400 hover:text-indigo-300 text-sm">Go back</button>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Header */}
      <div className="flex-shrink-0 px-6 py-4 border-b border-zinc-800 bg-zinc-950">
        <div className="flex items-center gap-3 mb-4">
          <button
            onClick={onBack}
            className="flex items-center gap-1.5 text-zinc-500 hover:text-zinc-200 text-sm transition-colors"
          >
            <ArrowLeft size={15} />
            Back
          </button>
          <span className="text-zinc-700">/</span>
          <span className="text-sm text-zinc-400">Agents</span>
          <span className="text-zinc-700">/</span>
          <span className="text-sm text-zinc-200 font-medium">{detail.name}</span>
        </div>

        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-indigo-500/20 border border-indigo-500/30 flex items-center justify-center">
              <Bot size={20} className="text-indigo-400" />
            </div>
            <div>
              <div className="text-lg font-bold text-zinc-100">{detail.name}</div>
              <div className="flex items-center gap-2 mt-0.5">
                <StatusBadge status={detail.status} />
                <span className="text-xs text-zinc-500 capitalize">{detail.role}</span>
                <span className="text-xs text-zinc-600 font-mono">{detail.id.slice(0, 12)}…</span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={handlePause}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-zinc-700 text-xs text-zinc-400 hover:text-amber-400 hover:border-amber-500/40 transition-all"
            >
              <Pause size={12} />
              Pause
            </button>
            <button
              onClick={handleStop}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-zinc-700 text-xs text-zinc-400 hover:text-red-400 hover:border-red-500/40 transition-all"
            >
              <Square size={12} />
              Stop
            </button>
            <button
              onClick={load}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-zinc-700 text-xs text-zinc-400 hover:text-zinc-200 transition-all"
            >
              <RefreshCw size={12} />
            </button>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-0.5 mt-4 overflow-x-auto" style={{ scrollbarWidth: 'none' }}>
          {DETAIL_TABS.map(t => {
            const TabIcon = t.icon;
            return (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={`flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-sm whitespace-nowrap transition-all ${
                  tab === t.id
                    ? 'bg-zinc-800 text-zinc-100 font-medium'
                    : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50'
                }`}
              >
                <TabIcon size={13} />
                {t.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Tab content */}
      <div
        className="flex-1 overflow-y-auto p-6"
        style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
      >
        {tab === 'overview'      && <OverviewTab detail={detail} />}
        {tab === 'activity'      && <ActivityTab agentId={detail.id} />}
        {tab === 'configuration' && <ConfigurationTab detail={detail} onSaved={load} />}
        {tab === 'messages'      && <MessagesTab detail={detail} />}
        {tab === 'metrics'       && <MetricsTab detail={detail} />}
      </div>
    </div>
  );
}

// ── Main Agents View ─────────────────────────────────────────────────────────

type MainTab = 'agents' | 'templates' | 'a2a';
type SortKey = 'created' | 'name' | 'status';

export default function AgentsView() {
  const [agents, setAgents] = useState<AgentData[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [mainTab, setMainTab] = useState<MainTab>('agents');
  const [form, setForm] = useState({ name: '', task: '', role: 'general' });

  // Filter / sort state
  const [search, setSearch] = useState('');
  const [filterStatus, setFilterStatus] = useState<string>('all');
  const [filterRole, setFilterRole] = useState<string>('all');
  const [sortKey, setSortKey] = useState<SortKey>('created');

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
    setMainTab('agents');
  };

  // Derived: filtered + sorted agents
  const visibleAgents = agents
    .filter(a => {
      if (filterStatus !== 'all' && a.status !== filterStatus) return false;
      if (filterRole !== 'all' && a.role !== filterRole) return false;
      if (search) {
        const q = search.toLowerCase();
        if (!a.name.toLowerCase().includes(q) && !a.task.toLowerCase().includes(q)) return false;
      }
      return true;
    })
    .sort((a, b) => {
      if (sortKey === 'name') return a.name.localeCompare(b.name);
      if (sortKey === 'status') return a.status.localeCompare(b.status);
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
    });

  const statusCounts = agents.reduce((acc, a) => {
    acc[a.status] = (acc[a.status] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  const allRoles = Array.from(new Set(agents.map(a => a.role)));

  // If an agent is selected, show its detail page
  if (selectedId) {
    return (
      <AgentDetailPage
        agentId={selectedId}
        onBack={() => setSelectedId(null)}
      />
    );
  }

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5 flex-wrap gap-3">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Agents</h2>
          <p className="text-sm text-zinc-500">
            {agents.length} total
            {statusCounts.running ? ` · ${statusCounts.running} running` : ''}
            {statusCounts.error ? ` · ${statusCounts.error} errored` : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={load} className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all">
            <RefreshCw size={14} />
          </button>
          <button
            onClick={() => { setCreating(true); setMainTab('agents'); }}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors"
          >
            <Plus size={14} />
            New Agent
          </button>
        </div>
      </div>

      {/* Main tabs */}
      <div className="flex gap-0.5 mb-5 p-0.5 rounded-lg bg-zinc-800/40 border border-zinc-700/40 w-fit">
        {(['agents', 'templates', 'a2a'] as const).map(t => (
          <button
            key={t}
            onClick={() => setMainTab(t)}
            className={`px-4 py-1.5 rounded-md text-sm capitalize transition-all ${
              mainTab === t ? 'bg-zinc-700 text-zinc-100 font-medium' : 'text-zinc-500 hover:text-zinc-300'
            }`}
          >
            {t === 'templates' && <span className="inline-flex items-center gap-1.5"><Sparkles size={12} className="text-amber-400" />Templates</span>}
            {t === 'agents' && <span className="inline-flex items-center gap-1.5"><Bot size={12} />Agents</span>}
            {t === 'a2a' && <span className="inline-flex items-center gap-1.5"><Radio size={12} className="text-purple-400" />A2A Remote</span>}
          </button>
        ))}
      </div>

      {/* Templates section */}
      {mainTab === 'templates' && (
        <div>
          <p className="text-sm text-zinc-500 mb-4">Pick a template to get started quickly.</p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {AGENT_TEMPLATES.map(t => (
              <TemplateCard key={t.name} template={t} onUse={useTemplate} />
            ))}
          </div>
        </div>
      )}

      {/* Agents section */}
      {mainTab === 'agents' && (
        <>
          {/* Create form */}
          {creating && (
            <div className="mb-5 p-5 rounded-xl border border-indigo-500/30 bg-indigo-500/5">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-zinc-200">Create New Agent</h3>
                <button onClick={() => setCreating(false)} className="text-zinc-600 hover:text-zinc-300">
                  <X size={16} />
                </button>
              </div>
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
                placeholder="Describe the agent's task…"
                value={form.task}
                onChange={e => setForm(f => ({ ...f, task: e.target.value }))}
                rows={2}
                className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 resize-none"
              />
            </div>
          )}

          {/* Search + Filter bar */}
          <div className="flex flex-wrap gap-2 mb-4">
            <div className="relative flex-1 min-w-48">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
              <input
                value={search}
                onChange={e => setSearch(e.target.value)}
                placeholder="Search agents…"
                className="w-full pl-8 pr-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
              />
              {search && (
                <button onClick={() => setSearch('')} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300">
                  <X size={13} />
                </button>
              )}
            </div>

            <div className="flex items-center gap-1.5">
              <Filter size={13} className="text-zinc-500" />
              <select
                value={filterStatus}
                onChange={e => setFilterStatus(e.target.value)}
                className="px-2.5 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60"
              >
                <option value="all">All statuses</option>
                {(['running', 'completed', 'stopped', 'error'] as const).map(s => (
                  <option key={s} value={s}>{STATUS_CONFIG[s].label}</option>
                ))}
              </select>

              <select
                value={filterRole}
                onChange={e => setFilterRole(e.target.value)}
                className="px-2.5 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60"
              >
                <option value="all">All roles</option>
                {allRoles.map(r => <option key={r} value={r}>{r}</option>)}
              </select>
            </div>

            <div className="flex items-center gap-1.5">
              <SortAsc size={13} className="text-zinc-500" />
              <select
                value={sortKey}
                onChange={e => setSortKey(e.target.value as SortKey)}
                className="px-2.5 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/60"
              >
                <option value="created">Sort: Created</option>
                <option value="name">Sort: Name</option>
                <option value="status">Sort: Status</option>
              </select>
            </div>
          </div>

          {/* Agent grid */}
          {loading ? (
            <div className="flex items-center gap-2 text-zinc-500 py-8">
              <Activity size={16} className="animate-spin" /> Loading agents…
            </div>
          ) : visibleAgents.length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {visibleAgents.map(agent => (
                <AgentCard key={agent.id} agent={agent} onClick={() => setSelectedId(agent.id)} />
              ))}
            </div>
          ) : (
            <div className="text-center py-16 text-zinc-500">
              <Bot size={40} className="mx-auto mb-3 opacity-30" />
              {agents.length === 0 ? (
                <>
                  <p className="mb-2">No agents running.</p>
                  <button onClick={() => setMainTab('templates')} className="text-indigo-400 hover:text-indigo-300 text-sm transition-colors">
                    Browse templates to get started
                  </button>
                </>
              ) : (
                <p>No agents match your filters.</p>
              )}
            </div>
          )}
        </>
      )}

      {/* A2A Remote Agents */}
      {mainTab === 'a2a' && <A2ASection />}
    </div>
  );
}

// ── A2A Section (embedded in Agents view) ───────────────────────────────────

interface RemoteAgentData {
  url: string;
  card: {
    name: string;
    description: string;
    version: string;
    skills: { id: string; name: string; description: string; tags: string[] }[];
    capabilities: { streaming: boolean; pushNotifications: boolean };
  };
  added_at: string;
  last_seen: string;
  status: string;
}

interface A2ATaskData {
  id: string;
  contextId: string;
  status: { state: string; timestamp: string; message?: { parts: { text?: string }[] } };
  artifacts: { artifactId: string; name: string; parts: { text?: string }[] }[];
  history: { messageId: string; role: string; parts: { text?: string }[] }[];
}

const A2A_STATE_COLORS: Record<string, string> = {
  submitted: 'text-blue-400 bg-blue-500/10',
  working: 'text-yellow-400 bg-yellow-500/10',
  completed: 'text-emerald-400 bg-emerald-500/10',
  failed: 'text-red-400 bg-red-500/10',
  canceled: 'text-zinc-400 bg-zinc-500/10',
  'input-required': 'text-purple-400 bg-purple-500/10',
  rejected: 'text-red-400 bg-red-500/10',
};

function A2ASection() {
  const [remoteAgents, setRemoteAgents] = useState<RemoteAgentData[]>([]);
  const [a2aTasks, setA2ATasks] = useState<A2ATaskData[]>([]);
  const [a2aTab, setA2ATab] = useState<'remote' | 'tasks' | 'card'>('remote');
  const [card, setCard] = useState<Record<string, unknown> | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [addURL, setAddURL] = useState('');
  const [showSend, setShowSend] = useState(false);
  const [sendTarget, setSendTarget] = useState('');
  const [sendMsg, setSendMsg] = useState('');
  const [busy, setBusy] = useState(false);
  const [expandedTask, setExpandedTask] = useState<string | null>(null);

  const loadAgents = useCallback(async () => {
    try {
      const res = await fetch('/api/a2a/agents');
      const data = await res.json();
      setRemoteAgents(data.agents || []);
    } catch { /* */ }
  }, []);

  const loadTasks = useCallback(async () => {
    try {
      const res = await fetch('/api/a2a/tasks');
      const data = await res.json();
      setA2ATasks(data.tasks || []);
    } catch { /* */ }
  }, []);

  const loadCard = useCallback(async () => {
    try {
      const res = await fetch('/api/a2a/card');
      setCard(await res.json());
    } catch { /* */ }
  }, []);

  useEffect(() => {
    loadAgents(); loadTasks(); loadCard();
    const iv = setInterval(() => { loadAgents(); loadTasks(); }, 5000);
    return () => clearInterval(iv);
  }, [loadAgents, loadTasks, loadCard]);

  const addAgent = async () => {
    if (!addURL) return;
    setBusy(true);
    try {
      const res = await fetch('/api/a2a/agents', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: addURL }),
      });
      if (!res.ok) { toast.error((await res.json()).error || 'Discovery failed'); return; }
      toast.success('Agent discovered'); setAddURL(''); setShowAdd(false); loadAgents();
    } catch { toast.error('Network error'); } finally { setBusy(false); }
  };

  const removeAgent = async (url: string) => {
    await fetch(`/api/a2a/agents/${encodeURIComponent(url)}`, { method: 'DELETE' });
    toast.success('Removed'); loadAgents();
  };

  const sendToAgent = async () => {
    if (!sendTarget || !sendMsg) return;
    setBusy(true);
    try {
      const res = await fetch('/api/a2a/send', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agentUrl: sendTarget, message: sendMsg }),
      });
      if (!res.ok) { toast.error((await res.json()).error || 'Send failed'); return; }
      toast.success('Message sent'); setSendMsg(''); setShowSend(false); loadTasks();
    } catch { toast.error('Network error'); } finally { setBusy(false); }
  };

  const cancelTask = async (id: string) => {
    await fetch(`/api/a2a/tasks/${id}/cancel`, { method: 'POST' });
    toast.success('Canceled'); loadTasks();
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <p className="text-sm text-zinc-400">
          Discover and communicate with remote A2A-compatible agents
        </p>
        <div className="flex gap-2">
          <button onClick={() => setShowAdd(true)} className="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs font-medium transition-colors">
            <Plus size={14} /> Discover Agent
          </button>
          <button onClick={() => setShowSend(true)} className="flex items-center gap-1.5 px-3 py-1.5 bg-purple-600 hover:bg-purple-700 text-white rounded-lg text-xs font-medium transition-colors">
            <Send size={14} /> Send Task
          </button>
        </div>
      </div>

      {/* Sub-tabs */}
      <div className="flex gap-0.5 p-0.5 rounded-lg bg-zinc-800/40 border border-zinc-700/40 w-fit">
        {(['remote', 'tasks', 'card'] as const).map(t => (
          <button key={t} onClick={() => setA2ATab(t)} className={`px-3 py-1 rounded-md text-xs transition-all ${a2aTab === t ? 'bg-zinc-700 text-zinc-100 font-medium' : 'text-zinc-500 hover:text-zinc-300'}`}>
            {t === 'remote' ? `Agents (${remoteAgents.length})` : t === 'tasks' ? `Tasks (${a2aTasks.length})` : 'My Card'}
          </button>
        ))}
      </div>

      {/* Remote Agents */}
      {a2aTab === 'remote' && (
        <div className="space-y-3">
          {remoteAgents.length === 0 ? (
            <div className="text-center py-10 text-zinc-500">
              <Radio size={32} className="mx-auto mb-2 opacity-30" />
              <p>No remote agents connected</p>
              <p className="text-xs mt-1">Click "Discover Agent" to find A2A-compatible agents</p>
            </div>
          ) : remoteAgents.map(agent => (
            <div key={agent.url} className="p-4 rounded-xl border border-zinc-700/50 bg-zinc-800/30 hover:border-zinc-600/60 transition-all">
              <div className="flex items-start justify-between">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <h4 className="font-medium text-zinc-100">{agent.card.name}</h4>
                    <span className={`px-1.5 py-0.5 rounded-full text-[10px] font-medium ${agent.status === 'online' ? 'bg-emerald-500/15 text-emerald-400' : 'bg-red-500/15 text-red-400'}`}>
                      {agent.status}
                    </span>
                    <span className="text-[10px] text-zinc-600">v{agent.card.version}</span>
                  </div>
                  <p className="text-xs text-zinc-500 mt-0.5">{agent.card.description}</p>
                  <p className="text-[10px] text-zinc-600 font-mono mt-1 truncate">{agent.url}</p>

                  {agent.card.capabilities && (
                    <div className="flex gap-1.5 mt-2">
                      {agent.card.capabilities.streaming && <span className="px-1.5 py-0.5 bg-blue-500/10 text-blue-400 text-[10px] rounded">Streaming</span>}
                      {agent.card.capabilities.pushNotifications && <span className="px-1.5 py-0.5 bg-purple-500/10 text-purple-400 text-[10px] rounded">Push</span>}
                    </div>
                  )}

                  {agent.card.skills?.length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-2">
                      {agent.card.skills.slice(0, 6).map(s => (
                        <span key={s.id} className="px-1.5 py-0.5 bg-zinc-700/50 text-zinc-400 text-[10px] rounded" title={s.description}>{s.name}</span>
                      ))}
                      {agent.card.skills.length > 6 && <span className="text-[10px] text-zinc-600">+{agent.card.skills.length - 6} more</span>}
                    </div>
                  )}
                </div>
                <div className="flex gap-1.5 ml-3">
                  <button onClick={() => { setSendTarget(agent.url); setShowSend(true); }} className="p-1.5 hover:bg-purple-500/10 text-purple-400 rounded transition-colors" title="Send task">
                    <Send size={14} />
                  </button>
                  <button onClick={() => removeAgent(agent.url)} className="p-1.5 hover:bg-red-500/10 text-red-400 rounded transition-colors" title="Remove">
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Tasks */}
      {a2aTab === 'tasks' && (
        <div className="space-y-2">
          {a2aTasks.length === 0 ? (
            <div className="text-center py-10 text-zinc-500">
              <Activity size={32} className="mx-auto mb-2 opacity-30" />
              <p>No A2A tasks yet</p>
              <p className="text-xs mt-1">Send a task to a remote agent to get started</p>
            </div>
          ) : a2aTasks.map(task => (
            <div key={task.id} onClick={() => setExpandedTask(expandedTask === task.id ? null : task.id)}
              className={`p-3 rounded-lg border cursor-pointer transition-all ${expandedTask === task.id ? 'border-blue-500/40 bg-zinc-800/60' : 'border-zinc-700/40 bg-zinc-800/20 hover:border-zinc-600/50'}`}>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${A2A_STATE_COLORS[task.status.state] || 'text-zinc-400 bg-zinc-500/10'}`}>
                    {task.status.state}
                  </span>
                  <span className="text-xs text-zinc-300 font-mono">{task.id.slice(0, 8)}</span>
                  <span className="text-[10px] text-zinc-600">{new Date(task.status.timestamp).toLocaleString()}</span>
                </div>
                {!task.status.state.match(/completed|failed|canceled|rejected/) && (
                  <button onClick={(e) => { e.stopPropagation(); cancelTask(task.id); }} className="px-2 py-0.5 bg-red-500/10 text-red-400 hover:bg-red-500/20 rounded text-[10px] transition-colors">
                    Cancel
                  </button>
                )}
              </div>
              {task.history?.[0] && (
                <p className="text-xs text-zinc-500 mt-1.5 truncate">
                  {task.history[0].parts?.map(p => p.text).filter(Boolean).join(' ')}
                </p>
              )}

              {/* Expanded */}
              {expandedTask === task.id && (
                <div className="mt-3 pt-3 border-t border-zinc-700/40 space-y-3">
                  <div className="grid grid-cols-2 gap-2 text-[10px]">
                    <div><span className="text-zinc-600">Task ID:</span> <span className="text-zinc-400 font-mono">{task.id}</span></div>
                    <div><span className="text-zinc-600">Context:</span> <span className="text-zinc-400 font-mono">{task.contextId?.slice(0, 12)}</span></div>
                  </div>

                  {task.history?.length > 0 && (
                    <div>
                      <p className="text-[10px] text-zinc-600 mb-1">History ({task.history.length})</p>
                      <div className="space-y-1.5 max-h-48 overflow-y-auto">
                        {task.history.map((msg, i) => (
                          <div key={i} className={`p-2 rounded text-xs ${msg.role === 'user' ? 'bg-blue-900/10 border-l-2 border-blue-500/40' : 'bg-zinc-700/20 border-l-2 border-emerald-500/40'}`}>
                            <span className="text-[10px] text-zinc-600">{msg.role}</span>
                            <p className="text-zinc-400 mt-0.5">{msg.parts?.map(p => p.text).filter(Boolean).join(' ')}</p>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {task.artifacts?.length > 0 && (
                    <div>
                      <p className="text-[10px] text-zinc-600 mb-1">Artifacts ({task.artifacts.length})</p>
                      {task.artifacts.map(art => (
                        <div key={art.artifactId} className="bg-zinc-700/20 p-2 rounded text-xs">
                          <span className="text-zinc-500">{art.name || art.artifactId}</span>
                          <pre className="text-zinc-400 mt-1 text-[10px] whitespace-pre-wrap">{art.parts?.map(p => p.text).filter(Boolean).join('\n')}</pre>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* My Card */}
      {a2aTab === 'card' && card && (
        <div className="p-4 rounded-xl border border-zinc-700/50 bg-zinc-800/30">
          <div className="flex items-center justify-between mb-3">
            <h4 className="text-sm font-medium text-zinc-200">Your Agent Card</h4>
            <span className="text-[10px] text-zinc-600 font-mono">/.well-known/agent.json</span>
          </div>
          <pre className="bg-zinc-900/80 p-3 rounded-lg text-[11px] text-zinc-400 overflow-x-auto max-h-80 overflow-y-auto">
            {JSON.stringify(card, null, 2)}
          </pre>
        </div>
      )}

      {/* Add Agent Modal */}
      {showAdd && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowAdd(false)}>
          <div className="bg-zinc-800 border border-zinc-700 rounded-xl p-5 w-full max-w-md" onClick={e => e.stopPropagation()}>
            <h3 className="text-base font-semibold text-white mb-3">Discover Remote Agent</h3>
            <p className="text-xs text-zinc-400 mb-3">Enter the base URL of an A2A-compatible agent.</p>
            <input type="text" value={addURL} onChange={e => setAddURL(e.target.value)}
              placeholder="https://agent.example.com"
              className="w-full bg-zinc-900 border border-zinc-600 rounded-lg px-3 py-2 text-sm text-white placeholder-zinc-500 focus:border-blue-500 focus:outline-none"
              onKeyDown={e => e.key === 'Enter' && addAgent()} />
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setShowAdd(false)} className="px-3 py-1.5 text-zinc-400 hover:text-white text-xs">Cancel</button>
              <button onClick={addAgent} disabled={busy} className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-xs disabled:opacity-50">
                {busy ? 'Discovering...' : 'Discover'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Send Task Modal */}
      {showSend && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowSend(false)}>
          <div className="bg-zinc-800 border border-zinc-700 rounded-xl p-5 w-full max-w-lg" onClick={e => e.stopPropagation()}>
            <h3 className="text-base font-semibold text-white mb-3">Send Task to Remote Agent</h3>
            <div className="space-y-3">
              <div>
                <label className="text-[10px] text-zinc-500 mb-1 block">Target Agent</label>
                {remoteAgents.length > 0 ? (
                  <select value={sendTarget} onChange={e => setSendTarget(e.target.value)}
                    className="w-full bg-zinc-900 border border-zinc-600 rounded-lg px-3 py-2 text-sm text-white focus:border-blue-500 focus:outline-none">
                    <option value="">Select an agent...</option>
                    {remoteAgents.map(a => <option key={a.url} value={a.url}>{a.card.name} — {a.url}</option>)}
                  </select>
                ) : (
                  <input type="text" value={sendTarget} onChange={e => setSendTarget(e.target.value)}
                    placeholder="https://agent.example.com"
                    className="w-full bg-zinc-900 border border-zinc-600 rounded-lg px-3 py-2 text-sm text-white placeholder-zinc-500 focus:border-blue-500 focus:outline-none" />
                )}
              </div>
              <div>
                <label className="text-[10px] text-zinc-500 mb-1 block">Task Message</label>
                <textarea value={sendMsg} onChange={e => setSendMsg(e.target.value)}
                  placeholder="What should the remote agent do?"
                  rows={3}
                  className="w-full bg-zinc-900 border border-zinc-600 rounded-lg px-3 py-2 text-sm text-white placeholder-zinc-500 focus:border-blue-500 focus:outline-none resize-none" />
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setShowSend(false)} className="px-3 py-1.5 text-zinc-400 hover:text-white text-xs">Cancel</button>
              <button onClick={sendToAgent} disabled={busy || !sendTarget || !sendMsg} className="px-3 py-1.5 bg-purple-600 hover:bg-purple-700 text-white rounded-lg text-xs disabled:opacity-50">
                {busy ? 'Sending...' : 'Send Task'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
