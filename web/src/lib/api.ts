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

export interface AgentConfig {
  model: string;
  provider: string;
  allowed_tools: string[];
  max_tokens: number;
  max_cost_usd: number;
  thinking_level: string;
  temperature: number;
  system_prompt: string;
  timeout_seconds: number;
  auto_restart: boolean;
}

export interface AgentMetrics {
  tokens_used: number;
  cost_usd: number;
  tool_call_count: number;
  model_call_count: number;
  error_count: number;
  avg_response_ms: number;
  duration: string;
  token_history?: { time: string; tokens: number; cost: number }[];
  tool_calls?: { name: string; count: number }[];
  response_times?: { bucket: string; count: number }[];
}

export interface AgentDetailData extends AgentData {
  config: AgentConfig;
  metrics: AgentMetrics;
  parent_id?: string;
  child_ids?: string[];
}

export interface AgentLogEntry {
  timestamp: string;
  type: string;
  message: string;
  metadata?: Record<string, unknown>;
}

export interface AgentMessage {
  id: string;
  role: 'user' | 'agent';
  content: string;
  timestamp: string;
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
  timestamp?: string; // API may return "timestamp" instead of "created_at"
}

export interface CostData {
  date: string;
  cost: number;
  tokens: number;
}

export interface CronJob {
  id: string;
  name: string;
  schedule: string;
  task: string;
  status: 'active' | 'paused' | 'error';
  last_run?: string;
  next_run?: string;
  created_at: string;
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

// ── Agent detail endpoints (404-safe with demo fallback) ───────────────────

function makeDemoMetrics(agent: AgentData): AgentMetrics {
  const now = Date.now();
  return {
    tokens_used: Math.floor(Math.random() * 12000) + 500,
    cost_usd: Math.random() * 0.08,
    tool_call_count: Math.floor(Math.random() * 30),
    model_call_count: Math.floor(Math.random() * 20) + 1,
    error_count: Math.floor(Math.random() * 3),
    avg_response_ms: Math.floor(Math.random() * 2000) + 400,
    duration: agent.status === 'running' ? 'ongoing' : '3m 42s',
    token_history: Array.from({ length: 10 }, (_, i) => ({
      time: new Date(now - (9 - i) * 30000).toISOString(),
      tokens: Math.floor(Math.random() * 1200) + 100,
      cost: Math.random() * 0.008,
    })),
    tool_calls: [
      { name: 'read_file', count: Math.floor(Math.random() * 10) + 1 },
      { name: 'write_file', count: Math.floor(Math.random() * 5) },
      { name: 'exec', count: Math.floor(Math.random() * 8) },
      { name: 'search', count: Math.floor(Math.random() * 6) },
    ].filter(t => t.count > 0),
    response_times: [
      { bucket: '0-500ms', count: Math.floor(Math.random() * 8) },
      { bucket: '500ms-1s', count: Math.floor(Math.random() * 6) },
      { bucket: '1s-2s', count: Math.floor(Math.random() * 4) },
      { bucket: '2s+', count: Math.floor(Math.random() * 2) },
    ],
  };
}

function makeDemoConfig(): AgentConfig {
  return {
    model: 'claude-sonnet-4-5',
    provider: 'anthropic',
    allowed_tools: ['read_file', 'write_file', 'exec', 'search'],
    max_tokens: 8192,
    max_cost_usd: 0.50,
    thinking_level: 'medium',
    temperature: 0.7,
    system_prompt: '',
    timeout_seconds: 300,
    auto_restart: false,
  };
}

export async function fetchAgentDetail(id: string): Promise<AgentDetailData> {
  const base = await safeFetch<AgentDetailData>(`${BASE}/api/agents/${encodeURIComponent(id)}`, null as unknown as AgentDetailData);
  if (base) return base;
  // Fallback: build from agents list
  const agents = await fetchAgents();
  const agent = agents.find(a => a.id === id);
  const stub: AgentData = agent ?? {
    id,
    name: 'Unknown Agent',
    role: 'general',
    task: '—',
    status: 'stopped',
    created_at: new Date().toISOString(),
  };
  return {
    ...stub,
    config: makeDemoConfig(),
    metrics: makeDemoMetrics(stub),
  };
}

export async function fetchAgentLog(id: string, limit = 50): Promise<AgentLogEntry[]> {
  const data = await safeFetch<AgentLogEntry[] | { entries?: AgentLogEntry[] }>(
    `${BASE}/api/agents/${encodeURIComponent(id)}/log?limit=${limit}`,
    []
  );
  if (Array.isArray(data)) return data;
  const cast = data as { entries?: AgentLogEntry[] };
  if (cast.entries) return cast.entries;
  // Demo entries
  const types = ['model_call', 'tool_start', 'tool_done', 'status', 'message_received', 'error'];
  const msgs: Record<string, string[]> = {
    model_call: ['Sending prompt to model', 'Model responded with tool call', 'Streaming response'],
    tool_start: ['Starting read_file: example.txt', 'Starting exec: ls -la', 'Starting search: query'],
    tool_done: ['read_file completed (1.2 KB)', 'exec completed (exit 0)', 'search returned 5 results'],
    status: ['Agent started', 'Waiting for tool result', 'Resuming after approval'],
    message_received: ['User: Continue with the task', 'System: timeout warning'],
    error: ['Tool exec failed: permission denied', 'Model rate limited, retrying'],
  };
  return Array.from({ length: Math.min(limit, 12) }, (_, i) => {
    const type = types[i % types.length];
    const options = msgs[type];
    return {
      timestamp: new Date(Date.now() - (11 - i) * 15000).toISOString(),
      type,
      message: options[Math.floor(Math.random() * options.length)],
    };
  });
}

export async function fetchAgentMessages(id: string): Promise<AgentMessage[]> {
  const data = await safeFetch<AgentMessage[] | { messages?: AgentMessage[] }>(
    `${BASE}/api/agents/${encodeURIComponent(id)}/messages`,
    []
  );
  if (Array.isArray(data)) return data;
  const cast = data as { messages?: AgentMessage[] };
  return cast.messages ?? [];
}

export async function updateAgentConfig(id: string, config: Partial<AgentConfig>): Promise<void> {
  await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/config`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  });
}

export async function sendAgentMessage(id: string, message: string): Promise<void> {
  await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/message`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  });
}

export async function stopAgent(id: string): Promise<void> {
  await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/stop`, { method: 'POST' });
}

export async function pauseAgent(id: string): Promise<void> {
  await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/pause`, { method: 'POST' });
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

// ── File Browser API ───────────────────────────────────────────────────────

export interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
}

export interface FilesResponse {
  entries: FileEntry[];
  path: string;
  available?: boolean;
}

export interface FileContentResponse {
  path: string;
  content: string;
}

export async function listFiles(path = '.'): Promise<FilesResponse> {
  const data = await safeFetch<FilesResponse>(
    `${BASE}/api/files?path=${encodeURIComponent(path)}`,
    { entries: [], path }
  );
  return data;
}

export async function readFile(path: string): Promise<FileContentResponse> {
  const res = await fetch(`${BASE}/api/file?path=${encodeURIComponent(path)}`);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error((err as { error?: string }).error || 'Failed to read file');
  }
  return res.json();
}

// ── Terminal / Exec API ────────────────────────────────────────────────────

export interface ExecResponse {
  output: string;
  exit_code: number;
  error?: string;
}

export async function execCommand(command: string): Promise<ExecResponse> {
  const res = await fetch(`${BASE}/api/exec`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ command }),
  });
  // Return parsed body regardless of status so callers can show exit_code
  return res.json();
}

// ── Cron API ───────────────────────────────────────────────────────────────

export async function fetchCronJobs(): Promise<CronJob[]> {
  const data = await safeFetch<{ jobs?: CronJob[] } | CronJob[]>(`${BASE}/api/cron`, []);
  if (Array.isArray(data)) return data;
  return (data as { jobs?: CronJob[] }).jobs || [];
}

export async function createCronJob(payload: { name: string; schedule: string; task: string }): Promise<CronJob> {
  const res = await fetch(`${BASE}/api/cron`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Failed to create cron job' }));
    throw new Error((err as { error?: string }).error || 'Failed to create cron job');
  }
  return res.json();
}

export async function deleteCronJob(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/cron/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error('Failed to delete cron job');
}

export async function toggleCronJob(id: string): Promise<CronJob> {
  const res = await fetch(`${BASE}/api/cron/${encodeURIComponent(id)}/toggle`, { method: 'POST' });
  if (!res.ok) throw new Error('Failed to toggle cron job');
  return res.json();
}

export async function runCronJobNow(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/cron/${encodeURIComponent(id)}/run`, { method: 'POST' });
  if (!res.ok) throw new Error('Failed to trigger cron job');
}
