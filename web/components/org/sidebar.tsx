'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useCallback, useLayoutEffect, useRef, useState } from 'react';
import {
  Home,
  FolderKanban,
  CheckSquare,
  Search,
  Trash2,
  Brain,
  Users,
  Settings,
  CreditCard,
  PanelLeftClose,
  PanelLeft,
  X,
  type LucideIcon,
} from 'lucide-react';

import { cn } from '@/lib/utils';
import { useOrg } from '@/lib/org/context';
import { usePresence } from '@/lib/motion/hooks';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { Tooltip } from '@/components/ui/tooltip';
import { LogoMark } from '@/components/brand/logo';

interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
  /** Exact-match active (home), else prefix-match. */
  exact?: boolean;
  section: 'workspace' | 'admin';
}

function useNavItems(): NavItem[] {
  const { slug, isAdmin } = useOrg();
  const base = `/app/${slug}`;
  const items: NavItem[] = [
    { href: base, label: 'Home', icon: Home, exact: true, section: 'workspace' },
    { href: `${base}/projects`, label: 'Projects', icon: FolderKanban, section: 'workspace' },
    { href: `${base}/my-tasks`, label: 'My tasks', icon: CheckSquare, section: 'workspace' },
    { href: `${base}/search`, label: 'Search', icon: Search, section: 'workspace' },
    { href: `${base}/trash`, label: 'Trash', icon: Trash2, section: 'workspace' },
    { href: `${base}/ai`, label: 'AI assistant', icon: Brain, section: 'workspace' },
  ];
  if (isAdmin) {
    items.push(
      { href: `${base}/settings/members`, label: 'Members', icon: Users, section: 'admin' },
      { href: `${base}/settings`, label: 'Settings', icon: Settings, section: 'admin' },
      { href: `${base}/billing`, label: 'Billing', icon: CreditCard, section: 'admin' },
    );
  }
  return items;
}

// "Settings" must not light up on /settings/members (which has its own item),
// so the most specific matching href wins.
function activeHref(items: NavItem[], pathname: string): string | null {
  let best: NavItem | null = null;
  for (const it of items) {
    const hit = it.exact ? pathname === it.href : pathname === it.href || pathname.startsWith(`${it.href}/`);
    if (hit && (!best || it.href.length > best.href.length)) best = it;
  }
  return best?.href ?? null;
}

// Role-aware org navigation. One shared highlight slides between items.
export function Sidebar({ collapsed, onToggle }: { collapsed: boolean; onToggle: () => void }) {
  const pathname = usePathname();
  const { slug } = useOrg();
  const items = useNavItems();
  const active = activeHref(items, pathname);
  const navRef = useRef<HTMLElement>(null);
  const linkRefs = useRef(new Map<string, HTMLAnchorElement>());
  const [indicator, setIndicator] = useState<{ y: number; h: number; ready: boolean }>({ y: 0, h: 0, ready: false });

  const measure = useCallback(() => {
    const el = active ? linkRefs.current.get(active) : undefined;
    const nav = navRef.current;
    if (!el || !nav) {
      setIndicator((s) => ({ ...s, ready: false }));
      return;
    }
    // Measure against the nav box: collapsed links sit inside tooltip wrappers,
    // so offsetTop would be relative to the wrong parent.
    const r = el.getBoundingClientRect();
    const n = nav.getBoundingClientRect();
    setIndicator({ y: r.top - n.top + nav.scrollTop, h: r.height, ready: true });
  }, [active]);

  useLayoutEffect(() => {
    measure();
  }, [measure, collapsed]);

  const workspace = items.filter((i) => i.section === 'workspace');
  const admin = items.filter((i) => i.section === 'admin');

  return (
    <aside
      className={cn(
        'sticky top-0 hidden h-screen shrink-0 flex-col border-r border-sidebar-border bg-sidebar transition-[width] duration-500 ease-out-expo md:flex',
        collapsed ? 'w-[68px]' : 'w-60',
      )}
      onTransitionEnd={measure}
    >
      <div className={cn('flex h-14 items-center border-b border-sidebar-border px-5', collapsed && 'justify-center px-0')}>
        <Link href={`/app/${slug}`} className="group/logo flex items-center gap-2.5" aria-label="Workspace home">
          <LogoMark className="h-[22px] w-[22px]" />
          <span
            className={cn(
              'font-display text-[16px] font-semibold tracking-[-0.03em] transition-[opacity,transform] duration-300',
              collapsed ? 'pointer-events-none w-0 -translate-x-2 opacity-0' : 'opacity-100',
            )}
          >
            fluxboard
          </span>
        </Link>
      </div>

      <nav
        ref={navRef}
        className={cn('relative flex flex-1 flex-col px-3 py-4', collapsed ? 'overflow-visible' : 'overflow-y-auto overflow-x-hidden')}
      >
        {/* Sliding highlight */}
        <span
          aria-hidden
          className={cn(
            'pointer-events-none absolute left-3 right-3 rounded-md border border-sidebar-border bg-background shadow-[0_1px_2px_hsl(var(--ink)/0.06)]',
            'transition-[transform,height,opacity] duration-500 ease-spring',
            indicator.ready ? 'opacity-100' : 'opacity-0',
          )}
          style={{ transform: `translateY(${indicator.y}px)`, height: indicator.h, top: 0 }}
        >
          <span className="absolute left-0 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r-sm bg-sidebar-active" />
        </span>

        <SectionLabel collapsed={collapsed}>Workspace</SectionLabel>
        {workspace.map((it, i) => (
          <NavLink
            key={it.href}
            item={it}
            index={i}
            active={active === it.href}
            collapsed={collapsed}
            linkRef={(el) => {
              if (el) linkRefs.current.set(it.href, el);
              else linkRefs.current.delete(it.href);
            }}
          />
        ))}

        {admin.length > 0 ? (
          <>
            <SectionLabel collapsed={collapsed} className="mt-6">
              Administration
            </SectionLabel>
            {admin.map((it, i) => (
              <NavLink
                key={it.href}
                item={it}
                index={workspace.length + i}
                active={active === it.href}
                collapsed={collapsed}
                linkRef={(el) => {
                  if (el) linkRefs.current.set(it.href, el);
                  else linkRefs.current.delete(it.href);
                }}
              />
            ))}
          </>
        ) : null}
      </nav>

      <div className={cn('flex items-center gap-1 border-t border-sidebar-border p-3', collapsed ? 'flex-col' : 'justify-between')}>
        <ThemeToggle />
        <button
          type="button"
          onClick={onToggle}
          className="flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
        </button>
      </div>
    </aside>
  );
}

function SectionLabel({ children, collapsed, className }: { children: string; collapsed: boolean; className?: string }) {
  return (
    <p
      className={cn(
        'mb-2 flex h-4 items-center px-3 font-mono text-[10px] font-medium uppercase tracking-[0.18em] text-sidebar-foreground/45',
        className,
      )}
    >
      {collapsed ? <span className="mx-auto h-px w-4 bg-sidebar-foreground/25" /> : children}
    </p>
  );
}

const NavLink = function NavLink({
  item,
  active,
  collapsed,
  index,
  linkRef,
}: {
  item: NavItem;
  active: boolean;
  collapsed: boolean;
  index: number;
  linkRef: (el: HTMLAnchorElement | null) => void;
}) {
  const Icon = item.icon;
  const link = (
    <Link
      ref={linkRef}
      href={item.href}
      aria-current={active ? 'page' : undefined}
      className={cn(
        'group relative z-[1] flex h-9 items-center gap-3 rounded-md px-3 text-[13.5px] transition-colors duration-200',
        'animate-[slide-in-left_0.5s_var(--ease-out-expo)_backwards]',
        active ? 'font-medium text-sidebar-foreground' : 'text-sidebar-foreground/60 hover:text-sidebar-foreground',
        collapsed && 'justify-center px-0',
      )}
      style={{ animationDelay: `${index * 35}ms` }}
    >
      <Icon
        className={cn(
          'h-4 w-4 shrink-0 transition-transform duration-300 ease-spring group-hover:scale-110',
          active && 'text-sidebar-foreground',
        )}
        strokeWidth={active ? 2.1 : 1.8}
      />
      {!collapsed ? <span className="truncate transition-transform duration-300 ease-out-expo group-hover:translate-x-0.5">{item.label}</span> : null}
    </Link>
  );

  if (collapsed) {
    return (
      <Tooltip content={item.label} side="right" className="z-50" wrapperClassName="flex w-full">
        {link}
      </Tooltip>
    );
  }
  return link;
};

/** Mobile drawer with slide-in/out and staggered links. */
export function MobileSidebar({ open, onClose }: { open: boolean; onClose: () => void }) {
  const pathname = usePathname();
  const items = useNavItems();
  const active = activeHref(items, pathname);
  const { mounted, state } = usePresence(open, 400);

  if (!mounted) return null;

  return (
    <div className="md:hidden" data-state={state}>
      <div
        className={cn(
          'fixed inset-0 z-30 bg-ink/50 backdrop-blur-[2px] transition-opacity duration-300',
          state === 'open' ? 'opacity-100' : 'opacity-0',
        )}
        onClick={onClose}
        aria-hidden
      />
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-40 flex w-72 flex-col bg-sidebar shadow-2xl transition-transform duration-400 ease-out-expo',
          state === 'open' ? 'translate-x-0' : '-translate-x-full',
        )}
        aria-label="Navigation"
      >
        <div className="flex h-14 items-center justify-between border-b border-sidebar-border px-5">
          <span className="flex items-center gap-2.5">
            <LogoMark className="h-[22px] w-[22px]" />
            <span className="font-display font-semibold tracking-[-0.03em]">fluxboard</span>
          </span>
          <button type="button" onClick={onClose} className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary" aria-label="Close navigation">
            <X className="h-4 w-4" />
          </button>
        </div>
        <nav className="flex flex-col gap-0.5 p-3">
          {items.map((it, i) => {
            const isActive = active === it.href;
            const Icon = it.icon;
            return (
              <Link
                key={it.href}
                href={it.href}
                onClick={onClose}
                className={cn(
                  'flex items-center gap-3 rounded-md px-3 py-2.5 text-sm transition-[background-color,color,transform,opacity] duration-500 ease-out-expo',
                  isActive ? 'bg-background font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:bg-secondary/60 hover:text-foreground',
                  state === 'open' ? 'translate-x-0 opacity-100' : '-translate-x-4 opacity-0',
                )}
                style={{ transitionDelay: state === 'open' ? `${80 + i * 35}ms` : '0ms' }}
              >
                <Icon className="h-4 w-4" />
                {it.label}
                {isActive ? <span className="ml-auto h-1.5 w-1.5 rounded-full bg-signal" /> : null}
              </Link>
            );
          })}
        </nav>
        <div className="mt-auto border-t border-sidebar-border p-3">
          <ThemeToggle />
        </div>
      </aside>
    </div>
  );
}
