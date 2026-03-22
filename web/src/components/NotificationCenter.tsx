import { useState, useEffect, useRef, useCallback } from 'react';
import { Bell, CheckCheck, X, Wifi, WifiOff, Bot, AlertTriangle, Info, Zap, Trash2, Activity } from 'lucide-react';
import {
  fetchNotifications, markNotificationRead, deleteNotification,
  markAllNotificationsRead, clearReadNotifications,
  type InboxNotification,
} from '../lib/api';

// Re-export the type for consumers
export type Notification = InboxNotification;

// ── Icon per kind ─────────────────────────────────────────────────────────────

const KIND_META: Record<string, { icon: React.ElementType; classes: string }> = {
  success:    { icon: Zap,           classes: 'text-emerald-400 bg-emerald-500/10' },
  error:      { icon: AlertTriangle, classes: 'text-red-400 bg-red-500/10' },
  warning:    { icon: AlertTriangle, classes: 'text-yellow-400 bg-yellow-500/10' },
  info:       { icon: Info,          classes: 'text-sky-400 bg-sky-500/10' },
  agent:      { icon: Bot,           classes: 'text-violet-400 bg-violet-500/10' },
  connection: { icon: Wifi,          classes: 'text-indigo-400 bg-indigo-500/10' },
  activity:   { icon: Activity,      classes: 'text-sky-400 bg-sky-500/10' },
};
const defaultKind = { icon: Info, classes: 'text-zinc-400 bg-zinc-500/10' };

function relativeTime(iso: string) {
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 60000) return 'just now';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  return `${Math.floor(diff / 86400000)}d ago`;
}

// ── Hook: fetch persistent notifications from backend ─────────────────────────

export function useNotifications() {
  const [notifications, setNotifications] = useState<InboxNotification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);

  const refresh = useCallback(async () => {
    try {
      const data = await fetchNotifications();
      setNotifications(data.notifications || []);
      setUnreadCount(data.unread || 0);
    } catch {
      // Silently fail
    }
  }, []);

  // Poll every 3 seconds
  useEffect(() => {
    refresh();
    const t = setInterval(refresh, 3000);
    return () => clearInterval(t);
  }, [refresh]);

  const markAllRead = useCallback(async () => {
    await markAllNotificationsRead();
    refresh();
  }, [refresh]);

  const dismiss = useCallback(async (id: string) => {
    await deleteNotification(id);
    refresh();
  }, [refresh]);

  const markRead = useCallback(async (id: string) => {
    await markNotificationRead(id);
    refresh();
  }, [refresh]);

  const clearRead = useCallback(async () => {
    await clearReadNotifications();
    refresh();
  }, [refresh]);

  return { notifications, unreadCount, markAllRead, dismiss, markRead, clearRead, refresh };
}

// ── Detail view for a single notification ─────────────────────────────────────

function NotificationDetail({ notification, onBack, onDismiss }: {
  notification: InboxNotification;
  onBack: () => void;
  onDismiss: (id: string) => void;
}) {
  const meta = KIND_META[notification.kind] || defaultKind;
  const Icon = meta.icon;

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-zinc-800">
        <button onClick={onBack} className="text-zinc-500 hover:text-zinc-300 text-xs">Back</button>
        <div className="flex-1" />
        <button
          onClick={() => onDismiss(notification.id)}
          className="text-xs text-zinc-600 hover:text-red-400 transition-colors"
        >
          Delete
        </button>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        <div className="flex items-start gap-3 mb-4">
          <div className={`flex-shrink-0 w-9 h-9 rounded-lg flex items-center justify-center ${meta.classes}`}>
            <Icon size={16} />
          </div>
          <div className="flex-1">
            <h3 className="text-sm font-semibold text-zinc-200">{notification.title}</h3>
            <p className="text-xs text-zinc-500 mt-0.5">{relativeTime(notification.timestamp)}</p>
          </div>
        </div>
        {notification.detail && (
          <div className="bg-zinc-800/40 rounded-lg p-3 border border-zinc-700/30">
            <p className="text-sm text-zinc-300 whitespace-pre-wrap">{notification.detail}</p>
          </div>
        )}
        {notification.metadata && Object.keys(notification.metadata).length > 0 && (
          <div className="mt-3 space-y-1">
            {Object.entries(notification.metadata).map(([k, v]) => (
              <div key={k} className="flex items-center gap-2 text-xs">
                <span className="text-zinc-600 w-24">{k}</span>
                <span className="text-zinc-400 font-mono">{String(v)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Notification Panel ────────────────────────────────────────────────────────

interface PanelProps {
  notifications: InboxNotification[];
  unreadCount: number;
  onMarkAllRead: () => void;
  onDismiss: (id: string) => void;
  onMarkRead: (id: string) => void;
  onClearRead: () => void;
  connected: boolean;
}

function NotificationPanel({ notifications, unreadCount, onMarkAllRead, onDismiss, onMarkRead, onClearRead, connected }: PanelProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const selected = selectedId ? notifications.find(n => n.id === selectedId) : null;

  if (selected) {
    return (
      <div className="w-96 h-[28rem] bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl overflow-hidden flex flex-col">
        <NotificationDetail
          notification={selected}
          onBack={() => setSelectedId(null)}
          onDismiss={(id) => { onDismiss(id); setSelectedId(null); }}
        />
      </div>
    );
  }

  const readCount = notifications.filter(n => n.read).length;

  return (
    <div className="w-96 bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800">
        <div className="flex items-center gap-2">
          {connected
            ? <Wifi size={13} className="text-emerald-400" />
            : <WifiOff size={13} className="text-red-400" />
          }
          <span className="text-sm font-semibold text-zinc-200">Notifications</span>
          {unreadCount > 0 && (
            <span className="text-xs bg-indigo-500/20 text-indigo-400 px-1.5 py-0.5 rounded-full font-medium">
              {unreadCount} new
            </span>
          )}
          <span className="text-[10px] text-zinc-600">{notifications.length} total</span>
        </div>
        <div className="flex items-center gap-1">
          {unreadCount > 0 && (
            <button
              onClick={onMarkAllRead}
              className="flex items-center gap-1 text-[10px] text-zinc-500 hover:text-zinc-300 transition-colors px-1.5 py-0.5 rounded hover:bg-zinc-800"
              title="Mark all read"
            >
              <CheckCheck size={11} />
            </button>
          )}
          {readCount > 0 && (
            <button
              onClick={onClearRead}
              className="flex items-center gap-1 text-[10px] text-zinc-600 hover:text-red-400 transition-colors px-1.5 py-0.5 rounded hover:bg-zinc-800"
              title="Clear read notifications"
            >
              <Trash2 size={11} />
            </button>
          )}
        </div>
      </div>

      {/* List */}
      <div className="max-h-96 overflow-y-auto divide-y divide-zinc-800/60" style={{ scrollbarWidth: 'thin' }}>
        {notifications.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
            <Bell size={20} className="text-zinc-700" />
            <p className="text-xs text-zinc-600">No notifications yet</p>
            <p className="text-[10px] text-zinc-700">Events will appear here automatically</p>
          </div>
        ) : (
          notifications.map(n => {
            const meta = KIND_META[n.kind] || defaultKind;
            const Icon = meta.icon;
            return (
              <div
                key={n.id}
                onClick={() => { onMarkRead(n.id); setSelectedId(n.id); }}
                className={`flex items-start gap-3 px-4 py-3 transition-colors hover:bg-zinc-800/40 cursor-pointer ${n.read ? 'opacity-50' : ''}`}
              >
                <div className={`flex-shrink-0 w-7 h-7 rounded-lg flex items-center justify-center ${meta.classes}`}>
                  <Icon size={13} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-xs font-medium text-zinc-200 truncate">{n.title}</p>
                    {!n.read && (
                      <span className="w-1.5 h-1.5 rounded-full bg-indigo-500 flex-shrink-0" />
                    )}
                  </div>
                  {n.detail && (
                    <p className="text-xs text-zinc-500 mt-0.5 truncate">{n.detail}</p>
                  )}
                  <p className="text-[10px] text-zinc-700 mt-1">{relativeTime(n.timestamp)}</p>
                </div>
                <button
                  onClick={e => { e.stopPropagation(); onDismiss(n.id); }}
                  className="flex-shrink-0 p-0.5 rounded hover:bg-zinc-700 text-zinc-700 hover:text-zinc-400 transition-colors"
                >
                  <X size={11} />
                </button>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

// ── Bell button (exported for Sidebar) ────────────────────────────────────────

interface BellProps {
  notifications: InboxNotification[];
  unreadCount: number;
  onMarkAllRead: () => void;
  onDismiss: (id: string) => void;
  onMarkRead: (id: string) => void;
  onClearRead: () => void;
  connected: boolean;
  collapsed?: boolean;
}

export default function NotificationCenter({
  notifications,
  unreadCount,
  onMarkAllRead,
  onDismiss,
  onMarkRead,
  onClearRead,
  connected,
  collapsed = false,
}: BellProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const btnRef = useRef<HTMLButtonElement>(null);
  const [panelPos, setPanelPos] = useState({ left: 0, bottom: 0 });

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  const handleOpen = () => {
    if (!open && btnRef.current) {
      const rect = btnRef.current.getBoundingClientRect();
      setPanelPos({
        left: rect.right + 8,
        bottom: Math.max(8, window.innerHeight - rect.bottom),
      });
    }
    setOpen(o => !o);
  };

  return (
    <div ref={ref} className="relative">
      <button
        ref={btnRef}
        onClick={handleOpen}
        title="Notifications"
        className={`relative flex items-center justify-center rounded-lg transition-all ${
          open ? 'bg-zinc-700 text-zinc-200' : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800'
        } ${collapsed ? 'w-9 h-9' : 'w-full px-3 py-2 gap-2'}`}
      >
        <Bell size={16} className="flex-shrink-0" />
        {!collapsed && <span className="text-xs flex-1 text-left">Notifications</span>}
        {unreadCount > 0 && (
          <span className={`absolute flex items-center justify-center bg-indigo-500 text-white text-[9px] font-bold rounded-full leading-none ${
            collapsed ? 'top-1 right-1 w-4 h-4' : 'top-1.5 right-1.5 min-w-[16px] h-4 px-1'
          }`}>
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="fixed z-50" style={{ left: panelPos.left, bottom: panelPos.bottom }}>
          <NotificationPanel
            notifications={notifications}
            unreadCount={unreadCount}
            onMarkAllRead={() => { onMarkAllRead(); }}
            onDismiss={onDismiss}
            onMarkRead={onMarkRead}
            onClearRead={onClearRead}
            connected={connected}
          />
        </div>
      )}
    </div>
  );
}
