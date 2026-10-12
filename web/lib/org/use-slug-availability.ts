'use client';

import { useEffect, useState } from 'react';

import { isSlugAvailable } from '@/lib/api/orgs';

export type SlugState = 'idle' | 'checking' | 'available' | 'taken' | 'invalid';

type Checked = { slug: string; state: 'available' | 'taken' | 'idle' };

/**
 * Debounced live availability of an org slug. `current` (the org's own slug,
 * '' for a new org) is 'idle' and under 3 characters is 'invalid', both derived
 * without a request. A result only counts for the slug it was fetched for, so a
 * slow response for an earlier keystroke never labels the current one.
 */
export function useSlugAvailability(slug: string, current = ''): SlugState {
  const [checked, setChecked] = useState<Checked | null>(null);
  const skip = slug === current || slug.length < 3;

  useEffect(() => {
    if (skip) return;
    const t = setTimeout(() => {
      isSlugAvailable(slug)
        .then((ok) => setChecked({ slug, state: ok ? 'available' : 'taken' }))
        .catch(() => setChecked({ slug, state: 'idle' }));
    }, 400);
    return () => clearTimeout(t);
  }, [slug, skip]);

  if (slug === current) return 'idle';
  if (slug.length < 3) return 'invalid';
  return checked?.slug === slug ? checked.state : 'checking';
}
