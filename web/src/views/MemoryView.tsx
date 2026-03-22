import { useState, useEffect } from 'react';
import { Brain, Plus, Search, Trash2, Edit3, Check, X, Activity } from 'lucide-react';
import { fetchMemory, setMemoryEntry, deleteMemoryEntry, type MemoryEntry } from '../lib/api';
import { formatRelativeTime } from '../lib/utils';
import toast from 'react-hot-toast';


function EntryRow({
  entry,
  onDelete,
  onSave,
}: {
  entry: MemoryEntry;
  onDelete: (key: string) => void;
  onSave: (key: string, value: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(entry.value);

  const save = () => {
    onSave(entry.key, draft);
    setEditing(false);
  };

  return (
    <tr className="border-b border-zinc-800/50 hover:bg-zinc-800/20 transition-colors group">
      <td className="px-4 py-3">
        <span className="font-mono text-xs text-indigo-300">{entry.key}</span>
      </td>
      <td className="px-4 py-3 max-w-64">
        {editing ? (
          <textarea
            value={draft}
            onChange={e => setDraft(e.target.value)}
            className="w-full px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-xs text-zinc-200 font-mono focus:outline-none focus:border-indigo-500/60 resize-none"
            rows={2}
          />
        ) : (
          <span className="text-xs text-zinc-300 font-mono truncate block max-w-xs">{entry.value}</span>
        )}
      </td>
      <td className="px-4 py-3">
        <span className={`text-xs px-2 py-0.5 rounded-full ${
          entry.type === 'json'   ? 'bg-amber-500/10 text-amber-400' :
          entry.type === 'vector' ? 'bg-violet-500/10 text-violet-400' :
                                    'bg-zinc-700/60 text-zinc-400'
        }`}>
          {entry.type}
        </span>
      </td>
      <td className="px-4 py-3 text-xs text-zinc-600">{formatRelativeTime(entry.updated_at)}</td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {editing ? (
            <>
              <button onClick={save} className="p-1 rounded hover:bg-emerald-500/10 text-emerald-400 transition-colors"><Check size={13} /></button>
              <button onClick={() => { setEditing(false); setDraft(entry.value); }} className="p-1 rounded hover:bg-zinc-700 text-zinc-500 transition-colors"><X size={13} /></button>
            </>
          ) : (
            <>
              <button onClick={() => setEditing(true)} className="p-1 rounded hover:bg-zinc-700 text-zinc-500 hover:text-zinc-300 transition-colors"><Edit3 size={13} /></button>
              <button onClick={() => onDelete(entry.key)} className="p-1 rounded hover:bg-red-500/10 text-zinc-500 hover:text-red-400 transition-colors"><Trash2 size={13} /></button>
            </>
          )}
        </div>
      </td>
    </tr>
  );
}

export default function MemoryView() {
  const [entries, setEntries] = useState<MemoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');
  const [adding, setAdding] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [semanticQuery, setSemanticQuery] = useState('');
  const [semanticResults, setSemanticResults] = useState<string[]>([]);

  const load = async () => {
    setLoading(true);
    try {
      const data = await fetchMemory();
      setEntries(data);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleDelete = async (key: string) => {
    try {
      await deleteMemoryEntry(key);
      setEntries(prev => prev.filter(e => e.key !== key));
      toast.success(`Deleted "${key}"`);
    } catch {
      toast.error('Failed to delete');
    }
  };

  const handleSave = async (key: string, value: string) => {
    try {
      await setMemoryEntry(key, value);
      setEntries(prev => prev.map(e => e.key === key ? { ...e, value, updated_at: new Date().toISOString() } : e));
      toast.success('Saved');
    } catch {
      toast.error('Failed to save');
    }
  };

  const handleAdd = async () => {
    if (!newKey.trim()) { toast.error('Key is required'); return; }
    try {
      await setMemoryEntry(newKey, newValue);
      const entry: MemoryEntry = {
        key: newKey, value: newValue, type: 'string',
        created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      };
      setEntries(prev => [...prev, entry]);
      setAdding(false);
      setNewKey('');
      setNewValue('');
      toast.success('Entry added');
    } catch {
      toast.error('Failed to add entry');
    }
  };

  const handleSemanticSearch = () => {
    if (!semanticQuery.trim()) return;
    // Keyword filter over current entries (vector store not available)
    const results = entries
      .filter(e => e.value.toLowerCase().includes(semanticQuery.toLowerCase()) || e.key.toLowerCase().includes(semanticQuery.toLowerCase()))
      .map(e => `${e.key}: ${e.value.slice(0, 80)}`);
    setSemanticResults(results.length > 0 ? results : ['No matching entries found']);
  };

  const filtered = entries.filter(e =>
    e.key.toLowerCase().includes(query.toLowerCase()) ||
    e.value.toLowerCase().includes(query.toLowerCase())
  );

  return (
    <div className="flex-1 overflow-y-auto p-6 bg-zinc-950" style={{ scrollbarWidth: 'thin', scrollbarColor: '#3f3f46 transparent' }}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Memory</h2>
          <p className="text-sm text-zinc-500">{entries.length} entries</p>
        </div>
        <button
          onClick={() => setAdding(true)}
          className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors"
        >
          <Plus size={14} />
          Add Entry
        </button>
      </div>

      {/* Add form */}
      {adding && (
        <div className="mb-5 p-4 rounded-xl border border-indigo-500/30 bg-indigo-500/5">
          <h4 className="text-sm font-semibold text-zinc-200 mb-3">New Memory Entry</h4>
          <div className="flex gap-2 mb-2">
            <input
              placeholder="Key (e.g. user.name)"
              value={newKey}
              onChange={e => setNewKey(e.target.value)}
              className="flex-1 px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm font-mono text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
            />
          </div>
          <textarea
            placeholder="Value"
            value={newValue}
            onChange={e => setNewValue(e.target.value)}
            rows={2}
            className="w-full px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60 resize-none mb-2"
          />
          <div className="flex gap-2">
            <button onClick={handleAdd} className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm transition-colors">Save</button>
            <button onClick={() => setAdding(false)} className="px-4 py-2 rounded-lg border border-zinc-700 text-sm text-zinc-400 hover:text-zinc-200 transition-all">Cancel</button>
          </div>
        </div>
      )}

      {/* Search bar */}
      <div className="flex gap-3 mb-4">
        <div className="flex-1 relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Filter by key or value…"
            className="w-full pl-9 pr-3 py-2.5 rounded-xl bg-zinc-800/60 border border-zinc-700/50 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-indigo-500/60"
          />
        </div>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-zinc-700/40 overflow-hidden mb-6">
        {loading ? (
          <div className="flex items-center gap-2 text-zinc-500 p-6"><Activity size={16} className="animate-spin" />Loading…</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-700/50 bg-zinc-800/60">
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Key</th>
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Value</th>
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Type</th>
                <th className="text-left px-4 py-2.5 text-xs text-zinc-500 font-medium uppercase tracking-wide">Updated</th>
                <th className="px-4 py-2.5" />
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr><td colSpan={5} className="text-center text-zinc-500 text-sm py-8">No entries</td></tr>
              ) : (
                filtered.map(entry => (
                  <EntryRow key={entry.key} entry={entry} onDelete={handleDelete} onSave={handleSave} />
                ))
              )}
            </tbody>
          </table>
        )}
      </div>

      {/* Semantic search */}
      <div className="rounded-xl border border-zinc-700/40 bg-zinc-800/20 p-5">
        <div className="flex items-center gap-2 mb-3">
          <Brain size={16} className="text-violet-400" />
          <h3 className="text-sm font-semibold text-zinc-300">Semantic Search</h3>
          <span className="text-xs text-zinc-600 bg-zinc-800 px-2 py-0.5 rounded-full">Keyword search</span>
        </div>
        <div className="flex gap-2 mb-3">
          <input
            value={semanticQuery}
            onChange={e => setSemanticQuery(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSemanticSearch()}
            placeholder="Find similar memories by meaning…"
            className="flex-1 px-3 py-2 rounded-lg bg-zinc-800 border border-zinc-700 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-violet-500/60"
          />
          <button
            onClick={handleSemanticSearch}
            className="px-4 py-2 rounded-lg bg-violet-600 hover:bg-violet-500 text-white text-sm transition-colors flex items-center gap-1.5"
          >
            <Search size={13} />
            Search
          </button>
        </div>
        {semanticResults.length > 0 && (
          <div className="space-y-2">
            {semanticResults.map((r, i) => (
              <div key={i} className="text-xs text-zinc-400 px-3 py-2 rounded-lg bg-zinc-800/60 border border-zinc-700/40 font-mono">{r}</div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
