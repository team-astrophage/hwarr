import { MapContainer, TileLayer, useMap, Marker } from 'react-leaflet'
import { useEffect, useCallback, useMemo, useRef } from 'react'
import L from 'leaflet'
import { useGeolocation } from '../hooks/useGeolocation'
import { useFireSocket } from '../hooks/useFireSocket'
import { useFire } from '../hooks/useFire'
import { getGridId } from '../utils/grid'
import { useAnimationStore } from '../stores/animationStore'
import { MapControls } from './MapControls'
import { BottomPanel } from './BottomPanel'
import { FireOverlay } from './FireOverlay'
import { Header } from '../../../components/Header'
import 'leaflet/dist/leaflet.css'

const KOREA_CENTER: [number, number] = [36.5, 127.5]
const KOREA_BOUNDS: [[number, number], [number, number]] = [
  [33.0, 124.5],
  [39.0, 132.0],
]

function FlyToUser({ lat, lng }: { lat: number; lng: number }) {
  const map = useMap()
  useEffect(() => {
    map.flyTo([lat, lng], 16, { duration: 2 })
  }, [map, lat, lng])
  return null
}

function UserLocationMarker({ lat, lng }: { lat: number; lng: number }) {
  const icon = useMemo(
    () =>
      L.divIcon({
        className: '',
        html: `<div style="position:relative;width:24px;height:24px">
          <div style="position:absolute;inset:0;border-radius:50%;background:rgba(66,133,244,0.2);animation:pulse 2s ease-out infinite"></div>
          <div style="position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);width:12px;height:12px;border-radius:50%;background:#4285f4;border:2.5px solid #fff;box-shadow:0 0 6px rgba(66,133,244,0.6)"></div>
        </div>`,
        iconSize: [24, 24],
        iconAnchor: [12, 12],
      }),
    [],
  )
  return <Marker position={[lat, lng]} icon={icon} interactive={false} />
}

function LocateButton({ lat, lng }: { lat: number; lng: number }) {
  const map = useMap()
  const handleLocate = useCallback(() => {
    map.flyTo([lat, lng], 16, { duration: 1 })
  }, [map, lat, lng])
  return <MapControls onLocate={handleLocate} />
}

export function MapPage() {
  const { lat, lng, loading, error } = useGeolocation()
  const gridId = lat && lng ? getGridId(lat, lng) : null

  useFireSocket()
  const { fire } = useFire()

  const throwMatch = useAnimationStore((s) => s.throwMatch)
  const startFlamethrower = useAnimationStore((s) => s.startFlamethrower)
  const stopFlamethrower = useAnimationStore((s) => s.stopFlamethrower)
  const flamethrowerIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // tap → 성냥 던지기 + 불 이벤트
  const handleFire = useCallback(() => {
    if (lat && lng && gridId) {
      throwMatch(gridId)
      fire(lat, lng)
    }
  }, [lat, lng, gridId, throwMatch, fire])

  // long press 시작 → 화염방사기 + 연속 불 이벤트
  const handleLongPressFire = useCallback(() => {
    if (!lat || !lng || !gridId) return
    startFlamethrower(gridId)
    fire(lat, lng)
    // 200ms 간격으로 연속 발사
    flamethrowerIntervalRef.current = setInterval(() => {
      fire(lat, lng)
    }, 200)
  }, [lat, lng, gridId, startFlamethrower, fire])

  // long press 종료
  const handleLongPressEnd = useCallback(() => {
    stopFlamethrower()
    if (flamethrowerIntervalRef.current) {
      clearInterval(flamethrowerIntervalRef.current)
      flamethrowerIntervalRef.current = null
    }
  }, [stopFlamethrower])

  return (
    <div className="relative h-svh w-full">
      <MapContainer
        center={KOREA_CENTER}
        zoom={7}
        className="h-full w-full"
        zoomControl={false}
        maxBounds={KOREA_BOUNDS}
        maxBoundsViscosity={1.0}
        minZoom={7}
        maxZoom={18}
      >
        <TileLayer
          attribution='&copy; <a href="https://carto.com/">CARTO</a>'
          url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
        />
        <FireOverlay />
        {lat && lng && (
          <>
            <FlyToUser lat={lat} lng={lng} />
            <UserLocationMarker lat={lat} lng={lng} />
            <LocateButton lat={lat} lng={lng} />
          </>
        )}
      </MapContainer>

      <Header />

      {!loading && error && (
        <div className="absolute top-16 left-4 right-4 z-[1000]">
          <div className="bg-[var(--color-bg-surface)] text-[var(--color-negative)] rounded-[12px] px-4 py-3 text-[0.8125rem] font-bold shadow-[var(--shadow-heavy)] text-center">
            위치 권한을 허용해주세요
          </div>
        </div>
      )}

      <BottomPanel
        gridId={gridId}
        onFire={handleFire}
        onLongPressFire={handleLongPressFire}
        onLongPressEnd={handleLongPressEnd}
        disabled={!lat || !lng}
      />
    </div>
  )
}
