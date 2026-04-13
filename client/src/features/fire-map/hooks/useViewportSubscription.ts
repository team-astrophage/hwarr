/**
 * 지도 viewport 기반 불 구독 훅
 *
 * subscribe:viewport 응답이 초기 데이터 역할을 겸한다 (gridMeta + activeFires).
 * 지도 이동 시 debounce 후 재구독하여 현재 영역의 active fires 만 수신한다.
 * 서버는 구독한 grid room 에만 fire:update / fire:ignite / fire:spread 를 broadcast 한다.
 */

import { useEffect } from 'react'
import type L from 'leaflet'
import { socket } from '../../../lib/socket'
import { useFireStore } from '../stores/fireStore'
import type { FireCell, GridMeta } from '../stores/fireStore'

interface SubscribeViewportAck {
  status: 'ok' | string
  gridMeta?: GridMeta
  subscribedGrids?: number
  activeFires?: FireCell[]
  error?: string
}

interface ViewportBounds {
  neLat: number
  neLng: number
  swLat: number
  swLng: number
}

const DEBOUNCE_MS = 300

function boundsFromMap(map: L.Map): ViewportBounds {
  const b = map.getBounds()
  const ne = b.getNorthEast()
  const sw = b.getSouthWest()
  return {
    neLat: ne.lat,
    neLng: ne.lng,
    swLat: sw.lat,
    swLng: sw.lng,
  }
}

export function useViewportSubscription(
  mapRef: React.MutableRefObject<L.Map | null>,
) {
  const syncFires = useFireStore((s) => s.syncFires)
  const setGridMeta = useFireStore((s) => s.setGridMeta)

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null
    let disposed = false

    const emitSubscribe = () => {
      const map = mapRef.current
      if (!map) return
      const bounds = boundsFromMap(map)
      socket.emit('subscribe:viewport', bounds, (ack: SubscribeViewportAck) => {
        if (disposed) return
        if (ack && ack.status === 'ok') {
          if (ack.gridMeta) setGridMeta(ack.gridMeta)
          if (ack.activeFires) syncFires(ack.activeFires)
        } else {
          console.warn('[subscribe:viewport] failed', ack)
        }
      })
    }

    const scheduleSubscribe = () => {
      if (timer) clearTimeout(timer)
      timer = setTimeout(emitSubscribe, DEBOUNCE_MS)
    }

    const attachMoveEnd = () => {
      const map = mapRef.current
      if (!map) return
      map.on('moveend', scheduleSubscribe)
      map.on('zoomend', scheduleSubscribe)
    }

    const detachMoveEnd = () => {
      const map = mapRef.current
      if (!map) return
      map.off('moveend', scheduleSubscribe)
      map.off('zoomend', scheduleSubscribe)
    }

    // 맵이 마운트된 뒤 한 틱 기다렸다가 초기 구독 + 리스너 부착
    const initTimer = setTimeout(() => {
      if (disposed) return
      attachMoveEnd()
      if (socket.connected) emitSubscribe()
    }, 0)

    // 재접속 시에도 재구독
    const onConnect = () => emitSubscribe()
    socket.on('connect', onConnect)

    return () => {
      disposed = true
      clearTimeout(initTimer)
      if (timer) clearTimeout(timer)
      detachMoveEnd()
      socket.off('connect', onConnect)
    }
  }, [mapRef, syncFires, setGridMeta])
}
