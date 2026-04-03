import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './app/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './lib/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        'xmr-orange': '#f97316',
        'xmr-orange-dark': '#ea580c',
        'xmr-orange-light': '#fb923c',
        /* Rubik's cube palette */
        'cube-orange': '#f97316',
        'cube-blue': '#3b82f6',
        'cube-green': '#22c55e',
        'cube-red': '#ef4444',
        'cube-yellow': '#eab308',
        'cube-white': '#a1a1aa',
      },
      fontFamily: {
        mono: [
          'ui-monospace',
          'SFMono-Regular',
          'Menlo',
          'Monaco',
          'Consolas',
          'Liberation Mono',
          'Courier New',
          'monospace',
        ],
      },
      animation: {
        'card-enter': 'card-enter 0.3s ease-out both',
        'cube-rotate': 'cube-rotate 10s linear infinite',
      },
    },
  },
  plugins: [],
}
export default config
