import { useState, useEffect, useCallback } from 'react';
import { Plug, CheckCircle, XCircle, ExternalLink, ChevronDown, ChevronRight, RefreshCw } from 'lucide-react';
import toast from 'react-hot-toast';
import { fetchConnectors, spawnConnector, type ConnectorClient } from '../lib/api';

interface ConnectorDef {
  id: string;
  name: string;
  description: string;
  icon: string;
  setupCmd: string;
  docsUrl?: string;
  fields: { key: string; label: string; type: 'text' | 'password' }[];
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
  },
  {
    id: 'discord', name: 'Discord', description: 'Discord bot integration', icon: '🎮',
    setupCmd: 'soulgate connector discord',
    fields: [{ key: 'token', label: 'Bot Token', type: 'password' }, { key: 'guild_id', label: 'Guild ID', type: 'text' }],
    docsUrl: 'https://discord.com/developers/docs',
  },
  {
    id: 'slack', name: 'Slack', description: 'Slack app with socket mode', icon: '💬',
    setupCmd: 'soulgate connector slack',
    fields: [{ key: 'app_token', label: 'App Token', type: 'password' }, { key: 'bot_token', label: 'Bot Token', type: 'password' }],
    docsUrl: 'https://api.slack.com',
  },
  {
    id: 'whatsapp', name: 'WhatsApp', description: 'WhatsApp Business API', icon: '📱',
    setupCmd: 'soulgate connector whatsapp',
    fields: [{ key: 'phone_id', label: 'Phone ID', type: 'text' }, { key: 'token', label: 'Access Token', type: 'password' }],
    docsUrl: 'https://developers.facebook.com/docs/whatsapp',
  },
  {
    id: 'signal', name: 'Signal', description: 'Signal messenger via signal-cli', icon: '🔒',
    setupCmd: 'soulgate connector signal',
    fields: [{ key: 'phone', label: 'Phone Number', type: 'text' }],
  },
  {
    id: 'teams', name: 'Microsoft Teams', description: 'Teams bot via Azure Bot Service', icon: '🏢',
    setupCmd: 'soulgate connector teams',
    fields: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'app_password', label: 'App Password', type: 'password' }],
  },
  {
    id: 'matrix', name: 'Matrix', description: 'Matrix/Element bridge', icon: '🔷',
    setupCmd: 'soulgate connector matrix',
    fields: [{ key: 'homeserver', label: 'Homeserver', type: 'text' }, { key: 'token', label: 'Access Token', type: 'password' }],
  },
  {
    id: 'imessage', name: 'iMessage', description: 'iMessage via BlueBubbles', icon: '🍎',
    setupCmd: 'soulgate connector imessage',
    fields: [{ key: 'server', label: 'BlueBubbles Server', type: 'text' }, { key: 'password', label: 'Password', type: 'password' }],
  },
  {
    id: 'irc', name: 'IRC', description: 'IRC bot integration', icon: '💻',
    setupCmd: 'soulgate connector irc',
    fields: [{ key: 'server', label: 'Server', type: 'text' }, { key: 'nick', label: 'Nickname', type: 'text' }],
  },
  {
    id: 'twitch', name: 'Twitch', description: 'Twitch chat bot', icon: '🎮',
    setupCmd: 'soulgate connector twitch',
    fields: [{ key: 'channel', label: 'Channel', type: 'text' }, { key: 'token', label: 'OAuth Token', type: 'password' }],
    docsUrl: 'https://dev.twitch.tv',
  },
  {
    id: 'nostr', name: 'Nostr', description: 'Nostr decentralized protocol', icon: '⚡',
    setupCmd: 'soulgate connector nostr',
    fields: [{ key: 'private_key', label: 'Private Key', type: 'password' }, { key: 'relay', label: 'Relay URL', type: 'text' }],
  },
  {
    id: 'mattermost', name: 'Mattermost', description: 'Mattermost bot integration', icon: '🔷',
    setupCmd: 'soulgate connector mattermost',
    fields: [{ key: 'server', label: 'Server URL', type: 'text' }, { key: 'token', label: 'Bot Token', type: 'password' }],
  },
  {
    id: 'feishu', name: 'Feishu / Lark', description: 'Feishu/Lark bot integration', icon: '🐦',
    setupCmd: 'soulgate connector feishu',
    fields: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'app_secret', label: 'App Secret', type: 'password' }],
  },
];

/** Mask a token/key for display: show first 4 and last 4 chars */
function maskToken(value: string): string {
  if (value.length <= 10) return '****';
  return value.slice(0, 4) + '****' + value.slice(-4);
}

function ConnectorCard({ connector, onRefresh }: { connector: ConnectorState; onRefresh: () => void }) {
  const [expanded, setExpanded] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [connecting, setConnecting] = useState(false);

  const connect = async () => {
    // Check that at least one field is filled
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
        setExpanded(false);
        setValues({});
        // Poll quickly to pick up the new connection
        setTimeout(onRefresh, 2000);
        setTimeout(onRefresh, 5000);
      }
    } catch (err) {
      toast.error(`Failed to connect ${connector.name}: ${err}`);
    } finally {
      setConnecting(false);
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
        onClick={() => setExpanded(e => !e)}
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
          {/* Live connection info when connected */}
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
                    {/* Show any extra metadata keys */}
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

          <div className="mb-3">
            <div className="text-xs text-zinc-500 mb-1">Setup command</div>
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

          {/* Only show config fields when not connected */}
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

              <button
                onClick={connect}
                disabled={connecting}
                className="w-full py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium transition-colors disabled:opacity-60 flex items-center justify-center gap-2"
              >
                {connecting ? (
                  <><span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />Connecting...</>
                ) : (
                  <><Plug size={13} />Connect</>
                )}
              </button>
            </>
          )}
        </div>
      )}
    </div>
  );
}

export default function ConnectorsView() {
  const [connectors, setConnectors] = useState<ConnectorState[]>(
    CONNECTOR_DEFS.map(def => ({ ...def, connected: false, liveClients: [], sessionCount: 0 }))
  );
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const data = await fetchConnectors();

      // Build a map: channel name -> list of live clients
      const channelMap = new Map<string, ConnectorClient[]>();
      for (const client of data.channels || []) {
        const ch = (client.channel || client.metadata?.channel || '').toLowerCase();
        if (!ch) continue;
        const existing = channelMap.get(ch) || [];
        existing.push(client);
        channelMap.set(ch, existing);
      }

      // Also check spawned processes (HTTP-based connectors)
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
      // Silently fail — keep showing disconnected state
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    // Poll every 5 seconds to pick up new/dropped connections
    const interval = setInterval(refresh, 5000);
    return () => clearInterval(interval);
  }, [refresh]);

  const connectedCount = connectors.filter(c => c.connected).length;

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Connectors</h2>
          <p className="text-sm text-zinc-500">
            {connectedCount} connected · {connectors.length} available
            {loading && ' · loading...'}
          </p>
        </div>
        <button
          onClick={refresh}
          className="p-2 rounded-lg bg-zinc-800/50 border border-zinc-700/40 text-zinc-400 hover:text-zinc-200 hover:border-zinc-600 transition-colors"
          title="Refresh connector status"
        >
          <RefreshCw size={14} />
        </button>
      </div>

      {/* Connected first, then disconnected */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {[...connectors].sort((a, b) => (b.connected ? 1 : 0) - (a.connected ? 1 : 0)).map(connector => (
          <ConnectorCard key={connector.id} connector={connector} onRefresh={refresh} />
        ))}
      </div>
    </div>
  );
}
