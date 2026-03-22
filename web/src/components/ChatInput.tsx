import { useState, useRef, useEffect, useCallback, type KeyboardEvent, type DragEvent } from 'react';
import { Send, Paperclip, Square, Smile, X } from 'lucide-react';

// ── Emoji groups ───────────────────────────────────────────────────────────────

const EMOJI_GROUPS = [
  { label: '😀 Smileys', emojis: ['😀', '😂', '🤣', '😊', '😍', '🥰', '😎', '🤔', '😅', '🙃', '😏', '🤗', '😤', '😭', '🥺', '😱', '🤯', '🫡', '🫠', '😴'] },
  { label: '👍 Gestures', emojis: ['👍', '👎', '👏', '🙏', '🤝', '✌️', '🤞', '💪', '👋', '🫶', '🖐️', '✋', '🤙', '👊', '🫰'] },
  { label: '❤️ Hearts', emojis: ['❤️', '🧡', '💛', '💚', '💙', '💜', '🖤', '💔', '❤️‍🔥', '💕', '💖', '💗', '💝'] },
  { label: '🎉 Celebrate', emojis: ['🎉', '🎊', '🥳', '🏆', '⭐', '🌟', '✨', '💫', '🔥', '💯', '🚀', '💎', '🎯', '🏅', '👑'] },
  { label: '💻 Tech', emojis: ['💻', '🖥️', '⌨️', '🖱️', '📱', '🤖', '⚡', '🔧', '🛠️', '⚙️', '🧪', '🔬', '📡', '🌐', '🔒'] },
  { label: '📝 Objects', emojis: ['📝', '📎', '📌', '📍', '✅', '❌', '⚠️', '💡', '📊', '📈', '📉', '🗂️', '📁', '🔔', '💬'] },
];

// ── Slash commands ─────────────────────────────────────────────────────────────

export interface SlashCommand {
  command: string;
  description: string;
  action: string;
}

export const SLASH_COMMANDS: SlashCommand[] = [
  { command: '/clear',     description: 'Clear chat history',        action: 'clear' },
  { command: '/new',       description: 'New conversation',          action: 'new' },
  { command: '/status',    description: 'Show system status',        action: 'status' },
  { command: '/tools',     description: 'List available tools',      action: 'tools' },
  { command: '/model',     description: 'Switch AI model',           action: 'model' },
  { command: '/export',    description: 'Export conversation',       action: 'export' },
  { command: '/heartbeat', description: 'Heartbeat status',          action: 'heartbeat' },
  { command: '/help',      description: 'Show all commands',         action: 'help' },
  { command: '/usage',     description: 'Token usage stats',         action: 'usage' },
  { command: '/doctor',    description: 'Run diagnostics',           action: 'doctor' },
  { command: '/agents',    description: 'List agents',               action: 'agents' },
  { command: '/memory',    description: 'Search memory',             action: 'memory' },
  { command: '/fork',      description: 'Fork conversation',         action: 'fork' },
  { command: '/theme',     description: 'Toggle dark/light theme',   action: 'theme' },
];

// ── Attached file ─────────────────────────────────────────────────────────────

interface AttachedFile {
  name: string;
  size: number;
  type: string;
  content: string;   // text content or base64 data URL for images
  isImage: boolean;
  isPdf: boolean;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

const MAX_TEXT_BYTES = 100 * 1024; // 100 KB

const TEXT_EXTENSIONS = new Set([
  'txt', 'md', 'js', 'ts', 'tsx', 'jsx', 'py', 'go', 'json', 'yml', 'yaml',
  'csv', 'html', 'css', 'sh', 'sql', 'rs', 'rb', 'java', 'c', 'cpp', 'h',
  'xml', 'toml', 'env', 'log',
]);

function isTextFile(name: string, mimeType: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() ?? '';
  return TEXT_EXTENSIONS.has(ext) || mimeType.startsWith('text/');
}

async function readAttachedFile(file: File): Promise<AttachedFile | string> {
  const isImage = file.type.startsWith('image/');
  const isPdf = file.type === 'application/pdf' || file.name.endsWith('.pdf');

  if (isImage) {
    return new Promise<AttachedFile>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        resolve({
          name: file.name,
          size: file.size,
          type: file.type,
          content: reader.result as string,
          isImage: true,
          isPdf: false,
        });
      };
      reader.onerror = () => reject(new Error('Failed to read image'));
      reader.readAsDataURL(file);
    });
  }

  if (isPdf) {
    return {
      name: file.name,
      size: file.size,
      type: file.type,
      content: '',
      isImage: false,
      isPdf: true,
    };
  }

  if (!isTextFile(file.name, file.type)) {
    return `Unsupported file type: ${file.name}`;
  }

  if (file.size > MAX_TEXT_BYTES) {
    return `File too large (max 100KB for text): ${file.name} is ${formatBytes(file.size)}`;
  }

  return new Promise<AttachedFile>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      resolve({
        name: file.name,
        size: file.size,
        type: file.type,
        content: reader.result as string,
        isImage: false,
        isPdf: false,
      });
    };
    reader.onerror = () => reject(new Error('Failed to read file'));
    reader.readAsText(file);
  });
}

// ── Props ──────────────────────────────────────────────────────────────────────

interface Props {
  onSend: (message: string) => void;
  onCancel?: () => void;
  onCommand?: (command: string) => void;
  disabled?: boolean;
  streaming?: boolean;
  modelSelector?: React.ReactNode;
}

// ── Component ──────────────────────────────────────────────────────────────────

export default function ChatInput({ onSend, onCancel, onCommand, disabled, streaming, modelSelector }: Props) {
  const [value, setValue] = useState('');
  const [showEmoji, setShowEmoji] = useState(false);
  const [slashOpen, setSlashOpen] = useState(false);
  const [slashFilter, setSlashFilter] = useState('');
  const [slashIndex, setSlashIndex] = useState(0);
  const [attachedFile, setAttachedFile] = useState<AttachedFile | null>(null);
  const [dragOver, setDragOver] = useState(false);

  const ref = useRef<HTMLTextAreaElement>(null);
  const emojiRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const slashRef = useRef<HTMLDivElement>(null);

  // Auto-resize textarea
  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = 'auto';
      ref.current.style.height = Math.min(ref.current.scrollHeight, 200) + 'px';
    }
  }, [value]);

  // Focus textarea when enabled
  useEffect(() => {
    if (!disabled && ref.current) ref.current.focus();
  }, [disabled]);

  // Close emoji picker on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (emojiRef.current && !emojiRef.current.contains(e.target as Node)) {
        setShowEmoji(false);
      }
    };
    if (showEmoji) document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showEmoji]);

  // Close slash dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (slashRef.current && !slashRef.current.contains(e.target as Node)) {
        setSlashOpen(false);
      }
    };
    if (slashOpen) document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [slashOpen]);

  // Filtered slash command list
  const filteredCommands = slashFilter
    ? SLASH_COMMANDS.filter(c =>
        c.command.startsWith('/' + slashFilter) ||
        c.description.toLowerCase().includes(slashFilter.toLowerCase())
      )
    : SLASH_COMMANDS;

  // Handle value changes: detect slash command trigger
  const handleChange = (newValue: string) => {
    setValue(newValue);

    if (newValue.startsWith('/') && !newValue.includes(' ')) {
      const filter = newValue.slice(1);
      setSlashFilter(filter);
      setSlashIndex(0);
      setSlashOpen(true);
    } else {
      setSlashOpen(false);
    }
  };

  const executeSlashCommand = useCallback((cmd: SlashCommand) => {
    setSlashOpen(false);
    setValue('');
    if (onCommand) onCommand(cmd.action);
  }, [onCommand]);

  const handleKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    // Handle slash dropdown navigation
    if (slashOpen && filteredCommands.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSlashIndex(i => (i + 1) % filteredCommands.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSlashIndex(i => (i - 1 + filteredCommands.length) % filteredCommands.length);
        return;
      }
      if (e.key === 'Enter') {
        e.preventDefault();
        executeSlashCommand(filteredCommands[slashIndex]);
        return;
      }
      if (e.key === 'Escape') {
        setSlashOpen(false);
        return;
      }
      if (e.key === 'Tab') {
        e.preventDefault();
        if (filteredCommands.length > 0) {
          executeSlashCommand(filteredCommands[slashIndex]);
        }
        return;
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
    if (e.key === 'Escape' && streaming && onCancel) {
      onCancel();
    }
  };

  const submit = () => {
    const text = value.trim();
    if (!text && !attachedFile) return;
    if (disabled) return;

    // If the entire input is a complete slash command with no extra text, execute it
    if (text && text.startsWith('/') && !text.includes(' ')) {
      const match = SLASH_COMMANDS.find(c => c.command === text);
      if (match) {
        executeSlashCommand(match);
        return;
      }
    }

    let finalText = text;

    if (attachedFile) {
      if (attachedFile.isImage) {
        const prefix = `[Image: ${attachedFile.name} (${formatBytes(attachedFile.size)})]\n[Base64 image data attached — describe what you see in this image]\n\n`;
        finalText = prefix + (text || 'What is in this image?');
      } else if (attachedFile.isPdf) {
        const prefix = `[PDF: ${attachedFile.name} (${formatBytes(attachedFile.size)})]\n\n`;
        finalText = prefix + (text || `Please read and summarize: ${attachedFile.name}`);
      } else {
        const ext = attachedFile.name.split('.').pop() ?? '';
        const lang = ext || '';
        const prefix = `[File: ${attachedFile.name} (${formatBytes(attachedFile.size)})]\n\`\`\`${lang}\n${attachedFile.content}\n\`\`\`\n\n`;
        finalText = prefix + (text || `I've attached the file ${attachedFile.name}.`);
      }
      setAttachedFile(null);
    }

    if (!finalText.trim()) return;
    onSend(finalText);
    setValue('');
    setShowEmoji(false);
    setSlashOpen(false);
  };

  const insertEmoji = (emoji: string) => {
    const textarea = ref.current;
    if (textarea) {
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const newValue = value.slice(0, start) + emoji + value.slice(end);
      setValue(newValue);
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = start + emoji.length;
        textarea.focus();
      }, 0);
    } else {
      setValue(v => v + emoji);
    }
  };

  const processFile = async (file: File) => {
    const result = await readAttachedFile(file);
    if (typeof result === 'string') {
      // Error message
      alert(result);
      return;
    }
    setAttachedFile(result);
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) processFile(file);
    // Reset input so same file can be re-selected
    e.target.value = '';
  };

  const handleDragOver = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragOver(true);
  };

  const handleDragLeave = () => {
    setDragOver(false);
  };

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files?.[0];
    if (file) processFile(file);
  };

  return (
    <div
      className={`flex flex-col gap-2 px-6 py-4 bg-zinc-900 border-t transition-colors ${
        dragOver ? 'border-indigo-500/60 bg-indigo-500/5' : 'border-zinc-800'
      }`}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {modelSelector && (
        <div className="flex items-center gap-2">
          {modelSelector}
        </div>
      )}

      {/* Attached file preview */}
      {attachedFile && (
        <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700/60">
          {attachedFile.isImage ? (
            <img
              src={attachedFile.content}
              alt={attachedFile.name}
              className="w-8 h-8 rounded object-cover flex-shrink-0 border border-zinc-700"
            />
          ) : (
            <span className="text-base flex-shrink-0">
              {attachedFile.isPdf ? '📄' : '📎'}
            </span>
          )}
          <span className="text-xs text-zinc-300 truncate flex-1">
            {attachedFile.name}
            <span className="text-zinc-500 ml-1.5">({formatBytes(attachedFile.size)})</span>
          </span>
          <button
            onClick={() => setAttachedFile(null)}
            className="flex-shrink-0 flex items-center justify-center w-5 h-5 rounded text-zinc-500 hover:text-zinc-300 hover:bg-zinc-700 transition-all"
            title="Remove attachment"
          >
            <X size={12} />
          </button>
        </div>
      )}

      {/* Drag-over hint */}
      {dragOver && (
        <div className="flex items-center justify-center py-2 text-xs text-indigo-400 gap-1.5">
          <Paperclip size={12} />
          Drop file to attach
        </div>
      )}

      {/* Slash command dropdown — appears ABOVE input */}
      {slashOpen && filteredCommands.length > 0 && (
        <div
          ref={slashRef}
          className="rounded-xl border border-zinc-700 bg-zinc-900 shadow-2xl shadow-black/40 overflow-hidden"
        >
          <div className="px-3 pt-2 pb-1 text-[10px] text-zinc-600 uppercase tracking-wider font-medium border-b border-zinc-800">
            Commands
          </div>
          <div className="max-h-48 overflow-y-auto" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
            {filteredCommands.map((cmd, i) => (
              <button
                key={cmd.command}
                onMouseDown={e => {
                  e.preventDefault(); // Prevent textarea blur
                  executeSlashCommand(cmd);
                }}
                className={`flex items-center gap-3 w-full px-3 py-2 text-left text-xs transition-colors ${
                  i === slashIndex
                    ? 'bg-indigo-500/15 text-zinc-100'
                    : 'text-zinc-300 hover:bg-zinc-800'
                }`}
              >
                <span className="font-mono text-indigo-400 w-24 flex-shrink-0">{cmd.command}</span>
                <span className="text-zinc-500 truncate">{cmd.description}</span>
              </button>
            ))}
          </div>
          <div className="px-3 py-1.5 border-t border-zinc-800 text-[10px] text-zinc-700">
            Arrow keys to navigate, Enter or Tab to select, Esc to dismiss
          </div>
        </div>
      )}

      <div className="flex items-end gap-2">
        {/* Paperclip / file attach */}
        <button
          className="flex-shrink-0 flex items-center justify-center w-9 h-9 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-all disabled:opacity-30"
          title="Attach file"
          disabled={disabled}
          onClick={() => fileInputRef.current?.click()}
        >
          <Paperclip size={17} />
        </button>

        {/* Hidden file input */}
        <input
          ref={fileInputRef}
          type="file"
          className="hidden"
          accept=".txt,.md,.js,.ts,.tsx,.jsx,.py,.go,.json,.yml,.yaml,.csv,.html,.css,.sh,.sql,.rs,.rb,.java,.c,.cpp,.h,.xml,.toml,.env,.log,.pdf,.png,.jpg,.jpeg,.gif,.svg,.webp"
          onChange={handleFileSelect}
        />

        {/* Emoji picker */}
        <div className="relative" ref={emojiRef}>
          <button
            onClick={() => setShowEmoji(s => !s)}
            className={`flex-shrink-0 flex items-center justify-center w-9 h-9 rounded-lg transition-all disabled:opacity-30 ${
              showEmoji
                ? 'text-amber-400 bg-amber-500/10'
                : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800'
            }`}
            title="Emoji"
            disabled={disabled}
          >
            <Smile size={17} />
          </button>

          {showEmoji && (
            <div className="absolute bottom-12 left-0 z-50 w-80 max-h-72 bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl shadow-black/40 overflow-hidden">
              <div className="overflow-y-auto max-h-72 p-2" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
                {EMOJI_GROUPS.map(group => (
                  <div key={group.label} className="mb-2">
                    <div className="text-[10px] text-zinc-600 uppercase tracking-wider font-medium px-1 py-1">
                      {group.label}
                    </div>
                    <div className="flex flex-wrap gap-0.5">
                      {group.emojis.map(emoji => (
                        <button
                          key={emoji}
                          onClick={() => insertEmoji(emoji)}
                          className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-zinc-700/60 transition-colors text-base"
                          title={emoji}
                        >
                          {emoji}
                        </button>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <textarea
          ref={ref}
          value={value}
          onChange={e => handleChange(e.target.value)}
          onKeyDown={handleKey}
          placeholder={
            dragOver
              ? 'Drop file here…'
              : streaming
              ? 'AI is responding… (Esc to cancel)'
              : 'Send a message… (type / for commands)'
          }
          rows={1}
          disabled={disabled}
          className="flex-1 resize-none px-3.5 py-2.5 rounded-xl border border-zinc-700 bg-zinc-800/60 text-zinc-100 text-sm leading-relaxed placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 transition-colors max-h-48 disabled:opacity-50"
        />

        {streaming ? (
          <button
            onClick={onCancel}
            title="Cancel"
            className="flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-xl bg-red-600 hover:bg-red-500 text-white transition-all"
          >
            <Square size={15} />
          </button>
        ) : (
          <button
            onClick={submit}
            disabled={disabled || (!value.trim() && !attachedFile)}
            title="Send (Enter)"
            className="flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white transition-all disabled:opacity-30 disabled:cursor-not-allowed"
          >
            <Send size={15} />
          </button>
        )}
      </div>

      <p className="text-xs text-zinc-600 text-center">
        Enter to send · Shift+Enter for new line · / for commands · drag &amp; drop to attach
      </p>
    </div>
  );
}
