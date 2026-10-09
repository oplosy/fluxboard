'use client';

import Link from 'next/link';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Building2, Bell, ChevronsUpDown, Plus, User, LogOut, Check, Shield, Menu, Search } from 'lucide-react';

import { cn } from '@/lib/utils';
import { useOrg } from '@/lib/org/context';
import { usePresence } from '@/lib/motion/hooks';
import { listMyOrgs } from '@/lib/api/orgs';
import { getUnreadCount } from '@/lib/api/notifications';
import { getMe } from '@/lib/api/user';
import { Avatar } from '@/components/ui/avatar';

export function Topbar({ onMobileMenuToggle }: { onMobileMenuToggle?: () => void }) {
  const { org, slug, role } = useOrg();
  const router = useRouter();

  // ⌘K / Ctrl+K jumps to search from anywhere in the org shell.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        router.push(`/app/${slug}/search`);
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [router, slug]);

  return (
    <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center gap-2 border-b border-border bg-topbar/85 px-4 backdrop-blur-md lg:px-6">
      {onMobileMenuToggle ? (
        <button
          type="button"
          onClick={onMobileMenuToggle}
          className="flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary md:hidden"
          aria-label="Open sidebar"
        >
          <Menu className="h-5 w-5" />
        </button>
      ) : null}

      <OrgSwitcher currentSlug={slug} currentName={org.name} logoUrl={org.logo_url} role={role} />
      <Breadcrumb slug={slug} />

      <div className="ml-auto flex items-center gap-1">
        <Link
          href={`/app/${slug}/search`}
          className="group hidden h-9 items-center gap-3 rounded-md border border-border bg-card px-3 text-xs text-muted-foreground transition-[border-color,color,width] duration-300 hover:border-foreground/40 hover:text-foreground sm:flex"
          aria-label="Search"
        >
          <Search className="h-3.5 w-3.5 transition-transform duration-500 ease-spring group-hover:-rotate-12 group-hover:scale-110" />
          <span className="pr-6">Search tasks…</span>
          <kbd className="rounded-sm border border-border bg-background px-1.5 py-0.5 font-mono text-[10px]">⌘K</kbd>
        </Link>
        <NotificationBell slug={slug} />
        <AccountMenu />
      </div>
    </header>
  );
}

/** Breadcrumb derived from the pathname; the last crumb re-animates on change. */
function Breadcrumb({ slug }: { slug: string }) {
  const pathname = usePathname();
  const base = `/app/${slug}`;
  const relative = pathname.replace(base, '').replace(/^\//, '');
  if (!relative) return null;

  const parts = relative.split('/').filter(Boolean);
  const labels = parts.map((p) => decodeURIComponent(p).replace(/-/g, ' '));

  return (
    <div className="hidden min-w-0 items-center gap-1.5 font-mono text-[12px] text-muted-foreground md:flex">
      {labels.map((label, i) => {
        const last = i === labels.length - 1;
        return (
          <span key={`${i}-${label}`} className="flex min-w-0 items-center gap-1.5">
            <span className="text-foreground/25">/</span>
            <span className={cn('truncate', last && 'animate-slide-down text-foreground')}>{label}</span>
          </span>
        );
      })}
    </div>
  );
}

/** Click-outside dropdown with enter/exit animation (no external dep). */
function DropdownMenu({
  button,
  children,
  align = 'left',
  label,
}: {
  button: (open: boolean) => ReactNode;
  children: (close: () => void) => ReactNode;
  align?: 'left' | 'right';
  label: string;
}) {
  const [open, setOpen] = useState(false);
  const { mounted, state } = usePresence(open, 180);
  const pathname = usePathname();
  const first = useRef(true);

  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    setOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  return (
    <div className="relative">
      <button type="button" onClick={() => setOpen((v) => !v)} className="flex items-center" aria-haspopup="menu" aria-expanded={open} aria-label={label}>
        {button(open)}
      </button>
      {mounted ? (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} aria-hidden />
          <div
            role="menu"
            data-state={state}
            className={cn(
              'glass absolute top-full z-20 mt-2 min-w-60 rounded-lg p-1.5',
              'transition-[opacity,transform] duration-200 ease-out-expo',
              'data-[state=closed]:-translate-y-1 data-[state=closed]:scale-[0.97] data-[state=closed]:opacity-0',
              align === 'right' ? 'right-0 origin-top-right' : 'left-0 origin-top-left',
            )}
          >
            {children(() => setOpen(false))}
          </div>
        </>
      ) : null}
    </div>
  );
}

const itemClass =
  'group flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors hover:bg-secondary [&>svg]:transition-transform [&>svg]:duration-300 hover:[&>svg]:translate-x-0.5';

function OrgSwitcher({
  currentSlug,
  currentName,
  logoUrl,
  role,
}: {
  currentSlug: string;
  currentName: string;
  logoUrl?: string;
  role: string;
}) {
  const { data: orgs } = useQuery({ queryKey: ['orgs'], queryFn: listMyOrgs });

  return (
    <DropdownMenu
      label="Switch organization"
      button={(open) => (
        <span className={cn('flex items-center gap-2 rounded-md px-1.5 py-1 text-sm transition-colors hover:bg-secondary', open && 'bg-secondary')}>
          {logoUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={logoUrl} alt="" className="h-7 w-7 rounded-md object-cover" />
          ) : (
            <span className="flex h-7 w-7 items-center justify-center rounded-md bg-foreground font-display text-[13px] font-semibold text-background">
              {currentName.slice(0, 1).toUpperCase()}
            </span>
          )}
          <span className="hidden max-w-[160px] truncate font-medium sm:block">{currentName}</span>
          <span className="hidden rounded-sm border border-border px-1 font-mono text-[10px] lowercase text-muted-foreground sm:block">
            {role.toLowerCase()}
          </span>
          <ChevronsUpDown className={cn('h-3.5 w-3.5 text-muted-foreground transition-transform duration-300', open && 'rotate-180')} />
        </span>
      )}
    >
      {(close) => (
        <>
          <p className="px-2.5 pb-1 pt-1.5 font-mono text-[10px] uppercase tracking-[0.16em] text-muted-foreground">Organizations</p>
          {orgs?.map((o, i) => (
            <Link
              key={o.org_id}
              href={`/app/${o.slug}`}
              onClick={close}
              className={cn(itemClass, 'animate-slide-down justify-between')}
              style={{ animationDelay: `${i * 30}ms` }}
            >
              <span className="flex min-w-0 items-center gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-sm bg-secondary font-mono text-[10px]">
                  {o.name.slice(0, 1).toUpperCase()}
                </span>
                <span className="truncate">{o.name}</span>
              </span>
              {o.slug === currentSlug ? <Check className="h-4 w-4 shrink-0 text-signal" /> : null}
            </Link>
          ))}
          <div className="my-1.5 h-px bg-border" />
          <Link href="/app/new-organization" onClick={close} className={itemClass}>
            <Plus className="h-4 w-4" /> New organization
          </Link>
          <Link href="/app" onClick={close} className={cn(itemClass, 'text-muted-foreground')}>
            <Building2 className="h-4 w-4" /> All organizations
          </Link>
        </>
      )}
    </DropdownMenu>
  );
}

function NotificationBell({ slug }: { slug: string }) {
  const { orgId } = useOrg();
  const { data: unread } = useQuery({
    queryKey: ['unread-count', orgId],
    queryFn: () => getUnreadCount(orgId),
  });
  const prev = useRef(unread ?? 0);
  const [ring, setRing] = useState(0);

  // Swing the bell whenever the unread count goes up.
  useEffect(() => {
    const n = unread ?? 0;
    if (n > prev.current) setRing((r) => r + 1);
    prev.current = n;
  }, [unread]);

  return (
    <Link
      href={`/app/${slug}/notifications`}
      className="group relative flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
      aria-label={unread ? `Notifications, ${unread} unread` : 'Notifications'}
    >
      <Bell
        key={ring}
        className={cn('h-4 w-4 origin-top group-hover:animate-[bell-swing_0.8s_var(--ease-out-expo)]', ring > 0 && 'animate-[bell-swing_0.8s_var(--ease-out-expo)]')}
      />
      {unread && unread > 0 ? (
        <span
          key={unread}
          className="absolute right-1 top-1 flex h-4 min-w-4 animate-scale-in items-center justify-center rounded-full bg-signal px-1 font-mono text-[9px] font-semibold text-signal-foreground"
        >
          {unread > 99 ? '99+' : unread}
        </span>
      ) : null}
    </Link>
  );
}

function AccountMenu() {
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: getMe });
  const isPlatformAdmin = me?.platform_role === 'admin';

  return (
    <DropdownMenu
      align="right"
      label="Account menu"
      button={(open) => (
        <span className={cn('flex h-9 w-9 items-center justify-center rounded-full transition-shadow', open && 'ring-2 ring-signal')}>
          <Avatar name={me?.name} size="sm" />
        </span>
      )}
    >
      {(close) => (
        <>
          {me ? (
            <div className="flex items-center gap-3 px-2.5 py-2">
              <Avatar name={me.name} size="md" />
              <div className="min-w-0 text-sm">
                <p className="truncate font-medium">{me.name}</p>
                <p className="truncate font-mono text-[11px] text-muted-foreground">{me.email}</p>
              </div>
            </div>
          ) : null}
          <div className="my-1.5 h-px bg-border" />
          <Link href="/account/profile" onClick={close} className={itemClass}>
            <User className="h-4 w-4" /> Account
          </Link>
          {isPlatformAdmin ? (
            <Link href="/admin" onClick={close} className={itemClass}>
              <Shield className="h-4 w-4" /> Platform admin
            </Link>
          ) : null}
          <div className="my-1.5 h-px bg-border" />
          <Link href="/logout" onClick={close} className={cn(itemClass, 'text-destructive hover:bg-destructive/10')}>
            <LogOut className="h-4 w-4" /> Sign out
          </Link>
        </>
      )}
    </DropdownMenu>
  );
}
