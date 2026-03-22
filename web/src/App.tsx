import { useState, useEffect, useCallback } from 'react';
import { Toaster } from 'react-hot-toast';
import toast from 'react-hot-toast';
import Sidebar, { type ViewId } from './components/Sidebar';
import CommandPalette from './components/CommandPalette';
import ShortcutsModal from './components/ShortcutsModal';
import { useHealth } from './hooks/useHealth';
import { useNotifications } from './components/NotificationCenter';

import ChatView from './views/ChatView';
import DashboardView from './views/DashboardView';
import AgentsView from './views/AgentsView';
import ToolsView from './views/ToolsView';
import MemoryView from './views/MemoryView';
import SessionsView from './views/SessionsView';
import AuditView from './views/AuditView';
import ConnectorsView from './views/ConnectorsView';
import SettingsView from './views/SettingsView';
import WebhookView from './views/WebhookView';
import CostView from './views/CostView';
import CronView from './views/CronView';
import CanvasView from './views/CanvasView';
import FileBrowserView from './views/FileBrowserView';
import TerminalView from './views/TerminalView';
import UsersView from './views/UsersView';
import TeamsView from './views/TeamsView';
import PoliciesView from './views/PoliciesView';
import ActivityView from './views/ActivityView';

// ── Theme persistence ─────────────────────────────────────────────────────────

type Theme = 'dark' | 'light';

function applyTheme(t: Theme) {
  const html = document.documentElement;
  if (t === 'light') {
    html.setAttribute('data-theme', 'light');
  } else {
    html.removeAttribute('data-theme');
  }
}

function loadTheme(): Theme {
  const stored = localStorage.getItem('sg-theme') as Theme | null;
  return stored === 'light' ? 'light' : 'dark';
}

// ── App ───────────────────────────────────────────────────────────────────────

export default function App() {
  const [view, setView] = useState<ViewId>('chat');
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [theme, setTheme] = useState<Theme>(loadTheme);
  const { health, sessions, connected, refresh } = useHealth();

  // Apply theme on mount and change
  useEffect(() => {
    applyTheme(theme);
    localStorage.setItem('sg-theme', theme);
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme(t => (t === 'dark' ? 'light' : 'dark'));
    toast.success(theme === 'dark' ? 'Light mode' : 'Dark mode');
  }, [theme]);

  // Notification center
  const { notifications, unreadCount, markAllRead, dismiss: dismissNotification } =
    useNotifications(health, sessions, connected);

  // Global keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const typing = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable;

      // Ctrl+K / Cmd+K — command palette (allowed everywhere)
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setPaletteOpen(p => !p);
        return;
      }

      // Skip remaining shortcuts when typing
      if (typing) return;

      // ? — shortcuts overlay
      if (e.key === '?' && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        setShortcutsOpen(s => !s);
        return;
      }

      // Ctrl+1 — Chat
      if ((e.ctrlKey || e.metaKey) && e.key === '1') { e.preventDefault(); setView('chat'); return; }
      // Ctrl+2 — Dashboard
      if ((e.ctrlKey || e.metaKey) && e.key === '2') { e.preventDefault(); setView('dashboard'); return; }
      // Ctrl+3 — Settings
      if ((e.ctrlKey || e.metaKey) && e.key === '3') { e.preventDefault(); setView('settings'); return; }
      // Ctrl+G — Agents
      if ((e.ctrlKey || e.metaKey) && e.key === 'g') { e.preventDefault(); setView('agents'); return; }
      // Ctrl+N — New conversation (navigate to chat)
      if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
        e.preventDefault();
        setView('chat');
        toast.success('New conversation');
        return;
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  const handleAction = useCallback((id: string) => {
    switch (id) {
      case 'new-chat':
        setView('chat');
        toast.success('New conversation');
        break;
      case 'clear-chat':
        toast.success('Chat cleared');
        break;
      case 'export-chat':
        toast.success('Chat exported');
        break;
      case 'refresh':
        refresh();
        toast.success('Data refreshed');
        break;
      default:
        break;
    }
  }, [refresh]);

  return (
    <div
      className="flex h-screen w-screen bg-zinc-950 text-zinc-100 overflow-hidden"
      data-theme={theme === 'light' ? 'light' : undefined}
    >
      {/* Sidebar */}
      <Sidebar
        view={view}
        onViewChange={setView}
        connected={connected}
        onCommandPalette={() => setPaletteOpen(true)}
        onShowShortcuts={() => setShortcutsOpen(true)}
        theme={theme}
        onToggleTheme={toggleTheme}
        notifications={notifications}
        unreadCount={unreadCount}
        onMarkAllRead={markAllRead}
        onDismissNotification={dismissNotification}
      />

      {/* Main content */}
      <div className="flex flex-col flex-1 overflow-hidden">
        {/* Offline banner */}
        {!connected && (
          <div className="bg-red-500/10 border-b border-red-500/20 text-red-400 text-center text-xs py-1.5 font-medium flex-shrink-0">
            Gateway disconnected — reconnecting…
          </div>
        )}

        {/* View area */}
        <div className="flex flex-1 overflow-hidden">
          {view === 'chat'       && <ChatView health={health} />}
          {view === 'dashboard'  && <DashboardView health={health} sessions={sessions} />}
          {view === 'agents'     && <AgentsView />}
          {view === 'tools'      && <ToolsView />}
          {view === 'memory'     && <MemoryView />}
          {view === 'sessions'   && <SessionsView sessions={sessions} />}
          {view === 'audit'      && <AuditView />}
          {view === 'connectors' && <ConnectorsView />}
          {view === 'activity'   && <ActivityView />}
          {view === 'webhooks'   && <WebhookView />}
          {view === 'costs'      && <CostView />}
          {view === 'cron'       && <CronView />}
          {view === 'canvas'     && <CanvasView />}
          {view === 'files'      && <FileBrowserView />}
          {view === 'terminal'   && <TerminalView />}
          {view === 'settings'   && <SettingsView health={health} onConfigSaved={refresh} />}
          {view === 'users'      && <UsersView />}
          {view === 'teams'      && <TeamsView />}
          {view === 'policies'   && <PoliciesView />}
        </div>
      </div>

      {/* Command palette */}
      <CommandPalette
        open={paletteOpen}
        onClose={() => setPaletteOpen(false)}
        onNavigate={v => { setView(v as ViewId); }}
        onAction={handleAction}
      />

      {/* Keyboard shortcuts overlay */}
      <ShortcutsModal
        open={shortcutsOpen}
        onClose={() => setShortcutsOpen(false)}
      />

      {/* Toast notifications */}
      <Toaster
        position="bottom-right"
        toastOptions={{
          style: {
            background: '#18181b',
            color: '#f4f4f5',
            border: '1px solid #3f3f46',
            borderRadius: '10px',
            fontSize: '13px',
          },
          success: {
            iconTheme: { primary: '#22c55e', secondary: '#18181b' },
          },
          error: {
            iconTheme: { primary: '#ef4444', secondary: '#18181b' },
          },
        }}
      />
    </div>
  );
}
