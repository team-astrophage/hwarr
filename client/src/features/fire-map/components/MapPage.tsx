import { MapContainer, TileLayer, useMap, Marker } from 'react-leaflet';
import { useEffect, useCallback, useMemo, useState, useRef } from 'react';
import L from 'leaflet';
import { useGeolocation } from '../hooks/useGeolocation';
import { useFireSocket } from '../hooks/useFireSocket';
import { useFire } from '../hooks/useFire';
import { getGridId, getGridCenter } from '../utils/grid';
import { useFireStore } from '../stores/fireStore';
import { useAnimationStore } from '../stores/animationStore';
import { useReverseGeocode } from '../hooks/useReverseGeocode';
import { MapControls } from './MapControls';
import { BottomPanel } from './BottomPanel';
import { FireOverlay } from './FireOverlay';
import { FiretruckOverlay } from './FiretruckOverlay';
import { ChatPanel } from '../../../components/ChatPanel';
import { Header } from '../../../components/Header';
import { LocationPermissionModal } from './LocationPermissionModal';
import { DisclaimerModal, isDismissedToday } from '../../../components/DisclaimerModal';
import 'leaflet/dist/leaflet.css';

const KOREA_CENTER: [number, number] = [36.5, 127.5];
const SEOUL_CENTER: [number, number] = [37.5665, 126.978];
const KOREA_BOUNDS: [[number, number], [number, number]] = [
  [32.0, 124.0],
  [39.5, 132.5],
];

function FlyToUser({ lat, lng }: { lat: number; lng: number }) {
  const map = useMap();
  useEffect(() => {
    map.flyTo([lat, lng], 16, { duration: 2 });
  }, [map, lat, lng]);
  return null;
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
  );
  return <Marker position={[lat, lng]} icon={icon} interactive={false} />;
}

function LocateButton({ lat, lng }: { lat: number; lng: number }) {
  const map = useMap();
  const handleLocate = useCallback(() => {
    map.flyTo([lat, lng], 16, { duration: 1 });
  }, [map, lat, lng]);
  return <MapControls onLocate={handleLocate} />;
}

function MapRef({ mapRef }: { mapRef: React.MutableRefObject<L.Map | null> }) {
  const map = useMap();
  mapRef.current = map;
  return null;
}

export function MapPage() {
  const mapRef = useRef<L.Map | null>(null);
  const { lat, lng, loading, error, permissionDenied, retry } = useGeolocation();
  const [locationDismissed, setLocationDismissed] = useState(false);
  const [disclaimerAccepted, setDisclaimerAccepted] = useState(() => isDismissedToday());
  const fires = useFireStore((s) => s.fires);
  const gridId = lat && lng ? getGridId(lat, lng) : null;

  useFireSocket();
  const { fire } = useFire();

  const throwMatch = useAnimationStore((s) => s.throwMatch);
  const [chatOpen, setChatOpen] = useState(false);
  const { parts } = useReverseGeocode(lat, lng);

  // tap → 성냥 던지기 + 불 이벤트
  const handleFire = useCallback(() => {
    if (lat && lng && gridId) {
      throwMatch(gridId);
      fire(lat, lng);
    }
  }, [lat, lng, gridId, throwMatch, fire]);

  // 랜덤 화재 지역 구경하기 (내 위치 제외)
  const handleVisit = useCallback(() => {
    if (fires.size === 0 || !mapRef.current) return;
    const keys = Array.from(fires.keys()).filter((k) => k !== gridId);
    if (keys.length === 0) return;
    const randomKey = keys[Math.floor(Math.random() * keys.length)];
    const [centerLat, centerLng] = getGridCenter(randomKey);
    mapRef.current.flyTo([centerLat, centerLng], 16, { duration: 1.5 });
  }, [fires, gridId]);

  return (
    <div className='relative h-svh w-full'>
      <title>실시간 불 지도 — 화르르</title>
      <meta name="description" content="내 위치에 불을 지르고 전국의 실시간 화재 현황을 확인하세요." />
      <MapContainer
        center={KOREA_CENTER}
        zoom={13}
        className='h-full w-full'
        zoomControl={false}
        maxBounds={KOREA_BOUNDS}
        maxBoundsViscosity={1.0}
        minZoom={7}
        maxZoom={18}
      >
        <MapRef mapRef={mapRef} />
        <TileLayer
          attribution='&copy; <a href="https://carto.com/">CARTO</a>'
          url='https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
        />
        <FireOverlay />
        <FiretruckOverlay />
        {lat && lng && (
          <>
            <FlyToUser lat={lat} lng={lng} />
            <UserLocationMarker lat={lat} lng={lng} />
            <LocateButton lat={lat} lng={lng} />
          </>
        )}
        {locationDismissed && !lat && <FlyToUser lat={SEOUL_CENTER[0]} lng={SEOUL_CENTER[1]} />}
      </MapContainer>

      <Header />

      {/* 현재 위치 도로명 주소 */}
      {parts && (
        <div className='absolute top-14 left-4 z-[1000]'>
          {parts.city && (
            <p className='text-4xl font-extrabold text-white leading-tight'>
              {parts.city}
            </p>
          )}
          {parts.district && (
            <p className='text-4xl font-extrabold text-white leading-tight'>
              {parts.district}
            </p>
          )}
          {parts.road && (
            <p className='text-4xl font-extrabold text-white leading-tight'>
              {parts.road}
            </p>
          )}
        </div>
      )}

      {!loading && error && !locationDismissed && (
        <LocationPermissionModal
          permissionDenied={permissionDenied}
          onRetry={retry}
          onDismiss={() => setLocationDismissed(true)}
        />
      )}

      {/* 채팅 토글 버튼 */}
      {!chatOpen && (
        <button
          onClick={() => setChatOpen(true)}
          className='absolute bottom-[180px] right-4 z-[1000] w-12 h-12 bg-[var(--color-bg-surface)] rounded-full shadow-[var(--shadow-heavy)] flex items-center justify-center transition-transform active:scale-90'
        >
          <svg
            width='20'
            height='20'
            viewBox='0 0 24 24'
            fill='none'
            stroke='var(--color-accent)'
            strokeWidth='2'
            strokeLinecap='round'
            strokeLinejoin='round'
          >
            <path d='M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z' />
          </svg>
        </button>
      )}

      <ChatPanel visible={chatOpen} onClose={() => setChatOpen(false)} />

      <BottomPanel
        gridId={gridId}
        onFire={handleFire}
        onVisit={handleVisit}
        disabled={!lat || !lng}
        noLocation={locationDismissed && !lat}
      />

      {!disclaimerAccepted && (
        <DisclaimerModal onAccept={() => setDisclaimerAccepted(true)} />
      )}
    </div>
  );
}
