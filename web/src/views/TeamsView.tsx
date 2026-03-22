import { useState, useEffect, useCallback } from 'react';
import {
  Building2, Plus, RefreshCw, Users, DollarSign, Bot, Trash2,
  X, AlertTriangle, Search, Settings,
} from 'lucide-react';
import {
  fetchTeams, createTeam, deleteTeam, fetchUsers,
  addTeamMember, removeTeamMember,
  type Team, type User, type CreateTeamPayload,
} from '../lib/api';
import { formatCost } from '../lib/utils';
import toast from 'react-hot-toast';

// ── Role badge ────────────────────────────────────────────────────────────────

const ROLE_COLORS: Record<string, string> = {
  admin:     'text-red-400 bg-red-500/10 border-red-500/20',
  developer: 'text-blue-400 bg-blue-500/10 border-blue-500/20',
  operator:  'text-amber-400 bg-amber-500/10 border-amber-500/20',
  viewer:    'text-zinc-400 bg-zinc-500/10 border-zinc-500/20',
};

function RoleBadge({ role }: { role: string }) {
  const cls = ROLE_COLORS[role] ?? 'text-zinc-400 bg-zinc-500/10 border-zinc-500/20';
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${cls} capitalize`}>
      {role}
    </span>
  );
}

// ── Create Team Modal ─────────────────────────────────────────────────────────

interface CreateTeamModalProps {
  onClose: () => void;
  onCreated: (team: Team) => void;
}

function CreateTeamModal({ onClose, onCreated }: CreateTeamModalProps) {
  const [form, setForm] = useState<CreateTeamPayload>({ name: '', description: '' });
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) {
      toast.error('Team name is required');
      return;
    }
    setSaving(true);
    try {
      const team = await createTeam(form);
      onCreated(team);
      toast.success(`Team "${team.name}" created`);
    } catch {
      toast.error('Failed to create team');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-zinc-900 border border-zinc-700/60 rounded-2xl w-full max-w-sm shadow-2xl">
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800">
          <h3 className="text-sm font-semibold text-zinc-100">Create Team</h3>
          <button onClick={onClose} className="p-1 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors">
            <X size={16} />
          </button>
        </div>
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Team Name *</label>
            <input
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
              placeholder="Platform"
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
          <div className="space-y-1">
            <label className="text-xs text-zinc-400 font-medium">Description</label>
            <textarea
              value={form.description ?? ''}
              onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
              placeholder="Team description…"
              rows={2}
              className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 resize-none"
            />
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
              {saving ? 'Creating…' : 'Create Team'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Delete confirmation ───────────────────────────────────────────────────────

interface DeleteConfirmProps {
  team: Team;
  onClose: () => void;
  onConfirm: () => void;
}

function DeleteConfirm({ team, onClose, onConfirm }: DeleteConfirmProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-zinc-900 border border-zinc-700/60 rounded-2xl w-full max-w-sm shadow-2xl p-6">
        <div className="flex items-start gap-3 mb-4">
          <div className="p-2 rounded-lg bg-red-500/10 flex-shrink-0">
            <AlertTriangle size={18} className="text-red-400" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-zinc-100 mb-1">Delete team</h3>
            <p className="text-xs text-zinc-400">
              Are you sure you want to delete <span className="text-zinc-200 font-medium">{team.name}</span>? Members will be unassigned.
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

// ── Team Detail Panel ─────────────────────────────────────────────────────────

interface TeamDetailProps {
  team: Team;
  allUsers: User[];
  onClose: () => void;
  onDelete: (team: Team) => void;
  onMembersChanged: () => void;
}

function TeamDetail({ team, allUsers, onClose, onDelete, onMembersChanged }: TeamDetailProps) {
  const [activeTab, setActiveTab] = useState<'members' | 'limits' | 'analytics'>('members');
  const [search, setSearch] = useState('');
  const [addingUsers, setAddingUsers] = useState<Set<string>>(new Set());
  const [removingUsers, setRemovingUsers] = useState<Set<string>>(new Set());

  const teamMembers = allUsers.filter(u => u.team_id === team.id);
  const nonMembers = allUsers.filter(u => u.team_id !== team.id)
    .filter(u => !search || u.username.toLowerCase().includes(search.toLowerCase()) || u.display_name.toLowerCase().includes(search.toLowerCase()));

  const handleAdd = async (user: User) => {
    setAddingUsers(s => new Set([...s, user.id]));
    try {
      await addTeamMember(team.id, user.id);
      onMembersChanged();
      toast.success(`${user.display_name || user.username} added to ${team.name}`);
    } catch {
      toast.error('Failed to add member');
    } finally {
      setAddingUsers(s => { const n = new Set(s); n.delete(user.id); return n; });
    }
  };

  const handleRemove = async (user: User) => {
    setRemovingUsers(s => new Set([...s, user.id]));
    try {
      await removeTeamMember(team.id, user.id);
      onMembersChanged();
      toast.success(`${user.display_name || user.username} removed from ${team.name}`);
    } catch {
      toast.error('Failed to remove member');
    } finally {
      setRemovingUsers(s => { const n = new Set(s); n.delete(user.id); return n; });
    }
  };

  const limits = team.limits ?? { max_cost_month: 0, max_concurrent_agents: 0, allowed_models: [] };

  // Mock analytics per team member
  const analytics = teamMembers.map(u => ({
    user: u,
    cost: u.usage?.cost_month ?? 0,
    tokens: u.usage?.tokens_today ?? 0,
  })).sort((a, b) => b.cost - a.cost);

  return (
    <div className="flex flex-col h-full bg-zinc-900 border-l border-zinc-800 w-[420px] flex-shrink-0">
      {/* Header */}
      <div className="flex items-start justify-between px-5 py-4 border-b border-zinc-800">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-indigo-500/20 flex items-center justify-center flex-shrink-0">
            <Building2 size={18} className="text-indigo-400" />
          </div>
          <div>
            <div className="text-sm font-semibold text-zinc-100">{team.name}</div>
            {team.description && <div className="text-xs text-zinc-500">{team.description}</div>}
          </div>
        </div>
        <button onClick={onClose} className="p-1.5 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors mt-0.5">
          <X size={16} />
        </button>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-3 divide-x divide-zinc-800 border-b border-zinc-800">
        {[
          { icon: Users, label: 'Members', value: String(team.member_count) },
          { icon: Bot, label: 'Agents', value: String(team.active_agents) },
          { icon: DollarSign, label: 'Month', value: formatCost(team.total_cost_month) },
        ].map(({ icon: Icon, label, value }) => (
          <div key={label} className="flex flex-col items-center py-3 gap-1">
            <Icon size={14} className="text-zinc-500" />
            <div className="text-sm font-semibold text-zinc-200">{value}</div>
            <div className="text-xs text-zinc-600">{label}</div>
          </div>
        ))}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 px-4 py-2 border-b border-zinc-800">
        {(['members', 'limits', 'analytics'] as const).map(t => (
          <button
            key={t}
            onClick={() => setActiveTab(t)}
            className={`px-3 py-1.5 rounded-lg text-xs font-medium capitalize transition-all ${
              activeTab === t ? 'bg-indigo-500/15 text-indigo-400' : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
        {activeTab === 'members' && (
          <>
            {/* Current members */}
            <div className="space-y-2">
              <h4 className="text-xs font-medium text-zinc-400 uppercase tracking-wider">Members ({teamMembers.length})</h4>
              {teamMembers.length === 0 ? (
                <p className="text-xs text-zinc-600 py-2">No members yet.</p>
              ) : (
                <div className="space-y-1">
                  {teamMembers.map(user => (
                    <div key={user.id} className="flex items-center justify-between p-2.5 rounded-lg bg-zinc-800/50 border border-zinc-700/30">
                      <div className="flex items-center gap-2.5">
                        <div className="w-7 h-7 rounded-full bg-zinc-700 flex items-center justify-center text-zinc-400 text-xs font-medium">
                          {(user.display_name || user.username).slice(0, 2).toUpperCase()}
                        </div>
                        <div>
                          <div className="text-sm text-zinc-200">{user.display_name || user.username}</div>
                          <div className="text-xs text-zinc-500">@{user.username}</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <RoleBadge role={user.role} />
                        <button
                          onClick={() => handleRemove(user)}
                          disabled={removingUsers.has(user.id)}
                          className="p-1.5 rounded-lg text-zinc-600 hover:text-red-400 hover:bg-red-500/10 transition-colors disabled:opacity-50"
                          title="Remove from team"
                        >
                          {removingUsers.has(user.id) ? <RefreshCw size={12} className="animate-spin" /> : <X size={12} />}
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Add members */}
            <div className="space-y-2">
              <h4 className="text-xs font-medium text-zinc-400 uppercase tracking-wider">Add Members</h4>
              <div className="relative">
                <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
                <input
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  placeholder="Search users to add…"
                  className="w-full pl-8 pr-3 py-2 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/50"
                />
              </div>
              {nonMembers.length === 0 ? (
                <p className="text-xs text-zinc-600 py-2">
                  {search ? 'No users found' : 'All users are already members'}
                </p>
              ) : (
                <div className="space-y-1">
                  {nonMembers.slice(0, 8).map(user => (
                    <div key={user.id} className="flex items-center justify-between p-2.5 rounded-lg hover:bg-zinc-800/50 border border-transparent hover:border-zinc-700/30 transition-all">
                      <div className="flex items-center gap-2.5">
                        <div className="w-7 h-7 rounded-full bg-zinc-700/50 flex items-center justify-center text-zinc-500 text-xs font-medium">
                          {(user.display_name || user.username).slice(0, 2).toUpperCase()}
                        </div>
                        <div>
                          <div className="text-sm text-zinc-300">{user.display_name || user.username}</div>
                          <div className="text-xs text-zinc-600">@{user.username}</div>
                        </div>
                      </div>
                      <button
                        onClick={() => handleAdd(user)}
                        disabled={addingUsers.has(user.id)}
                        className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 hover:bg-indigo-500/20 transition-colors disabled:opacity-50"
                      >
                        {addingUsers.has(user.id) ? <RefreshCw size={11} className="animate-spin" /> : <Plus size={11} />}
                        Add
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )}

        {activeTab === 'limits' && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <div className="p-3 rounded-xl bg-zinc-800/50 border border-zinc-700/40">
                <div className="text-xs text-zinc-500 mb-1">Max Cost/Month</div>
                <div className="text-lg font-semibold text-zinc-100">{formatCost(limits.max_cost_month)}</div>
                <div className="text-xs text-zinc-600 mt-1">
                  Used: {formatCost(team.total_cost_month)}
                </div>
                <div className="mt-2 h-1.5 bg-zinc-700 rounded-full">
                  <div
                    className={`h-full rounded-full ${
                      limits.max_cost_month > 0 && team.total_cost_month / limits.max_cost_month > 0.9
                        ? 'bg-red-500' : 'bg-indigo-500'
                    }`}
                    style={{ width: `${limits.max_cost_month > 0 ? Math.min(100, (team.total_cost_month / limits.max_cost_month) * 100) : 0}%` }}
                  />
                </div>
              </div>
              <div className="p-3 rounded-xl bg-zinc-800/50 border border-zinc-700/40">
                <div className="text-xs text-zinc-500 mb-1">Max Concurrent Agents</div>
                <div className="text-lg font-semibold text-zinc-100">{limits.max_concurrent_agents}</div>
                <div className="text-xs text-zinc-600 mt-1">Running: {team.active_agents}</div>
              </div>
            </div>
            <div className="space-y-2">
              <div className="text-xs font-medium text-zinc-400">Allowed Models ({limits.allowed_models.length})</div>
              {limits.allowed_models.length === 0 ? (
                <p className="text-xs text-zinc-600">All models allowed</p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {limits.allowed_models.map(m => (
                    <span key={m} className="px-2.5 py-1 rounded-lg text-xs bg-zinc-800 text-zinc-300 border border-zinc-700 font-mono">
                      {m}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === 'analytics' && (
          <div className="space-y-4">
            <h4 className="text-xs font-medium text-zinc-400 uppercase tracking-wider">Cost by Member (this month)</h4>
            {analytics.length === 0 ? (
              <p className="text-xs text-zinc-600">No data yet</p>
            ) : (
              <div className="space-y-2">
                {analytics.map(({ user, cost }) => {
                  const pct = team.total_cost_month > 0 ? (cost / team.total_cost_month) * 100 : 0;
                  return (
                    <div key={user.id} className="space-y-1">
                      <div className="flex items-center justify-between text-xs">
                        <span className="text-zinc-300">{user.display_name || user.username}</span>
                        <span className="text-zinc-400 font-mono">{formatCost(cost)}</span>
                      </div>
                      <div className="h-1.5 bg-zinc-800 rounded-full">
                        <div className="h-full bg-indigo-500 rounded-full" style={{ width: `${pct}%` }} />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="px-5 py-3 border-t border-zinc-800 flex justify-between">
        <button
          onClick={() => onDelete(team)}
          className="flex items-center gap-1.5 text-xs text-red-500 hover:text-red-400 transition-colors"
        >
          <Trash2 size={13} />
          Delete team
        </button>
        <button onClick={onClose} className="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors">
          Close
        </button>
      </div>
    </div>
  );
}

// ── Team Card ─────────────────────────────────────────────────────────────────

interface TeamCardProps {
  team: Team;
  selected: boolean;
  onClick: () => void;
  onDelete: () => void;
}

function TeamCard({ team, selected, onClick, onDelete }: TeamCardProps) {
  const usagePct = team.limits?.max_cost_month
    ? Math.min(100, (team.total_cost_month / team.limits.max_cost_month) * 100)
    : 0;

  return (
    <div
      onClick={onClick}
      className={`group relative cursor-pointer rounded-2xl border p-5 transition-all ${
        selected
          ? 'bg-indigo-500/5 border-indigo-500/30 shadow-lg shadow-indigo-500/5'
          : 'bg-zinc-900/40 border-zinc-800/60 hover:border-zinc-700/80 hover:bg-zinc-900/70'
      }`}
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-3">
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center transition-colors ${
            selected ? 'bg-indigo-500/20' : 'bg-zinc-800/80 group-hover:bg-zinc-800'
          }`}>
            <Building2 size={18} className={selected ? 'text-indigo-400' : 'text-zinc-500'} />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-zinc-100">{team.name}</h3>
            {team.description && (
              <p className="text-xs text-zinc-500 mt-0.5 max-w-[180px] truncate">{team.description}</p>
            )}
          </div>
        </div>
        <div className="flex gap-1">
          <button
            onClick={e => { e.stopPropagation(); onClick(); }}
            className="p-1.5 rounded-lg text-zinc-600 hover:text-zinc-300 hover:bg-zinc-800 transition-colors opacity-0 group-hover:opacity-100"
            title="View details"
          >
            <Settings size={13} />
          </button>
          <button
            onClick={e => { e.stopPropagation(); onDelete(); }}
            className="p-1.5 rounded-lg text-zinc-600 hover:text-red-400 hover:bg-red-500/10 transition-colors opacity-0 group-hover:opacity-100"
            title="Delete"
          >
            <Trash2 size={13} />
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3 mb-4">
        {[
          { icon: Users, label: 'Members', value: team.member_count },
          { icon: Bot, label: 'Agents', value: team.active_agents },
          { icon: DollarSign, label: 'Month', value: formatCost(team.total_cost_month) },
        ].map(({ icon: Icon, label, value }) => (
          <div key={label} className="text-center p-2 rounded-xl bg-zinc-800/50 border border-zinc-700/30">
            <Icon size={13} className="text-zinc-500 mx-auto mb-1" />
            <div className="text-sm font-semibold text-zinc-200">{value}</div>
            <div className="text-xs text-zinc-600">{label}</div>
          </div>
        ))}
      </div>

      {/* Cost bar */}
      {team.limits?.max_cost_month && (
        <div className="space-y-1">
          <div className="flex justify-between text-xs text-zinc-600">
            <span>Budget</span>
            <span>{Math.round(usagePct)}% used</span>
          </div>
          <div className="h-1.5 bg-zinc-800 rounded-full overflow-hidden">
            <div
              className={`h-full rounded-full transition-all ${
                usagePct > 90 ? 'bg-red-500' : usagePct > 70 ? 'bg-amber-500' : 'bg-indigo-500'
              }`}
              style={{ width: `${usagePct}%` }}
            />
          </div>
        </div>
      )}

      {selected && (
        <div className="absolute right-2 top-2 w-2 h-2 rounded-full bg-indigo-400" />
      )}
    </div>
  );
}

// ── Main View ─────────────────────────────────────────────────────────────────

export default function TeamsView() {
  const [teams, setTeams] = useState<Team[]>([]);
  const [allUsers, setAllUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedTeam, setSelectedTeam] = useState<Team | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Team | null>(null);
  const [search, setSearch] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [t, u] = await Promise.all([fetchTeams(), fetchUsers()]);
      setTeams(t);
      setAllUsers(u);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = teams.filter(t =>
    !search || t.name.toLowerCase().includes(search.toLowerCase()) || (t.description ?? '').toLowerCase().includes(search.toLowerCase())
  );

  const handleCreated = (team: Team) => {
    setTeams(ts => [...ts, team]);
    setShowCreate(false);
    setSelectedTeam(team);
  };

  const handleDelete = async (team: Team) => {
    setDeleteTarget(null);
    await deleteTeam(team.id);
    setTeams(ts => ts.filter(t => t.id !== team.id));
    if (selectedTeam?.id === team.id) setSelectedTeam(null);
    toast.success(`Team "${team.name}" deleted`);
  };

  // Total stats
  const totalMembers = teams.reduce((s, t) => s + t.member_count, 0);
  const totalCost = teams.reduce((s, t) => s + t.total_cost_month, 0);

  return (
    <div className="flex flex-1 overflow-hidden">
      {/* Main panel */}
      <div className="flex flex-col flex-1 overflow-hidden bg-zinc-950">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800 flex-shrink-0">
          <div>
            <h2 className="text-lg font-bold text-zinc-100">Teams</h2>
            <p className="text-sm text-zinc-500">{teams.length} teams, {totalMembers} members</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={load}
              className="p-2 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors"
            >
              <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            </button>
            <button
              onClick={() => setShowCreate(true)}
              className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors"
            >
              <Plus size={14} />
              Create Team
            </button>
          </div>
        </div>

        {/* Summary cards */}
        <div className="grid grid-cols-3 gap-4 px-6 py-4 border-b border-zinc-800/50 flex-shrink-0">
          {[
            { icon: Building2, label: 'Total Teams', value: String(teams.length), color: 'text-indigo-400' },
            { icon: Users, label: 'Total Members', value: String(totalMembers), color: 'text-sky-400' },
            { icon: DollarSign, label: 'Total Cost/Month', value: formatCost(totalCost), color: 'text-emerald-400' },
          ].map(({ icon: Icon, label, value, color }) => (
            <div key={label} className="flex items-center gap-3 p-4 rounded-xl bg-zinc-900/40 border border-zinc-800/60">
              <div className="p-2 rounded-lg bg-zinc-800/80">
                <Icon size={16} className={color} />
              </div>
              <div>
                <div className="text-lg font-bold text-zinc-100">{value}</div>
                <div className="text-xs text-zinc-500">{label}</div>
              </div>
            </div>
          ))}
        </div>

        {/* Search */}
        <div className="px-6 py-3 border-b border-zinc-800/50 flex-shrink-0">
          <div className="relative max-w-xs">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
            <input
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search teams…"
              className="w-full pl-8 pr-3 py-2 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/50"
            />
          </div>
        </div>

        {/* Team grid */}
        <div className="flex-1 overflow-y-auto p-6" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
          {loading ? (
            <div className="flex items-center justify-center h-40 text-zinc-500 gap-2">
              <RefreshCw size={16} className="animate-spin" />
              Loading teams…
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-40 text-zinc-600 gap-2">
              <Building2 size={32} strokeWidth={1.5} />
              <p className="text-sm">No teams found</p>
              <button
                onClick={() => setShowCreate(true)}
                className="flex items-center gap-1.5 text-xs text-indigo-400 hover:text-indigo-300 transition-colors mt-1"
              >
                <Plus size={13} />
                Create a team
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {filtered.map(team => (
                <TeamCard
                  key={team.id}
                  team={team}
                  selected={selectedTeam?.id === team.id}
                  onClick={() => setSelectedTeam(selectedTeam?.id === team.id ? null : team)}
                  onDelete={() => setDeleteTarget(team)}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Detail panel */}
      {selectedTeam && (
        <TeamDetail
          team={selectedTeam}
          allUsers={allUsers}
          onClose={() => setSelectedTeam(null)}
          onDelete={t => setDeleteTarget(t)}
          onMembersChanged={load}
        />
      )}

      {/* Modals */}
      {showCreate && (
        <CreateTeamModal
          onClose={() => setShowCreate(false)}
          onCreated={handleCreated}
        />
      )}
      {deleteTarget && (
        <DeleteConfirm
          team={deleteTarget}
          onClose={() => setDeleteTarget(null)}
          onConfirm={() => handleDelete(deleteTarget)}
        />
      )}
    </div>
  );
}
