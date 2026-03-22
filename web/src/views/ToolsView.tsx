import { useState, useEffect, useCallback } from 'react';
import {
  Wrench, Search, Play, ChevronDown, ChevronRight, Activity, X,
  Package, Download, Trash2, Star, ExternalLink, RefreshCw,
} from 'lucide-react';
import {
  fetchTools, tryTool, hubSearch, hubInstall, hubUninstall, hubInstalled,
  type ToolData, type HubItem, type InstalledHubItem,
} from '../lib/api';
import toast from 'react-hot-toast';

const CATEGORY_COLORS: Record<string, string> = {
  filesystem: 'text-sky-400 bg-sky-500/10',
  execution:  'text-amber-400 bg-amber-500/10',
  network:    'text-emerald-400 bg-emerald-500/10',
  memory:     'text-violet-400 bg-violet-500/10',
  security:   'text-red-400 bg-red-500/10',
  audit:      'text-orange-400 bg-orange-500/10',
  agents:     'text-indigo-400 bg-indigo-500/10',
  hub:        'text-fuchsia-400 bg-fuchsia-500/10',
  default:    'text-zinc-400 bg-zinc-500/10',
};

// ── Tool Card ──────────────────────────────────────────────────────────────

function ToolCard({ tool, onClick }: { tool: ToolData; onClick: () => void }) {
  const cat = tool.category || 'default';
  const colorClass = CATEGORY_COLORS[cat] || CATEGORY_COLORS.default;
  const [name, action] = tool.name.includes('.') ? tool.name.split('.') : [tool.name, ''];

  return (
    <button
      onClick={onClick}
      className="p-4 rounded-xl bg-zinc-800/40 border border-zinc-700/40 hover:border-zinc-600/60 hover:bg-zinc-800/60 text-left transition-all group"
    >
      <div className="flex items-start justify-between gap-2 mb-2">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg bg-zinc-800 border border-zinc-700/50 flex items-center justify-center">
            <Wrench size={13} className={colorClass.split(' ')[0]} />
          </div>
          <span className="font-mono text-sm font-semibold text-zinc-200">
            <span className="text-zinc-500">{name}</span>
            {action && <span>.{action}</span>}
          </span>
        </div>
        <span className={`text-xs px-2 py-0.5 rounded-full ${colorClass}`}>{cat}</span>
      </div>
      <p className="text-xs text-zinc-500 line-clamp-2">{tool.description}</p>
    </button>
  );
}

// ── Tool Detail Modal ──────────────────────────────────────────────────────

function ToolDetail({ tool, onClose }: { tool: ToolData; onClose: () => void }) {
  const [argsJson, setArgsJson] = useState('{}');
  const [result, setResult] = useState<unknown>(null);
  const [running, setRunning] = useState(false);
  const [schemaOpen, setSchemaOpen] = useState(true);

  const run = async () => {
    let args: Record<string, unknown>;
    try {
      args = JSON.parse(argsJson);
    } catch {
      toast.error('Invalid JSON arguments');
      return;
    }
    setRunning(true);
    try {
      const res = await tryTool(tool.name, args);
      setResult(res);
      toast.success('Tool invoked successfully');
    } catch (e: unknown) {
      toast.error((e as Error).message);
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm flex justify-center items-start pt-16 px-4" onClick={onClose}>
      <div
        className="w-full max-w-2xl bg-zinc-900 border border-zinc-700 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[80vh]"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800">
          <div>
            <div className="font-mono font-bold text-zinc-100">{tool.name}</div>
            <div className="text-xs text-zinc-500 mt-0.5">{tool.description}</div>
          </div>
          <button onClick={onClose} className="text-zinc-600 hover:text-zinc-300 transition-colors p-1">
            <X size={18} />
          </button>
        </div>

        <div className="overflow-y-auto p-6 space-y-4" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
          {tool.schema && (
            <div>
              <button
                onClick={() => setSchemaOpen(s => !s)}
                className="flex items-center gap-1.5 text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-2"
              >
                {schemaOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                Schema
              </button>
              {schemaOpen && (
                <pre className="text-xs font-mono text-zinc-400 bg-zinc-950 rounded-lg p-3 overflow-x-auto border border-zinc-800">
                  {JSON.stringify(tool.schema, null, 2)}
                </pre>
              )}
            </div>
          )}

          <div>
            <h4 className="text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-2">Try It</h4>
            <div className="mb-2">
              <label className="text-xs text-zinc-600 block mb-1">Arguments (JSON)</label>
              <textarea
                value={argsJson}
                onChange={e => setArgsJson(e.target.value)}
                rows={4}
                className="w-full px-3 py-2 rounded-lg bg-zinc-950 border border-zinc-700 text-xs font-mono text-zinc-300 focus:outline-none focus:border-indigo-500/60 resize-none"
              />
            </div>
            <button
              onClick={run}
              disabled={running}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors disabled:opacity-50"
            >
              {running ? <Activity size={14} className="animate-spin" /> : <Play size={14} />}
              {running ? 'Running...' : 'Run Tool'}
            </button>
          </div>

          {result !== null && (
            <div>
              <h4 className="text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-2">Result</h4>
              <pre className="text-xs font-mono text-emerald-400 bg-zinc-950 rounded-lg p-3 overflow-x-auto max-h-48 border border-zinc-800">
                {JSON.stringify(result, null, 2)}
              </pre>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Hub Search Panel ───────────────────────────────────────────────────────

function HubPanel({ installed, onRefresh }: { installed: InstalledHubItem[]; onRefresh: () => void }) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<HubItem[]>([]);
  const [searching, setSearching] = useState(false);
  const [installing, setInstalling] = useState<string | null>(null);
  const [uninstalling, setUninstalling] = useState<string | null>(null);

  const installedNames = new Set(installed.map(i => i.name));

  const search = useCallback(async () => {
    if (!query.trim()) return;
    setSearching(true);
    try {
      const r = await hubSearch(query);
      setResults(r);
      if (r.length === 0) toast('No results found', { icon: '🔍' });
    } catch (err) {
      toast.error(`Hub search failed: ${err}`);
    } finally {
      setSearching(false);
    }
  }, [query]);

  const install = async (name: string) => {
    setInstalling(name);
    try {
      const res = await hubInstall(name);
      if (res.error) {
        toast.error(res.error);
      } else {
        toast.success(`${name} installed`);
        onRefresh();
      }
    } catch (err) {
      toast.error(`Install failed: ${err}`);
    } finally {
      setInstalling(null);
    }
  };

  const uninstall = async (name: string) => {
    setUninstalling(name);
    try {
      const res = await hubUninstall(name);
      if (res.error) {
        toast.error(res.error);
      } else {
        toast.success(`${name} uninstalled`);
        onRefresh();
      }
    } catch (err) {
      toast.error(`Uninstall failed: ${err}`);
    } finally {
      setUninstalling(null);
    }
  };

  return (
    <div className="rounded-xl border border-zinc-700/40 bg-zinc-900/30 overflow-hidden">
      {/* Header */}
      <div className="px-5 py-4 border-b border-zinc-700/30 flex items-center gap-3">
        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-fuchsia-500/20 to-indigo-500/20 border border-fuchsia-500/20 flex items-center justify-center">
          <Package size={16} className="text-fuchsia-400" />
        </div>
        <div className="flex-1">
          <div className="text-sm font-semibold text-zinc-200">SoulGate Hub</div>
          <div className="text-[11px] text-zinc-500">Search, install, and manage community tools</div>
        </div>
        <a
          href="https://github.com/M4MEET/soulgate-hub"
          target="_blank"
          rel="noopener noreferrer"
          className="text-xs text-zinc-500 hover:text-zinc-300 flex items-center gap-1 transition-colors"
        >
          <ExternalLink size={10} /> GitHub
        </a>
      </div>

      {/* Search */}
      <div className="px-5 py-3 border-b border-zinc-700/20">
        <div className="flex gap-2">
          <div className="flex-1 relative">
            <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-600" />
            <input
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') search(); }}
              placeholder="Search tools, plugins, skills..."
              className="w-full pl-8 pr-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
          <button
            onClick={search}
            disabled={searching || !query.trim()}
            className="px-4 py-2 rounded-lg bg-fuchsia-600/80 hover:bg-fuchsia-500 text-white text-xs font-medium transition-colors disabled:opacity-40 flex items-center gap-1.5"
          >
            {searching ? <RefreshCw size={12} className="animate-spin" /> : <Search size={12} />}
            Search
          </button>
        </div>
      </div>

      {/* Results */}
      {results.length > 0 && (
        <div className="px-5 py-3 space-y-2 max-h-64 overflow-y-auto" style={{ scrollbarWidth: 'thin' }}>
          <div className="text-[10px] text-zinc-600 uppercase tracking-wider font-medium">
            {results.length} result{results.length !== 1 ? 's' : ''}
          </div>
          {results.map(item => {
            const isInstalled = installedNames.has(item.name);
            const isInstalling = installing === item.name;
            return (
              <div key={item.name} className="flex items-center gap-3 p-3 rounded-lg bg-zinc-800/30 border border-zinc-700/30">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold text-zinc-200 font-mono">{item.name}</span>
                    {item.version && <span className="text-[10px] text-zinc-600">v{item.version}</span>}
                    {item.rating != null && item.rating > 0 && (
                      <span className="flex items-center gap-0.5 text-[10px] text-amber-400">
                        <Star size={9} fill="currentColor" />{item.rating.toFixed(1)}
                      </span>
                    )}
                  </div>
                  <div className="text-xs text-zinc-500 truncate mt-0.5">{item.description}</div>
                  {item.author && <div className="text-[10px] text-zinc-600 mt-0.5">by {item.author}</div>}
                </div>
                {isInstalled ? (
                  <span className="text-[10px] text-emerald-400 px-2 py-1 rounded bg-emerald-500/10">Installed</span>
                ) : (
                  <button
                    onClick={() => install(item.name)}
                    disabled={isInstalling}
                    className="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium transition-colors disabled:opacity-50"
                  >
                    {isInstalling ? <RefreshCw size={11} className="animate-spin" /> : <Download size={11} />}
                    Install
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Installed items */}
      {installed.length > 0 && (
        <div className="px-5 py-3 border-t border-zinc-700/20">
          <div className="text-[10px] text-zinc-600 uppercase tracking-wider font-medium mb-2">
            Installed ({installed.length})
          </div>
          <div className="space-y-1.5">
            {installed.map(item => (
              <div key={`${item.type}-${item.name}`} className="flex items-center gap-3 px-3 py-2 rounded-lg bg-zinc-800/20">
                <div className="flex-1 min-w-0">
                  <span className="text-xs font-mono text-zinc-200">{item.name}</span>
                  <span className="text-[10px] text-zinc-600 ml-2">v{item.version}</span>
                  <span className="text-[10px] text-zinc-700 ml-2">{item.type}</span>
                </div>
                <button
                  onClick={() => uninstall(item.name)}
                  disabled={uninstalling === item.name}
                  className="flex items-center gap-1 px-2 py-1 rounded text-xs text-red-400 hover:bg-red-500/10 transition-colors disabled:opacity-50"
                >
                  {uninstalling === item.name ? <RefreshCw size={10} className="animate-spin" /> : <Trash2 size={10} />}
                  Uninstall
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ── Main View ──────────────────────────────────────────────────────────────

export default function ToolsView() {
  const [tools, setTools] = useState<ToolData[]>([]);
  const [installed, setInstalled] = useState<InstalledHubItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('all');
  const [selected, setSelected] = useState<ToolData | null>(null);
  const [tab, setTab] = useState<'tools' | 'hub'>('tools');

  const refresh = useCallback(async () => {
    const [t, h] = await Promise.all([fetchTools(), hubInstalled()]);
    setTools(t.length > 0 ? t : []);
    setInstalled(h);
    setLoading(false);
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const categories = ['all', ...Array.from(new Set(tools.map(t => t.category || 'default')))];

  const filtered = tools.filter(t => {
    const matchesQuery = t.name.toLowerCase().includes(query.toLowerCase()) ||
                         t.description.toLowerCase().includes(query.toLowerCase());
    const matchesCat = category === 'all' || (t.category || 'default') === category;
    return matchesQuery && matchesCat;
  });

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Tools</h2>
          <p className="text-sm text-zinc-500">
            {tools.length} active tools
            {installed.length > 0 && ` · ${installed.length} from Hub`}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {/* Tab switch */}
          <div className="flex rounded-lg bg-zinc-800/50 border border-zinc-700/40 p-0.5">
            <button
              onClick={() => setTab('tools')}
              className={`px-3 py-1 rounded-md text-xs font-medium transition-colors flex items-center gap-1.5 ${
                tab === 'tools' ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
              }`}
            >
              <Wrench size={11} /> Active
            </button>
            <button
              onClick={() => setTab('hub')}
              className={`px-3 py-1 rounded-md text-xs font-medium transition-colors flex items-center gap-1.5 ${
                tab === 'hub' ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
              }`}
            >
              <Package size={11} /> Hub
            </button>
          </div>
          <button onClick={refresh} className="p-2 rounded-lg bg-zinc-800/50 border border-zinc-700/40 text-zinc-400 hover:text-zinc-200 transition-colors">
            <RefreshCw size={14} />
          </button>
        </div>
      </div>

      {tab === 'hub' ? (
        <HubPanel installed={installed} onRefresh={refresh} />
      ) : (
        <>
          {/* Search and filter */}
          <div className="flex flex-col sm:flex-row gap-3 mb-6">
            <div className="flex-1 relative">
              <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
              <input
                value={query}
                onChange={e => setQuery(e.target.value)}
                placeholder="Search tools..."
                className="w-full pl-9 pr-3 py-2.5 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
              />
            </div>
            <div className="flex gap-2 overflow-x-auto pb-1">
              {categories.map(cat => (
                <button
                  key={cat}
                  onClick={() => setCategory(cat)}
                  className={`whitespace-nowrap px-3 py-2 rounded-lg text-xs font-medium transition-all ${
                    category === cat
                      ? 'bg-indigo-500/15 text-indigo-400 border border-indigo-500/30'
                      : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 border border-transparent'
                  }`}
                >
                  {cat}
                </button>
              ))}
            </div>
          </div>

          {/* Grid */}
          {loading ? (
            <div className="flex items-center gap-2 text-zinc-500"><Activity size={16} className="animate-spin" />Loading...</div>
          ) : filtered.length === 0 ? (
            <div className="text-center py-16 text-zinc-500">
              <Wrench size={40} className="mx-auto mb-3 opacity-30" />
              <p>No tools match your search.</p>
              <button onClick={() => setTab('hub')} className="mt-3 text-xs text-indigo-400 hover:text-indigo-300 transition-colors">
                Search the Hub for more tools
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {filtered.map(tool => (
                <ToolCard key={tool.name} tool={tool} onClick={() => setSelected(tool)} />
              ))}
            </div>
          )}
        </>
      )}

      {selected && <ToolDetail tool={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}
