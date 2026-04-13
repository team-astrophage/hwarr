/**
 * 불 지르기 액션 훅
 *
 * 쿨다운 없이 즉시 발사. socket.emit 으로 서버에 이벤트 전송.
 * 서버 측 rate limiter (17/s token-bucket)가 과도한 요청을 차단한다.
 *
 * 오프라인 정책: 연결이 끊긴 상태에서는 emit 을 드롭한다.
 *   - Socket.IO 기본 동작은 오프라인 emit 을 버퍼링해뒀다가 재접속 시 한꺼번에
 *     flush 하는데, long-press 연속 발사(200ms)와 맞물리면 재접속 직후 수십
 *     건이 서버로 쏟아져 애니메이션과 서버 부하가 동시에 터진다.
 *   - 현재 UX 상 "지른 순간"이 의미가 있으므로 뒤늦은 재시도보다 드롭이 낫다.
 */

import { useCallback } from 'react'
import { socket } from '../../../lib/socket'

interface FireIgniteAck {
  status?: 'ok' | string
  gridId?: string
  requestedGridId?: string
  eventId?: string
  lat?: number
  lng?: number
  activeCount?: number
  stage?: number
  error?: string
}

export function useFire() {
  const fire = useCallback(
    (
      lat: number,
      lng: number,
      onAck?: (ack: FireIgniteAck) => void,
    ) => {
      if (!socket.connected) {
        console.warn('[Socket] fire dropped (disconnected)', { lat, lng })
        return
      }
      socket.emit('fire:ignite', { lat, lng }, (ack: FireIgniteAck) => {
        onAck?.(ack)
      })
    },
    [],
  )

  return { fire }
}
