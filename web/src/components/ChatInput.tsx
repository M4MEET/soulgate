import { useState, useRef, useEffect, type KeyboardEvent } from 'react';
import { Send, Paperclip, Square } from 'lucide-react';

interface Props {
  onSend: (message: string) => void;
  onCancel?: () => void;
  disabled?: boolean;
  streaming?: boolean;
}

export default function ChatInput({ onSend, onCancel, disabled, streaming }: Props) {
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
    <div className="chat-input-container">
      <button className="input-action-btn" title="Attach file" disabled={disabled}>
        <Paperclip size={18} />
      </button>
      <textarea
        ref={ref}
        className="chat-textarea"
        value={value}
        onChange={e => setValue(e.target.value)}
        onKeyDown={handleKey}
        placeholder={streaming ? 'AI is responding... (Esc to cancel)' : 'Send a message...'}
        rows={1}
        disabled={disabled}
      />
      {streaming ? (
        <button className="send-btn cancel-btn" onClick={onCancel} title="Cancel">
          <Square size={16} />
        </button>
      ) : (
        <button className="send-btn" onClick={submit} disabled={disabled || !value.trim()} title="Send (Enter)">
          <Send size={16} />
        </button>
      )}
    </div>
  );
}
