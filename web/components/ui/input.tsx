import { forwardRef, type InputHTMLAttributes } from 'react';

import { cn } from '@/lib/utils';

export type InputProps = InputHTMLAttributes<HTMLInputElement>;

// Hairline field; on focus the border inks in and a signal underline draws
// across the bottom edge (box-shadow inset, so layout never shifts).
export const Input = forwardRef<HTMLInputElement, InputProps>(({ className, type, ...props }, ref) => (
  <input
    ref={ref}
    type={type}
    className={cn(
      'flex h-11 w-full rounded-md border border-input bg-card px-3.5 py-2 text-sm text-foreground',
      'transition-[border-color,box-shadow,background-color] duration-300 ease-out-expo',
      'placeholder:text-muted-foreground/80',
      'hover:border-foreground/40',
      'focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none',
      'aria-[invalid=true]:border-destructive',
      'disabled:cursor-not-allowed disabled:opacity-50',
      'file:border-0 file:bg-transparent file:text-sm file:font-medium',
      className,
    )}
    {...props}
  />
));
Input.displayName = 'Input';
