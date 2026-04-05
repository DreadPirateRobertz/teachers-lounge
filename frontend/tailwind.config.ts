import type { Config } from 'tailwindcss'

const config: Config = {
  content: ['./app/**/*.{ts,tsx}', './components/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        'neon-blue': '#00aaff',
        'neon-green': '#00ff88',
        'neon-pink': '#ff00aa',
        'neon-gold': '#ffdc00',
        'bg-deep': '#0a0a1a',
        'bg-panel': '#0f0f2a',
        'bg-card': '#141430',
        'bg-input': '#0d0d24',
        'border-dim': '#1a1a4a',
        'border-mid': '#252560',
        'text-dim': '#6666aa',
        'text-base': '#c8c8e8',
        'text-bright': '#e8e8ff',
      },
      fontFamily: {
        sans: ['var(--font-geist-sans)', 'system-ui', 'sans-serif'],
        mono: ['var(--font-geist-mono)', 'monospace'],
      },
      boxShadow: {
        'neon-blue': '0 0 8px #00aaff66, 0 0 20px #00aaff33',
        'neon-green': '0 0 8px #00ff8866, 0 0 20px #00ff8833',
        'neon-pink': '0 0 8px #ff00aa66, 0 0 20px #ff00aa33',
        'neon-gold': '0 0 8px #ffdc0066, 0 0 20px #ffdc0033',
        'neon-blue-sm': '0 0 4px #00aaff99',
        'neon-green-sm': '0 0 4px #00ff8899',
        'neon-pink-sm': '0 0 4px #ff00aa99',
        'glow-inner': 'inset 0 0 20px #00aaff11',
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'bounce-slow': 'bounce 2s infinite',
        'glow-pulse': 'glow-pulse 2s ease-in-out infinite',
        'fade-in': 'fade-in 0.2s ease-out',
        'slide-up': 'slide-up 0.3s ease-out',
        confetti: 'confetti 1.2s ease-out forwards',
      },
      keyframes: {
        confetti: {
          '0%': { opacity: '0', transform: 'translate(0, 0) rotate(0deg) scale(0)' },
          '40%': {
            opacity: '1',
            transform:
              'translate(var(--tw-translate-x), var(--tw-translate-y)) rotate(180deg) scale(1.2)',
          },
          '100%': {
            opacity: '0',
            transform:
              'translate(var(--tw-translate-x), calc(var(--tw-translate-y) + 40px)) rotate(360deg) scale(0.8)',
          },
        },
        'glow-pulse': {
          '0%, 100%': { opacity: '0.8' },
          '50%': { opacity: '1', filter: 'brightness(1.2)' },
        },
        'fade-in': {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        'slide-up': {
          from: { opacity: '0', transform: 'translateY(8px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
      },
    },
  },
  plugins: [],
}

export default config
