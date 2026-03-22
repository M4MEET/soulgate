import { useState, useEffect, useCallback } from 'react';
import {
  Users, Plus, Search, RefreshCw, ChevronRight, X, Save,
  Shield, Activity, Clock, DollarSign, Cpu, Key, Eye, EyeOff,
  CheckSquare, Square, ToggleLeft, ToggleRight, Trash2, AlertTriangle,
  ChevronDown, ChevronUp, Settings, BarChart2,
} from 'lucide-react';
import {
  fetchUsers, createUser, updateUser, deleteUser, regenerateApiKey,
  fetchTeams,
  type User, type UserRole, type UserStatus, type CreateUserPayload, type Team,
} from '../lib/api';
import { formatRelativeTime, formatCost } from '../lib/utils';
import toast from 'react-hot-toast';

// ── Role config ───────────────────────────────────────────────────────────────

const ROLE_CONFIG: Record<UserRole, { label: string; color: string; bg: string; border: string }> = {
  admin:     { label: 'Admin',     color: 'text-red-400',   bg: 'bg-red-500/10',   border: 'border-red-500/20' },
  developer: { label: 'Developer', color: 'text-blue-400',  bg: 'bg-blue-500/10',  border: 'border-blue-500/20' },
  operator:  { label: 'Operator',  color: 'text-amber-400', bg: 'bg-amber-500/10', border: 'border-amber-500/20' },
  viewer:    { label: 'Viewer',    color: 'text-zinc-400',  bg: 'bg-zinc-500/10',  border: 'border-zinc-500/20' },
};

function RoleBadge({ role }: { role: UserRole }) {
  const cfg = ROLE_CONFIG[role];
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${cfg.color} ${cfg.bg} ${cfg.border}`}>
      {cfg.label}
    </span>
  );
}

function StatusDot({ status }: { status: UserStatus }) {
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs ${status === 'active' ? 'text-emerald-400' : 'text-zinc-500'}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${status === 'active' ? 'bg-emerald-400' : 'bg-zinc-600'}`} />
      {status === 'active' ? 'Active' : 'Inactive'}
    </span>
  );
}

function ProgressBar({ value, max, color = 'bg-indigo-500' }: { value: number; max: number; color?: string }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  const warningColor = pct > 90 ? 'bg-red-500' : pct > 70 ? 'bg-amber-500' : color;
  return (
    <div className="h-1.5 bg-zinc-800 rounded-full overflow-hidden flex-1">
      <div className={`h-full rounded-full transition-all ${warningColor}`} style={{ width: `${pct}%` }} />
    </div>
  );
}

// ── Create User Modal ─────────────────────────────────────────────────────────

interface CreateUserModalProps {
  teams: Team[];
  onClose: () => void;
  onCreated: (user: User) => void;
}

function CreateUserModal({ teams, onClose, onCreated }: CreateUserModalProps) {
  const [form, setForm] = useState<CreateUserPayload>({
    username: '', display_name: '', email: '', role: 'developer', team_id: '',
  });
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.username.trim() || !form.email.trim()) {
      toast.error('Username and email are required');
      return;
    }
    setSaving(true);
    try {
      const user = await createUser({ ...form, team_id: form.team_id || undefined });
      onCreated(user);
      toast.success(`User ${user.username} created`);
    } catch {
      toast.error('Failed to create user');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-zinc-900 border border-zinc-700/60 rounded-2xl w-full max-w-md shadow-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800">
          <h3 className="text-sm font-semibold text-zinc-100">Create User</h3>
          <button onClick={onClose} className="p-1 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors">
            <X size={16} />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Username *</label>
            <input
              value={form.username}
              onChange={e => setForm(f => ({ ...f, username: e.target.value }))}
              placeholder="alice"
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Display Name</label>
            <input
              value={form.display_name}
              onChange={e => setForm(f => ({ ...f, display_name: e.target.value }))}
              placeholder="Alice Chen"
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Email *</label>
            <input
              type="email"
              value={form.email}
              onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
              placeholder="alice@example.com"
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-xs text-zinc-400 font-medium">Role</label>
              <select
                value={form.role}
                onChange={e => setForm(f => ({ ...f, role: e.target.value as UserRole }))}
                className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
              >
                {(Object.keys(ROLE_CONFIG) as UserRole[]).map(r => (
                  <option key={r} value={r}>{ROLE_CONFIG[r].label}</option>
                ))}
              </select>
            </div>
            <div className="space-y-1">
              <label className="text-xs text-zinc-400 font-medium">Team</label>
              <select
                value={form.team_id ?? ''}
                onChange={e => setForm(f => ({ ...f, team_id: e.target.value }))}
                className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
              >
                <option value="">No team</option>
                {teams.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
              </select>
            </div>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors">
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors disabled:opacity-60"
            >
              {saving ? <RefreshCw size={13} className="animate-spin" /> : <Plus size={13} />}
              {saving ? 'Creating…' : 'Create User'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Delete confirmation ───────────────────────────────────────────────────────

interface DeleteConfirmProps {
  user: User;
  onClose: () => void;
  onConfirm: () => void;
}

function DeleteConfirm({ user, onClose, onConfirm }: DeleteConfirmProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-zinc-900 border border-zinc-700/60 rounded-2xl w-full max-w-sm shadow-2xl p-6">
        <div className="flex items-start gap-3 mb-4">
          <div className="p-2 rounded-lg bg-red-500/10">
            <AlertTriangle size={18} className="text-red-400" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-zinc-100 mb-1">Delete user</h3>
            <p className="text-xs text-zinc-400">
              Are you sure you want to delete <span className="text-zinc-200 font-medium">{user.display_name || user.username}</span>? This action cannot be undone.
            </p>
          </div>
        </div>
        <div className="flex justify-end gap-2">
          <button onClick={onClose} className="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors">Cancel</button>
          <button onClick={onConfirm} className="px-4 py-2 rounded-lg text-sm bg-red-600 hover:bg-red-500 text-white transition-colors">Delete</button>
        </div>
      </div>
    </div>
  );
}

// ── User Detail Panel ─────────────────────────────────────────────────────────

type DetailTab = 'profile' | 'settings' | 'limits' | 'usage' | 'apikey';

const DETAIL_TABS: { id: DetailTab; label: string; icon: React.ElementType }[] = [
  { id: 'profile',  label: 'Profile',  icon: Users },
  { id: 'settings', label: 'Settings', icon: Settings },
  { id: 'limits',   label: 'Limits',   icon: Shield },
  { id: 'usage',    label: 'Usage',    icon: BarChart2 },
  { id: 'apikey',   label: 'API Key',  icon: Key },
];

const MODELS = ['claude-opus-4-5', 'claude-sonnet-4-5', 'claude-haiku-4-5', 'gpt-4.1', 'gpt-4o', 'gpt-4o-mini'];
const PROVIDERS = ['anthropic', 'openai', 'ollama', 'groq'];
const THINKING_LEVELS = ['none', 'low', 'medium', 'high'];
const ALL_TOOLS = ['read_file', 'write_file', 'exec', 'search', 'web_fetch', 'git', 'audit'];

interface UserDetailProps {
  user: User;
  teams: Team[];
  onClose: () => void;
  onUpdate: (updated: User) => void;
  onDelete: (user: User) => void;
}

function UserDetail({ user, teams, onClose, onUpdate, onDelete }: UserDetailProps) {
  const [tab, setTab] = useState<DetailTab>('profile');
  const [draft, setDraft] = useState<User>({ ...user });
  const [saving, setSaving] = useState(false);
  const [showApiKey, setShowApiKey] = useState(false);
  const [regenConfirm, setRegenConfirm] = useState(false);
  const [regening, setRegening] = useState(false);

  const save = async () => {
    setSaving(true);
    try {
      await updateUser(user.id, draft);
      onUpdate(draft);
      toast.success('User updated');
    } catch {
      toast.error('Failed to update user');
    } finally {
      setSaving(false);
    }
  };

  const handleRegen = async () => {
    if (!regenConfirm) { setRegenConfirm(true); return; }
    setRegening(true);
    try {
      const key = await regenerateApiKey(user.id);
      setDraft(d => ({ ...d, api_key_masked: key }));
      setRegenConfirm(false);
      toast.success('API key regenerated');
    } catch {
      toast.error('Failed to regenerate key');
    } finally {
      setRegening(false);
    }
  };

  const toggleModel = (model: string) => {
    setDraft(d => {
      const current = d.limits?.allowed_models ?? [];
      const next = current.includes(model) ? current.filter(m => m !== model) : [...current, model];
      return { ...d, limits: { ...d.limits!, allowed_models: next } };
    });
  };

  const toggleTool = (tool: string) => {
    setDraft(d => {
      const current = d.limits?.allowed_tools ?? [];
      const next = current.includes(tool) ? current.filter(t => t !== tool) : [...current, tool];
      return { ...d, limits: { ...d.limits!, allowed_tools: next } };
    });
  };

  const usage = draft.usage ?? { tokens_today: 0, cost_today: 0, cost_month: 0 };
  const limits = draft.limits ?? { max_tokens_day: 0, max_cost_day: 0, max_cost_month: 0, max_concurrent_agents: 0, allowed_models: [], allowed_tools: [] };
  const settings = draft.settings ?? { default_model: '', default_provider: '', thinking_level: 'medium', temperature: 0.7, streaming: true, theme: 'dark' };

  return (
    <div className="flex flex-col h-full bg-zinc-900 border-l border-zinc-800 w-[480px] flex-shrink-0">
      {/* Header */}
      <div className="flex items-start justify-between px-5 py-4 border-b border-zinc-800">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-full bg-indigo-500/20 flex items-center justify-center text-indigo-400 font-semibold text-sm flex-shrink-0">
            {(draft.display_name || draft.username).slice(0, 2).toUpperCase()}
          </div>
          <div>
            <div className="text-sm font-semibold text-zinc-100">{draft.display_name || draft.username}</div>
            <div className="text-xs text-zinc-500">@{draft.username}</div>
          </div>
        </div>
        <button onClick={onClose} className="p-1.5 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors mt-0.5">
          <X size={16} />
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 px-4 py-2 border-b border-zinc-800 overflow-x-auto">
        {DETAIL_TABS.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap transition-all ${
              tab === id ? 'bg-indigo-500/15 text-indigo-400' : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800'
            }`}
          >
            <Icon size={12} />
            {label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-y-auto p-5 space-y-4" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
        {tab === 'profile' && (
          <div className="space-y-4">
            <div className="space-y-3">
              {[
                { label: 'Username', key: 'username' as const },
                { label: 'Display Name', key: 'display_name' as const },
                { label: 'Email', key: 'email' as const },
              ].map(({ label, key }) => (
                <div key={key} className="space-y-1">
                  <label className="text-xs text-zinc-400 font-medium">{label}</label>
                  <input
                    value={(draft[key] as string) || ''}
                    onChange={e => setDraft(d => ({ ...d, [key]: e.target.value }))}
                    className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                  />
                </div>
              ))}
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1">
                  <label className="text-xs text-zinc-400 font-medium">Role</label>
                  <select
                    value={draft.role}
                    onChange={e => setDraft(d => ({ ...d, role: e.target.value as UserRole }))}
                    className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                  >
                    {(Object.keys(ROLE_CONFIG) as UserRole[]).map(r => (
                      <option key={r} value={r}>{ROLE_CONFIG[r].label}</option>
                    ))}
                  </select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-zinc-400 font-medium">Team</label>
                  <select
                    value={draft.team_id ?? ''}
                    onChange={e => setDraft(d => ({ ...d, team_id: e.target.value || undefined }))}
                    className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                  >
                    <option value="">No team</option>
                    {teams.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
                  </select>
                </div>
              </div>
              <div className="space-y-1">
                <label className="text-xs text-zinc-400 font-medium">Status</label>
                <button
                  onClick={() => setDraft(d => ({ ...d, status: d.status === 'active' ? 'inactive' : 'active' }))}
                  className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors ${
                    draft.status === 'active'
                      ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                      : 'bg-zinc-800 text-zinc-500 border border-zinc-700'
                  }`}
                >
                  {draft.status === 'active'
                    ? <ToggleRight size={16} />
                    : <ToggleLeft size={16} />
                  }
                  {draft.status === 'active' ? 'Active' : 'Inactive'}
                </button>
              </div>
            </div>

            <div className="pt-2 border-t border-zinc-800">
              <div className="grid grid-cols-2 gap-2 text-xs text-zinc-500">
                <div>Created: <span className="text-zinc-400">{new Date(user.created_at).toLocaleDateString()}</span></div>
                {user.last_active && <div>Last active: <span className="text-zinc-400">{formatRelativeTime(user.last_active)}</span></div>}
              </div>
            </div>

            <button
              onClick={() => onDelete(user)}
              className="flex items-center gap-1.5 text-xs text-red-500 hover:text-red-400 transition-colors"
            >
              <Trash2 size={13} />
              Delete user
            </button>
          </div>
        )}

        {tab === 'settings' && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <label className="text-xs text-zinc-400 font-medium">Default Provider</label>
                <select
                  value={settings.default_provider}
                  onChange={e => setDraft(d => ({ ...d, settings: { ...d.settings!, default_provider: e.target.value } }))}
                  className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                >
                  {PROVIDERS.map(p => <option key={p} value={p}>{p}</option>)}
                </select>
              </div>
              <div className="space-y-1">
                <label className="text-xs text-zinc-400 font-medium">Default Model</label>
                <select
                  value={settings.default_model}
                  onChange={e => setDraft(d => ({ ...d, settings: { ...d.settings!, default_model: e.target.value } }))}
                  className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                >
                  {MODELS.map(m => <option key={m} value={m}>{m}</option>)}
                </select>
              </div>
            </div>
            <div className="space-y-1">
              <label className="text-xs text-zinc-400 font-medium">Thinking Level</label>
              <div className="flex gap-2">
                {THINKING_LEVELS.map(l => (
                  <button
                    key={l}
                    onClick={() => setDraft(d => ({ ...d, settings: { ...d.settings!, thinking_level: l as typeof settings.thinking_level } }))}
                    className={`flex-1 py-1.5 rounded-lg text-xs font-medium transition-all ${
                      settings.thinking_level === l
                        ? 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30'
                        : 'bg-zinc-800 text-zinc-500 border border-zinc-700 hover:text-zinc-300'
                    }`}
                  >
                    {l.charAt(0).toUpperCase() + l.slice(1)}
                  </button>
                ))}
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-xs text-zinc-400 font-medium">Temperature</label>
              <div className="flex items-center gap-3">
                <input
                  type="range" min="0" max="2" step="0.05"
                  value={settings.temperature}
                  onChange={e => setDraft(d => ({ ...d, settings: { ...d.settings!, temperature: Number(e.target.value) } }))}
                  className="flex-1 accent-indigo-500"
                />
                <span className="text-sm text-zinc-300 w-8 text-right font-mono">{settings.temperature.toFixed(2)}</span>
              </div>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs text-zinc-400 font-medium">Streaming</span>
              <button
                onClick={() => setDraft(d => ({ ...d, settings: { ...d.settings!, streaming: !d.settings!.streaming } }))}
                className={`relative w-9 h-5 rounded-full transition-colors ${settings.streaming ? 'bg-indigo-500' : 'bg-zinc-700'}`}
              >
                <span className={`absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${settings.streaming ? 'translate-x-4' : 'translate-x-0.5'}`} />
              </button>
            </div>
            <div className="space-y-1">
              <label className="text-xs text-zinc-400 font-medium">Theme</label>
              <div className="flex gap-2">
                {(['dark', 'light', 'system'] as const).map(t => (
                  <button
                    key={t}
                    onClick={() => setDraft(d => ({ ...d, settings: { ...d.settings!, theme: t } }))}
                    className={`flex-1 py-1.5 rounded-lg text-xs font-medium capitalize transition-all ${
                      settings.theme === t
                        ? 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30'
                        : 'bg-zinc-800 text-zinc-500 border border-zinc-700 hover:text-zinc-300'
                    }`}
                  >
                    {t}
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}

        {tab === 'limits' && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              {([
                { label: 'Max Tokens/Day', key: 'max_tokens_day' as const },
                { label: 'Max Cost/Day ($)', key: 'max_cost_day' as const },
                { label: 'Max Cost/Month ($)', key: 'max_cost_month' as const },
                { label: 'Max Concurrent Agents', key: 'max_concurrent_agents' as const },
              ] as { label: string; key: keyof typeof limits }[]).map(({ label, key }) => (
                <div key={key} className="space-y-1">
                  <label className="text-xs text-zinc-400 font-medium">{label}</label>
                  <input
                    type="number"
                    value={limits[key] as number}
                    onChange={e => setDraft(d => ({ ...d, limits: { ...d.limits!, [key]: Number(e.target.value) } }))}
                    className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                  />
                </div>
              ))}
            </div>
            <div className="space-y-2">
              <label className="text-xs text-zinc-400 font-medium">Allowed Models</label>
              <div className="flex flex-wrap gap-2">
                {MODELS.map(m => {
                  const active = limits.allowed_models.includes(m);
                  return (
                    <button
                      key={m}
                      onClick={() => toggleModel(m)}
                      className={`flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs transition-all ${
                        active ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30' : 'bg-zinc-800 text-zinc-500 border border-zinc-700 hover:text-zinc-300'
                      }`}
                    >
                      {active ? <CheckSquare size={11} /> : <Square size={11} />}
                      {m}
                    </button>
                  );
                })}
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-xs text-zinc-400 font-medium">Allowed Tools</label>
              <div className="flex flex-wrap gap-2">
                {ALL_TOOLS.map(t => {
                  const active = limits.allowed_tools.includes(t);
                  return (
                    <button
                      key={t}
                      onClick={() => toggleTool(t)}
                      className={`flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-mono transition-all ${
                        active ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30' : 'bg-zinc-800 text-zinc-500 border border-zinc-700 hover:text-zinc-300'
                      }`}
                    >
                      {active ? <CheckSquare size={11} /> : <Square size={11} />}
                      {t}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        )}

        {tab === 'usage' && (
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-3">
              <div className="p-3 rounded-xl bg-zinc-800/50 border border-zinc-700/40 space-y-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Cpu size={13} className="text-zinc-500" />
                    <span className="text-xs text-zinc-400">Tokens Today</span>
                  </div>
                  <div className="text-xs text-zinc-200 font-mono">
                    {usage.tokens_today.toLocaleString()} / {limits.max_tokens_day.toLocaleString()}
                  </div>
                </div>
                <ProgressBar value={usage.tokens_today} max={limits.max_tokens_day} />
              </div>
              <div className="p-3 rounded-xl bg-zinc-800/50 border border-zinc-700/40 space-y-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <DollarSign size={13} className="text-zinc-500" />
                    <span className="text-xs text-zinc-400">Cost Today</span>
                  </div>
                  <div className="text-xs text-zinc-200 font-mono">
                    {formatCost(usage.cost_today)} / {formatCost(limits.max_cost_day)}
                  </div>
                </div>
                <ProgressBar value={usage.cost_today} max={limits.max_cost_day} color="bg-emerald-500" />
              </div>
              <div className="p-3 rounded-xl bg-zinc-800/50 border border-zinc-700/40 space-y-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <DollarSign size={13} className="text-zinc-500" />
                    <span className="text-xs text-zinc-400">Cost This Month</span>
                  </div>
                  <div className="text-xs text-zinc-200 font-mono">
                    {formatCost(usage.cost_month)} / {formatCost(limits.max_cost_month)}
                  </div>
                </div>
                <ProgressBar value={usage.cost_month} max={limits.max_cost_month} color="bg-violet-500" />
              </div>
            </div>
          </div>
        )}

        {tab === 'apikey' && (
          <div className="space-y-4">
            <div className="p-4 rounded-xl bg-zinc-800/50 border border-zinc-700/40 space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-xs text-zinc-400 font-medium">API Key</span>
                <button
                  onClick={() => setShowApiKey(v => !v)}
                  className="text-xs text-zinc-500 hover:text-zinc-300 flex items-center gap-1 transition-colors"
                >
                  {showApiKey ? <EyeOff size={12} /> : <Eye size={12} />}
                  {showApiKey ? 'Hide' : 'Show'}
                </button>
              </div>
              <code className={`block font-mono text-xs text-zinc-300 bg-zinc-900/60 rounded-lg px-3 py-2 break-all ${!showApiKey ? 'tracking-widest' : ''}`}>
                {showApiKey ? (draft.api_key_masked ?? 'sg_not_set') : '••••••••••••••••••••••••••••••••••••'}
              </code>
            </div>
            <div className="space-y-2">
              <p className="text-xs text-zinc-500">
                Regenerating the API key will immediately invalidate the current key. Any integrations using the old key will stop working.
              </p>
              {regenConfirm ? (
                <div className="p-3 rounded-xl bg-amber-500/5 border border-amber-500/20 space-y-3">
                  <p className="text-xs text-amber-400">Are you sure? The current key will be immediately revoked.</p>
                  <div className="flex gap-2">
                    <button onClick={() => setRegenConfirm(false)} className="flex-1 py-1.5 rounded-lg text-xs text-zinc-400 bg-zinc-800 hover:bg-zinc-700 transition-colors">
                      Cancel
                    </button>
                    <button onClick={handleRegen} disabled={regening} className="flex-1 py-1.5 rounded-lg text-xs bg-amber-600 hover:bg-amber-500 text-white transition-colors disabled:opacity-60">
                      {regening ? 'Regenerating…' : 'Yes, regenerate'}
                    </button>
                  </div>
                </div>
              ) : (
                <button
                  onClick={handleRegen}
                  className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs transition-colors"
                >
                  <RefreshCw size={13} />
                  Regenerate Key
                </button>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Footer actions */}
      <div className="px-5 py-3 border-t border-zinc-800 flex justify-end gap-2">
        <button onClick={onClose} className="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors">
          Close
        </button>
        <button
          onClick={save}
          disabled={saving}
          className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors disabled:opacity-60"
        >
          {saving ? <RefreshCw size={13} className="animate-spin" /> : <Save size={13} />}
          {saving ? 'Saving…' : 'Save Changes'}
        </button>
      </div>
    </div>
  );
}

// ── Main View ─────────────────────────────────────────────────────────────────

export default function UsersView() {
  const [users, setUsers] = useState<User[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [roleFilter, setRoleFilter] = useState<UserRole | 'all'>('all');
  const [statusFilter, setStatusFilter] = useState<UserStatus | 'all'>('all');
  const [sortField, setSortField] = useState<'username' | 'role' | 'last_active' | 'status'>('username');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [u, t] = await Promise.all([fetchUsers(), fetchTeams()]);
      setUsers(u);
      setTeams(t);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = users
    .filter(u => {
      const q = search.toLowerCase();
      const matchSearch = !q || u.username.includes(q) || u.display_name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q);
      const matchRole = roleFilter === 'all' || u.role === roleFilter;
      const matchStatus = statusFilter === 'all' || u.status === statusFilter;
      return matchSearch && matchRole && matchStatus;
    })
    .sort((a, b) => {
      let cmp = 0;
      if (sortField === 'username') cmp = a.username.localeCompare(b.username);
      else if (sortField === 'role') cmp = a.role.localeCompare(b.role);
      else if (sortField === 'status') cmp = a.status.localeCompare(b.status);
      else if (sortField === 'last_active') {
        const aTime = a.last_active ? new Date(a.last_active).getTime() : 0;
        const bTime = b.last_active ? new Date(b.last_active).getTime() : 0;
        cmp = aTime - bTime;
      }
      return sortDir === 'asc' ? cmp : -cmp;
    });

  const toggleSort = (field: typeof sortField) => {
    if (sortField === field) setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    else { setSortField(field); setSortDir('asc'); }
  };

  const SortIcon = ({ field }: { field: typeof sortField }) => {
    if (sortField !== field) return null;
    return sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />;
  };

  const toggleSelect = (id: string) => {
    setSelected(s => {
      const next = new Set(s);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

  const selectAll = () => {
    if (selected.size === filtered.length) setSelected(new Set());
    else setSelected(new Set(filtered.map(u => u.id)));
  };

  const bulkActivate = async (status: UserStatus) => {
    const ids = [...selected];
    await Promise.all(ids.map(id => updateUser(id, { status })));
    setUsers(us => us.map(u => selected.has(u.id) ? { ...u, status } : u));
    setSelected(new Set());
    toast.success(`${ids.length} users ${status === 'active' ? 'activated' : 'deactivated'}`);
  };

  const handleCreated = (user: User) => {
    setUsers(us => [...us, user]);
    setShowCreate(false);
  };

  const handleUpdate = (updated: User) => {
    setUsers(us => us.map(u => u.id === updated.id ? updated : u));
    if (selectedUser?.id === updated.id) setSelectedUser(updated);
  };

  const handleDelete = async (user: User) => {
    setDeleteTarget(null);
    await deleteUser(user.id);
    setUsers(us => us.filter(u => u.id !== user.id));
    if (selectedUser?.id === user.id) setSelectedUser(null);
    toast.success(`User ${user.username} deleted`);
  };

  return (
    <div className="flex flex-1 overflow-hidden">
      {/* Main panel */}
      <div className="flex flex-col flex-1 overflow-hidden bg-zinc-950">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800 flex-shrink-0">
          <div>
            <h2 className="text-lg font-bold text-zinc-100">Users</h2>
            <p className="text-sm text-zinc-500">{users.length} total members</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={load}
              className="p-2 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors"
              title="Refresh"
            >
              <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            </button>
            <button
              onClick={() => setShowCreate(true)}
              className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors"
            >
              <Plus size={14} />
              Create User
            </button>
          </div>
        </div>

        {/* Filters */}
        <div className="flex items-center gap-3 px-6 py-3 border-b border-zinc-800/50 flex-shrink-0">
          <div className="relative flex-1 max-w-xs">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
            <input
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search users…"
              className="w-full pl-8 pr-3 py-2 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/50"
            />
          </div>
          <select
            value={roleFilter}
            onChange={e => setRoleFilter(e.target.value as typeof roleFilter)}
            className="px-3 py-2 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-300 focus:outline-none focus:border-indigo-500/50"
          >
            <option value="all">All roles</option>
            {(Object.keys(ROLE_CONFIG) as UserRole[]).map(r => (
              <option key={r} value={r}>{ROLE_CONFIG[r].label}</option>
            ))}
          </select>
          <select
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value as typeof statusFilter)}
            className="px-3 py-2 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-300 focus:outline-none focus:border-indigo-500/50"
          >
            <option value="all">All statuses</option>
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
          </select>
          {selected.size > 0 && (
            <div className="flex items-center gap-2 ml-auto">
              <span className="text-xs text-zinc-500">{selected.size} selected</span>
              <button onClick={() => bulkActivate('active')} className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 hover:bg-emerald-500/20 transition-colors">
                <Activity size={11} /> Activate
              </button>
              <button onClick={() => bulkActivate('inactive')} className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs bg-zinc-800 text-zinc-400 border border-zinc-700 hover:bg-zinc-700 transition-colors">
                <ToggleLeft size={11} /> Deactivate
              </button>
            </div>
          )}
        </div>

        {/* Table */}
        <div className="flex-1 overflow-y-auto" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
          {loading ? (
            <div className="flex items-center justify-center h-40 text-zinc-500 gap-2">
              <RefreshCw size={16} className="animate-spin" />
              Loading users…
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-40 text-zinc-600 gap-2">
              <Users size={32} strokeWidth={1.5} />
              <p className="text-sm">No users found</p>
            </div>
          ) : (
            <table className="w-full">
              <thead className="sticky top-0 z-10">
                <tr className="bg-zinc-900/80 backdrop-blur-sm border-b border-zinc-800">
                  <th className="w-10 px-4 py-3">
                    <button onClick={selectAll} className="text-zinc-500 hover:text-zinc-300">
                      {selected.size === filtered.length && filtered.length > 0
                        ? <CheckSquare size={14} className="text-indigo-400" />
                        : <Square size={14} />
                      }
                    </button>
                  </th>
                  <th className="px-3 py-3 text-left">
                    <button onClick={() => toggleSort('username')} className="flex items-center gap-1 text-xs font-medium text-zinc-400 hover:text-zinc-200 transition-colors">
                      User <SortIcon field="username" />
                    </button>
                  </th>
                  <th className="px-3 py-3 text-left">
                    <button onClick={() => toggleSort('role')} className="flex items-center gap-1 text-xs font-medium text-zinc-400 hover:text-zinc-200 transition-colors">
                      Role <SortIcon field="role" />
                    </button>
                  </th>
                  <th className="px-3 py-3 text-left text-xs font-medium text-zinc-400">Team</th>
                  <th className="px-3 py-3 text-left">
                    <button onClick={() => toggleSort('last_active')} className="flex items-center gap-1 text-xs font-medium text-zinc-400 hover:text-zinc-200 transition-colors">
                      Last Active <SortIcon field="last_active" />
                    </button>
                  </th>
                  <th className="px-3 py-3 text-left">
                    <button onClick={() => toggleSort('status')} className="flex items-center gap-1 text-xs font-medium text-zinc-400 hover:text-zinc-200 transition-colors">
                      Status <SortIcon field="status" />
                    </button>
                  </th>
                  <th className="px-3 py-3 text-left text-xs font-medium text-zinc-400">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-800/50">
                {filtered.map(user => (
                  <tr
                    key={user.id}
                    onClick={() => setSelectedUser(selectedUser?.id === user.id ? null : user)}
                    className={`cursor-pointer transition-colors ${
                      selectedUser?.id === user.id
                        ? 'bg-indigo-500/5 border-l-2 border-l-indigo-500'
                        : 'hover:bg-zinc-900/40'
                    }`}
                  >
                    <td className="px-4 py-3" onClick={e => { e.stopPropagation(); toggleSelect(user.id); }}>
                      <button className="text-zinc-500 hover:text-zinc-300">
                        {selected.has(user.id)
                          ? <CheckSquare size={14} className="text-indigo-400" />
                          : <Square size={14} />
                        }
                      </button>
                    </td>
                    <td className="px-3 py-3">
                      <div className="flex items-center gap-2.5">
                        <div className="w-7 h-7 rounded-full bg-zinc-800 flex items-center justify-center text-zinc-400 text-xs font-medium flex-shrink-0">
                          {(user.display_name || user.username).slice(0, 2).toUpperCase()}
                        </div>
                        <div>
                          <div className="text-sm font-medium text-zinc-200">{user.display_name || user.username}</div>
                          <div className="text-xs text-zinc-500">{user.email}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-3"><RoleBadge role={user.role} /></td>
                    <td className="px-3 py-3">
                      {user.team_name
                        ? <span className="text-xs text-zinc-300">{user.team_name}</span>
                        : <span className="text-xs text-zinc-600">—</span>
                      }
                    </td>
                    <td className="px-3 py-3">
                      <div className="flex items-center gap-1 text-xs text-zinc-500">
                        <Clock size={11} />
                        {user.last_active ? formatRelativeTime(user.last_active) : 'Never'}
                      </div>
                    </td>
                    <td className="px-3 py-3"><StatusDot status={user.status} /></td>
                    <td className="px-3 py-3" onClick={e => e.stopPropagation()}>
                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => setSelectedUser(selectedUser?.id === user.id ? null : user)}
                          className="p-1.5 rounded-lg text-zinc-600 hover:text-indigo-400 hover:bg-indigo-500/10 transition-colors"
                          title="Edit user"
                        >
                          <Settings size={13} />
                        </button>
                        <button
                          onClick={() => setDeleteTarget(user)}
                          className="p-1.5 rounded-lg text-zinc-600 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                          title="Delete user"
                        >
                          <Trash2 size={13} />
                        </button>
                        <ChevronRight size={13} className="text-zinc-700" />
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Detail panel */}
      {selectedUser && (
        <UserDetail
          user={selectedUser}
          teams={teams}
          onClose={() => setSelectedUser(null)}
          onUpdate={handleUpdate}
          onDelete={u => setDeleteTarget(u)}
        />
      )}

      {/* Modals */}
      {showCreate && (
        <CreateUserModal
          teams={teams}
          onClose={() => setShowCreate(false)}
          onCreated={handleCreated}
        />
      )}
      {deleteTarget && (
        <DeleteConfirm
          user={deleteTarget}
          onClose={() => setDeleteTarget(null)}
          onConfirm={() => handleDelete(deleteTarget)}
        />
      )}
    </div>
  );
}
