import { useState } from 'react';
import { Plug, CheckCircle, XCircle, ExternalLink, ChevronDown, ChevronRight } from 'lucide-react';
import toast from 'react-hot-toast';

interface Connector {
  id: string;
  name: string;
  description: string;
  icon: string;
  connected: boolean;
  status?: 'active' | 'error' | 'pending';
  setupCmd: string;
  docsUrl?: string;
  fields: { key: string; label: string; type: 'text' | 'password' }[];
}

const CONNECTORS: Connector[] = [
  {
    id: 'telegram', name: 'Telegram', description: 'Bot API via Telegram gateway', icon: '✈',
    connected: false, setupCmd: 'soulgate connector telegram',
    fields: [{ key: 'token', label: 'Bot Token', type: 'password' }],
    docsUrl: 'https://core.telegram.org/bots/api',
  },
  {
    id: 'discord', name: 'Discord', description: 'Discord bot integration', icon: '🎮',
    connected: false, setupCmd: 'soulgate connector discord',
    fields: [{ key: 'token', label: 'Bot Token', type: 'password' }, { key: 'guild_id', label: 'Guild ID', type: 'text' }],
    docsUrl: 'https://discord.com/developers/docs',
  },
  {
    id: 'slack', name: 'Slack', description: 'Slack app with socket mode', icon: '💬',
    connected: false, setupCmd: 'soulgate connector slack',
    fields: [{ key: 'app_token', label: 'App Token', type: 'password' }, { key: 'bot_token', label: 'Bot Token', type: 'password' }],
    docsUrl: 'https://api.slack.com',
  },
  {
    id: 'whatsapp', name: 'WhatsApp', description: 'WhatsApp Business API', icon: '📱',
    connected: false, setupCmd: 'soulgate connector whatsapp',
    fields: [{ key: 'phone_id', label: 'Phone ID', type: 'text' }, { key: 'token', label: 'Access Token', type: 'password' }],
    docsUrl: 'https://developers.facebook.com/docs/whatsapp',
  },
  {
    id: 'signal', name: 'Signal', description: 'Signal messenger via signal-cli', icon: '🔒',
    connected: false, setupCmd: 'soulgate connector signal',
    fields: [{ key: 'phone', label: 'Phone Number', type: 'text' }],
  },
  {
    id: 'teams', name: 'Microsoft Teams', description: 'Teams bot via Azure Bot Service', icon: '🏢',
    connected: false, setupCmd: 'soulgate connector teams',
    fields: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'app_password', label: 'App Password', type: 'password' }],
  },
  {
    id: 'matrix', name: 'Matrix', description: 'Matrix/Element bridge', icon: '🔷',
    connected: false, setupCmd: 'soulgate connector matrix',
    fields: [{ key: 'homeserver', label: 'Homeserver', type: 'text' }, { key: 'token', label: 'Access Token', type: 'password' }],
  },
  {
    id: 'imessage', name: 'iMessage', description: 'iMessage via BlueBubbles', icon: '🍎',
    connected: false, setupCmd: 'soulgate connector imessage',
    fields: [{ key: 'server', label: 'BlueBubbles Server', type: 'text' }, { key: 'password', label: 'Password', type: 'password' }],
  },
  {
    id: 'irc', name: 'IRC', description: 'IRC bot integration', icon: '💻',
    connected: false, setupCmd: 'soulgate connector irc',
    fields: [{ key: 'server', label: 'Server', type: 'text' }, { key: 'nick', label: 'Nickname', type: 'text' }],
  },
  {
    id: 'twitch', name: 'Twitch', description: 'Twitch chat bot', icon: '🎮',
    connected: false, setupCmd: 'soulgate connector twitch',
    fields: [{ key: 'channel', label: 'Channel', type: 'text' }, { key: 'token', label: 'OAuth Token', type: 'password' }],
    docsUrl: 'https://dev.twitch.tv',
  },
  {
    id: 'nostr', name: 'Nostr', description: 'Nostr decentralized protocol', icon: '⚡',
    connected: false, setupCmd: 'soulgate connector nostr',
    fields: [{ key: 'private_key', label: 'Private Key', type: 'password' }, { key: 'relay', label: 'Relay URL', type: 'text' }],
  },
  {
    id: 'mattermost', name: 'Mattermost', description: 'Mattermost bot integration', icon: '🔷',
    connected: false, setupCmd: 'soulgate connector mattermost',
    fields: [{ key: 'server', label: 'Server URL', type: 'text' }, { key: 'token', label: 'Bot Token', type: 'password' }],
  },
  {
    id: 'feishu', name: 'Feishu / Lark', description: 'Feishu/Lark bot integration', icon: '🐦',
    connected: false, setupCmd: 'soulgate connector feishu',
    fields: [{ key: 'app_id', label: 'App ID', type: 'text' }, { key: 'app_secret', label: 'App Secret', type: 'password' }],
  },
];

function ConnectorCard({ connector }: { connector: Connector }) {
  const [expanded, setExpanded] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [connecting, setConnecting] = useState(false);

  const connect = async () => {
    setConnecting(true);
    await new Promise(r => setTimeout(r, 1200));
    setConnecting(false);
    toast(`${connector.name}: run the setup command in your terminal to connect.`, { icon: 'i' });
    setExpanded(false);
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
        <div className="w-10 h-10 rounded-xl bg-zinc-800 border border-zinc-700/50 flex items-center justify-center text-xl flex-shrink-0">
          {connector.icon}
        </div>
        <div className="flex-1 min-w-0">
          <div className="font-semibold text-zinc-100 text-sm">{connector.name}</div>
          <div className="text-xs text-zinc-500 truncate">{connector.description}</div>
        </div>
        <div className="flex items-center gap-2">
          {connector.connected ? (
            <div className="flex items-center gap-1.5 text-xs text-emerald-400">
              <CheckCircle size={14} />
              Connected
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

      {/* Setup panel */}
      {expanded && (
        <div className="px-5 pb-5 border-t border-zinc-700/40 pt-4">
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
              <><span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />Connecting…</>
            ) : (
              <><Plug size={13} />Connect</>
            )}
          </button>
        </div>
      )}
    </div>
  );
}

export default function ConnectorsView() {
  const connected = CONNECTORS.filter(c => c.connected).length;

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="mb-6">
        <h2 className="text-lg font-bold text-zinc-100">Connectors</h2>
        <p className="text-sm text-zinc-500">{connected} connected · {CONNECTORS.length} available</p>
      </div>

      {/* Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {CONNECTORS.map(connector => (
          <ConnectorCard key={connector.id} connector={connector} />
        ))}
      </div>
    </div>
  );
}
