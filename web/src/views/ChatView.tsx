import { useState, useRef, useEffect, useCallback } from 'react';
import ChatMessage, { type Message } from '../components/ChatMessage';
import ChatInput from '../components/ChatInput';
import { useToast } from '../components/Toast';
import { streamChat } from '../lib/api';
import { MessageSquare, Sparkles, Terminal, Globe, Brain } from 'lucide-react';

let msgId = 0;
const uid = () => `msg_${++msgId}`;

export default function ChatView() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [streaming, setStreaming] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const toast = useToast();

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

    try {
      let fullText = '';
      for await (const chunk of streamChat(text)) {
        fullText += chunk;
        setMessages(prev => prev.map(m => m.id === aiId ? { ...m, content: fullText } : m));
      }
      setMessages(prev => prev.map(m => m.id === aiId ? { ...m, streaming: false } : m));
    } catch (err: any) {
      setMessages(prev => prev.map(m =>
        m.id === aiId ? { ...m, content: `Error: ${err.message}`, streaming: false } : m
      ));
      toast(err.message, 'error');
    } finally {
      setStreaming(false);
    }
  };

  const handleCancel = () => {
    abortRef.current?.abort();
    setStreaming(false);
    setMessages(prev => prev.map(m => m.streaming ? { ...m, streaming: false } : m));
  };

  if (messages.length === 0) {
    return (
      <div className="chat-view">
        <div className="welcome-screen" ref={scrollRef}>
          <div className="welcome-logo">
            <div className="welcome-icon">
              <Sparkles size={40} />
            </div>
            <h1>SoulGate</h1>
            <p className="welcome-tagline">Your AI, everywhere.</p>
          </div>
          <div className="quick-actions">
            <button className="quick-action" onClick={() => handleSend('What can you do?')}>
              <Brain size={20} />
              <span>What can you do?</span>
            </button>
            <button className="quick-action" onClick={() => handleSend('List files in the current directory')}>
              <Terminal size={20} />
              <span>List files</span>
            </button>
            <button className="quick-action" onClick={() => handleSend('Search the web for latest tech news')}>
              <Globe size={20} />
              <span>Web search</span>
            </button>
            <button className="quick-action" onClick={() => handleSend('Show system status')}>
              <MessageSquare size={20} />
              <span>System status</span>
            </button>
          </div>
        </div>
        <ChatInput onSend={handleSend} disabled={streaming} streaming={streaming} onCancel={handleCancel} />
      </div>
    );
  }

  return (
    <div className="chat-view">
      <div className="chat-messages" ref={scrollRef}>
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
      <ChatInput onSend={handleSend} onCancel={handleCancel} disabled={streaming} streaming={streaming} />
    </div>
  );
}
