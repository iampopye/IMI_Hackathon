import type { Config } from 'tailwindcss'

const config: Config = {
  content: ['./app/**/*.{ts,tsx}', './components/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        warm: {
          white:  '#FAFAF8',
          beige:  '#F5F0E8',
          border: '#E8E0D0',
          strong: '#D4C9B0',
        },
        ink: {
          DEFAULT:   '#1A1A1A',
          secondary: '#6B6560',
          muted:     '#9B9088',
        },
        orange: {
          DEFAULT: '#E85D04',
          hover:   '#C94E00',
          light:   '#FFF0E6',
          border:  '#FDDCC4',
        },
        forest:  { DEFAULT: '#2D6A4F', light: '#F0F7F4' },
        amber:   { DEFAULT: '#B45309', light: '#FEF9EE' },
        danger:  { DEFAULT: '#C0392B', light: '#FEF2F2' },
      },
      fontFamily: {
        display: ['Playfair Display', 'Georgia', 'serif'],
        sans:    ['DM Sans', 'system-ui', 'sans-serif'],
        mono:    ['JetBrains Mono', 'Menlo', 'monospace'],
      },
      borderRadius: {
        card:  '12px',
        input: '10px',
        pill:  '999px',
      },
      boxShadow: {
        card:  '0 1px 3px rgba(0,0,0,0.06)',
        focus: '0 0 0 3px #FFF0E6',
      },
    },
  },
  plugins: [],
}

export default config
