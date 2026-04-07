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
import { useFireStore } from '../stores/fireStore'
import { useAnimationStore } from '../stores/animationStore'
import { LAT_UNIT, LNG_UNIT } from '../../../lib/config'

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
}

interface Smoke {
  rx: number
  ry: number
  vx: number
  vy: number
  life: number
  maxLife: number
  size: number
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

// ── 연기 파티클 생성 ──

function spawnSmoke(cfg: StageCfg): Smoke {
  const life = 60 + Math.random() * 80
  return {
    rx: 0.2 + Math.random() * 0.6,
    ry: cfg.maxHeight * 0.6 + Math.random() * cfg.maxHeight * 0.3, // 불꽃 위에서 시작
    vx: (Math.random() - 0.5) * 0.015,
    vy: 0.003 + Math.random() * 0.004,
    life,
    maxLife: life,
    size: 0.3 + Math.random() * 0.4,
  }
}

// ── 줌 임계값 ──
const GLOW_DOT_ZOOM = 12   // 이 줌 미만이면 글로우 도트로 전환

/** 성냥 비행 시간 (ms) */
const MATCH_DURATION = 500

// ── 파티클 생성 ──

function spawnFlame(cfg: StageCfg): Flame {
  const life = 40 + Math.random() * 50
  return {
    rx: 0.3 + Math.random() * 0.4, // 중앙 근처에서 시작
    ry: 0,
    vx: (Math.random() - 0.5) * cfg.spreadX * 0.02,
    vy: cfg.baseSpeed * (0.7 + Math.random() * 0.6),
    life,
    maxLife: life,
    size: cfg.baseSize * (0.6 + Math.random() * 0.8),
    seed: Math.random(),
  }
}

// ── 높이에 따른 색상 보간 ──

function getFlameColor(cfg: StageCfg, heightRatio: number): [number, number, number, number] {
  const stops = cfg.colorStops
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

    const animate = () => {
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      animRef.current = requestAnimationFrame(animate)

      frameCount++
      const dpr = window.devicePixelRatio
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      ctx.save()
      ctx.scale(dpr, dpr)

      const zoom = map.getZoom()
      const currentFires = firesRef.current
      const allFlames = flamesRef.current
      const sw = canvas.width / dpr
      const sh = canvas.height / dpr

      const allSmokes = smokesRef.current

      // 불 없는 격자의 파티클 제거
      for (const gid of allFlames.keys()) {
        if (!currentFires.has(gid)) allFlames.delete(gid)
      }
      for (const gid of allSmokes.keys()) {
        if (!currentFires.has(gid)) allSmokes.delete(gid)
      }

      for (const [gridId, cell] of currentFires) {
        const cfg = STAGES[cell.stage] ?? STAGES[1]
        if (!cfg) continue

        const [latStr, lngStr] = gridId.split(':')
        const gLat = Number(latStr) * LAT_UNIT
        const gLng = Number(lngStr) * LNG_UNIT

        const tl = map.latLngToContainerPoint(L.latLng(gLat + LAT_UNIT, gLng))
        const br = map.latLngToContainerPoint(L.latLng(gLat, gLng + LNG_UNIT))

        // 줌 축소 시 불꽃이 너무 작아지지 않도록 최소 크기 보정
        const zoomScale = zoom >= 18 ? 1 : Math.max(1, 1 + (18 - zoom) * 0.7)
        const rawW = Math.abs(br.x - tl.x)
        const rawH = Math.abs(br.y - tl.y)
        const w = rawW * zoomScale
        const h = rawH * zoomScale
        const left = Math.min(tl.x, br.x) - (w - rawW) / 2
        const top = Math.min(tl.y, br.y) - (h - rawH) / 2

        const cx = left + w / 2
        const cy = top + h / 2

        // 화면 밖이면 스킵 (여유 포함)
        const margin = 30
        if (cx + margin < 0 || cx - margin > sw || cy + margin < 0 || cy - margin > sh) continue

        // ── 줌 축소: 글로우 도트 ──
        if (zoom < GLOW_DOT_ZOOM) {
          const pulse = Math.sin(frameCount * cfg.dotPulse) * 0.3 + 0.7
          const dotR = Math.max(cfg.dotSize, 6) * pulse

          ctx.globalCompositeOperation = 'lighter'

          // 외부 글로우
          const outerGlow = ctx.createRadialGradient(cx, cy, 0, cx, cy, dotR * 2.5)
          outerGlow.addColorStop(0, cfg.dotColor + '60')
          outerGlow.addColorStop(0.5, cfg.dotColor + '20')
          outerGlow.addColorStop(1, cfg.dotColor + '00')
          ctx.globalAlpha = 1
          ctx.fillStyle = outerGlow
          ctx.beginPath()
          ctx.arc(cx, cy, dotR * 2.5, 0, Math.PI * 2)
          ctx.fill()

          // 내부 밝은 코어
          const coreGlow = ctx.createRadialGradient(cx, cy, 0, cx, cy, dotR)
          coreGlow.addColorStop(0, '#ffffcc')
          coreGlow.addColorStop(0.4, cfg.dotColor)
          coreGlow.addColorStop(1, cfg.dotColor + '00')
          ctx.globalAlpha = pulse
          ctx.fillStyle = coreGlow
          ctx.beginPath()
          ctx.arc(cx, cy, dotR, 0, Math.PI * 2)
          ctx.fill()

          continue
        }

        // ── 줌 확대: 화염 파티클 ──
        if (w < 3) continue

        // ── 톤온톤 격자선 ──
        ctx.save()
        ctx.globalCompositeOperation = 'source-over'
        ctx.strokeStyle = 'rgba(120, 60, 60, 0.25)'
        ctx.lineWidth = zoom >= 15 ? 1.5 : 1
        ctx.strokeRect(left + 0.5, top + 0.5, w - 1, h - 1)
        ctx.restore()

        if (!allFlames.has(gridId)) allFlames.set(gridId, [])
        const flames = allFlames.get(gridId)!

        // 파티클 보충
        while (flames.length < cfg.particleCount) {
          const f = spawnFlame(cfg)
          f.ry = Math.random() * cfg.maxHeight
          f.life = Math.random() * f.maxLife
          flames.push(f)
        }
        // 초과 파티클 제거
        while (flames.length > cfg.particleCount) {
          flames.pop()
        }

        // 클리핑 없이 자연스러운 페이드로 경계 처리
        ctx.save()

        ctx.globalCompositeOperation = 'lighter'

        // ── 바닥 코어 글로우 ── (중심을 격자 바닥보다 살짝 위로)
        const coreX = cx
        const coreY = top + h * 0.85
        const coreR = w * cfg.glowRadius

        const coreGrad = ctx.createRadialGradient(coreX, coreY, 0, coreX, coreY, coreR)
        coreGrad.addColorStop(0, `rgba(255, 220, 100, ${cfg.coreAlpha})`)
        coreGrad.addColorStop(0.3, `rgba(255, 150, 30, ${cfg.coreAlpha * 0.6})`)
        coreGrad.addColorStop(0.7, `rgba(200, 50, 0, ${cfg.coreAlpha * 0.2})`)
        coreGrad.addColorStop(1, 'rgba(150, 20, 0, 0)')
        ctx.globalAlpha = 1
        ctx.fillStyle = coreGrad
        ctx.beginPath()
        ctx.arc(coreX, coreY, coreR, 0, Math.PI * 2)
        ctx.fill()

        // ── 화염 파티클 업데이트 & 렌더 ──
        for (let i = flames.length - 1; i >= 0; i--) {
          const f = flames[i]

          // 물리
          f.ry += f.vy
          f.rx += f.vx
          f.vx += (Math.random() - 0.5) * cfg.turbulence
          // 위로 갈수록 좌우 흔들림 증가
          f.vx *= 0.98
          f.life--

          if (f.life <= 0 || f.ry > cfg.maxHeight) {
            flames[i] = spawnFlame(cfg)
            continue
          }

          // rx 범위 제한
          f.rx = Math.max(0.05, Math.min(0.95, f.rx))

          const heightRatio = f.ry / cfg.maxHeight
          const lifeRatio = f.life / f.maxLife

          // 화면 좌표
          const px = left + f.rx * w
          const py = (top + h) - f.ry * h  // 바닥에서 위로

          // 크기: 바닥에서 크고, 위로 갈수록 작아짐 (역삼각형 형태)
          const sizeDecay = (1 - heightRatio * 0.7)
          const particleR = w * f.size * sizeDecay * 0.5
          if (particleR < 0.5) continue

          // 높이에 따른 색상
          const [cr, cg, cb, ca] = getFlameColor(cfg, heightRatio)
          // 격자 경계 근처에서 자연스럽게 페이드아웃
          const edgeFadeTop = Math.min(1, (cfg.maxHeight - f.ry) / (cfg.maxHeight * 0.3))
          const edgeFadeBottom = Math.min(1, (f.ry + 0.15) / 0.15)
          const alpha = ca * lifeRatio * sizeDecay * edgeFadeTop * edgeFadeBottom

          if (alpha < 0.01) continue

          // 소프트 라디얼 그라디언트 파티클
          const grad = ctx.createRadialGradient(px, py, 0, px, py, particleR)
          grad.addColorStop(0, `rgba(${Math.round(cr)}, ${Math.round(cg)}, ${Math.round(cb)}, ${alpha})`)
          grad.addColorStop(0.4, `rgba(${Math.round(cr * 0.9)}, ${Math.round(cg * 0.7)}, ${Math.round(cb * 0.5)}, ${alpha * 0.6})`)
          grad.addColorStop(1, `rgba(${Math.round(cr * 0.5)}, ${Math.round(cg * 0.2)}, 0, 0)`)

          ctx.globalAlpha = 1
          ctx.fillStyle = grad
          ctx.beginPath()
          ctx.arc(px, py, particleR, 0, Math.PI * 2)
          ctx.fill()
        }

        // ── 연기 파티클 (4~5단계) ──
        if (cfg.smokeCount > 0) {
          if (!allSmokes.has(gridId)) allSmokes.set(gridId, [])
          const smokes = allSmokes.get(gridId)!

          while (smokes.length < cfg.smokeCount) {
            const s = spawnSmoke(cfg)
            s.ry = cfg.maxHeight * 0.6 + Math.random() * cfg.smokeMaxHeight * 0.5
            s.life = Math.random() * s.maxLife
            smokes.push(s)
          }
          while (smokes.length > cfg.smokeCount) smokes.pop()

          ctx.globalCompositeOperation = 'source-over'

          for (let i = smokes.length - 1; i >= 0; i--) {
            const s = smokes[i]
            s.ry += s.vy
            s.rx += s.vx
            s.vx += (Math.random() - 0.5) * 0.002
            s.vx *= 0.99
            s.life--

            if (s.life <= 0 || s.ry > cfg.smokeMaxHeight) {
              smokes[i] = spawnSmoke(cfg)
              continue
            }

            s.rx = Math.max(0, Math.min(1, s.rx))
            const lifeRatio = s.life / s.maxLife
            const heightRatio = Math.min(s.ry / cfg.smokeMaxHeight, 1)

            const px = left + s.rx * w
            const py = (top + h) - s.ry * h

            const smokeR = w * s.size * (0.8 + heightRatio * 0.5)
            if (smokeR < 1) continue

            const fadeIn = Math.min(1, (1 - lifeRatio) * 3)
            const fadeOut = lifeRatio
            const alpha = cfg.smokeAlpha * fadeIn * fadeOut * (1 - heightRatio * 0.5)
            if (alpha < 0.01) continue

            const gray = Math.round(30 + heightRatio * 20)
            const grad = ctx.createRadialGradient(px, py, 0, px, py, smokeR)
            grad.addColorStop(0, `rgba(${gray}, ${gray}, ${gray}, ${alpha})`)
            grad.addColorStop(0.6, `rgba(${gray}, ${gray}, ${gray}, ${alpha * 0.4})`)
            grad.addColorStop(1, `rgba(${gray}, ${gray}, ${gray}, 0)`)

            ctx.globalAlpha = 1
            ctx.fillStyle = grad
            ctx.beginPath()
            ctx.arc(px, py, smokeR, 0, Math.PI * 2)
            ctx.fill()
          }
        }

        ctx.restore() // clip 해제
      }

      // ── 성냥 던지기 포물선 ──
      const now = performance.now()

      for (const m of matchesRef.current) {
        const elapsed = now - m.startTime
        const progress = Math.min(elapsed / MATCH_DURATION, 1)

        const [mLatStr, mLngStr] = m.gridId.split(':')
        const mLat = Number(mLatStr) * LAT_UNIT + LAT_UNIT / 2
        const mLng = Number(mLngStr) * LNG_UNIT + LNG_UNIT / 2
        const targetPt = map.latLngToContainerPoint(L.latLng(mLat, mLng))

        const startX = sw / 2
        const startY = sh - 80
        const t = progress
        const x = startX + (targetPt.x - startX) * t
        const parabola = -4 * t * (t - 1)
        const baseY = startY + (targetPt.y - startY) * t
        const y = baseY - parabola * 120

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

          // 중심 플래시
          const flashR = 32 + impP * 60
          const fg = ctx.createRadialGradient(targetPt.x, targetPt.y, 0, targetPt.x, targetPt.y, flashR)
          fg.addColorStop(0, `rgba(255,255,200,${0.9 * (1 - impP)})`)
          fg.addColorStop(0.3, `rgba(255,180,40,${0.6 * (1 - impP)})`)
          fg.addColorStop(1, 'rgba(255,60,0,0)')
          ctx.fillStyle = fg
          ctx.beginPath()
          ctx.arc(targetPt.x, targetPt.y, flashR, 0, Math.PI * 2)
          ctx.fill()

          // 스파크 방사
          const sparkCount = 18
          for (let si = 0; si < sparkCount; si++) {
            const angle = (si / sparkCount) * Math.PI * 2 + impP * 0.5
            const sparkDist = impP * (45 + ((si * 31) % 17) * 3.5)
            const spx = targetPt.x + Math.cos(angle) * sparkDist
            const spy = targetPt.y + Math.sin(angle) * sparkDist
            const sparkSize = (1 - impP) * (2.5 + (si % 3) * 1.5)
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
            const len = impP * (20 + (li * 19) % 25)
            const ex = targetPt.x + Math.cos(a) * len
            const ey = targetPt.y + Math.sin(a) * len
            ctx.globalAlpha = (1 - impP) * 0.7
            ctx.strokeStyle = li % 2 === 0 ? '#ffaa00' : '#ff4400'
            ctx.beginPath()
            ctx.moveTo(targetPt.x + Math.cos(a) * 8, targetPt.y + Math.sin(a) * 8)
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

        const [eLat, eLng] = exp.gridId.split(':')
        const eCenterLat = Number(eLat) * LAT_UNIT + LAT_UNIT / 2
        const eCenterLng = Number(eLng) * LNG_UNIT + LNG_UNIT / 2
        const ePt = map.latLngToContainerPoint(L.latLng(eCenterLat, eCenterLng))

        // 강한 ease-out (초반 폭발감)
        const easeOut = 1 - Math.pow(1 - progress, 3)

        ctx.globalCompositeOperation = 'lighter'

        // ── 핵 화이트 임팩트 (0~80ms, "빵!!" 의 섬광) ──
        if (elapsed < 80) {
          const p = elapsed / 80
          const coreAlpha = 1 - p
          const coreRadius = 20 + p * 140
          const coreGrad = ctx.createRadialGradient(ePt.x, ePt.y, 0, ePt.x, ePt.y, coreRadius)
          coreGrad.addColorStop(0, `rgba(255, 255, 255, ${coreAlpha})`)
          coreGrad.addColorStop(0.35, `rgba(255, 245, 210, ${coreAlpha * 0.95})`)
          coreGrad.addColorStop(0.7, `rgba(255, 180, 80, ${coreAlpha * 0.6})`)
          coreGrad.addColorStop(1, 'rgba(255, 120, 20, 0)')
          ctx.globalAlpha = 1
          ctx.fillStyle = coreGrad
          ctx.beginPath()
          ctx.arc(ePt.x, ePt.y, coreRadius, 0, Math.PI * 2)
          ctx.fill()
        }

        // ── 파이어볼 (가운데 확 부풀었다 사그라드는 공) ──
        if (progress < 0.6) {
          const ballProgress = progress / 0.6
          const ballRadius = 40 + easeOut * 120
          const ballAlpha = Math.pow(1 - ballProgress, 1.1) * 1.0
          const ballGrad = ctx.createRadialGradient(ePt.x, ePt.y, 0, ePt.x, ePt.y, ballRadius)
          ballGrad.addColorStop(0, `rgba(255, 250, 220, ${ballAlpha})`)
          ballGrad.addColorStop(0.25, `rgba(255, 180, 60, ${ballAlpha})`)
          ballGrad.addColorStop(0.6, `rgba(230, 80, 20, ${ballAlpha * 0.7})`)
          ballGrad.addColorStop(1, 'rgba(120, 10, 0, 0)')
          ctx.globalAlpha = 1
          ctx.fillStyle = ballGrad
          ctx.beginPath()
          ctx.arc(ePt.x, ePt.y, ballRadius, 0, Math.PI * 2)
          ctx.fill()
        }

        // ── 충격파 링 (이중, 빠르게) ──
        if (elapsed < 180) {
          const p1 = elapsed / 180
          const r1 = p1 * 180
          ctx.globalAlpha = (1 - p1) * 0.95
          ctx.strokeStyle = '#ffffff'
          ctx.lineWidth = 4 - p1 * 3
          ctx.beginPath()
          ctx.arc(ePt.x, ePt.y, r1, 0, Math.PI * 2)
          ctx.stroke()
        }
        if (elapsed < 280 && elapsed > 60) {
          const p2 = (elapsed - 60) / 220
          const r2 = p2 * 240
          ctx.globalAlpha = (1 - p2) * 0.7
          ctx.strokeStyle = '#ffaa44'
          ctx.lineWidth = 3 - p2 * 2.5
          ctx.beginPath()
          ctx.arc(ePt.x, ePt.y, r2, 0, Math.PI * 2)
          ctx.stroke()
        }

        // ── 파편 파티클 (적당, 근거리 산발) ──
        const particleCount = 18
        const maxRadius = 110
        const colorChoices = ['#ffaa44', '#ff6622', '#cc2200', '#661100']
        for (let i = 0; i < particleCount; i++) {
          const baseAngle = (i / particleCount) * Math.PI * 2
          const angle = baseAngle + (Math.random() - 0.5) * (Math.PI / 2)
          const speed = 0.4 + Math.random() * 0.9
          const radius = maxRadius * easeOut * speed
          const px = ePt.x + Math.cos(angle) * radius
          const py = ePt.y + Math.sin(angle) * radius
          const decay = Math.pow(1 - progress, 1.4)
          const pSize = decay * (3 + Math.random() * 5)
          ctx.fillStyle = colorChoices[i % colorChoices.length]
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
      const TRAJECTORY_ARC = 0.35  // 포물선 꼭대기 높이 (이동 거리 대비)

      for (const traj of trajectoriesRef.current) {
        const elapsed = now - traj.startTime
        const progress = Math.min(elapsed / TRAJECTORY_DURATION, 1)

        const [fromLatStr, fromLngStr] = traj.fromGridId.split(':')
        const [toLatStr, toLngStr] = traj.toGridId.split(':')
        const fromLat = Number(fromLatStr) * LAT_UNIT + LAT_UNIT / 2
        const fromLng = Number(fromLngStr) * LNG_UNIT + LNG_UNIT / 2
        const toLat = Number(toLatStr) * LAT_UNIT + LAT_UNIT / 2
        const toLng = Number(toLngStr) * LNG_UNIT + LNG_UNIT / 2

        const srcPt = map.latLngToContainerPoint(L.latLng(fromLat, fromLng))
        const dstPt = map.latLngToContainerPoint(L.latLng(toLat, toLng))

        const dx = dstPt.x - srcPt.x
        const dy = dstPt.y - srcPt.y
        const dist = Math.hypot(dx, dy)

        ctx.globalCompositeOperation = 'lighter'

        // 파편 여러 개가 조금씩 시차를 두고 날아감
        for (let i = 0; i < TRAJECTORY_PARTICLES; i++) {
          const stagger = i * 0.06  // 각 파편은 60ms씩 지연
          const localP = Math.min(Math.max(progress - stagger, 0), 1)
          if (localP <= 0 || localP >= 1) continue

          // ease-out으로 자연스럽게 도달
          const eased = 1 - Math.pow(1 - localP, 2)
          // 포물선: y = -4h*t*(1-t) (t=0과 1에서 0, t=0.5에서 -h)
          const arcOffset = -4 * TRAJECTORY_ARC * dist * eased * (1 - eased)
          const jitter = (Math.sin(i * 1.7) + Math.cos(i * 2.3)) * 4
          const px = srcPt.x + dx * eased + jitter
          const py = srcPt.y + dy * eased + arcOffset

          // 크기/알파: 중반에 가장 크고 밝음 → 끝에서 페이드
          const fade = Math.sin(localP * Math.PI)
          const pSize = 2 + fade * 3
          const pAlpha = 0.85 * fade

          // 색: 오렌지→빨강 그라디언트
          const grad = ctx.createRadialGradient(px, py, 0, px, py, pSize * 2)
          grad.addColorStop(0, `rgba(255, 230, 140, ${pAlpha})`)
          grad.addColorStop(0.4, `rgba(255, 140, 40, ${pAlpha * 0.85})`)
          grad.addColorStop(1, 'rgba(180, 40, 0, 0)')
          ctx.globalAlpha = 1
          ctx.fillStyle = grad
          ctx.beginPath()
          ctx.arc(px, py, pSize * 2, 0, Math.PI * 2)
          ctx.fill()
        }

        // 도착 지점 작은 파편 폭발 (마지막 25%)
        if (progress > 0.75) {
          const landP = (progress - 0.75) / 0.25
          const landFade = 1 - landP
          const landR = 3 + landP * 10
          const landGrad = ctx.createRadialGradient(
            dstPt.x, dstPt.y, 0, dstPt.x, dstPt.y, landR,
          )
          landGrad.addColorStop(0, `rgba(255, 220, 130, ${landFade * 0.9})`)
          landGrad.addColorStop(0.5, `rgba(255, 120, 40, ${landFade * 0.6})`)
          landGrad.addColorStop(1, 'rgba(180, 40, 0, 0)')
          ctx.globalAlpha = 1
          ctx.fillStyle = landGrad
          ctx.beginPath()
          ctx.arc(dstPt.x, dstPt.y, landR, 0, Math.PI * 2)
          ctx.fill()
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
