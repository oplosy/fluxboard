// The operator facts the legal pages state (report B09), in one place. Values
// come from build-time env (NEXT_PUBLIC_LEGAL_*); a release build refuses to
// run without them (next.config.mjs), so the public site never ships with
// placeholder contacts. Dev and CI builds render visible "[not configured]"
// markers instead of invented details.

function env(value: string | undefined, name: string): string {
  const v = value?.trim();
  return v ? v : `[${name} not configured]`;
}

// Read with literal process.env.X so Next inlines them at build time.
export const operator = {
  /** Registered legal name of the operating company (data controller). */
  entity: env(process.env.NEXT_PUBLIC_LEGAL_ENTITY, 'NEXT_PUBLIC_LEGAL_ENTITY'),
  /** Registered address. */
  address: env(process.env.NEXT_PUBLIC_LEGAL_ADDRESS, 'NEXT_PUBLIC_LEGAL_ADDRESS'),
  /** Privacy / data-protection contact (KVKK art. 11 and GDPR requests). */
  privacyEmail: env(process.env.NEXT_PUBLIC_LEGAL_PRIVACY_EMAIL, 'NEXT_PUBLIC_LEGAL_PRIVACY_EMAIL'),
  /** Security reports. */
  securityEmail: env(
    process.env.NEXT_PUBLIC_LEGAL_SECURITY_EMAIL || process.env.NEXT_PUBLIC_LEGAL_PRIVACY_EMAIL,
    'NEXT_PUBLIC_LEGAL_SECURITY_EMAIL',
  ),
  /** Governing law of the terms, e.g. "the Republic of Türkiye". */
  governingLaw: env(process.env.NEXT_PUBLIC_LEGAL_GOVERNING_LAW, 'NEXT_PUBLIC_LEGAL_GOVERNING_LAW'),
};

/** Where service data lives (ADR-031): AWS eu-central-1. */
export const hosting = {
  provider: 'Amazon Web Services (AWS)',
  region: 'eu-central-1 (Frankfurt, Germany)',
};

export interface Subprocessor {
  name: string;
  purpose: string;
  location: string;
}

/**
 * Every third party that processes personal data for the service (ADR-031).
 * Adding one means updating this list (and announcing it per the DPA) before
 * it receives data.
 */
export const subprocessors: Subprocessor[] = [
  {
    name: hosting.provider,
    purpose:
      'Hosting, database, cache, file storage, backups, and transactional email (Amazon SES)',
    location: hosting.region,
  },
  {
    name: 'Stripe',
    purpose:
      'Payment processing: checkout, subscriptions, invoices, and the billing portal. Card data goes to Stripe directly and never reaches our servers.',
    location: 'Stripe’s infrastructure (European Union and United States)',
  },
];

/**
 * Retention of deleted data in backups (deploy/aws, ADR-028): point-in-time
 * recovery 14 days, overwritten/deleted file versions 30 days, daily vault
 * copies 35 days. So deleted data leaves every backup within 35 days.
 */
export const backupRetentionDays = 35;
