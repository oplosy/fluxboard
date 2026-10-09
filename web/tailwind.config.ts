import type { Config } from 'tailwindcss';

// Design system: "ink & paper" — a monochrome editorial base (primary = ink in
// light mode, paper in dark) with a single vermilion `signal` accent used
// deliberately for state, focus and motion. Radii are tight and tokenised so
// every `rounded-*` utility in the app follows `--radius`.
const config: Config = {
  darkMode: ['class'],
  content: ['./app/**/*.{ts,tsx}', './components/**/*.{ts,tsx}', './lib/**/*.{ts,tsx}'],
  theme: {
    container: {
      center: true,
      padding: { DEFAULT: '1.25rem', sm: '1.5rem', lg: '2rem' },
      screens: { '2xl': '1280px' },
    },
    extend: {
      fontFamily: {
        sans: ['var(--font-inter)', 'Inter', 'system-ui', '-apple-system', 'Segoe UI', 'sans-serif'],
        display: ['var(--font-display)', 'var(--font-inter)', 'system-ui', 'sans-serif'],
        mono: ['var(--font-jetbrains-mono)', 'JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      colors: {
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        signal: {
          DEFAULT: 'hsl(var(--signal))',
          foreground: 'hsl(var(--signal-foreground))',
          ink: 'hsl(var(--signal-ink))',
        },
        cobalt: 'hsl(var(--cobalt))',
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        destructive: {
          DEFAULT: 'hsl(var(--destructive))',
          foreground: 'hsl(var(--destructive-foreground))',
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        success: {
          DEFAULT: 'hsl(var(--success))',
          foreground: 'hsl(var(--success-foreground))',
        },
        warning: {
          DEFAULT: 'hsl(var(--warning))',
          foreground: 'hsl(var(--warning-foreground))',
        },
        popover: {
          DEFAULT: 'hsl(var(--popover))',
          foreground: 'hsl(var(--popover-foreground))',
        },
        sidebar: {
          DEFAULT: 'hsl(var(--sidebar))',
          foreground: 'hsl(var(--sidebar-foreground))',
          border: 'hsl(var(--sidebar-border))',
          active: 'hsl(var(--sidebar-active))',
        },
        topbar: {
          DEFAULT: 'hsl(var(--topbar))',
          foreground: 'hsl(var(--topbar-foreground))',
        },
        ink: 'hsl(var(--ink))',
        paper: 'hsl(var(--paper))',
      },
      borderRadius: {
        sm: 'calc(var(--radius) - 3px)',
        md: 'calc(var(--radius) - 1px)',
        lg: 'var(--radius)',
        xl: 'calc(var(--radius) + 2px)',
        '2xl': 'calc(var(--radius) + 4px)',
        '3xl': 'calc(var(--radius) + 8px)',
      },
      transitionTimingFunction: {
        'out-expo': 'cubic-bezier(0.16, 1, 0.3, 1)',
        spring: 'cubic-bezier(0.34, 1.56, 0.64, 1)',
        'in-out-quart': 'cubic-bezier(0.76, 0, 0.24, 1)',
      },
      transitionDuration: {
        250: '250ms',
        400: '400ms',
        600: '600ms',
        800: '800ms',
      },
      animation: {
        'fade-in': 'fade-in 0.4s cubic-bezier(0.16, 1, 0.3, 1) backwards',
        'slide-up': 'slide-up 0.55s cubic-bezier(0.16, 1, 0.3, 1) backwards',
        'slide-down': 'slide-down 0.3s cubic-bezier(0.16, 1, 0.3, 1) backwards',
        'slide-in-left': 'slide-in-left 0.45s cubic-bezier(0.16, 1, 0.3, 1) backwards',
        'slide-in-right': 'slide-in-right 0.45s cubic-bezier(0.16, 1, 0.3, 1) backwards',
        'scale-in': 'scale-in 0.28s cubic-bezier(0.34, 1.56, 0.64, 1) backwards',
        shimmer: 'shimmer 1.8s ease-in-out infinite',
        'spin-slow': 'spin-slow 1.2s linear infinite',
        marquee: 'marquee var(--marquee-duration, 40s) linear infinite',
        'bar-bounce': 'bar-bounce 1s cubic-bezier(0.76, 0, 0.24, 1) infinite',
        blink: 'blink 1.1s steps(2, start) infinite',
        'pulse-ring': 'pulse-ring 1.8s cubic-bezier(0.16, 1, 0.3, 1) infinite',
      },
      keyframes: {
        'fade-in': { from: { opacity: '0' }, to: { opacity: '1' } },
        'slide-up': {
          from: { opacity: '0', transform: 'translateY(14px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        'slide-down': {
          from: { opacity: '0', transform: 'translateY(-8px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        'slide-in-left': {
          from: { opacity: '0', transform: 'translateX(-16px)' },
          to: { opacity: '1', transform: 'translateX(0)' },
        },
        'slide-in-right': {
          from: { opacity: '0', transform: 'translateX(24px)' },
          to: { opacity: '1', transform: 'translateX(0)' },
        },
        'scale-in': {
          from: { opacity: '0', transform: 'scale(0.94)' },
          to: { opacity: '1', transform: 'scale(1)' },
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
        'spin-slow': { from: { transform: 'rotate(0deg)' }, to: { transform: 'rotate(360deg)' } },
        marquee: { from: { transform: 'translateX(0)' }, to: { transform: 'translateX(-50%)' } },
        'bar-bounce': {
          '0%, 100%': { transform: 'scaleY(0.35)' },
          '50%': { transform: 'scaleY(1)' },
        },
        blink: { to: { visibility: 'hidden' } },
        'pulse-ring': {
          '0%': { transform: 'scale(0.6)', opacity: '0.7' },
          '100%': { transform: 'scale(2.2)', opacity: '0' },
        },
      },
    },
  },
  plugins: [],
};

export default config;
