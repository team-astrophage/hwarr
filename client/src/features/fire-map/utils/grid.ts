import { LAT_UNIT, LNG_UNIT } from '../../../lib/config'

export function getGridId(lat: number, lng: number): string {
  const gridLat = Math.floor(lat / LAT_UNIT)
  const gridLng = Math.floor(lng / LNG_UNIT)
  return `${gridLat}:${gridLng}`
}

export function getGridCenter(gridId: string): [number, number] {
  const [latStr, lngStr] = gridId.split(':')
  const lat = Number(latStr) * LAT_UNIT + LAT_UNIT / 2
  const lng = Number(lngStr) * LNG_UNIT + LNG_UNIT / 2
  return [lat, lng]
}
