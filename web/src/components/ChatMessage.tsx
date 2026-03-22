import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Copy, Check, ChevronDown, ChevronRight, Zap } from 'lucide-react';

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

export default function ChatMessage({ message }: { message: Message }) {
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
        {isUser ? '👤' : '🤖'}
      </div>

      {/* Content */}
      <div className={`min-w-0 max-w-3xl ${isUser ? 'items-end' : 'items-start'} flex flex-col`}>
        {/* Header */}
        <div className={`flex items-center gap-2 mb-1 ${isUser ? 'flex-row-reverse' : ''}`}>
          <span className={`text-xs font-semibold ${isUser ? 'text-blue-400' : 'text-emerald-400'}`}>
            {isUser ? '💬 You' : '🤖 SoulGate'}
          </span>
          <span className="text-xs text-zinc-600">
            {message.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </span>
        </div>

        {/* Thinking Log */}
        {!isUser && message.thinkingLog && message.thinkingLog.length > 0 && (
          <details
            className="mb-2.5 rounded-2xl overflow-hidden transition-all duration-300"
            open={message.streaming}
            style={{
              background: message.streaming
                ? 'linear-gradient(135deg, rgba(99,102,241,0.06) 0%, rgba(139,92,246,0.04) 50%, rgba(59,130,246,0.06) 100%)'
                : 'rgba(24,24,27,0.4)',
              border: `1px solid ${message.streaming ? 'rgba(99,102,241,0.15)' : 'rgba(63,63,70,0.3)'}`,
            }}
          >
            <summary className="flex items-center gap-2.5 px-4 py-2.5 cursor-pointer select-none group transition-colors hover:bg-white/[0.02]">
              {message.streaming ? (
                <span className="relative flex h-2.5 w-2.5">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-50" />
                  <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-indigo-500" />
                </span>
              ) : (
                <span className="w-2.5 h-2.5 rounded-full bg-zinc-600 group-hover:bg-zinc-500 transition-colors" />
              )}
              <span className={`text-xs font-medium transition-colors ${message.streaming ? 'text-indigo-300' : 'text-zinc-500 group-hover:text-zinc-300'}`}>
                {message.streaming ? '🧠 Thinking...' : `🧠 Thought process · ${message.thinkingLog.length} steps`}
              </span>
              {message.streaming && (
                <span className="ml-auto flex gap-0.5">
                  {[0, 1, 2].map(i => (
                    <span
                      key={i}
                      className="w-1 h-1 rounded-full bg-indigo-400"
                      style={{ animation: `pulse 1.4s ease-in-out ${i * 0.2}s infinite` }}
                    />
                  ))}
                </span>
              )}
            </summary>

            <div
              className="border-t border-white/[0.04] max-h-64 overflow-y-auto px-1 py-1.5"
              style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
            >
              {message.thinkingLog.map((line, i) => {
                const isLast = i === message.thinkingLog!.length - 1;
                const isAIText = line.startsWith('🤖');
                const isIteration = line.startsWith('──');
                const isModelCall = line.startsWith('⟶');
                const isModelDone = line.startsWith('⟵');
                const isTool = line.startsWith('⚡');
                const isResult = line.startsWith('  ↳');
                const isStatus = !isAIText && !isIteration && !isModelCall && !isModelDone && !isTool && !isResult;

                if (isIteration) {
                  return (
                    <div key={i} className="flex items-center gap-2 px-3 py-1.5 my-0.5">
                      <span className="flex-1 h-px bg-gradient-to-r from-transparent via-zinc-700/50 to-transparent" />
                      <span className="text-[10px] text-zinc-600 uppercase tracking-widest font-medium">{line.replace(/──/g, '').trim()}</span>
                      <span className="flex-1 h-px bg-gradient-to-r from-transparent via-zinc-700/50 to-transparent" />
                    </div>
                  );
                }

                if (isAIText) {
                  return (
                    <div
                      key={i}
                      className={`mx-2 my-1 px-3 py-2 rounded-xl text-xs leading-relaxed transition-all duration-300 ${
                        isLast && message.streaming
                          ? 'bg-indigo-500/[0.07] border border-indigo-500/10 text-zinc-200'
                          : 'bg-zinc-800/30 border border-zinc-700/20 text-zinc-300'
                      }`}
                    >
                      <div className="flex items-start gap-2">
                        <span className="text-indigo-400 mt-0.5 flex-shrink-0">✦</span>
                        <span className="italic">{line.replace('🤖 ', '')}</span>
                        {isLast && message.streaming && (
                          <span className="inline-block w-0.5 h-3.5 bg-indigo-400 ml-0.5 flex-shrink-0 animate-pulse" />
                        )}
                      </div>
                    </div>
                  );
                }

                let icon = '';
                let color = 'text-zinc-500';
                let bg = '';

                if (isModelCall) { icon = '⟶'; color = 'text-violet-400'; bg = 'bg-violet-500/[0.04]'; }
                else if (isModelDone) { icon = '✓'; color = 'text-emerald-400'; bg = 'bg-emerald-500/[0.04]'; }
                else if (isTool) { icon = '⚡'; color = 'text-amber-400'; bg = 'bg-amber-500/[0.04]'; }
                else if (isResult) { icon = '↳'; color = 'text-emerald-400/60'; }
                else if (isStatus) { icon = '•'; color = 'text-zinc-600'; }

                const text = line.replace(/^[⟶⟵⚡]\s*/, '').replace(/^\s*↳\s*/, '').replace(/^\s*/, '');

                return (
                  <div
                    key={i}
                    className={`flex items-start gap-2 px-3 py-1 mx-1 rounded-lg text-xs font-mono transition-all duration-200 ${bg} ${
                      isLast && message.streaming ? 'opacity-100' : 'opacity-80'
                    }`}
                    style={{ animation: isLast && message.streaming ? 'none' : `fadeSlideIn 0.3s ease ${Math.min(i * 0.05, 0.5)}s both` }}
                  >
                    <span className={`${color} flex-shrink-0 mt-px w-3 text-center`}>{icon}</span>
                    <span className={`${color} break-all`}>{text}</span>
                  </div>
                );
              })}
            </div>
          </details>
        )}

        {/* Bubble */}
        <div className={`px-4 py-3 rounded-2xl text-sm leading-relaxed ${
          isUser
            ? 'bg-gradient-to-br from-blue-600 to-blue-700 text-white rounded-br-md shadow-lg shadow-blue-900/20'
            : 'bg-zinc-800/60 border border-zinc-700/40 text-zinc-100 rounded-bl-md backdrop-blur-sm'
        }`}>
          {isUser ? (
            <p className="whitespace-pre-wrap">{message.content}</p>
          ) : (
            <div className="prose prose-sm prose-invert max-w-none">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  pre({ children }) {
                    return <>{children}</>;
                  },
                  code({ className, children, node, ...props }) {
                    const match = /language-(\w+)/.exec(className || '');
                    const code = String(children).replace(/\n$/, '');
                    const isBlock = (node?.position && node.position.start.line !== node.position.end.line)
                      || code.includes('\n')
                      || (className || '').includes('language-');
                    if (match) {
                      return (
                        <div className="rounded-xl overflow-hidden my-3 border border-zinc-700/30 shadow-lg shadow-black/20">
                          <div className="flex items-center justify-between px-4 py-2 bg-zinc-800/90 border-b border-zinc-700/30">
                            <div className="flex items-center gap-2">
                              <div className="flex gap-1.5">
                                <span className="w-2.5 h-2.5 rounded-full bg-zinc-600/60" />
                                <span className="w-2.5 h-2.5 rounded-full bg-zinc-600/60" />
                                <span className="w-2.5 h-2.5 rounded-full bg-zinc-600/60" />
                              </div>
                              <span className="font-mono text-[10px] uppercase tracking-[0.15em] text-zinc-500 font-medium ml-1">
                                {match[1]}
                              </span>
                            </div>
                            <CopyButton text={code} />
                          </div>
                          <SyntaxHighlighter
                            style={{
                              ...oneDark,
                              'pre[class*="language-"]': {
                                ...oneDark['pre[class*="language-"]'],
                                background: '#0c0c14',
                              },
                              'code[class*="language-"]': {
                                ...oneDark['code[class*="language-"]'],
                                background: 'none',
                              },
                            }}
                            language={match[1]}
                            customStyle={{
                              margin: 0,
                              borderRadius: '0 0 12px 12px',
                              fontSize: '12.5px',
                              lineHeight: '1.6',
                              background: '#0c0c14',
                              padding: '16px',
                            }}
                            codeTagProps={{
                              style: { background: 'none' },
                            }}
                            showLineNumbers={code.split('\n').length > 3}
                            lineNumberStyle={{ color: '#3f3f46', fontSize: '11px', paddingRight: '12px', minWidth: '2em', userSelect: 'none' }}
                            wrapLines
                            lineProps={() => ({ style: { background: 'none', display: 'block' } })}
                          >
                            {code}
                          </SyntaxHighlighter>
                        </div>
                      );
                    }
                    // Block-level code without a language (diagrams, ASCII art, plain blocks)
                    if (isBlock) {
                      const isDiagram = /[→←↑↓│├└┌┐┘┤┬┴┼╭╮╰╯─═║▼▲◆●]/.test(code);
                      return (
                        <div className={`rounded-xl overflow-hidden my-3 border shadow-lg shadow-black/20 ${
                          isDiagram ? 'border-emerald-500/20' : 'border-zinc-700/30'
                        }`}>
                          <div className="flex items-center justify-between px-4 py-2 bg-zinc-800/90 border-b border-zinc-700/30">
                            <div className="flex items-center gap-2">
                              <div className="flex gap-1.5">
                                <span className={`w-2.5 h-2.5 rounded-full ${isDiagram ? 'bg-emerald-500/40' : 'bg-zinc-600/60'}`} />
                                <span className={`w-2.5 h-2.5 rounded-full ${isDiagram ? 'bg-emerald-500/30' : 'bg-zinc-600/60'}`} />
                                <span className={`w-2.5 h-2.5 rounded-full ${isDiagram ? 'bg-emerald-500/20' : 'bg-zinc-600/60'}`} />
                              </div>
                              {isDiagram && (
                                <span className="font-mono text-[10px] uppercase tracking-[0.15em] text-emerald-500/60 font-medium ml-1">
                                  diagram
                                </span>
                              )}
                            </div>
                            <CopyButton text={code} />
                          </div>
                          <pre className={`p-4 overflow-x-auto font-mono text-[12.5px] leading-[1.7] ${
                            isDiagram ? 'bg-[#0a100f] text-cyan-200' : 'bg-[#0c0c14] text-zinc-200'
                          }`}>
                            <code>{code}</code>
                          </pre>
                        </div>
                      );
                    }
                    return (
                      <code
                        className="bg-zinc-900/80 text-indigo-300 px-1.5 py-0.5 rounded-md text-[12.5px] font-mono border border-zinc-700/30"
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
                  h1({ children }) {
                    return <h1 className="text-base font-bold text-violet-300 mt-4 mb-2 pb-1.5 border-b border-zinc-700/50 first:mt-0">{children}</h1>;
                  },
                  h2({ children }) {
                    return <h2 className="text-[15px] font-semibold text-indigo-300 mt-3.5 mb-1.5 first:mt-0">{children}</h2>;
                  },
                  h3({ children }) {
                    return <h3 className="text-sm font-semibold text-zinc-200 mt-3 mb-1 first:mt-0">{children}</h3>;
                  },
                  hr() {
                    return <hr className="border-none h-px bg-gradient-to-r from-transparent via-zinc-600/50 to-transparent my-4" />;
                  },
                  blockquote({ children }) {
                    return (
                      <blockquote className="border-l-2 border-indigo-500/40 pl-3 py-1 my-2.5 bg-indigo-500/[0.04] rounded-r-lg text-zinc-400 italic">
                        {children}
                      </blockquote>
                    );
                  },
                  table({ children }) {
                    return (
                      <div className="overflow-x-auto my-3 rounded-lg border border-zinc-700/40">
                        <table className="text-xs border-collapse w-full">{children}</table>
                      </div>
                    );
                  },
                  th({ children }) {
                    return <th className="border-b-2 border-zinc-700/60 px-3 py-2 bg-zinc-800/60 text-left text-indigo-300 font-semibold text-[11px] uppercase tracking-wider">{children}</th>;
                  },
                  td({ children }) {
                    return <td className="border-b border-zinc-700/30 px-3 py-1.5 text-zinc-200">{children}</td>;
                  },
                  strong({ children }) {
                    return <strong className="font-bold text-zinc-50">{children}</strong>;
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
              <span className="text-xs text-zinc-500">🧠 Thinking...</span>
            </div>
          )}
          {message.streaming && message.content && (
            <span className="inline-block w-0.5 h-4 bg-indigo-400 ml-0.5 animate-pulse" />
          )}
        </div>

        {/* Tool calls */}
        {message.toolCalls?.map((tc, i) => <ToolCallCard key={i} tool={tc} />)}

        {/* Footer — tokens/cost only, action buttons are in ChatView's MessageActions */}
        {(message.tokens != null || (message.cost != null && message.cost > 0)) && (
          <div className={`flex items-center gap-2 mt-1.5 ${isUser ? 'flex-row-reverse' : ''}`}>
            {message.tokens != null && (
              <span className="text-xs text-zinc-600">{message.tokens} tok</span>
            )}
            {message.cost != null && message.cost > 0 && (
              <span className="text-xs text-zinc-600">${message.cost.toFixed(5)}</span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
