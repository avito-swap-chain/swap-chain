import { QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { queryClient } from '@/shared/config/queryClient'
import { BackendEvents } from './BackendEvents'

export function Providers({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <BackendEvents />
      {children}
    </QueryClientProvider>
  )
}
