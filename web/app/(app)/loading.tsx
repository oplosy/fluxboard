import { Spinner } from '@/components/ui/spinner';

export default function AppLoading() {
  return (
    <div className="flex min-h-[50vh] animate-fade-in flex-col items-center justify-center gap-4 text-muted-foreground">
      <Spinner size="lg" className="text-foreground" />
      <span className="text-xs font-medium">Loading</span>
    </div>
  );
}
