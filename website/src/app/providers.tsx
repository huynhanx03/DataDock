import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createContext, useContext, useState, type ReactNode } from 'react'
import { createDataDockGateway } from '../data/adapter-factory'
import type { DataDockGateway } from '../data/gateway'
import { APP_CONFIG } from '../shared/config/constants'

const DataDockGatewayContext = createContext<DataDockGateway | null>(null)

export function createDataDockQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: APP_CONFIG.cache.staleTimeMs,
        gcTime: APP_CONFIG.cache.gcTimeMs,
        retry: 1,
        refetchOnMount: false,
        refetchOnReconnect: true,
        refetchOnWindowFocus: false,
      },
      mutations: {
        retry: 0,
      },
    },
  })
}

export function DataDockProviders({ children, gateway }: { children: ReactNode; gateway?: DataDockGateway }) {
  const [queryClient] = useState(createDataDockQueryClient)
  const [dataGateway] = useState(() => gateway ?? createDataDockGateway())
  return (
    <DataDockGatewayContext.Provider value={dataGateway}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </DataDockGatewayContext.Provider>
  )
}

export function useDataDockGateway() {
  const gateway = useContext(DataDockGatewayContext)
  if (!gateway) throw new Error('useDataDockGateway must be used within DataDockProviders')
  return gateway
}
