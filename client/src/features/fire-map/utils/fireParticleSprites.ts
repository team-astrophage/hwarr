/**
 * Pre-rendered particle sprite cache for FireCanvas.
 *
 * Radial-gradient circles are baked into OffscreenCanvas at init time.
 * At draw time only ctx.drawImage() is needed, eliminating per-frame
 * createRadialGradient / arc / fill calls.
 */

// ── Sprite configuration ──

const SPRITE_SIZE = 64
const FLAME_BUCKETS = 32
const SMOKE_BUCKETS = 8

// ── Types (mirrors FireCanvas StageCfg subset) ──

export interface StageCfgForSprite {
  colorStops: [number, number, number, number][]
  coreAlpha: number
  dotColor: string
}

// ── Color interpolation ──

export function lerpColor(
  stops: [number, number, number, number][],
  heightRatio: number,
): [number, number, number, number] {
  const t = Math.min(heightRatio, 1) * (stops.length - 1)
  const i = Math.floor(t)
  const f = t - i
  const a = stops[Math.min(i, stops.length - 1)]
  const b = stops[Math.min(i + 1, stops.length - 1)]
  return [
    a[0] + (b[0] - a[0]) * f,
    a[1] + (b[1] - a[1]) * f,
    a[2] + (b[2] - a[2]) * f,
    a[3] + (b[3] - a[3]) * f,
  ]
}

export function lerpAlpha(
  stops: [number, number, number, number][],
  heightRatio: number,
): number {
  const t = Math.min(heightRatio, 1) * (stops.length - 1)
  const i = Math.floor(t)
  const f = t - i
  const a = stops[Math.min(i, stops.length - 1)]
  const b = stops[Math.min(i + 1, stops.length - 1)]
  return a[3] + (b[3] - a[3]) * f
}

// ── Helper: create a small offscreen canvas with a radial gradient ──

function makeRadial(
  size: number,
  stops: [number, string][],
): OffscreenCanvas {
  const oc = new OffscreenCanvas(size, size)
  const ctx = oc.getContext('2d')!
  const half = size / 2
  const grad = ctx.createRadialGradient(half, half, 0, half, half, half)
  for (const [offset, color] of stops) {
    grad.addColorStop(offset, color)
  }
  ctx.fillStyle = grad
  ctx.fillRect(0, 0, size, size)
  return oc
}

// ── Public sprite sheet ──

export interface SpriteSheet {
  /** flame[stageIdx][bucket] — stageIdx 0-4 corresponds to stage 1-5 */
  flame: OffscreenCanvas[][]
  /** smoke[bucket] */
  smoke: OffscreenCanvas[]
  /** Per-stage glow dot outer */
  glowDotOuter: OffscreenCanvas[]
  /** Per-stage glow dot core */
  glowDotCore: OffscreenCanvas[]
  /** Per-stage floor core glow */
  coreGlow: OffscreenCanvas[]
  /** Trajectory particle sprite */
  trajectoryParticle: OffscreenCanvas
  /** Trajectory landing glow sprite */
  trajectoryLanding: OffscreenCanvas
  /** Match impact flash sprite */
  matchFlash: OffscreenCanvas
  /** Explosion core sprite */
  explosionCore: OffscreenCanvas
  /** Explosion fireball sprite */
  explosionFireball: OffscreenCanvas
}

export function createSpriteSheet(stages: StageCfgForSprite[]): SpriteSheet {
  const flame: OffscreenCanvas[][] = []
  const glowDotOuter: OffscreenCanvas[] = []
  const glowDotCore: OffscreenCanvas[] = []
  const coreGlow: OffscreenCanvas[] = []

  for (const cfg of stages) {
    // ── Flame sprites per height bucket ──
    const buckets: OffscreenCanvas[] = []
    for (let bi = 0; bi < FLAME_BUCKETS; bi++) {
      const heightRatio = bi / (FLAME_BUCKETS - 1)
      const [r, g, b] = lerpColor(cfg.colorStops, heightRatio)
      const ri = Math.round(r)
      const gi = Math.round(g)
      const bri = Math.round(b)
      buckets.push(makeRadial(SPRITE_SIZE, [
        [0, `rgba(${ri}, ${gi}, ${bri}, 1)`],
        [0.4, `rgba(${Math.round(r * 0.9)}, ${Math.round(g * 0.7)}, ${Math.round(b * 0.5)}, 0.6)`],
        [1, `rgba(${Math.round(r * 0.5)}, ${Math.round(g * 0.2)}, 0, 0)`],
      ]))
    }
    flame.push(buckets)

    // ── Glow dot outer ──
    glowDotOuter.push(makeRadial(SPRITE_SIZE, [
      [0, cfg.dotColor + '60'],
      [0.5, cfg.dotColor + '20'],
      [1, cfg.dotColor + '00'],
    ]))

    // ── Glow dot core ──
    glowDotCore.push(makeRadial(SPRITE_SIZE, [
      [0, '#ffffcc'],
      [0.4, cfg.dotColor],
      [1, cfg.dotColor + '00'],
    ]))

    // ── Floor core glow ──
    coreGlow.push(makeRadial(SPRITE_SIZE, [
      [0, `rgba(255, 220, 100, ${cfg.coreAlpha})`],
      [0.3, `rgba(255, 150, 30, ${cfg.coreAlpha * 0.6})`],
      [0.7, `rgba(200, 50, 0, ${cfg.coreAlpha * 0.2})`],
      [1, 'rgba(150, 20, 0, 0)'],
    ]))
  }

  // ── Smoke sprites per height bucket ──
  const smoke: OffscreenCanvas[] = []
  for (let b = 0; b < SMOKE_BUCKETS; b++) {
    const heightRatio = b / (SMOKE_BUCKETS - 1)
    const gray = Math.round(30 + heightRatio * 20)
    // Alpha baked at full; actual alpha via globalAlpha at draw time.
    smoke.push(makeRadial(SPRITE_SIZE, [
      [0, `rgba(${gray}, ${gray}, ${gray}, 1)`],
      [0.6, `rgba(${gray}, ${gray}, ${gray}, 0.4)`],
      [1, `rgba(${gray}, ${gray}, ${gray}, 0)`],
    ]))
  }

  // ── Trajectory particle sprite ──
  const trajectoryParticle = makeRadial(SPRITE_SIZE, [
    [0, 'rgba(255, 230, 140, 1)'],
    [0.4, 'rgba(255, 140, 40, 0.85)'],
    [1, 'rgba(180, 40, 0, 0)'],
  ])

  // ── Trajectory landing glow sprite ──
  const trajectoryLanding = makeRadial(SPRITE_SIZE, [
    [0, 'rgba(255, 220, 130, 0.9)'],
    [0.5, 'rgba(255, 120, 40, 0.6)'],
    [1, 'rgba(180, 40, 0, 0)'],
  ])

  // ── Match impact flash sprite ──
  const matchFlash = makeRadial(SPRITE_SIZE, [
    [0, 'rgba(255, 255, 200, 0.9)'],
    [0.3, 'rgba(255, 180, 40, 0.6)'],
    [1, 'rgba(255, 60, 0, 0)'],
  ])

  // ── Explosion core sprite ──
  const explosionCore = makeRadial(SPRITE_SIZE, [
    [0, 'rgba(255, 255, 255, 1)'],
    [0.35, 'rgba(255, 245, 210, 0.95)'],
    [0.7, 'rgba(255, 180, 80, 0.6)'],
    [1, 'rgba(255, 120, 20, 0)'],
  ])

  // ── Explosion fireball sprite ──
  const explosionFireball = makeRadial(SPRITE_SIZE, [
    [0, 'rgba(255, 250, 220, 1)'],
    [0.25, 'rgba(255, 180, 60, 1)'],
    [0.6, 'rgba(230, 80, 20, 0.7)'],
    [1, 'rgba(120, 10, 0, 0)'],
  ])

  return {
    flame,
    smoke,
    glowDotOuter,
    glowDotCore,
    coreGlow,
    trajectoryParticle,
    trajectoryLanding,
    matchFlash,
    explosionCore,
    explosionFireball,
  }
}

export const FLAME_BUCKET_COUNT = FLAME_BUCKETS
export const SMOKE_BUCKET_COUNT = SMOKE_BUCKETS
