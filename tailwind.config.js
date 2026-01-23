/** @type {import('tailwindcss').Config} */

export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{js,ts,vue}"],
  theme: {
    container: {
      center: true,
    },
    extend: {
      fontFamily: {
        sans: ['Nunito', 'sans-serif'],
      },
      colors: {
        primary: '#FF8A65', // Soft Coral
        secondary: '#7CD6C9',
        accent: '#FFD54F', // Soft Yellow
        background: '#FFF8F5', // Warm White/Peach tint
        surface: '#FFFFFF',
      },
      borderRadius: {
        'xl': '1rem',
        '2xl': '1.5rem',
        '3xl': '2rem',
      },
      boxShadow: {
        'soft': '0 4px 20px -2px rgba(0, 0, 0, 0.05)',
        'card': '0 0 20px rgba(0, 0, 0, 0.03)',
      }
    },
  },
  plugins: [],
};
