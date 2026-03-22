import { useState, useRef, useEffect, useCallback } from 'react';
import ChatMessage, { type Message } from '../components/ChatMessage';
import ChatInput from '../components/ChatInput';
import { streamChat } from '../lib/api';
import { MessageSquare, Sparkles, Terminal, Globe, Brain, ChevronDown } from 'lucide-react';
import toast from 'react-hot-toast';

let msgId = 0;
const uid = () => `msg_${++msgId}`;

const MODELS = [
  'claude-opus-4-5',
  'claude-sonnet-4-5',
  'claude-haiku-3-5',
  'gpt-4o',
  'gpt-4o-mini',
  'gpt-4-turbo',
];

export default function ChatView() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [model, setModel] = useState(MODELS[0]);
  const [showModels, setShowModels] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  const scrollToBottom = useCallback(() => {
    setTimeout(() => {
      scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
    }, 50);
  }, []);

  useEffect(scrollToBottom, [messages, scrollToBottom]);

  const handleSend = async (text: string) => {
    const userMsg: Message = { id: uid(), role: 'user', content: text, timestamp: new Date() };
    const aiId = uid();
    const aiMsg: Message = { id: aiId, role: 'assistant', content: '', timestamp: new Date(), streaming: true };

    setMessages(prev => [...prev, userMsg, aiMsg]);
    setStreaming(true);
    abortRef.current = new AbortController();

    try {
      let fullText = '';
      for await (const chunk of streamChat(text, abortRef.current.signal)) {
        fullText += chunk;
        setMessages(prev => prev.map(m => m.id === aiId ? { ...m, content: fullText } : m));
      }
      setMessages(prev => prev.map(m => m.id === aiId ? { ...m, streaming: false } : m));
    } catch (err: unknown) {
      if ((err as Error).name === 'AbortError') {
        setMessages(prev => prev.map(m =>
          m.id === aiId ? { ...m, content: m.content || '(cancelled)', streaming: false } : m
        ));
        return;
      }
      const msg = (err as Error).message;
      setMessages(prev => prev.map(m =>
        m.id === aiId ? { ...m, content: `Error: ${msg}`, streaming: false } : m
      ));
      toast.error(msg);
    } finally {
      setStreaming(false);
    }
  };

  const handleCancel = () => {
    abortRef.current?.abort();
    setStreaming(false);
    setMessages(prev => prev.map(m => m.streaming ? { ...m, streaming: false } : m));
  };

  const handleClear = () => {
    setMessages([]);
    toast.success('Chat cleared');
  };

  const modelSelector = (
    <div className="relative">
      <button
        onClick={() => setShowModels(s => !s)}
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-xs text-zinc-400 hover:text-zinc-200 transition-all"
      >
        <span className="font-mono">{model}</span>
        <ChevronDown size={12} />
      </button>
      {showModels && (
        <div className="absolute bottom-full mb-1 left-0 z-30 bg-zinc-900 border border-zinc-700 rounded-lg shadow-xl overflow-hidden min-w-48">
          {MODELS.map(m => (
            <button
              key={m}
              onClick={() => { setModel(m); setShowModels(false); toast.success(`Model: ${m}`); }}
              className={`block w-full px-3 py-2 text-left text-xs font-mono hover:bg-zinc-800 transition-colors ${
                m === model ? 'text-indigo-400 bg-indigo-500/10' : 'text-zinc-400'
              }`}
            >
              {m}
            </button>
          ))}
        </div>
      )}
    </div>
  );

  if (messages.length === 0) {
    return (
      <div className="flex flex-col flex-1 overflow-hidden bg-zinc-950">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-3 border-b border-zinc-800/60">
          <div className="flex items-center gap-2">
            <Sparkles size={16} className="text-indigo-400" />
            <span className="text-sm font-semibold text-zinc-300">Chat</span>
          </div>
          <div className="flex items-center gap-2">{modelSelector}</div>
        </div>

        {/* Welcome screen */}
        <div className="flex-1 flex flex-col items-center justify-center gap-8 p-10">
          <div className="text-center">
            <div className="w-20 h-20 rounded-2xl bg-gradient-to-br from-indigo-600 to-violet-600 flex items-center justify-center mx-auto mb-4 shadow-lg shadow-indigo-500/20">
              <Sparkles size={40} className="text-white" />
            </div>
            <h1 className="text-3xl font-bold text-zinc-100">SoulGate</h1>
            <p className="text-zinc-500 mt-1">Your AI, everywhere.</p>
          </div>

          <div className="grid grid-cols-2 gap-3 max-w-md w-full">
            {[
              { icon: Brain,       label: 'What can you do?',             prompt: 'What can you do?' },
              { icon: Terminal,    label: 'List files',                   prompt: 'List files in the current directory' },
              { icon: Globe,       label: 'Search the web',               prompt: 'Search the web for latest tech news' },
              { icon: MessageSquare, label: 'System status',              prompt: 'Show system status' },
            ].map(({ icon: Icon, label, prompt }) => (
              <button
                key={label}
                onClick={() => handleSend(prompt)}
                className="flex items-center gap-3 p-4 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-zinc-400 hover:text-zinc-100 hover:border-indigo-500/40 hover:bg-zinc-800 transition-all text-sm text-left group"
              >
                <Icon size={18} className="text-zinc-500 group-hover:text-indigo-400 transition-colors" />
                <span>{label}</span>
              </button>
            ))}
          </div>
        </div>

        <ChatInput onSend={handleSend} disabled={streaming} streaming={streaming} onCancel={handleCancel} modelSelector={modelSelector} />
      </div>
    );
  }

  return (
    <div className="flex flex-col flex-1 overflow-hidden bg-zinc-950">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-3 border-b border-zinc-800/60 flex-shrink-0">
        <div className="flex items-center gap-2">
          <Sparkles size={16} className="text-indigo-400" />
          <span className="text-sm font-semibold text-zinc-300">Chat</span>
          <span className="text-xs text-zinc-600">({messages.filter(m => m.role !== 'system').length / 2 | 0} turns)</span>
        </div>
        <div className="flex items-center gap-2">
          {modelSelector}
          <button
            onClick={handleClear}
            className="text-xs text-zinc-600 hover:text-zinc-400 transition-colors px-2 py-1 rounded hover:bg-zinc-800"
          >
            Clear
          </button>
        </div>
      </div>

      {/* Messages */}
      <div
        ref={scrollRef}
        className="flex-1 overflow-y-auto px-6 py-5 scroll-smooth"
        style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}
      >
        {messages.map(msg => (
          <ChatMessage
            key={msg.id}
            message={msg}
            onRetry={msg.role === 'assistant' ? () => {
              const idx = messages.findIndex(m => m.id === msg.id);
              const userMsg = messages[idx - 1];
              if (userMsg?.role === 'user') handleSend(userMsg.content);
            } : undefined}
          />
        ))}
      </div>

      {/* Scroll to bottom indicator */}
      <div className="flex justify-center py-1">
        <button
          onClick={scrollToBottom}
          className="flex items-center gap-1 text-xs text-zinc-700 hover:text-zinc-500 transition-colors"
        >
          <ChevronDown size={12} />
        </button>
      </div>

      <ChatInput onSend={handleSend} onCancel={handleCancel} disabled={streaming} streaming={streaming} modelSelector={modelSelector} />
    </div>
  );
}
