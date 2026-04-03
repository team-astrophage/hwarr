/**
 * Overpass API 건물 폴리곤 fetcher + 캐시
 *
 * 줌 >= 16에서 뷰포트 내 건물 데이터를 가져와 캐싱.
 * 건물 폴리곤 좌표를 반환하여 Canvas clip에 사용.
 */

export interface BuildingPolygon {
  id: number
  /** [lat, lng][] 좌표 배열 */
  coords: [number, number][]
}

// 캐시: 뷰포트 키 → 건물 목록
const buildingCache = new Map<string, BuildingPolygon[]>()

/** 뷰포트 키 생성 (소수점 3자리로 양자화) */
function viewportKey(south: number, west: number, north: number, east: number): string {
  const q = (n: number) => (Math.floor(n * 1000) / 1000).toFixed(3)
  return `${q(south)},${q(west)},${q(north)},${q(east)}`
}

/** Overpass API로 건물 폴리곤 fetch */
export async function fetchBuildings(
  south: number,
  west: number,
  north: number,
  east: number,
): Promise<BuildingPolygon[]> {
  const key = viewportKey(south, west, north, east)

  // 캐시 히트
  if (buildingCache.has(key)) return buildingCache.get(key)!

  const query = `
    [out:json][timeout:5];
    way["building"](${south},${west},${north},${east});
    out geom;
  `.trim()

  try {
    const res = await fetch('https://overpass-api.de/api/interpreter', {
      method: 'POST',
      body: `data=${encodeURIComponent(query)}`,
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      signal: AbortSignal.timeout(5000),
    })

    if (!res.ok) return []

    const data = await res.json()
    const buildings: BuildingPolygon[] = []

    for (const el of data.elements ?? []) {
      if (el.type === 'way' && el.geometry) {
        buildings.push({
          id: el.id,
          coords: el.geometry.map((g: { lat: number; lon: number }) => [g.lat, g.lon] as [number, number]),
        })
      }
    }

    buildingCache.set(key, buildings)

    // 캐시 크기 제한 (최대 20개 뷰포트)
    if (buildingCache.size > 20) {
      const first = buildingCache.keys().next().value
      if (first) buildingCache.delete(first)
    }

    return buildings
  } catch {
    // fetch 실패 시 빈 배열 (기존 격자로 fallback)
    return []
  }
}
