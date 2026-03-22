import {
  useState,
  useRef,
  useEffect,
  useCallback,
  type KeyboardEvent,
} from 'react';
import ChatMessage, { type Message } from '../components/ChatMessage';
import ChatInput from '../components/ChatInput';
import { streamChatSSE } from '../lib/api';
import {
  MessageSquare,
  Sparkles,
  Terminal,
  Globe,
  Brain,
  ChevronDown,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Search,
  Clock,
  MoreHorizontal,
  Pencil,
  GitFork,
  Download,
  Pin,
  Archive,
  Trash2,
  X,
  Check,
  ChevronRight,
  Copy,
  RotateCcw,
  FileText,
} from 'lucide-react';
import toast from 'react-hot-toast';

// ── ID helpers ─────────────────────────────────────────────────────────────────

let _msgId = 0;
const uid = () => `msg_${++_msgId}_${Date.now()}`;
const threadId = () => `thread_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;

// ── Models ────────────────────────────────────────────────────────────────────

const MODELS = [
  'claude-opus-4-5',
  'claude-sonnet-4-5',
  'claude-haiku-3-5',
  'gpt-4o',
  'gpt-4o-mini',
  'gpt-4-turbo',
];

// ── Thread data model ─────────────────────────────────────────────────────────

interface ChatThread {
  id: string;
  title: string;
  messages: Message[];
  model: string;
  createdAt: string;   // ISO string for JSON serialization
  updatedAt: string;
  archived: boolean;
  pinned: boolean;
  tags: string[];
  tokenCount: number;
  costTotal: number;
}

// ── localStorage persistence ──────────────────────────────────────────────────

const STORAGE_KEY = 'soulgate-threads';

function saveThreads(threads: ChatThread[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(threads));
  } catch {
    // Storage quota exceeded or unavailable — fail silently
  }
}

function loadThreads(): ChatThread[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as ChatThread[];
    // Ensure messages have proper Date objects reconstructed later as strings
    return parsed;
  } catch {
    return [];
  }
}

function hydrateMessages(messages: Message[]): Message[] {
  return messages.map(m => ({
    ...m,
    timestamp: m.timestamp instanceof Date ? m.timestamp : new Date(m.timestamp as unknown as string),
  }));
}

// ── Auto-title ────────────────────────────────────────────────────────────────

function autoTitle(firstUserMessage: string): string {
  const clean = firstUserMessage.replace(/\s+/g, ' ').trim();
  return clean.length > 40 ? clean.slice(0, 40) + '…' : clean;
}

// ── Export helpers ────────────────────────────────────────────────────────────

function exportAsJSON(thread: ChatThread): void {
  const data = JSON.stringify(thread, null, 2);
  const blob = new Blob([data], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${thread.title.slice(0, 40).replace(/[^a-z0-9]/gi, '_')}.json`;
  a.click();
  URL.revokeObjectURL(url);
}

function exportAsMarkdown(thread: ChatThread): void {
  const header = `# ${thread.title}\n\n*Exported from SoulGate — ${new Date().toLocaleString()}*\n\n---\n\n`;
  const body = thread.messages
    .filter(m => m.role !== 'system')
    .map(m => {
      if (m.role === 'user') return `## You\n\n${m.content}\n`;
      return `## SoulGate\n\n${m.content}\n`;
    })
    .join('\n---\n\n');
  const blob = new Blob([header + body], { type: 'text/markdown' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${thread.title.slice(0, 40).replace(/[^a-z0-9]/gi, '_')}.md`;
  a.click();
  URL.revokeObjectURL(url);
}

// ── Relative time ─────────────────────────────────────────────────────────────

function formatRelative(isoStr: string): string {
  try {
    const diff = Date.now() - new Date(isoStr).getTime();
    const mins = Math.floor(diff / 60_000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    return `${Math.floor(hrs / 24)}d ago`;
  } catch {
    return '';
  }
}

// ── Confirmation dialog ───────────────────────────────────────────────────────

interface ConfirmDialogProps {
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
}

function ConfirmDialog({ message, onConfirm, onCancel }: ConfirmDialogProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl p-6 max-w-sm w-full mx-4">
        <p className="text-sm text-zinc-200 mb-5">{message}</p>
        <div className="flex gap-3 justify-end">
          <button
            onClick={onCancel}
            className="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-all"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className="px-4 py-2 rounded-lg text-sm bg-red-600 hover:bg-red-500 text-white transition-all"
          >
            Delete
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Thread context menu ───────────────────────────────────────────────────────

interface ThreadMenuProps {
  thread: ChatThread;
  onClose: () => void;
  onRename: () => void;
  onFork: () => void;
  onExportJSON: () => void;
  onExportMD: () => void;
  onTogglePin: () => void;
  onArchive: () => void;
  onDelete: () => void;
}

function ThreadMenu({
  thread,
  onClose,
  onRename,
  onFork,
  onExportJSON,
  onExportMD,
  onTogglePin,
  onArchive,
  onDelete,
}: ThreadMenuProps) {
  // Close on outside click
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [onClose]);

  const item = (
    icon: React.ReactNode,
    label: string,
    action: () => void,
    danger = false
  ) => (
    <button
      key={label}
      onClick={() => { action(); onClose(); }}
      className={`flex items-center gap-2.5 w-full px-3 py-2 text-xs text-left transition-colors rounded-lg ${
        danger
          ? 'text-red-400 hover:bg-red-500/10'
          : 'text-zinc-300 hover:bg-zinc-800'
      }`}
    >
      {icon}
      {label}
    </button>
  );

  return (
    <div
      ref={ref}
      className="absolute left-full top-0 ml-1 z-40 w-44 bg-zinc-900 border border-zinc-700/80 rounded-xl shadow-2xl overflow-hidden p-1"
    >
      {item(<Pencil size={13} />, 'Rename', onRename)}
      {item(<GitFork size={13} />, 'Fork thread', onFork)}
      <div className="my-1 border-t border-zinc-800" />
      {item(<Download size={13} />, 'Export JSON', onExportJSON)}
      {item(<FileText size={13} />, 'Export Markdown', onExportMD)}
      <div className="my-1 border-t border-zinc-800" />
      {item(<Pin size={13} />, thread.pinned ? 'Unpin' : 'Pin', onTogglePin)}
      {item(<Archive size={13} />, thread.archived ? 'Unarchive' : 'Archive', onArchive)}
      <div className="my-1 border-t border-zinc-800" />
      {item(<Trash2 size={13} />, 'Delete', onDelete, true)}
    </div>
  );
}

// ── Thread list item ──────────────────────────────────────────────────────────

interface ThreadItemProps {
  thread: ChatThread;
  isActive: boolean;
  onSelect: () => void;
  onRename: (id: string, title: string) => void;
  onFork: (id: string) => void;
  onExportJSON: (id: string) => void;
  onExportMD: (id: string) => void;
  onTogglePin: (id: string) => void;
  onArchive: (id: string) => void;
  onDelete: (id: string) => void;
}

function ThreadItem({
  thread,
  isActive,
  onSelect,
  onRename,
  onFork,
  onExportJSON,
  onExportMD,
  onTogglePin,
  onArchive,
  onDelete,
}: ThreadItemProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [renameValue, setRenameValue] = useState(thread.title);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (renaming && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [renaming]);

  const submitRename = () => {
    const trimmed = renameValue.trim();
    if (trimmed && trimmed !== thread.title) onRename(thread.id, trimmed);
    setRenaming(false);
  };

  const handleRenameKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') submitRename();
    if (e.key === 'Escape') { setRenameValue(thread.title); setRenaming(false); }
  };

  const msgCount = thread.messages.filter(m => m.role !== 'system').length;

  return (
    <div
      className={`relative group flex flex-col w-full px-3 py-2.5 border-b border-zinc-800/30 cursor-pointer transition-all ${
        isActive
          ? 'bg-indigo-500/10 border-l-2 border-l-indigo-500'
          : 'hover:bg-zinc-800/50'
      }`}
      onClick={() => { if (!menuOpen && !renaming) onSelect(); }}
    >
      {/* Pin indicator */}
      {thread.pinned && (
        <Pin size={9} className="absolute top-2 right-7 text-indigo-400/60" />
      )}

      {/* Title row */}
      <div className="flex items-center justify-between gap-1 mb-0.5">
        {renaming ? (
          <input
            ref={inputRef}
            value={renameValue}
            onChange={e => setRenameValue(e.target.value)}
            onKeyDown={handleRenameKey}
            onBlur={submitRename}
            onClick={e => e.stopPropagation()}
            className="flex-1 bg-zinc-800 text-zinc-100 text-xs rounded px-1.5 py-0.5 outline-none border border-indigo-500/60 min-w-0"
          />
        ) : (
          <span className="text-xs font-medium text-zinc-300 truncate flex-1">
            {thread.title}
          </span>
        )}
        {!renaming && (
          <button
            onClick={e => { e.stopPropagation(); setMenuOpen(o => !o); }}
            className="flex-shrink-0 opacity-0 group-hover:opacity-100 flex items-center justify-center w-5 h-5 rounded text-zinc-500 hover:text-zinc-300 hover:bg-zinc-700 transition-all"
          >
            <MoreHorizontal size={12} />
          </button>
        )}
      </div>

      {/* Meta row */}
      <div className="flex items-center gap-2">
        <span className="text-xs text-zinc-600 font-mono truncate">{thread.model?.split('-')[0]}</span>
        {msgCount > 0 && (
          <span className="text-xs text-zinc-700">{msgCount} msg{msgCount !== 1 ? 's' : ''}</span>
        )}
        <span className="text-xs text-zinc-600 flex-1 text-right">{formatRelative(thread.updatedAt)}</span>
      </div>

      {/* Context menu */}
      {menuOpen && (
        <ThreadMenu
          thread={thread}
          onClose={() => setMenuOpen(false)}
          onRename={() => { setRenameValue(thread.title); setRenaming(true); }}
          onFork={() => onFork(thread.id)}
          onExportJSON={() => onExportJSON(thread.id)}
          onExportMD={() => onExportMD(thread.id)}
          onTogglePin={() => onTogglePin(thread.id)}
          onArchive={() => onArchive(thread.id)}
          onDelete={() => onDelete(thread.id)}
        />
      )}
    </div>
  );
}

// ── Thread sidebar ────────────────────────────────────────────────────────────

interface SidebarProps {
  threads: ChatThread[];
  activeId: string | null;
  onNewChat: () => void;
  onSelect: (id: string) => void;
  onRename: (id: string, title: string) => void;
  onFork: (id: string) => void;
  onExportJSON: (id: string) => void;
  onExportMD: (id: string) => void;
  onTogglePin: (id: string) => void;
  onArchive: (id: string) => void;
  onDelete: (id: string) => void;
}

function ThreadSidebar({
  threads,
  activeId,
  onNewChat,
  onSelect,
  onRename,
  onFork,
  onExportJSON,
  onExportMD,
  onTogglePin,
  onArchive,
  onDelete,
}: SidebarProps) {
  const [query, setQuery] = useState('');
  const [archivedExpanded, setArchivedExpanded] = useState(false);

  const active = threads.filter(t => !t.archived);
  const archived = threads.filter(t => t.archived);

  const filterThreads = (list: ChatThread[]) =>
    query.trim()
      ? list.filter(t =>
          t.title.toLowerCase().includes(query.toLowerCase()) ||
          t.model.toLowerCase().includes(query.toLowerCase())
        )
      : list;

  // Pinned first, then by updatedAt
  const sorted = (list: ChatThread[]) =>
    [...filterThreads(list)].sort((a, b) => {
      if (a.pinned && !b.pinned) return -1;
      if (!a.pinned && b.pinned) return 1;
      return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
    });

  const itemProps = (thread: ChatThread) => ({
    thread,
    isActive: thread.id === activeId,
    onSelect: () => onSelect(thread.id),
    onRename,
    onFork,
    onExportJSON,
    onExportMD,
    onTogglePin,
    onArchive,
    onDelete,
  });

  return (
    <div
      className="flex flex-col h-full bg-zinc-900 border-r border-zinc-800 flex-shrink-0"
      style={{ width: 260 }}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-3 border-b border-zinc-800 flex-shrink-0">
        <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">Threads</span>
        <button
          onClick={onNewChat}
          title="New chat"
          className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs text-white bg-indigo-600 hover:bg-indigo-500 transition-all font-medium"
        >
          <Plus size={12} />
          New Chat
        </button>
      </div>

      {/* Search */}
      <div className="px-3 py-2 border-b border-zinc-800/60 flex-shrink-0">
        <div className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg bg-zinc-800/60 border border-zinc-700/40">
          <Search size={12} className="text-zinc-600 flex-shrink-0" />
          <input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search threads…"
            className="flex-1 bg-transparent text-xs text-zinc-300 placeholder-zinc-600 outline-none min-w-0"
          />
          {query && (
            <button onClick={() => setQuery('')} className="text-zinc-600 hover:text-zinc-400">
              <X size={11} />
            </button>
          )}
        </div>
      </div>

      {/* Thread list */}
      <div
        className="flex-1 overflow-y-auto"
        style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
      >
        {active.length === 0 && archived.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 px-4 gap-2 text-center">
            <Clock size={18} className="text-zinc-700" />
            <p className="text-xs text-zinc-600">No threads yet. Start a new chat.</p>
          </div>
        ) : sorted(active).length === 0 && query ? (
          <div className="py-8 text-center text-xs text-zinc-600">No matches</div>
        ) : (
          sorted(active).map(t => <ThreadItem key={t.id} {...itemProps(t)} />)
        )}

        {/* Archived section */}
        {archived.length > 0 && (
          <div className="mt-2">
            <button
              onClick={() => setArchivedExpanded(e => !e)}
              className="flex items-center gap-1.5 w-full px-3 py-2 text-xs text-zinc-500 hover:text-zinc-400 transition-colors"
            >
              {archivedExpanded ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
              <Archive size={11} />
              Archived ({archived.length})
            </button>
            {archivedExpanded && sorted(archived).map(t => <ThreadItem key={t.id} {...itemProps(t)} />)}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Model selector ────────────────────────────────────────────────────────────

function ModelSelector({ model, onChange }: { model: string; onChange: (m: string) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="relative">
      <button
        onClick={() => setOpen(s => !s)}
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-xs text-zinc-400 hover:text-zinc-200 transition-all"
      >
        <span className="font-mono">{model}</span>
        <ChevronDown size={12} />
      </button>
      {open && (
        <div className="absolute bottom-full mb-1 left-0 z-30 bg-zinc-900 border border-zinc-700 rounded-lg shadow-xl overflow-hidden min-w-48">
          {MODELS.map(m => (
            <button
              key={m}
              onClick={() => { onChange(m); setOpen(false); toast.success(`Model: ${m}`); }}
              className={`block w-full px-3 py-2 text-left text-xs font-mono hover:bg-zinc-800 transition-colors ${
                m === model ? 'text-indigo-400 bg-indigo-500/10' : 'text-zinc-400'
              }`}
            >
              {m}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Chat header ───────────────────────────────────────────────────────────────

interface ChatHeaderProps {
  thread: ChatThread | null;
  showSidebar: boolean;
  onToggleSidebar: () => void;
  onRenameThread: (title: string) => void;
  onForkThread: () => void;
  onExportJSON: () => void;
  onExportMD: () => void;
  onDeleteThread: () => void;
}

function ChatHeader({
  thread,
  showSidebar,
  onToggleSidebar,
  onRenameThread,
  onForkThread,
  onExportJSON,
  onExportMD,
  onDeleteThread,
}: ChatHeaderProps) {
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState('');
  const [actionsOpen, setActionsOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const actionsRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editing]);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (actionsRef.current && !actionsRef.current.contains(e.target as Node)) {
        setActionsOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  const startEdit = () => {
    if (!thread) return;
    setEditValue(thread.title);
    setEditing(true);
  };

  const submitEdit = () => {
    const trimmed = editValue.trim();
    if (trimmed) onRenameThread(trimmed);
    setEditing(false);
  };

  const handleEditKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') submitEdit();
    if (e.key === 'Escape') setEditing(false);
  };

  const tokenLabel = thread
    ? thread.tokenCount > 0
      ? `${thread.tokenCount.toLocaleString()} tok`
      : null
    : null;

  const costLabel = thread && thread.costTotal > 0
    ? `$${thread.costTotal.toFixed(4)}`
    : null;

  return (
    <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800/60 flex-shrink-0 gap-3">
      <div className="flex items-center gap-2 min-w-0">
        {/* Sidebar toggle */}
        <button
          onClick={onToggleSidebar}
          title={showSidebar ? 'Hide sidebar' : 'Show sidebar'}
          className="flex-shrink-0 flex items-center justify-center w-7 h-7 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-all"
        >
          {showSidebar ? <PanelLeftClose size={15} /> : <PanelLeftOpen size={15} />}
        </button>

        <Sparkles size={15} className="text-indigo-400 flex-shrink-0" />

        {/* Thread title */}
        {thread ? (
          editing ? (
            <input
              ref={inputRef}
              value={editValue}
              onChange={e => setEditValue(e.target.value)}
              onKeyDown={handleEditKey}
              onBlur={submitEdit}
              className="text-sm font-semibold bg-zinc-800 text-zinc-100 rounded px-2 py-0.5 outline-none border border-indigo-500/60 min-w-0 max-w-xs"
            />
          ) : (
            <button
              onClick={startEdit}
              title="Click to rename"
              className="text-sm font-semibold text-zinc-300 hover:text-zinc-100 truncate transition-colors"
            >
              {thread.title}
            </button>
          )
        ) : (
          <span className="text-sm font-semibold text-zinc-300">Chat</span>
        )}

        {/* Token / cost badges */}
        {tokenLabel && (
          <span className="text-xs text-zinc-600 flex-shrink-0">{tokenLabel}</span>
        )}
        {costLabel && (
          <span className="text-xs text-zinc-600 flex-shrink-0">{costLabel}</span>
        )}
      </div>

      <div className="flex items-center gap-2 flex-shrink-0">
        {thread && (
          <div className="relative" ref={actionsRef}>
            <button
              onClick={() => setActionsOpen(o => !o)}
              className="flex items-center justify-center w-7 h-7 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-all"
              title="Thread actions"
            >
              <MoreHorizontal size={15} />
            </button>
            {actionsOpen && (
              <div className="absolute top-full right-0 mt-1 z-30 w-44 bg-zinc-900 border border-zinc-700/80 rounded-xl shadow-2xl overflow-hidden p-1">
                {[
                  { icon: <GitFork size={13} />, label: 'Fork thread', action: onForkThread },
                  { icon: <Download size={13} />, label: 'Export JSON', action: onExportJSON },
                  { icon: <FileText size={13} />, label: 'Export Markdown', action: onExportMD },
                  { icon: <Trash2 size={13} />, label: 'Delete thread', action: onDeleteThread, danger: true },
                ].map(({ icon, label, action, danger }) => (
                  <button
                    key={label}
                    onClick={() => { action(); setActionsOpen(false); }}
                    className={`flex items-center gap-2.5 w-full px-3 py-2 text-xs text-left rounded-lg transition-colors ${
                      danger ? 'text-red-400 hover:bg-red-500/10' : 'text-zinc-300 hover:bg-zinc-800'
                    }`}
                  >
                    {icon}
                    {label}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Message action buttons (per-message hover) ────────────────────────────────

interface MessageActionsProps {
  message: Message;
  allMessages: Message[];
  onRetry?: () => void;
  onForkHere: () => void;
  onDelete: () => void;
  onEdit?: (newContent: string) => void;
}

function MessageActions({ message, allMessages, onRetry, onForkHere, onDelete, onEdit }: MessageActionsProps) {
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState(message.content);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [copied, setCopied] = useState(false);
  const [copiedMd, setCopiedMd] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (editing && textareaRef.current) textareaRef.current.focus();
  }, [editing]);

  const copyText = () => {
    navigator.clipboard.writeText(message.content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const copyMarkdown = () => {
    const role = message.role === 'user' ? '## You' : '## SoulGate';
    navigator.clipboard.writeText(`${role}\n\n${message.content}`);
    setCopiedMd(true);
    setTimeout(() => setCopiedMd(false), 2000);
  };

  const isUser = message.role === 'user';

  // Find the previous user message for retry from assistant
  const prevUserMsg = (() => {
    if (isUser) return null;
    const idx = allMessages.findIndex(m => m.id === message.id);
    if (idx > 0 && allMessages[idx - 1].role === 'user') return allMessages[idx - 1];
    return null;
  })();

  if (editing && isUser && onEdit) {
    return (
      <div className="mt-2 flex flex-col gap-2">
        <textarea
          ref={textareaRef}
          value={editValue}
          onChange={e => setEditValue(e.target.value)}
          rows={3}
          className="w-full bg-zinc-800 text-zinc-100 text-sm rounded-lg px-3 py-2 outline-none border border-indigo-500/60 resize-none"
        />
        <div className="flex gap-2">
          <button
            onClick={() => { onEdit(editValue.trim()); setEditing(false); }}
            disabled={!editValue.trim()}
            className="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium transition-all disabled:opacity-40"
          >
            <Check size={11} /> Send edit
          </button>
          <button
            onClick={() => { setEditValue(message.content); setEditing(false); }}
            className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-zinc-400 hover:bg-zinc-800 text-xs transition-all"
          >
            <X size={11} /> Cancel
          </button>
        </div>
      </div>
    );
  }

  return (
    <>
      {confirmDelete && (
        <ConfirmDialog
          message="Delete this message?"
          onConfirm={() => { onDelete(); setConfirmDelete(false); }}
          onCancel={() => setConfirmDelete(false)}
        />
      )}
      <div className={`flex items-center gap-1 mt-1.5 opacity-0 group-hover:opacity-100 transition-opacity ${isUser ? 'justify-end' : ''}`}>
        <button
          onClick={copyText}
          title="Copy text"
          className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-zinc-600/60 text-zinc-400 hover:text-zinc-200 transition-all"
        >
          {copied ? <Check size={12} /> : <Copy size={12} />}
        </button>
        <button
          onClick={copyMarkdown}
          title="Copy as Markdown"
          className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-zinc-600/60 text-zinc-400 hover:text-zinc-200 transition-all"
        >
          {copiedMd ? <Check size={12} /> : <FileText size={12} />}
        </button>
        {isUser && onEdit && (
          <button
            onClick={() => setEditing(true)}
            title="Edit & re-send"
            className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-zinc-600/60 text-zinc-400 hover:text-zinc-200 transition-all"
          >
            <Pencil size={12} />
          </button>
        )}
        {(onRetry || prevUserMsg) && !isUser && (
          <button
            onClick={onRetry}
            title="Retry"
            className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-zinc-600/60 text-zinc-400 hover:text-zinc-200 transition-all"
          >
            <RotateCcw size={12} />
          </button>
        )}
        <button
          onClick={onForkHere}
          title="Fork from here"
          className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-zinc-600/60 text-zinc-400 hover:text-zinc-200 transition-all"
        >
          <GitFork size={12} />
        </button>
        <button
          onClick={() => setConfirmDelete(true)}
          title="Delete message"
          className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-red-900/40 text-zinc-500 hover:text-red-400 transition-all"
        >
          <Trash2 size={12} />
        </button>
      </div>
    </>
  );
}

// ── Welcome screen ────────────────────────────────────────────────────────────

function WelcomeScreen({ onSend }: { onSend: (t: string) => void }) {
  return (
    <div className="flex-1 flex flex-col items-center justify-center gap-8 p-10">
      <div className="text-center">
        <div className="w-20 h-20 rounded-2xl bg-gradient-to-br from-indigo-600 to-violet-600 flex items-center justify-center mx-auto mb-4 shadow-lg shadow-indigo-500/20">
          <Sparkles size={40} className="text-white" />
        </div>
        <h1 className="text-3xl font-bold text-zinc-100">SoulGate</h1>
        <p className="text-zinc-500 mt-1">Your AI, everywhere.</p>
      </div>
      <div className="grid grid-cols-2 gap-3 max-w-md w-full">
        {[
          { icon: Brain,         label: 'What can you do?',           prompt: 'What can you do?' },
          { icon: Terminal,      label: 'List files',                  prompt: 'List files in the current directory' },
          { icon: Globe,         label: 'Search the web',              prompt: 'Search the web for latest tech news' },
          { icon: MessageSquare, label: 'System status',               prompt: 'Show system status' },
        ].map(({ icon: Icon, label, prompt }) => (
          <button
            key={label}
            onClick={() => onSend(prompt)}
            className="flex items-center gap-3 p-4 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-zinc-400 hover:text-zinc-100 hover:border-indigo-500/40 hover:bg-zinc-800 transition-all text-sm text-left group"
          >
            <Icon size={18} className="text-zinc-500 group-hover:text-indigo-400 transition-colors" />
            <span>{label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

// ── Main ChatView ─────────────────────────────────────────────────────────────

export default function ChatView() {
  const [threads, setThreads] = useState<ChatThread[]>(() => loadThreads());
  const [activeId, setActiveId] = useState<string | null>(() => {
    const saved = loadThreads();
    const first = saved.filter(t => !t.archived).sort((a, b) =>
      new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
    )[0];
    return first?.id ?? null;
  });
  const [streaming, setStreaming] = useState(false);
  const [model, setModel] = useState(MODELS[0]);
  const [showSidebar, setShowSidebar] = useState(true);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Persist threads whenever they change
  useEffect(() => {
    saveThreads(threads);
  }, [threads]);

  const activeThread = threads.find(t => t.id === activeId) ?? null;

  const scrollToBottom = useCallback(() => {
    setTimeout(() => {
      scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
    }, 50);
  }, []);

  useEffect(scrollToBottom, [activeThread?.messages, scrollToBottom]);

  // ── Thread mutations ──────────────────────────────────────────────────────

  const updateThread = useCallback((id: string, updater: (t: ChatThread) => ChatThread) => {
    setThreads(prev => prev.map(t => t.id === id ? updater(t) : t));
  }, []);

  const createThread = useCallback((messages: Message[] = [], title = 'New Chat', inheritModel?: string): ChatThread => {
    const now = new Date().toISOString();
    const t: ChatThread = {
      id: threadId(),
      title,
      messages,
      model: inheritModel ?? model,
      createdAt: now,
      updatedAt: now,
      archived: false,
      pinned: false,
      tags: [],
      tokenCount: 0,
      costTotal: 0,
    };
    setThreads(prev => [t, ...prev]);
    return t;
  }, [model]);

  const handleNewChat = useCallback(() => {
    const t = createThread();
    setActiveId(t.id);
  }, [createThread]);

  const handleSelectThread = useCallback((id: string) => {
    setActiveId(id);
    if (streaming) {
      abortRef.current?.abort();
      setStreaming(false);
    }
  }, [streaming]);

  const handleRenameThread = useCallback((id: string, title: string) => {
    updateThread(id, t => ({ ...t, title, updatedAt: new Date().toISOString() }));
  }, [updateThread]);

  const handleForkThread = useCallback((id: string, upToMessageId?: string) => {
    const src = threads.find(t => t.id === id);
    if (!src) return;
    let msgs = src.messages;
    if (upToMessageId) {
      const idx = msgs.findIndex(m => m.id === upToMessageId);
      if (idx >= 0) msgs = msgs.slice(0, idx + 1);
    }
    const forked = createThread(hydrateMessages(msgs), `Fork of ${src.title}`, src.model);
    setActiveId(forked.id);
    toast.success('Thread forked');
  }, [threads, createThread]);

  const handleExportJSON = useCallback((id: string) => {
    const t = threads.find(th => th.id === id);
    if (t) exportAsJSON(t);
  }, [threads]);

  const handleExportMD = useCallback((id: string) => {
    const t = threads.find(th => th.id === id);
    if (t) exportAsMarkdown(t);
  }, [threads]);

  const handleTogglePin = useCallback((id: string) => {
    updateThread(id, t => ({ ...t, pinned: !t.pinned, updatedAt: new Date().toISOString() }));
  }, [updateThread]);

  const handleArchive = useCallback((id: string) => {
    updateThread(id, t => ({ ...t, archived: !t.archived, updatedAt: new Date().toISOString() }));
    if (id === activeId) {
      const next = threads.find(t => t.id !== id && !t.archived);
      setActiveId(next?.id ?? null);
    }
    toast.success('Thread archived');
  }, [updateThread, activeId, threads]);

  const handleDeleteThread = useCallback((id: string) => {
    setConfirmDeleteId(id);
  }, []);

  const confirmDelete = useCallback(() => {
    if (!confirmDeleteId) return;
    setThreads(prev => prev.filter(t => t.id !== confirmDeleteId));
    if (activeId === confirmDeleteId) {
      const next = threads.find(t => t.id !== confirmDeleteId && !t.archived);
      setActiveId(next?.id ?? null);
    }
    setConfirmDeleteId(null);
    toast.success('Thread deleted');
  }, [confirmDeleteId, activeId, threads]);

  // ── Message mutations ────────────────────────────────────────────────────

  const appendMessages = useCallback((threadId: string, msgs: Message[]) => {
    updateThread(threadId, t => ({
      ...t,
      messages: [...t.messages, ...msgs],
      updatedAt: new Date().toISOString(),
    }));
  }, [updateThread]);

  const updateMessage = useCallback((threadId: string, msgId: string, updater: (m: Message) => Message) => {
    updateThread(threadId, t => ({
      ...t,
      messages: t.messages.map(m => m.id === msgId ? updater(m) : m),
      updatedAt: new Date().toISOString(),
    }));
  }, [updateThread]);

  const deleteMessage = useCallback((threadId: string, msgId: string) => {
    updateThread(threadId, t => ({
      ...t,
      messages: t.messages.filter(m => m.id !== msgId),
      updatedAt: new Date().toISOString(),
    }));
  }, [updateThread]);

  // ── Send handler ──────────────────────────────────────────────────────────

  const handleSend = useCallback(async (text: string, overrideThreadId?: string) => {
    // Determine which thread to operate on
    let tid = overrideThreadId ?? activeId;

    // If no thread exists yet, create one
    if (!tid || !threads.find(t => t.id === tid)) {
      const t = createThread([], 'New Chat');
      tid = t.id;
      setActiveId(tid);
    }

    const userMsg: Message = { id: uid(), role: 'user', content: text, timestamp: new Date() };
    const aiId = uid();
    const aiMsg: Message = { id: aiId, role: 'assistant', content: '', timestamp: new Date(), streaming: true };

    // Auto-title: if this is the first user message
    const currentThread = threads.find(t => t.id === tid);
    const isFirstMessage = !currentThread || currentThread.messages.filter(m => m.role === 'user').length === 0;

    appendMessages(tid, [userMsg, aiMsg]);

    if (isFirstMessage) {
      updateThread(tid, t => ({ ...t, title: autoTitle(text), updatedAt: new Date().toISOString() }));
    }

    setStreaming(true);
    abortRef.current = new AbortController();

    const capturedTid = tid;
    try {
      let fullText = '';
      const thinkingLog: string[] = [];
      for await (const evt of streamChatSSE(text, abortRef.current.signal)) {
        switch (evt.kind) {
          case 'iteration':
            thinkingLog.push(`── ${evt.message} ──`);
            updateMessage(capturedTid, aiId, m => ({ ...m, thinkingLog: [...thinkingLog] }));
            break;
          case 'model_call':
            thinkingLog.push(`⟶ ${evt.message}`);
            updateMessage(capturedTid, aiId, m => ({ ...m, thinkingLog: [...thinkingLog] }));
            break;
          case 'model_done':
            thinkingLog.push(`⟵ ${evt.message}${evt.tokens ? ` (${evt.tokens} tok)` : ''}`);
            updateMessage(capturedTid, aiId, m => ({ ...m, thinkingLog: [...thinkingLog] }));
            break;
          case 'tool_start':
            thinkingLog.push(`⚡ ${evt.message} ${evt.data || ''}`);
            updateMessage(capturedTid, aiId, m => ({ ...m, thinkingLog: [...thinkingLog] }));
            break;
          case 'tool_done':
            thinkingLog.push(`  ↳ ${evt.data?.slice(0, 100) || 'done'}`);
            updateMessage(capturedTid, aiId, m => ({ ...m, thinkingLog: [...thinkingLog] }));
            break;
          case 'status':
            thinkingLog.push(`  ${evt.message}`);
            updateMessage(capturedTid, aiId, m => ({ ...m, thinkingLog: [...thinkingLog] }));
            break;
          case 'stream':
            fullText += evt.message;
            updateMessage(capturedTid, aiId, m => ({ ...m, content: fullText }));
            break;
          case 'done':
            if (evt.message && !fullText) fullText = evt.message;
            updateMessage(capturedTid, aiId, m => ({ ...m, content: fullText || evt.message, streaming: false }));
            break;
          case 'error':
            updateMessage(capturedTid, aiId, m => ({ ...m, content: `Error: ${evt.message}`, streaming: false }));
            break;
        }
      }
      updateMessage(capturedTid, aiId, m => ({ ...m, streaming: false }));
    } catch (err: unknown) {
      if ((err as Error).name === 'AbortError') {
        updateMessage(capturedTid, aiId, m => ({
          ...m, content: m.content || '(cancelled)', streaming: false,
        }));
        return;
      }
      const msg = (err as Error).message;
      updateMessage(capturedTid, aiId, m => ({ ...m, content: `Error: ${msg}`, streaming: false }));
      toast.error(msg);
    } finally {
      setStreaming(false);
    }
  }, [activeId, threads, createThread, appendMessages, updateMessage, updateThread]);

  const handleCancel = useCallback(() => {
    abortRef.current?.abort();
    setStreaming(false);
    if (activeId) {
      updateThread(activeId, t => ({
        ...t,
        messages: t.messages.map(m => m.streaming ? { ...m, streaming: false } : m),
      }));
    }
  }, [activeId, updateThread]);

  // Fork from a specific message in the active thread
  const handleForkFromMessage = useCallback((msgId: string) => {
    if (!activeId) return;
    handleForkThread(activeId, msgId);
  }, [activeId, handleForkThread]);

  // Edit a user message: remove from that point forward and re-send
  const handleEditMessage = useCallback((msgId: string, newContent: string) => {
    if (!activeId) return;
    const thread = threads.find(t => t.id === activeId);
    if (!thread) return;
    const idx = thread.messages.findIndex(m => m.id === msgId);
    if (idx < 0) return;
    // Truncate messages from this index forward
    updateThread(activeId, t => ({
      ...t,
      messages: t.messages.slice(0, idx),
      updatedAt: new Date().toISOString(),
    }));
    // Re-send
    handleSend(newContent, activeId);
  }, [activeId, threads, updateThread, handleSend]);

  // Retry: find the user message before an assistant message and re-send
  const handleRetry = useCallback((aiMsgId: string) => {
    if (!activeId) return;
    const thread = threads.find(t => t.id === activeId);
    if (!thread) return;
    const idx = thread.messages.findIndex(m => m.id === aiMsgId);
    if (idx < 1) return;
    const userMsg = thread.messages[idx - 1];
    if (userMsg?.role !== 'user') return;
    // Remove from the AI message forward
    updateThread(activeId, t => ({
      ...t,
      messages: t.messages.slice(0, idx),
      updatedAt: new Date().toISOString(),
    }));
    handleSend(userMsg.content, activeId);
  }, [activeId, threads, updateThread, handleSend]);

  // ── Model selector node (passed to ChatInput) ────────────────────────────

  const modelSelector = (
    <ModelSelector
      model={activeThread?.model ?? model}
      onChange={m => {
        setModel(m);
        if (activeId) updateThread(activeId, t => ({ ...t, model: m }));
      }}
    />
  );

  // Hydrate messages for display (ensure Date objects)
  const displayMessages = activeThread
    ? hydrateMessages(activeThread.messages)
    : [];

  // ── Render ───────────────────────────────────────────────────────────────

  return (
    <>
      {confirmDeleteId && (
        <ConfirmDialog
          message="Permanently delete this thread and all its messages?"
          onConfirm={confirmDelete}
          onCancel={() => setConfirmDeleteId(null)}
        />
      )}

      <div className="flex flex-1 overflow-hidden">
        {/* Sidebar */}
        {showSidebar && (
          <ThreadSidebar
            threads={threads}
            activeId={activeId}
            onNewChat={handleNewChat}
            onSelect={handleSelectThread}
            onRename={handleRenameThread}
            onFork={id => handleForkThread(id)}
            onExportJSON={handleExportJSON}
            onExportMD={handleExportMD}
            onTogglePin={handleTogglePin}
            onArchive={handleArchive}
            onDelete={handleDeleteThread}
          />
        )}

        {/* Main chat area */}
        <div className="flex flex-col flex-1 overflow-hidden bg-zinc-950">
          {/* Header */}
          <ChatHeader
            thread={activeThread}
            showSidebar={showSidebar}
            onToggleSidebar={() => setShowSidebar(s => !s)}
            onRenameThread={title => activeId && handleRenameThread(activeId, title)}
            onForkThread={() => activeId && handleForkThread(activeId)}
            onExportJSON={() => activeId && handleExportJSON(activeId)}
            onExportMD={() => activeId && handleExportMD(activeId)}
            onDeleteThread={() => activeId && handleDeleteThread(activeId)}
          />

          {/* Messages or welcome */}
          {displayMessages.filter(m => m.role !== 'system').length === 0 ? (
            <WelcomeScreen onSend={handleSend} />
          ) : (
            <div
              ref={scrollRef}
              className="flex-1 overflow-y-auto px-6 py-5 scroll-smooth"
              style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
            >
              {displayMessages.map(msg => (
                <div key={msg.id} className="group">
                  <ChatMessage
                    message={msg}
                    onRetry={msg.role === 'assistant' ? () => handleRetry(msg.id) : undefined}
                    onFork={() => handleForkFromMessage(msg.id)}
                  />
                  {!msg.streaming && msg.role !== 'system' && (
                    <MessageActions
                      message={msg}
                      allMessages={displayMessages}
                      onRetry={msg.role === 'assistant' ? () => handleRetry(msg.id) : undefined}
                      onForkHere={() => handleForkFromMessage(msg.id)}
                      onDelete={() => activeId && deleteMessage(activeId, msg.id)}
                      onEdit={msg.role === 'user' ? (newContent) => handleEditMessage(msg.id, newContent) : undefined}
                    />
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Input */}
          <ChatInput
            onSend={text => handleSend(text)}
            onCancel={handleCancel}
            disabled={streaming}
            streaming={streaming}
            modelSelector={modelSelector}
          />
        </div>
      </div>
    </>
  );
}
