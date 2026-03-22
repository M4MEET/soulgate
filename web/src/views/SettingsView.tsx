import type { HealthData } from '../lib/api';
import { Server, Key, Shield, FileText } from 'lucide-react';

interface Props { health: HealthData | null; }

export default function SettingsView({ health }: Props) {
  return (
    <div className="settings-view">
      <div className="settings-grid">
        <div className="settings-card">
          <div className="settings-card-header"><Server size={16} /> Configuration</div>
          <div className="settings-rows">
            <div className="settings-row"><span>Provider</span><span className="settings-val">{health?.provider || '--'}</span></div>
            <div className="settings-row"><span>Model</span><span className="settings-val">{health?.model || '--'}</span></div>
            <div className="settings-row"><span>Status</span><span className={`settings-val badge-${health?.status}`}>{health?.status || '--'}</span></div>
            <div className="settings-row"><span>Uptime</span><span className="settings-val">{health?.uptime || '--'}</span></div>
          </div>
        </div>

        <div className="settings-card">
          <div className="settings-card-header"><Key size={16} /> API Endpoints</div>
          <div className="settings-rows">
            <div className="settings-row"><span>Chat</span><code>POST /api/chat</code></div>
            <div className="settings-row"><span>Health</span><code>GET /api/health</code></div>
            <div className="settings-row"><span>Status</span><code>GET /api/status</code></div>
            <div className="settings-row"><span>Sessions</span><code>GET /api/sessions</code></div>
            <div className="settings-row"><span>WebSocket</span><code>ws://localhost:8080/ws</code></div>
            <div className="settings-row"><span>Webhooks</span><code>POST /webhook/&#123;name&#125;</code></div>
          </div>
        </div>

        <div className="settings-card">
          <div className="settings-card-header"><Shield size={16} /> Runtime</div>
          <div className="settings-rows">
            <div className="settings-row"><span>Memory</span><span className="settings-val">{health?.memory.alloc_mb || 0} MB</span></div>
            <div className="settings-row"><span>System</span><span className="settings-val">{health?.memory.sys_mb || 0} MB</span></div>
            <div className="settings-row"><span>Goroutines</span><span className="settings-val">{health?.memory.goroutines || 0}</span></div>
            <div className="settings-row"><span>GC Runs</span><span className="settings-val">{health?.memory.num_gc || 0}</span></div>
          </div>
        </div>

        <div className="settings-card">
          <div className="settings-card-header"><FileText size={16} /> Connectors</div>
          <div className="settings-rows">
            {['Telegram', 'Discord', 'Slack', 'WhatsApp', 'Signal', 'Teams', 'Matrix', 'iMessage', 'IRC', 'Twitch', 'Nostr', 'Mattermost', 'Feishu'].map(c => (
              <div className="settings-row" key={c}>
                <span>{c}</span>
                <code>soulgate connector {c.toLowerCase()}</code>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
