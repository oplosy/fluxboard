'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Download, Upload } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Select } from '@/components/ui/select';
import { useToast } from '@/components/ui/toast';
import { ApiError } from '@/lib/api/client';
import { useOrg } from '@/lib/org/context';
import {
  IMPORT_FIELDS,
  downloadExport,
  importTasks,
  previewImport,
  type ImportField,
  type ImportMapping,
  type ImportPreview,
} from '@/lib/api/transfer';
import type { Project } from '@/lib/api/types';
import { SettingsSection } from '@/components/board/settings-section';

const MAX_FILE_BYTES = 4 * 1024 * 1024;

function newKey() {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `${Date.now()}`;
}

function message(err: unknown) {
  return err instanceof ApiError ? err.message : 'Something went wrong';
}

// Import & export (FR-TASK-012/013). The server enforces the project role:
// CONTRIBUTOR to import, VIEWER to export.
export function TransferSection({ project }: { project: Project }) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [fileName, setFileName] = useState('');
  const [csv, setCsv] = useState('');
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  // One key per prepared import: a retry after a failure reuses it, so rows
  // that already landed are not created twice. Changing input mints a new one.
  const [idemKey, setIdemKey] = useState(newKey);

  const check = useMutation({
    mutationFn: (v: { csv: string; mapping?: ImportMapping }) =>
      previewImport(orgId, project.id, v.csv, v.mapping),
    onSuccess: (p) => {
      setPreview(p);
      setIdemKey(newKey());
    },
    onError: (err) =>
      toast({ title: 'Couldn’t read the CSV', description: message(err), variant: 'error' }),
  });

  const run = useMutation({
    mutationFn: () => importTasks(orgId, project.id, csv, preview?.mapping ?? {}, idemKey),
    onSuccess: (res) => {
      toast({
        title: `Imported ${res.created} task${res.created === 1 ? '' : 's'}`,
        description: res.existing
          ? `${res.existing} were already imported by an earlier attempt.`
          : undefined,
        variant: 'success',
      });
      reset();
      queryClient.invalidateQueries();
    },
    onError: (err) =>
      toast({
        title: 'Import failed — you can retry safely',
        description: message(err),
        variant: 'error',
      }),
  });

  const exporting = useMutation({
    mutationFn: (format: 'csv' | 'json') => downloadExport(orgId, project.id, project.key, format),
    onError: (err) =>
      toast({ title: 'Export failed', description: message(err), variant: 'error' }),
  });

  function reset() {
    setFileName('');
    setCsv('');
    setPreview(null);
  }

  async function onFile(file: File | undefined) {
    if (!file) return;
    if (file.size > MAX_FILE_BYTES) {
      toast({
        title: 'File too large',
        description: 'Import at most 4 MB at a time.',
        variant: 'error',
      });
      return;
    }
    const text = await file.text();
    setFileName(file.name);
    setCsv(text);
    check.mutate({ csv: text });
  }

  function remap(field: ImportField, header: string) {
    if (!preview) return;
    const mapping = { ...preview.mapping, [field]: header };
    if (!header) delete mapping[field];
    check.mutate({ csv, mapping });
  }

  const headerOptions = [
    { value: '', label: '— Not imported —' },
    ...(preview?.headers ?? []).map((h) => ({ value: h, label: h })),
  ];

  return (
    <SettingsSection
      title="Import & export"
      description="Bring tasks in from a CSV (Excel, Google Sheets, Trello) or download this project’s tasks. Imported tasks are created like any other task."
    >
      <div className="space-y-6">
        <div className="space-y-2">
          <p className="text-sm font-medium">Export</p>
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => exporting.mutate('csv')}
              isLoading={exporting.isPending && exporting.variables === 'csv'}
            >
              <Download className="h-4 w-4" /> Tasks (CSV)
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => exporting.mutate('json')}
              isLoading={exporting.isPending && exporting.variables === 'json'}
            >
              <Download className="h-4 w-4" /> Full export (JSON)
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            JSON includes comments, labels and attachment details. Attachment files stay
            downloadable from each task.
          </p>
        </div>

        <div className="space-y-3">
          <p className="text-sm font-medium">Import from CSV</p>
          <label className="flex cursor-pointer items-center gap-2 rounded-md border border-dashed border-border px-4 py-3 text-[13px] text-muted-foreground hover:border-foreground/40">
            <Upload className="h-4 w-4 shrink-0" />
            <span className="min-w-0 truncate">
              {fileName || 'Choose a .csv file (first row = column names, up to 1000 rows)'}
            </span>
            <input
              type="file"
              accept=".csv,text/csv"
              className="sr-only"
              onChange={(e) => {
                void onFile(e.target.files?.[0]);
                e.target.value = '';
              }}
            />
          </label>

          {preview ? (
            <>
              <div className="grid gap-2 sm:grid-cols-2">
                {IMPORT_FIELDS.map(({ field, label }) => (
                  <label key={field} className="flex flex-col gap-1 text-[13px]">
                    <span className="text-muted-foreground">
                      {label}
                      {field === 'title' ? ' *' : ''}
                    </span>
                    <Select
                      size="sm"
                      className="w-full"
                      aria-label={`CSV column for ${label}`}
                      value={preview.mapping[field] ?? ''}
                      onValueChange={(v) => remap(field, v)}
                      disabled={check.isPending}
                      options={headerOptions}
                    />
                  </label>
                ))}
              </div>

              <p className="text-[13px]" aria-live="polite">
                <strong className="font-medium">{preview.valid}</strong> ready to import
                {preview.invalid ? (
                  <>
                    , <strong className="font-medium text-destructive">{preview.invalid}</strong>{' '}
                    with errors — fix the file or the mapping and choose it again.
                  </>
                ) : null}
              </p>

              <div className="max-h-80 overflow-auto rounded-md border border-border">
                <table className="w-full text-left text-xs">
                  <thead className="sticky top-0 bg-card text-muted-foreground">
                    <tr>
                      <th className="px-2 py-1.5 font-medium">Line</th>
                      <th className="px-2 py-1.5 font-medium">Title</th>
                      <th className="px-2 py-1.5 font-medium">Column</th>
                      <th className="px-2 py-1.5 font-medium">Assignee</th>
                      <th className="px-2 py-1.5 font-medium">Due</th>
                      <th className="px-2 py-1.5 font-medium">Priority</th>
                      <th className="px-2 py-1.5 font-medium">Labels</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {preview.rows.map((r) => (
                      <tr key={r.line} className={r.errors.length ? 'bg-destructive/5' : undefined}>
                        <td className="px-2 py-1.5 font-mono text-muted-foreground">{r.line}</td>
                        <td className="px-2 py-1.5">
                          {r.title}
                          {r.errors.map((e) => (
                            <span key={e} className="block text-destructive">
                              {e}
                            </span>
                          ))}
                        </td>
                        <td className="px-2 py-1.5">{r.column_name}</td>
                        <td className="px-2 py-1.5">{r.assignee_email}</td>
                        <td className="px-2 py-1.5">{r.due_date?.slice(0, 10)}</td>
                        <td className="px-2 py-1.5">{r.priority === 'none' ? '' : r.priority}</td>
                        <td className="px-2 py-1.5">{r.labels.join(', ')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  onClick={() => run.mutate()}
                  isLoading={run.isPending}
                  disabled={check.isPending || preview.invalid > 0 || preview.valid === 0}
                >
                  Import {preview.valid} task{preview.valid === 1 ? '' : 's'}
                </Button>
                <Button size="sm" variant="ghost" onClick={reset} disabled={run.isPending}>
                  Cancel
                </Button>
              </div>
            </>
          ) : null}
        </div>
      </div>
    </SettingsSection>
  );
}
