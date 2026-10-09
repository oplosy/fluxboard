import { Skeleton } from '@/components/ui/skeleton';

export default function AccountLoading() {
  return (
    <div className="max-w-2xl space-y-4" aria-busy="true" aria-label="Loading account">
      <Skeleton className="h-5 w-40" />
      <Skeleton className="h-11 w-full" />
      <Skeleton className="h-11 w-full" />
      <Skeleton className="h-11 w-2/3" />
    </div>
  );
}
