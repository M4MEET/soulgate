import type { LucideIcon } from 'lucide-react';

interface Props {
  icon: LucideIcon;
  label: string;
  value: string | number;
  subtext?: string;
  color?: string;
  trend?: 'up' | 'down' | 'neutral';
}

export default function StatCard({ icon: Icon, label, value, subtext, color = 'text-indigo-400' }: Props) {
  return (
    <div className="flex items-center gap-4 p-4 rounded-xl bg-zinc-800/50 border border-zinc-700/50 hover:border-zinc-600/50 transition-all">
      <div className={`flex-shrink-0 p-2 rounded-lg bg-zinc-800 ${color}`}>
        <Icon size={18} />
      </div>
      <div className="min-w-0">
        <div className="text-lg font-bold text-zinc-100 leading-tight truncate">{value}</div>
        <div className="text-xs text-zinc-500 uppercase tracking-wide">{label}</div>
        {subtext && <div className="text-xs text-zinc-600 mt-0.5">{subtext}</div>}
      </div>
    </div>
  );
}
