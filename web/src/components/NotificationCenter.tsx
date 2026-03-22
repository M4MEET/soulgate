import { useState, useEffect, useRef, useCallback } from 'react';
import { Bell, CheckCheck, X, Wifi, WifiOff, Bot, AlertTriangle, Info, Zap } from 'lucide-react';
import type { HealthData, SessionData } from '../lib/api';
import { formatRelativeTime } from '../lib/utils';

// ── Types ─────────────────────────────────────────────────────────────────────

type NotifKind = 'success' | 'error' | 'warning' | 'info' | 'agent' | 'connection';

export interface Notification {
  id: string;
  kind: NotifKind;
  title: string;
  detail?: string;
  timestamp: Date;
  read: boolean;
}

// ── Icon per kind ─────────────────────────────────────────────────────────────

const KIND_META: Record<NotifKind, { icon: React.ElementType; classes: string }> = {
  success:    { icon: Zap,           classes: 'text-emerald-400 bg-emerald-500/10' },
  error:      { icon: AlertTriangle, classes: 'text-red-400 bg-red-500/10' },
  warning:    { icon: AlertTriangle, classes: 'text-yellow-400 bg-yellow-500/10' },
  info:       { icon: Info,          classes: 'text-sky-400 bg-sky-500/10' },
  agent:      { icon: Bot,           classes: 'text-violet-400 bg-violet-500/10' },
  connection: { icon: Wifi,          classes: 'text-indigo-400 bg-indigo-500/10' },
};

// ── Hook: feed notifications from health polling ───────────────────────────────

let _notifId = 0;
const nid = () => `n_${++_notifId}`;

export function useNotifications(health: HealthData | null, sessions: SessionData[], connected: boolean) {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const prevConnected = useRef<boolean | null>(null);
  const prevSessionCount = useRef<number>(0);
  const prevStatus = useRef<string | null>(null);
  const prevModel = useRef<string | null>(null);

  const push = useCallback((n: Omit<Notification, 'id' | 'timestamp' | 'read'>) => {
    setNotifications(prev => [
      { ...n, id: nid(), timestamp: new Date(), read: false },
      ...prev.slice(0, 49), // cap at 50
    ]);
  }, []);

  // Connection changes
  useEffect(() => {
    if (prevConnected.current === null) {
      prevConnected.current = connected;
      return;
    }
    if (prevConnected.current !== connected) {
      push(connected
        ? { kind: 'connection', title: 'Gateway connected', detail: 'SoulGate is reachable' }
        : { kind: 'error',      title: 'Gateway disconnected', detail: 'Attempting to reconnect…' }
      );
      prevConnected.current = connected;
    }
  }, [connected, push]);

  // New sessions
  useEffect(() => {
    const count = sessions.length;
    if (prevSessionCount.current > 0 && count > prevSessionCount.current) {
      const diff = count - prevSessionCount.current;
      push({ kind: 'info', title: `${diff} new session${diff > 1 ? 's' : ''}`, detail: `Total active sessions: ${count}` });
    }
    prevSessionCount.current = count;
  }, [sessions, push]);

  // Health status changes
  useEffect(() => {
    if (!health) return;
    if (prevStatus.current !== null && prevStatus.current !== health.status) {
      push(health.status === 'ok'
        ? { kind: 'success', title: 'Gateway status: OK', detail: 'All checks passed' }
        : { kind: 'warning', title: `Gateway status: ${health.status}`, detail: health.checks?.find(c => c.status !== 'ok')?.detail }
      );
    }
    prevStatus.current = health.status;
  }, [health, push]);

  // Model switch
  useEffect(() => {
    if (!health) return;
    const model = health.model;
    if (prevModel.current !== null && prevModel.current !== model) {
      push({ kind: 'agent', title: 'Model switched', detail: `Now using ${model}` });
    }
    prevModel.current = model ?? null;
  }, [health, push]);

  const markAllRead = useCallback(() => {
    setNotifications(prev => prev.map(n => ({ ...n, read: true })));
  }, []);

  const dismiss = useCallback((id: string) => {
    setNotifications(prev => prev.filter(n => n.id !== id));
  }, []);

  const unreadCount = notifications.filter(n => !n.read).length;

  return { notifications, unreadCount, markAllRead, dismiss };
}

// ── Dropdown panel ────────────────────────────────────────────────────────────

interface PanelProps {
  notifications: Notification[];
  unreadCount: number;
  onMarkAllRead: () => void;
  onDismiss: (id: string) => void;
  connected: boolean;
}

function NotificationPanel({ notifications, unreadCount, onMarkAllRead, onDismiss, connected }: PanelProps) {
  return (
    <div className="absolute bottom-full mb-2 right-0 w-80 bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl z-40 overflow-hidden">
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
              {unreadCount}
            </span>
          )}
        </div>
        {unreadCount > 0 && (
          <button
            onClick={onMarkAllRead}
            className="flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            <CheckCheck size={12} />
            Mark all read
          </button>
        )}
      </div>

      {/* List */}
      <div className="max-h-80 overflow-y-auto divide-y divide-zinc-800/60">
        {notifications.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
            <Bell size={20} className="text-zinc-700" />
            <p className="text-xs text-zinc-600">No notifications yet</p>
          </div>
        ) : (
          notifications.map(n => {
            const { icon: Icon, classes } = KIND_META[n.kind];
            return (
              <div
                key={n.id}
                className={`flex items-start gap-3 px-4 py-3 transition-colors hover:bg-zinc-800/40 ${n.read ? 'opacity-60' : ''}`}
              >
                {/* Kind icon */}
                <div className={`flex-shrink-0 w-7 h-7 rounded-lg flex items-center justify-center ${classes}`}>
                  <Icon size={13} />
                </div>

                {/* Text */}
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
                  <p className="text-xs text-zinc-700 mt-1">{formatRelativeTime(n.timestamp)}</p>
                </div>

                {/* Dismiss */}
                <button
                  onClick={() => onDismiss(n.id)}
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
  notifications: Notification[];
  unreadCount: number;
  onMarkAllRead: () => void;
  onDismiss: (id: string) => void;
  connected: boolean;
  collapsed?: boolean;
}

export default function NotificationCenter({
  notifications,
  unreadCount,
  onMarkAllRead,
  onDismiss,
  connected,
  collapsed = false,
}: BellProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(o => !o)}
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
        <NotificationPanel
          notifications={notifications}
          unreadCount={unreadCount}
          onMarkAllRead={() => { onMarkAllRead(); }}
          onDismiss={onDismiss}
          connected={connected}
        />
      )}
    </div>
  );
}
