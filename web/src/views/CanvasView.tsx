import { useState, useEffect, useCallback } from 'react';
import {
  Palette, ExternalLink, Copy, Trash2, Code2, Globe, GitBranch,
  ImageIcon, RefreshCw, ChevronRight, X,
} from 'lucide-react';
import { sendChat } from '../lib/api';
import { formatRelativeTime } from '../lib/utils';
import toast from 'react-hot-toast';

// ── Types ─────────────────────────────────────────────────────────────────────

type ArtifactType = 'html' | 'react' | 'svg' | 'mermaid' | 'unknown';

interface Artifact {
  id: string;
  title: string;
  type: ArtifactType;
  content: string;
  created_at: string;
  preview_url?: string;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function detectType(content: string, title?: string): ArtifactType {
  const lower = (title ?? '').toLowerCase();
  if (lower.includes('mermaid') || content.trimStart().startsWith('graph ') || content.trimStart().startsWith('sequenceDiagram')) return 'mermaid';
  if (lower.includes('svg') || content.trimStart().startsWith('<svg')) return 'svg';
  if (lower.includes('react') || content.includes('import React') || content.includes('export default function')) return 'react';
  if (content.includes('<!DOCTYPE') || content.includes('<html') || content.includes('<body')) return 'html';
  return 'unknown';
}

function buildPreviewHtml(artifact: Artifact): string {
  if (artifact.type === 'html') return artifact.content;
  if (artifact.type === 'svg') {
    return `<!DOCTYPE html><html><body style="margin:0;background:#18181b;display:flex;align-items:center;justify-content:center;min-height:100vh;">${artifact.content}</body></html>`;
  }
  if (artifact.type === 'mermaid') {
    return `<!DOCTYPE html><html><head>
<script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"><\/script>
</head><body style="margin:0;padding:24px;background:#18181b;color:#f4f4f5;">
<div class="mermaid" style="display:flex;justify-content:center;">${artifact.content}</div>
<script>mermaid.initialize({startOnLoad:true,theme:'dark'});<\/script>
</body></html>`;
  }
  // react / unknown — show source
  return `<!DOCTYPE html><html><head><style>
body{margin:0;padding:24px;background:#18181b;color:#f4f4f5;font-family:monospace;font-size:13px;line-height:1.6;}
pre{white-space:pre-wrap;word-break:break-all;}
</style></head><body><pre>${artifact.content.replace(/</g, '&lt;')}</pre></body></html>`;
}

const TYPE_STYLES: Record<ArtifactType, { label: string; classes: string; icon: React.ElementType }> = {
  html:    { label: 'HTML',    classes: 'bg-orange-500/15 text-orange-400',  icon: Globe },
  react:   { label: 'React',   classes: 'bg-sky-500/15 text-sky-400',       icon: Code2 },
  svg:     { label: 'SVG',     classes: 'bg-violet-500/15 text-violet-400', icon: ImageIcon },
  mermaid: { label: 'Mermaid', classes: 'bg-emerald-500/15 text-emerald-400', icon: GitBranch },
  unknown: { label: 'Code',    classes: 'bg-zinc-500/15 text-zinc-400',     icon: Code2 },
};

// ── Parsing ───────────────────────────────────────────────────────────────────

function parseArtifactsFromText(raw: string): Artifact[] {
  const artifacts: Artifact[] = [];

  // Try JSON parse first (structured response)
  try {
    const json = JSON.parse(raw);
    const list = Array.isArray(json) ? json : json.artifacts ?? json.items ?? [];
    if (Array.isArray(list) && list.length > 0) {
      return list.map((item: Record<string, unknown>, i: number) => {
        const content = String(item.content ?? item.html ?? item.code ?? '');
        const title = String(item.title ?? item.name ?? `Artifact ${i + 1}`);
        const type = (item.type as ArtifactType) ?? detectType(content, title);
        return {
          id: String(item.id ?? `art_${i}`),
          title,
          type,
          content,
          created_at: String(item.created_at ?? item.createdAt ?? new Date().toISOString()),
          preview_url: item.preview_url ? String(item.preview_url) : undefined,
        };
      });
    }
  } catch { /* not JSON */ }

  // Extract fenced code blocks: ```html, ```svg, etc.
  const fenceRe = /```(\w+)?\n([\s\S]*?)```/g;
  let match: RegExpExecArray | null;
  let idx = 0;
  while ((match = fenceRe.exec(raw)) !== null) {
    const lang = (match[1] ?? 'unknown').toLowerCase() as ArtifactType;
    const content = match[2].trim();
    if (!content) continue;
    const type: ArtifactType = ['html', 'react', 'svg', 'mermaid'].includes(lang) ? lang : detectType(content);
    artifacts.push({
      id: `art_${idx++}`,
      title: `${TYPE_STYLES[type].label} Artifact ${idx}`,
      type,
      content,
      created_at: new Date().toISOString(),
    });
  }

  return artifacts;
}

// ── Empty state ───────────────────────────────────────────────────────────────

function EmptyState({ onFetch }: { onFetch: () => void }) {
  return (
    <div className="flex-1 flex flex-col items-center justify-center gap-5 p-10 text-center">
      <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-violet-600 to-indigo-600 flex items-center justify-center shadow-lg shadow-violet-500/20">
        <Palette size={32} className="text-white" />
      </div>
      <div>
        <h2 className="text-xl font-semibold text-zinc-200 mb-1">No artifacts yet</h2>
        <p className="text-sm text-zinc-500 max-w-sm">
          Ask the AI to create something — HTML pages, SVG diagrams, Mermaid charts, or React components.
        </p>
      </div>
      <button
        onClick={onFetch}
        className="flex items-center gap-2 px-4 py-2 bg-indigo-500/15 hover:bg-indigo-500/25 border border-indigo-500/30 text-indigo-400 rounded-lg text-sm font-medium transition-all"
      >
        <RefreshCw size={14} />
        Check for artifacts
      </button>
    </div>
  );
}

// ── Preview panel ─────────────────────────────────────────────────────────────

function PreviewPanel({ artifact, onClose }: { artifact: Artifact; onClose: () => void }) {
  const TypeIcon = TYPE_STYLES[artifact.type].icon;
  const srcDoc = buildPreviewHtml(artifact);

  const handleCopy = () => {
    navigator.clipboard.writeText(artifact.content).then(() => toast.success('Copied to clipboard'));
  };

  const handleOpenTab = () => {
    const blob = new Blob([srcDoc], { type: 'text/html' });
    const url = URL.createObjectURL(blob);
    window.open(url, '_blank');
  };

  return (
    <div className="flex flex-col h-full border-l border-zinc-800 bg-zinc-900 min-w-0 w-full">
      {/* Panel header */}
      <div className="flex items-center gap-3 px-4 py-3 border-b border-zinc-800 flex-shrink-0">
        <TypeIcon size={15} className="text-zinc-400 flex-shrink-0" />
        <span className="text-sm font-medium text-zinc-200 flex-1 truncate">{artifact.title}</span>
        <div className="flex items-center gap-1">
          <button
            onClick={handleCopy}
            title="Copy source"
            className="p-1.5 rounded hover:bg-zinc-700 text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            <Copy size={13} />
          </button>
          <button
            onClick={handleOpenTab}
            title="Open in new tab"
            className="p-1.5 rounded hover:bg-zinc-700 text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            <ExternalLink size={13} />
          </button>
          <button
            onClick={onClose}
            title="Close preview"
            className="p-1.5 rounded hover:bg-zinc-700 text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            <X size={13} />
          </button>
        </div>
      </div>

      {/* iframe */}
      <div className="flex-1 overflow-hidden bg-zinc-950">
        <iframe
          srcDoc={srcDoc}
          sandbox="allow-scripts"
          title={artifact.title}
          className="w-full h-full border-0"
        />
      </div>
    </div>
  );
}

// ── Artifact card ─────────────────────────────────────────────────────────────

function ArtifactCard({
  artifact,
  active,
  onClick,
  onDelete,
}: {
  artifact: Artifact;
  active: boolean;
  onClick: () => void;
  onDelete: () => void;
}) {
  const meta = TYPE_STYLES[artifact.type];
  const TypeIcon = meta.icon;

  return (
    <button
      onClick={onClick}
      className={`group relative flex flex-col gap-2 p-4 rounded-xl border text-left transition-all w-full ${
        active
          ? 'border-indigo-500/40 bg-indigo-500/5'
          : 'border-zinc-700/50 bg-zinc-800/40 hover:border-zinc-600 hover:bg-zinc-800/80'
      }`}
    >
      {/* Thumbnail */}
      <div className="h-28 rounded-lg overflow-hidden bg-zinc-900 border border-zinc-700/50 flex items-center justify-center">
        <TypeIcon size={32} className="text-zinc-700" />
      </div>

      {/* Title row */}
      <div className="flex items-start gap-2 min-w-0">
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-zinc-200 truncate">{artifact.title}</p>
          <p className="text-xs text-zinc-500 mt-0.5">{formatRelativeTime(artifact.created_at)}</p>
        </div>
        <ChevronRight size={14} className={`flex-shrink-0 mt-0.5 transition-colors ${active ? 'text-indigo-400' : 'text-zinc-700'}`} />
      </div>

      {/* Type badge */}
      <span className={`self-start text-xs font-medium px-2 py-0.5 rounded-full ${meta.classes}`}>
        {meta.label}
      </span>

      {/* Delete */}
      <button
        onClick={e => { e.stopPropagation(); onDelete(); }}
        title="Delete artifact"
        className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 p-1 rounded bg-zinc-900 hover:bg-red-500/20 text-zinc-600 hover:text-red-400 transition-all"
      >
        <Trash2 size={12} />
      </button>
    </button>
  );
}

// ── Main view ─────────────────────────────────────────────────────────────────

export default function CanvasView() {
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [selected, setSelected] = useState<Artifact | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchArtifacts = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await sendChat('canvas_list — return raw JSON array of any HTML, SVG, React, or Mermaid artifacts you have created this session. If none, return []');
      const list = parseArtifactsFromText(resp.response ?? '');
      setArtifacts(list);
      if (list.length === 0) toast('No artifacts found in this session', { icon: 'ℹ️' });
    } catch {
      toast.error('Failed to fetch artifacts');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchArtifacts();
  }, [fetchArtifacts]);

  const handleDelete = (id: string) => {
    setArtifacts(prev => prev.filter(a => a.id !== id));
    if (selected?.id === id) setSelected(null);
    toast.success('Artifact removed');
  };

  return (
    <div className="flex flex-col flex-1 overflow-hidden bg-zinc-950">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-3 border-b border-zinc-800/60 flex-shrink-0">
        <div className="flex items-center gap-2">
          <Palette size={16} className="text-violet-400" />
          <span className="text-sm font-semibold text-zinc-300">Canvas</span>
          {artifacts.length > 0 && (
            <span className="text-xs bg-zinc-800 text-zinc-400 px-1.5 py-0.5 rounded-full">
              {artifacts.length}
            </span>
          )}
        </div>
        <button
          onClick={fetchArtifacts}
          disabled={loading}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-zinc-400 hover:text-zinc-200 bg-zinc-800 hover:bg-zinc-700 rounded-lg border border-zinc-700 transition-all disabled:opacity-50"
        >
          <RefreshCw size={12} className={loading ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {/* Body — split layout when preview open */}
      <div className="flex flex-1 overflow-hidden">
        {/* Grid */}
        <div className={`flex flex-col overflow-hidden transition-all ${selected ? 'w-72 flex-shrink-0' : 'flex-1'}`}>
          {artifacts.length === 0 && !loading ? (
            <EmptyState onFetch={fetchArtifacts} />
          ) : (
            <div className="flex-1 overflow-y-auto p-4">
              {loading && artifacts.length === 0 ? (
                <div className="flex items-center justify-center h-40">
                  <RefreshCw size={20} className="text-zinc-600 animate-spin" />
                </div>
              ) : (
                <div className={`grid gap-3 ${selected ? 'grid-cols-1' : 'grid-cols-2 md:grid-cols-3 lg:grid-cols-4'}`}>
                  {artifacts.map(art => (
                    <ArtifactCard
                      key={art.id}
                      artifact={art}
                      active={selected?.id === art.id}
                      onClick={() => setSelected(selected?.id === art.id ? null : art)}
                      onDelete={() => handleDelete(art.id)}
                    />
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Preview */}
        {selected && (
          <PreviewPanel artifact={selected} onClose={() => setSelected(null)} />
        )}
      </div>
    </div>
  );
}
