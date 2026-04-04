/**
 * 소방차 시각 이펙트 오버레이
 *
 * 4단계(대형화재) 이상 격자 옆에 소방차 아이콘 표시.
 * 좌우 흔들림 + 경광등 점멸 CSS 애니메이션.
 * 실제 불 단계 감소 기능 없음 (시각 전용).
 */

import { useMemo, useState, useEffect } from 'react'
import { Marker, useMap } from 'react-leaflet'
import L from 'leaflet'
import { useFireStore } from '../stores/fireStore'
import { LAT_UNIT, LNG_UNIT } from '../../../lib/config'

const FIRETRUCK_STAGE_THRESHOLD = 4
const FIRETRUCK_MIN_ZOOM = 14

function createFiretruckIcon() {
  return L.divIcon({
    className: '',
    html: `<div style="position:relative;width:32px;height:32px;animation:truck-wobble 0.8s ease-in-out infinite alternate">
      <div style="position:absolute;top:-4px;left:50%;transform:translateX(-50%);width:6px;height:6px;border-radius:50%;animation:siren 0.4s linear infinite alternate;box-shadow:0 0 8px currentColor" class="firetruck-siren"></div>
      <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="4" y="10" width="24" height="14" rx="2" fill="#cc2222"/>
        <rect x="2" y="14" width="6" height="10" rx="1" fill="#aa1111"/>
        <rect x="22" y="8" width="8" height="6" rx="1" fill="#333" stroke="#555" stroke-width="0.5"/>
        <rect x="6" y="12" width="4" height="3" rx="0.5" fill="#ffdd00"/>
        <rect x="12" y="12" width="4" height="3" rx="0.5" fill="#ffdd00"/>
        <circle cx="8" cy="26" r="3" fill="#333" stroke="#555" stroke-width="0.5"/>
        <circle cx="24" cy="26" r="3" fill="#333" stroke="#555" stroke-width="0.5"/>
        <rect x="18" y="12" width="2" height="8" rx="0.5" fill="#ccc"/>
      </svg>
    </div>`,
    iconSize: [32, 32],
    iconAnchor: [16, 28],
  })
}

export function FiretruckOverlay() {
  const map = useMap()
  const fires = useFireStore((s) => s.fires)
  const [zoom, setZoom] = useState(map.getZoom())

  useEffect(() => {
    const onZoom = () => setZoom(map.getZoom())
    map.on('zoomend', onZoom)
    return () => { map.off('zoomend', onZoom) }
  }, [map])

  const icon = useMemo(() => createFiretruckIcon(), [])

  const firetruckPositions = useMemo(() => {
    const positions: { gridId: string; lat: number; lng: number }[] = []
    for (const [gridId, cell] of fires) {
      if (cell.stage >= FIRETRUCK_STAGE_THRESHOLD) {
        const [latStr, lngStr] = gridId.split(':')
        // 격자 오른쪽 하단 모서리에 배치
        const lat = Number(latStr) * LAT_UNIT
        const lng = Number(lngStr) * LNG_UNIT + LNG_UNIT
        positions.push({ gridId, lat, lng })
      }
    }
    return positions
  }, [fires])

  if (firetruckPositions.length === 0 || zoom < FIRETRUCK_MIN_ZOOM) return null

  return (
    <>
      {firetruckPositions.map((pos) => (
        <Marker
          key={pos.gridId}
          position={[pos.lat, pos.lng]}
          icon={icon}
          interactive={false}
        />
      ))}
    </>
  )
}
