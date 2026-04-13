/**
 * Count how many of the 8 neighbors have active fires.
 * Used for density-based LOD: dense clusters can reduce per-cell particles
 * since overlapping flames are visually indistinguishable.
 */
export function getNeighborFireCount(
  gridId: string,
  fires: Map<string, unknown>,
): number {
  const [latStr, lngStr] = gridId.split(':')
  const lat = Number(latStr)
  const lng = Number(lngStr)
  let count = 0
  for (let dl = -1; dl <= 1; dl++) {
    for (let dg = -1; dg <= 1; dg++) {
      if (dl === 0 && dg === 0) continue
      if (fires.has(`${lat + dl}:${lng + dg}`)) count++
    }
  }
  return count
}

/**
 * Particle multiplier based on neighbor density.
 * Conservative: preserves visual quality while reducing load.
 */
export function getDensityMultiplier(neighborCount: number): number {
  if (neighborCount <= 0) return 1.0
  if (neighborCount <= 3) return 0.9
  if (neighborCount <= 6) return 0.75
  return 0.6
}
