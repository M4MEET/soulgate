import { useState, useRef, useEffect, useCallback, type KeyboardEvent } from 'react';
import { Terminal, Trash2, Copy } from 'lucide-react';
import { execCommand } from '../lib/api';
import toast from 'react-hot-toast';

// ── ANSI stripping ─────────────────────────────────────────────────────────
// Remove ANSI escape sequences so terminal output renders cleanly as plain text.
const ANSI_RE = /\x1B\[[0-9;]*[A-Za-z]/g;
function stripAnsi(s: string): string {
  return s.replace(ANSI_RE, '');
}

// ── Types ──────────────────────────────────────────────────────────────────

interface TermLine {
  id: number;
  kind: 'input' | 'output' | 'error' | 'info';
  text: string;
}

let lineId = 0;
const nextId = () => ++lineId;

const WELCOME_LINES: TermLine[] = [
  { id: nextId(), kind: 'info', text: 'SoulGate Terminal — commands run inside the workspace root.' },
  { id: nextId(), kind: 'info', text: 'Type a shell command and press Enter. Policy is enforced on every execution.' },
  { id: nextId(), kind: 'info', text: '' },
];

// ── History management ─────────────────────────────────────────────────────

function useCommandHistory() {
  const [history, setHistory] = useState<string[]>([]);
  const [historyIdx, setHistoryIdx] = useState(-1);

  const push = useCallback((cmd: string) => {
    if (!cmd.trim()) return;
    setHistory(prev => {
      const next = [cmd, ...prev.filter(c => c !== cmd)].slice(0, 200);
      return next;
    });
    setHistoryIdx(-1);
  }, []);

  const up = useCallback(() => {
    setHistoryIdx(prev => {
      return Math.min(prev + 1, history.length - 1);
    });
  }, [history.length]);

  const down = useCallback(() => {
    setHistoryIdx(prev => Math.max(prev - 1, -1));
  }, []);

  const current = historyIdx >= 0 && historyIdx < history.length
    ? history[historyIdx]
    : null;

  return { push, up, down, current, historyIdx };
}

// ── Terminal line renderer ─────────────────────────────────────────────────

function TermLineView({ line }: { line: TermLine }) {
  const colors: Record<TermLine['kind'], string> = {
    input:  'text-emerald-400',
    output: 'text-zinc-200',
    error:  'text-red-400',
    info:   'text-zinc-500',
  };

  const prefix: Record<TermLine['kind'], string> = {
    input:  '$ ',
    output: '  ',
    error:  '  ',
    info:   '  ',
  };

  return (
    <div className={`font-mono text-xs leading-5 whitespace-pre-wrap break-words ${colors[line.kind]}`}>
      <span className="select-none opacity-60">{prefix[line.kind]}</span>
      {line.text}
    </div>
  );
}

// ── Main component ─────────────────────────────────────────────────────────

export default function TerminalView() {
  const [lines, setLines] = useState<TermLine[]>(WELCOME_LINES);
  const [input, setInput] = useState('');
  const [running, setRunning] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const { push: pushHistory, up: histUp, down: histDown, current: histCurrent } = useCommandHistory();

  // Auto-scroll to bottom on new lines
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [lines]);

  // Keep input in sync with history navigation
  useEffect(() => {
    if (histCurrent !== null) setInput(histCurrent);
  }, [histCurrent]);

  const appendLines = useCallback((newLines: TermLine[]) => {
    setLines(prev => [...prev, ...newLines]);
  }, []);

  const runCommand = useCallback(async (cmd: string) => {
    const trimmed = cmd.trim();
    if (!trimmed) return;

    pushHistory(trimmed);
    appendLines([{ id: nextId(), kind: 'input', text: trimmed }]);
    setInput('');
    setRunning(true);

    try {
      const res = await execCommand(trimmed);
      const output = stripAnsi(res.output || '');
      const outputLines = output.split('\n');
      // Remove trailing empty line that commands often add
      while (outputLines.length > 0 && outputLines[outputLines.length - 1] === '') {
        outputLines.pop();
      }

      const kind: TermLine['kind'] = res.exit_code !== 0 ? 'error' : 'output';

      if (outputLines.length > 0) {
        appendLines(outputLines.map(t => ({ id: nextId(), kind, text: t })));
      } else if (res.exit_code !== 0) {
        appendLines([{ id: nextId(), kind: 'error', text: `(exit code ${res.exit_code})` }]);
      } else {
        appendLines([{ id: nextId(), kind: 'info', text: '(no output)' }]);
      }

      if (res.error) {
        appendLines([{ id: nextId(), kind: 'error', text: `error: ${res.error}` }]);
      }
    } catch (err) {
      appendLines([{
        id: nextId(),
        kind: 'error',
        text: `failed to run command: ${(err as Error).message}`,
      }]);
    } finally {
      setRunning(false);
      // Re-focus input after execution
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [pushHistory, appendLines]);

  const handleKeyDown = useCallback((e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (!running) runCommand(input);
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      histUp();
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      histDown();
      return;
    }
    if (e.key === 'l' && e.ctrlKey) {
      e.preventDefault();
      setLines(WELCOME_LINES);
      return;
    }
    if (e.key === 'c' && e.ctrlKey && running) {
      // No actual cancellation — just show a message
      appendLines([{ id: nextId(), kind: 'info', text: '^C (cannot cancel server-side execution)' }]);
      return;
    }
  }, [input, running, runCommand, histUp, histDown, appendLines]);

  const handleClear = () => {
    setLines(WELCOME_LINES);
    inputRef.current?.focus();
  };

  const handleCopySession = () => {
    const text = lines
      .filter(l => l.kind !== 'info' || l.text)
      .map(l => {
        if (l.kind === 'input') return `$ ${l.text}`;
        return `  ${l.text}`;
      })
      .join('\n');
    navigator.clipboard.writeText(text).then(() => toast.success('Session copied'));
  };

  return (
    <div
      className="flex flex-col h-full overflow-hidden bg-zinc-950 cursor-text"
      onClick={() => inputRef.current?.focus()}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 h-11 border-b border-zinc-800 bg-zinc-900 flex-shrink-0">
        <div className="flex items-center gap-2 text-zinc-300">
          <Terminal size={15} className="text-emerald-400" />
          <span className="text-sm font-medium">Terminal</span>
          <span className="text-xs text-zinc-600">workspace root</span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={handleCopySession}
            title="Copy session to clipboard"
            className="p-1.5 rounded text-zinc-600 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
          >
            <Copy size={13} />
          </button>
          <button
            onClick={handleClear}
            title="Clear terminal (Ctrl+L)"
            className="p-1.5 rounded text-zinc-600 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
          >
            <Trash2 size={13} />
          </button>
        </div>
      </div>

      {/* Output area */}
      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-0.5">
        {lines.map(line => (
          <TermLineView key={line.id} line={line} />
        ))}

        {/* Running indicator */}
        {running && (
          <div className="flex items-center gap-1.5 text-zinc-500 font-mono text-xs mt-1">
            <span className="animate-pulse">&#9646;</span>
            <span>running…</span>
          </div>
        )}

        <div ref={bottomRef} />
      </div>

      {/* Input bar */}
      <div className="flex items-center gap-2 px-4 py-2 border-t border-zinc-800 bg-zinc-900/60 flex-shrink-0">
        <span className="font-mono text-xs text-emerald-400 select-none flex-shrink-0">$</span>
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={running}
          autoFocus
          spellCheck={false}
          placeholder={running ? 'running…' : 'enter command…'}
          className="flex-1 bg-transparent font-mono text-xs text-zinc-100 placeholder-zinc-700 outline-none disabled:opacity-50 min-w-0"
        />
        {running && (
          <span className="text-xs text-zinc-600 flex-shrink-0">running</span>
        )}
      </div>
    </div>
  );
}
