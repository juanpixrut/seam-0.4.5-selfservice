/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './internal/**/*.go',
    './web/static/js/*.js',
    './web/templates/**/*.html',
  ],
  theme: { extend: {} },
  plugins: [],
}
