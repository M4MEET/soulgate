import { useState, useEffect } from 'react';
import { Wrench, Search, Play, ChevronDown, ChevronRight, Activity, X } from 'lucide-react';
import { fetchTools, tryTool, type ToolData } from '../lib/api';
import toast from 'react-hot-toast';

const FALLBACK_TOOLS: ToolData[] = [
  { name: 'files.read',    description: 'Read file contents from the workspace',       category: 'filesystem' },
  { name: 'files.write',   description: 'Write content to a file in the workspace',    category: 'filesystem' },
  { name: 'files.list',    description: 'List files in a directory',                   category: 'filesystem' },
  { name: 'files.delete',  description: 'Delete a file from the workspace',            category: 'filesystem' },
  { name: 'exec.run',      description: 'Execute a shell command',                     category: 'execution' },
  { name: 'net.fetch',     description: 'Make an HTTP request to a URL',               category: 'network' },
  { name: 'net.search',    description: 'Search the web',                              category: 'network' },
  { name: 'memory.get',    description: 'Retrieve a value from memory',                category: 'memory' },
  { name: 'memory.set',    description: 'Store a key-value pair in memory',            category: 'memory' },
  { name: 'memory.search', description: 'Semantic search over memory',                 category: 'memory' },
  { name: 'secrets.get',   description: 'Retrieve a secret by name',                   category: 'security' },
  { name: 'audit.log',     description: 'Write an audit log entry',                    category: 'audit' },
  { name: 'agent.spawn',   description: 'Spawn a sub-agent with a task',               category: 'agents' },
  { name: 'agent.message', description: 'Send a message to an agent',                  category: 'agents' },
];

const CATEGORY_COLORS: Record<string, string> = {
  filesystem: 'text-sky-400 bg-sky-500/10',
  execution:  'text-amber-400 bg-amber-500/10',
  network:    'text-emerald-400 bg-emerald-500/10',
  memory:     'text-violet-400 bg-violet-500/10',
  security:   'text-red-400 bg-red-500/10',
  audit:      'text-orange-400 bg-orange-500/10',
  agents:     'text-indigo-400 bg-indigo-500/10',
  default:    'text-zinc-400 bg-zinc-500/10',
};

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
          {/* Schema */}
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

          {/* Try it */}
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
              {running ? 'Running…' : 'Run Tool'}
            </button>
          </div>

          {/* Result */}
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

export default function ToolsView() {
  const [tools, setTools] = useState<ToolData[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('all');
  const [selected, setSelected] = useState<ToolData | null>(null);

  useEffect(() => {
    fetchTools().then(data => {
      setTools(data.length > 0 ? data : FALLBACK_TOOLS);
    }).finally(() => setLoading(false));
  }, []);

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
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Tools</h2>
          <p className="text-sm text-zinc-500">{tools.length} available tools</p>
        </div>
      </div>

      {/* Search and filter */}
      <div className="flex flex-col sm:flex-row gap-3 mb-6">
        <div className="flex-1 relative">
          <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search tools…"
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
        <div className="flex items-center gap-2 text-zinc-500"><Activity size={16} className="animate-spin" />Loading…</div>
      ) : filtered.length === 0 ? (
        <div className="text-center py-16 text-zinc-500">
          <Wrench size={40} className="mx-auto mb-3 opacity-30" />
          <p>No tools match your search.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {filtered.map(tool => (
            <ToolCard key={tool.name} tool={tool} onClick={() => setSelected(tool)} />
          ))}
        </div>
      )}

      {selected && <ToolDetail tool={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}
