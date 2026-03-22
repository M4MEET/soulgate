const BASE = window.location.origin;

// ── Types ────────────────────────────────────────────────────────────────────

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

export interface ToolData {
  name: string;
  description: string;
  schema?: Record<string, unknown>;
  category?: string;
}

export interface AgentData {
  id: string;
  name: string;
  role: string;
  task: string;
  status: 'running' | 'completed' | 'stopped' | 'error';
  created_at: string;
  last_activity?: string;
  message_count?: number;
  log?: string[];
}

export interface MemoryEntry {
  key: string;
  value: string;
  type: 'string' | 'json' | 'vector';
  created_at: string;
  updated_at: string;
  tags?: string[];
}

export interface AuditEvent {
  id: string;
  type: string;
  category: string;
  session_id?: string;
  run_id?: string;
  status: string;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface CostData {
  date: string;
  cost: number;
  tokens: number;
}

export interface ConfigData {
  provider: string;
  model: string;
  max_tokens?: number;
  temperature?: number;
  max_turns?: number;
  timeout?: string;
  webhooks?: { name: string; url: string; events: string[] }[];
  policies?: { name: string; action: string; resource: string; decision: string; priority: number }[];
}

// ── Safe fetch helper ─────────────────────────────────────────────────────────

async function safeFetch<T>(url: string, fallback: T): Promise<T> {
  try {
    const res = await fetch(url);
    if (!res.ok) return fallback;
    return (await res.json()) as T;
  } catch {
    return fallback;
  }
}

// ── Core endpoints ────────────────────────────────────────────────────────────

export async function fetchHealth(): Promise<HealthData> {
  const res = await fetch(`${BASE}/api/health`);
  return res.json();
}

export async function fetchStatus(): Promise<unknown> {
  return safeFetch(`${BASE}/api/status`, null);
}

export async function fetchSessions(): Promise<SessionData[]> {
  const data = await safeFetch<{ sessions?: SessionData[] }>(`${BASE}/api/sessions`, {});
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

export async function* streamChat(message: string, signal?: AbortSignal): AsyncGenerator<string> {
  const res = await fetch(`${BASE}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
    signal,
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

// ── Optional endpoints (404-safe) ────────────────────────────────────────────

export async function fetchTools(): Promise<ToolData[]> {
  const data = await safeFetch<{ tools?: ToolData[] } | ToolData[]>(`${BASE}/api/tools`, []);
  if (Array.isArray(data)) return data;
  return (data as { tools?: ToolData[] }).tools || [];
}

export async function fetchAgents(): Promise<AgentData[]> {
  const data = await safeFetch<{ agents?: AgentData[] } | AgentData[]>(`${BASE}/api/agents`, []);
  if (Array.isArray(data)) return data;
  return (data as { agents?: AgentData[] }).agents || [];
}

export async function createAgent(payload: { name: string; task: string; role: string }): Promise<AgentData> {
  const res = await fetch(`${BASE}/api/agents`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error('Failed to create agent');
  return res.json();
}

export async function fetchMemory(): Promise<MemoryEntry[]> {
  const data = await safeFetch<{ entries?: MemoryEntry[] } | MemoryEntry[]>(`${BASE}/api/memory`, []);
  if (Array.isArray(data)) return data;
  return (data as { entries?: MemoryEntry[] }).entries || [];
}

export async function setMemoryEntry(key: string, value: string): Promise<void> {
  await fetch(`${BASE}/api/memory`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, value }),
  });
}

export async function deleteMemoryEntry(key: string): Promise<void> {
  await fetch(`${BASE}/api/memory/${encodeURIComponent(key)}`, { method: 'DELETE' });
}

export async function fetchAuditEvents(limit = 100): Promise<AuditEvent[]> {
  const data = await safeFetch<{ events?: AuditEvent[] } | AuditEvent[]>(
    `${BASE}/api/audit?limit=${limit}`,
    []
  );
  if (Array.isArray(data)) return data;
  return (data as { events?: AuditEvent[] }).events || [];
}

export async function fetchCosts(): Promise<CostData[]> {
  return safeFetch<CostData[]>(`${BASE}/api/costs`, []);
}

export async function fetchConfig(): Promise<ConfigData | null> {
  return safeFetch<ConfigData | null>(`${BASE}/api/config`, null);
}

export async function updateConfig(config: Partial<ConfigData>): Promise<void> {
  await fetch(`${BASE}/api/config`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  });
}

export async function tryTool(name: string, args: Record<string, unknown>): Promise<unknown> {
  const res = await fetch(`${BASE}/api/tools/${encodeURIComponent(name)}/invoke`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(args),
  });
  return res.json();
}
