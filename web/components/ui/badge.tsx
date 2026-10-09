import { cn } from '@/lib/utils';

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'success' | 'warning' | 'outline';

const variants: Record<BadgeVariant, string> = {
  default: 'bg-foreground text-background',
  secondary: 'bg-secondary text-secondary-foreground',
  destructive: 'bg-destructive/12 text-destructive',
  success: 'bg-success/12 text-success',
  warning: 'bg-warning/20 text-foreground',
  outline: 'border border-border text-foreground',
};

interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
}

/** Small label badge for status, tags, and metadata. */
export function Badge({ variant = 'default', className, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-sm px-1.5 py-0.5 text-[11px] font-medium leading-tight tracking-[-0.005em]',
        variants[variant],
        className,
      )}
      {...props}
    />
  );
}
