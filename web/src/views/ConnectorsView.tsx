import { useState, useEffect, useCallback } from 'react';
import {
  Plug, CheckCircle, XCircle, ExternalLink, ChevronDown, ChevronRight,
  RefreshCw, Power, Search, Package, Globe,
} from 'lucide-react';
import toast from 'react-hot-toast';
import { fetchConnectors, spawnConnector, disconnectConnector, type ConnectorClient } from '../lib/api';

interface ConnectorDef {
  id: string;
  name: string;
  description: string;
  icon: string;
  setupCmd: string;
  docsUrl?: string;
  fields: { key: string; label: string; type: 'text' | 'password' }[];
  installed?: boolean; // whether it's a built-in or installed from hub
}

interface ConnectorState extends ConnectorDef {
  connected: boolean;
  status?: 'active' | 'error' | 'pending';
  liveClients: ConnectorClient[];
  sessionCount: number;
}

const CONNECTOR_DEFS: ConnectorDef[] = [
  {
    id: 'telegram', name: 'Telegram', description: 'Bot API via Telegram gateway', icon: '✈',
    setupCmd: 'soulgate connector telegram',
    fields: [{ key: 'token', label: 'Bot Token', type: 'password' }],
    docsUrl: 'https://core.telegram.org/bots/api',
    installed: true,
  },
  {
    id: 'discord', name: 'Discord', description: 'Discord bot integration', icon: '🎮',
    setupCmd: 'soulgate connector discord',
    fields: [{ key: 'token', label: 'Bot Token', type: 'password' }, { key: 'guild_id', label: 'Guild ID', type: 'text' }],
    docsUrl: 'https://discord.com/developers/docs',
    installed: true,
  },
  {
    id: 'slack', name: 'Slack', description: 'Slack app with socket mode', icon: '💬',
    setupCmd: 'soulgate connector slack',
    fields: [{ key: 'app_token', label: 'App Token', type: 'password' }, { key: 'bot_token', label: 'Bot Token', type: 'password' }],
    docsUrl: 'https://api.slack.com',
    installed: true,
  },
  {
    id: 'whatsapp', name: 'WhatsApp', description: 'WhatsApp Business API', icon: '📱',
    setupCmd: 'soulgate connector whatsapp',
    fields: [{ key: 'phone_id', label: 'Phone ID', type: 'text' }, { key: 'token', label: 'Access Token', type: 'password' }],
    docsUrl: 'https://developers.facebook.com/docs/whatsapp',
    installed: true,
  },
  {
    id: 'signal', name: 'Signal', description: 'Signal messenger via signal-cli', icon: '🔒',
    setupCmd: 'soulgate connector signal',
    fields: [{ key: 'phone', label: 'Phone Number', type: 'text' }],
    installed: true,
  },
  {
    id: 'teams', name: 'Microsoft Teams', description: 'Teams bot via Azure Bot Service', icon: '🏢',
    setupCmd: 'soulgate connector teams',
    fields: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'app_password', label: 'App Password', type: 'password' }],
    installed: true,
  },
  {
    id: 'matrix', name: 'Matrix', description: 'Matrix/Element bridge', icon: '🔷',
    setupCmd: 'soulgate connector matrix',
    fields: [{ key: 'homeserver', label: 'Homeserver', type: 'text' }, { key: 'token', label: 'Access Token', type: 'password' }],
    installed: true,
  },
  {
    id: 'imessage', name: 'iMessage', description: 'iMessage via BlueBubbles', icon: '🍎',
    setupCmd: 'soulgate connector imessage',
    fields: [{ key: 'server', label: 'BlueBubbles Server', type: 'text' }, { key: 'password', label: 'Password', type: 'password' }],
    installed: true,
  },
  {
    id: 'irc', name: 'IRC', description: 'IRC bot integration', icon: '💻',
    setupCmd: 'soulgate connector irc',
    fields: [{ key: 'server', label: 'Server', type: 'text' }, { key: 'nick', label: 'Nickname', type: 'text' }],
    installed: true,
  },
  {
    id: 'twitch', name: 'Twitch', description: 'Twitch chat bot', icon: '🎮',
    setupCmd: 'soulgate connector twitch',
    fields: [{ key: 'channel', label: 'Channel', type: 'text' }, { key: 'token', label: 'OAuth Token', type: 'password' }],
    docsUrl: 'https://dev.twitch.tv',
    installed: true,
  },
  {
    id: 'nostr', name: 'Nostr', description: 'Nostr decentralized protocol', icon: '⚡',
    setupCmd: 'soulgate connector nostr',
    fields: [{ key: 'private_key', label: 'Private Key', type: 'password' }, { key: 'relay', label: 'Relay URL', type: 'text' }],
    installed: true,
  },
  {
    id: 'mattermost', name: 'Mattermost', description: 'Mattermost bot integration', icon: '🔷',
    setupCmd: 'soulgate connector mattermost',
    fields: [{ key: 'server', label: 'Server URL', type: 'text' }, { key: 'token', label: 'Bot Token', type: 'password' }],
    installed: true,
  },
  {
    id: 'feishu', name: 'Feishu / Lark', description: 'Feishu/Lark bot integration', icon: '🐦',
    setupCmd: 'soulgate connector feishu',
    fields: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'app_secret', label: 'App Secret', type: 'password' }],
    installed: true,
  },
];

function maskToken(value: string): string {
  if (value.length <= 10) return '****';
  return value.slice(0, 4) + '****' + value.slice(-4);
}

// ── Connector Card ─────────────────────────────────────────────────────────

function ConnectorCard({ connector, expanded, onToggle, onRefresh }: {
  connector: ConnectorState;
  expanded: boolean;
  onToggle: () => void;
  onRefresh: () => void;
}) {
  const [values, setValues] = useState<Record<string, string>>({});
  const [connecting, setConnecting] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);

  const connect = async () => {
    const hasValue = connector.fields.some(f => values[f.key]?.trim());
    if (!hasValue) {
      toast.error(`Enter credentials to connect ${connector.name}`);
      return;
    }
    setConnecting(true);
    try {
      const result = await spawnConnector(connector.id, values);
      if (result.error) {
        toast.error(`${connector.name}: ${result.error}`);
      } else {
        toast.success(`${connector.name} connector starting...`);
        setValues({});
        setTimeout(onRefresh, 2000);
        setTimeout(onRefresh, 5000);
      }
    } catch (err) {
      toast.error(`Failed to connect ${connector.name}: ${err}`);
    } finally {
      setConnecting(false);
    }
  };

  const disconnect = async () => {
    setDisconnecting(true);
    try {
      const result = await disconnectConnector(connector.id);
      if (result.error) {
        toast.error(`${connector.name}: ${result.error}`);
      } else {
        toast.success(`${connector.name} disconnected`);
        setTimeout(onRefresh, 1000);
      }
    } catch (err) {
      toast.error(`Failed to disconnect ${connector.name}: ${err}`);
    } finally {
      setDisconnecting(false);
    }
  };

  return (
    <div className={`rounded-xl border transition-all ${
      connector.connected
        ? 'bg-emerald-500/5 border-emerald-500/20'
        : 'bg-zinc-800/30 border-zinc-700/40 hover:border-zinc-600/60'
    }`}>
      {/* Header */}
      <div
        className="flex items-center gap-4 p-5 cursor-pointer"
        onClick={onToggle}
      >
        <div className={`w-10 h-10 rounded-xl border flex items-center justify-center text-xl flex-shrink-0 ${
          connector.connected
            ? 'bg-emerald-900/30 border-emerald-700/50'
            : 'bg-zinc-800 border-zinc-700/50'
        }`}>
          {connector.icon}
        </div>
        <div className="flex-1 min-w-0">
          <div className="font-semibold text-zinc-100 text-sm">{connector.name}</div>
          <div className="text-xs text-zinc-500 truncate">
            {connector.connected && connector.liveClients.length > 0
              ? connector.liveClients.map(c => {
                  const username = c.metadata?.bot_username;
                  return username ? `@${username}` : c.client_id;
                }).join(', ')
              : connector.description
            }
          </div>
        </div>
        <div className="flex items-center gap-2">
          {connector.connected ? (
            <div className="flex items-center gap-1.5 text-xs text-emerald-400">
              <CheckCircle size={14} />
              Connected
              {connector.sessionCount > 0 && (
                <span className="text-emerald-500/70 ml-1">
                  ({connector.sessionCount} session{connector.sessionCount !== 1 ? 's' : ''})
                </span>
              )}
            </div>
          ) : (
            <div className="flex items-center gap-1.5 text-xs text-zinc-600">
              <XCircle size={14} />
              Disconnected
            </div>
          )}
          {expanded ? <ChevronDown size={14} className="text-zinc-500" /> : <ChevronRight size={14} className="text-zinc-500" />}
        </div>
      </div>

      {/* Expanded panel */}
      {expanded && (
        <div className="px-5 pb-5 border-t border-zinc-700/40 pt-4">
          {/* Live connection info */}
          {connector.connected && connector.liveClients.length > 0 && (
            <div className="mb-4 space-y-2">
              {connector.liveClients.map(client => (
                <div key={client.client_id} className="bg-emerald-900/10 border border-emerald-800/30 rounded-lg px-3 py-2.5">
                  <div className="text-xs text-emerald-400 font-medium mb-1.5">Live Connection</div>
                  <div className="space-y-1">
                    {client.metadata?.bot_username && (
                      <div className="flex items-center gap-2 text-xs">
                        <span className="text-zinc-500 w-20">Bot</span>
                        <span className="text-zinc-200 font-mono">@{client.metadata.bot_username}</span>
                      </div>
                    )}
                    {client.metadata?.bot_id && (
                      <div className="flex items-center gap-2 text-xs">
                        <span className="text-zinc-500 w-20">Bot ID</span>
                        <span className="text-zinc-300 font-mono">{client.metadata.bot_id}</span>
                      </div>
                    )}
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-zinc-500 w-20">Client ID</span>
                      <span className="text-zinc-400 font-mono">{maskToken(client.client_id)}</span>
                    </div>
                    {Object.entries(client.metadata || {}).map(([key, val]) => {
                      if (['channel', 'bot_username', 'bot_id'].includes(key)) return null;
                      return (
                        <div key={key} className="flex items-center gap-2 text-xs">
                          <span className="text-zinc-500 w-20">{key}</span>
                          <span className="text-zinc-300 font-mono">{val}</span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Action buttons for connected connectors */}
          {connector.connected && (
            <div className="flex gap-2 mb-4">
              <button
                onClick={disconnect}
                disabled={disconnecting}
                className="flex-1 flex items-center justify-center gap-2 py-2 rounded-lg bg-red-600/10 border border-red-500/20 text-red-400 text-xs font-medium hover:bg-red-600/20 transition-colors disabled:opacity-50"
              >
                {disconnecting ? (
                  <><span className="w-3 h-3 border-2 border-red-400/30 border-t-red-400 rounded-full animate-spin" />Disconnecting...</>
                ) : (
                  <><Power size={12} />Disconnect</>
                )}
              </button>
            </div>
          )}

          {/* Setup info */}
          <div className="mb-3">
            <div className="text-xs text-zinc-500 mb-1">CLI command</div>
            <code className="text-xs font-mono text-indigo-300 bg-zinc-900/60 px-3 py-1.5 rounded-lg block">
              {connector.setupCmd}
            </code>
          </div>

          {connector.docsUrl && (
            <a
              href={connector.docsUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-300 mb-3 transition-colors"
            >
              <ExternalLink size={11} />
              Documentation
            </a>
          )}

          {/* Config fields when not connected */}
          {!connector.connected && (
            <>
              <div className="space-y-2 mb-4">
                {connector.fields.map(field => (
                  <div key={field.key}>
                    <label className="text-xs text-zinc-500 block mb-1">{field.label}</label>
                    <input
                      type={field.type}
                      value={values[field.key] || ''}
                      onChange={e => setValues(v => ({ ...v, [field.key]: e.target.value }))}
                      placeholder={field.type === 'password' ? '••••••••' : `Enter ${field.label.toLowerCase()}`}
                      className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 placeholder-zinc-700 focus:outline-none focus:border-indigo-500/60"
                    />
                  </div>
                ))}
              </div>

              <div className="flex gap-2">
                <button
                  onClick={connect}
                  disabled={connecting}
                  className="flex-1 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium transition-colors disabled:opacity-60 flex items-center justify-center gap-2"
                >
                  {connecting ? (
                    <><span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />Connecting...</>
                  ) : (
                    <><Plug size={13} />Connect</>
                  )}
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}

// ── Hub Search ─────────────────────────────────────────────────────────────

function HubSearch() {
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);

  const hubUrl = 'https://github.com/M4MEET/soulgate-hub';

  const search = () => {
    if (!query.trim()) {
      window.open(hubUrl, '_blank');
      return;
    }
    setSearching(true);
    // Open GitHub search with the query scoped to the hub repo
    const searchUrl = `${hubUrl}/search?q=${encodeURIComponent(query)}&type=code`;
    window.open(searchUrl, '_blank');
    setTimeout(() => setSearching(false), 1000);
  };

  return (
    <div className="rounded-xl border border-dashed border-zinc-600/50 bg-zinc-900/20 p-5">
      <div className="flex items-center gap-3 mb-3">
        <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-500/20 to-violet-500/20 border border-indigo-500/20 flex items-center justify-center">
          <Package size={20} className="text-indigo-400" />
        </div>
        <div>
          <div className="text-sm font-semibold text-zinc-200">SoulGate Hub</div>
          <div className="text-xs text-zinc-500">Browse community connectors and plugins</div>
        </div>
      </div>

      <div className="flex gap-2 mb-3">
        <div className="flex-1 relative">
          <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-600" />
          <input
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') search(); }}
            placeholder="Search connectors in Hub..."
            className="w-full pl-8 pr-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
          />
        </div>
        <button
          onClick={search}
          disabled={searching}
          className="px-4 py-2 rounded-lg bg-indigo-600/80 hover:bg-indigo-500 text-white text-xs font-medium transition-colors disabled:opacity-60 flex items-center gap-1.5"
        >
          {searching ? (
            <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
          ) : (
            <Globe size={12} />
          )}
          Browse Hub
        </button>
      </div>

      <div className="flex flex-wrap gap-1.5">
        {['telegram', 'discord', 'slack', 'whatsapp', 'custom'].map(tag => (
          <button
            key={tag}
            onClick={() => { setQuery(tag); }}
            className="px-2 py-0.5 rounded text-[10px] bg-zinc-800 border border-zinc-700/50 text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-colors capitalize"
          >
            {tag}
          </button>
        ))}
      </div>

      <div className="mt-3 flex items-center gap-4 text-[10px] text-zinc-600">
        <a href={hubUrl} target="_blank" rel="noopener noreferrer" className="hover:text-zinc-400 transition-colors flex items-center gap-1">
          <ExternalLink size={9} /> View all on GitHub
        </a>
        <span>Community-built connectors work automatically with SoulGate</span>
      </div>
    </div>
  );
}

// ── Main View ──────────────────────────────────────────────────────────────

export default function ConnectorsView() {
  const [connectors, setConnectors] = useState<ConnectorState[]>(
    CONNECTOR_DEFS.map(def => ({ ...def, connected: false, liveClients: [], sessionCount: 0 }))
  );
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'connected' | 'disconnected'>('all');
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const data = await fetchConnectors();

      const channelMap = new Map<string, ConnectorClient[]>();
      for (const client of data.channels || []) {
        const ch = (client.channel || client.metadata?.channel || '').toLowerCase();
        if (!ch) continue;
        const existing = channelMap.get(ch) || [];
        existing.push(client);
        channelMap.set(ch, existing);
      }

      const spawnedSet = new Set<string>();
      for (const sc of data.spawned || []) {
        if (sc.status === 'running') spawnedSet.add(sc.type);
      }

      setConnectors(
        CONNECTOR_DEFS.map(def => {
          const liveClients = channelMap.get(def.id) || [];
          const sessionCount = data.sessions_by_channel?.[def.id] || 0;
          const isConnected = liveClients.length > 0 || spawnedSet.has(def.id);
          return {
            ...def,
            connected: isConnected,
            status: isConnected ? 'active' as const : undefined,
            liveClients,
            sessionCount,
          };
        })
      );
    } catch {
      // keep showing current state
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 5000);
    return () => clearInterval(interval);
  }, [refresh]);

  const connectedCount = connectors.filter(c => c.connected).length;

  const filtered = connectors.filter(c => {
    if (filter === 'connected') return c.connected;
    if (filter === 'disconnected') return !c.connected;
    return true;
  });

  // Sort: connected first
  const sorted = [...filtered].sort((a, b) => (b.connected ? 1 : 0) - (a.connected ? 1 : 0));

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="mb-5 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Connectors</h2>
          <p className="text-sm text-zinc-500">
            {connectedCount} connected · {connectors.length} installed
            {loading && ' · loading...'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {/* Filter tabs */}
          <div className="flex rounded-lg bg-zinc-800/50 border border-zinc-700/40 p-0.5">
            {(['all', 'connected', 'disconnected'] as const).map(f => (
              <button
                key={f}
                onClick={() => setFilter(f)}
                className={`px-3 py-1 rounded-md text-xs font-medium transition-colors capitalize ${
                  filter === f
                    ? 'bg-zinc-700 text-zinc-100'
                    : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                {f}
              </button>
            ))}
          </div>
          <button
            onClick={refresh}
            className="p-2 rounded-lg bg-zinc-800/50 border border-zinc-700/40 text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-colors"
            title="Refresh"
          >
            <RefreshCw size={14} />
          </button>
        </div>
      </div>

      {/* Hub search */}
      <div className="mb-5">
        <HubSearch />
      </div>

      {/* Connector grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {sorted.map(connector => (
          <ConnectorCard
            key={connector.id}
            connector={connector}
            expanded={expandedId === connector.id}
            onToggle={() => setExpandedId(prev => prev === connector.id ? null : connector.id)}
            onRefresh={refresh}
          />
        ))}
      </div>
    </div>
  );
}
