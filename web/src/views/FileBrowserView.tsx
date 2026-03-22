import { useState, useEffect, useCallback } from 'react';
import {
  FolderOpen, Folder, FileText, ChevronRight, ChevronDown,
  Home, RefreshCw, Copy, AlertCircle, Loader2,
} from 'lucide-react';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { listFiles, readFile, type FileEntry } from '../lib/api';
import toast from 'react-hot-toast';

// ── Helpers ────────────────────────────────────────────────────────────────

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function langFromExt(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() ?? '';
  const MAP: Record<string, string> = {
    ts: 'typescript', tsx: 'tsx', js: 'javascript', jsx: 'jsx',
    go: 'go', py: 'python', sh: 'bash', bash: 'bash', zsh: 'bash',
    json: 'json', yaml: 'yaml', yml: 'yaml', toml: 'toml',
    md: 'markdown', css: 'css', html: 'html', xml: 'xml',
    sql: 'sql', rs: 'rust', c: 'c', cpp: 'cpp', h: 'c',
    tf: 'hcl', hcl: 'hcl', dockerfile: 'docker',
  };
  return MAP[ext] ?? 'text';
}

// ── Breadcrumb ─────────────────────────────────────────────────────────────

interface BreadcrumbProps {
  path: string;
  onNavigate: (p: string) => void;
}

function Breadcrumb({ path, onNavigate }: BreadcrumbProps) {
  const parts = path === '.' ? [] : path.split('/').filter(Boolean);

  return (
    <div className="flex items-center gap-1 text-xs text-zinc-400 min-w-0 flex-wrap">
      <button
        onClick={() => onNavigate('.')}
        className="flex items-center gap-1 text-zinc-300 hover:text-indigo-400 transition-colors flex-shrink-0"
      >
        <Home size={12} />
        <span>workspace</span>
      </button>
      {parts.map((part, i) => {
        const targetPath = parts.slice(0, i + 1).join('/');
        const isLast = i === parts.length - 1;
        return (
          <span key={i} className="flex items-center gap-1 min-w-0">
            <ChevronRight size={11} className="flex-shrink-0 text-zinc-600" />
            {isLast ? (
              <span className="text-zinc-200 truncate">{part}</span>
            ) : (
              <button
                onClick={() => onNavigate(targetPath)}
                className="hover:text-indigo-400 transition-colors truncate"
              >
                {part}
              </button>
            )}
          </span>
        );
      })}
    </div>
  );
}

// ── File Tree Node ─────────────────────────────────────────────────────────

interface TreeNode {
  entry: FileEntry;
  path: string;         // workspace-relative path
  children?: TreeNode[];
  loaded: boolean;
  expanded: boolean;
}

interface TreeItemProps {
  node: TreeNode;
  depth: number;
  selectedPath: string | null;
  onSelect: (node: TreeNode) => void;
  onToggle: (node: TreeNode) => void;
  loadingPath: string | null;
}

function TreeItem({ node, depth, selectedPath, onSelect, onToggle, loadingPath }: TreeItemProps) {
  const isSelected = selectedPath === node.path;
  const isLoading = loadingPath === node.path;
  const indent = depth * 12;

  return (
    <div>
      <button
        onClick={() => node.entry.is_dir ? onToggle(node) : onSelect(node)}
        style={{ paddingLeft: `${12 + indent}px` }}
        className={`w-full flex items-center gap-1.5 py-0.5 pr-2 text-xs transition-colors group text-left ${
          isSelected
            ? 'bg-indigo-500/15 text-indigo-300'
            : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'
        }`}
      >
        {node.entry.is_dir ? (
          <>
            {isLoading ? (
              <Loader2 size={12} className="flex-shrink-0 animate-spin text-zinc-500" />
            ) : node.expanded ? (
              <ChevronDown size={12} className="flex-shrink-0 text-zinc-500" />
            ) : (
              <ChevronRight size={12} className="flex-shrink-0 text-zinc-500" />
            )}
            {node.expanded ? (
              <FolderOpen size={13} className="flex-shrink-0 text-amber-400" />
            ) : (
              <Folder size={13} className="flex-shrink-0 text-amber-400/70" />
            )}
          </>
        ) : (
          <>
            <span style={{ width: 12 }} className="flex-shrink-0" />
            <FileText size={13} className={`flex-shrink-0 ${isSelected ? 'text-indigo-400' : 'text-zinc-500 group-hover:text-zinc-400'}`} />
          </>
        )}
        <span className="truncate flex-1">{node.entry.name}</span>
        {!node.entry.is_dir && node.entry.size > 0 && (
          <span className="text-zinc-600 flex-shrink-0 ml-1">{formatSize(node.entry.size)}</span>
        )}
      </button>

      {node.entry.is_dir && node.expanded && node.children && (
        <div>
          {node.children.length === 0 ? (
            <div
              style={{ paddingLeft: `${24 + indent}px` }}
              className="py-0.5 text-xs text-zinc-600 italic"
            >
              empty
            </div>
          ) : (
            node.children.map(child => (
              <TreeItem
                key={child.path}
                node={child}
                depth={depth + 1}
                selectedPath={selectedPath}
                onSelect={onSelect}
                onToggle={onToggle}
                loadingPath={loadingPath}
              />
            ))
          )}
        </div>
      )}
    </div>
  );
}

// ── File Content Panel ─────────────────────────────────────────────────────

interface ContentPanelProps {
  path: string | null;
  content: string | null;
  loading: boolean;
  error: string | null;
}

function ContentPanel({ path, content, loading, error }: ContentPanelProps) {
  const copyPath = () => {
    if (!path) return;
    navigator.clipboard.writeText(path).then(() => toast.success('Path copied'));
  };

  if (!path) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-zinc-600 gap-3">
        <FileText size={40} strokeWidth={1} />
        <p className="text-sm">Select a file to preview</p>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 size={28} className="animate-spin text-zinc-600" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-red-400">
        <AlertCircle size={36} strokeWidth={1.5} />
        <p className="text-sm text-center px-6">{error}</p>
      </div>
    );
  }

  const filename = path.split('/').pop() ?? path;
  const lang = langFromExt(filename);

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* File header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-zinc-800 flex-shrink-0 gap-3">
        <div className="flex items-center gap-2 min-w-0">
          <FileText size={14} className="text-zinc-400 flex-shrink-0" />
          <span className="text-sm text-zinc-200 truncate">{filename}</span>
          <span className="text-xs text-zinc-600 truncate hidden sm:block">{path}</span>
        </div>
        <button
          onClick={copyPath}
          title="Copy path"
          className="flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-200 transition-colors flex-shrink-0 px-2 py-1 rounded hover:bg-zinc-800"
        >
          <Copy size={12} />
          <span>Copy path</span>
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        <SyntaxHighlighter
          language={lang}
          style={oneDark}
          customStyle={{
            margin: 0,
            borderRadius: 0,
            background: 'transparent',
            fontSize: '12px',
            lineHeight: '1.6',
            minHeight: '100%',
          }}
          showLineNumbers
          lineNumberStyle={{ color: '#52525b', minWidth: '2.5em', paddingRight: '1em' }}
          wrapLongLines={false}
        >
          {content ?? ''}
        </SyntaxHighlighter>
      </div>
    </div>
  );
}

// ── Main View ──────────────────────────────────────────────────────────────

export default function FileBrowserView() {
  const [rootNodes, setRootNodes] = useState<TreeNode[]>([]);
  const [treeLoading, setTreeLoading] = useState(true);
  const [loadingPath, setLoadingPath] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<TreeNode | null>(null);
  const [fileContent, setFileContent] = useState<string | null>(null);
  const [contentLoading, setContentLoading] = useState(false);
  const [contentError, setContentError] = useState<string | null>(null);
  const [currentDir, setCurrentDir] = useState('.');

  const loadDirectory = useCallback(async (path: string): Promise<TreeNode[]> => {
    const res = await listFiles(path);
    if (!res.entries) return [];
    const sorted = [...res.entries].sort((a, b) => {
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
    return sorted.map(entry => ({
      entry,
      path: path === '.' ? entry.name : `${path}/${entry.name}`,
      children: undefined,
      loaded: false,
      expanded: false,
    }));
  }, []);

  const refresh = useCallback(async () => {
    setTreeLoading(true);
    try {
      const nodes = await loadDirectory('.');
      setRootNodes(nodes);
      setCurrentDir('.');
    } catch (err) {
      toast.error('Failed to load workspace files');
    } finally {
      setTreeLoading(false);
    }
  }, [loadDirectory]);

  useEffect(() => { refresh(); }, [refresh]);

  const handleToggle = useCallback(async (node: TreeNode) => {
    const toggle = (nodes: TreeNode[]): TreeNode[] =>
      nodes.map(n => {
        if (n.path === node.path) {
          if (n.expanded) {
            return { ...n, expanded: false };
          }
          return { ...n, expanded: true };
        }
        if (n.children) return { ...n, children: toggle(n.children) };
        return n;
      });

    // Expand immediately so the chevron flips; load children if needed
    setRootNodes(prev => toggle(prev));

    if (!node.loaded && !node.expanded) {
      setLoadingPath(node.path);
      try {
        const children = await loadDirectory(node.path);
        const patch = (nodes: TreeNode[]): TreeNode[] =>
          nodes.map(n => {
            if (n.path === node.path) return { ...n, children, loaded: true };
            if (n.children) return { ...n, children: patch(n.children) };
            return n;
          });
        setRootNodes(prev => patch(prev));
      } catch {
        toast.error(`Failed to load ${node.entry.name}`);
      } finally {
        setLoadingPath(null);
      }
    }
  }, [loadDirectory]);

  const handleSelect = useCallback(async (node: TreeNode) => {
    setSelectedNode(node);
    setCurrentDir(node.path.includes('/') ? node.path.substring(0, node.path.lastIndexOf('/')) : '.');
    setContentLoading(true);
    setContentError(null);
    setFileContent(null);
    try {
      const res = await readFile(node.path);
      setFileContent(res.content);
    } catch (err) {
      setContentError((err as Error).message || 'Failed to read file');
    } finally {
      setContentLoading(false);
    }
  }, []);

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Top bar */}
      <div className="flex items-center gap-3 px-4 h-12 border-b border-zinc-800 flex-shrink-0 bg-zinc-900">
        <Breadcrumb
          path={selectedNode?.path ?? currentDir}
          onNavigate={() => {
            setSelectedNode(null);
            setFileContent(null);
            setCurrentDir('.');
          }}
        />
        <div className="ml-auto flex-shrink-0">
          <button
            onClick={refresh}
            title="Refresh"
            disabled={treeLoading}
            className="p-1.5 rounded text-zinc-500 hover:text-zinc-200 hover:bg-zinc-800 transition-colors disabled:opacity-40"
          >
            <RefreshCw size={14} className={treeLoading ? 'animate-spin' : ''} />
          </button>
        </div>
      </div>

      {/* Body */}
      <div className="flex flex-1 overflow-hidden">
        {/* File tree */}
        <div className="w-56 flex-shrink-0 border-r border-zinc-800 overflow-y-auto bg-zinc-900/50">
          {treeLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 size={18} className="animate-spin text-zinc-600" />
            </div>
          ) : rootNodes.length === 0 ? (
            <div className="text-center text-xs text-zinc-600 py-8 px-4">
              No files found in workspace
            </div>
          ) : (
            <div className="py-2">
              {rootNodes.map(node => (
                <TreeItem
                  key={node.path}
                  node={node}
                  depth={0}
                  selectedPath={selectedNode?.path ?? null}
                  onSelect={handleSelect}
                  onToggle={handleToggle}
                  loadingPath={loadingPath}
                />
              ))}
            </div>
          )}
        </div>

        {/* Content panel */}
        <div className="flex-1 overflow-hidden bg-zinc-950">
          <ContentPanel
            path={selectedNode?.path ?? null}
            content={fileContent}
            loading={contentLoading}
            error={contentError}
          />
        </div>
      </div>
    </div>
  );
}
