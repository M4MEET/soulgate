import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  MessageSquare, LayoutDashboard, Settings, Bot, Wrench,
  Brain, BookOpen, Shield, Plug, ChevronLeft, ChevronRight,
  Hexagon, Webhook, DollarSign, Clock, Palette, Sun, Moon,
  FolderOpen, Terminal, Users, Building2, Activity,
} from 'lucide-react';
import NotificationCenter, { type Notification } from './NotificationCenter';

export type ViewId =
  | 'chat'
  | 'dashboard'
  | 'activity'
  | 'agents'
  | 'tools'
  | 'memory'
  | 'sessions'
  | 'audit'
  | 'connectors'
  | 'webhooks'
  | 'costs'
  | 'cron'
  | 'canvas'
  | 'files'
  | 'terminal'
  | 'settings'
  | 'users'
  | 'teams'
  | 'policies';

const NAV_ITEMS: { id: ViewId; icon: React.ElementType; label: string }[] = [
  { id: 'chat',       icon: MessageSquare,   label: 'Chat' },
  { id: 'dashboard',  icon: LayoutDashboard, label: 'Dashboard' },
  { id: 'activity',   icon: Activity,        label: 'Activity' },
  { id: 'agents',     icon: Bot,             label: 'Agents' },
  { id: 'tools',      icon: Wrench,          label: 'Tools' },
  { id: 'memory',     icon: Brain,           label: 'Memory' },
  { id: 'sessions',   icon: BookOpen,        label: 'Sessions' },
  { id: 'audit',      icon: Shield,          label: 'Audit' },
  { id: 'connectors', icon: Plug,            label: 'Connectors' },
  { id: 'webhooks',   icon: Webhook,         label: 'Webhooks' },
  { id: 'costs',      icon: DollarSign,      label: 'Costs' },
  { id: 'cron',       icon: Clock,           label: 'Cron' },
  { id: 'canvas',     icon: Palette,         label: 'Canvas' },
  { id: 'files',      icon: FolderOpen,      label: 'Files' },
  { id: 'terminal',   icon: Terminal,        label: 'Terminal' },
  { id: 'settings',   icon: Settings,        label: 'Settings' },
];

const ADMIN_ITEMS: { id: ViewId; icon: React.ElementType; label: string }[] = [
  { id: 'users',    icon: Users,     label: 'Users' },
  { id: 'teams',    icon: Building2, label: 'Teams' },
  { id: 'policies', icon: Shield,    label: 'Policies' },
];

interface Props {
  view: ViewId;
  onViewChange: (v: ViewId) => void;
  connected: boolean;
  onCommandPalette: () => void;
  onShowShortcuts: () => void;
  theme: 'dark' | 'light';
  onToggleTheme: () => void;
  notifications: Notification[];
  unreadCount: number;
  onMarkAllRead: () => void;
  onDismissNotification: (id: string) => void;
}

export default function Sidebar({
  view,
  onViewChange,
  connected,
  onCommandPalette,
  onShowShortcuts,
  theme,
  onToggleTheme,
  notifications,
  unreadCount,
  onMarkAllRead,
  onDismissNotification,
}: Props) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <motion.aside
      animate={{ width: collapsed ? 60 : 240 }}
      transition={{ duration: 0.2, ease: 'easeInOut' }}
      className="flex flex-col h-full bg-zinc-900 border-r border-zinc-800 flex-shrink-0 overflow-hidden z-20"
    >
      {/* Logo */}
      <div className="flex items-center gap-3 px-4 h-14 border-b border-zinc-800 flex-shrink-0">
        <div className="flex-shrink-0 text-indigo-400">
          <Hexagon size={24} strokeWidth={1.5} />
        </div>
        <AnimatePresence>
          {!collapsed && (
            <motion.div
              initial={{ opacity: 0, x: -8 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -8 }}
              transition={{ duration: 0.15 }}
              className="flex items-center gap-2 min-w-0"
            >
              <span className="font-semibold text-sm tracking-tight text-zinc-100 whitespace-nowrap">
                SoulGate
              </span>
              <span
                className={`text-xs px-1.5 py-0.5 rounded-full font-medium flex-shrink-0 ${
                  connected
                    ? 'bg-emerald-500/15 text-emerald-400'
                    : 'bg-red-500/15 text-red-400'
                }`}
              >
                {connected ? 'live' : 'off'}
              </span>
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* Nav items */}
      <nav className="flex-1 px-2 py-3 flex flex-col gap-0.5 overflow-y-auto" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
        {NAV_ITEMS.map(({ id, icon: Icon, label }) => {
          const active = view === id;
          return (
            <button
              key={id}
              onClick={() => onViewChange(id)}
              title={collapsed ? label : undefined}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-150 w-full text-left group ${
                active
                  ? 'bg-indigo-500/15 text-indigo-400'
                  : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800'
              }`}
            >
              <Icon
                size={17}
                className={`flex-shrink-0 ${active ? 'text-indigo-400' : 'text-zinc-500 group-hover:text-zinc-300'}`}
              />
              <AnimatePresence>
                {!collapsed && (
                  <motion.span
                    initial={{ opacity: 0, x: -6 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: -6 }}
                    transition={{ duration: 0.12 }}
                    className="whitespace-nowrap"
                  >
                    {label}
                  </motion.span>
                )}
              </AnimatePresence>
            </button>
          );
        })}

        {/* Admin section separator */}
        <div className="mt-2 mb-1">
          {!collapsed ? (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="flex items-center gap-2 px-3 py-1"
            >
              <div className="flex-1 h-px bg-zinc-800" />
              <span className="text-xs text-zinc-600 font-medium uppercase tracking-wider whitespace-nowrap">Admin</span>
              <div className="flex-1 h-px bg-zinc-800" />
            </motion.div>
          ) : (
            <div className="h-px bg-zinc-800 mx-2" />
          )}
        </div>

        {ADMIN_ITEMS.map(({ id, icon: Icon, label }) => {
          const active = view === id;
          return (
            <button
              key={id}
              onClick={() => onViewChange(id)}
              title={collapsed ? label : undefined}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-150 w-full text-left group ${
                active
                  ? 'bg-indigo-500/15 text-indigo-400'
                  : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800'
              }`}
            >
              <Icon
                size={17}
                className={`flex-shrink-0 ${active ? 'text-indigo-400' : 'text-zinc-500 group-hover:text-zinc-300'}`}
              />
              <AnimatePresence>
                {!collapsed && (
                  <motion.span
                    initial={{ opacity: 0, x: -6 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: -6 }}
                    transition={{ duration: 0.12 }}
                    className="whitespace-nowrap"
                  >
                    {label}
                  </motion.span>
                )}
              </AnimatePresence>
            </button>
          );
        })}
      </nav>

      {/* Bottom actions */}
      <div className="px-2 pb-3 flex flex-col gap-1 border-t border-zinc-800 pt-3">
        {/* Notification center */}
        <NotificationCenter
          notifications={notifications}
          unreadCount={unreadCount}
          onMarkAllRead={onMarkAllRead}
          onDismiss={onDismissNotification}
          connected={connected}
          collapsed={collapsed}
        />

        {/* Theme toggle */}
        <button
          onClick={onToggleTheme}
          title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
          className={`flex items-center rounded-lg transition-all text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 ${
            collapsed ? 'justify-center w-9 h-9 mx-auto' : 'gap-2 px-3 py-2 w-full'
          }`}
        >
          {theme === 'dark'
            ? <Sun size={16} className="flex-shrink-0" />
            : <Moon size={16} className="flex-shrink-0" />
          }
          <AnimatePresence>
            {!collapsed && (
              <motion.span
                initial={{ opacity: 0, x: -6 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -6 }}
                transition={{ duration: 0.12 }}
                className="text-xs whitespace-nowrap"
              >
                {theme === 'dark' ? 'Light mode' : 'Dark mode'}
              </motion.span>
            )}
          </AnimatePresence>
        </button>

        {/* Command palette (expanded only) */}
        {!collapsed && (
          <button
            onClick={onCommandPalette}
            className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-all w-full"
          >
            <span className="flex-1 text-left">Command palette</span>
            <kbd className="font-mono text-xs bg-zinc-800 px-1.5 py-0.5 rounded">⌘K</kbd>
          </button>
        )}

        {/* Keyboard shortcuts (expanded only) */}
        {!collapsed && (
          <button
            onClick={onShowShortcuts}
            className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-all w-full"
          >
            <span className="flex-1 text-left">Keyboard shortcuts</span>
            <kbd className="font-mono text-xs bg-zinc-800 px-1.5 py-0.5 rounded">?</kbd>
          </button>
        )}

        {/* Collapse toggle */}
        <button
          onClick={() => setCollapsed(c => !c)}
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          className="flex items-center justify-center w-full h-8 rounded-lg text-zinc-600 hover:text-zinc-300 hover:bg-zinc-800 transition-all"
        >
          {collapsed ? <ChevronRight size={16} /> : <ChevronLeft size={16} />}
        </button>
      </div>
    </motion.aside>
  );
}
