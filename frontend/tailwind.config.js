/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#14181a',
        panel: '#1c2326',
        line: '#314046',
        lime: '#d6ff4a',
        mist: '#e7eef0',
        mute: '#93a4ab',
        ember: '#e23d1a',
        tide: '#1d6fbf',
        brass: '#d97706',
      },
      fontFamily: {
        sans: ['Barlow', 'Segoe UI', 'sans-serif'],
        display: ['Barlow Condensed', 'Barlow', 'sans-serif'],
        mono: ['IBM Plex Mono', 'ui-monospace', 'monospace'],
      },
    },
  },
  plugins: [],
}
