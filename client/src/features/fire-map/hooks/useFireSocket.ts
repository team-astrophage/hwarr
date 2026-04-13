/**
 * 불 이벤트 구독 훅
 *
 * 소켓 자체의 연결/해제는 전역 SocketProvider 가 담당한다.
 * 이 훅은 리스너만 등록/해제한다. 초기 viewport 동기화는 useViewportSubscription 이
 * subscribe:viewport 를 통해 수행한다.
 *
 * 이벤트:
 * - fire:update  — 현재 viewport 구독자에게 전달되는 격자 업데이트 (room-scoped)
 * - fire:ignite  — 새 발화 알림 (room-scoped)
 * - fire:spread  — 인접 격자로의 확산 궤적
 * - users:count  — 실시간 접속자 수
 */

import { useEffect } from 'react'
import { socket } from '../../../lib/socket'
import { useAnimationStore } from '../stores/animationStore'
import { useFireStore } from '../stores/fireStore'
import type { FireCell } from '../stores/fireStore'

interface FireSpreadSegment {
  from: string
  to: string
  fromLat: number
  fromLng: number
  toLat: number
  toLng: number
}

interface FireSpreadPayload {
  path: FireSpreadSegment[]
  eventId?: string
  timestamp?: number
}

export function useFireSocket() {
  const updateFire = useFireStore((s) => s.updateFire)
  const setOnlineUsers = useFireStore((s) => s.setOnlineUsers)

  useEffect(() => {
    const onFireUpdate = (data: FireCell) => updateFire(data)
    const onFireIgnite = (data: FireCell) => updateFire(data)
    const onUsersCount = (data: { count: number }) => setOnlineUsers(data.count)
    const onFireSpread = (data: FireSpreadPayload) => {
      const addTrajectory = useAnimationStore.getState().addTrajectory
      for (const segment of data.path) {
        addTrajectory(
          segment.from,
          segment.to,
          segment.fromLat,
          segment.fromLng,
          segment.toLat,
          segment.toLng,
        )
      }
    }

    socket.on('fire:update', onFireUpdate)
    socket.on('fire:ignite', onFireIgnite)
    socket.on('users:count', onUsersCount)
    socket.on('fire:spread', onFireSpread)

    return () => {
      socket.off('fire:update', onFireUpdate)
      socket.off('fire:ignite', onFireIgnite)
      socket.off('users:count', onUsersCount)
      socket.off('fire:spread', onFireSpread)
    }
  }, [updateFire, setOnlineUsers])
}
