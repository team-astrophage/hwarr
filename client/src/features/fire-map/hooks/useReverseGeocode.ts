/**
 * 역지오코딩 훅 — GPS 좌표 → 도로명 주소
 *
 * Nominatim (OpenStreetMap) 무료 API 사용.
 * 좌표가 변경될 때 한 번만 호출, 결과 캐싱.
 */

import { useState, useEffect } from 'react'

interface ReverseGeocodeState {
  address: string | null
  loading: boolean
}

export function useReverseGeocode(lat: number | null, lng: number | null): ReverseGeocodeState {
  const [state, setState] = useState<ReverseGeocodeState>({
    address: null,
    loading: false,
  })

  useEffect(() => {
    if (!lat || !lng) return

    setState({ address: null, loading: true })

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
        // 도로명 주소 조합: 도시 + 구 + 도로명
        const parts = [
          addr?.city || addr?.town || addr?.county || '',
          addr?.borough || addr?.suburb || addr?.quarter || '',
          addr?.road || '',
        ].filter(Boolean)

        setState({
          address: parts.length > 0 ? parts.join(' ') : data.display_name?.split(',')[0] || null,
          loading: false,
        })
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setState({ address: null, loading: false })
        }
      })

    return () => controller.abort()
  }, [lat, lng])

  return state
}
