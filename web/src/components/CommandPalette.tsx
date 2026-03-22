import { useEffect, useRef, useState } from 'react';
import {
  MessageSquare, LayoutDashboard, Bot, Wrench, Brain,
  BookOpen, Shield, Plug, Settings, Trash2, Download,
  RefreshCw, Terminal, Search, Webhook, FolderOpen,
} from 'lucide-react';
import type { ViewId } from './Sidebar';

interface Command {
  id: string;
  label: string;
  description?: string;
  icon: React.ElementType;
  shortcut?: string;
  action: 'navigate' | 'action';
  target?: ViewId;
}

const COMMANDS: Command[] = [
  { id: 'nav-chat',       label: 'Go to Chat',        icon: MessageSquare,   action: 'navigate', target: 'chat' },
  { id: 'nav-dashboard',  label: 'Go to Dashboard',   icon: LayoutDashboard, action: 'navigate', target: 'dashboard' },
  { id: 'nav-agents',     label: 'Go to Agents',      icon: Bot,             action: 'navigate', target: 'agents' },
  { id: 'nav-tools',      label: 'Go to Tools',       icon: Wrench,          action: 'navigate', target: 'tools' },
  { id: 'nav-memory',     label: 'Go to Memory',      icon: Brain,           action: 'navigate', target: 'memory' },
  { id: 'nav-sessions',   label: 'Go to Sessions',    icon: BookOpen,        action: 'navigate', target: 'sessions' },
  { id: 'nav-audit',      label: 'Go to Audit Log',   icon: Shield,          action: 'navigate', target: 'audit' },
  { id: 'nav-connectors', label: 'Go to Connectors',  icon: Plug,            action: 'navigate', target: 'connectors' },
  { id: 'nav-webhooks',   label: 'Go to Webhooks',    icon: Webhook,         action: 'navigate', target: 'webhooks' },
  { id: 'nav-files',      label: 'Go to File Browser', icon: FolderOpen,      action: 'navigate', target: 'files' },
  { id: 'nav-terminal',   label: 'Go to Terminal',     icon: Terminal,        action: 'navigate', target: 'terminal' },
  { id: 'nav-settings',   label: 'Go to Settings',    icon: Settings,        action: 'navigate', target: 'settings' },
  { id: 'new-chat',       label: 'New Conversation',  icon: MessageSquare,   action: 'action',   shortcut: '⌘N' },
  { id: 'clear-chat',     label: 'Clear Chat',        icon: Trash2,          action: 'action',   shortcut: '⌘L' },
  { id: 'export-chat',    label: 'Export Chat',       icon: Download,        action: 'action' },
  { id: 'refresh',        label: 'Refresh Data',      icon: RefreshCw,       action: 'action' },
  { id: 'diagnostics',    label: 'Run Diagnostics',   icon: Terminal,        action: 'navigate', target: 'dashboard' },
];

interface Props {
  open: boolean;
  onClose: () => void;
  onNavigate: (view: ViewId) => void;
  onAction: (id: string) => void;
}

export default function CommandPalette({ open, onClose, onNavigate, onAction }: Props) {
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setQuery('');
      setSelected(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  if (!open) return null;

  const filtered = COMMANDS.filter(c =>
    c.label.toLowerCase().includes(query.toLowerCase()) ||
    c.id.includes(query.toLowerCase())
  );

  const handleKey = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') { onClose(); return; }
    if (e.key === 'ArrowDown') { e.preventDefault(); setSelected(s => Math.min(s + 1, filtered.length - 1)); }
    if (e.key === 'ArrowUp') { e.preventDefault(); setSelected(s => Math.max(s - 1, 0)); }
    if (e.key === 'Enter' && filtered[selected]) {
      execute(filtered[selected]);
    }
  };

  const execute = (cmd: Command) => {
    if (cmd.action === 'navigate' && cmd.target) onNavigate(cmd.target);
    else onAction(cmd.id);
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex justify-center pt-28"
      onClick={onClose}
    >
      <div
        className="w-[480px] max-h-96 bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl overflow-hidden flex flex-col"
        onClick={e => e.stopPropagation()}
      >
        {/* Input */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-zinc-800">
          <Search size={16} className="text-zinc-500 flex-shrink-0" />
          <input
            ref={inputRef}
            value={query}
            onChange={e => { setQuery(e.target.value); setSelected(0); }}
            onKeyDown={handleKey}
            placeholder="Type a command or search…"
            className="flex-1 bg-transparent text-zinc-100 placeholder-zinc-600 text-sm outline-none"
          />
          <kbd className="text-xs text-zinc-600 font-mono bg-zinc-800 px-1.5 py-0.5 rounded">Esc</kbd>
        </div>

        {/* List */}
        <div className="overflow-y-auto p-2">
          {filtered.length === 0 ? (
            <div className="text-center text-zinc-500 text-sm py-8">No commands found</div>
          ) : (
            filtered.map((cmd, i) => (
              <button
                key={cmd.id}
                onClick={() => execute(cmd)}
                onMouseEnter={() => setSelected(i)}
                className={`flex items-center gap-3 w-full px-3 py-2 rounded-lg text-sm text-left transition-all ${
                  i === selected
                    ? 'bg-indigo-500/15 text-zinc-100'
                    : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60'
                }`}
              >
                <cmd.icon size={15} className={i === selected ? 'text-indigo-400' : 'text-zinc-600'} />
                <span className="flex-1">{cmd.label}</span>
                {cmd.shortcut && (
                  <kbd className="text-xs text-zinc-600 font-mono bg-zinc-800 px-1.5 py-0.5 rounded">
                    {cmd.shortcut}
                  </kbd>
                )}
                {cmd.action === 'navigate' && (
                  <span className="text-xs text-zinc-700">navigate</span>
                )}
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
