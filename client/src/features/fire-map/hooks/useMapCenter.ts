/**
 * 맵 중심점 추적 훅 — moveend 이벤트 + debounce
 *
 * react-leaflet의 useMap() 사용 → MapContainer 내부에서만 호출 가능.
 */

import { useState, useEffect, useCallback, useRef } from 'react'
import { useMap } from 'react-leaflet'

interface MapCenter {
  lat: number
  lng: number
}

interface UseMapCenterReturn extends MapCenter {
  /** debounce를 무시하고 현재 중심점을 즉시 반영 */
  immediate: () => void
}

export function useMapCenter(debounceMs = 800): UseMapCenterReturn {
  const map = useMap()
  const [center, setCenter] = useState<MapCenter>(() => {
    const c = map.getCenter()
    return { lat: c.lat, lng: c.lng }
  })
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  const immediate = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current)
    const c = map.getCenter()
    setCenter({ lat: c.lat, lng: c.lng })
  }, [map])

  useEffect(() => {
    const onMoveEnd = () => {
      if (timerRef.current) clearTimeout(timerRef.current)
      timerRef.current = setTimeout(() => {
        const c = map.getCenter()
        setCenter({ lat: c.lat, lng: c.lng })
      }, debounceMs)
    }

    map.on('moveend', onMoveEnd)
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
      map.off('moveend', onMoveEnd)
    }
  }, [map, debounceMs])

  return { ...center, immediate }
}
