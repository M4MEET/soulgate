import { useEffect, useState, useMemo } from 'react';
import {
  LineChart, Line, BarChart, Bar, PieChart, Pie, Cell,
  AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, Legend,
} from 'recharts';
import {
  DollarSign, TrendingUp, Calendar, AlertTriangle,
  RefreshCw, ChevronUp, ChevronDown, ChevronsUpDown,
} from 'lucide-react';
import { fetchCosts, type CostData } from '../lib/api';
import StatCard from '../components/StatCard';
import toast from 'react-hot-toast';

// ── Extended types ────────────────────────────────────────────────────────────

interface ExtendedCostData extends CostData {
  provider?: string;
  model?: string;
  input_tokens?: number;
  output_tokens?: number;
}

interface ProviderCost {
  name: string;
  value: number;
}

interface ModelCost {
  model: string;
  cost: number;
}

// ── Constants ─────────────────────────────────────────────────────────────────

const PIE_COLORS = ['#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#38bdf8', '#a78bfa'];

const PROVIDERS = ['Anthropic', 'OpenAI', 'Groq', 'Cohere'];
const MODELS = [
  'claude-opus-4-5', 'claude-sonnet-4-5', 'gpt-4o',
  'gpt-4o-mini', 'gpt-3.5-turbo', 'llama-3.3-70b',
];

// ── Mock data generator ───────────────────────────────────────────────────────

function generateMockData(): ExtendedCostData[] {
  return Array.from({ length: 30 }, (_, i) => {
    const d = new Date();
    d.setDate(d.getDate() - (29 - i));
    const provider = PROVIDERS[Math.floor(Math.random() * PROVIDERS.length)];
    const model = MODELS[Math.floor(Math.random() * MODELS.length)];
    const inputTokens = Math.floor(Math.random() * 40000 + 5000);
    const outputTokens = Math.floor(Math.random() * 15000 + 2000);
    return {
      date: d.toLocaleDateString('en', { month: 'short', day: 'numeric' }),
      cost: Math.random() * 1.2 + 0.05,
      tokens: inputTokens + outputTokens,
      provider,
      model,
      input_tokens: inputTokens,
      output_tokens: outputTokens,
    };
  });
}

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

type SortDir = 'asc' | 'desc' | null;
type SortKey = 'date' | 'provider' | 'model' | 'tokens' | 'cost';

function SortIcon({ dir }: { dir: SortDir }) {
  if (dir === 'asc') return <ChevronUp size={12} />;
  if (dir === 'desc') return <ChevronDown size={12} />;
  return <ChevronsUpDown size={12} className="opacity-30" />;
}

// ── Main view ─────────────────────────────────────────────────────────────────

export default function CostView() {
  const [data, setData] = useState<ExtendedCostData[]>([]);
  const [loading, setLoading] = useState(true);
  const [dailyBudget, setDailyBudget] = useState<number>(5.0);
  const [budgetInput, setBudgetInput] = useState('5.00');
  const [sortKey, setSortKey] = useState<SortKey>('date');
  const [sortDir, setSortDir] = useState<SortDir>('desc');

  const load = async () => {
    setLoading(true);
    try {
      const raw = await fetchCosts();
      if (raw && raw.length > 0) {
        setData(raw as ExtendedCostData[]);
      } else {
        setData(generateMockData());
      }
    } catch {
      toast.error('Failed to load cost data');
      setData(generateMockData());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  // ── Derived stats ──────────────────────────────────────────────────────────

  const totalSpend = useMemo(() => data.reduce((s, d) => s + d.cost, 0), [data]);
  const todaySpend = useMemo(() => data[data.length - 1]?.cost ?? 0, [data]);
  const avgPerDay = useMemo(() => (data.length > 0 ? totalSpend / data.length : 0), [data, totalSpend]);
  const projectedMonthly = useMemo(() => avgPerDay * 30, [avgPerDay]);

  const providerBreakdown = useMemo<ProviderCost[]>(() => {
    const map: Record<string, number> = {};
    for (const d of data) {
      const p = d.provider || 'Unknown';
      map[p] = (map[p] || 0) + d.cost;
    }
    return Object.entries(map).map(([name, value]) => ({ name, value }));
  }, [data]);

  const modelBreakdown = useMemo<ModelCost[]>(() => {
    const map: Record<string, number> = {};
    for (const d of data) {
      const m = d.model || 'Unknown';
      map[m] = (map[m] || 0) + d.cost;
    }
    return Object.entries(map)
      .map(([model, cost]) => ({ model, cost }))
      .sort((a, b) => b.cost - a.cost)
      .slice(0, 8);
  }, [data]);

  const tokenData = useMemo(() => data.map(d => ({
    date: d.date,
    input: d.input_tokens ?? Math.floor(d.tokens * 0.68),
    output: d.output_tokens ?? Math.floor(d.tokens * 0.32),
  })), [data]);

  const budgetExceeded = todaySpend > dailyBudget;

  // ── Sorted table ───────────────────────────────────────────────────────────

  const sortedData = useMemo(() => {
    const copy = [...data];
    if (!sortDir || !sortKey) return copy.reverse();
    return copy.sort((a, b) => {
      let av: string | number = a[sortKey as keyof ExtendedCostData] as string | number ?? '';
      let bv: string | number = b[sortKey as keyof ExtendedCostData] as string | number ?? '';
      if (typeof av === 'string') av = av.toLowerCase();
      if (typeof bv === 'string') bv = bv.toLowerCase();
      if (av < bv) return sortDir === 'asc' ? -1 : 1;
      if (av > bv) return sortDir === 'asc' ? 1 : -1;
      return 0;
    });
  }, [data, sortKey, sortDir]);

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

  return (
    <div
      className="flex-1 overflow-y-auto p-6 bg-zinc-950"
      style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Cost Analytics</h2>
          <p className="text-sm text-zinc-500">Last {data.length} days of usage</p>
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

      {/* Cost over time */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
        <Panel title="Cost over time (30d)" className="md:col-span-2">
          <ResponsiveContainer width="100%" height={180}>
            <LineChart data={data}>
              <XAxis
                dataKey="date"
                tick={{ fill: '#71717a', fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                interval={Math.floor(data.length / 6)}
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
        </Panel>

        <Panel title="Cost by provider">
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
        </Panel>
      </div>

      {/* Cost by model + token area chart */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
        <Panel title="Cost by model (top 8)">
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={modelBreakdown} layout="vertical" margin={{ left: 8 }}>
              <XAxis
                type="number"
                tick={{ fill: '#71717a', fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                tickFormatter={v => `$${(v as number).toFixed(2)}`}
              />
              <YAxis
                type="category"
                dataKey="model"
                tick={{ fill: '#a1a1aa', fontSize: 10 }}
                axisLine={false}
                tickLine={false}
                width={120}
              />
              <Tooltip
                {...TOOLTIP_STYLE}
                itemStyle={{ color: '#f59e0b' }}
                formatter={(v: unknown) => [`$${(v as number).toFixed(4)}`, 'Cost']}
              />
              <Bar dataKey="cost" fill="#f59e0b" radius={[0, 3, 3, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Panel>

        <Panel title="Token usage over time (input vs output)">
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={tokenData}>
              <defs>
                <linearGradient id="colorInput" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#6366f1" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#6366f1" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="colorOutput" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="date"
                tick={{ fill: '#71717a', fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                interval={Math.floor(tokenData.length / 5)}
              />
              <YAxis
                tick={{ fill: '#71717a', fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                tickFormatter={v => `${((v as number) / 1000).toFixed(0)}k`}
              />
              <Tooltip
                {...TOOLTIP_STYLE}
                formatter={(v: unknown, name: unknown) => [
                  `${((v as number) / 1000).toFixed(1)}k`,
                  String(name) === 'input' ? 'Input tokens' : 'Output tokens',
                ]}
              />
              <Area type="monotone" dataKey="input" stroke="#6366f1" strokeWidth={1.5} fill="url(#colorInput)" />
              <Area type="monotone" dataKey="output" stroke="#22c55e" strokeWidth={1.5} fill="url(#colorOutput)" />
              <Legend iconType="circle" iconSize={8} wrapperStyle={{ fontSize: 11, color: '#a1a1aa' }} />
            </AreaChart>
          </ResponsiveContainer>
        </Panel>
      </div>

      {/* Detailed cost table */}
      <Panel title="Detailed cost breakdown">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-700/50">
                {(
                  [
                    { key: 'date',     label: 'Date' },
                    { key: 'provider', label: 'Provider' },
                    { key: 'model',    label: 'Model' },
                    { key: 'tokens',   label: 'Tokens' },
                    { key: 'cost',     label: 'Cost' },
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
              {sortedData.map((row, i) => (
                <tr key={i} className="border-b border-zinc-800/40 hover:bg-zinc-800/20 transition-colors">
                  <td className="px-3 py-2.5 text-xs text-zinc-400 font-mono">{row.date}</td>
                  <td className="px-3 py-2.5">
                    <span className="text-xs px-2 py-0.5 rounded-full bg-indigo-500/10 text-indigo-400">
                      {row.provider || 'Unknown'}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 text-xs text-zinc-300 font-mono">{row.model || '—'}</td>
                  <td className="px-3 py-2.5 text-xs text-zinc-400">
                    {row.tokens.toLocaleString()}
                  </td>
                  <td className="px-3 py-2.5 text-xs font-semibold text-emerald-400">
                    ${row.cost.toFixed(4)}
                  </td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr className="border-t border-zinc-700/60 bg-zinc-800/30">
                <td colSpan={3} className="px-3 py-2.5 text-xs text-zinc-500 font-medium">Total</td>
                <td className="px-3 py-2.5 text-xs text-zinc-300 font-medium">
                  {data.reduce((s, d) => s + d.tokens, 0).toLocaleString()}
                </td>
                <td className="px-3 py-2.5 text-xs font-bold text-emerald-400">
                  ${totalSpend.toFixed(4)}
                </td>
              </tr>
            </tfoot>
          </table>
        </div>
      </Panel>
    </div>
  );
}
