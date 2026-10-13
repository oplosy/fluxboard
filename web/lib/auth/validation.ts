// Client-side mirrors of the backend auth rules (backend argon2x.CheckPolicy and
// authuc.validEmail). They give instant feedback; the server stays the
// authority and re-checks everything (e.g. the breached-password list).

export const PASSWORD_MIN = 12;
export const PASSWORD_MAX = 128;
export const NAME_MAX = 100;

/** Character count the way the backend counts it (Unicode code points). */
export function charCount(s: string): number {
  return Array.from(s).length;
}

export function validateEmail(raw: string): string | undefined {
  const email = raw.trim();
  if (!email) return 'Enter your email address.';
  if (email.length > 254) return 'That email address is too long.';
  // One @, something before it, a dotted domain after it, no spaces/brackets.
  if (!/^[^\s@<>]+@[^\s@<>]+\.[^\s@<>.]+$/.test(email) || email.includes('..')) {
    return 'Enter a valid email address, like name@example.com.';
  }
  return undefined;
}

export function validateName(raw: string): string | undefined {
  const name = raw.trim();
  if (!name) return 'Enter your name.';
  if (charCount(name) > NAME_MAX) return `Name must be at most ${NAME_MAX} characters.`;
  return undefined;
}

export interface PasswordRule {
  id: 'min' | 'max' | 'email';
  label: string;
  ok: boolean;
}

/** The live checklist rules shown under a new-password field. */
export function passwordRules(password: string, email = ''): PasswordRule[] {
  const n = charCount(password);
  const e = email.trim().toLowerCase();
  const local = e.includes('@') ? e.slice(0, e.indexOf('@')) : e;
  const p = password.toLowerCase();
  const matchesEmail = p.length > 0 && ((local !== '' && p === local) || (e !== '' && p === e));
  return [
    { id: 'min', label: `At least ${PASSWORD_MIN} characters`, ok: n >= PASSWORD_MIN },
    { id: 'max', label: `No more than ${PASSWORD_MAX} characters`, ok: n <= PASSWORD_MAX },
    { id: 'email', label: 'Not the same as your email', ok: !matchesEmail },
  ];
}

export function validateNewPassword(password: string, email = ''): string | undefined {
  if (!password) return 'Choose a password.';
  const n = charCount(password);
  if (n < PASSWORD_MIN) {
    const left = PASSWORD_MIN - n;
    return `Password is too short — add ${left} more character${left === 1 ? '' : 's'} (minimum ${PASSWORD_MIN}).`;
  }
  if (n > PASSWORD_MAX) {
    return `Password is too long — remove ${n - PASSWORD_MAX} character${n - PASSWORD_MAX === 1 ? '' : 's'} (maximum ${PASSWORD_MAX}).`;
  }
  if (!passwordRules(password, email).every((r) => r.ok)) {
    return 'Password must not be the same as your email.';
  }
  return undefined;
}

export function validateConfirm(password: string, confirm: string): string | undefined {
  if (!confirm) return 'Re-enter your password.';
  if (password !== confirm) return 'Passwords don’t match.';
  return undefined;
}

/**
 * A rough 0–4 strength score for the meter: length carries most weight, then
 * character variety. Purely advisory — the policy is length + breach list.
 */
export function passwordStrength(password: string): 0 | 1 | 2 | 3 | 4 {
  const n = charCount(password);
  if (n === 0) return 0;
  if (n < PASSWORD_MIN) return 1;
  const classes = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((r) => r.test(password)).length;
  if (n >= 20 || (n >= 16 && classes >= 3)) return 4;
  if (n >= 14 || classes >= 3) return 3;
  return 2;
}
