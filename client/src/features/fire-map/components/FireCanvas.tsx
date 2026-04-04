/**
 * Canvas 기반 불 시각 이펙트
 *
 * 줌 레벨에 따른 동작:
 * - 축소 (zoom < 14): 빨간색 격자 채우기 + 빨간 테두리만 표시
 *   → 멀리서 불 강도를 색상 강도로 판단
 * - 확대 (zoom >= 14): 격자 안에 화염 애니메이션 추가
 *   → ctx.clip()으로 격자 영역 밖 절대 안 나감
 *
 * 화염 렌더링:
 * - 격자 바닥에서 위로 타오르는 불꽃 형태
 * - 단계별로 불꽃 밀도/크기/색상 강도 증가
 */

import { useEffect, useRef } from 'react'
import { useMap } from 'react-leaflet'
import L from 'leaflet'
import { useFireStore } from '../stores/fireStore'
import { useAnimationStore } from '../stores/animationStore'
import { GRID_SIZE } from '../../../lib/config'
import { fetchBuildings, type BuildingPolygon } from '../utils/buildings'

// 화염 파티클
interface Flame {
  /** 격자 내 상대 위치 0~1 */
  rx: number
  /** 격자 바닥 기준 높이 0~1 (0=바닥, 1=꼭대기) */
  ry: number
  /** 수평 속도 */
  vx: number
  /** 상승 속도 */
  vy: number
  /** 남은 수명 */
  life: number
  /** 최대 수명 */
  maxLife: number
  /** 불꽃 상대 크기 0~1 */
  size: number
  /** 색상 인덱스 */
  colorIdx: number
}

// 단계별 설정
const STAGE = [
  null, // 0
  {
    // 1: 불씨
    fill: 'rgba(180, 30, 0, 0.12)',
    border: 'rgba(200, 50, 20, 0.5)',
    flameCount: 6,
    maxFlameH: 0.35,
    flameSpeed: 0.004,
    colors: ['#ff6b35', '#ff8c42', '#cc4400'],
    turbulence: 0.001,
    glowAlpha: 0.08,
  },
  {
    // 2: 모닥불
    fill: 'rgba(200, 30, 0, 0.20)',
    border: 'rgba(220, 40, 10, 0.6)',
    flameCount: 14,
    maxFlameH: 0.50,
    flameSpeed: 0.006,
    colors: ['#ff4500', '#ff6b35', '#ff8c42', '#ffaa00'],
    turbulence: 0.0015,
    glowAlpha: 0.12,
  },
  {
    // 3: 화재
    fill: 'rgba(220, 20, 0, 0.30)',
    border: 'rgba(240, 30, 0, 0.7)',
    flameCount: 25,
    maxFlameH: 0.65,
    flameSpeed: 0.008,
    colors: ['#ff2200', '#ff4500', '#ff6b35', '#ffcc00'],
    turbulence: 0.002,
    glowAlpha: 0.18,
  },
  {
    // 4: 대형화재
    fill: 'rgba(240, 10, 0, 0.40)',
    border: 'rgba(255, 20, 0, 0.85)',
    flameCount: 40,
    maxFlameH: 0.80,
    flameSpeed: 0.012,
    colors: ['#ff0000', '#ff2200', '#ff4500', '#ffdd00', '#ffffff'],
    turbulence: 0.003,
    glowAlpha: 0.25,
  },
  {
    // 5: 전소
    fill: 'rgba(255, 0, 0, 0.50)',
    border: 'rgba(255, 0, 0, 1.0)',
    flameCount: 60,
    maxFlameH: 1.0,
    flameSpeed: 0.016,
    colors: ['#ff0000', '#cc0000', '#ff4500', '#ffdd00', '#ffffff'],
    turbulence: 0.004,
    glowAlpha: 0.35,
  },
]

const ANIM_ZOOM_THRESHOLD = 14

/** 성냥 비행 시간 (ms) */
const MATCH_DURATION = 500

// 화염방사기 스트림 파티클
interface StreamParticle {
  x: number
  y: number
  vx: number
  vy: number
  life: number
  maxLife: number
  size: number
  colorIdx: number
}

const STREAM_COLORS = ['#ff4500', '#ff6b35', '#ffaa00', '#ffdd00', '#ffffff']

/** 건물 LOD 줌 임계값 */
const BUILDING_ZOOM_THRESHOLD = 16

function spawnFlame(cfg: (typeof STAGE)[1]): Flame {
  if (!cfg) return {} as Flame
  return {
    rx: Math.random(),
    ry: 0,
    vx: (Math.random() - 0.5) * cfg.turbulence,
    vy: cfg.flameSpeed * (0.6 + Math.random() * 0.8),
    life: 30 + Math.random() * 40,
    maxLife: 30 + Math.random() * 40,
    size: 0.15 + Math.random() * 0.25,
    colorIdx: Math.floor(Math.random() * cfg.colors.length),
  }
}

/** 불꽃 모양 — 아래가 넓고 위가 뾰족한 역물방울 */
function drawFlameShape(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  color: string,
  alpha: number,
) {
  ctx.globalAlpha = alpha
  ctx.fillStyle = color

  ctx.beginPath()
  // 꼭대기 뾰족 점
  ctx.moveTo(x, y - h)
  // 왼쪽 곡선
  ctx.bezierCurveTo(
    x - w * 0.3, y - h * 0.6,
    x - w * 0.5, y - h * 0.1,
    x - w * 0.4, y,
  )
  // 바닥 둥근 부분
  ctx.quadraticCurveTo(x, y + h * 0.15, x + w * 0.4, y)
  // 오른쪽 곡선
  ctx.bezierCurveTo(
    x + w * 0.5, y - h * 0.1,
    x + w * 0.3, y - h * 0.6,
    x, y - h,
  )
  ctx.closePath()
  ctx.fill()
}

export function FireCanvas() {
  const map = useMap()
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const flamesRef = useRef<Map<string, Flame[]>>(new Map())
  const streamRef = useRef<StreamParticle[]>([])
  const buildingsRef = useRef<BuildingPolygon[]>([])
  const buildingFetchKeyRef = useRef('')
  const animRef = useRef<number>(0)
  const fires = useFireStore((s) => s.fires)
  const firesRef = useRef(fires)
  firesRef.current = fires

  // 애니메이션 스토어 (성냥 + 화염방사기)
  const matches = useAnimationStore((s) => s.matches)
  const matchesRef = useRef(matches)
  matchesRef.current = matches
  const removeMatch = useAnimationStore((s) => s.removeMatch)
  const removeMatchRef = useRef(removeMatch)
  removeMatchRef.current = removeMatch

  const flamethrowerActive = useAnimationStore((s) => s.flamethrowerActive)
  const flamethrowerRef = useRef(flamethrowerActive)
  flamethrowerRef.current = flamethrowerActive
  const targetGridId = useAnimationStore((s) => s.targetGridId)
  const targetGridRef = useRef(targetGridId)
  targetGridRef.current = targetGridId

  const explosions = useAnimationStore((s) => s.explosions)
  const explosionsRef = useRef(explosions)
  explosionsRef.current = explosions
  const removeExplosion = useAnimationStore((s) => s.removeExplosion)
  const removeExplosionRef = useRef(removeExplosion)
  removeExplosionRef.current = removeExplosion

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

    // 건물 LOD: 뷰포트 변경 시 건물 데이터 fetch
    const fetchBuildingsForViewport = () => {
      const zoom = map.getZoom()
      if (zoom < BUILDING_ZOOM_THRESHOLD) {
        buildingsRef.current = []
        buildingFetchKeyRef.current = ''
        return
      }
      const bounds = map.getBounds()
      const key = `${bounds.getSouth().toFixed(3)},${bounds.getWest().toFixed(3)},${bounds.getNorth().toFixed(3)},${bounds.getEast().toFixed(3)}`
      if (key !== buildingFetchKeyRef.current) {
        buildingFetchKeyRef.current = key
        fetchBuildings(
          bounds.getSouth(), bounds.getWest(),
          bounds.getNorth(), bounds.getEast(),
        ).then((b) => { buildingsRef.current = b })
      }
    }
    // 즉시 + 이동/줌 시 fetch
    fetchBuildingsForViewport()
    map.on('moveend', fetchBuildingsForViewport)
    map.on('zoomend', fetchBuildingsForViewport)

    let time = 0

    // 건물 폴리곤 경로를 ctx에 추가하는 헬퍼
    const traceBuildingPath = (
      ctx: CanvasRenderingContext2D,
      bldg: BuildingPolygon,
    ) => {
      for (let ci = 0; ci < bldg.coords.length; ci++) {
        const pt = map.latLngToContainerPoint(
          L.latLng(bldg.coords[ci][0], bldg.coords[ci][1]),
        )
        if (ci === 0) ctx.moveTo(pt.x, pt.y)
        else ctx.lineTo(pt.x, pt.y)
      }
      ctx.closePath()
    }

    // 격자에 겹치는 건물들 찾기 (AABB 바운딩 박스 교차)
    const findOverlappingBuildings = (
      gLat0: number, gLng0: number, gLat1: number, gLng1: number,
    ): BuildingPolygon[] => {
      const result: BuildingPolygon[] = []
      for (const bldg of buildingsRef.current) {
        // 건물 바운딩 박스 계산
        let minLat = Infinity, maxLat = -Infinity
        let minLng = Infinity, maxLng = -Infinity
        for (const [bLat, bLng] of bldg.coords) {
          if (bLat < minLat) minLat = bLat
          if (bLat > maxLat) maxLat = bLat
          if (bLng < minLng) minLng = bLng
          if (bLng > maxLng) maxLng = bLng
        }
        // AABB 교차: 건물 bbox와 격자가 겹치면 매칭
        if (maxLat >= gLat0 && minLat <= gLat1 && maxLng >= gLng0 && minLng <= gLng1) {
          result.push(bldg)
        }
      }
      return result
    }

    const animate = () => {
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      animRef.current = requestAnimationFrame(animate)

      time++
      const dpr = window.devicePixelRatio
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      ctx.save()
      ctx.scale(dpr, dpr)

      const zoom = map.getZoom()
      const showAnim = zoom >= ANIM_ZOOM_THRESHOLD
      const showBuildings = zoom >= BUILDING_ZOOM_THRESHOLD && buildingsRef.current.length > 0
      const currentFires = firesRef.current
      const allFlames = flamesRef.current

      // 불 없는 격자의 파티클 제거
      for (const gid of allFlames.keys()) {
        if (!currentFires.has(gid)) allFlames.delete(gid)
      }

      for (const [gridId, cell] of currentFires) {
        const cfg = STAGE[cell.stage] ?? STAGE[1]
        if (!cfg) continue

        // 격자 → 픽셀 좌표
        const [latStr, lngStr] = gridId.split(':')
        const gLat = Number(latStr) * GRID_SIZE
        const gLng = Number(lngStr) * GRID_SIZE

        const tl = map.latLngToContainerPoint(L.latLng(gLat + GRID_SIZE, gLng))
        const br = map.latLngToContainerPoint(L.latLng(gLat, gLng + GRID_SIZE))

        const left = Math.min(tl.x, br.x)
        const top = Math.min(tl.y, br.y)
        const w = Math.abs(br.x - tl.x)
        const h = Math.abs(br.y - tl.y)

        // 화면 밖이면 스킵
        const sw = canvas.width / dpr
        const sh = canvas.height / dpr
        if (left + w < 0 || left > sw || top + h < 0 || top > sh) continue

        // 건물 모드: 이 격자에 겹치는 건물 찾기
        const gLat0 = Number(latStr) * GRID_SIZE
        const gLng0 = Number(lngStr) * GRID_SIZE
        const overlapping = showBuildings
          ? findOverlappingBuildings(gLat0, gLng0, gLat0 + GRID_SIZE, gLng0 + GRID_SIZE)
          : []
        const useBuildingMode = overlapping.length > 0

        // ── 배경 채우기 + 테두리 ──
        ctx.globalAlpha = 1
        ctx.globalCompositeOperation = 'source-over'

        if (useBuildingMode) {
          // 건물 윤곽선만 빨갛게 + 내부 채우기 없음 (지도 타일이 보이게)
          for (const bldg of overlapping) {
            ctx.beginPath()
            traceBuildingPath(ctx, bldg)
            // 건물 내부: 아주 약한 빨간 틴트만
            ctx.fillStyle = `rgba(255, 30, 0, ${0.08 * cell.stage})`
            ctx.fill()
            // 건물 윤곽: 밝은 빨간 테두리로 "불타는 건물" 강조
            ctx.strokeStyle = cfg.border
            ctx.lineWidth = 2.5
            ctx.stroke()
          }
        } else {
          // 기존 격자 사각형
          ctx.fillStyle = cfg.fill
          ctx.fillRect(left, top, w, h)
          ctx.strokeStyle = cfg.border
          ctx.lineWidth = zoom >= 14 ? 2 : 1
          ctx.strokeRect(left + 0.5, top + 0.5, w - 1, h - 1)
        }

        // ── 확대 시에만: 화염 애니메이션 ──
        if (showAnim && w > 8) {
          // 화염 파티클 관리
          if (!allFlames.has(gridId)) allFlames.set(gridId, [])
          const flames = allFlames.get(gridId)!

          // 부족한 파티클 보충
          while (flames.length < cfg.flameCount) {
            const f = spawnFlame(cfg)
            f.ry = Math.random() * cfg.maxFlameH
            f.life = Math.random() * f.maxLife
            flames.push(f)
          }

          // clip 영역: 건물 or 격자
          ctx.save()
          ctx.beginPath()
          if (useBuildingMode) {
            for (const bldg of overlapping) {
              traceBuildingPath(ctx, bldg)
            }
          } else {
            ctx.rect(left, top, w, h)
          }
          ctx.clip()

          // 하단 글로우 (바닥에서 불이 타는 느낌)
          const glowGrad = ctx.createLinearGradient(left, top + h, left, top + h * 0.3)
          glowGrad.addColorStop(0, `rgba(255, 60, 0, ${cfg.glowAlpha})`)
          glowGrad.addColorStop(0.5, `rgba(255, 30, 0, ${cfg.glowAlpha * 0.4})`)
          glowGrad.addColorStop(1, 'rgba(255, 0, 0, 0)')
          ctx.globalCompositeOperation = 'lighter'
          ctx.fillStyle = glowGrad
          ctx.fillRect(left, top, w, h)

          // 화염 파티클 업데이트 & 렌더
          for (let i = flames.length - 1; i >= 0; i--) {
            const f = flames[i]

            // 물리 업데이트
            f.ry += f.vy
            f.rx += f.vx
            f.vx += (Math.random() - 0.5) * cfg.turbulence * 2 // 흔들림
            f.life--

            // 수명 종료 또는 격자 상단 초과 → 재생성
            if (f.life <= 0 || f.ry > cfg.maxFlameH) {
              flames[i] = spawnFlame(cfg)
              continue
            }

            // rx는 0~1 범위 순환
            if (f.rx < 0) f.rx += 1
            if (f.rx > 1) f.rx -= 1

            const lifeRatio = f.life / f.maxLife

            // 화면 좌표 계산
            // ry: 0=바닥, maxFlameH=최대 높이
            const px = left + f.rx * w
            const py = top + h - (f.ry / cfg.maxFlameH) * h * cfg.maxFlameH

            // 불꽃 크기: 바닥에서 클수록 크고, 올라갈수록 작아짐
            const sizeScale = (1 - f.ry / cfg.maxFlameH) * 0.7 + 0.3
            const flameW = w * f.size * sizeScale * 0.3
            const flameH = h * f.size * sizeScale * 0.5

            if (flameW < 1 || flameH < 1) continue

            const color = cfg.colors[f.colorIdx]
            const alpha = lifeRatio * (0.6 + sizeScale * 0.4)

            // 바닥 근처 불꽃: 밝은 색(노란/흰), 위쪽: 어두운 색(빨강)
            ctx.globalCompositeOperation = 'lighter'
            drawFlameShape(ctx, px, py, flameW, flameH, color, alpha)

            // 바닥 근처에 추가 글로우
            if (f.ry < cfg.maxFlameH * 0.3) {
              const glowR = flameW * 1.5
              const glow = ctx.createRadialGradient(px, py, 0, px, py, glowR)
              glow.addColorStop(0, `rgba(255, 150, 0, ${alpha * 0.3})`)
              glow.addColorStop(1, 'rgba(255, 50, 0, 0)')
              ctx.globalAlpha = alpha * 0.5
              ctx.fillStyle = glow
              ctx.beginPath()
              ctx.arc(px, py, glowR, 0, Math.PI * 2)
              ctx.fill()
            }
          }

          ctx.restore() // clip 해제
        }
      }

      // ── 성냥 던지기 포물선 애니메이션 ──
      const now = performance.now()
      const sw = canvas.width / dpr
      const sh = canvas.height / dpr

      for (const m of matchesRef.current) {
        const elapsed = now - m.startTime
        const progress = Math.min(elapsed / MATCH_DURATION, 1)

        // 대상 격자 중심 좌표 계산
        const [mLatStr, mLngStr] = m.gridId.split(':')
        const mLat = Number(mLatStr) * GRID_SIZE + GRID_SIZE / 2
        const mLng = Number(mLngStr) * GRID_SIZE + GRID_SIZE / 2
        const targetPt = map.latLngToContainerPoint(L.latLng(mLat, mLng))

        // 시작점: 화면 하단 중앙 (버튼 위치)
        const startX = sw / 2
        const startY = sh - 80

        // 포물선 보간 (위로 볼록)
        const t = progress
        const x = startX + (targetPt.x - startX) * t
        const parabola = -4 * t * (t - 1) // 0→1→0 포물선
        const baseY = startY + (targetPt.y - startY) * t
        const y = baseY - parabola * 120 // 포물선 높이

        // 성냥 회전 (2바퀴)
        const rotation = t * Math.PI * 4

        ctx.save()
        ctx.globalAlpha = 1 - t * 0.3 // 도달 시 약간 투명
        ctx.globalCompositeOperation = 'source-over'
        ctx.translate(x, y)
        ctx.rotate(rotation)

        // 성냥 몸체
        ctx.fillStyle = '#8B6914'
        ctx.fillRect(-2, -10, 4, 16)

        // 성냥 머리 (빨간색 + 불꽃)
        ctx.fillStyle = '#ff4444'
        ctx.beginPath()
        ctx.arc(0, -10, 4, 0, Math.PI * 2)
        ctx.fill()

        // 불꽃 이펙트
        if (t < 0.8) {
          ctx.fillStyle = '#ffaa00'
          ctx.globalAlpha = 0.8 - t
          ctx.beginPath()
          ctx.moveTo(0, -16)
          ctx.bezierCurveTo(-3, -20, -2, -24, 0, -22)
          ctx.bezierCurveTo(2, -24, 3, -20, 0, -16)
          ctx.fill()
        }

        ctx.restore()

        // 완료된 성냥 제거
        if (progress >= 1) {
          removeMatchRef.current(m.id)
        }
      }

      // ── 화염방사기 스트림 파티클 ──
      if (flamethrowerRef.current && targetGridRef.current) {
        const gid = targetGridRef.current
        const [fLatStr, fLngStr] = gid.split(':')
        const fLat = Number(fLatStr) * GRID_SIZE + GRID_SIZE / 2
        const fLng = Number(fLngStr) * GRID_SIZE + GRID_SIZE / 2
        const targetPt = map.latLngToContainerPoint(L.latLng(fLat, fLng))

        const srcX = sw / 2
        const srcY = sh - 80

        // 새 스트림 파티클 생성 (프레임당 3개)
        for (let i = 0; i < 3; i++) {
          const dx = targetPt.x - srcX
          const dy = targetPt.y - srcY
          const dist = Math.sqrt(dx * dx + dy * dy) || 1
          const speed = 8 + Math.random() * 4
          streamRef.current.push({
            x: srcX + (Math.random() - 0.5) * 10,
            y: srcY + (Math.random() - 0.5) * 6,
            vx: (dx / dist) * speed + (Math.random() - 0.5) * 2,
            vy: (dy / dist) * speed + (Math.random() - 0.5) * 2,
            life: 20 + Math.random() * 15,
            maxLife: 20 + Math.random() * 15,
            size: 3 + Math.random() * 4,
            colorIdx: Math.floor(Math.random() * STREAM_COLORS.length),
          })
        }
      }

      // 스트림 파티클 업데이트 & 렌더
      const stream = streamRef.current
      for (let i = stream.length - 1; i >= 0; i--) {
        const p = stream[i]
        p.x += p.vx
        p.y += p.vy
        p.life--

        if (p.life <= 0) {
          stream.splice(i, 1)
          continue
        }

        const lifeRatio = p.life / p.maxLife
        ctx.globalAlpha = lifeRatio * 0.9
        ctx.globalCompositeOperation = 'lighter'
        ctx.fillStyle = STREAM_COLORS[p.colorIdx]
        ctx.beginPath()
        ctx.arc(p.x, p.y, p.size * lifeRatio, 0, Math.PI * 2)
        ctx.fill()
      }

      // ── 전소 폭발 이펙트 (5단계) ──
      const EXPLOSION_DURATION = 2000

      for (const exp of explosionsRef.current) {
        const elapsed = now - exp.startTime
        const progress = Math.min(elapsed / EXPLOSION_DURATION, 1)

        // 폭발 중심 좌표
        const [eLat, eLng] = exp.gridId.split(':')
        const eCenterLat = Number(eLat) * GRID_SIZE + GRID_SIZE / 2
        const eCenterLng = Number(eLng) * GRID_SIZE + GRID_SIZE / 2
        const ePt = map.latLngToContainerPoint(L.latLng(eCenterLat, eCenterLng))

        ctx.globalCompositeOperation = 'lighter'

        // 방사형 파티클 (원형으로 퍼져나감)
        const particleCount = 24
        for (let i = 0; i < particleCount; i++) {
          const angle = (i / particleCount) * Math.PI * 2
          const maxRadius = 120 * progress
          const radius = maxRadius * (0.5 + Math.random() * 0.5)
          const px = ePt.x + Math.cos(angle) * radius
          const py = ePt.y + Math.sin(angle) * radius
          const pSize = (1 - progress) * (4 + Math.random() * 6)

          const colorChoices = ['#ff0000', '#ff4500', '#ffaa00', '#ffdd00', '#ffffff']
          ctx.fillStyle = colorChoices[i % colorChoices.length]
          ctx.globalAlpha = (1 - progress) * 0.8
          ctx.beginPath()
          ctx.arc(px, py, pSize, 0, Math.PI * 2)
          ctx.fill()
        }

        // 중앙 플래시 글로우
        if (progress < 0.5) {
          const flashAlpha = (1 - progress * 2) * 0.4
          const flashRadius = 80 + progress * 200
          const flashGrad = ctx.createRadialGradient(ePt.x, ePt.y, 0, ePt.x, ePt.y, flashRadius)
          flashGrad.addColorStop(0, `rgba(255, 255, 200, ${flashAlpha})`)
          flashGrad.addColorStop(0.4, `rgba(255, 100, 0, ${flashAlpha * 0.5})`)
          flashGrad.addColorStop(1, 'rgba(255, 0, 0, 0)')
          ctx.globalAlpha = 1
          ctx.fillStyle = flashGrad
          ctx.beginPath()
          ctx.arc(ePt.x, ePt.y, flashRadius, 0, Math.PI * 2)
          ctx.fill()
        }

        // 화면 전체 플래시 (처음 0.3초)
        if (progress < 0.15) {
          ctx.globalCompositeOperation = 'source-over'
          ctx.globalAlpha = (1 - progress / 0.15) * 0.25
          ctx.fillStyle = '#ffffff'
          ctx.fillRect(0, 0, sw, sh)
        }

        if (progress >= 1) {
          removeExplosionRef.current(exp.id)
        }
      }

      ctx.restore()
    }

    animRef.current = requestAnimationFrame(animate)

    return () => {
      cancelAnimationFrame(animRef.current)
      map.off('resize', resize)
      map.off('zoom', resize)
      map.off('moveend', fetchBuildingsForViewport)
      map.off('zoomend', fetchBuildingsForViewport)
      if (canvas.parentNode) canvas.parentNode.removeChild(canvas)
    }
  }, [map])

  return null
}
