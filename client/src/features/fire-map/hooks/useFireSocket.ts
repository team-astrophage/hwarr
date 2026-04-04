/**
 * 소켓 연결 + 불 이벤트 구독 훅
 *
 * 이벤트:
 * - fire:update — 개별 격자 불 업데이트
 * - fires:sync — 전체 활성 불 목록 동기화
 * - users:count — 실시간 접속자 수
 */

import { useEffect } from 'react'
import { socket } from '../../../lib/socket'
import { useFireStore } from '../stores/fireStore'
import type { FireCell } from '../stores/fireStore'

export function useFireSocket() {
  const updateFire = useFireStore((s) => s.updateFire)
  const syncFires = useFireStore((s) => s.syncFires)
  const setOnlineUsers = useFireStore((s) => s.setOnlineUsers)

  useEffect(() => {
    socket.connect()

    socket.on('connect', () => {
      console.log('[Socket] Connected')
      socket.emit('get_fires', {})
    })

    socket.on('fire:update', (data: FireCell) => {
      console.log('[Socket] fire:update received:', data)
      updateFire(data)
    })

    socket.on('fires:sync', (data: FireCell[]) => {
      console.log('[Socket] fires:sync received:', data.length, 'fires')
      syncFires(data)
    })

    socket.on('users:count', (data: { count: number }) => {
      console.log('[Socket] users:count received:', data.count)
      setOnlineUsers(data.count)
    })

    return () => {
      socket.off('fire:update')
      socket.off('fires:sync')
      socket.off('users:count')
      socket.disconnect()
      console.log('[Socket] Disconnected')
    }
  }, [updateFire, syncFires, setOnlineUsers])
}
