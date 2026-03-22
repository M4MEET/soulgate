import { useEffect } from 'react';
import { X, Keyboard } from 'lucide-react';

interface ShortcutRow {
  keys: string;
  description: string;
}

interface ShortcutSection {
  title: string;
  rows: ShortcutRow[];
}

const SECTIONS: ShortcutSection[] = [
  {
    title: 'Navigation',
    rows: [
      { keys: 'Ctrl+K',   description: 'Open command palette' },
      { keys: 'Ctrl+1',   description: 'Go to Chat' },
      { keys: 'Ctrl+2',   description: 'Go to Dashboard' },
      { keys: 'Ctrl+3',   description: 'Go to Settings' },
      { keys: 'Ctrl+G',   description: 'Go to Agents' },
      { keys: '?',        description: 'Show this help' },
    ],
  },
  {
    title: 'Chat',
    rows: [
      { keys: 'Enter',         description: 'Send message' },
      { keys: 'Shift+Enter',   description: 'New line in message' },
      { keys: 'Esc',           description: 'Cancel streaming' },
      { keys: 'Ctrl+N',        description: 'New conversation' },
      { keys: 'Ctrl+L',        description: 'Clear chat' },
    ],
  },
  {
    title: 'Global',
    rows: [
      { keys: 'Esc',     description: 'Close any modal / overlay' },
    ],
  },
];

interface Props {
  open: boolean;
  onClose: () => void;
}

function Keys({ combo }: { combo: string }) {
  return (
    <span className="flex items-center gap-1">
      {combo.split('+').map((part, i) => (
        <span key={i} className="flex items-center gap-1">
          {i > 0 && <span className="text-zinc-700 text-xs">+</span>}
          <kbd className="inline-block min-w-[26px] px-1.5 py-0.5 text-center bg-zinc-800 border border-zinc-700 rounded text-xs font-mono text-zinc-300">
            {part.trim()}
          </kbd>
        </span>
      ))}
    </span>
  );
}

export default function ShortcutsModal({ open, onClose }: Props) {
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-2xl bg-zinc-900 border border-zinc-700 rounded-2xl shadow-2xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800">
          <div className="flex items-center gap-2.5">
            <Keyboard size={16} className="text-indigo-400" />
            <span className="font-semibold text-sm text-zinc-200">Keyboard Shortcuts</span>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg hover:bg-zinc-800 text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            <X size={15} />
          </button>
        </div>

        {/* Shortcut grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-px bg-zinc-800 max-h-[70vh] overflow-y-auto">
          {SECTIONS.map(section => (
            <div key={section.title} className="bg-zinc-900 p-5">
              <h3 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-3">
                {section.title}
              </h3>
              <div className="flex flex-col gap-2.5">
                {section.rows.map(row => (
                  <div key={row.keys} className="flex items-center justify-between gap-4 min-h-[26px]">
                    <span className="text-xs text-zinc-400">{row.description}</span>
                    <Keys combo={row.keys} />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-zinc-800 flex items-center justify-between">
          <span className="text-xs text-zinc-600">Press <kbd className="font-mono bg-zinc-800 border border-zinc-700 px-1 py-0.5 rounded text-zinc-400">?</kbd> anywhere to toggle</span>
          <button
            onClick={onClose}
            className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
