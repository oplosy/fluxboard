import { Hero } from '@/components/marketing/landing/hero';
import { BoardStory } from '@/components/marketing/landing/board-story';
import {
  BillingSection,
  ClosingCta,
  RealtimeSection,
  SecuritySection,
  Ticker,
} from '@/components/marketing/landing/sections';
import { PricingTable } from '@/components/marketing/pricing-table';
import { Reveal, SplitText } from '@/components/motion/reveal';

export default function LandingPage() {
  return (
    <>
      <Hero />
      <Ticker />
      <BoardStory />
      <RealtimeSection />
      <BillingSection />
      <SecuritySection />

      <section className="border-t border-border py-24 md:py-36">
        <div className="container">
          <div className="mb-14 grid gap-6 lg:grid-cols-[0.9fr_1.1fr] lg:items-end">
            <div>
              <Reveal variant="fade" className="kicker">
                06 — Pricing
              </Reveal>
              <h2 className="mt-4 text-[clamp(2.2rem,4.6vw,4rem)] font-semibold leading-[0.95] tracking-[-0.045em]">
                <SplitText text="Pay for the work you do." />
              </h2>
            </div>
            <Reveal index={2} className="max-w-[44ch] text-[17px] leading-relaxed text-muted-foreground lg:justify-self-end">
              Start free. Upgrades apply immediately with proration; downgrades wait for the end of the period.
            </Reveal>
          </div>
          <PricingTable />
        </div>
      </section>

      <ClosingCta />
    </>
  );
}
