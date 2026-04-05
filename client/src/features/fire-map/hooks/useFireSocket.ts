/**
 * 불 이벤트 구독 훅
 *
 * 소켓 자체의 연결/해제는 전역 SocketProvider 가 담당한다.
 * 이 훅은 리스너만 등록/해제하고, 재접속 시에도 자동으로 초기 동기화를 요청한다.
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
    const requestSync = () => {
      socket.emit('get_fires', {})
    }
    const onFireUpdate = (data: FireCell) => updateFire(data)
    const onFiresSync = (data: FireCell[]) => syncFires(data)
    const onUsersCount = (data: { count: number }) => setOnlineUsers(data.count)

    // 이미 연결되어 있으면 즉시 초기 동기화 요청
    if (socket.connected) requestSync()
    // 이후 재접속할 때마다 자동 재동기화
    socket.on('connect', requestSync)
    socket.on('fire:update', onFireUpdate)
    socket.on('fires:sync', onFiresSync)
    socket.on('users:count', onUsersCount)

    return () => {
      socket.off('connect', requestSync)
      socket.off('fire:update', onFireUpdate)
      socket.off('fires:sync', onFiresSync)
      socket.off('users:count', onUsersCount)
    }
  }, [updateFire, syncFires, setOnlineUsers])
}
