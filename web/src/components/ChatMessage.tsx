import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Copy, Check, RotateCcw, GitFork, ChevronDown, ChevronRight, Zap } from 'lucide-react';

export interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: Date;
  tokens?: number;
  cost?: number;
  toolCalls?: ToolCall[];
  streaming?: boolean;
  thinkingLog?: string[];
}

export interface ToolCall {
  name: string;
  args: string;
  result: string;
  duration: string;
  status: 'success' | 'error' | 'running';
}

function CopyButton({ text, size = 14 }: { text: string; size?: number }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      onClick={() => {
        navigator.clipboard.writeText(text);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }}
      title="Copy"
      className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-zinc-600/60 text-zinc-400 hover:text-zinc-200 transition-all"
    >
      {copied ? <Check size={size} /> : <Copy size={size} />}
    </button>
  );
}

function ToolCallCard({ tool }: { tool: ToolCall }) {
  const [expanded, setExpanded] = useState(false);

  const statusColors: Record<ToolCall['status'], string> = {
    success: 'text-emerald-400',
    error: 'text-red-400',
    running: 'text-amber-400 animate-pulse',
  };

  return (
    <div className="my-2 rounded-lg border border-zinc-700/60 overflow-hidden bg-zinc-900/60">
      <button
        onClick={() => setExpanded(e => !e)}
        className="flex items-center gap-2 w-full px-3 py-2 bg-zinc-800/60 hover:bg-zinc-800 transition-colors text-left"
      >
        <Zap size={13} className="text-amber-400 flex-shrink-0" />
        <span className="font-mono text-xs font-semibold text-zinc-200 flex-1">{tool.name}</span>
        <span className="text-xs text-zinc-500">{tool.duration}</span>
        <span className={`text-xs ${statusColors[tool.status]}`}>
          {tool.status === 'success' ? '✓' : tool.status === 'error' ? '✗' : '⟳'}
        </span>
        {expanded ? <ChevronDown size={12} className="text-zinc-500" /> : <ChevronRight size={12} className="text-zinc-500" />}
      </button>
      {expanded && (
        <div className="p-3 bg-zinc-950/40 text-xs font-mono">
          {tool.args && (
            <div className="mb-2">
              <div className="text-zinc-500 mb-1">args</div>
              <code className="text-zinc-300 break-all">{tool.args}</code>
            </div>
          )}
          {tool.result && (
            <div className="border-t border-zinc-800 pt-2 mt-2">
              <div className="text-zinc-500 mb-1">result</div>
              <pre className="text-zinc-300 whitespace-pre-wrap max-h-48 overflow-y-auto">
                {tool.result.slice(0, 2000)}{tool.result.length > 2000 ? '\n...' : ''}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default function ChatMessage({
  message,
  onRetry,
  onFork,
}: {
  message: Message;
  onRetry?: () => void;
  onFork?: () => void;
}) {
  const isUser = message.role === 'user';
  const isSystem = message.role === 'system';

  if (isSystem) {
    return (
      <div className="text-center text-xs text-zinc-500 py-2 px-4">{message.content}</div>
    );
  }

  return (
    <div className={`flex gap-3 mb-5 group ${isUser ? 'flex-row-reverse' : ''}`}>
      {/* Avatar */}
      <div className={`w-7 h-7 rounded-full flex items-center justify-center text-sm flex-shrink-0 mt-0.5 ${
        isUser ? 'bg-blue-600 text-white' : 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30'
      }`}>
        {isUser ? '👤' : '✦'}
      </div>

      {/* Content */}
      <div className={`min-w-0 max-w-3xl ${isUser ? 'items-end' : 'items-start'} flex flex-col`}>
        {/* Header */}
        <div className={`flex items-center gap-2 mb-1 ${isUser ? 'flex-row-reverse' : ''}`}>
          <span className={`text-xs font-semibold ${isUser ? 'text-blue-400' : 'text-emerald-400'}`}>
            {isUser ? 'You' : 'SoulGate'}
          </span>
          <span className="text-xs text-zinc-600">
            {message.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </span>
        </div>

        {/* Thinking Log */}
        {!isUser && message.thinkingLog && message.thinkingLog.length > 0 && (
          <details className="mb-2 rounded-xl bg-zinc-900/60 border border-zinc-700/40 overflow-hidden" open={message.streaming}>
            <summary className="flex items-center gap-2 px-3 py-2 cursor-pointer text-xs text-zinc-500 hover:text-zinc-300 transition-colors select-none">
              <span className={`w-1.5 h-1.5 rounded-full ${message.streaming ? 'bg-amber-400 animate-pulse' : 'bg-zinc-600'}`} />
              <span>{message.streaming ? 'Thinking...' : `Thought process (${message.thinkingLog.length} steps)`}</span>
            </summary>
            <div className="px-3 py-2 border-t border-zinc-800/60 max-h-48 overflow-y-auto font-mono text-xs space-y-0.5" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
              {message.thinkingLog.map((line, i) => {
                let color = 'text-zinc-500';
                if (line.startsWith('──')) color = 'text-zinc-600';
                else if (line.startsWith('⟶')) color = 'text-violet-400';
                else if (line.startsWith('⟵')) color = 'text-emerald-400';
                else if (line.startsWith('⚡')) color = 'text-amber-400';
                else if (line.startsWith('  ↳')) color = 'text-emerald-400/70';
                else if (line.startsWith('🤖')) color = 'text-zinc-200 italic';
                return <div key={i} className={`${color} ${line.startsWith('🤖') ? 'pl-2 border-l-2 border-indigo-500/30 ml-1' : ''}`}>{line}</div>;
              })}
              {message.streaming && (
                <div className="text-amber-400 animate-pulse">thinking...</div>
              )}
            </div>
          </details>
        )}

        {/* Bubble */}
        <div className={`px-4 py-3 rounded-2xl text-sm leading-relaxed ${
          isUser
            ? 'bg-blue-600 text-white rounded-br-md'
            : 'bg-zinc-800/80 border border-zinc-700/50 text-zinc-100 rounded-bl-md'
        }`}>
          {isUser ? (
            <p className="whitespace-pre-wrap">{message.content}</p>
          ) : (
            <div className="prose prose-sm prose-invert max-w-none">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  code({ className, children, ...props }) {
                    const match = /language-(\w+)/.exec(className || '');
                    const code = String(children).replace(/\n$/, '');
                    if (match) {
                      return (
                        <div className="rounded-lg overflow-hidden my-2 border border-zinc-700/50">
                          <div className="flex items-center justify-between px-3 py-1.5 bg-zinc-800/80">
                            <span className="font-mono text-xs uppercase tracking-widest text-zinc-500">
                              {match[1]}
                            </span>
                            <CopyButton text={code} />
                          </div>
                          <SyntaxHighlighter
                            style={oneDark}
                            language={match[1]}
                            customStyle={{
                              margin: 0,
                              borderRadius: '0 0 8px 8px',
                              fontSize: '12px',
                              background: '#0d0d18',
                              padding: '12px',
                            }}
                          >
                            {code}
                          </SyntaxHighlighter>
                        </div>
                      );
                    }
                    return (
                      <code
                        className="bg-zinc-900/60 text-indigo-300 px-1.5 py-0.5 rounded text-xs font-mono"
                        {...props}
                      >
                        {children}
                      </code>
                    );
                  },
                  a({ href, children }) {
                    return (
                      <a
                        href={href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-indigo-400 hover:underline"
                      >
                        {children}
                      </a>
                    );
                  },
                  blockquote({ children }) {
                    return (
                      <blockquote className="border-l-2 border-indigo-500/50 pl-3 text-zinc-400 italic my-2">
                        {children}
                      </blockquote>
                    );
                  },
                  table({ children }) {
                    return (
                      <div className="overflow-x-auto my-2">
                        <table className="text-xs border-collapse w-full">{children}</table>
                      </div>
                    );
                  },
                  th({ children }) {
                    return <th className="border border-zinc-700 px-2 py-1 bg-zinc-800 text-left">{children}</th>;
                  },
                  td({ children }) {
                    return <td className="border border-zinc-700 px-2 py-1">{children}</td>;
                  },
                }}
              >
                {message.content}
              </ReactMarkdown>
            </div>
          )}
          {message.streaming && !message.content && (
            <div className="flex items-center gap-2 py-1">
              <div className="flex gap-1">
                <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 animate-bounce" style={{ animationDelay: '0ms' }} />
                <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 animate-bounce" style={{ animationDelay: '150ms' }} />
                <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 animate-bounce" style={{ animationDelay: '300ms' }} />
              </div>
              <span className="text-xs text-zinc-500">Thinking...</span>
            </div>
          )}
          {message.streaming && message.content && (
            <span className="inline-block w-0.5 h-4 bg-indigo-400 ml-0.5 animate-pulse" />
          )}
        </div>

        {/* Tool calls */}
        {message.toolCalls?.map((tc, i) => <ToolCallCard key={i} tool={tc} />)}

        {/* Footer */}
        <div className={`flex items-center gap-2 mt-1.5 min-h-5 ${isUser ? 'flex-row-reverse' : ''}`}>
          {message.tokens != null && (
            <span className="text-xs text-zinc-600">{message.tokens} tok</span>
          )}
          {message.cost != null && message.cost > 0 && (
            <span className="text-xs text-zinc-600">${message.cost.toFixed(5)}</span>
          )}
          {!isUser && !message.streaming && (
            <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity ml-auto">
              <CopyButton text={message.content} />
              {onRetry && (
                <button
                  onClick={onRetry}
                  title="Retry"
                  className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-zinc-600/60 text-zinc-400 hover:text-zinc-200 transition-all"
                >
                  <RotateCcw size={12} />
                </button>
              )}
              {onFork && (
                <button
                  onClick={onFork}
                  title="Fork"
                  className="flex items-center justify-center w-6 h-6 rounded bg-zinc-700/50 hover:bg-zinc-600/60 text-zinc-400 hover:text-zinc-200 transition-all"
                >
                  <GitFork size={12} />
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
