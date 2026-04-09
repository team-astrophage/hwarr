import { useState, useEffect, useCallback, useRef } from 'react'

interface GeolocationState {
  lat: number | null
  lng: number | null
  error: string | null
  loading: boolean
  idle: boolean
  permissionDenied: boolean
  retry: () => void
}

// GPS override backdoor for demo
declare global {
  interface Window {
    __MELTTOWN_GPS?: { lat: number; lng: number }
  }
}

export function useGeolocation(options?: { enabled?: boolean }): GeolocationState {
  const enabled = options?.enabled !== false
  const [lat, setLat] = useState<number | null>(null)
  const [lng, setLng] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState<boolean>(false)
  const [idle, setIdle] = useState<boolean>(true)
  const [permissionDenied, setPermissionDenied] = useState<boolean>(false)
  const hasRequested = useRef(false)

  const requestLocation = useCallback(() => {
    setIdle(false)
    hasRequested.current = true

    // Check backdoor first
    if (window.__MELTTOWN_GPS) {
      setLat(window.__MELTTOWN_GPS.lat)
      setLng(window.__MELTTOWN_GPS.lng)
      setError(null)
      setPermissionDenied(false)
      setLoading(false)
      return
    }

    if (!navigator.geolocation) {
      setError('Geolocation을 지원하지 않는 브라우저입니다')
      setLoading(false)
      return
    }

    setLoading(true)
    setError(null)

    navigator.geolocation.getCurrentPosition(
      (position) => {
        setLat(position.coords.latitude)
        setLng(position.coords.longitude)
        setError(null)
        setPermissionDenied(false)
        setLoading(false)
      },
      (err) => {
        setError(err.message)
        setLoading(false)
        if (err.code === err.PERMISSION_DENIED) {
          setPermissionDenied(true)
        }
      },
      {
        enableHighAccuracy: true,
        timeout: 10000,
        maximumAge: 0,
      },
    )
  }, [])

  useEffect(() => {
    // Pre-check permission state (when supported) so we can show the
    // "blocked" variant immediately without waiting for a timeout.
    if (navigator.permissions?.query) {
      navigator.permissions
        .query({ name: 'geolocation' as PermissionName })
        .then((status) => {
          if (status.state === 'denied') {
            setPermissionDenied(true)
          }
        })
        .catch(() => {
          // ignore — fall through to normal getCurrentPosition flow
        })
    }

    if (enabled && !hasRequested.current) {
      requestLocation()
    }
  }, [requestLocation, enabled])

  return { lat, lng, error, loading, idle, permissionDenied, retry: requestLocation }
}
