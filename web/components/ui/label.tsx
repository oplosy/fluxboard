import { forwardRef, type LabelHTMLAttributes } from 'react';

import { cn } from '@/lib/utils';

export const Label = forwardRef<HTMLLabelElement, LabelHTMLAttributes<HTMLLabelElement>>(
  ({ className, ...props }, ref) => (
    <label
      ref={ref}
      className={cn('text-[13px] font-medium leading-none tracking-[-0.005em]', className)}
      {...props}
    />
  ),
);
Label.displayName = 'Label';
