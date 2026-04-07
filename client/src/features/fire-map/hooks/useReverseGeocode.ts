/**
 * 역지오코딩 훅 — GPS 좌표 → 도로명 주소
 *
 * Nominatim (OpenStreetMap) 무료 API 사용.
 * - 좌표를 소수점 3자리로 반올림 (~100m) → 미세 이동 시 재요청 방지
 * - 모듈 레벨 캐시 → 이전 위치 복귀 시 네트워크 요청 없음
 * - 1초 rate limit 안전장치
 * - 로딩 중 이전 주소 유지 (깜빡임 방지)
 *
 * 표시 범위: 시 > 구 > 도로명 + 번호까지만 (건물명·호수 제외)
 */

import { useState, useEffect, useRef } from 'react'

interface AddressParts {
  city: string    // 시 (서울특별시)
  district: string // 구 (광진구)
  road: string     // 로 + 번호 (모래내로 1길 4)
}

interface CacheEntry {
  address: string
  parts: AddressParts
}

interface ReverseGeocodeState {
  address: string | null
  parts: AddressParts | null
  loading: boolean
}

const MAX_CACHE = 200
const cache = new Map<string, CacheEntry>()
let lastFetchTime = 0

function round3(n: number): number {
  return Math.round(n * 1000) / 1000
}

function cacheKey(lat: number, lng: number): string {
  return `${round3(lat)},${round3(lng)}`
}

function evictIfNeeded() {
  if (cache.size <= MAX_CACHE) return
  const first = cache.keys().next().value
  if (first !== undefined) cache.delete(first)
}

export function useReverseGeocode(lat: number | null, lng: number | null): ReverseGeocodeState {
  const [state, setState] = useState<ReverseGeocodeState>({
    address: null,
    parts: null,
    loading: false,
  })
  const delayTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => {
    if (!lat || !lng) return

    const rLat = round3(lat)
    const rLng = round3(lng)
    const key = cacheKey(rLat, rLng)

    // cache hit → 즉시 반환
    const cached = cache.get(key)
    if (cached) {
      setState({ address: cached.address, parts: cached.parts, loading: false })
      return
    }

    // 로딩 시작 — 이전 parts 유지
    setState((prev) => ({ ...prev, loading: true }))

    const controller = new AbortController()

    const doFetch = () => {
      lastFetchTime = Date.now()

      fetch(
        `https://nominatim.openstreetmap.org/reverse?lat=${rLat}&lon=${rLng}&format=json&accept-language=ko&zoom=18&addressdetails=1`,
        {
          signal: controller.signal,
          headers: { 'User-Agent': 'MELTTOWN/1.0' },
        },
      )
        .then((res) => res.json())
        .then((data) => {
          const addr = data.address
          const city = addr?.city || addr?.town || addr?.county || ''
          const district = addr?.borough || addr?.city_district || addr?.suburb || addr?.quarter || ''
          const road = [addr?.road, addr?.house_number].filter(Boolean).join(' ') || ''
          const allParts = [city, district, road].filter(Boolean)

          const result: ReverseGeocodeState = {
            address: allParts.length > 0 ? allParts.join(' ') : data.display_name?.split(',')[0] || null,
            parts: allParts.length > 0 ? { city, district, road } : null,
            loading: false,
          }

          // 캐시 저장
          if (result.address && result.parts) {
            evictIfNeeded()
            cache.set(key, { address: result.address, parts: result.parts })
          }

          setState(result)
        })
        .catch(() => {
          if (!controller.signal.aborted) {
            setState((prev) => ({ ...prev, loading: false }))
          }
        })
    }

    // rate limit: 마지막 fetch로부터 1초 이내면 지연
    const elapsed = Date.now() - lastFetchTime
    if (elapsed < 1000) {
      delayTimerRef.current = setTimeout(doFetch, 1000 - elapsed)
    } else {
      doFetch()
    }

    return () => {
      controller.abort()
      if (delayTimerRef.current) clearTimeout(delayTimerRef.current)
    }
  }, [lat, lng])

  return state
}
