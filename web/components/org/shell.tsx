'use client';

import { useCallback, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';

import { useOrg } from '@/lib/org/context';
import { useRouteEnter } from '@/components/motion/route-transition';
import { Sidebar, MobileSidebar } from './sidebar';
import { Topbar } from './topbar';
import { PastDueBanner } from './past-due-banner';
import { SoftDeleteScreen } from './soft-delete-screen';
import { Realtime } from './realtime';

const SIDEBAR_KEY = 'fluxboard-sidebar-collapsed';

const noSubscribe = () => () => {};
function readSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_KEY) === 'true';
  } catch {
    return false; // Storage unavailable (private mode) — keep the default.
  }
}

// Task routes open as an intercepted modal over the board; animating the page
// underneath would make the board jump, so they are excluded.
const NO_ENTER = /\/tasks\/\d+$/;

// Chrome for the org shell. A soft-deleted org takes over the whole viewport with
// the restore screen; otherwise the standard sidebar + topbar frame renders.
export function OrgShell({ children }: { children: ReactNode }) {
  const { org } = useOrg();
  // Stored preference (expanded on the server), until the user toggles it here.
  const stored = useSyncExternalStore(noSubscribe, readSidebarCollapsed, () => false);
  const [toggled, setToggled] = useState<boolean | null>(null);
  const collapsed = toggled ?? stored;
  const [mobileOpen, setMobileOpen] = useState(false);
  const main = useRef<HTMLElement>(null);
  useRouteEnter(main, NO_ENTER);

  const toggleSidebar = useCallback(() => {
    const next = !collapsed;
    setToggled(next);
    try {
      localStorage.setItem(SIDEBAR_KEY, String(next));
    } catch {
      // Non-critical preference.
    }
  }, [collapsed]);

  if (org.deleted_at) return <SoftDeleteScreen />;

  return (
    <div className="flex min-h-screen">
      <Realtime />
      <Sidebar collapsed={collapsed} onToggle={toggleSidebar} />
      <MobileSidebar open={mobileOpen} onClose={() => setMobileOpen(false)} />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar onMobileMenuToggle={() => setMobileOpen((v) => !v)} />
        <PastDueBanner />
        <main ref={main} className="min-w-0 flex-1">
          {children}
        </main>
      </div>
    </div>
  );
}
