import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'

import { ApiError } from './api/errors'
import App from './App.tsx'
import { AdminProvider } from './app/admin'
import { LiveProvider } from './app/live'
import './index.css'

// Live events keep the data fresh, so refetching on window focus only adds noise.
// A request the server refused (4xx) fails the same way again, so only network
// errors and 5xx are retried: the page shows what went wrong without waiting.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: (failures, error) => !(error instanceof ApiError && error.status >= 400 && error.status < 500) && failures < 2,
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AdminProvider>
          <LiveProvider>
            <App />
          </LiveProvider>
        </AdminProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
