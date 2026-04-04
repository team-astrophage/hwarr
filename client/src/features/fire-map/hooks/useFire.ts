/**
 * 불 지르기 액션 훅
 *
 * 쿨다운 없이 즉시 발사. socket.emit으로 서버에 이벤트 전송.
 */

import { useCallback } from 'react'
import { socket } from '../../../lib/socket'

export function useFire() {
  const fire = useCallback((lat: number, lng: number) => {
    socket.emit('fire', { lat, lng })
  }, [])

  return { fire }
}
