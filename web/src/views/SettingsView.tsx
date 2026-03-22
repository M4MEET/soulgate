import { useState, useEffect, useCallback } from 'react';
import {
  Settings, Server, Key, Shield, Webhook, Plus, Trash2, Save, RefreshCw,
  Check, X, Eye, EyeOff, Search, ChevronDown, Heart, Play
} from 'lucide-react';
import {
  fetchConfig, updateConfig, fetchProviders, fetchProviderModels, fetchAPIKeyStatus, saveAPIKey,
  fetchHeartbeatStatus, toggleHeartbeat, triggerHeartbeat,
  type HealthData, type ConfigData, type ModelInfo, type HeartbeatStatus,
} from '../lib/api';
import toast from 'react-hot-toast';

interface Props { health: HealthData | null; onConfigSaved?: () => void; }

const DECISIONS = ['allow', 'deny', 'require_approval'];

function Section({ title, icon: Icon, badge, children }: { title: string; icon: React.ElementType; badge?: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-zinc-700/40 bg-zinc-800/30">
      <div className="flex items-center gap-2 px-5 py-3.5 border-b border-zinc-700/40 bg-zinc-800/60">
        <Icon size={15} className="text-zinc-400" />
        <h3 className="text-sm font-semibold text-zinc-300">{title}</h3>
        {badge && <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-indigo-500/20 text-indigo-400">{badge}</span>}
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

// ── Searchable dropdown ──────────────────────────────────────────────────────

function SearchableSelect({ value, options, onChange, placeholder, loading }: {
  value: string;
  options: { id: string; label: string; detail?: string }[];
  onChange: (v: string) => void;
  placeholder?: string;
  loading?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');

  const filtered = search
    ? options.filter(o => o.id.toLowerCase().includes(search.toLowerCase()) || o.label.toLowerCase().includes(search.toLowerCase()))
    : options;

  const selectedLabel = options.find(o => o.id === value)?.label || value || placeholder || 'Select...';

  return (
    <div className="relative min-w-56">
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center justify-between gap-2 px-3 py-1.5 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-200 hover:border-zinc-600 transition-colors text-left"
      >
        <span className="truncate font-mono text-xs">{loading ? 'Loading...' : selectedLabel}</span>
        <ChevronDown size={12} className={`text-zinc-500 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && (
        <div className="absolute top-full mt-1 left-0 right-0 z-50 bg-zinc-900 border border-zinc-700 rounded-lg shadow-xl overflow-hidden max-h-64 flex flex-col">
          <div className="flex items-center gap-2 px-3 py-2 border-b border-zinc-800">
            <Search size={12} className="text-zinc-500" />
            <input
              autoFocus
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search..."
              className="flex-1 bg-transparent text-xs text-zinc-200 placeholder-zinc-600 outline-none"
            />
          </div>
          <div className="overflow-y-auto" style={{ scrollbarWidth: 'thin' }}>
            {filtered.length === 0 ? (
              <div className="px-3 py-3 text-xs text-zinc-600 text-center">No matches</div>
            ) : (
              filtered.map(o => (
                <button
                  key={o.id}
                  onClick={() => { onChange(o.id); setOpen(false); setSearch(''); }}
                  className={`w-full text-left px-3 py-2 text-xs hover:bg-zinc-800 transition-colors flex items-center justify-between ${
                    o.id === value ? 'text-indigo-400 bg-indigo-500/5' : 'text-zinc-300'
                  }`}
                >
                  <div className="min-w-0">
                    <div className="font-mono truncate">{o.label}</div>
                    {o.detail && <div className="text-[10px] text-zinc-600 truncate">{o.detail}</div>}
                  </div>
                  {o.id === value && <Check size={12} className="text-indigo-400 flex-shrink-0 ml-2" />}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// ── Add Key Dropdown (standalone, not clipped by parent overflow) ─────────────

function AddKeyDropdown({ providers, configuredKeys, onSelect }: {
  providers: string[];
  configuredKeys: Set<string>;
  onSelect: (p: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');

  const filtered = search
    ? providers.filter(p => p.toLowerCase().includes(search.toLowerCase()))
    : providers;

  return (
    <div className="relative">
      <button
        onClick={() => { setOpen(o => !o); setSearch(''); }}
        className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
      >
        <Plus size={12} />
        Add API key for provider...
        <ChevronDown size={11} className={`ml-1 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && (
        <div className="absolute top-full mt-1 left-0 z-50 bg-zinc-900 border border-zinc-700 rounded-lg shadow-2xl w-72 max-h-80 flex flex-col">
          <div className="flex items-center gap-2 px-3 py-2 border-b border-zinc-800 flex-shrink-0">
            <Search size={12} className="text-zinc-500" />
            <input
              autoFocus
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search providers..."
              className="flex-1 bg-transparent text-xs text-zinc-200 placeholder-zinc-600 outline-none"
            />
            <span className="text-[10px] text-zinc-600">{filtered.length}</span>
          </div>
          <div className="overflow-y-auto flex-1" style={{ scrollbarWidth: 'thin' }}>
            {filtered.length === 0 ? (
              <div className="px-3 py-4 text-xs text-zinc-600 text-center">No providers match "{search}"</div>
            ) : (
              filtered.map(p => (
                <button
                  key={p}
                  onClick={() => { onSelect(p); setOpen(false); setSearch(''); }}
                  className="w-full text-left px-3 py-2 text-xs hover:bg-zinc-800 transition-colors flex items-center justify-between"
                >
                  <span className="text-zinc-300 font-mono">{p}</span>
                  {configuredKeys.has(p) ? (
                    <span className="text-[10px] text-amber-400">replace</span>
                  ) : (
                    <span className="text-[10px] text-zinc-600">add key</span>
                  )}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// ── API Keys Management ──────────────────────────────────────────────────────

function APIKeysSection({ providers, keyStatus, currentProvider, onKeyStatusChange, onSelectProvider }: {
  providers: string[];
  keyStatus: Record<string, boolean>;
  currentProvider: string;
  onKeyStatusChange: (s: Record<string, boolean>) => void;
  onSelectProvider: (p: string) => void;
}) {
  const [addingFor, setAddingFor] = useState<string | null>(null);
  const [keyInput, setKeyInput] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [saving, setSaving] = useState(false);

  const configuredKeys = Object.entries(keyStatus).filter(([, has]) => has);
  const unconfiguredProviders = providers.filter(p => !keyStatus[p]);

  const handleSave = async (provider: string) => {
    if (!keyInput.trim()) return;
    setSaving(true);
    try {
      const result = await saveAPIKey(provider, keyInput.trim());
      if (result.error) {
        toast.error(result.error);
      } else {
        toast.success(`API key saved for ${provider}`);
        setKeyInput('');
        setAddingFor(null);
        setShowKey(false);
        onKeyStatusChange({ ...keyStatus, [provider]: true });
      }
    } catch {
      toast.error('Failed to save API key');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Section title="API Keys" icon={Key} badge={`${configuredKeys.length} saved`}>
      {/* Configured keys */}
      {configuredKeys.length > 0 ? (
        <div className="space-y-1.5 mb-4">
          {configuredKeys.map(([provider]) => (
            <div
              key={provider}
              className={`flex items-center gap-3 px-3 py-2.5 rounded-lg border transition-all ${
                provider === currentProvider
                  ? 'bg-indigo-500/5 border-indigo-500/20'
                  : 'bg-zinc-800/30 border-zinc-700/30 hover:border-zinc-600/50'
              }`}
            >
              <div className="w-2 h-2 rounded-full bg-emerald-400 flex-shrink-0" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm text-zinc-200 font-medium">{provider}</span>
                  {provider === currentProvider && (
                    <span className="text-[9px] px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-400 uppercase font-bold">active</span>
                  )}
                </div>
                <div className="text-[10px] text-zinc-600 font-mono">****...****</div>
              </div>
              <div className="flex items-center gap-1">
                {provider !== currentProvider && (
                  <button
                    onClick={() => onSelectProvider(provider)}
                    className="px-2 py-1 rounded text-[10px] text-zinc-500 hover:text-zinc-200 hover:bg-zinc-700/50 transition-colors"
                    title="Switch to this provider"
                  >
                    Use
                  </button>
                )}
                <button
                  onClick={() => { setAddingFor(provider); setKeyInput(''); setShowKey(false); }}
                  className="px-2 py-1 rounded text-[10px] text-zinc-500 hover:text-amber-400 hover:bg-amber-500/10 transition-colors"
                  title="Replace API key"
                >
                  Replace
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center py-6 text-zinc-600 gap-2 mb-4">
          <Key size={20} />
          <span className="text-xs">No API keys configured yet</span>
          <span className="text-[10px] text-zinc-700">Add a key below to get started</span>
        </div>
      )}

      {/* Add / Replace key form */}
      {addingFor ? (
        <div className="rounded-lg border border-indigo-500/20 bg-indigo-500/5 p-4 mb-3">
          <div className="flex items-center justify-between mb-3">
            <div className="text-sm text-zinc-200">
              {keyStatus[addingFor] ? 'Replace' : 'Add'} API key for <span className="font-semibold text-indigo-400">{addingFor}</span>
            </div>
            <button onClick={() => { setAddingFor(null); setKeyInput(''); }} className="text-zinc-500 hover:text-zinc-300">
              <X size={14} />
            </button>
          </div>
          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <input
                autoFocus
                type={showKey ? 'text' : 'password'}
                value={keyInput}
                onChange={e => setKeyInput(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') handleSave(addingFor); if (e.key === 'Escape') { setAddingFor(null); setKeyInput(''); } }}
                placeholder="Paste your API key here..."
                className="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-xs font-mono text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 pr-8"
              />
              <button
                onClick={() => setShowKey(s => !s)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-zinc-600 hover:text-zinc-400"
              >
                {showKey ? <EyeOff size={13} /> : <Eye size={13} />}
              </button>
            </div>
            <button
              onClick={() => handleSave(addingFor)}
              disabled={saving || !keyInput.trim()}
              className="px-3 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-colors disabled:opacity-40 flex items-center gap-1.5"
            >
              {saving ? <RefreshCw size={12} className="animate-spin" /> : <Save size={12} />}
              {keyStatus[addingFor] ? 'Overwrite' : 'Save'}
            </button>
          </div>
          <p className="text-[10px] text-zinc-600 mt-2">
            Saved to .soulgate/config.yml. Other provider keys are preserved.
            {keyStatus[addingFor] && ' This will overwrite the existing key.'}
          </p>
        </div>
      ) : (
        <AddKeyDropdown
          providers={[
            // Current provider first if it needs a key
            ...(currentProvider && !keyStatus[currentProvider] ? [currentProvider] : []),
            // Then unconfigured providers
            ...unconfiguredProviders.filter(p => p !== currentProvider),
            // Then configured ones (for replacing)
            ...configuredKeys.map(([p]) => p),
          ]}
          configuredKeys={new Set(configuredKeys.map(([p]) => p))}
          onSelect={p => { setAddingFor(p); setKeyInput(''); setShowKey(false); }}
        />
      )}
    </Section>
  );
}

// ── Main component ───────────────────────────────────────────────────────────

function HeartbeatSection() {
  const [status, setStatus] = useState<HeartbeatStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState(false);

  const load = useCallback(async () => {
    const data = await fetchHeartbeatStatus();
    setStatus(data);
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const toggle = async () => {
    if (!status) return;
    const newState = !status.enabled;
    const result = await toggleHeartbeat(newState);
    setStatus(s => s ? { ...s, enabled: result } : s);
    toast.success(result ? 'Heartbeat enabled' : 'Heartbeat disabled');
  };

  const runNow = async () => {
    setRunning(true);
    try {
      const result = await triggerHeartbeat();
      toast.success(result.length > 50 ? result.slice(0, 50) + '...' : result);
      load();
    } catch {
      toast.error('Heartbeat run failed');
    } finally {
      setRunning(false);
    }
  };

  return (
    <Section title="Heartbeat" icon={Heart}>
      <SettingRow label="Status">
        <button
          onClick={toggle}
          disabled={loading}
          className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
            status?.enabled
              ? 'bg-emerald-500/15 border border-emerald-500/30 text-emerald-400'
              : 'bg-zinc-800 border border-zinc-700 text-zinc-400 hover:text-zinc-200'
          }`}
        >
          <span className={`w-2 h-2 rounded-full ${status?.enabled ? 'bg-emerald-400 animate-pulse' : 'bg-zinc-600'}`} />
          {status?.enabled ? 'On' : 'Off'}
        </button>
      </SettingRow>
      <SettingRow label="Interval">
        <span className="text-sm text-zinc-300 font-mono">{status?.interval || '30m'}</span>
      </SettingRow>
      <SettingRow label="Last run">
        <span className="text-sm text-zinc-300 font-mono">
          {status?.last_run ? new Date(status.last_run).toLocaleString() : 'never'}
        </span>
      </SettingRow>
      <SettingRow label="Run count">
        <span className="text-sm text-zinc-300 font-mono">{status?.run_count ?? 0}</span>
      </SettingRow>
      <SettingRow label="Last result">
        <span className="text-sm text-zinc-400 max-w-xs truncate">
          {status?.last_result ? (status.last_result.length > 60 ? status.last_result.slice(0, 60) + '...' : status.last_result) : '—'}
        </span>
      </SettingRow>
      <div className="pt-2">
        <button
          onClick={runNow}
          disabled={running}
          className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-indigo-600/20 border border-indigo-500/30 text-indigo-400 text-xs font-medium hover:bg-indigo-600/30 transition-all disabled:opacity-50"
        >
          {running ? <RefreshCw size={12} className="animate-spin" /> : <Play size={12} />}
          {running ? 'Running...' : 'Run Heartbeat Now'}
        </button>
      </div>
    </Section>
  );
}

export default function SettingsView({ health, onConfigSaved }: Props) {
  const [config, setConfig] = useState<ConfigData | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Dynamic provider/model state
  const [providers, setProviders] = useState<string[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [loadingProviders, setLoadingProviders] = useState(false);
  const [loadingModels, setLoadingModels] = useState(false);

  // API key state
  const [keyStatus, setKeyStatus] = useState<Record<string, boolean>>({});

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [data, provs, keys] = await Promise.all([
        fetchConfig(),
        fetchProviders(),
        fetchAPIKeyStatus(),
      ]);
      if (data) {
        setConfig(data);
      } else {
        setConfig({
          provider: health?.provider || 'anthropic',
          model: health?.model || 'claude-sonnet-4-20250514',
          max_tokens: 4096,
          temperature: 0.7,
        });
      }
      setProviders(provs);
      setKeyStatus(keys);
    } finally {
      setLoading(false);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Load once on mount only — don't re-fetch on health changes
  // to avoid resetting user's unsaved edits.
  useEffect(() => { load(); }, [load]);

  // Fetch models when provider changes
  const loadModels = useCallback(async (provider: string) => {
    if (!provider) return;
    setLoadingModels(true);
    try {
      const m = await fetchProviderModels(provider);
      setModels(m);
    } finally {
      setLoadingModels(false);
    }
  }, []);

  useEffect(() => {
    if (config?.provider) {
      loadModels(config.provider);
    }
  }, [config?.provider, loadModels]);

  // Refresh providers from API
  const refreshProviders = async () => {
    setLoadingProviders(true);
    try {
      const provs = await fetchProviders();
      setProviders(provs);
      toast.success(`${provs.length} providers loaded`);
    } finally {
      setLoadingProviders(false);
    }
  };

  const save = async () => {
    if (!config) return;
    setSaving(true);
    try {
      await updateConfig(config);
      toast.success('Settings saved');
      onConfigSaved?.();
    } catch (err) {
      toast.error(`Failed to save: ${err instanceof Error ? err.message : 'unknown error'}`);
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
        Loading settings...
      </div>
    );
  }

  const currentProvider = config?.provider || '';

  // Build provider options with key status indicator
  const providerOptions = providers.map(p => ({
    id: p,
    label: p,
    detail: keyStatus[p] ? 'API key configured' : 'No API key',
  }));

  // Build model options
  const modelOptions = models.map(m => ({
    id: m.id,
    label: m.name || m.id,
    detail: m.description || (m.context_length ? `${(m.context_length / 1000).toFixed(0)}k context` : ''),
  }));

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
          {saving ? 'Saving...' : 'Save Changes'}
        </button>
      </div>

      <div className="space-y-5">
        {/* Provider & Model */}
        <Section title="Provider & Model" icon={Server} badge={`${providers.length} providers`}>
          <SettingRow label="Provider">
            <div className="flex items-center gap-2">
              <SearchableSelect
                value={currentProvider}
                options={providerOptions}
                onChange={v => setConfig(c => c ? { ...c, provider: v, model: '' } : c)}
                placeholder="Select provider..."
                loading={loadingProviders}
              />
              <button
                onClick={refreshProviders}
                disabled={loadingProviders}
                className="p-1.5 rounded-lg hover:bg-zinc-700/40 text-zinc-500 hover:text-zinc-300 transition-colors"
                title="Refresh provider list"
              >
                <RefreshCw size={13} className={loadingProviders ? 'animate-spin' : ''} />
              </button>
            </div>
          </SettingRow>

          <SettingRow label="Model">
            <SearchableSelect
              value={config?.model || ''}
              options={modelOptions}
              onChange={v => setConfig(c => c ? { ...c, model: v } : c)}
              placeholder={loadingModels ? 'Loading models...' : 'Select model...'}
              loading={loadingModels}
            />
          </SettingRow>

          <SettingRow label="Status">
            <span className={`text-sm font-medium ${
              health?.status === 'healthy' ? 'text-emerald-400' :
              health?.status === 'degraded' ? 'text-amber-400' : 'text-zinc-500'
            }`}>
              {health?.status || '---'}
            </span>
          </SettingRow>
          <SettingRow label="Uptime">
            <span className="text-sm text-zinc-300 font-mono">{health?.uptime || '---'}</span>
          </SettingRow>
        </Section>

        {/* API Keys */}
        <APIKeysSection
          providers={providers}
          keyStatus={keyStatus}
          currentProvider={currentProvider}
          onKeyStatusChange={setKeyStatus}
          onSelectProvider={p => setConfig(c => c ? { ...c, provider: p } : c)}
        />

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
                type="range" min="0" max="2" step="0.1"
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

        {/* Heartbeat */}
        <HeartbeatSection />

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
          <button onClick={addPolicy} className="flex items-center gap-1.5 text-xs text-zinc-500 hover:text-zinc-300 transition-colors">
            <Plus size={13} /> Add rule
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
                  placeholder="https://..."
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
          <button onClick={addWebhook} className="flex items-center gap-1.5 text-xs text-zinc-500 hover:text-zinc-300 transition-colors">
            <Plus size={13} /> Add webhook
          </button>
        </Section>
      </div>
    </div>
  );
}
