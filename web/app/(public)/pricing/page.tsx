import type { Metadata } from 'next';

import { PricingTable } from '@/components/marketing/pricing-table';
import { PageIntro } from '@/components/marketing/page-intro';
import { Faq } from '@/components/marketing/faq';
import { Reveal } from '@/components/motion/reveal';

export const metadata: Metadata = { title: 'Pricing' };

const faqs = [
  {
    q: 'How does metered billing work?',
    a: 'Your base plan includes seats, storage, and API-call allowances. Usage beyond the included amount is metered and reported to Stripe, so your invoice reflects what you actually consumed each cycle.',
  },
  {
    q: 'What counts as a seat?',
    a: 'Any active member of an organization. Guests with read-only access to a single project do not consume a seat.',
  },
  {
    q: 'Can I change plans anytime?',
    a: 'Yes. Upgrades apply immediately with prorated charges; downgrades take effect at the end of the current billing period.',
  },
  {
    q: 'What happens if I hit a plan limit?',
    a: 'Writes that would exceed a hard limit return a clear upgrade prompt rather than failing silently. Reads are never blocked.',
  },
];

const flow = [
  { n: '01', t: 'Allowance', b: 'Every plan bundles seats, storage and API calls.' },
  { n: '02', t: 'Metering', b: 'Usage is measured continuously and reported to Stripe.' },
  { n: '03', t: 'Invoice', b: 'The flat plan fee plus any overage, itemised line by line.' },
  { n: '04', t: 'Dashboard', b: 'Live meters versus limits and an estimated invoice.' },
];

export default function PricingPage() {
  return (
    <>
      <PageIntro
        kicker="Pricing"
        title="Plans that grow with the team."
        lede="Transparent plans with usage-based metering. You only pay for the seats, storage and API calls beyond your plan's included allowance."
      />

      <section className="container">
        <PricingTable />
      </section>

      <section className="container py-24 md:py-32">
        <Reveal variant="fade" className="kicker">
          How metered billing works
        </Reveal>
        <ol className="mt-8 grid gap-px overflow-hidden rounded-lg border border-border bg-border md:grid-cols-4">
          {flow.map((f, i) => (
            <Reveal as="li" key={f.n} index={i} className="group relative bg-background p-6">
              <span className="font-mono text-xs text-muted-foreground">{f.n}</span>
              <p className="mt-8 font-display text-xl font-semibold tracking-[-0.02em]">{f.t}</p>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{f.b}</p>
              {i < flow.length - 1 ? (
                <span
                  aria-hidden
                  className="absolute right-4 top-6 hidden font-mono text-muted-foreground transition-transform duration-500 ease-spring group-hover:translate-x-1 group-hover:text-signal md:block"
                >
                  →
                </span>
              ) : null}
            </Reveal>
          ))}
        </ol>
      </section>

      <section className="container grid gap-10 pb-28 md:grid-cols-[0.8fr_1.2fr]">
        <div>
          <Reveal variant="fade" className="kicker">
            FAQ
          </Reveal>
          <h2 className="mt-4 text-4xl font-semibold tracking-[-0.04em] md:text-5xl">Questions, answered.</h2>
        </div>
        <Faq items={faqs} />
      </section>
    </>
  );
}
