import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Shield, Plus, RefreshCw, Search, Trash2, X, Save, Upload,
  Download, Play, ChevronDown, ChevronUp, ToggleLeft, ToggleRight,
  AlertTriangle, CheckCircle, Clock, Globe, Users, Bot, Building2,
  Filter, GripVertical, Info,
} from 'lucide-react';
import {
  fetchPolicies, createPolicy, updatePolicy, deletePolicy, testPolicy, exportPolicies,
  fetchUsers, fetchTeams,
  type PolicyRule, type PolicyDecision, type PolicyScope,
  type PolicyTestRequest, type PolicyTestResult,
  type User, type Team,
} from '../lib/api';
import toast from 'react-hot-toast';

// ── Constants ─────────────────────────────────────────────────────────────────

const SCOPES: { id: PolicyScope; label: string; icon: React.ElementType }[] = [
  { id: 'global', label: 'Global',  icon: Globe },
  { id: 'team',   label: 'Teams',   icon: Building2 },
  { id: 'user',   label: 'Users',   icon: Users },
  { id: 'agent',  label: 'Agents',  icon: Bot },
];

const DECISIONS: { id: PolicyDecision; label: string; color: string; bg: string; border: string }[] = [
  { id: 'allow',            label: 'Allow',            color: 'text-emerald-400', bg: 'bg-emerald-500/10', border: 'border-emerald-500/20' },
  { id: 'deny',             label: 'Deny',             color: 'text-red-400',     bg: 'bg-red-500/10',     border: 'border-red-500/20' },
  { id: 'require_approval', label: 'Require Approval', color: 'text-amber-400',   bg: 'bg-amber-500/10',   border: 'border-amber-500/20' },
];

const COMMON_ACTIONS = [
  'files.read', 'files.write', 'files.*',
  'exec.run', 'exec.*',
  'net.get', 'net.post', 'net.*',
  'audit.read', 'audit.write', 'audit.*',
  'secrets.read', 'secrets.*',
  '*',
];

const DAYS_OF_WEEK = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const MODELS = ['claude-opus-4-5', 'claude-sonnet-4-5', 'claude-haiku-4-5', 'gpt-4.1', 'gpt-4o', 'gpt-4o-mini'];

// ── Helpers ───────────────────────────────────────────────────────────────────

function DecisionBadge({ decision }: { decision: PolicyDecision }) {
  const cfg = DECISIONS.find(d => d.id === decision)!;
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${cfg.color} ${cfg.bg} ${cfg.border}`}>
      {cfg.label}
    </span>
  );
}

function ScopeBadge({ scope }: { scope: PolicyScope }) {
  const cfg = SCOPES.find(s => s.id === scope)!;
  const Icon = cfg.icon;
  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-zinc-800 text-zinc-400 border border-zinc-700">
      <Icon size={10} />
      {cfg.label}
    </span>
  );
}

// ── Empty rule factory ────────────────────────────────────────────────────────

function emptyRule(): Omit<PolicyRule, 'id' | 'created_at' | 'updated_at'> {
  return {
    name: '',
    scope: 'global',
    applies_to: undefined,
    action: 'files.read',
    resource: './**',
    decision: 'allow',
    priority: 10,
    enabled: true,
  };
}

// ── Rule Builder ──────────────────────────────────────────────────────────────

interface RuleBuilderProps {
  initial?: PolicyRule | null;
  users: User[];
  teams: Team[];
  onSave: (rule: Omit<PolicyRule, 'id' | 'created_at' | 'updated_at'>) => Promise<void>;
  onCancel: () => void;
}

function RuleBuilder({ initial, users, teams, onSave, onCancel }: RuleBuilderProps) {
  const [draft, setDraft] = useState<Omit<PolicyRule, 'id' | 'created_at' | 'updated_at'>>(
    initial ? {
      name: initial.name,
      scope: initial.scope,
      applies_to: initial.applies_to,
      action: initial.action,
      resource: initial.resource,
      decision: initial.decision,
      priority: initial.priority,
      enabled: initial.enabled,
      time_restriction: initial.time_restriction,
      cost_limit: initial.cost_limit,
      models: initial.models,
      pii_action: initial.pii_action,
    } : emptyRule()
  );
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!draft.name.trim()) { toast.error('Rule name is required'); return; }
    if (!draft.action.trim()) { toast.error('Action is required'); return; }
    if (!draft.resource.trim()) { toast.error('Resource is required'); return; }
    setSaving(true);
    try {
      await onSave(draft);
    } finally {
      setSaving(false);
    }
  };

  const toggleDay = (day: string) => {
    setDraft(d => {
      const days = d.time_restriction?.days ?? [];
      const next = days.includes(day) ? days.filter(dd => dd !== day) : [...days, day];
      return { ...d, time_restriction: { start: d.time_restriction?.start ?? '09:00', end: d.time_restriction?.end ?? '18:00', days: next } };
    });
  };

  const toggleModel = (model: string) => {
    setDraft(d => {
      const models = d.models ?? [];
      const next = models.includes(model) ? models.filter(m => m !== model) : [...models, model];
      return { ...d, models: next.length > 0 ? next : undefined };
    });
  };

  const appliesOptions = draft.scope === 'user' ? users.map(u => ({ id: u.id, label: u.display_name || u.username }))
    : draft.scope === 'team' ? teams.map(t => ({ id: t.id, label: t.name }))
    : [];

  return (
    <div className="rounded-2xl border border-zinc-700/60 bg-zinc-900/60 overflow-hidden shadow-xl">
      {/* Header */}
      <div className="flex items-center justify-between px-5 py-3.5 border-b border-zinc-700/40 bg-zinc-900/80">
        <div className="flex items-center gap-2">
          <Shield size={14} className="text-indigo-400" />
          <h3 className="text-sm font-semibold text-zinc-200">
            {initial ? `Edit: ${initial.name}` : 'New Rule'}
          </h3>
        </div>
        <button onClick={onCancel} className="p-1.5 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors">
          <X size={14} />
        </button>
      </div>

      <div className="p-5 space-y-4">
        {/* Name */}
        <div className="space-y-1">
          <label className="text-xs text-zinc-400 font-medium">Rule Name</label>
          <input
            value={draft.name}
            onChange={e => setDraft(d => ({ ...d, name: e.target.value }))}
            placeholder="allow-workspace-reads"
            className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 placeholder-zinc-600 font-mono focus:outline-none focus:border-indigo-500/60"
          />
        </div>

        {/* Scope + Applies To */}
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Scope</label>
            <div className="flex gap-1.5 flex-wrap">
              {SCOPES.map(({ id, label, icon: Icon }) => (
                <button
                  key={id}
                  onClick={() => setDraft(d => ({ ...d, scope: id, applies_to: undefined }))}
                  className={`flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-medium transition-all ${
                    draft.scope === id
                      ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30'
                      : 'bg-zinc-800 text-zinc-500 border border-zinc-700 hover:text-zinc-300'
                  }`}
                >
                  <Icon size={10} />
                  {label}
                </button>
              ))}
            </div>
          </div>
          {appliesOptions.length > 0 && (
            <div className="space-y-1">
              <label className="text-xs text-zinc-400 font-medium">Applies To</label>
              <select
                value={draft.applies_to ?? ''}
                onChange={e => setDraft(d => ({ ...d, applies_to: e.target.value || undefined }))}
                className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
              >
                <option value="">Everyone</option>
                {appliesOptions.map(o => <option key={o.id} value={o.id}>{o.label}</option>)}
              </select>
            </div>
          )}
        </div>

        {/* Action + Resource */}
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Action</label>
            <input
              list="action-suggestions"
              value={draft.action}
              onChange={e => setDraft(d => ({ ...d, action: e.target.value }))}
              placeholder="files.read"
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 font-mono placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
            <datalist id="action-suggestions">
              {COMMON_ACTIONS.map(a => <option key={a} value={a} />)}
            </datalist>
          </div>
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Resource</label>
            <input
              value={draft.resource}
              onChange={e => setDraft(d => ({ ...d, resource: e.target.value }))}
              placeholder="./**"
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 font-mono placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
        </div>

        {/* Decision */}
        <div className="space-y-1">
          <label className="text-xs text-zinc-400 font-medium">Decision</label>
          <div className="flex gap-2">
            {DECISIONS.map(({ id, label, color, bg, border }) => (
              <button
                key={id}
                onClick={() => setDraft(d => ({ ...d, decision: id }))}
                className={`flex-1 py-2 rounded-xl text-xs font-medium transition-all border ${
                  draft.decision === id
                    ? `${color} ${bg} ${border}`
                    : 'text-zinc-500 bg-zinc-800/80 border-zinc-700 hover:text-zinc-300'
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>

        {/* Priority + Enabled */}
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Priority</label>
            <input
              type="number"
              value={draft.priority}
              onChange={e => setDraft(d => ({ ...d, priority: Number(e.target.value) }))}
              min={1}
              max={1000}
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
            />
            <p className="text-xs text-zinc-600">Higher number = evaluated first</p>
          </div>
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Enabled</label>
            <button
              onClick={() => setDraft(d => ({ ...d, enabled: !d.enabled }))}
              className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors ${
                draft.enabled
                  ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                  : 'bg-zinc-800 text-zinc-500 border border-zinc-700'
              }`}
            >
              {draft.enabled
                ? <ToggleRight size={16} />
                : <ToggleLeft size={16} />
              }
              {draft.enabled ? 'Enabled' : 'Disabled'}
            </button>
          </div>
        </div>

        {/* Advanced toggle */}
        <button
          onClick={() => setShowAdvanced(v => !v)}
          className="flex items-center gap-1.5 text-xs text-zinc-500 hover:text-zinc-300 transition-colors w-full"
        >
          <div className="flex-1 h-px bg-zinc-800" />
          <span className="px-2">Advanced</span>
          {showAdvanced ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
          <div className="flex-1 h-px bg-zinc-800" />
        </button>

        {showAdvanced && (
          <div className="space-y-4 pt-1">
            {/* Time restriction */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-xs text-zinc-400 font-medium flex items-center gap-1.5">
                  <Clock size={12} />
                  Time Restriction
                </label>
                <button
                  onClick={() => setDraft(d => ({
                    ...d,
                    time_restriction: d.time_restriction ? undefined : { start: '09:00', end: '18:00', days: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri'] },
                  }))}
                  className={`text-xs px-2 py-1 rounded-lg transition-colors ${
                    draft.time_restriction ? 'text-amber-400 bg-amber-500/10' : 'text-zinc-500 bg-zinc-800 hover:text-zinc-300'
                  }`}
                >
                  {draft.time_restriction ? 'Remove' : 'Add'}
                </button>
              </div>
              {draft.time_restriction && (
                <div className="space-y-2 p-3 rounded-xl bg-zinc-800/50 border border-zinc-700/40">
                  <div className="flex items-center gap-2">
                    <input
                      type="time"
                      value={draft.time_restriction.start}
                      onChange={e => setDraft(d => ({ ...d, time_restriction: { ...d.time_restriction!, start: e.target.value } }))}
                      className="px-2 py-1 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                    />
                    <span className="text-xs text-zinc-500">to</span>
                    <input
                      type="time"
                      value={draft.time_restriction.end}
                      onChange={e => setDraft(d => ({ ...d, time_restriction: { ...d.time_restriction!, end: e.target.value } }))}
                      className="px-2 py-1 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                    />
                  </div>
                  <div className="flex gap-1 flex-wrap">
                    {DAYS_OF_WEEK.map(day => {
                      const active = draft.time_restriction!.days.includes(day);
                      return (
                        <button
                          key={day}
                          onClick={() => toggleDay(day)}
                          className={`px-2 py-1 rounded-lg text-xs font-medium transition-all ${
                            active ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30' : 'bg-zinc-900 text-zinc-500 border border-zinc-700 hover:text-zinc-300'
                          }`}
                        >
                          {day}
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>

            {/* Cost limit */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-xs text-zinc-400 font-medium">Cost Limit</label>
                <button
                  onClick={() => setDraft(d => ({
                    ...d,
                    cost_limit: d.cost_limit ? undefined : { daily: 10, monthly: 100 },
                  }))}
                  className={`text-xs px-2 py-1 rounded-lg transition-colors ${
                    draft.cost_limit ? 'text-amber-400 bg-amber-500/10' : 'text-zinc-500 bg-zinc-800 hover:text-zinc-300'
                  }`}
                >
                  {draft.cost_limit ? 'Remove' : 'Add'}
                </button>
              </div>
              {draft.cost_limit && (
                <div className="grid grid-cols-2 gap-2 p-3 rounded-xl bg-zinc-800/50 border border-zinc-700/40">
                  <div className="space-y-1">
                    <label className="text-xs text-zinc-600">Daily ($)</label>
                    <input
                      type="number"
                      step="0.1"
                      value={draft.cost_limit.daily ?? ''}
                      onChange={e => setDraft(d => ({ ...d, cost_limit: { ...d.cost_limit!, daily: Number(e.target.value) } }))}
                      className="w-full px-2 py-1 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-zinc-600">Monthly ($)</label>
                    <input
                      type="number"
                      step="1"
                      value={draft.cost_limit.monthly ?? ''}
                      onChange={e => setDraft(d => ({ ...d, cost_limit: { ...d.cost_limit!, monthly: Number(e.target.value) } }))}
                      className="w-full px-2 py-1 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                    />
                  </div>
                </div>
              )}
            </div>

            {/* Model restriction */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-xs text-zinc-400 font-medium">Model Restriction</label>
                <button
                  onClick={() => setDraft(d => ({ ...d, models: d.models ? undefined : [] }))}
                  className={`text-xs px-2 py-1 rounded-lg transition-colors ${
                    draft.models !== undefined ? 'text-amber-400 bg-amber-500/10' : 'text-zinc-500 bg-zinc-800 hover:text-zinc-300'
                  }`}
                >
                  {draft.models !== undefined ? 'Remove' : 'Add'}
                </button>
              </div>
              {draft.models !== undefined && (
                <div className="flex flex-wrap gap-1.5 p-3 rounded-xl bg-zinc-800/50 border border-zinc-700/40">
                  {MODELS.map(m => {
                    const active = draft.models!.includes(m);
                    return (
                      <button
                        key={m}
                        onClick={() => toggleModel(m)}
                        className={`px-2.5 py-1 rounded-lg text-xs font-mono transition-all ${
                          active ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30' : 'bg-zinc-900 text-zinc-500 border border-zinc-700 hover:text-zinc-300'
                        }`}
                      >
                        {m}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>

            {/* PII action */}
            <div className="space-y-2">
              <label className="text-xs text-zinc-400 font-medium">PII Handling</label>
              <div className="flex gap-2">
                {([undefined, 'allow', 'block', 'redact'] as const).map(pii => (
                  <button
                    key={String(pii)}
                    onClick={() => setDraft(d => ({ ...d, pii_action: pii }))}
                    className={`flex-1 py-1.5 rounded-lg text-xs font-medium capitalize transition-all border ${
                      draft.pii_action === pii
                        ? 'bg-indigo-500/20 text-indigo-300 border-indigo-500/30'
                        : 'bg-zinc-800 text-zinc-500 border-zinc-700 hover:text-zinc-300'
                    }`}
                  >
                    {pii === undefined ? 'Default' : pii}
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* Actions */}
        <div className="flex justify-end gap-2 pt-1 border-t border-zinc-800">
          <button onClick={onCancel} className="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors">
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors disabled:opacity-60"
          >
            {saving ? <RefreshCw size={13} className="animate-spin" /> : <Save size={13} />}
            {saving ? 'Saving…' : 'Save Rule'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Rule Test Panel ───────────────────────────────────────────────────────────

interface TestPanelProps {
  users: User[];
  onClose: () => void;
}

function TestPanel({ users, onClose }: TestPanelProps) {
  const [req, setReq] = useState<PolicyTestRequest>({ action: 'files.read', resource: './**' });
  const [result, setResult] = useState<PolicyTestResult | null>(null);
  const [testing, setTesting] = useState(false);

  const runTest = async () => {
    setTesting(true);
    try {
      const r = await testPolicy(req);
      setResult(r);
    } catch {
      toast.error('Test failed');
    } finally {
      setTesting(false);
    }
  };

  const decisionCfg = result ? DECISIONS.find(d => d.id === result.decision) : null;

  return (
    <div className="rounded-2xl border border-zinc-700/60 bg-zinc-900/60 overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3.5 border-b border-zinc-700/40 bg-zinc-900/80">
        <div className="flex items-center gap-2">
          <Play size={14} className="text-emerald-400" />
          <h3 className="text-sm font-semibold text-zinc-200">Test a Request</h3>
        </div>
        <button onClick={onClose} className="p-1.5 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors">
          <X size={14} />
        </button>
      </div>
      <div className="p-5 space-y-4">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Action</label>
            <input
              list="test-action-suggestions"
              value={req.action}
              onChange={e => setReq(r => ({ ...r, action: e.target.value }))}
              placeholder="files.read"
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 font-mono placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
            <datalist id="test-action-suggestions">
              {COMMON_ACTIONS.map(a => <option key={a} value={a} />)}
            </datalist>
          </div>
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Resource</label>
            <input
              value={req.resource}
              onChange={e => setReq(r => ({ ...r, resource: e.target.value }))}
              placeholder="./**"
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 font-mono placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">User (optional)</label>
            <select
              value={req.user ?? ''}
              onChange={e => setReq(r => ({ ...r, user: e.target.value || undefined }))}
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
            >
              <option value="">Any user</option>
              {users.map(u => <option key={u.id} value={u.id}>{u.display_name || u.username}</option>)}
            </select>
          </div>
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Scope</label>
            <select
              value={req.scope ?? ''}
              onChange={e => setReq(r => ({ ...r, scope: (e.target.value as PolicyScope) || undefined }))}
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
            >
              <option value="">All scopes</option>
              {SCOPES.map(s => <option key={s.id} value={s.id}>{s.label}</option>)}
            </select>
          </div>
        </div>
        <button
          onClick={runTest}
          disabled={testing}
          className="flex items-center gap-1.5 w-full justify-center py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-sm transition-colors disabled:opacity-60"
        >
          {testing ? <RefreshCw size={13} className="animate-spin" /> : <Play size={13} />}
          {testing ? 'Testing…' : 'Run Test'}
        </button>

        {result && (
          <div className={`rounded-xl border p-4 space-y-3 ${
            result.decision === 'allow' ? 'bg-emerald-500/5 border-emerald-500/20' :
            result.decision === 'deny' ? 'bg-red-500/5 border-red-500/20' :
            'bg-amber-500/5 border-amber-500/20'
          }`}>
            <div className="flex items-center gap-2">
              {result.decision === 'allow'
                ? <CheckCircle size={16} className="text-emerald-400" />
                : result.decision === 'deny'
                  ? <AlertTriangle size={16} className="text-red-400" />
                  : <Clock size={16} className="text-amber-400" />
              }
              <span className={`text-sm font-semibold ${decisionCfg?.color}`}>
                {decisionCfg?.label ?? result.decision}
              </span>
            </div>
            <p className="text-xs text-zinc-400">{result.reason}</p>
            {result.matched_rule && (
              <div className="p-2.5 rounded-lg bg-zinc-900/60 border border-zinc-700/40 space-y-1">
                <div className="text-xs text-zinc-500">Matched rule:</div>
                <div className="text-xs font-mono text-zinc-300">{result.matched_rule.name}</div>
                <div className="flex gap-2 flex-wrap">
                  <ScopeBadge scope={result.matched_rule.scope} />
                  <span className="text-xs text-zinc-500 font-mono">{result.matched_rule.action}</span>
                  <span className="text-xs text-zinc-500">priority: {result.matched_rule.priority}</span>
                </div>
              </div>
            )}
            {result.all_matched.length === 0 && (
              <p className="text-xs text-zinc-500 flex items-center gap-1">
                <Info size={11} />
                No rules matched — default-deny applied
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Rule Row ──────────────────────────────────────────────────────────────────

interface RuleRowProps {
  rule: PolicyRule;
  onEdit: () => void;
  onDelete: () => void;
  onToggle: () => void;
}

function RuleRow({ rule, onEdit, onDelete, onToggle }: RuleRowProps) {
  return (
    <tr className={`group border-b border-zinc-800/50 transition-colors hover:bg-zinc-900/40 ${!rule.enabled ? 'opacity-50' : ''}`}>
      <td className="px-3 py-3 w-6">
        <GripVertical size={14} className="text-zinc-700 cursor-grab" />
      </td>
      <td className="px-3 py-3">
        <div className="flex items-center gap-2">
          <span className="text-sm font-mono text-zinc-200">{rule.name}</span>
          {rule.time_restriction && <span title="Time restricted"><Clock size={11} className="text-amber-400 flex-shrink-0" /></span>}
          {rule.cost_limit && <span className="text-amber-400 text-xs flex-shrink-0" title="Cost limit">$</span>}
          {rule.pii_action && <span className="text-violet-400 text-xs flex-shrink-0 font-mono" title={`PII: ${rule.pii_action}`}>PII</span>}
        </div>
      </td>
      <td className="px-3 py-3"><ScopeBadge scope={rule.scope} /></td>
      <td className="px-3 py-3"><code className="text-xs font-mono text-indigo-300">{rule.action}</code></td>
      <td className="px-3 py-3"><code className="text-xs font-mono text-zinc-400">{rule.resource}</code></td>
      <td className="px-3 py-3"><DecisionBadge decision={rule.decision} /></td>
      <td className="px-3 py-3">
        <span className="text-xs text-zinc-500 font-mono">{rule.priority}</span>
      </td>
      <td className="px-3 py-3">
        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <button onClick={onToggle} className="p-1.5 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors" title={rule.enabled ? 'Disable' : 'Enable'}>
            {rule.enabled ? <ToggleRight size={14} className="text-emerald-400" /> : <ToggleLeft size={14} />}
          </button>
          <button onClick={onEdit} className="p-1.5 rounded-lg text-zinc-500 hover:text-indigo-400 hover:bg-indigo-500/10 transition-colors" title="Edit">
            <Shield size={13} />
          </button>
          <button onClick={onDelete} className="p-1.5 rounded-lg text-zinc-500 hover:text-red-400 hover:bg-red-500/10 transition-colors" title="Delete">
            <Trash2 size={13} />
          </button>
        </div>
      </td>
    </tr>
  );
}

// ── Main View ─────────────────────────────────────────────────────────────────

export default function PoliciesView() {
  const [rules, setRules] = useState<PolicyRule[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeScope, setActiveScope] = useState<PolicyScope | 'all'>('all');
  const [search, setSearch] = useState('');
  const [decisionFilter, setDecisionFilter] = useState<PolicyDecision | 'all'>('all');
  const [editingRule, setEditingRule] = useState<PolicyRule | null | 'new'>(null);
  const [showTest, setShowTest] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PolicyRule | null>(null);
  const [sortField, setSortField] = useState<'name' | 'priority' | 'scope'>('priority');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');
  const fileInputRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [p, u, t] = await Promise.all([fetchPolicies(), fetchUsers(), fetchTeams()]);
      setRules(p);
      setUsers(u);
      setTeams(t);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = rules
    .filter(r => {
      const matchScope = activeScope === 'all' || r.scope === activeScope;
      const matchDecision = decisionFilter === 'all' || r.decision === decisionFilter;
      const q = search.toLowerCase();
      const matchSearch = !q || r.name.toLowerCase().includes(q) || r.action.toLowerCase().includes(q) || r.resource.toLowerCase().includes(q);
      return matchScope && matchDecision && matchSearch;
    })
    .sort((a, b) => {
      let cmp = 0;
      if (sortField === 'name') cmp = a.name.localeCompare(b.name);
      else if (sortField === 'priority') cmp = a.priority - b.priority;
      else if (sortField === 'scope') cmp = a.scope.localeCompare(b.scope);
      return sortDir === 'asc' ? cmp : -cmp;
    });

  const toggleSort = (field: typeof sortField) => {
    if (sortField === field) setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    else { setSortField(field); setSortDir('desc'); }
  };

  const SortIcon = ({ field }: { field: typeof sortField }) => {
    if (sortField !== field) return null;
    return sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />;
  };

  const handleSaveRule = async (data: Omit<PolicyRule, 'id' | 'created_at' | 'updated_at'>) => {
    if (editingRule && editingRule !== 'new') {
      await updatePolicy(editingRule.id, data);
      setRules(rs => rs.map(r => r.id === editingRule.id ? { ...r, ...data, updated_at: new Date().toISOString() } : r));
      toast.success('Rule updated');
    } else {
      const newRule = await createPolicy(data);
      setRules(rs => [...rs, newRule]);
      toast.success('Rule created');
    }
    setEditingRule(null);
  };

  const handleDelete = async (rule: PolicyRule) => {
    setDeleteTarget(null);
    await deletePolicy(rule.id);
    setRules(rs => rs.filter(r => r.id !== rule.id));
    toast.success('Rule deleted');
  };

  const handleToggle = async (rule: PolicyRule) => {
    const updated = { ...rule, enabled: !rule.enabled };
    await updatePolicy(rule.id, { enabled: updated.enabled });
    setRules(rs => rs.map(r => r.id === rule.id ? updated : r));
    toast.success(updated.enabled ? 'Rule enabled' : 'Rule disabled');
  };

  const handleExport = async () => {
    const yaml = await exportPolicies();
    const blob = new Blob([yaml], { type: 'text/yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'policy.yml';
    a.click();
    URL.revokeObjectURL(url);
    toast.success('Policies exported');
  };

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const imported = JSON.parse(text);
      if (Array.isArray(imported)) {
        for (const rule of imported) {
          if (rule.name && rule.action) {
            await createPolicy(rule);
          }
        }
        toast.success(`Imported ${imported.length} rules from ${file.name}`);
        load();
      } else {
        toast.error('Invalid format — expected JSON array of rules');
      }
    } catch {
      toast.error('Failed to parse import file');
    }
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  // Stats
  const stats = {
    total: rules.length,
    enabled: rules.filter(r => r.enabled).length,
    allow: rules.filter(r => r.decision === 'allow').length,
    deny: rules.filter(r => r.decision === 'deny').length,
    approval: rules.filter(r => r.decision === 'require_approval').length,
  };

  return (
    <div className="flex-1 overflow-y-auto bg-zinc-950 p-6" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Policy Editor</h2>
          <p className="text-sm text-zinc-500">Visual security policy management</p>
        </div>
        <div className="flex items-center gap-2">
          <input ref={fileInputRef} type="file" accept=".yml,.yaml" onChange={handleImport} className="hidden" />
          <button
            onClick={() => fileInputRef.current?.click()}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-sm transition-colors"
          >
            <Upload size={14} />
            Import
          </button>
          <button
            onClick={handleExport}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-sm transition-colors"
          >
            <Download size={14} />
            Export
          </button>
          <button
            onClick={load}
            className="p-2 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors"
          >
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
          </button>
          <button
            onClick={() => { setShowTest(v => !v); }}
            className={`flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm transition-colors ${
              showTest ? 'bg-emerald-600 text-white' : 'bg-zinc-800 hover:bg-zinc-700 text-zinc-300'
            }`}
          >
            <Play size={14} />
            Test
          </button>
          <button
            onClick={() => setEditingRule('new')}
            className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors"
          >
            <Plus size={14} />
            New Rule
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-5 gap-3 mb-5">
        {[
          { label: 'Total',    value: stats.total,    color: 'text-zinc-300' },
          { label: 'Enabled',  value: stats.enabled,  color: 'text-emerald-400' },
          { label: 'Allow',    value: stats.allow,    color: 'text-emerald-400' },
          { label: 'Deny',     value: stats.deny,     color: 'text-red-400' },
          { label: 'Approval', value: stats.approval, color: 'text-amber-400' },
        ].map(({ label, value, color }) => (
          <div key={label} className="p-3 rounded-xl bg-zinc-900/40 border border-zinc-800/60 text-center">
            <div className={`text-xl font-bold ${color}`}>{value}</div>
            <div className="text-xs text-zinc-600 mt-0.5">{label}</div>
          </div>
        ))}
      </div>

      {/* Test panel */}
      {showTest && (
        <div className="mb-5">
          <TestPanel users={users} onClose={() => setShowTest(false)} />
        </div>
      )}

      {/* Rule builder */}
      {editingRule !== null && (
        <div className="mb-5">
          <RuleBuilder
            initial={editingRule === 'new' ? null : editingRule}
            users={users}
            teams={teams}
            onSave={handleSaveRule}
            onCancel={() => setEditingRule(null)}
          />
        </div>
      )}

      {/* Scope tabs */}
      <div className="flex gap-1 mb-4">
        <button
          onClick={() => setActiveScope('all')}
          className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
            activeScope === 'all' ? 'bg-indigo-500/15 text-indigo-400' : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800'
          }`}
        >
          All ({rules.length})
        </button>
        {SCOPES.map(({ id, label, icon: Icon }) => {
          const count = rules.filter(r => r.scope === id).length;
          return (
            <button
              key={id}
              onClick={() => setActiveScope(id)}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                activeScope === id ? 'bg-indigo-500/15 text-indigo-400' : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800'
              }`}
            >
              <Icon size={11} />
              {label} ({count})
            </button>
          );
        })}
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative flex-1 max-w-xs">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search rules…"
            className="w-full pl-8 pr-3 py-2 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/50"
          />
        </div>
        <select
          value={decisionFilter}
          onChange={e => setDecisionFilter(e.target.value as typeof decisionFilter)}
          className="px-3 py-2 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-300 focus:outline-none focus:border-indigo-500/50"
        >
          <option value="all">All decisions</option>
          {DECISIONS.map(d => <option key={d.id} value={d.id}>{d.label}</option>)}
        </select>
        <div className="flex items-center gap-1 text-xs text-zinc-600">
          <Filter size={12} />
          {filtered.length} rules
        </div>
      </div>

      {/* Rules table */}
      <div className="rounded-xl border border-zinc-800/60 overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center h-40 text-zinc-500 gap-2">
            <RefreshCw size={16} className="animate-spin" />
            Loading policies…
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-40 text-zinc-600 gap-2">
            <Shield size={32} strokeWidth={1.5} />
            <p className="text-sm">No rules found</p>
            <button
              onClick={() => setEditingRule('new')}
              className="flex items-center gap-1.5 text-xs text-indigo-400 hover:text-indigo-300 transition-colors mt-1"
            >
              <Plus size={13} />
              Add a rule
            </button>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="bg-zinc-900/80 border-b border-zinc-800">
                <th className="w-8 px-3 py-3" />
                <th className="px-3 py-3 text-left">
                  <button onClick={() => toggleSort('name')} className="flex items-center gap-1 text-xs font-medium text-zinc-400 hover:text-zinc-200">
                    Name <SortIcon field="name" />
                  </button>
                </th>
                <th className="px-3 py-3 text-left">
                  <button onClick={() => toggleSort('scope')} className="flex items-center gap-1 text-xs font-medium text-zinc-400 hover:text-zinc-200">
                    Scope <SortIcon field="scope" />
                  </button>
                </th>
                <th className="px-3 py-3 text-left text-xs font-medium text-zinc-400">Action</th>
                <th className="px-3 py-3 text-left text-xs font-medium text-zinc-400">Resource</th>
                <th className="px-3 py-3 text-left text-xs font-medium text-zinc-400">Decision</th>
                <th className="px-3 py-3 text-left">
                  <button onClick={() => toggleSort('priority')} className="flex items-center gap-1 text-xs font-medium text-zinc-400 hover:text-zinc-200">
                    Priority <SortIcon field="priority" />
                  </button>
                </th>
                <th className="px-3 py-3 text-left text-xs font-medium text-zinc-400">Actions</th>
              </tr>
            </thead>
            <tbody className="bg-zinc-900/20">
              {filtered.map(rule => (
                <RuleRow
                  key={rule.id}
                  rule={rule}
                  onEdit={() => setEditingRule(rule)}
                  onDelete={() => setDeleteTarget(rule)}
                  onToggle={() => handleToggle(rule)}
                />
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Delete confirmation */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-zinc-900 border border-zinc-700/60 rounded-2xl w-full max-w-sm shadow-2xl p-6">
            <div className="flex items-start gap-3 mb-4">
              <div className="p-2 rounded-lg bg-red-500/10 flex-shrink-0">
                <AlertTriangle size={18} className="text-red-400" />
              </div>
              <div>
                <h3 className="text-sm font-semibold text-zinc-100 mb-1">Delete rule</h3>
                <p className="text-xs text-zinc-400">
                  Delete <span className="text-zinc-200 font-mono">{deleteTarget.name}</span>? This cannot be undone.
                </p>
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <button onClick={() => setDeleteTarget(null)} className="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors">Cancel</button>
              <button onClick={() => handleDelete(deleteTarget)} className="px-4 py-2 rounded-lg text-sm bg-red-600 hover:bg-red-500 text-white transition-colors">Delete</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
