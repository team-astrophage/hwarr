/**
 * 앱 전역에서 1회만 소켓을 연결하고 유지하는 Provider.
 * 지도 페이지 진입 여부와 무관하게 앱이 살아있는 동안 연결을 보장한다.
 */

import { useEffect, type ReactNode } from 'react'
import { startSocket } from '../lib/socketManager'

export function SocketProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    startSocket()
    // 전역 소켓은 앱 수명 동안 유지하므로 cleanup 에서 disconnect 하지 않음.
    // startSocket 내부의 `started` 가드가 StrictMode 더블마운트를 흡수한다.
  }, [])
  return <>{children}</>
}
