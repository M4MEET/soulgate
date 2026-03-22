import { useEffect, useState } from 'react';
import {
  LineChart, Line, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, Tooltip, ResponsiveContainer, Legend,
} from 'recharts';
import { useReactTable, getCoreRowModel, flexRender, type ColumnDef } from '@tanstack/react-table';
import {
  Activity, Cpu, HardDrive, Users, Zap, CheckCircle,
  AlertTriangle, XCircle, DollarSign, Clock, GitBranch, Server,
} from 'lucide-react';
import type { HealthData, SessionData, CostData, CostSummary } from '../lib/api';
import { fetchCosts, fetchCostSummary } from '../lib/api';
import StatCard from '../components/StatCard';
import { formatRelativeTime } from '../lib/utils';

interface Props {
  health: HealthData | null;
  sessions: SessionData[];
}

const SESSION_COLS: ColumnDef<SessionData, unknown>[] = [
  { accessorKey: 'id', header: 'ID', cell: ({ getValue }) => (
    <span className="font-mono text-xs text-indigo-400 truncate max-w-24 block">{String(getValue()).slice(0, 12)}…</span>
  )},
  { accessorKey: 'channel', header: 'Channel', cell: ({ getValue }) => (
    <span className="text-xs text-zinc-300 px-2 py-0.5 rounded-full bg-zinc-800">{String(getValue())}</span>
  )},
  { accessorKey: 'message_count', header: 'Messages' },
  { accessorKey: 'last_activity', header: 'Last Active', cell: ({ getValue }) => (
    <span className="text-xs text-zinc-500">{getValue() ? formatRelativeTime(String(getValue())) : '—'}</span>
  )},
];

const PIE_COLORS = ['#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#38bdf8'];

function HealthCheck({ check }: { check: { name: string; status: string; detail?: string } }) {
  const isPass = check.status === 'pass';
  const isWarn = check.status === 'warn';

  return (
    <div className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm ${
      isPass ? '' : isWarn ? 'bg-amber-500/5 border border-amber-500/20' : 'bg-red-500/5 border border-red-500/20'
    }`}>
      {isPass ? (
        <CheckCircle size={14} className="text-emerald-400 flex-shrink-0" />
      ) : isWarn ? (
        <AlertTriangle size={14} className="text-amber-400 flex-shrink-0" />
      ) : (
        <XCircle size={14} className="text-red-400 flex-shrink-0" />
      )}
      <span className={`font-medium capitalize ${isPass ? 'text-zinc-300' : isWarn ? 'text-amber-300' : 'text-red-300'}`}>
        {check.name.replace(/_/g, ' ')}
      </span>
      {check.detail && (
        <span className="text-xs text-zinc-500 ml-auto">{check.detail}</span>
      )}
    </div>
  );
}

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

export default function DashboardView({ health, sessions }: Props) {
  const [costs, setCosts] = useState<CostData[]>([]);
  const [costSummary, setCostSummary] = useState<CostSummary | null>(null);

  useEffect(() => {
    fetchCosts().then(setCosts);
    fetchCostSummary().then(setCostSummary);
  }, []);

  const sessionTable = useReactTable({
    data: sessions,
    columns: SESSION_COLS,
    getCoreRowModel: getCoreRowModel(),
  });

  if (!health) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-500 gap-2">
        <Activity size={20} className="animate-spin" />
        Loading dashboard…
      </div>
    );
  }

  const totalClients = Object.values(health.clients).reduce((a, b) => a + b, 0);
  const memPct = Math.min((health.memory.alloc_mb / health.memory.sys_mb) * 100, 100);

  // Build provider pie from real cost data
  const providerPieData = costSummary?.by_provider
    ? Object.entries(costSummary.by_provider).map(([name, value]) => ({ name, value: Number(value.toFixed(4)) }))
    : [];

  const clientPieData = Object.entries(health.clients).map(([name, value]) => ({ name, value }));

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Stat grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
        <StatCard icon={Zap}        label="Provider"    value={health.provider || '—'}                 color="text-indigo-400" />
        <StatCard icon={Cpu}        label="Model"       value={health.model?.split('/').pop() || '—'}  color="text-sky-400" />
        <StatCard icon={Clock}      label="Uptime"      value={health.uptime || '—'}                   color="text-emerald-400" />
        <StatCard icon={Users}      label="Clients"     value={totalClients}                           color="text-amber-400" />
        <StatCard icon={GitBranch}  label="Sessions"    value={health.sessions}                        color="text-violet-400" />
        <StatCard icon={HardDrive}  label="Memory"      value={`${health.memory.alloc_mb} MB`}         color="text-rose-400" />
        <StatCard icon={Server}     label="Goroutines"  value={health.memory.goroutines}               color="text-cyan-400" />
        <StatCard icon={DollarSign} label="Cost Today"  value={costSummary ? `$${costSummary.today_cost_usd.toFixed(4)}` : '$0.00'} color="text-orange-400" />
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
        <Panel title="Cost over time (7d)" className="md:col-span-2">
          <ResponsiveContainer width="100%" height={160}>
            <LineChart data={costs}>
              <XAxis dataKey="date" tick={{ fill: '#71717a', fontSize: 11 }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fill: '#71717a', fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={v => `$${v.toFixed(2)}`} />
              <Tooltip
                contentStyle={{ background: '#18181b', border: '1px solid #3f3f46', borderRadius: 8, fontSize: 12 }}
                labelStyle={{ color: '#a1a1aa' }}
                itemStyle={{ color: '#818cf8' }}
                formatter={(v: unknown) => [`$${(v as number).toFixed(4)}`, 'Cost']}
              />
              <Line type="monotone" dataKey="cost" stroke="#6366f1" strokeWidth={2} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </Panel>

        <Panel title="Cost by provider">
          <ResponsiveContainer width="100%" height={160}>
            <PieChart>
              <Pie data={providerPieData} cx="50%" cy="50%" innerRadius={45} outerRadius={65} dataKey="value" paddingAngle={3}>
                {providerPieData.map((_, i) => (
                  <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />
                ))}
              </Pie>
              <Tooltip contentStyle={{ background: '#18181b', border: '1px solid #3f3f46', borderRadius: 8, fontSize: 12 }} />
              <Legend iconType="circle" iconSize={8} wrapperStyle={{ fontSize: 12, color: '#a1a1aa' }} />
            </PieChart>
          </ResponsiveContainer>
        </Panel>
      </div>

      {/* Second row */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
        <Panel title="Token volume (7d)" className="md:col-span-2">
          <ResponsiveContainer width="100%" height={140}>
            <BarChart data={costs}>
              <XAxis dataKey="date" tick={{ fill: '#71717a', fontSize: 11 }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fill: '#71717a', fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={v => `${(v / 1000).toFixed(0)}k`} />
              <Tooltip
                contentStyle={{ background: '#18181b', border: '1px solid #3f3f46', borderRadius: 8, fontSize: 12 }}
                itemStyle={{ color: '#22c55e' }}
              />
              <Bar dataKey="tokens" fill="#22c55e" radius={[3, 3, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Panel>

        <Panel title="Health checks">
          <div className="flex flex-col gap-1.5 max-h-40 overflow-y-auto" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
            {health.checks.length > 0 ? (
              health.checks.map((c, i) => <HealthCheck key={i} check={c} />)
            ) : (
              <div className="text-sm text-zinc-500">No checks available</div>
            )}
          </div>
        </Panel>
      </div>

      {/* Memory bar */}
      <Panel title="Memory usage" className="mb-4">
        <div className="space-y-3">
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-zinc-500">Allocated</span>
              <span className="text-xs text-zinc-400">{health.memory.alloc_mb} / {health.memory.sys_mb} MB</span>
            </div>
            <div className="h-2 bg-zinc-700 rounded-full overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-indigo-600 to-indigo-400 rounded-full transition-all duration-500"
                style={{ width: `${memPct}%` }}
              />
            </div>
          </div>
          <div className="flex gap-6 text-xs text-zinc-500">
            <span>GC runs: {health.memory.num_gc}</span>
            <span>Goroutines: {health.memory.goroutines}</span>
            <span>Uptime: {health.uptime}</span>
          </div>
        </div>
      </Panel>

      {/* Sessions table */}
      <Panel title={`Active sessions (${sessions.length})`}>
        {sessions.length === 0 ? (
          <p className="text-sm text-zinc-500">No active sessions</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                {sessionTable.getHeaderGroups().map(hg => (
                  <tr key={hg.id} className="border-b border-zinc-700/50">
                    {hg.headers.map(h => (
                      <th key={h.id} className="text-left px-3 py-2 text-xs text-zinc-500 font-medium uppercase tracking-wide">
                        {flexRender(h.column.columnDef.header, h.getContext())}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody>
                {sessionTable.getRowModel().rows.map(row => (
                  <tr key={row.id} className="border-b border-zinc-800/30 hover:bg-zinc-800/20 transition-colors">
                    {row.getVisibleCells().map(cell => (
                      <td key={cell.id} className="px-3 py-2.5">
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      {/* Client breakdown */}
      {clientPieData.length > 0 && (
        <Panel title="Connected clients" className="mt-4">
          <div className="flex flex-wrap gap-3">
            {clientPieData.map(({ name, value }) => (
              <div key={name} className="flex items-center gap-2 px-3 py-2 rounded-full bg-zinc-800 border border-zinc-700/50 text-sm">
                <span className="text-zinc-400">{name}</span>
                <span className="font-bold text-indigo-400">{value}</span>
              </div>
            ))}
          </div>
        </Panel>
      )}
    </div>
  );
}
