const BASE = window.location.origin;

export interface HealthData {
  status: string;
  uptime: string;
  started_at: string;
  clients: Record<string, number>;
  sessions: number;
  provider: string;
  model: string;
  memory: { alloc_mb: number; sys_mb: number; num_gc: number; goroutines: number };
  checks: { name: string; status: string; detail?: string }[];
}

export interface SessionData {
  id: string;
  conversation_id: string;
  channel: string;
  message_count: number;
  created_at: string;
  last_activity: string;
}

export interface ChatResponse {
  response: string;
  error?: string;
}

export async function fetchHealth(): Promise<HealthData> {
  const res = await fetch(`${BASE}/api/health`);
  return res.json();
}

export async function fetchStatus(): Promise<any> {
  const res = await fetch(`${BASE}/api/status`);
  return res.json();
}

export async function fetchSessions(): Promise<SessionData[]> {
  const res = await fetch(`${BASE}/api/sessions`);
  const data = await res.json();
  return data.sessions || [];
}

export async function sendChat(message: string): Promise<ChatResponse> {
  const res = await fetch(`${BASE}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  });
  return res.json();
}

export async function* streamChat(message: string): AsyncGenerator<string> {
  const res = await fetch(`${BASE}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  });

  const contentType = res.headers.get('content-type') || '';

  if (contentType.includes('text/event-stream') || contentType.includes('application/x-ndjson')) {
    const reader = res.body!.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) {
        if (line.trim()) {
          try {
            const data = JSON.parse(line.startsWith('data: ') ? line.slice(6) : line);
            if (data.response) yield data.response;
            if (data.delta) yield data.delta;
          } catch { yield line; }
        }
      }
    }
  } else {
    const data: ChatResponse = await res.json();
    if (data.error) throw new Error(data.error);
    if (data.response) yield data.response;
  }
}
