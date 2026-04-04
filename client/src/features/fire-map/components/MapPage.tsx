import { MapContainer, TileLayer, useMap, Marker } from 'react-leaflet'
import { useEffect, useCallback, useMemo } from 'react'
import L from 'leaflet'
import { useGeolocation } from '../hooks/useGeolocation'
import { useFireSocket } from '../hooks/useFireSocket'
import { useFire } from '../hooks/useFire'
import { getGridId } from '../utils/grid'
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

  const handleFire = () => {
    if (lat && lng) {
      fire(lat, lng)
    }
  }

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
        disabled={!lat || !lng}
      />
    </div>
  )
}
