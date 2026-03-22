import { useState, useEffect, useCallback } from 'react';
import ChatView from './views/ChatView';
import DashboardView from './views/DashboardView';
import SettingsView from './views/SettingsView';
import StatusDot from './components/StatusDot';
import CommandPalette from './components/CommandPalette';
import { ToastProvider, useToast } from './components/Toast';
import { useHealth } from './hooks/useHealth';
import { MessageSquare, LayoutDashboard, Settings, Command } from 'lucide-react';

type View = 'chat' | 'dashboard' | 'settings';

function AppInner() {
  const [view, setView] = useState<View>('chat');
  const [paletteOpen, setPaletteOpen] = useState(false);
  const { health, sessions, connected } = useHealth();
  const toast = useToast();

  const handleCommand = useCallback((cmd: string) => {
    switch (cmd) {
      case 'new': setView('chat'); toast('New conversation', 'info'); break;
      case 'clear': toast('Chat cleared', 'info'); break;
      case 'doctor': setView('dashboard'); break;
    }
  }, [toast]);

  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setPaletteOpen(p => !p);
      }
    };
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  }, []);

  const tabs: { id: View; icon: typeof MessageSquare; label: string }[] = [
    { id: 'chat', icon: MessageSquare, label: 'Chat' },
    { id: 'dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    { id: 'settings', icon: Settings, label: 'Settings' },
  ];

  return (
    <div className="app">
      <nav className="nav">
        <div className="nav-brand">
          <div className="nav-logo">&#x2B21;</div>
          <span className="nav-wordmark">SoulGate</span>
        </div>
        <div className="nav-tabs">
          {tabs.map(t => (
            <button key={t.id} className={`nav-tab ${view === t.id ? 'active' : ''}`} onClick={() => setView(t.id)}>
              <t.icon size={15} />
              <span>{t.label}</span>
            </button>
          ))}
        </div>
        <div className="nav-right">
          <button className="nav-cmd-btn" onClick={() => setPaletteOpen(true)} title="Ctrl+K">
            <Command size={14} />
          </button>
          <StatusDot connected={connected} />
        </div>
      </nav>

      {!connected && <div className="disconnect-banner">Gateway disconnected — reconnecting...</div>}

      <main className="main-content">
        {view === 'chat' && <ChatView />}
        {view === 'dashboard' && <DashboardView health={health} sessions={sessions} />}
        {view === 'settings' && <SettingsView health={health} />}
      </main>

      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} onCommand={handleCommand} />
    </div>
  );
}

export default function App() {
  return (
    <ToastProvider>
      <AppInner />
    </ToastProvider>
  );
}
