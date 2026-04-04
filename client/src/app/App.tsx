import { RouterProvider } from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { router } from './router'
import { SocketProvider } from './SocketProvider'

const queryClient = new QueryClient()

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <SocketProvider>
        <RouterProvider router={router} />
      </SocketProvider>
    </QueryClientProvider>
  )
}
