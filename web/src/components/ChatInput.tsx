import { useState, useRef, useEffect, type KeyboardEvent } from 'react';
import { Send, Paperclip, Square, Smile } from 'lucide-react';

const EMOJI_GROUPS = [
  { label: '😀 Smileys', emojis: ['😀', '😂', '🤣', '😊', '😍', '🥰', '😎', '🤔', '😅', '🙃', '😏', '🤗', '😤', '😭', '🥺', '😱', '🤯', '🫡', '🫠', '😴'] },
  { label: '👍 Gestures', emojis: ['👍', '👎', '👏', '🙏', '🤝', '✌️', '🤞', '💪', '👋', '🫶', '🖐️', '✋', '🤙', '👊', '🫰'] },
  { label: '❤️ Hearts', emojis: ['❤️', '🧡', '💛', '💚', '💙', '💜', '🖤', '💔', '❤️‍🔥', '💕', '💖', '💗', '💝'] },
  { label: '🎉 Celebrate', emojis: ['🎉', '🎊', '🥳', '🏆', '⭐', '🌟', '✨', '💫', '🔥', '💯', '🚀', '💎', '🎯', '🏅', '👑'] },
  { label: '💻 Tech', emojis: ['💻', '🖥️', '⌨️', '🖱️', '📱', '🤖', '⚡', '🔧', '🛠️', '⚙️', '🧪', '🔬', '📡', '🌐', '🔒'] },
  { label: '📝 Objects', emojis: ['📝', '📎', '📌', '📍', '✅', '❌', '⚠️', '💡', '📊', '📈', '📉', '🗂️', '📁', '🔔', '💬'] },
];

interface Props {
  onSend: (message: string) => void;
  onCancel?: () => void;
  disabled?: boolean;
  streaming?: boolean;
  modelSelector?: React.ReactNode;
}

export default function ChatInput({ onSend, onCancel, disabled, streaming, modelSelector }: Props) {
  const [value, setValue] = useState('');
  const [showEmoji, setShowEmoji] = useState(false);
  const ref = useRef<HTMLTextAreaElement>(null);
  const emojiRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = 'auto';
      ref.current.style.height = Math.min(ref.current.scrollHeight, 200) + 'px';
    }
  }, [value]);

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

  const handleKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
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
    if (!text || disabled) return;
    onSend(text);
    setValue('');
    setShowEmoji(false);
  };

  const insertEmoji = (emoji: string) => {
    const textarea = ref.current;
    if (textarea) {
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const newValue = value.slice(0, start) + emoji + value.slice(end);
      setValue(newValue);
      // Restore cursor position after the emoji
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = start + emoji.length;
        textarea.focus();
      }, 0);
    } else {
      setValue(v => v + emoji);
    }
  };

  return (
    <div className="flex flex-col gap-2 px-6 py-4 bg-zinc-900 border-t border-zinc-800">
      {modelSelector && (
        <div className="flex items-center gap-2">
          {modelSelector}
        </div>
      )}
      <div className="flex items-end gap-2">
        <button
          className="flex-shrink-0 flex items-center justify-center w-9 h-9 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-all disabled:opacity-30"
          title="Attach file"
          disabled={disabled}
        >
          <Paperclip size={17} />
        </button>

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
          onChange={e => setValue(e.target.value)}
          onKeyDown={handleKey}
          placeholder={streaming ? 'AI is responding… (Esc to cancel)' : 'Send a message…'}
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
            disabled={disabled || !value.trim()}
            title="Send (Enter)"
            className="flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white transition-all disabled:opacity-30 disabled:cursor-not-allowed"
          >
            <Send size={15} />
          </button>
        )}
      </div>
      <p className="text-xs text-zinc-600 text-center">
        Enter to send · Shift+Enter for new line · Esc to cancel
      </p>
    </div>
  );
}
