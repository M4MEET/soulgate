import { useState, useEffect } from 'react';
import { Settings, Server, Key, Shield, Webhook, Plus, Trash2, Save, RefreshCw } from 'lucide-react';
import { fetchConfig, updateConfig, type HealthData, type ConfigData } from '../lib/api';
import toast from 'react-hot-toast';

interface Props { health: HealthData | null; }

const PROVIDERS = ['anthropic', 'openai', 'ollama', 'groq', 'together'];
const DECISIONS = ['allow', 'deny', 'require_approval'];

function Section({ title, icon: Icon, children }: { title: string; icon: React.ElementType; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-zinc-700/40 bg-zinc-800/30 overflow-hidden">
      <div className="flex items-center gap-2 px-5 py-3.5 border-b border-zinc-700/40 bg-zinc-800/60">
        <Icon size={15} className="text-zinc-400" />
        <h3 className="text-sm font-semibold text-zinc-300">{title}</h3>
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

function SettingRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between py-2.5 border-b border-zinc-800/50 last:border-0">
      <span className="text-sm text-zinc-400 flex-shrink-0 mr-4">{label}</span>
      <div className="flex-1 flex justify-end">{children}</div>
    </div>
  );
}

export default function SettingsView({ health }: Props) {
  const [config, setConfig] = useState<ConfigData | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const data = await fetchConfig();
      if (data) {
        setConfig(data);
      } else {
        setConfig({
          provider: health?.provider || 'anthropic',
          model: health?.model || 'claude-opus-4-5',
          max_tokens: 4096,
          temperature: 0.7,
          max_turns: 30,
          timeout: '30s',
          webhooks: [],
          policies: [
            { name: 'allow-workspace-reads', action: 'files.read',  resource: './**', decision: 'allow', priority: 10 },
            { name: 'deny-parent-access',     action: 'files.*',    resource: '../**', decision: 'deny', priority: 20 },
          ],
        });
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [health]);

  const save = async () => {
    if (!config) return;
    setSaving(true);
    try {
      await updateConfig(config);
      toast.success('Settings saved');
    } catch {
      toast.error('Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  const addWebhook = () => {
    setConfig(c => c ? { ...c, webhooks: [...(c.webhooks || []), { name: '', url: '', events: [] }] } : c);
  };

  const removeWebhook = (i: number) => {
    setConfig(c => c ? { ...c, webhooks: c.webhooks?.filter((_, idx) => idx !== i) } : c);
  };

  const addPolicy = () => {
    setConfig(c => c ? {
      ...c,
      policies: [...(c.policies || []), { name: '', action: 'files.read', resource: './**', decision: 'allow', priority: 10 }],
    } : c);
  };

  const removePolicy = (i: number) => {
    setConfig(c => c ? { ...c, policies: c.policies?.filter((_, idx) => idx !== i) } : c);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-500 gap-2">
        <RefreshCw size={16} className="animate-spin" />
        Loading settings…
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Settings</h2>
          <p className="text-sm text-zinc-500">Configure SoulGate</p>
        </div>
        <button
          onClick={save}
          disabled={saving}
          className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors disabled:opacity-60"
        >
          {saving ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
          {saving ? 'Saving…' : 'Save Changes'}
        </button>
      </div>

      <div className="space-y-5">
        {/* Model / Provider */}
        <Section title="Model & Provider" icon={Server}>
          <SettingRow label="Provider">
            <select
              value={config?.provider || ''}
              onChange={e => setConfig(c => c ? { ...c, provider: e.target.value } : c)}
              className="px-3 py-1.5 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60"
            >
              {PROVIDERS.map(p => <option key={p} value={p}>{p}</option>)}
            </select>
          </SettingRow>
          <SettingRow label="Model">
            <input
              value={config?.model || ''}
              onChange={e => setConfig(c => c ? { ...c, model: e.target.value } : c)}
              className="px-3 py-1.5 rounded-lg bg-zinc-800 border border-zinc-700 text-sm font-mono text-zinc-200 focus:outline-none focus:border-indigo-500/60 w-48"
            />
          </SettingRow>
          <SettingRow label="Status">
            <span className={`text-sm font-medium ${
              health?.status === 'healthy' ? 'text-emerald-400' :
              health?.status === 'degraded' ? 'text-amber-400' : 'text-zinc-500'
            }`}>
              {health?.status || '—'}
            </span>
          </SettingRow>
          <SettingRow label="Uptime">
            <span className="text-sm text-zinc-300 font-mono">{health?.uptime || '—'}</span>
          </SettingRow>
        </Section>

        {/* Execution limits */}
        <Section title="Execution Limits" icon={Settings}>
          <SettingRow label="Max Tokens">
            <input
              type="number"
              value={config?.max_tokens || ''}
              onChange={e => setConfig(c => c ? { ...c, max_tokens: Number(e.target.value) } : c)}
              className="px-3 py-1.5 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60 w-28"
            />
          </SettingRow>
          <SettingRow label="Temperature">
            <div className="flex items-center gap-2">
              <input
                type="range"
                min="0"
                max="2"
                step="0.1"
                value={config?.temperature ?? 0.7}
                onChange={e => setConfig(c => c ? { ...c, temperature: Number(e.target.value) } : c)}
                className="w-28 accent-indigo-500"
              />
              <span className="text-sm text-zinc-300 w-8 text-right">{config?.temperature?.toFixed(1) ?? '0.7'}</span>
            </div>
          </SettingRow>
          <SettingRow label="Max Turns">
            <input
              type="number"
              value={config?.max_turns || ''}
              onChange={e => setConfig(c => c ? { ...c, max_turns: Number(e.target.value) } : c)}
              className="px-3 py-1.5 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 focus:outline-none focus:border-indigo-500/60 w-28"
            />
          </SettingRow>
          <SettingRow label="Timeout">
            <input
              value={config?.timeout || ''}
              onChange={e => setConfig(c => c ? { ...c, timeout: e.target.value } : c)}
              placeholder="30s"
              className="px-3 py-1.5 rounded-lg bg-zinc-800 border border-zinc-700 text-sm font-mono text-zinc-200 focus:outline-none focus:border-indigo-500/60 w-28"
            />
          </SettingRow>
        </Section>

        {/* Runtime info */}
        <Section title="Runtime" icon={Shield}>
          <SettingRow label="Memory allocated">
            <span className="text-sm text-zinc-300 font-mono">{health?.memory.alloc_mb || 0} MB</span>
          </SettingRow>
          <SettingRow label="System memory">
            <span className="text-sm text-zinc-300 font-mono">{health?.memory.sys_mb || 0} MB</span>
          </SettingRow>
          <SettingRow label="Goroutines">
            <span className="text-sm text-zinc-300 font-mono">{health?.memory.goroutines || 0}</span>
          </SettingRow>
          <SettingRow label="GC runs">
            <span className="text-sm text-zinc-300 font-mono">{health?.memory.num_gc || 0}</span>
          </SettingRow>
        </Section>

        {/* API Endpoints */}
        <Section title="API Endpoints" icon={Key}>
          {[
            ['Chat',     'POST /api/chat'],
            ['Health',   'GET /api/health'],
            ['Status',   'GET /api/status'],
            ['Sessions', 'GET /api/sessions'],
            ['WebSocket','ws:///ws'],
            ['Webhooks', 'POST /webhook/{name}'],
            ['Agents',   'GET /api/agents'],
            ['Memory',   'GET /api/memory'],
            ['Tools',    'GET /api/tools'],
            ['Audit',    'GET /api/audit'],
          ].map(([label, endpoint]) => (
            <SettingRow key={label} label={label}>
              <code className="text-xs font-mono bg-zinc-900/60 text-indigo-300 px-2 py-1 rounded-lg">{endpoint}</code>
            </SettingRow>
          ))}
        </Section>

        {/* Policy rules */}
        <Section title="Policy Rules" icon={Shield}>
          <div className="space-y-2 mb-3">
            {config?.policies?.map((policy, i) => (
              <div key={i} className="flex items-center gap-2 p-3 rounded-lg bg-zinc-900/40 border border-zinc-700/40">
                <div className="flex-1 grid grid-cols-2 md:grid-cols-4 gap-2">
                  <input
                    value={policy.name}
                    onChange={e => setConfig(c => c ? { ...c, policies: c.policies?.map((p, j) => j === i ? { ...p, name: e.target.value } : p) } : c)}
                    placeholder="Name"
                    className="px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-xs text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
                  />
                  <input
                    value={policy.action}
                    onChange={e => setConfig(c => c ? { ...c, policies: c.policies?.map((p, j) => j === i ? { ...p, action: e.target.value } : p) } : c)}
                    placeholder="Action"
                    className="px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-xs font-mono text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
                  />
                  <input
                    value={policy.resource}
                    onChange={e => setConfig(c => c ? { ...c, policies: c.policies?.map((p, j) => j === i ? { ...p, resource: e.target.value } : p) } : c)}
                    placeholder="Resource"
                    className="px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-xs font-mono text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
                  />
                  <select
                    value={policy.decision}
                    onChange={e => setConfig(c => c ? { ...c, policies: c.policies?.map((p, j) => j === i ? { ...p, decision: e.target.value } : p) } : c)}
                    className="px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-xs text-zinc-200 focus:outline-none focus:border-indigo-500/60"
                  >
                    {DECISIONS.map(d => <option key={d} value={d}>{d}</option>)}
                  </select>
                </div>
                <button onClick={() => removePolicy(i)} className="p-1.5 rounded hover:bg-red-500/10 text-zinc-600 hover:text-red-400 transition-colors flex-shrink-0">
                  <Trash2 size={13} />
                </button>
              </div>
            ))}
          </div>
          <button
            onClick={addPolicy}
            className="flex items-center gap-1.5 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            <Plus size={13} />
            Add rule
          </button>
        </Section>

        {/* Webhooks */}
        <Section title="Webhooks" icon={Webhook}>
          <div className="space-y-2 mb-3">
            {config?.webhooks?.map((wh, i) => (
              <div key={i} className="flex items-center gap-2 p-3 rounded-lg bg-zinc-900/40 border border-zinc-700/40">
                <input
                  value={wh.name}
                  onChange={e => setConfig(c => c ? { ...c, webhooks: c.webhooks?.map((w, j) => j === i ? { ...w, name: e.target.value } : w) } : c)}
                  placeholder="Name"
                  className="w-32 px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-xs text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
                />
                <input
                  value={wh.url}
                  onChange={e => setConfig(c => c ? { ...c, webhooks: c.webhooks?.map((w, j) => j === i ? { ...w, url: e.target.value } : w) } : c)}
                  placeholder="https://…"
                  className="flex-1 px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-xs font-mono text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
                />
                <button onClick={() => removeWebhook(i)} className="p-1.5 rounded hover:bg-red-500/10 text-zinc-600 hover:text-red-400 transition-colors">
                  <Trash2 size={13} />
                </button>
              </div>
            ))}
            {config?.webhooks?.length === 0 && (
              <p className="text-xs text-zinc-600">No webhooks configured.</p>
            )}
          </div>
          <button
            onClick={addWebhook}
            className="flex items-center gap-1.5 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            <Plus size={13} />
            Add webhook
          </button>
        </Section>
      </div>
    </div>
  );
}
