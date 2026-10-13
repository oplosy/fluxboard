import { describe, expect, it } from 'vitest';

import { ApiError } from '@/lib/api/client';
import { apiFieldErrors, authErrorMessage, retryAfterSeconds } from './errors';
import { passwordRules, validateConfirm, validateEmail, validateName, validateNewPassword } from './validation';

describe('validateEmail', () => {
  it.each(['a@b.co', ' Name.Last+tag@sub.example.org '])('accepts %s', (e) => {
    expect(validateEmail(e)).toBeUndefined();
  });
  it.each(['', 'plain', 'a@localhost', 'a@b.', 'a b@c.io', 'a@@b.io', 'a@b..io'])('rejects %j', (e) => {
    expect(validateEmail(e)).toBeTypeOf('string');
  });
});

describe('validateNewPassword', () => {
  it('says how many characters are missing', () => {
    expect(validateNewPassword('short')).toMatch(/add 7 more characters/);
    expect(validateNewPassword('elevenchars')).toMatch(/add 1 more character \(/);
  });
  it('flags passwords that are too long', () => {
    expect(validateNewPassword('a'.repeat(130))).toMatch(/too long — remove 2 characters/);
  });
  it('counts characters, not UTF-16 units or bytes', () => {
    expect(validateNewPassword('şifreğüçöıŞİ')).toBeUndefined(); // 12 chars
    expect(validateNewPassword('🔒🔒🔒🔒🔒🔒🔒🔒🔒🔒🔒')).toMatch(/add 1 more/); // 11 emoji
  });
  it('rejects the email or its local part', () => {
    expect(validateNewPassword('longlocalpart', 'longlocalpart@x.io')).toMatch(/same as your email/);
    expect(validateNewPassword('me@example.com', 'me@example.com')).toMatch(/same as your email/);
    expect(passwordRules('a-good-passphrase', 'me@example.com').every((r) => r.ok)).toBe(true);
  });
});

describe('other field validators', () => {
  it('requires a bounded name', () => {
    expect(validateName('  ')).toBe('Enter your name.');
    expect(validateName('x'.repeat(101))).toMatch(/at most 100/);
    expect(validateName('Ada')).toBeUndefined();
  });
  it('checks the confirmation matches', () => {
    expect(validateConfirm('abc', 'abd')).toMatch(/don’t match/);
    expect(validateConfirm('abc', 'abc')).toBeUndefined();
  });
});

describe('auth error mapping', () => {
  it('maps a coded field error onto its field with friendly copy', () => {
    const err = new ApiError(409, {
      code: 'email_taken',
      message: 'An account with this email already exists.',
      details: { fields: { email: 'server text' } },
    });
    expect(apiFieldErrors(err)).toEqual({ email: 'An account with this email already exists.' });
  });
  it('turns a 429 into a wait time', () => {
    const err = new ApiError(429, { code: 'rate_limited', message: 'x', details: { retry_after: 125 } });
    expect(retryAfterSeconds(err)).toBe(125);
    expect(authErrorMessage(err)).toBe('Too many attempts. Try again in 3 minutes.');
  });
  it('never shows raw 5xx text', () => {
    const err = new ApiError(500, { code: 'internal', message: 'internal server error' });
    expect(authErrorMessage(err)).toMatch(/on our side/);
  });
  it('explains a network failure', () => {
    expect(authErrorMessage(new TypeError('Failed to fetch'))).toMatch(/reach the server/);
  });
});
