'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { ArrowUpRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { Logo } from '@/components/brand/logo';
import { usePresence } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';

const nav = [
  { href: '/features', label: 'Features', n: '01' },
  { href: '/pricing', label: 'Pricing', n: '02' },
  { href: '/changelog', label: 'Changelog', n: '03' },
  { href: '/status', label: 'Status', n: '04' },
];

export function SiteHeader() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const [hidden, setHidden] = useState(false);
  const { mounted, state } = usePresence(mobileOpen, 450);

  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  // Condense after the first scroll; slide away while scrolling down, return on scroll up.
  useEffect(() => {
    let lastY = window.scrollY;
    const handler = () => {
      const y = window.scrollY;
      setScrolled(y > 12);
      setHidden(y > 240 && y > lastY + 2);
      if (y < lastY - 2 || y < 240) setHidden(false);
      lastY = y;
    };
    handler();
    window.addEventListener('scroll', handler, { passive: true });
    return () => window.removeEventListener('scroll', handler);
  }, []);

  useEffect(() => {
    document.body.style.overflow = mobileOpen ? 'hidden' : '';
    return () => {
      document.body.style.overflow = '';
    };
  }, [mobileOpen]);

  return (
    <>
      <header
        className={cn(
          'fixed inset-x-0 top-0 z-50 transition-[transform,background-color,border-color] duration-500 ease-out-expo',
          scrolled ? 'border-b border-border bg-background/80 backdrop-blur-md' : 'border-b border-transparent',
          hidden && !mobileOpen && '-translate-y-full',
        )}
      >
        <div className={cn('container flex items-center justify-between transition-[height] duration-500 ease-out-expo', scrolled ? 'h-14' : 'h-[72px]')}>
          <Logo />
          <HoverNav pathname={pathname} />
          <div className="flex items-center gap-1.5">
            <ThemeToggle compact />
            <Link href="/login" className="hidden px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground sm:block">
              <span className="link-underline">Sign in</span>
            </Link>
            <Link href="/register" className="hidden sm:block" tabIndex={-1}>
              <Button size="sm" className="gap-1.5 pl-3.5 pr-3">
                Get started <ArrowUpRight className="h-3.5 w-3.5 transition-transform duration-300 group-hover:-translate-y-0.5 group-hover:translate-x-0.5" />
              </Button>
            </Link>
            <button
              type="button"
              onClick={() => setMobileOpen((v) => !v)}
              className="relative flex h-9 w-9 items-center justify-center md:hidden"
              aria-label={mobileOpen ? 'Close menu' : 'Open menu'}
              aria-expanded={mobileOpen}
            >
              <span className={cn('absolute h-[1.5px] w-5 bg-foreground transition-transform duration-500 ease-spring', mobileOpen ? 'rotate-45' : '-translate-y-[4px]')} />
              <span className={cn('absolute h-[1.5px] w-5 bg-foreground transition-transform duration-500 ease-spring', mobileOpen ? '-rotate-45' : 'translate-y-[4px]')} />
            </button>
          </div>
        </div>
      </header>

      {/* Mobile menu: full-screen sheet; links rise in one after another. */}
      {mounted ? (
        <div
          data-state={state}
          className={cn(
            'fixed inset-0 z-40 flex flex-col bg-background px-5 pb-8 pt-24 md:hidden',
            'transition-[clip-path] duration-500 ease-in-out-quart',
            'data-[state=closed]:[clip-path:inset(0_0_100%_0)] data-[state=open]:[clip-path:inset(0_0_0_0)]',
          )}
        >
          <nav className="flex flex-col">
            {nav.map((n, i) => (
              <Link
                key={n.href}
                href={n.href}
                className={cn(
                  'flex items-baseline gap-4 border-b border-border py-4 font-display text-4xl font-semibold tracking-[-0.04em]',
                  'transition-[transform,opacity] duration-700 ease-out-expo',
                  state === 'open' ? 'translate-y-0 opacity-100' : 'translate-y-6 opacity-0',
                )}
                style={{ transitionDelay: state === 'open' ? `${120 + i * 60}ms` : '0ms' }}
              >
                <span className="font-mono text-xs font-normal text-muted-foreground">{n.n}</span>
                {n.label}
              </Link>
            ))}
          </nav>
          <div className="mt-auto grid grid-cols-2 gap-2">
            <Link href="/login">
              <Button variant="outline" className="w-full">
                Sign in
              </Button>
            </Link>
            <Link href="/register">
              <Button className="w-full">Get started</Button>
            </Link>
          </div>
        </div>
      ) : null}
    </>
  );
}

/** Desktop nav with a highlight pill that glides to whichever link is hovered. */
function HoverNav({ pathname }: { pathname: string }) {
  const wrap = useRef<HTMLDivElement>(null);
  const [pill, setPill] = useState<{ x: number; w: number; on: boolean }>({ x: 0, w: 0, on: false });

  const moveTo = (el: HTMLElement) => {
    const parent = wrap.current;
    if (!parent) return;
    const a = el.getBoundingClientRect();
    const b = parent.getBoundingClientRect();
    setPill({ x: a.left - b.left, w: a.width, on: true });
  };

  return (
    <nav ref={wrap} className="relative hidden items-center md:flex" onMouseLeave={() => setPill((p) => ({ ...p, on: false }))}>
      <span
        aria-hidden
        className={cn(
          'absolute top-1/2 h-8 -translate-y-1/2 rounded-md bg-secondary transition-[transform,width,opacity] duration-400 ease-out-expo',
          pill.on ? 'opacity-100' : 'opacity-0',
        )}
        style={{ width: pill.w, transform: `translate(${pill.x}px, -50%)`, left: 0 }}
      />
      {nav.map((n) => {
        const active = pathname === n.href;
        return (
          <Link
            key={n.href}
            href={n.href}
            onMouseEnter={(e) => moveTo(e.currentTarget)}
            onFocus={(e) => moveTo(e.currentTarget)}
            className={cn(
              'relative flex items-center gap-1.5 px-3 py-2 text-sm transition-colors',
              active ? 'text-foreground' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            <span className="font-mono text-[10px] text-muted-foreground/70">{n.n}</span>
            {n.label}
            {active ? <span className="absolute -bottom-0.5 left-3 right-3 h-px bg-signal" /> : null}
          </Link>
        );
      })}
    </nav>
  );
}
