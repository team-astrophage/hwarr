import { useState, useEffect } from 'react'

interface GeolocationState {
  lat: number | null
  lng: number | null
  error: string | null
  loading: boolean
}

// GPS override backdoor for demo
declare global {
  interface Window {
    __MELTTOWN_GPS?: { lat: number; lng: number }
  }
}

export function useGeolocation(): GeolocationState {
  const [state, setState] = useState<GeolocationState>({
    lat: null,
    lng: null,
    error: null,
    loading: true,
  })

  useEffect(() => {
    // Check backdoor first
    if (window.__MELTTOWN_GPS) {
      setState({
        lat: window.__MELTTOWN_GPS.lat,
        lng: window.__MELTTOWN_GPS.lng,
        error: null,
        loading: false,
      })
      return
    }

    if (!navigator.geolocation) {
      setState((prev) => ({
        ...prev,
        error: 'Geolocation을 지원하지 않는 브라우저입니다',
        loading: false,
      }))
      return
    }

    navigator.geolocation.getCurrentPosition(
      (position) => {
        setState({
          lat: position.coords.latitude,
          lng: position.coords.longitude,
          error: null,
          loading: false,
        })
      },
      (err) => {
        setState((prev) => ({
          ...prev,
          error: err.message,
          loading: false,
        }))
      },
      {
        enableHighAccuracy: true,
        timeout: 10000,
        maximumAge: 0,
      },
    )
  }, [])

  return state
}
