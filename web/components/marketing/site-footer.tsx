'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { ArrowUp } from 'lucide-react';

import { Magnetic } from '@/components/motion/effects';
import { Reveal } from '@/components/motion/reveal';
import { LogoMark } from '@/components/brand/logo';

const columns: { title: string; links: { href: string; label: string }[] }[] = [
  {
    title: 'Product',
    links: [
      { href: '/features', label: 'Features' },
      { href: '/pricing', label: 'Pricing' },
      { href: '/changelog', label: 'Changelog' },
      { href: '/status', label: 'Status' },
    ],
  },
  {
    title: 'Legal',
    links: [
      { href: '/legal/terms', label: 'Terms' },
      { href: '/legal/privacy', label: 'Privacy' },
      { href: '/legal/dpa', label: 'DPA' },
    ],
  },
  {
    title: 'Account',
    links: [
      { href: '/login', label: 'Sign in' },
      { href: '/register', label: 'Create account' },
    ],
  },
];

function UtcClock() {
  const [now, setNow] = useState<string | null>(null);
  useEffect(() => {
    const tick = () => setNow(new Date().toISOString().slice(11, 19));
    tick();
    const id = window.setInterval(tick, 1000);
    return () => window.clearInterval(id);
  }, []);
  return <span className="tabular">{now ?? '--:--:--'} UTC</span>;
}

export function SiteFooter() {
  return (
    <footer className="relative overflow-hidden border-t border-border">
      <div className="container grid gap-12 py-16 md:grid-cols-[1.4fr_repeat(3,1fr)]">
        <div className="max-w-sm">
          <p className="kicker">Fluxboard / portfolio build</p>
          <p className="mt-4 text-[15px] leading-relaxed text-muted-foreground">
            Kanban boards, isolated team workspaces and metered billing in one product — built to scale from a
            side project to an organisation without switching tools.
          </p>
          <div className="mt-6 flex items-center gap-2 font-mono text-[11px] text-muted-foreground">
            <span className="relative flex h-2 w-2">
              <span className="absolute inset-0 animate-pulse-ring rounded-full bg-success" />
              <span className="relative h-2 w-2 rounded-full bg-success" />
            </span>
            <Link href="/status" className="link-underline hover:text-foreground">
              System status
            </Link>
            · <UtcClock />
          </div>
        </div>
        {columns.map((col, i) => (
          <Reveal key={col.title} index={i}>
            <p className="kicker">{col.title}</p>
            <ul className="mt-4 space-y-2.5">
              {col.links.map((l) => (
                <li key={l.href}>
                  <Link href={l.href} className="link-underline text-sm text-foreground/80 transition-colors hover:text-foreground">
                    {l.label}
                  </Link>
                </li>
              ))}
            </ul>
          </Reveal>
        ))}
      </div>

      {/* Oversized wordmark that wipes in when the footer enters view. */}
      <div className="container relative">
        <Reveal variant="clip-up" className="select-none">
          <p className="flex items-end gap-[2vw] font-display text-[19vw] font-bold leading-[0.78] tracking-[-0.07em] text-foreground md:text-[17vw] 2xl:text-[230px]">
            <LogoMark className="mb-[1.6vw] h-[11vw] w-[11vw] 2xl:h-[150px] 2xl:w-[150px]" />
            flux
          </p>
        </Reveal>
      </div>

      <div className="container flex items-center justify-between gap-4 border-t border-border py-5 font-mono text-[11px] text-muted-foreground">
        <span>© {new Date().getFullYear()} Fluxboard</span>
        <Magnetic>
          <button
            type="button"
            onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
            className="group flex items-center gap-2 uppercase tracking-[0.16em] transition-colors hover:text-foreground"
          >
            Back to top
            <span className="flex h-7 w-7 items-center justify-center rounded-full border border-border transition-[transform,border-color] duration-500 ease-spring group-hover:-translate-y-1 group-hover:border-foreground">
              <ArrowUp className="h-3.5 w-3.5" />
            </span>
          </button>
        </Magnetic>
      </div>
    </footer>
  );
}
