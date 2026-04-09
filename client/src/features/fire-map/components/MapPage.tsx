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
import { useMapCenter } from '../hooks/useMapCenter';
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

/** 소수점 3자리 반올림 (~100m) 기준으로 두 좌표가 같은 위치인지 판단 */
function isSameLocation(
  aLat: number | null,
  aLng: number | null,
  bLat: number,
  bLng: number,
): boolean {
  if (aLat == null || aLng == null) return false;
  return (
    Math.round(aLat * 1000) === Math.round(bLat * 1000) &&
    Math.round(aLng * 1000) === Math.round(bLng * 1000)
  );
}

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

/** 맵 중심점을 부모로 전달하는 브릿지 컴포넌트 */
function MapCenterTracker({
  onCenterChange,
  immediateRef,
}: {
  onCenterChange: (lat: number, lng: number) => void;
  immediateRef: React.MutableRefObject<(() => void) | null>;
}) {
  const { lat, lng, immediate } = useMapCenter(800);
  immediateRef.current = immediate;

  // 마운트 직후 초기 중심(KOREA_CENTER)은 건너뛰고, 실제 이동 후부터 보고
  const isFirstRef = useRef(true);
  useEffect(() => {
    if (isFirstRef.current) {
      isFirstRef.current = false;
      return;
    }
    onCenterChange(lat, lng);
  }, [lat, lng, onCenterChange]);

  return null;
}

export function MapPage() {
  const mapRef = useRef<L.Map | null>(null);
  const immediateRef = useRef<(() => void) | null>(null);
  const [locationRequested, setLocationRequested] = useState(false);
  const { lat, lng, loading, error, permissionDenied, retry } = useGeolocation({
    enabled: locationRequested,
  });
  const [locationDismissed, setLocationDismissed] = useState(false);
  const [disclaimerAccepted, setDisclaimerAccepted] = useState(() => isDismissedToday());

  // Auto-skip pre-permission for returning users with granted permission or demo mode
  useEffect(() => {
    if (!disclaimerAccepted || locationRequested) return;
    if (window.__MELTTOWN_GPS) { setLocationRequested(true); return; }

    navigator.permissions?.query?.({ name: 'geolocation' as PermissionName })
      .then((s) => { if (s.state === 'granted') setLocationRequested(true); })
      .catch(() => {});
  }, [disclaimerAccepted, locationRequested]);
  const fires = useFireStore((s) => s.fires);
  const gridId = lat && lng ? getGridId(lat, lng) : null;

  useFireSocket();
  const { fire } = useFire();

  const throwMatch = useAnimationStore((s) => s.throwMatch);
  const [chatOpen, setChatOpen] = useState(false);

  // 맵 중심점 상태
  const [mapCenter, setMapCenter] = useState<{ lat: number; lng: number } | null>(null);
  const handleCenterChange = useCallback((cLat: number, cLng: number) => {
    setMapCenter({ lat: cLat, lng: cLng });
  }, []);

  // 내 GPS 위치를 보고 있는지 판단
  const isAtMyLocation = mapCenter
    ? isSameLocation(lat, lng, mapCenter.lat, mapCenter.lng)
    : true; // 초기 상태에서는 내 위치로 간주

  // 주소: mapCenter가 null(초기 상태)이면 GPS fallback, 이동 후에는 맵 중심 기반
  const geocodeLat = mapCenter?.lat ?? lat;
  const geocodeLng = mapCenter?.lng ?? lng;
  const { parts } = useReverseGeocode(geocodeLat, geocodeLng);

  // tap → 성냥 던지기 + 불 이벤트
  const handleFire = useCallback(() => {
    if (lat && lng && gridId) {
      throwMatch(gridId);
      fire(lat, lng);
    }
  }, [lat, lng, gridId, throwMatch, fire]);

  // 랜덤 화재 지역 구경하기 (내 위치 제외)
  const visitListenerRef = useRef<(() => void) | null>(null);
  const handleVisit = useCallback(() => {
    if (fires.size === 0 || !mapRef.current) return;
    const keys = Array.from(fires.keys()).filter((k) => k !== gridId);
    if (keys.length === 0) return;
    const randomKey = keys[Math.floor(Math.random() * keys.length)];
    const [centerLat, centerLng] = getGridCenter(randomKey);
    // 이전 리스너 제거 (연타 시 누적 방지)
    if (visitListenerRef.current) {
      mapRef.current.off('moveend', visitListenerRef.current);
    }
    const onArrival = () => {
      visitListenerRef.current = null;
      immediateRef.current?.();
    };
    visitListenerRef.current = onArrival;
    mapRef.current.flyTo([centerLat, centerLng], 16, { duration: 1.5 });
    mapRef.current.once('moveend', onArrival);
  }, [fires, gridId]);

  return (
    <div className='relative h-svh w-full'>
      <title>실시간 가상 불 지도 — 화르르</title>
      <meta name="description" content="지도 위 내 위치에 가상의 불을 피우고 전국의 실시간 현황을 확인하세요." />
      <MapContainer
        center={KOREA_CENTER}
        zoom={7}
        className='h-full w-full'
        zoomControl={false}
        maxBounds={KOREA_BOUNDS}
        maxBoundsViscosity={1.0}
        minZoom={7}
        maxZoom={18}
      >
        <MapRef mapRef={mapRef} />
        <MapCenterTracker onCenterChange={handleCenterChange} immediateRef={immediateRef} />
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

      {/* 십자선 — 내 GPS 위치가 아닐 때만 표시 */}
      {!isAtMyLocation && (
        <div
          className='absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-[1000] pointer-events-none'
          aria-hidden='true'
        >
          <svg width='24' height='24' viewBox='0 0 24 24' fill='none'>
            <line x1='12' y1='4' x2='12' y2='10' stroke='rgba(255,255,255,0.5)' strokeWidth='1.5' strokeLinecap='round' />
            <line x1='12' y1='14' x2='12' y2='20' stroke='rgba(255,255,255,0.5)' strokeWidth='1.5' strokeLinecap='round' />
            <line x1='4' y1='12' x2='10' y2='12' stroke='rgba(255,255,255,0.5)' strokeWidth='1.5' strokeLinecap='round' />
            <line x1='14' y1='12' x2='20' y2='12' stroke='rgba(255,255,255,0.5)' strokeWidth='1.5' strokeLinecap='round' />
          </svg>
        </div>
      )}

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

      {/* Location permission flow — only after disclaimer */}
      {disclaimerAccepted && !locationDismissed && !lat && (() => {
        // GPS loading
        if (locationRequested && loading) {
          return (
            <LocationPermissionModal
              mode='loading'
              permissionDenied={false}
              onRetry={retry}
              onDismiss={() => setLocationDismissed(true)}
            />
          );
        }
        // Error/denied after attempt
        if (locationRequested && !loading && error) {
          return (
            <LocationPermissionModal
              mode='error'
              permissionDenied={permissionDenied}
              onRetry={retry}
              onDismiss={() => setLocationDismissed(true)}
            />
          );
        }
        // Already denied from previous session — skip pre-permission
        if (!locationRequested && permissionDenied) {
          return (
            <LocationPermissionModal
              mode='error'
              permissionDenied={true}
              onRetry={retry}
              onDismiss={() => setLocationDismissed(true)}
            />
          );
        }
        // First visit — show pre-permission
        if (!locationRequested) {
          return (
            <LocationPermissionModal
              mode='pre-permission'
              permissionDenied={false}
              onRetry={() => setLocationRequested(true)}
              onDismiss={() => setLocationDismissed(true)}
            />
          );
        }
        return null;
      })()}

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
