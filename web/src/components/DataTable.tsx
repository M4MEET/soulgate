import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table';
import { useState } from 'react';
import { ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react';

interface Props<T> {
  data: T[];
  columns: ColumnDef<T, unknown>[];
  filterPlaceholder?: string;
  globalFilter?: string;
  onGlobalFilterChange?: (v: string) => void;
}

export default function DataTable<T>({
  data,
  columns,
  globalFilter = '',
  onGlobalFilterChange,
}: Props<T>) {
  const [sorting, setSorting] = useState<SortingState>([]);

  const table = useReactTable({
    data,
    columns,
    state: { sorting, globalFilter },
    onSortingChange: setSorting,
    onGlobalFilterChange,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
  });

  return (
    <div className="overflow-auto rounded-xl border border-zinc-700/50">
      <table className="w-full text-sm border-collapse">
        <thead>
          {table.getHeaderGroups().map(hg => (
            <tr key={hg.id} className="border-b border-zinc-700/50 bg-zinc-800/60">
              {hg.headers.map(header => (
                <th
                  key={header.id}
                  onClick={header.column.getToggleSortingHandler()}
                  className="px-4 py-2.5 text-left text-xs font-semibold text-zinc-400 uppercase tracking-wide whitespace-nowrap select-none"
                  style={{ cursor: header.column.getCanSort() ? 'pointer' : 'default' }}
                >
                  <div className="flex items-center gap-1">
                    {flexRender(header.column.columnDef.header, header.getContext())}
                    {header.column.getCanSort() && (
                      <span className="text-zinc-600">
                        {header.column.getIsSorted() === 'asc' ? (
                          <ChevronUp size={12} />
                        ) : header.column.getIsSorted() === 'desc' ? (
                          <ChevronDown size={12} />
                        ) : (
                          <ChevronsUpDown size={12} />
                        )}
                      </span>
                    )}
                  </div>
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row, i) => (
            <tr
              key={row.id}
              className={`border-b border-zinc-800/50 hover:bg-zinc-800/30 transition-colors ${
                i % 2 === 0 ? 'bg-zinc-900/20' : ''
              }`}
            >
              {row.getVisibleCells().map(cell => (
                <td key={cell.id} className="px-4 py-2.5 text-zinc-300">
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {table.getRowModel().rows.length === 0 && (
        <div className="text-center text-zinc-500 text-sm py-8">No data</div>
      )}
    </div>
  );
}
