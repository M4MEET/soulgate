import { useState, useEffect, useCallback } from 'react';
import {
  Wrench, Search, Play, ChevronDown, ChevronRight, Activity, X,
  Package, Download, Trash2, Star, RefreshCw,
  Power, PowerOff, Globe,
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

// ── Featured Hub catalog (shown when Hub API is offline) ───────────────────

const FEATURED_HUB_ITEMS: HubItem[] = [
  { name: 'web-scraper',       description: 'Scrape and extract structured data from websites',         category: 'network',    author: 'soulgate',   downloads: 1200, rating: 4.8, tags: ['web', 'scraping'] },
  { name: 'code-review',       description: 'AI-powered code review with security and quality checks',  category: 'dev',        author: 'soulgate',   downloads: 980,  rating: 4.7, tags: ['code', 'review'] },
  { name: 'pdf-ocr',           description: 'Extract text from scanned PDFs using OCR',                category: 'filesystem', author: 'community',  downloads: 850,  rating: 4.5, tags: ['pdf', 'ocr'] },
  { name: 'git-ops',           description: 'Git operations: branch, commit, PR automation',            category: 'dev',        author: 'soulgate',   downloads: 750,  rating: 4.6, tags: ['git', 'automation'] },
  { name: 'db-query',          description: 'Query SQLite, PostgreSQL, and MySQL databases',            category: 'data',       author: 'community',  downloads: 620,  rating: 4.4, tags: ['database', 'sql'] },
  { name: 'image-gen',         description: 'Generate images using DALL-E, Stable Diffusion, Flux',    category: 'creative',   author: 'soulgate',   downloads: 1500, rating: 4.9, tags: ['image', 'ai'] },
  { name: 'email-send',        description: 'Send emails via SMTP or SendGrid/Mailgun APIs',           category: 'network',    author: 'community',  downloads: 430,  rating: 4.3, tags: ['email', 'smtp'] },
  { name: 'calendar-sync',     description: 'Sync with Google Calendar, Outlook, and CalDAV',           category: 'productivity', author: 'community', downloads: 380,  rating: 4.2, tags: ['calendar', 'sync'] },
  { name: 'kubernetes-ops',    description: 'Manage Kubernetes clusters: pods, deploys, logs',          category: 'infra',      author: 'community',  downloads: 510,  rating: 4.5, tags: ['k8s', 'devops'] },
  { name: 'notion-sync',       description: 'Read and write Notion pages and databases',               category: 'productivity', author: 'community', downloads: 690,  rating: 4.6, tags: ['notion', 'sync'] },
  { name: 's3-storage',        description: 'Upload, download, and manage files in S3/R2/MinIO',       category: 'infra',      author: 'community',  downloads: 410,  rating: 4.3, tags: ['s3', 'cloud'] },
  { name: 'speech-to-text',    description: 'Transcribe audio files using Whisper or Deepgram',         category: 'ai',         author: 'soulgate',   downloads: 820,  rating: 4.7, tags: ['audio', 'stt'] },
];

// ── Tool Card ──────────────────────────────────────────────────────────────

function ToolCard({ tool, disabled, onToggle, onClick }: {
  tool: ToolData;
  disabled: boolean;
  onToggle: () => void;
  onClick: () => void;
}) {
  const cat = tool.category || 'default';
  const colorClass = CATEGORY_COLORS[cat] || CATEGORY_COLORS.default;
  const [name, action] = tool.name.includes('.') ? tool.name.split('.') : [tool.name, ''];

  return (
    <div className={`p-4 rounded-xl border transition-all ${
      disabled
        ? 'bg-zinc-900/30 border-zinc-800/40 opacity-50'
        : 'bg-zinc-800/40 border-zinc-700/40 hover:border-zinc-600/60 hover:bg-zinc-800/60'
    }`}>
      <div className="flex items-start justify-between gap-2 mb-2">
        <button onClick={onClick} className="flex items-center gap-2 text-left flex-1 min-w-0">
          <div className="w-7 h-7 rounded-lg bg-zinc-800 border border-zinc-700/50 flex items-center justify-center flex-shrink-0">
            <Wrench size={13} className={colorClass.split(' ')[0]} />
          </div>
          <span className="font-mono text-sm font-semibold text-zinc-200 truncate">
            <span className="text-zinc-500">{name}</span>
            {action && <span>.{action}</span>}
          </span>
        </button>
        <div className="flex items-center gap-1.5 flex-shrink-0">
          <span className={`text-[10px] px-2 py-0.5 rounded-full ${colorClass}`}>{cat}</span>
          <button
            onClick={e => { e.stopPropagation(); onToggle(); }}
            title={disabled ? 'Activate tool' : 'Deactivate tool'}
            className={`p-1 rounded transition-colors ${
              disabled
                ? 'text-zinc-600 hover:text-emerald-400 hover:bg-emerald-500/10'
                : 'text-emerald-400 hover:text-red-400 hover:bg-red-500/10'
            }`}
          >
            {disabled ? <PowerOff size={12} /> : <Power size={12} />}
          </button>
        </div>
      </div>
      <p className="text-xs text-zinc-500 line-clamp-2 cursor-pointer" onClick={onClick}>{tool.description}</p>
    </div>
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
    try { args = JSON.parse(argsJson); } catch { toast.error('Invalid JSON'); return; }
    setRunning(true);
    try {
      const res = await tryTool(tool.name, args);
      setResult(res);
      toast.success('Tool invoked');
    } catch (e: unknown) { toast.error((e as Error).message); }
    finally { setRunning(false); }
  };

  return (
    <div className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm flex justify-center items-start pt-16 px-4" onClick={onClose}>
      <div className="w-full max-w-2xl bg-zinc-900 border border-zinc-700 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[80vh]" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-800">
          <div>
            <div className="font-mono font-bold text-zinc-100">{tool.name}</div>
            <div className="text-xs text-zinc-500 mt-0.5">{tool.description}</div>
          </div>
          <button onClick={onClose} className="text-zinc-600 hover:text-zinc-300 p-1"><X size={18} /></button>
        </div>
        <div className="overflow-y-auto p-6 space-y-4" style={{ scrollbarWidth: 'thin' }}>
          {tool.schema && (
            <div>
              <button onClick={() => setSchemaOpen(s => !s)} className="flex items-center gap-1.5 text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-2">
                {schemaOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />} Schema
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
            <textarea value={argsJson} onChange={e => setArgsJson(e.target.value)} rows={4}
              className="w-full px-3 py-2 rounded-lg bg-zinc-950 border border-zinc-700 text-xs font-mono text-zinc-300 focus:outline-none focus:border-indigo-500/60 resize-none mb-2" />
            <button onClick={run} disabled={running}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors disabled:opacity-50">
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

// ── Hub Panel ──────────────────────────────────────────────────────────────

function HubPanel({ installed, onRefresh }: { installed: InstalledHubItem[]; onRefresh: () => void }) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<HubItem[]>([]);
  const [searched, setSearched] = useState(false);
  const [searching, setSearching] = useState(false);
  const [installing, setInstalling] = useState<string | null>(null);
  const [uninstalling, setUninstalling] = useState<string | null>(null);
  const [showFeatured, setShowFeatured] = useState(true);

  const installedNames = new Set(installed.map(i => i.name));
  const hubUrl = 'https://github.com/M4MEET/soulgate-hub';

  const search = useCallback(async () => {
    if (!query.trim()) return;
    setSearching(true);
    setSearched(true);
    try {
      const r = await hubSearch(query);
      if (r.length > 0) {
        setResults(r);
        setShowFeatured(false);
      } else {
        // Hub API offline or no results — filter featured catalog locally
        const q = query.toLowerCase();
        const local = FEATURED_HUB_ITEMS.filter(i =>
          i.name.includes(q) || i.description.toLowerCase().includes(q) ||
          (i.tags || []).some(t => t.includes(q)) || (i.category || '').includes(q)
        );
        setResults(local);
        setShowFeatured(false);
        if (local.length === 0) toast('No results. Browse the Hub for more.', { icon: '🔍' });
      }
    } catch {
      // Hub offline — search local catalog
      const q = query.toLowerCase();
      const local = FEATURED_HUB_ITEMS.filter(i =>
        i.name.includes(q) || i.description.toLowerCase().includes(q) ||
        (i.tags || []).some(t => t.includes(q))
      );
      setResults(local);
      setShowFeatured(false);
    } finally {
      setSearching(false);
    }
  }, [query]);

  const clearSearch = () => {
    setQuery('');
    setResults([]);
    setSearched(false);
    setShowFeatured(true);
  };

  const install = async (name: string) => {
    setInstalling(name);
    try {
      const res = await hubInstall(name);
      if (res.error) toast.error(res.error);
      else { toast.success(`${name} installed`); onRefresh(); }
    } catch (err) { toast.error(`Install failed: ${err}`); }
    finally { setInstalling(null); }
  };

  const uninstall = async (name: string) => {
    setUninstalling(name);
    try {
      const res = await hubUninstall(name);
      if (res.error) toast.error(res.error);
      else { toast.success(`${name} uninstalled`); onRefresh(); }
    } catch (err) { toast.error(`Uninstall failed: ${err}`); }
    finally { setUninstalling(null); }
  };

  const displayItems = showFeatured ? FEATURED_HUB_ITEMS : results;

  return (
    <div className="space-y-4">
      {/* Header card */}
      <div className="rounded-xl border border-zinc-700/40 bg-zinc-900/30 overflow-hidden">
        <div className="px-5 py-4 border-b border-zinc-700/30 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-gradient-to-br from-fuchsia-500/20 to-indigo-500/20 border border-fuchsia-500/20 flex items-center justify-center">
            <Package size={18} className="text-fuchsia-400" />
          </div>
          <div className="flex-1">
            <div className="text-sm font-semibold text-zinc-200">SoulGate Hub</div>
            <div className="text-[11px] text-zinc-500">Search, install, and manage community tools and plugins</div>
          </div>
          <a href={hubUrl} target="_blank" rel="noopener noreferrer"
            className="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-zinc-800 border border-zinc-700/40 text-xs text-zinc-400 hover:text-zinc-200 transition-colors">
            <Globe size={11} /> Browse Hub
          </a>
        </div>

        {/* Search */}
        <div className="px-5 py-3">
          <div className="flex gap-2">
            <div className="flex-1 relative">
              <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-600" />
              <input type="text" value={query} onChange={e => setQuery(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') search(); }}
                placeholder="Search tools, plugins, skills..."
                className="w-full pl-8 pr-3 py-2 rounded-lg bg-zinc-900 border border-zinc-700 text-xs text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-fuchsia-500/60" />
            </div>
            <button onClick={search} disabled={searching || !query.trim()}
              className="px-4 py-2 rounded-lg bg-fuchsia-600/80 hover:bg-fuchsia-500 text-white text-xs font-medium transition-colors disabled:opacity-40 flex items-center gap-1.5">
              {searching ? <RefreshCw size={12} className="animate-spin" /> : <Search size={12} />} Search
            </button>
            {searched && (
              <button onClick={clearSearch} className="px-3 py-2 rounded-lg bg-zinc-800 text-xs text-zinc-400 hover:text-zinc-200 transition-colors">
                Clear
              </button>
            )}
          </div>
          {/* Quick filter tags */}
          <div className="flex flex-wrap gap-1.5 mt-2">
            {['web', 'database', 'ai', 'devops', 'email', 'image', 'code'].map(tag => (
              <button key={tag} onClick={() => { setQuery(tag); }}
                className="px-2 py-0.5 rounded text-[10px] bg-zinc-800 border border-zinc-700/50 text-zinc-500 hover:text-zinc-200 hover:border-zinc-600 transition-colors">
                {tag}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Installed items */}
      {installed.length > 0 && (
        <div className="rounded-xl border border-emerald-500/15 bg-emerald-500/[0.03] overflow-hidden">
          <div className="px-5 py-3 border-b border-emerald-500/10 flex items-center gap-2">
            <Download size={13} className="text-emerald-400" />
            <span className="text-xs font-semibold text-emerald-400">Installed from Hub ({installed.length})</span>
          </div>
          <div className="px-5 py-3 space-y-1.5">
            {installed.map(item => (
              <div key={`${item.type}-${item.name}`} className="flex items-center gap-3 px-3 py-2 rounded-lg bg-zinc-800/30 border border-zinc-700/20">
                <div className="w-6 h-6 rounded bg-emerald-500/10 flex items-center justify-center flex-shrink-0">
                  <Package size={11} className="text-emerald-400" />
                </div>
                <div className="flex-1 min-w-0">
                  <span className="text-xs font-mono font-semibold text-zinc-200">{item.name}</span>
                  <span className="text-[10px] text-zinc-600 ml-2">v{item.version}</span>
                  <span className="text-[10px] text-zinc-700 ml-2">{item.type}</span>
                </div>
                <button onClick={() => uninstall(item.name)} disabled={uninstalling === item.name}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs text-red-400 bg-red-500/5 border border-red-500/15 hover:bg-red-500/10 transition-colors disabled:opacity-50">
                  {uninstalling === item.name ? <RefreshCw size={10} className="animate-spin" /> : <Trash2 size={10} />}
                  Uninstall
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Catalog / Search Results */}
      <div>
        <div className="flex items-center gap-2 mb-3">
          <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">
            {showFeatured ? 'Featured' : `Results (${results.length})`}
          </span>
          {showFeatured && (
            <span className="text-[10px] text-zinc-600">Popular tools from the community</span>
          )}
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
          {displayItems.map(item => {
            const isInstalled = installedNames.has(item.name);
            const isInstalling = installing === item.name;
            return (
              <div key={item.name} className="flex items-center gap-3 p-3.5 rounded-xl bg-zinc-800/30 border border-zinc-700/30 hover:border-zinc-600/40 transition-all">
                <div className="w-8 h-8 rounded-lg bg-zinc-800 border border-zinc-700/50 flex items-center justify-center flex-shrink-0">
                  <Package size={14} className="text-fuchsia-400/60" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold text-zinc-200 font-mono">{item.name}</span>
                    {item.rating != null && item.rating > 0 && (
                      <span className="flex items-center gap-0.5 text-[10px] text-amber-400">
                        <Star size={8} fill="currentColor" />{item.rating.toFixed(1)}
                      </span>
                    )}
                    {item.downloads != null && item.downloads > 0 && (
                      <span className="text-[10px] text-zinc-600">{item.downloads.toLocaleString()} DLs</span>
                    )}
                  </div>
                  <div className="text-[11px] text-zinc-500 truncate mt-0.5">{item.description}</div>
                  <div className="flex items-center gap-2 mt-1">
                    {item.author && <span className="text-[10px] text-zinc-600">by {item.author}</span>}
                    {item.category && (
                      <span className={`text-[9px] px-1.5 py-0.5 rounded ${CATEGORY_COLORS[item.category] || CATEGORY_COLORS.default}`}>
                        {item.category}
                      </span>
                    )}
                  </div>
                </div>
                {isInstalled ? (
                  <span className="text-[10px] text-emerald-400 px-2 py-1 rounded bg-emerald-500/10 flex-shrink-0">Installed</span>
                ) : (
                  <button onClick={() => install(item.name)} disabled={isInstalling}
                    className="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-fuchsia-600/80 hover:bg-fuchsia-500 text-white text-xs font-medium transition-colors disabled:opacity-50 flex-shrink-0">
                    {isInstalling ? <RefreshCw size={11} className="animate-spin" /> : <Download size={11} />}
                    Install
                  </button>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// ── Main View ──────────────────────────────────────────────────────────────

export default function ToolsView() {
  const [tools, setTools] = useState<ToolData[]>([]);
  const [installed, setInstalled] = useState<InstalledHubItem[]>([]);
  const [disabledTools, setDisabledTools] = useState<Set<string>>(() => {
    try { return new Set(JSON.parse(localStorage.getItem('sg-disabled-tools') || '[]')); }
    catch { return new Set(); }
  });
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

  const toggleTool = (name: string) => {
    setDisabledTools(prev => {
      const next = new Set(prev);
      if (next.has(name)) { next.delete(name); toast.success(`${name} activated`); }
      else { next.add(name); toast(`${name} deactivated`, { icon: '⏸' }); }
      localStorage.setItem('sg-disabled-tools', JSON.stringify([...next]));
      return next;
    });
  };

  const categories = ['all', ...Array.from(new Set(tools.map(t => t.category || 'default')))];

  const filtered = tools.filter(t => {
    const matchesQuery = t.name.toLowerCase().includes(query.toLowerCase()) ||
                         t.description.toLowerCase().includes(query.toLowerCase());
    const matchesCat = category === 'all' || (t.category || 'default') === category;
    return matchesQuery && matchesCat;
  });

  const activeCount = tools.filter(t => !disabledTools.has(t.name)).length;

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Tools</h2>
          <p className="text-sm text-zinc-500">
            {activeCount} active · {tools.length} total
            {installed.length > 0 && ` · ${installed.length} from Hub`}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex rounded-lg bg-zinc-800/50 border border-zinc-700/40 p-0.5">
            <button onClick={() => setTab('tools')}
              className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors flex items-center gap-1.5 ${
                tab === 'tools' ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
              }`}>
              <Wrench size={11} /> Active Tools
            </button>
            <button onClick={() => setTab('hub')}
              className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors flex items-center gap-1.5 ${
                tab === 'hub' ? 'bg-fuchsia-500/20 text-fuchsia-300' : 'text-zinc-500 hover:text-zinc-300'
              }`}>
              <Package size={11} /> Hub Store
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
              <input value={query} onChange={e => setQuery(e.target.value)} placeholder="Search tools..."
                className="w-full pl-9 pr-3 py-2.5 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60" />
            </div>
            <div className="flex gap-2 overflow-x-auto pb-1">
              {categories.map(cat => (
                <button key={cat} onClick={() => setCategory(cat)}
                  className={`whitespace-nowrap px-3 py-2 rounded-lg text-xs font-medium transition-all ${
                    category === cat
                      ? 'bg-indigo-500/15 text-indigo-400 border border-indigo-500/30'
                      : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 border border-transparent'
                  }`}>
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
              <button onClick={() => setTab('hub')} className="mt-3 text-xs text-fuchsia-400 hover:text-fuchsia-300 transition-colors flex items-center gap-1 mx-auto">
                <Package size={12} /> Browse the Hub Store for more tools
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {filtered.map(tool => (
                <ToolCard
                  key={tool.name}
                  tool={tool}
                  disabled={disabledTools.has(tool.name)}
                  onToggle={() => toggleTool(tool.name)}
                  onClick={() => setSelected(tool)}
                />
              ))}
            </div>
          )}
        </>
      )}

      {selected && <ToolDetail tool={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}
