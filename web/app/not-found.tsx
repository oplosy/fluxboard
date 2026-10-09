import Link from 'next/link';
import { ArrowLeft } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Logo } from '@/components/brand/logo';
import { ScatterText } from '@/components/canvas/scatter-text';

export default function NotFound() {
  return (
    <div className="blueprint relative flex min-h-screen flex-col">
      <div className="flex items-center justify-between p-5 sm:p-8">
        <Logo />
        <span className="font-mono text-[11px] uppercase tracking-[0.2em] text-muted-foreground">Error 404</span>
      </div>

      <div className="relative mx-auto h-[38vh] min-h-[220px] w-full max-w-5xl px-4">
        <ScatterText text="404" />
        <p className="pointer-events-none absolute bottom-2 left-1/2 -translate-x-1/2 font-mono text-[10px] uppercase tracking-[0.2em] text-muted-foreground">
          Push the pixels · click to scatter
        </p>
      </div>

      <div className="mx-auto flex max-w-md flex-col items-center px-6 pb-20 text-center">
        <h1 className="animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] text-3xl font-semibold tracking-[-0.04em]">
          This card isn&apos;t on any board.
        </h1>
        <p className="mt-3 animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] text-sm text-muted-foreground [animation-delay:100ms]">
          The page you&apos;re looking for doesn&apos;t exist or has moved. Check the URL or head back.
        </p>
        <div className="mt-8 flex animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] gap-3 [animation-delay:200ms]">
          <Link href="/" tabIndex={-1}>
            <Button className="gap-2">
              <ArrowLeft className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:-translate-x-1" /> Back home
            </Button>
          </Link>
          <Link href="/app" tabIndex={-1}>
            <Button variant="outline">Go to app</Button>
          </Link>
        </div>
      </div>
    </div>
  );
}
