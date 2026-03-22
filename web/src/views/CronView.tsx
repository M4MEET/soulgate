import { useEffect, useState, useCallback } from 'react';
import {
  Clock, Plus, Play, Pause, Trash2, RefreshCw,
  ChevronDown, CheckCircle, XCircle, AlertCircle,
  Zap,
} from 'lucide-react';
import {
  fetchCronJobs, createCronJob, deleteCronJob,
  toggleCronJob, runCronJobNow, type CronJob,
} from '../lib/api';
import toast from 'react-hot-toast';

// ── Schedule presets ──────────────────────────────────────────────────────────

const SCHEDULE_PRESETS = [
  { label: 'Every minute',    value: '* * * * *' },
  { label: 'Every 5 minutes', value: '*/5 * * * *' },
  { label: 'Every 15 minutes',value: '*/15 * * * *' },
  { label: 'Every 30 minutes',value: '*/30 * * * *' },
  { label: 'Every hour',      value: '0 * * * *' },
  { label: 'Every 6 hours',   value: '0 */6 * * *' },
  { label: 'Every 12 hours',  value: '0 */12 * * *' },
  { label: 'Every day at midnight', value: '0 0 * * *' },
  { label: 'Every day at noon',     value: '0 12 * * *' },
  { label: 'Every Monday',    value: '0 9 * * 1' },
  { label: 'Every weekday',   value: '0 9 * * 1-5' },
  { label: 'Every Sunday',    value: '0 0 * * 0' },
  { label: 'Custom…',         value: '' },
];

// ── Helpers ───────────────────────────────────────────────────────────────────

function humanizeSchedule(schedule: string): string {
  const preset = SCHEDULE_PRESETS.find(p => p.value === schedule && p.value !== '');
  if (preset) return preset.label;
  // Simple partial descriptions
  const parts = schedule.split(' ');
  if (parts.length !== 5) return schedule;
  const [min, hr] = parts;
  if (min === '*' && hr === '*') return 'Every minute';
  if (min.startsWith('*/') && hr === '*') return `Every ${min.slice(2)} min`;
  if (min === '0' && hr.startsWith('*/')) return `Every ${hr.slice(2)} hour(s)`;
  if (min === '0' && hr !== '*') return `Daily at ${hr}:00`;
  return schedule;
}

function formatRelTime(ts: string): string {
  if (!ts) return '—';
  const date = new Date(ts);
  if (isNaN(date.getTime())) return '—';
  const diff = Date.now() - date.getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

// ── Status badge ──────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: CronJob['status'] }) {
  if (status === 'active') {
    return (
      <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400">
        <CheckCircle size={10} />
        Active
      </span>
    );
  }
  if (status === 'paused') {
    return (
      <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-400">
        <AlertCircle size={10} />
        Paused
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-red-500/15 text-red-400">
      <XCircle size={10} />
      Error
    </span>
  );
}

// ── Create job form ───────────────────────────────────────────────────────────

interface CreateFormProps {
  onCreated: () => void;
}

function CreateJobForm({ onCreated }: CreateFormProps) {
  const [name, setName] = useState('');
  const [schedule, setSchedule] = useState('0 * * * *');
  const [customSchedule, setCustomSchedule] = useState('');
  const [presetOpen, setPresetOpen] = useState(false);
  const [task, setTask] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [showCustom, setShowCustom] = useState(false);

  const activeSchedule = showCustom ? customSchedule : schedule;

  const selectPreset = (value: string) => {
    if (value === '') {
      setShowCustom(true);
    } else {
      setSchedule(value);
      setShowCustom(false);
    }
    setPresetOpen(false);
  };

  const selectedPresetLabel = showCustom
    ? 'Custom…'
    : (SCHEDULE_PRESETS.find(p => p.value === schedule)?.label ?? schedule);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) { toast.error('Job name is required'); return; }
    if (!task.trim()) { toast.error('Task description is required'); return; }
    if (!activeSchedule.trim()) { toast.error('Schedule is required'); return; }

    setSubmitting(true);
    try {
      await createCronJob({ name: name.trim(), schedule: activeSchedule.trim(), task: task.trim() });
      toast.success(`Cron job "${name.trim()}" created`);
      setName('');
      setSchedule('0 * * * *');
      setCustomSchedule('');
      setTask('');
      setShowCustom(false);
      onCreated();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create cron job');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Name */}
        <div>
          <label className="block text-xs text-zinc-500 font-medium mb-1.5 uppercase tracking-wide">
            Job Name
          </label>
          <input
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="e.g. daily-backup"
            className="w-full px-3 py-2.5 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 transition-colors"
          />
        </div>

        {/* Schedule */}
        <div>
          <label className="block text-xs text-zinc-500 font-medium mb-1.5 uppercase tracking-wide">
            Schedule
          </label>
          <div className="flex gap-2">
            {/* Preset dropdown */}
            <div className="relative flex-shrink-0">
              <button
                type="button"
                onClick={() => setPresetOpen(o => !o)}
                className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-300 hover:border-zinc-600 transition-colors min-w-40"
              >
                <span className="flex-1 text-left truncate">{selectedPresetLabel}</span>
                <ChevronDown size={13} className={`flex-shrink-0 transition-transform ${presetOpen ? 'rotate-180' : ''}`} />
              </button>

              {presetOpen && (
                <div className="absolute top-full left-0 mt-1 w-52 bg-zinc-900 border border-zinc-700 rounded-lg shadow-xl z-50 overflow-hidden">
                  {SCHEDULE_PRESETS.map(p => (
                    <button
                      key={p.label}
                      type="button"
                      onClick={() => selectPreset(p.value)}
                      className={`w-full text-left px-3 py-2 text-sm hover:bg-zinc-800 transition-colors ${
                        !showCustom && schedule === p.value && p.value !== ''
                          ? 'text-indigo-400 bg-indigo-500/10'
                          : 'text-zinc-300'
                      }`}
                    >
                      <div>{p.label}</div>
                      {p.value && (
                        <div className="text-xs text-zinc-600 font-mono">{p.value}</div>
                      )}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Custom cron expression */}
            {showCustom ? (
              <input
                value={customSchedule}
                onChange={e => setCustomSchedule(e.target.value)}
                placeholder="* * * * *"
                className="flex-1 px-3 py-2.5 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 font-mono focus:outline-none focus:border-indigo-500/60 transition-colors"
              />
            ) : (
              <div className="flex-1 px-3 py-2.5 rounded-lg bg-zinc-800/30 border border-zinc-700/30 text-sm text-zinc-500 font-mono">
                {schedule}
              </div>
            )}
          </div>
          <p className="mt-1 text-xs text-zinc-600">
            {activeSchedule && humanizeSchedule(activeSchedule)}
            {!activeSchedule && 'minute hour day month weekday'}
          </p>
        </div>
      </div>

      {/* Task */}
      <div>
        <label className="block text-xs text-zinc-500 font-medium mb-1.5 uppercase tracking-wide">
          Task Description
        </label>
        <textarea
          value={task}
          onChange={e => setTask(e.target.value)}
          placeholder="Describe what this job should do, e.g. 'Summarize today's audit logs and save to summary.txt'"
          rows={3}
          className="w-full px-3 py-2.5 rounded-lg bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 transition-colors resize-none"
        />
      </div>

      <button
        type="submit"
        disabled={submitting}
        className="flex items-center gap-2 px-4 py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium transition-colors"
      >
        <Plus size={15} />
        {submitting ? 'Creating…' : 'Create Job'}
      </button>
    </form>
  );
}

// ── Main view ─────────────────────────────────────────────────────────────────

export default function CronView() {
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionIds, setActionIds] = useState<Set<string>>(new Set());

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await fetchCronJobs();
      setJobs(data);
    } catch {
      toast.error('Failed to load cron jobs');
      setJobs([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const withAction = async (id: string, fn: () => Promise<void>) => {
    setActionIds(s => new Set(s).add(id));
    try {
      await fn();
      await load();
    } finally {
      setActionIds(s => { const n = new Set(s); n.delete(id); return n; });
    }
  };

  const handleToggle = (job: CronJob) =>
    withAction(job.id, async () => {
      await toggleCronJob(job.id);
      toast.success(job.status === 'active' ? `Paused "${job.name}"` : `Resumed "${job.name}"`);
    });

  const handleDelete = (job: CronJob) =>
    withAction(job.id, async () => {
      await deleteCronJob(job.id);
      toast.success(`Deleted "${job.name}"`);
    });

  const handleRunNow = (job: CronJob) =>
    withAction(job.id, async () => {
      await runCronJobNow(job.id);
      toast.success(`Triggered "${job.name}"`);
    });

  const activeCount = jobs.filter(j => j.status === 'active').length;
  const pausedCount = jobs.filter(j => j.status === 'paused').length;

  return (
    <div
      className="flex-1 overflow-y-auto p-6 bg-zinc-950"
      style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Cron Job Manager</h2>
          <p className="text-sm text-zinc-500">
            {jobs.length} job{jobs.length !== 1 ? 's' : ''} —{' '}
            <span className="text-emerald-400">{activeCount} active</span>
            {pausedCount > 0 && <span className="text-amber-400">, {pausedCount} paused</span>}
          </p>
        </div>
        <button
          onClick={load}
          className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
        >
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {/* Create form */}
      <div className="rounded-xl bg-zinc-800/40 border border-zinc-700/40 overflow-hidden mb-6">
        <div className="px-5 py-3.5 border-b border-zinc-700/40 flex items-center gap-2">
          <Plus size={15} className="text-indigo-400" />
          <h3 className="text-sm font-semibold text-zinc-300">Create New Job</h3>
        </div>
        <div className="p-5">
          <CreateJobForm onCreated={load} />
        </div>
      </div>

      {/* Job list */}
      <div className="rounded-xl bg-zinc-800/40 border border-zinc-700/40 overflow-hidden">
        <div className="px-5 py-3.5 border-b border-zinc-700/40 flex items-center gap-2">
          <Clock size={15} className="text-zinc-400" />
          <h3 className="text-sm font-semibold text-zinc-300">Scheduled Jobs</h3>
        </div>

        {loading ? (
          <div className="flex items-center gap-2 text-zinc-500 p-6">
            <RefreshCw size={16} className="animate-spin" /> Loading jobs...
          </div>
        ) : jobs.length === 0 ? (
          <div className="text-center text-zinc-500 text-sm py-16 px-6">
            <Clock size={32} className="mx-auto mb-3 opacity-30" />
            <p className="font-medium text-zinc-400 mb-1">No cron jobs yet</p>
            <p>Create your first scheduled job using the form above.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-zinc-700/50 bg-zinc-800/60">
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Name</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Schedule</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Task</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide w-20">Status</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Last Run</th>
                  <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Next Run</th>
                  <th className="text-right px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Actions</th>
                </tr>
              </thead>
              <tbody>
                {jobs.map(job => {
                  const busy = actionIds.has(job.id);
                  return (
                    <tr key={job.id} className="border-b border-zinc-800/40 hover:bg-zinc-800/20 transition-colors">
                      <td className="px-4 py-3">
                        <span className="font-medium text-zinc-200 text-sm">{job.name}</span>
                      </td>
                      <td className="px-4 py-3">
                        <div className="font-mono text-xs text-indigo-400 bg-indigo-500/10 px-2 py-0.5 rounded inline-block">
                          {job.schedule}
                        </div>
                        <div className="text-xs text-zinc-600 mt-0.5">{humanizeSchedule(job.schedule)}</div>
                      </td>
                      <td className="px-4 py-3 max-w-xs">
                        <span className="text-xs text-zinc-400 line-clamp-2" title={job.task}>
                          {job.task}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <StatusBadge status={job.status} />
                      </td>
                      <td className="px-4 py-3 text-xs text-zinc-500">
                        {formatRelTime(job.last_run || '')}
                      </td>
                      <td className="px-4 py-3 text-xs text-zinc-500">
                        {job.next_run ? formatRelTime(job.next_run) : '—'}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-1.5">
                          {/* Run now */}
                          <button
                            onClick={() => handleRunNow(job)}
                            disabled={busy}
                            title="Run now"
                            className="p-1.5 rounded-lg text-zinc-500 hover:text-zinc-200 hover:bg-zinc-700 disabled:opacity-40 transition-all"
                          >
                            <Zap size={13} />
                          </button>

                          {/* Pause / Resume */}
                          <button
                            onClick={() => handleToggle(job)}
                            disabled={busy}
                            title={job.status === 'active' ? 'Pause' : 'Resume'}
                            className={`p-1.5 rounded-lg disabled:opacity-40 transition-all ${
                              job.status === 'active'
                                ? 'text-amber-400 hover:text-amber-300 hover:bg-amber-500/10'
                                : 'text-emerald-400 hover:text-emerald-300 hover:bg-emerald-500/10'
                            }`}
                          >
                            {job.status === 'active' ? <Pause size={13} /> : <Play size={13} />}
                          </button>

                          {/* Delete */}
                          <button
                            onClick={() => handleDelete(job)}
                            disabled={busy}
                            title="Delete"
                            className="p-1.5 rounded-lg text-zinc-600 hover:text-red-400 hover:bg-red-500/10 disabled:opacity-40 transition-all"
                          >
                            <Trash2 size={13} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
