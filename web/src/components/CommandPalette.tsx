import { useState, useEffect, useRef } from 'react';
import { Search, MessageSquare, Trash2, Download, Cpu } from 'lucide-react';

interface Props {
  open: boolean;
  onClose: () => void;
  onCommand: (cmd: string) => void;
}

const commands = [
  { id: 'new', label: 'New Conversation', icon: MessageSquare, shortcut: 'Ctrl+N' },
  { id: 'clear', label: 'Clear Chat', icon: Trash2, shortcut: 'Ctrl+L' },
  { id: 'export', label: 'Export Conversation', icon: Download },
  { id: 'doctor', label: 'Run Diagnostics', icon: Cpu },
];

export default function CommandPalette({ open, onClose, onCommand }: Props) {
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setQuery('');
      setSelected(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  if (!open) return null;

  const filtered = commands.filter(c =>
    c.label.toLowerCase().includes(query.toLowerCase()) ||
    c.id.includes(query.toLowerCase())
  );

  const handleKey = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') onClose();
    if (e.key === 'ArrowDown') setSelected(s => Math.min(s + 1, filtered.length - 1));
    if (e.key === 'ArrowUp') setSelected(s => Math.max(s - 1, 0));
    if (e.key === 'Enter' && filtered[selected]) {
      onCommand(filtered[selected].id);
      onClose();
    }
  };

  return (
    <div className="palette-overlay" onClick={onClose}>
      <div className="palette" onClick={e => e.stopPropagation()}>
        <div className="palette-input-row">
          <Search size={16} className="palette-icon" />
          <input
            ref={inputRef}
            className="palette-input"
            value={query}
            onChange={e => { setQuery(e.target.value); setSelected(0); }}
            onKeyDown={handleKey}
            placeholder="Type a command..."
          />
        </div>
        <div className="palette-list">
          {filtered.map((cmd, i) => (
            <button
              key={cmd.id}
              className={`palette-item ${i === selected ? 'selected' : ''}`}
              onClick={() => { onCommand(cmd.id); onClose(); }}
              onMouseEnter={() => setSelected(i)}
            >
              <cmd.icon size={16} />
              <span className="palette-label">{cmd.label}</span>
              {cmd.shortcut && <span className="palette-shortcut">{cmd.shortcut}</span>}
            </button>
          ))}
          {filtered.length === 0 && <div className="palette-empty">No commands found</div>}
        </div>
      </div>
    </div>
  );
}
