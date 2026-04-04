/**
 * 역지오코딩 훅 — GPS 좌표 → 도로명 주소
 *
 * Nominatim (OpenStreetMap) 무료 API 사용.
 * 좌표가 변경될 때 한 번만 호출, 결과 캐싱.
 */

import { useState, useEffect } from 'react'

interface AddressParts {
  city: string    // 시 (서울특별시)
  district: string // 구 (광진구)
  road: string     // 로 (구의강변로)
}

interface ReverseGeocodeState {
  address: string | null
  parts: AddressParts | null
  loading: boolean
}

export function useReverseGeocode(lat: number | null, lng: number | null): ReverseGeocodeState {
  const [state, setState] = useState<ReverseGeocodeState>({
    address: null,
    parts: null,
    loading: false,
  })

  useEffect(() => {
    if (!lat || !lng) return

    setState({ address: null, parts: null, loading: true })

    const controller = new AbortController()

    fetch(
      `https://nominatim.openstreetmap.org/reverse?lat=${lat}&lon=${lng}&format=json&accept-language=ko&zoom=18&addressdetails=1`,
      {
        signal: controller.signal,
        headers: { 'User-Agent': 'MELTTOWN/1.0' },
      },
    )
      .then((res) => res.json())
      .then((data) => {
        const addr = data.address
        const city = addr?.city || addr?.town || addr?.county || ''
        const district = addr?.borough || addr?.suburb || addr?.quarter || ''
        const road = [addr?.road, addr?.house_number].filter(Boolean).join(' ') || ''
        const allParts = [city, district, road].filter(Boolean)

        setState({
          address: allParts.length > 0 ? allParts.join(' ') : data.display_name?.split(',')[0] || null,
          parts: allParts.length > 0 ? { city, district, road } : null,
          loading: false,
        })
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setState({ address: null, parts: null, loading: false })
        }
      })

    return () => controller.abort()
  }, [lat, lng])

  return state
}
