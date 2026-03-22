import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Copy, Check, RotateCcw, GitFork } from 'lucide-react';

export interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: Date;
  tokens?: number;
  cost?: number;
  toolCalls?: ToolCall[];
  streaming?: boolean;
}

export interface ToolCall {
  name: string;
  args: string;
  result: string;
  duration: string;
  status: 'success' | 'error' | 'running';
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      className="copy-btn"
      onClick={() => {
        navigator.clipboard.writeText(text);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }}
      title="Copy"
    >
      {copied ? <Check size={14} /> : <Copy size={14} />}
    </button>
  );
}

function ToolCallCard({ tool }: { tool: ToolCall }) {
  const [expanded, setExpanded] = useState(false);
  const statusIcon = tool.status === 'success' ? '✓' : tool.status === 'error' ? '✗' : '⟳';
  const statusClass = `tool-status-${tool.status}`;

  return (
    <div className={`tool-card ${statusClass}`}>
      <div className="tool-header" onClick={() => setExpanded(!expanded)}>
        <span className="tool-icon">⚡</span>
        <span className="tool-name">{tool.name}</span>
        <span className="tool-duration">{tool.duration}</span>
        <span className={`tool-indicator ${statusClass}`}>{statusIcon}</span>
        <span className="tool-expand">{expanded ? '▲' : '▼'}</span>
      </div>
      {expanded && (
        <div className="tool-body">
          {tool.args && <div className="tool-args"><code>{tool.args}</code></div>}
          {tool.result && (
            <div className="tool-result">
              <pre>{tool.result.slice(0, 2000)}{tool.result.length > 2000 ? '\n...' : ''}</pre>
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
    return <div className="msg msg-system">{message.content}</div>;
  }

  return (
    <div className={`msg ${isUser ? 'msg-user' : 'msg-ai'}`}>
      <div className="msg-avatar">
        {isUser ? '👤' : '🤖'}
      </div>
      <div className="msg-content">
        <div className="msg-header">
          <span className="msg-role">{isUser ? 'You' : 'SoulGate'}</span>
          <span className="msg-time">
            {message.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </span>
        </div>

        <div className={`msg-body ${message.streaming ? 'streaming' : ''}`}>
          {isUser ? (
            <p>{message.content}</p>
          ) : (
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                code({ className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '');
                  const code = String(children).replace(/\n$/, '');
                  if (match) {
                    return (
                      <div className="code-block">
                        <div className="code-header">
                          <span className="code-lang">{match[1]}</span>
                          <CopyButton text={code} />
                        </div>
                        <SyntaxHighlighter
                          style={oneDark}
                          language={match[1]}
                          customStyle={{
                            margin: 0,
                            borderRadius: '0 0 8px 8px',
                            fontSize: '13px',
                            background: '#0d0d18',
                          }}
                        >
                          {code}
                        </SyntaxHighlighter>
                      </div>
                    );
                  }
                  return <code className="inline-code" {...props}>{children}</code>;
                },
                a({ href, children }) {
                  return <a href={href} target="_blank" rel="noopener noreferrer" className="msg-link">{children}</a>;
                },
              }}
            >
              {message.content}
            </ReactMarkdown>
          )}
          {message.streaming && <span className="cursor-blink">▎</span>}
        </div>

        {message.toolCalls?.map((tc, i) => <ToolCallCard key={i} tool={tc} />)}

        <div className="msg-footer">
          {message.tokens != null && (
            <span className="msg-tokens">{message.tokens} tok</span>
          )}
          {message.cost != null && message.cost > 0 && (
            <span className="msg-cost">${message.cost.toFixed(4)}</span>
          )}
          {!isUser && !message.streaming && (
            <div className="msg-actions">
              <CopyButton text={message.content} />
              {onRetry && <button className="action-btn" onClick={onRetry} title="Retry"><RotateCcw size={13} /></button>}
              {onFork && <button className="action-btn" onClick={onFork} title="Fork"><GitFork size={13} /></button>}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
