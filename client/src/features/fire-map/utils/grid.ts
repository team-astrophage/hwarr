export const GRID_SIZE = 0.001 // ~100m

export function getGridId(lat: number, lng: number): string {
  const gridLat = Math.floor(lat / GRID_SIZE)
  const gridLng = Math.floor(lng / GRID_SIZE)
  return `${gridLat}:${gridLng}`
}

export function getGridCenter(gridId: string): [number, number] {
  const [latStr, lngStr] = gridId.split(':')
  const lat = Number(latStr) * GRID_SIZE + GRID_SIZE / 2
  const lng = Number(lngStr) * GRID_SIZE + GRID_SIZE / 2
  return [lat, lng]
}
