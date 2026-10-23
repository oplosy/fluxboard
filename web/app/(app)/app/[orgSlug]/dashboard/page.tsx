'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, X } from 'lucide-react';

import {
  getDashboard,
  getDashboardLayout,
  saveDashboardLayout,
  type DashboardData,
  type DashboardWidget,
  type WidgetType,
} from '@/lib/api/dashboard';
import { OverdueChart, StatusChart, ThroughputChart, WorkloadChart } from '@/components/dashboard/dashboard-charts';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useToast } from '@/components/ui/toast';
import { coveredProjects, weeklyTotals, WIDGET_TITLES, workloadTotals } from '@/lib/board/dashboard';
import { useOrg } from '@/lib/org/context';
import { memberName, useOrgMembers } from '@/lib/org/use-members';

const MAX_WIDGETS = 12;

// Org dashboard (FR-AN-005): widgets over the projects the caller can see. The
// server only ever returns visible projects; the saved layout holds widget types
// and project choices per user.
export default function DashboardPage() {
  const { orgId } = useOrg();
  const qc = useQueryClient();
  const { toast } = useToast();
  const [adding, setAdding] = useState<WidgetType>('project_status');

  const data = useQuery({ queryKey: ['dashboard', orgId], queryFn: () => getDashboard(orgId) });
  const layout = useQuery({ queryKey: ['dashboard-layout', orgId], queryFn: () => getDashboardLayout(orgId) });

  const save = useMutation({
    mutationFn: (widgets: DashboardWidget[]) => saveDashboardLayout(orgId, widgets),
    onMutate: (widgets) => {
      qc.setQueryData<DashboardWidget[]>(['dashboard-layout', orgId], widgets);
    },
    onError: () => {
      toast({ title: 'Could not save the dashboard', variant: 'error' });
      void qc.invalidateQueries({ queryKey: ['dashboard-layout', orgId] });
    },
  });

  const widgets = layout.data ?? [];

  if (data.isLoading || layout.isLoading) return <div className="p-8 text-sm text-muted-foreground">Loading…</div>;
  if (data.isError || !data.data || layout.isError) {
    return <div className="p-8 text-sm text-destructive">Couldn’t load the dashboard.</div>;
  }

  return (
    <div className="px-6 py-6">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-sm text-muted-foreground">Across the projects you can see. Your layout is saved to your account.</p>
        </div>
        <div className="flex items-center gap-2">
          <select
            aria-label="Widget type"
            className="h-8 rounded-md border border-border bg-background px-2 text-sm"
            value={adding}
            onChange={(e) => setAdding(e.target.value as WidgetType)}
          >
            {(Object.keys(WIDGET_TITLES) as WidgetType[]).map((t) => (
              <option key={t} value={t}>
                {WIDGET_TITLES[t]}
              </option>
            ))}
          </select>
          <Button
            size="sm"
            variant="outline"
            disabled={widgets.length >= MAX_WIDGETS}
            onClick={() => save.mutate([...widgets, { id: crypto.randomUUID().slice(0, 8), type: adding, project_ids: [] }])}
          >
            <Plus className="mr-1 h-4 w-4" aria-hidden /> Add widget
          </Button>
        </div>
      </div>

      {widgets.length === 0 ? (
        <p className="text-sm text-muted-foreground">No widgets yet. Add one above.</p>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2">
          {widgets.map((w) => (
            <WidgetCard
              key={w.id}
              widget={w}
              data={data.data}
              onChange={(nw) => save.mutate(widgets.map((x) => (x.id === w.id ? nw : x)))}
              onRemove={() => save.mutate(widgets.filter((x) => x.id !== w.id))}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function WidgetCard({
  widget,
  data,
  onChange,
  onRemove,
}: {
  widget: DashboardWidget;
  data: DashboardData;
  onChange: (w: DashboardWidget) => void;
  onRemove: () => void;
}) {
  const { byId } = useOrgMembers();
  const covered = coveredProjects(data, widget.project_ids);
  const selected = new Set(widget.project_ids);
  const toggle = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onChange({ ...widget, project_ids: [...next] });
  };

  let body: React.ReactNode;
  if (widget.type === 'project_status') {
    body = <StatusChart rows={covered.map((p) => ({ name: p.key, open: p.open, done: p.done }))} />;
  } else if (widget.type === 'overdue') {
    body = <OverdueChart rows={covered.map((p) => ({ name: p.key, overdue: p.overdue }))} />;
  } else if (widget.type === 'throughput') {
    body = <ThroughputChart rows={weeklyTotals(data, covered)} />;
  } else {
    body = <WorkloadChart rows={workloadTotals(data, covered).map((r) => ({ name: memberName(byId, r.userId), open: r.open }))} />;
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-2 pb-2">
        <div>
          <CardTitle className="text-base">{WIDGET_TITLES[widget.type]}</CardTitle>
          <details className="mt-1 text-xs text-muted-foreground">
            <summary className="cursor-pointer">
              {widget.project_ids.length === 0 ? 'All projects' : `${covered.length} of ${data.projects.length} projects`}
            </summary>
            <fieldset className="mt-2 grid gap-1">
              <legend className="sr-only">Projects shown in this widget</legend>
              {data.projects.map((p) => (
                <label key={p.project_id} className="flex items-center gap-2 text-foreground">
                  <input type="checkbox" checked={selected.has(p.project_id)} onChange={() => toggle(p.project_id)} />
                  {p.name}
                </label>
              ))}
              <span>None checked shows all projects.</span>
            </fieldset>
          </details>
        </div>
        <Button size="icon" variant="ghost" aria-label={`Remove ${WIDGET_TITLES[widget.type]}`} onClick={onRemove}>
          <X className="h-4 w-4" aria-hidden />
        </Button>
      </CardHeader>
      <CardContent>{body}</CardContent>
    </Card>
  );
}
