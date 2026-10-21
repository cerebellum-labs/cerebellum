import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'

import App from './App.tsx'
import { AdminProvider } from './app/admin'
import { LiveProvider } from './app/live'
import './index.css'

// Live events keep the data fresh, so refetching on window focus only adds noise.
const queryClient = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } },
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
