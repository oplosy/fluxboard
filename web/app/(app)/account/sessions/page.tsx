'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Globe, Monitor, Smartphone, SquareTerminal, type LucideIcon } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import { useAuth } from '@/lib/auth/context';
import { listSessions, revokeSession, logoutAll } from '@/lib/api/auth';

interface ParsedUA {
  label: string;
  icon: LucideIcon;
}

/** Small local user-agent parser: good enough for a friendly headline. */
function parseUserAgent(ua: string): ParsedUA {
  if (!ua) return { label: 'Unknown device', icon: Globe };

  let os = '';
  if (/Windows/i.test(ua)) os = 'Windows';
  else if (/Android/i.test(ua)) os = 'Android';
  else if (/iPhone|iPad|iPod|iOS/i.test(ua)) os = 'iOS';
  else if (/Mac OS X|Macintosh/i.test(ua)) os = 'macOS';
  else if (/CrOS/i.test(ua)) os = 'ChromeOS';
  else if (/Linux|X11/i.test(ua)) os = 'Linux';

  const mobile = /Android|iPhone|iPod|Mobile/i.test(ua);
  const on = (name: string) => (os ? `${name} on ${os}` : name);
  const ver = (re: RegExp) => ua.match(re)?.[1]?.split('.')[0];

  const ps = ua.match(/PowerShell\/([\d.]+)/i) ?? ua.match(/WindowsPowerShell\/([\d.]+)/i);
  if (ps) {
    const v = ps[1]?.split('.').slice(0, 2).join('.');
    return { label: on(`PowerShell ${v}`), icon: SquareTerminal };
  }
  if (/^curl\//i.test(ua)) return { label: on('curl'), icon: SquareTerminal };
  if (/^(node|undici|axios|python-requests|Go-http-client|okhttp)/i.test(ua)) {
    return { label: on(ua.split(/[\s/]/)[0] ?? 'Script'), icon: SquareTerminal };
  }

  const icon = mobile ? Smartphone : Monitor;
  if (/Claude\//i.test(ua)) return { label: on('Claude desktop'), icon };
  if (/Electron\//i.test(ua)) return { label: on('Desktop app'), icon };

  const edge = ver(/Edg(?:e|A|iOS)?\/([\d.]+)/);
  if (edge) return { label: on(`Edge ${edge}`), icon };
  const opera = ver(/OPR\/([\d.]+)/);
  if (opera) return { label: on(`Opera ${opera}`), icon };
  const firefox = ver(/(?:Firefox|FxiOS)\/([\d.]+)/);
  if (firefox) return { label: on(`Firefox ${firefox}`), icon };
  const chrome = ver(/(?:Chrome|CriOS)\/([\d.]+)/);
  if (chrome) return { label: on(`Chrome ${chrome}`), icon };
  const safari = ver(/Version\/([\d.]+).*Safari/);
  if (safari) return { label: on(`Safari ${safari}`), icon };

  return { label: os ? `Unknown browser on ${os}` : 'Unknown device', icon: Globe };
}

function formatWhen(iso: string): string {
  return new Date(iso).toLocaleString();
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const s = Math.round(diff / 1000);
  if (Number.isNaN(s)) return iso;
  if (s < 45) return 'just now';
  const units: [number, string][] = [
    [60, 'minute'],
    [3600, 'hour'],
    [86400, 'day'],
    [604800, 'week'],
    [2592000, 'month'],
    [31536000, 'year'],
  ];
  let value = Math.max(1, Math.round(s / 60));
  let unit = 'minute';
  for (const [secs, name] of units) {
    if (s >= secs) {
      value = Math.round(s / secs);
      unit = name;
    }
  }
  return `${value} ${unit}${value === 1 ? '' : 's'} ago`;
}

export default function SessionsPage() {
  const queryClient = useQueryClient();
  const router = useRouter();
  const { logout } = useAuth();
  const { toast } = useToast();
  const [confirmingId, setConfirmingId] = useState<string | null>(null);
  const [confirmAll, setConfirmAll] = useState(false);

  const { data, isLoading, isError } = useQuery({
    queryKey: ['sessions'],
    queryFn: listSessions,
  });

  const revoke = useMutation({
    mutationFn: (id: string) => revokeSession(id),
    onSuccess: async () => {
      setConfirmingId(null);
      await queryClient.invalidateQueries({ queryKey: ['sessions'] });
      toast({ title: 'Session revoked', variant: 'success' });
    },
  });

  const logoutEverywhere = useMutation({
    mutationFn: logoutAll,
    onSuccess: async () => {
      await logout();
      router.replace('/login');
    },
  });

  return (
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle>Active sessions</CardTitle>
        <CardDescription>Devices currently signed in to your account. Revoke any you do not recognise.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-[88px] w-full" />
            <Skeleton className="h-[88px] w-full" />
          </div>
        ) : isError ? (
          <p className="text-sm text-destructive">Couldn&apos;t load sessions.</p>
        ) : (
          <ul className="divide-y divide-border overflow-hidden rounded-md border border-border">
            {data?.items.map((s) => {
              const parsed = parseUserAgent(s.user_agent);
              const Icon = parsed.icon;
              const confirming = confirmingId === s.id;
              return (
                <li key={s.id} className="flex flex-wrap items-start gap-x-4 gap-y-3 p-4 sm:flex-nowrap">
                  <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-secondary text-muted-foreground">
                    <Icon className="h-4 w-4" />
                  </span>

                  <div className="min-w-[60%] flex-1 space-y-1 sm:min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-medium">{parsed.label}</p>
                      {s.current ? <Badge variant="success">This device</Badge> : null}
                    </div>
                    {s.user_agent ? (
                      <div
                        className="scrollbar-thin overflow-x-auto whitespace-nowrap rounded-sm bg-muted/60 px-2 py-1 font-mono text-xs text-muted-foreground"
                        tabIndex={0}
                        role="region"
                        aria-label="Full user agent"
                      >
                        {s.user_agent}
                      </div>
                    ) : null}
                    <p className="text-xs text-muted-foreground">
                      {s.ip ? `${s.ip} · ` : ''}
                      <span title={formatWhen(s.last_used_at)}>Last used {relativeTime(s.last_used_at)}</span>
                    </p>
                  </div>

                  <div className="flex shrink-0 items-center gap-2">
                    {s.current ? null : confirming ? (
                      <>
                        <Button
                          variant="destructive"
                          size="sm"
                          isLoading={revoke.isPending}
                          onClick={() => revoke.mutate(s.id)}
                        >
                          Confirm
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => setConfirmingId(null)} disabled={revoke.isPending}>
                          Cancel
                        </Button>
                      </>
                    ) : (
                      <Button variant="outline" size="sm" onClick={() => setConfirmingId(s.id)}>
                        Revoke
                      </Button>
                    )}
                  </div>
                </li>
              );
            })}
            {data && data.items.length === 0 ? (
              <li className="p-6 text-center text-sm text-muted-foreground">No active sessions.</li>
            ) : null}
          </ul>
        )}

        <div className="flex flex-wrap items-center gap-2">
          {confirmAll ? (
            <>
              <span className="text-sm text-muted-foreground">Sign out everywhere, including this device?</span>
              <Button
                variant="destructive"
                size="sm"
                isLoading={logoutEverywhere.isPending}
                onClick={() => logoutEverywhere.mutate()}
              >
                Yes, log out of all
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setConfirmAll(false)} disabled={logoutEverywhere.isPending}>
                Cancel
              </Button>
            </>
          ) : (
            <Button variant="destructive" onClick={() => setConfirmAll(true)}>
              Log out of all sessions
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
