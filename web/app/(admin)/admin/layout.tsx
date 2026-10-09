'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Shield, Building2, ScrollText, ListChecks, LayoutDashboard, ExternalLink } from 'lucide-react';

import { cn } from '@/lib/utils';
import { useAuth } from '@/lib/auth/context';
import { getMe } from '@/lib/api/user';
import { LogoMark } from '@/components/brand/logo';
import { Spinner } from '@/components/ui/spinner';

// Platform-admin shell (docs/02 §10). A SEPARATE route group with its own dark
// chrome, deliberately outside the org shell + tenant context. Gate: the caller's
// platform_role must be `admin` (the backend additionally enforces 2FA on every
// /admin call). Non-admins are bounced to /app.
export default function AdminLayout({ children }: { children: ReactNode }) {
  const { loading, isAuthenticated } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  const me = useQuery({ queryKey: ['me'], queryFn: getMe, enabled: isAuthenticated });
  const isPlatformAdmin = me.data?.platform_role === 'admin';

  useEffect(() => {
    if (loading) return;
    if (!isAuthenticated) {
      router.replace('/login?next=/admin');
      return;
    }
    if (me.isSuccess && !isPlatformAdmin) router.replace('/app');
  }, [loading, isAuthenticated, me.isSuccess, isPlatformAdmin, router]);

  if (loading || me.isLoading || !isPlatformAdmin) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-ink text-paper/60">
        <Spinner size="lg" className="text-paper" />
        <p className="font-mono text-[11px] uppercase tracking-[0.2em]">Checking platform access</p>
      </div>
    );
  }

  const nav = [
    { href: '/admin', label: 'Dashboard', icon: LayoutDashboard, exact: true },
    { href: '/admin/tenants', label: 'Tenants', icon: Building2 },
    { href: '/admin/audit-log', label: 'Audit log', icon: ScrollText },
    { href: '/admin/jobs', label: 'Jobs', icon: ListChecks },
  ];

  return (
    <div className="dark flex min-h-screen bg-background text-foreground">
      <aside className="sticky top-0 hidden h-screen w-60 shrink-0 flex-col bg-ink text-paper md:flex">
        <div className="flex h-14 items-center gap-2.5 border-b border-paper/10 px-5">
          <LogoMark className="h-[22px] w-[22px]" />
          <span className="font-display text-[15px] font-semibold tracking-[-0.02em]">Platform</span>
          <span className="ml-auto flex items-center gap-1 rounded-sm bg-signal px-1.5 py-0.5 font-mono text-[9px] font-semibold uppercase tracking-[0.12em] text-signal-foreground">
            <Shield className="h-2.5 w-2.5" /> admin
          </span>
        </div>
        <nav className="flex flex-col gap-1 p-3">
          {nav.map((it) => {
            const active = it.exact ? pathname === it.href : pathname.startsWith(it.href);
            const Icon = it.icon;
            return (
              <Link
                key={it.href}
                href={it.href}
                className={cn(
                  'group relative flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors duration-200',
                  active ? 'bg-paper/10 font-medium text-paper' : 'text-paper/55 hover:bg-paper/5 hover:text-paper',
                )}
              >
                {active ? <span className="absolute left-0 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r-sm bg-signal" /> : null}
                <Icon className="h-4 w-4 transition-transform duration-300 ease-spring group-hover:scale-110" />
                {it.label}
              </Link>
            );
          })}
          <div className="my-2 border-t border-paper/10" />
          <Link
            href="/app"
            className="flex items-center gap-2.5 rounded-md px-3 py-2 text-sm text-paper/55 transition-colors hover:bg-paper/5 hover:text-paper"
          >
            <ExternalLink className="h-4 w-4" /> Back to app
          </Link>
        </nav>
      </aside>
      <main className="min-w-0 flex-1 px-5 py-8 lg:px-10">{children}</main>
    </div>
  );
}
