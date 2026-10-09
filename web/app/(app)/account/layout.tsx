'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useRef, type ReactNode } from 'react';
import { ArrowLeft, LogOut } from 'lucide-react';

import { SectionShell } from '@/components/org/section-shell';
import { Logo } from '@/components/brand/logo';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { useRouteEnter } from '@/components/motion/route-transition';

const nav = [
  { href: '/account/profile', label: 'Profile' },
  { href: '/account/security', label: 'Security' },
  { href: '/account/sessions', label: 'Sessions' },
  { href: '/account/notifications', label: 'Notifications' },
  { href: '/account/danger', label: 'Danger zone' },
];

export default function AccountLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const body = useRef<HTMLDivElement>(null);
  useRouteEnter(body);
  const currentLabel = nav.find((n) => pathname === n.href)?.label ?? 'Account';

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-20 flex h-14 items-center justify-between border-b border-border bg-background/85 px-5 backdrop-blur-md lg:px-10">
        <div className="flex items-center gap-5">
          <Logo href="/app" />
          <Link
            href="/app"
            className="group hidden items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground sm:flex"
          >
            <ArrowLeft className="h-3.5 w-3.5 transition-transform duration-500 ease-spring group-hover:-translate-x-1" />
            Organizations
          </Link>
        </div>
        <div className="flex items-center gap-1">
          <ThemeToggle compact />
          <Link
            href="/logout"
            className="flex h-9 items-center gap-2 rounded-md px-3 text-sm text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            <LogOut className="h-3.5 w-3.5" /> Sign out
          </Link>
        </div>
      </header>
      <div ref={body}>
        <SectionShell
          kicker="Account"
          title={currentLabel}
          tabs={nav.map((n) => ({ href: n.href, label: n.label, active: pathname === n.href }))}
        >
          {children}
        </SectionShell>
      </div>
    </div>
  );
}
