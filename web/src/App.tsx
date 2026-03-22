import { useState, useEffect, useCallback } from 'react';
import { Toaster } from 'react-hot-toast';
import toast from 'react-hot-toast';
import Sidebar, { type ViewId } from './components/Sidebar';
import CommandPalette from './components/CommandPalette';
import { useHealth } from './hooks/useHealth';

import ChatView from './views/ChatView';
import DashboardView from './views/DashboardView';
import AgentsView from './views/AgentsView';
import ToolsView from './views/ToolsView';
import MemoryView from './views/MemoryView';
import SessionsView from './views/SessionsView';
import AuditView from './views/AuditView';
import ConnectorsView from './views/ConnectorsView';
import SettingsView from './views/SettingsView';

export default function App() {
  const [view, setView] = useState<ViewId>('chat');
  const [paletteOpen, setPaletteOpen] = useState(false);
  const { health, sessions, connected, refresh } = useHealth();

  // Ctrl+K / Cmd+K global shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setPaletteOpen(p => !p);
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
    <div className="flex h-screen w-screen bg-zinc-950 text-zinc-100 overflow-hidden">
      {/* Sidebar */}
      <Sidebar
        view={view}
        onViewChange={setView}
        connected={connected}
        onCommandPalette={() => setPaletteOpen(true)}
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
          {view === 'chat'       && <ChatView />}
          {view === 'dashboard'  && <DashboardView health={health} sessions={sessions} />}
          {view === 'agents'     && <AgentsView />}
          {view === 'tools'      && <ToolsView />}
          {view === 'memory'     && <MemoryView />}
          {view === 'sessions'   && <SessionsView sessions={sessions} />}
          {view === 'audit'      && <AuditView />}
          {view === 'connectors' && <ConnectorsView />}
          {view === 'settings'   && <SettingsView health={health} />}
        </div>
      </div>

      {/* Command palette */}
      <CommandPalette
        open={paletteOpen}
        onClose={() => setPaletteOpen(false)}
        onNavigate={v => { setView(v); }}
        onAction={handleAction}
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
