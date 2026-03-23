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
  schedule_enabled: boolean;
  schedule_cron: string;
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

export interface CostSummary {
  today_cost_usd: number;
  total_cost_usd: number;
  session_cost_usd: number;
  total_calls: number;
  session_calls: number;
  by_provider: Record<string, number>;
  by_model: Record<string, number>;
  last_7_days: { date: string; cost_usd: number; tokens?: number }[];
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

// ── Notification types ────────────────────────────────────────────────────────

export interface InboxNotification {
  id: string;
  kind: string;
  title: string;
  detail?: string;
  metadata?: Record<string, unknown>;
  timestamp: string;
  read: boolean;
  pinned?: boolean;
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

// ── Session detail & activity ────────────────────────────────────────────────

export interface SessionMessage {
  ts: number;
  type: string;
  data: Record<string, unknown>;
}

export interface SessionDetail {
  session_id: string;
  meta: {
    id: string;
    conversation_id: string;
    channel: string;
    state: string;
    message_count: number;
    assigned_agent: string;
    created_at: string;
    last_activity: string;
  } | null;
  messages: SessionMessage[];
}

export interface ActivityEntry {
  session_id: string;
  channel: string;
  ts: number;
  type: string;
  data: Record<string, unknown>;
}

export async function fetchSessionDetail(sessionId: string): Promise<SessionDetail> {
  return safeFetch<SessionDetail>(`${BASE}/api/sessions/${encodeURIComponent(sessionId)}`, {
    session_id: sessionId,
    meta: null,
    messages: [],
  });
}

export async function fetchActivity(limit = 50, channel?: string): Promise<ActivityEntry[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (channel) params.set('channel', channel);
  const data = await safeFetch<{ activity?: ActivityEntry[] }>(`${BASE}/api/activity?${params}`, {});
  return data.activity || [];
}

export async function sendChat(message: string): Promise<ChatResponse> {
  const res = await fetch(`${BASE}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  });
  return res.json();
}

export interface StreamEvent {
  kind: 'iteration' | 'model_call' | 'model_done' | 'tool_start' | 'tool_done' | 'stream' | 'status' | 'done' | 'error';
  message: string;
  data?: string;
  tokens?: number;
}

// streamChatSSE streams thinking events via SSE — yields structured events
export async function* streamChatSSE(message: string, signal?: AbortSignal): AsyncGenerator<StreamEvent> {
  const res = await fetch(`${BASE}/api/chat?stream=true`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Accept': 'text/event-stream' },
    body: JSON.stringify({ message }),
    signal,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error((body as { error?: string }).error || `HTTP ${res.status}`);
  }

  const contentType = res.headers.get('content-type') || '';

  if (contentType.includes('text/event-stream')) {
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
        const trimmed = line.trim();
        if (trimmed.startsWith('data: ')) {
          try {
            const evt: StreamEvent = JSON.parse(trimmed.slice(6));
            yield evt;
          } catch { /* skip malformed */ }
        }
      }
    }
  } else {
    // Fallback: non-streaming JSON response
    const data: ChatResponse = await res.json();
    if (data.error) throw new Error(data.error);
    if (data.response) {
      yield { kind: 'stream', message: data.response };
      yield { kind: 'done', message: data.response };
    }
  }
}

// streamChat — backward-compatible string-only generator (used by ChatView)
export async function* streamChat(message: string, signal?: AbortSignal): AsyncGenerator<string> {
  let fullText = '';
  for await (const evt of streamChatSSE(message, signal)) {
    if (evt.kind === 'stream') {
      fullText += evt.message;
      yield fullText; // yield accumulated text
    } else if (evt.kind === 'done') {
      if (evt.message && evt.message !== fullText) {
        yield evt.message; // yield final response
      }
    } else if (evt.kind === 'error') {
      throw new Error(evt.message);
    }
    // thinking events are ignored in string mode
  }
}

// ── Connector endpoints ───────────────────────────────────────────────────────

export interface ConnectorClient {
  client_id: string;
  channel: string;
  metadata: Record<string, string>;
}

export interface SpawnedConnector {
  type: string;
  pid: number;
  started_at: string;
  status: string;
}

export interface ConnectorsData {
  channels: ConnectorClient[];
  agents: { client_id: string; metadata: Record<string, string> }[];
  uis: { client_id: string }[];
  sessions_by_channel: Record<string, number>;
  spawned?: SpawnedConnector[];
}

export async function disconnectConnector(
  type: string,
): Promise<{ status: string; type: string; killed_count: number; error?: string }> {
  const res = await fetch(`${BASE}/api/connectors/${encodeURIComponent(type)}`, {
    method: 'DELETE',
  });
  const text = await res.text();
  try {
    return JSON.parse(text);
  } catch {
    return { status: 'error', type, killed_count: 0, error: text || `HTTP ${res.status}` };
  }
}

export async function fetchConnectors(): Promise<ConnectorsData> {
  return safeFetch<ConnectorsData>(`${BASE}/api/connectors`, {
    channels: [],
    agents: [],
    uis: [],
    sessions_by_channel: {},
  });
}

export async function replyToSession(
  sessionId: string,
  message: string,
): Promise<{ status: string; response: string; error?: string }> {
  const res = await fetch(`${BASE}/api/sessions/${encodeURIComponent(sessionId)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  });
  return res.json();
}

// ── Provider & Model catalog endpoints ────────────────────────────────────────

export interface ProviderInfo {
  id: string;
  name: string;
  models: number;
}

export interface ModelInfo {
  id: string;
  name: string;
  description?: string;
  provider?: string;
  context_length?: number;
}

// ── Persistent Notifications ──────────────────────────────────────────────────

export async function fetchNotifications(): Promise<{ notifications: InboxNotification[]; unread: number }> {
  return safeFetch(`${BASE}/api/notifications`, { notifications: [], unread: 0 });
}

export async function markNotificationRead(id: string): Promise<void> {
  await fetch(`${BASE}/api/notifications/${encodeURIComponent(id)}`, { method: 'POST' });
}

export async function deleteNotification(id: string): Promise<void> {
  await fetch(`${BASE}/api/notifications/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function markAllNotificationsRead(): Promise<void> {
  await fetch(`${BASE}/api/notifications`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action: 'mark_all_read' }),
  });
}

export async function clearReadNotifications(): Promise<void> {
  await fetch(`${BASE}/api/notifications`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action: 'clear_read' }),
  });
}

// ── Provider & Model catalog endpoints ────────────────────────────────────────

export async function fetchProviders(): Promise<string[]> {
  const data = await safeFetch<{ providers?: string[] }>(`${BASE}/api/providers`, {});
  return data.providers || [];
}

export async function fetchProviderModels(provider: string): Promise<ModelInfo[]> {
  const data = await safeFetch<{ models?: ModelInfo[] }>(`${BASE}/api/providers/${encodeURIComponent(provider)}`, {});
  return data.models || [];
}

export async function fetchAPIKeyStatus(): Promise<Record<string, boolean>> {
  const data = await safeFetch<{ keys?: Record<string, boolean> }>(`${BASE}/api/apikeys`, {});
  return data.keys || {};
}

export async function saveAPIKey(provider: string, key: string): Promise<{ status?: string; error?: string }> {
  const res = await fetch(`${BASE}/api/apikeys`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ provider, key }),
  });
  return res.json();
}

// ── Connector endpoints ───────────────────────────────────────────────────────

export async function spawnConnector(
  type: string,
  config: Record<string, string>,
): Promise<{ status: string; message: string; error?: string }> {
  const res = await fetch(`${BASE}/api/connectors`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ type, config }),
  });
  const text = await res.text();
  try {
    return JSON.parse(text);
  } catch {
    return { status: 'error', message: '', error: text || `HTTP ${res.status}` };
  }
}

// ── Optional endpoints (404-safe) ────────────────────────────────────────────

export async function fetchTools(): Promise<ToolData[]> {
  const data = await safeFetch<{ tools?: ToolData[] } | ToolData[]>(`${BASE}/api/tools`, []);
  if (Array.isArray(data)) return data;
  return (data as { tools?: ToolData[] }).tools || [];
}

// ── Hub endpoints ─────────────────────────────────────────────────────────────

export interface HubItem {
  name: string;
  description: string;
  version?: string;
  author?: string;
  category?: string;
  tags?: string[];
  rating?: number;
  downloads?: number;
}

export interface InstalledHubItem {
  type: string;
  name: string;
  version: string;
  installed_at: string;
}

export async function hubSearch(query: string): Promise<HubItem[]> {
  const data = await safeFetch<{ results?: HubItem[]; error?: string }>(
    `${BASE}/api/hub?q=${encodeURIComponent(query)}`, {}
  );
  return data.results || [];
}

export async function hubInstall(name: string): Promise<{ status: string; error?: string }> {
  const res = await fetch(`${BASE}/api/hub/install`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  return res.json();
}

export async function hubUninstall(name: string): Promise<{ status: string; error?: string }> {
  const res = await fetch(`${BASE}/api/hub/uninstall`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  return res.json();
}

export async function hubInstalled(): Promise<InstalledHubItem[]> {
  const data = await safeFetch<{ installed?: InstalledHubItem[] }>(`${BASE}/api/hub/installed`, {});
  return data.installed || [];
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

// ── Agent detail endpoints (404-safe) ─────────────────────────────────────

function makeEmptyMetrics(): AgentMetrics {
  return {
    tokens_used: 0,
    cost_usd: 0,
    tool_call_count: 0,
    model_call_count: 0,
    error_count: 0,
    avg_response_ms: 0,
    duration: '',
    token_history: [],
    tool_calls: [],
    response_times: [],
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
    schedule_enabled: false,
    schedule_cron: '',
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
    metrics: makeEmptyMetrics(),
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
  return [];
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
  const res = await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/config`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  });
  if (!res.ok) throw new Error((await res.json()).error || 'Failed to update config');
}

export async function sendAgentMessage(id: string, message: string): Promise<void> {
  const res = await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/message`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message }),
  });
  if (!res.ok) throw new Error((await res.json()).error || 'Failed to send message');
}

export async function stopAgent(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error((await res.json()).error || 'Failed to stop agent');
}

export async function deleteAgent(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/delete`, { method: 'POST' });
  if (!res.ok) throw new Error((await res.json()).error || 'Failed to delete agent');
}

export async function pauseAgent(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error((await res.json()).error || 'Failed to pause agent');
}

export async function restartAgent(id: string): Promise<{ new_id: string; old_id: string }> {
  const res = await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/restart`, { method: 'POST' });
  if (!res.ok) throw new Error((await res.json()).error || 'Failed to restart agent');
  return res.json();
}

export async function activateStandby(id: string): Promise<void> {
  const res = await fetch(`${BASE}/api/agents/${encodeURIComponent(id)}/standby`, { method: 'POST' });
  if (!res.ok) throw new Error((await res.json()).error || 'Failed to activate standby');
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

export async function fetchCostSummary(): Promise<CostSummary | null> {
  return safeFetch<CostSummary | null>(`${BASE}/api/costs`, null);
}

export async function fetchCosts(): Promise<CostData[]> {
  const summary = await fetchCostSummary();
  if (!summary || !summary.last_7_days) return [];
  return summary.last_7_days.map(d => ({
    date: d.date,
    cost: d.cost_usd,
    tokens: d.tokens ?? 0,
  }));
}

export async function fetchConfig(): Promise<ConfigData | null> {
  return safeFetch<ConfigData | null>(`${BASE}/api/config`, null);
}

export async function updateConfig(config: Partial<ConfigData>): Promise<void> {
  // The gateway accepts individual key/value POST updates.
  // Send provider and model as separate requests.
  const updates: { key: string; value: string }[] = [];
  if (config.provider) updates.push({ key: 'provider', value: config.provider });
  if (config.model) updates.push({ key: 'model', value: config.model });

  for (const update of updates) {
    const res = await fetch(`${BASE}/api/config`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(update),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      throw new Error((body as { error?: string }).error || `Failed to update ${update.key}`);
    }
  }
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

// ── Enterprise: Users ──────────────────────────────────────────────────────

export type UserRole = 'admin' | 'developer' | 'viewer' | 'operator';
export type UserStatus = 'active' | 'inactive';

export interface UserLimits {
  max_tokens_day: number;
  max_cost_day: number;
  max_cost_month: number;
  max_concurrent_agents: number;
  allowed_models: string[];
  allowed_tools: string[];
}

export interface UserSettings {
  default_model: string;
  default_provider: string;
  thinking_level: 'none' | 'low' | 'medium' | 'high';
  temperature: number;
  streaming: boolean;
  theme: 'dark' | 'light' | 'system';
}

export interface UserUsage {
  tokens_today: number;
  cost_today: number;
  cost_month: number;
}

export interface User {
  id: string;
  username: string;
  display_name: string;
  email: string;
  role: UserRole;
  team_id?: string;
  team_name?: string;
  status: UserStatus;
  created_at: string;
  last_active?: string;
  settings?: UserSettings;
  limits?: UserLimits;
  usage?: UserUsage;
  api_key_masked?: string;
}

export interface CreateUserPayload {
  username: string;
  display_name: string;
  email: string;
  role: UserRole;
  team_id?: string;
}

// Demo data helpers
function makeDemoUsers(): User[] {
  const now = new Date().toISOString();
  const teams = [
    { id: 'team-1', name: 'Platform' },
    { id: 'team-2', name: 'Security' },
    { id: 'team-3', name: 'Data' },
  ];
  return [
    {
      id: 'u-1', username: 'alice', display_name: 'Alice Chen', email: 'alice@example.com',
      role: 'admin', team_id: 'team-1', team_name: 'Platform', status: 'active',
      created_at: new Date(Date.now() - 90 * 86400000).toISOString(),
      last_active: new Date(Date.now() - 5 * 60000).toISOString(),
      api_key_masked: 'sg_****************************a1b2',
      settings: { default_model: 'claude-opus-4-5', default_provider: 'anthropic', thinking_level: 'high', temperature: 0.7, streaming: true, theme: 'dark' },
      limits: { max_tokens_day: 1000000, max_cost_day: 20, max_cost_month: 400, max_concurrent_agents: 10, allowed_models: ['claude-opus-4-5', 'gpt-4.1'], allowed_tools: ['read_file', 'write_file', 'exec', 'search'] },
      usage: { tokens_today: 48200, cost_today: 1.24, cost_month: 18.50 },
    },
    {
      id: 'u-2', username: 'bob', display_name: 'Bob Smith', email: 'bob@example.com',
      role: 'developer', team_id: 'team-1', team_name: 'Platform', status: 'active',
      created_at: new Date(Date.now() - 60 * 86400000).toISOString(),
      last_active: new Date(Date.now() - 2 * 3600000).toISOString(),
      api_key_masked: 'sg_****************************c3d4',
      settings: { default_model: 'claude-sonnet-4-5', default_provider: 'anthropic', thinking_level: 'medium', temperature: 0.5, streaming: true, theme: 'dark' },
      limits: { max_tokens_day: 200000, max_cost_day: 5, max_cost_month: 80, max_concurrent_agents: 3, allowed_models: ['claude-sonnet-4-5', 'gpt-4o'], allowed_tools: ['read_file', 'search'] },
      usage: { tokens_today: 12400, cost_today: 0.31, cost_month: 4.20 },
    },
    {
      id: 'u-3', username: 'carol', display_name: 'Carol Davis', email: 'carol@example.com',
      role: 'operator', team_id: 'team-2', team_name: 'Security', status: 'active',
      created_at: new Date(Date.now() - 45 * 86400000).toISOString(),
      last_active: new Date(Date.now() - 30 * 60000).toISOString(),
      api_key_masked: 'sg_****************************e5f6',
      settings: { default_model: 'claude-haiku-4-5', default_provider: 'anthropic', thinking_level: 'low', temperature: 0.3, streaming: false, theme: 'light' },
      limits: { max_tokens_day: 100000, max_cost_day: 2, max_cost_month: 40, max_concurrent_agents: 2, allowed_models: ['claude-haiku-4-5'], allowed_tools: ['read_file'] },
      usage: { tokens_today: 5100, cost_today: 0.08, cost_month: 1.90 },
    },
    {
      id: 'u-4', username: 'dan', display_name: 'Dan Wilson', email: 'dan@example.com',
      role: 'viewer', team_id: 'team-3', team_name: 'Data', status: 'inactive',
      created_at: new Date(Date.now() - 120 * 86400000).toISOString(),
      last_active: new Date(Date.now() - 7 * 86400000).toISOString(),
      api_key_masked: 'sg_****************************g7h8',
      settings: { default_model: 'gpt-4o-mini', default_provider: 'openai', thinking_level: 'none', temperature: 1.0, streaming: true, theme: 'system' },
      limits: { max_tokens_day: 50000, max_cost_day: 1, max_cost_month: 20, max_concurrent_agents: 1, allowed_models: ['gpt-4o-mini'], allowed_tools: [] },
      usage: { tokens_today: 0, cost_today: 0, cost_month: 0.42 },
    },
    {
      id: 'u-5', username: 'eve', display_name: 'Eve Martinez', email: 'eve@example.com',
      role: 'developer', team_id: 'team-3', team_name: 'Data', status: 'active',
      created_at: new Date(Date.now() - 30 * 86400000).toISOString(),
      last_active: new Date(Date.now() - 10 * 60000).toISOString(),
      api_key_masked: 'sg_****************************i9j0',
      settings: { default_model: 'gpt-4.1', default_provider: 'openai', thinking_level: 'medium', temperature: 0.8, streaming: true, theme: 'dark' },
      limits: { max_tokens_day: 300000, max_cost_day: 8, max_cost_month: 150, max_concurrent_agents: 5, allowed_models: ['gpt-4.1', 'gpt-4o'], allowed_tools: ['read_file', 'write_file', 'search'] },
      usage: { tokens_today: 31000, cost_today: 0.92, cost_month: 22.10 },
    },
  ];
  void teams; void now;
  return makeDemoUsers.cache ?? (makeDemoUsers.cache = []);
}
makeDemoUsers.cache = null as User[] | null;
// Initialize cache
(function() { makeDemoUsers.cache = makeDemoUsers(); })();

export async function fetchUsers(): Promise<User[]> {
  const data = await safeFetch<User[] | { users?: User[] }>(`${BASE}/api/users`, []);
  if (Array.isArray(data)) return data.length ? data : makeDemoUsers.cache!;
  const cast = data as { users?: User[] };
  return cast.users?.length ? cast.users : makeDemoUsers.cache!;
}

export async function fetchCurrentUser(): Promise<User> {
  const data = await safeFetch<User>(`${BASE}/api/users/me`, null as unknown as User);
  return data ?? makeDemoUsers.cache![0];
}

export async function createUser(payload: CreateUserPayload): Promise<User> {
  const res = await fetch(`${BASE}/api/users`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    // Return demo stub
    const newUser: User = {
      id: `u-${Date.now()}`,
      username: payload.username,
      display_name: payload.display_name,
      email: payload.email,
      role: payload.role,
      team_id: payload.team_id,
      status: 'active',
      created_at: new Date().toISOString(),
      api_key_masked: 'sg_****************************new1',
      settings: { default_model: 'claude-sonnet-4-5', default_provider: 'anthropic', thinking_level: 'medium', temperature: 0.7, streaming: true, theme: 'dark' },
      limits: { max_tokens_day: 200000, max_cost_day: 5, max_cost_month: 80, max_concurrent_agents: 3, allowed_models: [], allowed_tools: [] },
      usage: { tokens_today: 0, cost_today: 0, cost_month: 0 },
    };
    makeDemoUsers.cache = [...(makeDemoUsers.cache ?? []), newUser];
    return newUser;
  }
  return res.json();
}

export async function updateUser(id: string, data: Partial<User>): Promise<void> {
  await fetch(`${BASE}/api/users/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  // Update demo cache
  if (makeDemoUsers.cache) {
    makeDemoUsers.cache = makeDemoUsers.cache.map(u => u.id === id ? { ...u, ...data } : u);
  }
}

export async function deleteUser(id: string): Promise<void> {
  await fetch(`${BASE}/api/users/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (makeDemoUsers.cache) {
    makeDemoUsers.cache = makeDemoUsers.cache.filter(u => u.id !== id);
  }
}

export async function regenerateApiKey(id: string): Promise<string> {
  const res = await fetch(`${BASE}/api/users/${encodeURIComponent(id)}/api-key`, { method: 'POST' });
  if (!res.ok) return `sg_${'*'.repeat(28)}regen`;
  const data = await res.json();
  return (data as { api_key?: string }).api_key ?? `sg_${'*'.repeat(28)}regen`;
}

// ── Enterprise: Teams ─────────────────────────────────────────────────────

export interface Team {
  id: string;
  name: string;
  description?: string;
  member_count: number;
  active_agents: number;
  total_cost_month: number;
  created_at: string;
  members?: User[];
  limits?: {
    max_cost_month: number;
    max_concurrent_agents: number;
    allowed_models: string[];
  };
}

export interface CreateTeamPayload {
  name: string;
  description?: string;
}

function makeDemoTeams(): Team[] {
  return [
    {
      id: 'team-1', name: 'Platform', description: 'Core platform engineering',
      member_count: 2, active_agents: 3, total_cost_month: 22.70,
      created_at: new Date(Date.now() - 180 * 86400000).toISOString(),
      limits: { max_cost_month: 500, max_concurrent_agents: 20, allowed_models: ['claude-opus-4-5', 'claude-sonnet-4-5', 'gpt-4.1'] },
    },
    {
      id: 'team-2', name: 'Security', description: 'Security operations and auditing',
      member_count: 1, active_agents: 1, total_cost_month: 1.90,
      created_at: new Date(Date.now() - 150 * 86400000).toISOString(),
      limits: { max_cost_month: 100, max_concurrent_agents: 5, allowed_models: ['claude-haiku-4-5'] },
    },
    {
      id: 'team-3', name: 'Data', description: 'Data science and analytics',
      member_count: 2, active_agents: 2, total_cost_month: 22.52,
      created_at: new Date(Date.now() - 120 * 86400000).toISOString(),
      limits: { max_cost_month: 300, max_concurrent_agents: 10, allowed_models: ['gpt-4.1', 'gpt-4o', 'gpt-4o-mini'] },
    },
  ];
}

makeDemoTeams.cache = null as Team[] | null;
(function() { makeDemoTeams.cache = makeDemoTeams(); })();

export async function fetchTeams(): Promise<Team[]> {
  const data = await safeFetch<Team[] | { teams?: Team[] }>(`${BASE}/api/teams`, []);
  if (Array.isArray(data)) return data.length ? data : makeDemoTeams.cache!;
  const cast = data as { teams?: Team[] };
  return cast.teams?.length ? cast.teams : makeDemoTeams.cache!;
}

export async function createTeam(payload: CreateTeamPayload): Promise<Team> {
  const res = await fetch(`${BASE}/api/teams`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const newTeam: Team = {
      id: `team-${Date.now()}`,
      name: payload.name,
      description: payload.description,
      member_count: 0,
      active_agents: 0,
      total_cost_month: 0,
      created_at: new Date().toISOString(),
      limits: { max_cost_month: 100, max_concurrent_agents: 5, allowed_models: [] },
    };
    makeDemoTeams.cache = [...(makeDemoTeams.cache ?? []), newTeam];
    return newTeam;
  }
  return res.json();
}

export async function deleteTeam(id: string): Promise<void> {
  await fetch(`${BASE}/api/teams/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (makeDemoTeams.cache) {
    makeDemoTeams.cache = makeDemoTeams.cache.filter(t => t.id !== id);
  }
}

export async function addTeamMember(teamId: string, userId: string): Promise<void> {
  await fetch(`${BASE}/api/teams/${encodeURIComponent(teamId)}/members`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ user_id: userId }),
  });
}

export async function removeTeamMember(teamId: string, userId: string): Promise<void> {
  await fetch(`${BASE}/api/teams/${encodeURIComponent(teamId)}/members/${encodeURIComponent(userId)}`, { method: 'DELETE' });
}

// ── Enterprise: Policies ──────────────────────────────────────────────────

export type PolicyDecision = 'allow' | 'deny' | 'require_approval';
export type PolicyScope = 'global' | 'team' | 'user' | 'agent';

export interface PolicyRule {
  id: string;
  name: string;
  scope: PolicyScope;
  applies_to?: string; // team id, user id, or agent id
  action: string;
  resource: string;
  decision: PolicyDecision;
  priority: number;
  enabled: boolean;
  // Advanced conditions
  time_restriction?: { start: string; end: string; days: string[] };
  cost_limit?: { daily?: number; monthly?: number };
  models?: string[];
  pii_action?: 'allow' | 'block' | 'redact';
  created_at: string;
  updated_at: string;
}

export interface PolicyTestRequest {
  action: string;
  resource: string;
  user?: string;
  scope?: PolicyScope;
}

export interface PolicyTestResult {
  decision: PolicyDecision;
  matched_rule?: PolicyRule;
  all_matched: PolicyRule[];
  reason: string;
}

function makeDemoPolicies(): PolicyRule[] {
  const now = new Date().toISOString();
  return [
    {
      id: 'p-1', name: 'allow-workspace-reads', scope: 'global', applies_to: undefined,
      action: 'files.read', resource: './**', decision: 'allow', priority: 10, enabled: true,
      created_at: now, updated_at: now,
    },
    {
      id: 'p-2', name: 'deny-parent-access', scope: 'global', applies_to: undefined,
      action: 'files.*', resource: '../**', decision: 'deny', priority: 20, enabled: true,
      created_at: now, updated_at: now,
    },
    {
      id: 'p-3', name: 'require-exec-approval', scope: 'global', applies_to: undefined,
      action: 'exec.*', resource: '*', decision: 'require_approval', priority: 30, enabled: true,
      created_at: now, updated_at: now,
    },
    {
      id: 'p-4', name: 'viewer-read-only', scope: 'user', applies_to: 'u-4',
      action: 'files.*', resource: '*', decision: 'allow', priority: 5, enabled: true,
      created_at: now, updated_at: now,
    },
    {
      id: 'p-5', name: 'security-team-audit', scope: 'team', applies_to: 'team-2',
      action: 'audit.*', resource: '*', decision: 'allow', priority: 15, enabled: true,
      time_restriction: { start: '09:00', end: '18:00', days: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri'] },
      created_at: now, updated_at: now,
    },
    {
      id: 'p-6', name: 'block-pii-in-logs', scope: 'global', applies_to: undefined,
      action: 'audit.write', resource: '*', decision: 'allow', priority: 25, enabled: true,
      pii_action: 'redact',
      created_at: now, updated_at: now,
    },
  ];
}

makeDemoPolicies.cache = null as PolicyRule[] | null;
(function() { makeDemoPolicies.cache = makeDemoPolicies(); })();

export async function fetchPolicies(): Promise<PolicyRule[]> {
  const data = await safeFetch<PolicyRule[] | { policies?: PolicyRule[] }>(`${BASE}/api/policies`, []);
  if (Array.isArray(data)) return data.length ? data : makeDemoPolicies.cache!;
  const cast = data as { policies?: PolicyRule[] };
  return cast.policies?.length ? cast.policies : makeDemoPolicies.cache!;
}

export async function createPolicy(rule: Omit<PolicyRule, 'id' | 'created_at' | 'updated_at'>): Promise<PolicyRule> {
  const res = await fetch(`${BASE}/api/policies`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(rule),
  });
  const now = new Date().toISOString();
  if (!res.ok) {
    const newRule: PolicyRule = { ...rule, id: `p-${Date.now()}`, created_at: now, updated_at: now };
    makeDemoPolicies.cache = [...(makeDemoPolicies.cache ?? []), newRule];
    return newRule;
  }
  return res.json();
}

export async function updatePolicy(id: string, data: Partial<PolicyRule>): Promise<void> {
  await fetch(`${BASE}/api/policies/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (makeDemoPolicies.cache) {
    makeDemoPolicies.cache = makeDemoPolicies.cache.map(p =>
      p.id === id ? { ...p, ...data, updated_at: new Date().toISOString() } : p
    );
  }
}

export async function deletePolicy(id: string): Promise<void> {
  await fetch(`${BASE}/api/policies/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (makeDemoPolicies.cache) {
    makeDemoPolicies.cache = makeDemoPolicies.cache.filter(p => p.id !== id);
  }
}

export async function testPolicy(req: PolicyTestRequest): Promise<PolicyTestResult> {
  const data = await safeFetch<PolicyTestResult>(`${BASE}/api/policies/test`, null as unknown as PolicyTestResult);
  if (data) return data;
  // Local simulation against demo cache
  const rules = (makeDemoPolicies.cache ?? [])
    .filter(r => r.enabled)
    .sort((a, b) => b.priority - a.priority);
  for (const rule of rules) {
    const actionMatch = rule.action === req.action || rule.action.endsWith('.*') && req.action.startsWith(rule.action.slice(0, -2));
    const resourceMatch = rule.resource === '*' || rule.resource === req.resource || req.resource.startsWith(rule.resource.replace('**', ''));
    if (actionMatch && resourceMatch) {
      return { decision: rule.decision, matched_rule: rule, all_matched: [rule], reason: `Matched rule: ${rule.name}` };
    }
  }
  return { decision: 'deny', matched_rule: undefined, all_matched: [], reason: 'No matching rule (default deny)' };
}

export async function exportPolicies(): Promise<string> {
  const policies = makeDemoPolicies.cache ?? [];
  return `version: "1"\npolicies:\n${policies.map(p =>
    `  - name: "${p.name}"\n    action: "${p.action}"\n    resource: "${p.resource}"\n    decision: ${p.decision}\n    priority: ${p.priority}`
  ).join('\n')}`;
}

// ── Thread persistence API ─────────────────────────────────────────────────
// These endpoints back the web UI's chat thread sidebar so threads survive
// browser clears, device changes, and server restarts.

// ChatThread mirrors the shape used by ChatView so the API layer stays type-safe.
export interface ChatThread {
  id: string;
  title: string;
  messages: unknown[];
  model: string;
  createdAt: string;
  updatedAt: string;
  archived: boolean;
  pinned: boolean;
  tags: string[];
  tokenCount: number;
  costTotal: number;
}

export async function fetchThreads(): Promise<ChatThread[]> {
  try {
    const res = await fetch(`${BASE}/api/threads`);
    if (!res.ok) return [];
    const data = await res.json() as { threads?: ChatThread[] };
    return data.threads ?? [];
  } catch {
    return [];
  }
}

export async function saveThread(thread: ChatThread): Promise<void> {
  try {
    await fetch(`${BASE}/api/threads`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(thread),
    });
  } catch {
    // Best-effort — localStorage remains the authoritative offline cache.
  }
}

export async function deleteThread(id: string): Promise<void> {
  try {
    await fetch(`${BASE}/api/threads/${encodeURIComponent(id)}`, { method: 'DELETE' });
  } catch {
    // Best-effort.
  }
}

// ── Heartbeat API ─────────────────────────────────────────────────────────────

export interface HeartbeatStatus {
  enabled: boolean;
  running: boolean;
  interval: string;
  last_run?: string;
  next_run?: string;
  last_result?: string;
  run_count: number;
}

export async function fetchHeartbeatStatus(): Promise<HeartbeatStatus | null> {
  return safeFetch<HeartbeatStatus | null>(`${BASE}/api/heartbeat`, null);
}

export async function toggleHeartbeat(enabled: boolean): Promise<boolean> {
  try {
    const res = await fetch(`${BASE}/api/heartbeat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    });
    const data = await res.json();
    return data.enabled ?? enabled;
  } catch {
    return enabled;
  }
}

export async function triggerHeartbeat(): Promise<string> {
  const res = await fetch(`${BASE}/api/heartbeat/run`, { method: 'POST' });
  const data = await res.json();
  return data.result || data.error || 'No response';
}
