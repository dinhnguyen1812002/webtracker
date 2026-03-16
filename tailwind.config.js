/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./interface/http/templates/**/*.{templ,html,go}",
    "./interface/http/**/*.{templ,html,go}",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
