'use client';

import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useRef, useState } from 'react';
import { X } from 'lucide-react';

import { useProjectByKey } from '@/lib/board/use-project';
import { useTaskIdByNumber } from '@/lib/board/use-task';
import { TaskDetail } from '@/components/task/task-detail';
import { Spinner } from '@/components/ui/spinner';
import { cn } from '@/lib/utils';

const EXIT_MS = 260;

// Intercepted task route (ADR-018): clicking a card opens the task detail in a
// modal over the board instead of navigating away. Refresh/direct load still
// serves the full page at .../tasks/[taskNumber].
export default function InterceptedTaskModal({
  params,
}: {
  params: { projectKey: string; taskNumber: string };
}) {
  const router = useRouter();
  const number = Number(params.taskNumber);
  const { project } = useProjectByKey(params.projectKey);
  const { taskId, isLoading, isError } = useTaskIdByNumber(project?.id, number);
  const [closing, setClosing] = useState(false);
  const closingRef = useRef(false);

  // Play the exit animation, then pop the intercepted route.
  const close = useCallback(() => {
    if (closingRef.current) return;
    closingRef.current = true;
    setClosing(true);
    window.setTimeout(() => router.back(), EXIT_MS);
  }, [router]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') close();
    }
    window.addEventListener('keydown', onKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [close]);

  return (
    <div
      className={cn(
        'fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-3 sm:p-8',
        'bg-ink/55 backdrop-blur-[3px] transition-opacity duration-300',
        closing ? 'opacity-0' : 'animate-fade-in',
      )}
      onClick={close}
      role="dialog"
      aria-modal="true"
      aria-label={Number.isNaN(number) ? 'Task' : `Task #${number}`}
    >
      <div
        className={cn(
          'relative w-full max-w-3xl rounded-lg border border-border bg-background p-6 shadow-[0_40px_80px_-30px_hsl(var(--ink)/0.6)] sm:p-8',
          closing
            ? 'translate-y-4 scale-[0.98] opacity-0 transition-[transform,opacity] duration-[260ms] ease-in-out-quart'
            : 'animate-[modal-in_0.55s_var(--ease-out-expo)_backwards]',
        )}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="absolute left-6 top-0 h-[2px] w-16 bg-signal sm:left-8" aria-hidden />
        <button
          onClick={close}
          aria-label="Close"
          className="group absolute right-4 top-4 flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        >
          <X className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:rotate-90" />
        </button>
        {Number.isNaN(number) || isError || (!isLoading && !taskId) ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            Couldn’t load this task.{' '}
            <button onClick={close} className="text-foreground underline underline-offset-4 hover:decoration-signal">
              Back to board
            </button>
          </p>
        ) : isLoading || !project || !taskId ? (
          <div className="flex items-center justify-center gap-3 py-12 text-sm text-muted-foreground">
            <Spinner /> Loading task…
          </div>
        ) : (
          <TaskDetail taskId={taskId} projectId={project.id} projectKey={project.key} />
        )}
      </div>
    </div>
  );
}
