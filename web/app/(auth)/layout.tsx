import Link from 'next/link';
import type { ReactNode } from 'react';
import { ArrowLeft } from 'lucide-react';

import { Logo, LogoMark } from '@/components/brand/logo';
import { DotMatrix } from '@/components/canvas/dot-matrix';
import { ThemeToggle } from '@/components/ui/theme-toggle';

// Split-screen auth layout: an ink panel with the LED dot-matrix canvas on the
// left, the form on the right. On mobile only the form shows.
export default function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen">
      <aside className="relative hidden w-[46%] max-w-[760px] flex-col overflow-hidden bg-ink text-paper lg:flex">
        <div className="absolute inset-0">
          <DotMatrix words={['PLAN', 'MOVE', 'SHIP']} />
        </div>
        <div className="relative flex items-center justify-between p-8">
          <Link href="/" className="group/logo flex items-center gap-2.5">
            <LogoMark className="h-6 w-6" />
            <span className="font-display text-[17px] font-semibold tracking-[-0.03em]">fluxboard</span>
          </Link>
          <span className="font-mono text-[10px] uppercase tracking-[0.2em] text-paper/40">v1 · workspace</span>
        </div>
        <div className="relative mt-auto p-8">
          <p className="max-w-[22ch] animate-slide-up font-display text-4xl font-semibold leading-[1.02] tracking-[-0.045em]">
            Keep the important work in motion.
          </p>
          <ul className="mt-8 grid grid-cols-3 gap-px overflow-hidden rounded-md border border-paper/10 bg-paper/10 font-mono text-[10px] uppercase tracking-[0.14em] text-paper/60">
            {['Realtime boards', 'Tenant isolation', 'Metered billing'].map((f, i) => (
              <li key={f} className="animate-slide-up bg-ink p-3" style={{ animationDelay: `${200 + i * 90}ms` }}>
                <span className="block text-paper/30">0{i + 1}</span>
                {f}
              </li>
            ))}
          </ul>
        </div>
      </aside>

      <div className="relative flex flex-1 flex-col bg-background">
        <div className="flex items-center justify-between p-5 sm:p-8">
          <div className="lg:hidden">
            <Logo />
          </div>
          <Link
            href="/"
            className="group hidden items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground lg:flex"
          >
            <ArrowLeft className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:-translate-x-1" />
            Back to site
          </Link>
          <ThemeToggle compact />
        </div>
        <div className="flex flex-1 items-center justify-center px-5 pb-16 sm:px-8">
          <div className="auth-form w-full max-w-[400px] animate-[slide-up_0.8s_var(--ease-out-expo)_backwards]">{children}</div>
        </div>
      </div>
    </div>
  );
}
