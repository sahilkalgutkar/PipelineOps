/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        healthy: "#16a34a",
        late: "#d97706",
        failed: "#dc2626",
        unknown: "#6b7280",
      },
    },
  },
  plugins: [],
};
