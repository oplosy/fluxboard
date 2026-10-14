import { Check, Circle } from 'lucide-react';

import { cn } from '@/lib/utils';
import { charCount, PASSWORD_MAX, passwordRules, passwordStrength } from '@/lib/auth/validation';

const STRENGTH_LABEL = ['', 'Too short', 'Okay', 'Strong', 'Very strong'] as const;
const STRENGTH_COLOR = ['bg-muted', 'bg-destructive', 'bg-warning', 'bg-success', 'bg-success'] as const;

// Live password requirements under a new-password field: a strength meter plus
// the policy rules ticking off as the user types. Mirrors the server policy;
// the breached-password check only runs server-side.
export function PasswordChecklist({ id, password, email }: { id?: string; password: string; email?: string }) {
  const rules = passwordRules(password, email);
  const strength = passwordStrength(password);
  const n = charCount(password);

  return (
    <div id={id} className="space-y-2" aria-live="polite">
      <div className="flex items-center gap-2">
        <div className="flex flex-1 gap-1" aria-hidden>
          {[1, 2, 3, 4].map((level) => (
            <div
              key={level}
              className={cn(
                'h-1 flex-1 rounded-full transition-colors duration-300',
                level <= strength ? STRENGTH_COLOR[strength] : 'bg-muted',
              )}
            />
          ))}
        </div>
        <span className="min-w-[76px] shrink-0 whitespace-nowrap text-right text-xs text-muted-foreground font-medium">
          {password ? STRENGTH_LABEL[strength] : `${n}/${PASSWORD_MAX}`}
        </span>
      </div>
      <ul className="grid gap-1 text-[12.5px]">
        {rules
          // The max rule only matters once it is at risk.
          .filter((r) => r.id !== 'max' || n > PASSWORD_MAX - 16)
          .map((r) => {
            // Nothing is "met" before the user has typed anything.
            const met = r.ok && password.length > 0;
            return (
              <li
                key={r.id}
                className={cn(
                  'flex items-center gap-1.5 transition-colors',
                  met ? 'text-success' : password ? 'text-foreground/80' : 'text-muted-foreground',
                )}
              >
                {met ? <Check className="h-3.5 w-3.5" aria-hidden /> : <Circle className="h-3 w-3" aria-hidden />}
                <span>
                  {r.label}
                  <span className="sr-only">{met ? ' — met' : ' — not met'}</span>
                </span>
              </li>
            );
          })}
      </ul>
    </div>
  );
}
