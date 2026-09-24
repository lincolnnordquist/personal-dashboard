import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // In development, forward API calls to a backend running locally (`go run .`), matching
  // what nginx does in production.
  server: {
    proxy: {
      '/graphql': 'http://localhost:8080',
      '/playground': 'http://localhost:8080',
    },
  },
})
