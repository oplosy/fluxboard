import type { Metadata, Viewport } from 'next';
import type { ReactNode } from 'react';
import { Bricolage_Grotesque, Inter, JetBrains_Mono } from 'next/font/google';

import './globals.css';
import { Providers } from './providers';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-inter',
  display: 'swap',
});

const display = Bricolage_Grotesque({
  subsets: ['latin'],
  variable: '--font-display',
  display: 'swap',
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ['latin'],
  variable: '--font-jetbrains-mono',
  display: 'swap',
});

export const metadata: Metadata = {
  title: {
    default: 'Fluxboard',
    template: '%s · Fluxboard',
  },
  description: 'Multi-tenant project management with usage-based billing.',
};

export const viewport: Viewport = {
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#f5f3ee' },
    { media: '(prefers-color-scheme: dark)', color: '#111110' },
  ],
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={`${inter.variable} ${display.variable} ${jetbrainsMono.variable}`}
    >
      <head>
        {/* Prevent flash of wrong theme by applying stored preference before render */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('fluxboard-theme');var d=t==='dark'||(t!=='light'&&matchMedia('(prefers-color-scheme:dark)').matches);document.documentElement.classList.toggle('dark',d)}catch(e){}})()`,
          }}
        />
        {/* Scroll-reveal content starts hidden and is revealed by JS; without JS, show it. */}
        <noscript>
          <style>{`[data-reveal],.split-inner{opacity:1!important;transform:none!important;filter:none!important;clip-path:none!important}`}</style>
        </noscript>
      </head>
      <body className="min-h-screen font-sans antialiased">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
