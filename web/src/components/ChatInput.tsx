import { useState, useRef, useEffect, type KeyboardEvent } from 'react';
import { Send, Paperclip, Square } from 'lucide-react';

interface Props {
  onSend: (message: string) => void;
  onCancel?: () => void;
  disabled?: boolean;
  streaming?: boolean;
  modelSelector?: React.ReactNode;
}

export default function ChatInput({ onSend, onCancel, disabled, streaming, modelSelector }: Props) {
  const [value, setValue] = useState('');
  const ref = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = 'auto';
      ref.current.style.height = Math.min(ref.current.scrollHeight, 200) + 'px';
    }
  }, [value]);

  useEffect(() => {
    if (!disabled && ref.current) ref.current.focus();
  }, [disabled]);

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
