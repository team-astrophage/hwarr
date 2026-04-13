/**
 * Canvas 기반 고퀄리티 화염 이펙트
 *
 * 소프트 라디얼 그라디언트 파티클 + additive blending.
 * 바닥: 밝은 노랑/흰 코어, 중간: 주황, 상단: 붉은 혀 → 페이드아웃.
 * 줌 축소 시 글로우 도트로 전환하여 원거리에서도 가시성 확보.
 */

import { useEffect, useRef } from 'react'
import { useMap } from 'react-leaflet'
import L from 'leaflet'
import { useFireStore, type FireCell } from '../stores/fireStore'
import { useAnimationStore } from '../stores/animationStore'
import {
  createSpriteSheet,
  lerpAlpha,
  FLAME_BUCKET_COUNT,
  SMOKE_BUCKET_COUNT,
  type SpriteSheet,
  type StageCfgForSprite,
} from '../utils/fireParticleSprites'
import { getNeighborFireCount, getDensityMultiplier } from '../utils/grid'

// ── 파티클 타입 ──

interface Flame {
  rx: number   // 격자 내 수평 위치 0~1
  ry: number   // 높이 0(바닥)~1(꼭대기)
  vx: number
  vy: number
  life: number
  maxLife: number
  size: number
  /** 높이 비율에 따라 색상 결정 (0=바닥 밝은색, 1=꼭대기 어두운색) */
  seed: number
  active: boolean  // object pooling flag
}

interface Smoke {
  rx: number
  ry: number
  vx: number
  vy: number
  life: number
  maxLife: number
  size: number
  active: boolean  // object pooling flag
}

// ── 단계별 설정 ──
// 불씨(1) → 모닥불(2) → 화재(3) → 대화재(4) → 전소(5)

interface StageCfg {
  particleCount: number
  maxHeight: number       // 불꽃 최대 높이 (격자 높이 대비 비율)
  baseSpeed: number       // 상승 기본 속도
  spreadX: number         // 수평 퍼짐 정도
  turbulence: number      // 흔들림 강도
  baseSize: number        // 파티클 기본 크기 (격자 너비 대비)
  // 높이별 색상 그라디언트 (바닥→꼭대기)
  colorStops: [number, number, number, number][]  // [r, g, b, a]
  coreAlpha: number       // 바닥 코어 밝기
  glowRadius: number      // 바닥 글로우 반경 (격자 너비 대비)
  // 원거리 글로우 도트
  dotColor: string
  dotSize: number         // 최소 px 크기
  dotPulse: number        // 펄스 속도
  // 연기 (4~5단계)
  smokeCount: number
  smokeMaxHeight: number
  smokeAlpha: number
}

const STAGES: (StageCfg | null)[] = [
  null, // 0: 없음
  { // 1: 불씨 — 잔불 (가시성 위해 파티클·글로우 강화)
    particleCount: 26,
    maxHeight: 0.48,
    baseSpeed: 0.0065,
    spreadX: 0.16,
    turbulence: 0.0022,
    baseSize: 0.2,
    colorStops: [
      [255, 210, 90, 0.92],   // 바닥: 밝은 노랑 코어
      [255, 100, 35, 0.65],   // 중간: 주황
      [220, 45, 12, 0.35],    // 상단: 붉은 기운
      [100, 20, 0, 0],
    ],
    coreAlpha: 0.34,
    glowRadius: 0.44,
    dotColor: '#ff7a45',
    dotSize: 9,
    dotPulse: 0.028,
    smokeCount: 0,
    smokeMaxHeight: 0,
    smokeAlpha: 0,
  },
  { // 2: 모닥불 — 따뜻한 불꽃
    particleCount: 22,
    maxHeight: 0.6,
    baseSpeed: 0.007,
    spreadX: 0.18,
    turbulence: 0.003,
    baseSize: 0.2,
    colorStops: [
      [255, 230, 120, 0.95],  // 밝은 노랑
      [255, 140, 25, 0.7],    // 주황
      [200, 50, 0, 0.3],      // 빨강
      [100, 15, 0, 0],
    ],
    coreAlpha: 0.4,
    glowRadius: 0.45,
    dotColor: '#ff8c42',
    dotSize: 9,
    dotPulse: 0.035,
    smokeCount: 0,
    smokeMaxHeight: 0,
    smokeAlpha: 0,
  },
  { // 3: 화재 — 격렬한 불
    particleCount: 45,
    maxHeight: 0.9,
    baseSpeed: 0.01,
    spreadX: 0.28,
    turbulence: 0.004,
    baseSize: 0.26,
    colorStops: [
      [255, 255, 200, 1.0],  // 흰노랑 코어
      [255, 180, 40, 0.9],   // 밝은 주황
      [255, 80, 0, 0.5],     // 주황빨강
      [180, 20, 0, 0],
    ],
    coreAlpha: 0.7,
    glowRadius: 0.6,
    dotColor: '#ff4500',
    dotSize: 14,
    dotPulse: 0.05,
    smokeCount: 0,
    smokeMaxHeight: 0,
    smokeAlpha: 0,
  },
  { // 4: 대화재 — 맹렬한 화염 + 연기
    particleCount: 80,
    maxHeight: 1.4,
    baseSpeed: 0.014,
    spreadX: 0.35,
    turbulence: 0.006,
    baseSize: 0.32,
    colorStops: [
      [255, 255, 245, 1.0],  // 거의 흰색 코어
      [255, 220, 60, 1.0],   // 밝은 노랑
      [255, 100, 0, 0.8],    // 주황
      [220, 30, 0, 0.15],
    ],
    coreAlpha: 0.95,
    glowRadius: 0.9,
    dotColor: '#ff2200',
    dotSize: 18,
    dotPulse: 0.07,
    smokeCount: 12,
    smokeMaxHeight: 2.0,
    smokeAlpha: 0.25,
  },
  { // 5: 지옥불 — 최대 화염 + 짙은 연기
    particleCount: 120,
    maxHeight: 1.8,
    baseSpeed: 0.018,
    spreadX: 0.45,
    turbulence: 0.008,
    baseSize: 0.38,
    colorStops: [
      [255, 255, 255, 1.0],  // 완전 흰색 코어
      [255, 240, 80, 1.0],   // 밝은 노랑
      [255, 60, 0, 0.9],     // 강렬한 빨강
      [180, 0, 0, 0.2],
    ],
    coreAlpha: 1.0,
    glowRadius: 1.2,
    dotColor: '#ff0000',
    dotSize: 22,
    dotPulse: 0.09,
    smokeCount: 25,
    smokeMaxHeight: 3.0,
    smokeAlpha: 0.4,
  },
]

// ── 오브젝트 풀링: 인플레이스 리셋 ──

function resetFlame(f: Flame, cfg: StageCfg): void {
  const life = 40 + Math.random() * 50
  f.rx = 0.3 + Math.random() * 0.4
  f.ry = 0
  f.vx = (Math.random() - 0.5) * cfg.spreadX * 0.02
  f.vy = cfg.baseSpeed * (0.7 + Math.random() * 0.6)
  f.life = life
  f.maxLife = life
  f.size = cfg.baseSize * (0.6 + Math.random() * 0.8)
  f.seed = Math.random()
  f.active = true
}

function resetSmoke(s: Smoke, cfg: StageCfg): void {
  const life = 60 + Math.random() * 80
  s.rx = 0.2 + Math.random() * 0.6
  s.ry = cfg.maxHeight * 0.6 + Math.random() * cfg.maxHeight * 0.3
  s.vx = (Math.random() - 0.5) * 0.015
  s.vy = 0.003 + Math.random() * 0.004
  s.life = life
  s.maxLife = life
  s.size = 0.3 + Math.random() * 0.4
  s.active = true
}

function createFlame(): Flame {
  return { rx: 0, ry: 0, vx: 0, vy: 0, life: 0, maxLife: 1, size: 0, seed: 0, active: false }
}

function createSmoke(): Smoke {
  return { rx: 0, ry: 0, vx: 0, vy: 0, life: 0, maxLife: 1, size: 0, active: false }
}

// ── 폭발 파편 색상 (모듈 상수) ──
const EXPLOSION_DEBRIS_COLORS = ['#ffaa44', '#ff6622', '#cc2200', '#661100']

// ── 줌 임계값 ──
const GLOW_DOT_ZOOM = 15   // 이 줌 미만이면 글로우 도트로 전환

/** 성냥 비행 시간 (ms) */
const MATCH_DURATION = 500

/** Alpha quantization: 20 discrete levels */
const ALPHA_BUCKETS = 20
/** flameCmds flat array stride: [alpha, stageIdx, bucket, px, py, particleR] */
const CMD_STRIDE = 6

// ── 스프라이트 시트 lazy init ──

let spriteSheetCache: SpriteSheet | null = null

function getSpriteSheet(): SpriteSheet {
  if (spriteSheetCache) return spriteSheetCache
  const cfgs: StageCfgForSprite[] = []
  for (let i = 1; i <= 5; i++) {
    const s = STAGES[i]!
    cfgs.push({ colorStops: s.colorStops, coreAlpha: s.coreAlpha, dotColor: s.dotColor })
  }
  spriteSheetCache = createSpriteSheet(cfgs)
  return spriteSheetCache
}

// ── Per-grid precomputed data for 2-pass rendering ──

interface GridRenderData {
  gridId: string
  cfg: StageCfg
  stageIdx: number  // 0-4
  left: number
  top: number
  w: number
  h: number
  cx: number
  cy: number
  densityMul: number
  effectiveFlames: number
  effectiveSmokes: number
}

// ── 메인 컴포넌트 ──

export function FireCanvas() {
  const map = useMap()
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const flamesRef = useRef<Map<string, Flame[]>>(new Map())
  const smokesRef = useRef<Map<string, Smoke[]>>(new Map())
  const animRef = useRef<number>(0)
  const fires = useFireStore((s) => s.fires)
  const firesRef = useRef(fires)
  firesRef.current = fires
  const gridMeta = useFireStore((s) => s.gridMeta)
  const gridMetaRef = useRef(gridMeta)
  gridMetaRef.current = gridMeta

  const matches = useAnimationStore((s) => s.matches)
  const matchesRef = useRef(matches)
  matchesRef.current = matches
  const removeMatch = useAnimationStore((s) => s.removeMatch)
  const removeMatchRef = useRef(removeMatch)
  removeMatchRef.current = removeMatch

  const explosions = useAnimationStore((s) => s.explosions)
  const explosionsRef = useRef(explosions)
  explosionsRef.current = explosions
  const removeExplosion = useAnimationStore((s) => s.removeExplosion)
  const removeExplosionRef = useRef(removeExplosion)
  removeExplosionRef.current = removeExplosion

  const trajectories = useAnimationStore((s) => s.trajectories)
  const trajectoriesRef = useRef(trajectories)
  trajectoriesRef.current = trajectories
  const removeTrajectory = useAnimationStore((s) => s.removeTrajectory)
  const removeTrajectoryRef = useRef(removeTrajectory)
  removeTrajectoryRef.current = removeTrajectory

  const densityCacheRef = useRef<Map<string, number>>(new Map())
  const lastFiresIdRef = useRef<Map<string, FireCell> | null>(null)

  useEffect(() => {
    const container = map.getContainer()
    const canvas = document.createElement('canvas')
    canvas.style.position = 'absolute'
    canvas.style.top = '0'
    canvas.style.left = '0'
    canvas.style.pointerEvents = 'none'
    canvas.style.zIndex = '450'
    container.appendChild(canvas)
    canvasRef.current = canvas

    const resize = () => {
      const size = map.getSize()
      canvas.width = size.x * window.devicePixelRatio
      canvas.height = size.y * window.devicePixelRatio
      canvas.style.width = `${size.x}px`
      canvas.style.height = `${size.y}px`
    }
    resize()
    map.on('resize', resize)
    map.on('zoom', resize)

    let frameCount = 0
    let lastFrameTime = 0
    let fpsMultiplier = 1.0
    // Reusable arrays for alpha-batched flame rendering
    const flameCmds: number[] = []
    const flameCmdIndices: number[] = []
    // Pre-allocated counting sort buffers (ALPHA_BUCKETS + 1 slots)
    const alphaBucketCounts = new Array<number>(ALPHA_BUCKETS + 1).fill(0)
    const alphaBucketCounts2 = new Array<number>(ALPHA_BUCKETS + 1).fill(0)
    const alphaBucketOffsets = new Array<number>(ALPHA_BUCKETS + 1).fill(0)

    const animate = () => {
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      animRef.current = requestAnimationFrame(animate)

      // FPS adaptive throttle
      const frameNow = performance.now()
      if (lastFrameTime > 0) {
        const dt = frameNow - lastFrameTime
        if (dt > 20) {
          fpsMultiplier = Math.max(0.3, fpsMultiplier - 0.04)
        } else if (dt < 18) {
          fpsMultiplier = Math.min(1.0, fpsMultiplier + 0.03)
        }
      }
      lastFrameTime = frameNow

      frameCount++
      const dpr = window.devicePixelRatio
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      ctx.save()
      ctx.scale(dpr, dpr)

      const sprites = getSpriteSheet()
      const zoom = map.getZoom()
      const currentFires = firesRef.current
      const allFlames = flamesRef.current
      const sw = (canvas.width / dpr) | 0
      const sh = (canvas.height / dpr) | 0

      const allSmokes = smokesRef.current

      // Recompute density cache when fires change
      const densityCache = densityCacheRef.current
      if (currentFires !== lastFiresIdRef.current) {
        lastFiresIdRef.current = currentFires
        densityCache.clear()
        for (const gridId of currentFires.keys()) {
          densityCache.set(gridId, getNeighborFireCount(gridId, currentFires))
        }
      }

      // 불 없는 격자의 파티클 제거
      for (const gid of allFlames.keys()) {
        if (!currentFires.has(gid)) allFlames.delete(gid)
      }
      for (const gid of allSmokes.keys()) {
        if (!currentFires.has(gid)) allSmokes.delete(gid)
      }

      // ── Precompute grid render data for 2-pass rendering ──
      const grids: GridRenderData[] = []

      const meta = gridMetaRef.current
      const latSize = meta?.latSize ?? 0.0009
      const lngSize = meta?.lngSize ?? 0.0011
      const halfLat = latSize / 2
      const halfLng = lngSize / 2

      for (const [gridId, cell] of currentFires) {
        const cfg = STAGES[cell.stage] ?? STAGES[1]
        if (!cfg) continue

        // cell.lat/lng is the grid center — derive corners via gridMeta.
        const gLat = cell.lat - halfLat
        const gLng = cell.lng - halfLng

        const tl = map.latLngToContainerPoint(L.latLng(gLat + latSize, gLng))
        const br = map.latLngToContainerPoint(L.latLng(gLat, gLng + lngSize))

        const zoomScale = zoom >= 18 ? 1 : Math.max(1, 1 + (18 - zoom) * 0.5)
        const rawW = Math.abs(br.x - tl.x)
        const rawH = Math.abs(br.y - tl.y)
        const w = rawW * zoomScale
        const h = rawH * zoomScale
        const left = Math.min(tl.x, br.x) - (w - rawW) / 2
        const top = Math.min(tl.y, br.y) - (h - rawH) / 2

        const cx = (left + w / 2) | 0
        const cy = (top + h / 2) | 0

        // 화면 밖이면 스킵 (여유 포함)
        const margin = 30
        if (cx + margin < 0 || cx - margin > sw || cy + margin < 0 || cy - margin > sh) continue

        const neighborCount = densityCache.get(gridId) ?? 0
        const densityMul = getDensityMultiplier(neighborCount) * fpsMultiplier

        grids.push({
          gridId,
          cfg,
          stageIdx: Math.max(0, cell.stage - 1),
          left,
          top,
          w,
          h,
          cx,
          cy,
          densityMul,
          effectiveFlames: Math.ceil(cfg.particleCount * densityMul),
          effectiveSmokes: Math.ceil(cfg.smokeCount * densityMul),
        })
      }

      // ════════════════════════════════════════════
      // PASS 1: source-over (grid lines + smoke)
      // ════════════════════════════════════════════
      ctx.globalCompositeOperation = 'source-over'

      for (const g of grids) {
        const { gridId, cfg, left, top, w, h, effectiveSmokes } = g

        if (zoom >= GLOW_DOT_ZOOM && w >= 3) {
          // ── 톤온톤 격자선 ──
          ctx.globalAlpha = 1
          ctx.strokeStyle = 'rgba(120, 60, 60, 0.25)'
          ctx.lineWidth = zoom >= 15 ? 1.5 : 1
          ctx.strokeRect((left + 0.5) | 0, (top + 0.5) | 0, (w - 1) | 0, (h - 1) | 0)

          // ── 연기 파티클 (4~5단계) — 물리 + 렌더 ──
          if (cfg.smokeCount > 0) {
            if (!allSmokes.has(gridId)) allSmokes.set(gridId, [])
            const smokes = allSmokes.get(gridId)!

            // 오브젝트 풀링: 부족하면 추가, 초과하면 비활성화
            while (smokes.length < effectiveSmokes) {
              const s = createSmoke()
              resetSmoke(s, cfg)
              s.ry = cfg.maxHeight * 0.6 + Math.random() * cfg.smokeMaxHeight * 0.5
              s.life = Math.random() * s.maxLife
              smokes.push(s)
            }
            for (let i = effectiveSmokes; i < smokes.length; i++) {
              smokes[i].active = false
            }

            for (let i = smokes.length - 1; i >= 0; i--) {
              const s = smokes[i]
              if (!s.active) continue

              s.ry += s.vy
              s.rx += s.vx
              s.vx += (Math.random() - 0.5) * 0.002
              s.vx *= 0.99
              s.life--

              if (s.life <= 0 || s.ry > cfg.smokeMaxHeight) {
                resetSmoke(s, cfg)
                continue
              }

              s.rx = Math.max(0, Math.min(1, s.rx))
              const lifeRatio = s.life / s.maxLife
              const heightRatio = Math.min(s.ry / cfg.smokeMaxHeight, 1)

              const px = (left + s.rx * w) | 0
              const py = ((top + h) - s.ry * h) | 0

              const smokeR = (w * s.size * (0.8 + heightRatio * 0.5)) | 0
              if (smokeR < 1) continue

              const fadeIn = Math.min(1, (1 - lifeRatio) * 3)
              const fadeOut = lifeRatio
              const alpha = cfg.smokeAlpha * fadeIn * fadeOut * (1 - heightRatio * 0.5)
              if (alpha < 0.01) continue

              const bucket = Math.min((heightRatio * (SMOKE_BUCKET_COUNT - 1)) | 0, SMOKE_BUCKET_COUNT - 1)
              const d = smokeR * 2
              ctx.globalAlpha = alpha
              ctx.drawImage(sprites.smoke[bucket], px - smokeR, py - smokeR, d, d)
            }
          }
        }
      }

      // ════════════════════════════════════════════
      // PASS 2: lighter (glow dots, core glow, flames, effects)
      // ════════════════════════════════════════════
      ctx.globalCompositeOperation = 'lighter'

      for (const g of grids) {
        const { gridId, cfg, stageIdx, left, top, w, h, cx, cy, effectiveFlames } = g

        // ── 줌 축소: 글로우 도트 ──
        if (zoom < GLOW_DOT_ZOOM) {
          const pulse = Math.sin(frameCount * cfg.dotPulse) * 0.3 + 0.7
          const dotR = (Math.max(cfg.dotSize, 6) * pulse) | 0

          // 외부 글로우
          const outerD = (dotR * 5) | 0
          ctx.globalAlpha = 1
          ctx.drawImage(sprites.glowDotOuter[stageIdx], cx - (outerD >> 1), cy - (outerD >> 1), outerD, outerD)

          // 내부 밝은 코어
          const coreD = dotR * 2
          ctx.globalAlpha = pulse
          ctx.drawImage(sprites.glowDotCore[stageIdx], cx - dotR, cy - dotR, coreD, coreD)

          continue
        }

        // ── 줌 확대: 화염 파티클 ──
        if (w < 3) continue

        if (!allFlames.has(gridId)) allFlames.set(gridId, [])
        const flames = allFlames.get(gridId)!

        // 오브젝트 풀링: 부족하면 추가, 초과하면 비활성화
        while (flames.length < effectiveFlames) {
          const f = createFlame()
          resetFlame(f, cfg)
          f.ry = Math.random() * cfg.maxHeight
          f.life = Math.random() * f.maxLife
          flames.push(f)
        }
        for (let i = effectiveFlames; i < flames.length; i++) {
          flames[i].active = false
        }

        // ── 바닥 코어 글로우 ──
        const coreX = cx
        const coreY = (top + h * 0.85) | 0
        const coreR = (w * cfg.glowRadius) | 0
        const coreDiameter = coreR * 2
        ctx.globalAlpha = 1
        ctx.drawImage(sprites.coreGlow[stageIdx], coreX - coreR, coreY - coreR, coreDiameter, coreDiameter)

        // ── 화염 파티클: 물리 업데이트 + 렌더 커맨드 수집 ──
        for (let i = flames.length - 1; i >= 0; i--) {
          const f = flames[i]
          if (!f.active) continue

          // 물리
          f.ry += f.vy
          f.rx += f.vx
          f.vx += (Math.random() - 0.5) * cfg.turbulence
          f.vx *= 0.98
          f.life--

          if (f.life <= 0 || f.ry > cfg.maxHeight) {
            resetFlame(f, cfg)
            continue
          }

          f.rx = Math.max(0.05, Math.min(0.95, f.rx))

          const heightRatio = f.ry / cfg.maxHeight
          const lifeRatio = f.life / f.maxLife

          const px = (left + f.rx * w) | 0
          const py = ((top + h) - f.ry * h) | 0

          const sizeDecay = (1 - heightRatio * 0.7)
          const particleR = (w * f.size * sizeDecay * 0.5) | 0
          if (particleR < 1) continue

          const ca = lerpAlpha(cfg.colorStops, heightRatio)
          const edgeFadeTop = Math.min(1, (cfg.maxHeight - f.ry) / (cfg.maxHeight * 0.3))
          const edgeFadeBottom = Math.min(1, (f.ry + 0.15) / 0.15)
          const rawAlpha = ca * lifeRatio * sizeDecay * edgeFadeTop * edgeFadeBottom

          if (rawAlpha < 0.01) continue

          const alphaBucket = Math.max(1, Math.min((rawAlpha * ALPHA_BUCKETS + 0.5) | 0, ALPHA_BUCKETS))

          const bucket = Math.min((heightRatio * (FLAME_BUCKET_COUNT - 1)) | 0, FLAME_BUCKET_COUNT - 1)
          flameCmds.push(alphaBucket, stageIdx, bucket, px, py, particleR)
        }
      }

      // ── Batch render flames by alpha bucket (counting sort, O(n)) ──
      const cmdCount = flameCmds.length / CMD_STRIDE
      if (cmdCount > 0) {
        // Counting sort: group commands by alpha bucket (0..ALPHA_BUCKETS)
        const bucketCounts = alphaBucketCounts
        for (let b = 0; b <= ALPHA_BUCKETS; b++) bucketCounts[b] = 0
        for (let i = 0; i < cmdCount; i++) bucketCounts[flameCmds[i * CMD_STRIDE]]++

        // Build offset table
        const bucketOffsets = alphaBucketOffsets
        bucketOffsets[0] = 0
        for (let b = 1; b <= ALPHA_BUCKETS; b++) bucketOffsets[b] = bucketOffsets[b - 1] + bucketCounts[b - 1]

        // Place indices into sorted order
        while (flameCmdIndices.length < cmdCount) flameCmdIndices.push(0)
        flameCmdIndices.length = cmdCount
        const placeCounts = alphaBucketCounts2
        for (let b = 0; b <= ALPHA_BUCKETS; b++) placeCounts[b] = 0
        for (let i = 0; i < cmdCount; i++) {
          const ab = flameCmds[i * CMD_STRIDE]
          flameCmdIndices[bucketOffsets[ab] + placeCounts[ab]] = i
          placeCounts[ab]++
        }

        // Render in alpha-bucket order
        let prevBucket = -1
        for (let ii = 0; ii < cmdCount; ii++) {
          const base = flameCmdIndices[ii] * CMD_STRIDE
          const ab = flameCmds[base]
          const si = flameCmds[base + 1]
          const bucket = flameCmds[base + 2]
          const px = flameCmds[base + 3]
          const py = flameCmds[base + 4]
          const pr = flameCmds[base + 5]
          if (ab !== prevBucket) {
            ctx.globalAlpha = ab / ALPHA_BUCKETS
            prevBucket = ab
          }
          const d = pr * 2
          ctx.drawImage(sprites.flame[si][bucket], px - pr, py - pr, d, d)
        }
      }
      flameCmds.length = 0

      // ── 성냥 던지기 포물선 ──
      const now = performance.now()

      for (const m of matchesRef.current) {
        const elapsed = now - m.startTime
        const progress = Math.min(elapsed / MATCH_DURATION, 1)

        // cell 에서 중심 lat/lng 를 직접 조회. 도착 전 cell 이 없으면 스킵.
        const targetCell = firesRef.current.get(m.gridId)
        if (!targetCell) {
          if (progress >= 1) removeMatchRef.current(m.id)
          continue
        }
        const targetPt = map.latLngToContainerPoint(L.latLng(targetCell.lat, targetCell.lng))

        const startX = sw / 2
        const startY = sh - 80
        const t = progress
        const x = (startX + (targetPt.x - startX) * t) | 0
        const parabola = -4 * t * (t - 1)
        const baseY = startY + (targetPt.y - startY) * t
        const y = (baseY - parabola * 120) | 0

        const rotation = t * Math.PI * 4

        ctx.save()
        ctx.globalAlpha = 1 - t * 0.3
        ctx.globalCompositeOperation = 'source-over'
        ctx.translate(x, y)
        ctx.rotate(rotation)

        ctx.fillStyle = '#8B6914'
        ctx.fillRect(-4, -18, 8, 30)
        ctx.fillStyle = '#ff4444'
        ctx.beginPath()
        ctx.arc(0, -18, 7, 0, Math.PI * 2)
        ctx.fill()

        if (t < 0.8) {
          ctx.fillStyle = '#ffaa00'
          ctx.globalAlpha = 0.8 - t
          ctx.beginPath()
          ctx.moveTo(0, -28)
          ctx.bezierCurveTo(-5, -34, -4, -40, 0, -37)
          ctx.bezierCurveTo(4, -40, 5, -34, 0, -28)
          ctx.fill()
        }

        ctx.restore()

        // 착탄 임팩트 — 성냥이 도착하는 순간 불꽃 스파크
        if (progress >= 1) {
          removeMatchRef.current(m.id)
        }

        const impactT = Math.max(0, (progress - 0.85) / 0.15)
        if (impactT > 0) {
          ctx.save()
          ctx.globalCompositeOperation = 'lighter'
          const impP = Math.min(impactT, 1)

          // 중심 플래시 (sprite)
          const flashR = ((32 + impP * 60) | 0)
          const flashD = flashR * 2
          const tpx = targetPt.x | 0
          const tpy = targetPt.y | 0
          ctx.globalAlpha = 1 - impP
          ctx.drawImage(sprites.matchFlash, tpx - flashR, tpy - flashR, flashD, flashD)

          // 스파크 방사
          const sparkCount = 18
          for (let si = 0; si < sparkCount; si++) {
            const angle = (si / sparkCount) * Math.PI * 2 + impP * 0.5
            const sparkDist = (impP * (45 + ((si * 31) % 17) * 3.5)) | 0
            const spx = (tpx + Math.cos(angle) * sparkDist) | 0
            const spy = (tpy + Math.sin(angle) * sparkDist) | 0
            const sparkSize = ((1 - impP) * (2.5 + (si % 3) * 1.5)) | 0
            ctx.globalAlpha = (1 - impP) * 0.95
            ctx.fillStyle = si % 3 === 0 ? '#ffffff' : si % 3 === 1 ? '#ffdd44' : '#ff6600'
            ctx.beginPath()
            ctx.arc(spx, spy, sparkSize, 0, Math.PI * 2)
            ctx.fill()
          }

          // 불꽃 혀 (짧은 라인)
          ctx.lineWidth = 2
          ctx.lineCap = 'round'
          for (let li = 0; li < 10; li++) {
            const a = (li / 10) * Math.PI * 2
            const len = (impP * (20 + (li * 19) % 25)) | 0
            const ex = (tpx + Math.cos(a) * len) | 0
            const ey = (tpy + Math.sin(a) * len) | 0
            ctx.globalAlpha = (1 - impP) * 0.7
            ctx.strokeStyle = li % 2 === 0 ? '#ffaa00' : '#ff4400'
            ctx.beginPath()
            ctx.moveTo((tpx + Math.cos(a) * 8) | 0, (tpy + Math.sin(a) * 8) | 0)
            ctx.lineTo(ex, ey)
            ctx.stroke()
          }

          ctx.restore()
        }
      }

      // ── 전소 폭발 이펙트 ("빵!" 느낌) ──
      const EXPLOSION_DURATION = 700

      for (const exp of explosionsRef.current) {
        const elapsed = now - exp.startTime
        const progress = Math.min(elapsed / EXPLOSION_DURATION, 1)

        const expCell = firesRef.current.get(exp.gridId)
        if (!expCell) {
          if (progress >= 1) removeExplosionRef.current(exp.id)
          continue
        }
        const ePt = map.latLngToContainerPoint(L.latLng(expCell.lat, expCell.lng))
        const epx = ePt.x | 0
        const epy = ePt.y | 0

        const easeOut = 1 - Math.pow(1 - progress, 3)

        ctx.globalCompositeOperation = 'lighter'

        // ── 핵 화이트 임팩트 (0~80ms, "빵!!" 의 섬광) ──
        if (elapsed < 80) {
          const p = elapsed / 80
          const coreAlpha = 1 - p
          const coreRadius = (20 + p * 140) | 0
          const coreD = coreRadius * 2
          ctx.globalAlpha = coreAlpha
          ctx.drawImage(sprites.explosionCore, epx - coreRadius, epy - coreRadius, coreD, coreD)
        }

        // ── 파이어볼 ──
        if (progress < 0.6) {
          const ballProgress = progress / 0.6
          const ballRadius = (40 + easeOut * 120) | 0
          const ballAlpha = Math.pow(1 - ballProgress, 1.1)
          const ballD = ballRadius * 2
          ctx.globalAlpha = ballAlpha
          ctx.drawImage(sprites.explosionFireball, epx - ballRadius, epy - ballRadius, ballD, ballD)
        }

        // ── 충격파 링 (이중, 빠르게) ──
        if (elapsed < 180) {
          const p1 = elapsed / 180
          const r1 = (p1 * 180) | 0
          ctx.globalAlpha = (1 - p1) * 0.95
          ctx.strokeStyle = '#ffffff'
          ctx.lineWidth = 4 - p1 * 3
          ctx.beginPath()
          ctx.arc(epx, epy, r1, 0, Math.PI * 2)
          ctx.stroke()
        }
        if (elapsed < 280 && elapsed > 60) {
          const p2 = (elapsed - 60) / 220
          const r2 = (p2 * 240) | 0
          ctx.globalAlpha = (1 - p2) * 0.7
          ctx.strokeStyle = '#ffaa44'
          ctx.lineWidth = 3 - p2 * 2.5
          ctx.beginPath()
          ctx.arc(epx, epy, r2, 0, Math.PI * 2)
          ctx.stroke()
        }

        // ── 파편 파티클 ──
        const particleCount = 18
        const maxRadius = 110
        for (let i = 0; i < particleCount; i++) {
          const baseAngle = (i / particleCount) * Math.PI * 2
          const angle = baseAngle + (Math.random() - 0.5) * (Math.PI / 2)
          const speed = 0.4 + Math.random() * 0.9
          const radius = (maxRadius * easeOut * speed) | 0
          const px = (epx + Math.cos(angle) * radius) | 0
          const py = (epy + Math.sin(angle) * radius) | 0
          const decay = Math.pow(1 - progress, 1.4)
          const pSize = (decay * (3 + Math.random() * 5)) | 0
          ctx.fillStyle = EXPLOSION_DEBRIS_COLORS[i % EXPLOSION_DEBRIS_COLORS.length]
          ctx.globalAlpha = decay * 0.7
          ctx.beginPath()
          ctx.arc(px, py, pSize, 0, Math.PI * 2)
          ctx.fill()
        }

        if (progress >= 1) {
          removeExplosionRef.current(exp.id)
        }
      }

      // ── 불 확산 궤적 (500 하드캡 초과 → 이웃 그리드로) ──
      const TRAJECTORY_DURATION = 400
      const TRAJECTORY_PARTICLES = 6
      const TRAJECTORY_ARC = 0.35

      for (const traj of trajectoriesRef.current) {
        const elapsed = now - traj.startTime
        const progress = Math.min(elapsed / TRAJECTORY_DURATION, 1)

        const srcPt = map.latLngToContainerPoint(L.latLng(traj.fromLat, traj.fromLng))
        const dstPt = map.latLngToContainerPoint(L.latLng(traj.toLat, traj.toLng))

        const dx = dstPt.x - srcPt.x
        const dy = dstPt.y - srcPt.y
        const dist = Math.hypot(dx, dy)

        for (let i = 0; i < TRAJECTORY_PARTICLES; i++) {
          const stagger = i * 0.06
          const localP = Math.min(Math.max(progress - stagger, 0), 1)
          if (localP <= 0 || localP >= 1) continue

          const eased = 1 - Math.pow(1 - localP, 2)
          const arcOffset = -4 * TRAJECTORY_ARC * dist * eased * (1 - eased)
          const jitter = (Math.sin(i * 1.7) + Math.cos(i * 2.3)) * 4
          const px = (srcPt.x + dx * eased + jitter) | 0
          const py = (srcPt.y + dy * eased + arcOffset) | 0

          const fade = Math.sin(localP * Math.PI)
          const pSize = ((2 + fade * 3) * 2) | 0
          const pAlpha = 0.85 * fade

          ctx.globalAlpha = pAlpha
          ctx.drawImage(sprites.trajectoryParticle, px - (pSize >> 1), py - (pSize >> 1), pSize, pSize)
        }

        // 도착 지점 작은 파편 폭발 (마지막 25%)
        if (progress > 0.75) {
          const landP = (progress - 0.75) / 0.25
          const landFade = 1 - landP
          const landR = (3 + landP * 10) | 0
          const landD = landR * 2
          const dpx = dstPt.x | 0
          const dpy = dstPt.y | 0
          ctx.globalAlpha = landFade
          ctx.drawImage(sprites.trajectoryLanding, dpx - landR, dpy - landR, landD, landD)
        }

        if (progress >= 1) {
          removeTrajectoryRef.current(traj.id)
        }
      }


      ctx.restore()
    }

    animRef.current = requestAnimationFrame(animate)

    return () => {
      cancelAnimationFrame(animRef.current)
      map.off('resize', resize)
      map.off('zoom', resize)
      if (canvas.parentNode) canvas.parentNode.removeChild(canvas)
    }
  }, [map])

  return null
}
