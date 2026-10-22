import type { ReactNode } from 'react';

import type { Project } from '@/lib/api/types';
import { ProjectHeader } from '@/components/board/project-header';
import { cn } from '@/lib/utils';

// The one outer frame every project tab renders inside: identical padding and
// header position, so switching tabs never shifts the header or tab bar. Tabs
// that want a narrower reading column constrain their own content via `narrow`.
export function ProjectFrame({
  project,
  narrow,
  children,
}: {
  project: Project;
  narrow?: boolean;
  children: ReactNode;
}) {
  return (
    <div className="px-4 py-5 sm:px-6">
      <ProjectHeader project={project} />
      <div className={cn(narrow && 'max-w-4xl')}>{children}</div>
    </div>
  );
}
