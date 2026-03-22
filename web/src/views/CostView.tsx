import { useEffect, useState, useMemo } from 'react';
import {
  LineChart, Line, PieChart, Pie, Cell,
  XAxis, YAxis, Tooltip, ResponsiveContainer, Legend,
} from 'recharts';
import {
  DollarSign, TrendingUp, Calendar, AlertTriangle,
  RefreshCw, ChevronUp, ChevronDown, ChevronsUpDown,
} from 'lucide-react';
import { fetchCostSummary, type CostSummary } from '../lib/api';
import StatCard from '../components/StatCard';
import toast from 'react-hot-toast';

// ── Types ─────────────────────────────────────────────────────────────────────

interface DayRow {
  date: string;
  cost: number;
}

interface ProviderCost {
  name: string;
  value: number;
}

// ── Constants ─────────────────────────────────────────────────────────────────

const PIE_COLORS = ['#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#38bdf8', '#a78bfa'];

type SortDir = 'asc' | 'desc' | null;
type SortKey = 'date' | 'cost';

// ── Sub-components ────────────────────────────────────────────────────────────

function Panel({ title, children, className = '' }: { title: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={`rounded-xl bg-zinc-800/40 border border-zinc-700/40 overflow-hidden ${className}`}>
      <div className="px-5 py-3.5 border-b border-zinc-700/40">
        <h3 className="text-sm font-semibold text-zinc-300">{title}</h3>
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

const TOOLTIP_STYLE = {
  contentStyle: { background: '#18181b', border: '1px solid #3f3f46', borderRadius: 8, fontSize: 12 },
  labelStyle: { color: '#a1a1aa' },
};

function SortIcon({ dir }: { dir: SortDir }) {
  if (dir === 'asc') return <ChevronUp size={12} />;
  if (dir === 'desc') return <ChevronDown size={12} />;
  return <ChevronsUpDown size={12} className="opacity-30" />;
}

function EmptyChart({ height = 180 }: { height?: number }) {
  return (
    <div
      className="flex items-center justify-center text-zinc-600 text-sm"
      style={{ height }}
    >
      No cost data yet
    </div>
  );
}

// ── Main view ─────────────────────────────────────────────────────────────────

export default function CostView() {
  const [summary, setSummary] = useState<CostSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [dailyBudget, setDailyBudget] = useState<number>(5.0);
  const [budgetInput, setBudgetInput] = useState('5.00');
  const [sortKey, setSortKey] = useState<SortKey>('date');
  const [sortDir, setSortDir] = useState<SortDir>('desc');

  const load = async () => {
    setLoading(true);
    try {
      const data = await fetchCostSummary();
      setSummary(data);
    } catch {
      toast.error('Failed to load cost data');
      setSummary(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  // ── Derived stats from real API data ───────────────────────────────────────

  const days = useMemo<DayRow[]>(() => {
    if (!summary?.last_7_days) return [];
    return summary.last_7_days.map(d => ({ date: d.date, cost: d.cost_usd }));
  }, [summary]);

  const totalSpend = summary?.total_cost_usd ?? 0;
  const todaySpend = summary?.today_cost_usd ?? 0;
  const avgPerDay = days.length > 0 ? days.reduce((s, d) => s + d.cost, 0) / days.length : 0;
  const projectedMonthly = avgPerDay * 30;

  const providerBreakdown = useMemo<ProviderCost[]>(() => {
    if (!summary?.by_provider) return [];
    return Object.entries(summary.by_provider).map(([name, value]) => ({ name, value }));
  }, [summary]);

  const budgetExceeded = todaySpend > dailyBudget;
  const hasData = days.length > 0 || totalSpend > 0;

  // ── Sorted table ───────────────────────────────────────────────────────────

  const sortedDays = useMemo(() => {
    const copy = [...days];
    if (!sortDir || !sortKey) return copy.reverse();
    return copy.sort((a, b) => {
      const av = a[sortKey];
      const bv = b[sortKey];
      if (av < bv) return sortDir === 'asc' ? -1 : 1;
      if (av > bv) return sortDir === 'asc' ? 1 : -1;
      return 0;
    });
  }, [days, sortKey, sortDir]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir(d => (d === 'asc' ? 'desc' : d === 'desc' ? null : 'asc'));
    } else {
      setSortKey(key);
      setSortDir('desc');
    }
  };

  const handleBudgetSave = () => {
    const v = parseFloat(budgetInput);
    if (isNaN(v) || v <= 0) { toast.error('Enter a valid budget amount'); return; }
    setDailyBudget(v);
    toast.success(`Daily budget set to $${v.toFixed(2)}`);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-500 gap-2">
        <RefreshCw size={18} className="animate-spin" />
        Loading cost data...
      </div>
    );
  }

  if (!hasData) {
    return (
      <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-lg font-bold text-zinc-100">Cost Analytics</h2>
            <p className="text-sm text-zinc-500">No cost data yet</p>
          </div>
          <button
            onClick={load}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
          >
            <RefreshCw size={14} />
            Refresh
          </button>
        </div>
        <div className="flex flex-col items-center justify-center py-24 text-zinc-600">
          <DollarSign size={40} className="mb-3 opacity-30" />
          <p className="text-sm">No cost data yet. Run some agents to start tracking costs.</p>
        </div>
      </div>
    );
  }

  return (
    <div
      className="flex-1 overflow-y-auto p-6 bg-zinc-950"
      style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Cost Analytics</h2>
          <p className="text-sm text-zinc-500">
            {summary?.total_calls ?? 0} total calls
            {summary?.session_calls ? ` · ${summary.session_calls} this session` : ''}
          </p>
        </div>
        <button
          onClick={load}
          className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-all"
        >
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>

      {/* Budget alert */}
      {budgetExceeded && (
        <div className="flex items-center gap-3 mb-5 px-4 py-3 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-400 text-sm">
          <AlertTriangle size={16} className="flex-shrink-0" />
          <span>
            Today's spend <strong>${todaySpend.toFixed(4)}</strong> exceeds your daily budget of{' '}
            <strong>${dailyBudget.toFixed(2)}</strong>.
          </span>
        </div>
      )}

      {/* Summary cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
        <StatCard icon={DollarSign}   label="Total Spend"        value={`$${totalSpend.toFixed(4)}`}       color="text-indigo-400" />
        <StatCard icon={DollarSign}   label="Today's Spend"      value={`$${todaySpend.toFixed(4)}`}       color={budgetExceeded ? 'text-amber-400' : 'text-emerald-400'} />
        <StatCard icon={TrendingUp}   label="Average / Day"      value={`$${avgPerDay.toFixed(4)}`}        color="text-sky-400" />
        <StatCard icon={Calendar}     label="Projected Monthly"  value={`$${projectedMonthly.toFixed(2)}`} color="text-violet-400" />
      </div>

      {/* Budget setting */}
      <Panel title="Daily Budget Alert" className="mb-5">
        <div className="flex items-center gap-3">
          <span className="text-sm text-zinc-400">Notify when daily spend exceeds:</span>
          <div className="flex items-center gap-0">
            <span className="px-2 py-1.5 bg-zinc-800 border border-zinc-700 border-r-0 rounded-l-lg text-sm text-zinc-500">$</span>
            <input
              type="number"
              min="0.01"
              step="0.01"
              value={budgetInput}
              onChange={e => setBudgetInput(e.target.value)}
              className="w-24 px-2 py-1.5 bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 focus:outline-none focus:border-indigo-500/60"
            />
            <button
              onClick={handleBudgetSave}
              className="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-r-lg transition-colors"
            >
              Save
            </button>
          </div>
          <span className={`text-xs px-2 py-1 rounded-full font-medium ${
            budgetExceeded
              ? 'bg-amber-500/15 text-amber-400'
              : 'bg-emerald-500/15 text-emerald-400'
          }`}>
            {budgetExceeded ? 'Budget exceeded' : 'Within budget'}
          </span>
        </div>
      </Panel>

      {/* Cost over time + provider breakdown */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
        <Panel title="Cost over time (last 7 days)" className="md:col-span-2">
          {days.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <LineChart data={days}>
                <XAxis
                  dataKey="date"
                  tick={{ fill: '#71717a', fontSize: 11 }}
                  axisLine={false}
                  tickLine={false}
                />
                <YAxis
                  tick={{ fill: '#71717a', fontSize: 11 }}
                  axisLine={false}
                  tickLine={false}
                  tickFormatter={v => `$${(v as number).toFixed(2)}`}
                />
                <Tooltip
                  {...TOOLTIP_STYLE}
                  itemStyle={{ color: '#818cf8' }}
                  formatter={(v: unknown) => [`$${(v as number).toFixed(4)}`, 'Cost']}
                />
                <Line type="monotone" dataKey="cost" stroke="#6366f1" strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
              </LineChart>
            </ResponsiveContainer>
          ) : (
            <EmptyChart height={180} />
          )}
        </Panel>

        <Panel title="Cost by provider">
          {providerBreakdown.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <PieChart>
                <Pie
                  data={providerBreakdown}
                  cx="50%"
                  cy="50%"
                  innerRadius={45}
                  outerRadius={68}
                  dataKey="value"
                  paddingAngle={3}
                >
                  {providerBreakdown.map((_, i) => (
                    <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip
                  {...TOOLTIP_STYLE}
                  formatter={(v: unknown) => [`$${(v as number).toFixed(4)}`, 'Cost']}
                />
                <Legend iconType="circle" iconSize={8} wrapperStyle={{ fontSize: 11, color: '#a1a1aa' }} />
              </PieChart>
            </ResponsiveContainer>
          ) : (
            <EmptyChart height={180} />
          )}
        </Panel>
      </div>

      {/* Detailed cost table */}
      <Panel title="Daily cost breakdown (last 7 days)">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-700/50">
                {(
                  [
                    { key: 'date', label: 'Date' },
                    { key: 'cost', label: 'Cost' },
                  ] as { key: SortKey; label: string }[]
                ).map(col => (
                  <th
                    key={col.key}
                    onClick={() => handleSort(col.key)}
                    className="text-left px-3 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide cursor-pointer select-none hover:text-zinc-300 transition-colors"
                  >
                    <span className="flex items-center gap-1">
                      {col.label}
                      <SortIcon dir={sortKey === col.key ? sortDir : null} />
                    </span>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {sortedDays.length === 0 ? (
                <tr>
                  <td colSpan={2} className="text-center text-zinc-600 text-sm py-8">No cost data yet</td>
                </tr>
              ) : (
                sortedDays.map((row, i) => (
                  <tr key={i} className="border-b border-zinc-800/40 hover:bg-zinc-800/20 transition-colors">
                    <td className="px-3 py-2.5 text-xs text-zinc-400 font-mono">{row.date}</td>
                    <td className="px-3 py-2.5 text-xs font-semibold text-emerald-400">
                      ${row.cost.toFixed(4)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
            {sortedDays.length > 0 && (
              <tfoot>
                <tr className="border-t border-zinc-700/60 bg-zinc-800/30">
                  <td className="px-3 py-2.5 text-xs text-zinc-500 font-medium">Total (all time)</td>
                  <td className="px-3 py-2.5 text-xs font-bold text-emerald-400">
                    ${totalSpend.toFixed(4)}
                  </td>
                </tr>
              </tfoot>
            )}
          </table>
        </div>
      </Panel>
    </div>
  );
}
