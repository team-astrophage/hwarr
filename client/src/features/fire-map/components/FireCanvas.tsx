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
import { GRID_SIZE } from '../../../lib/config'

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
  const animRef = useRef<number>(0)
  const fires = useFireStore((s) => s.fires)
  const firesRef = useRef(fires)
  firesRef.current = fires

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

    let time = 0

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

        // ── 항상 그리는 것: 채우기 + 테두리 ──
        ctx.globalAlpha = 1
        ctx.globalCompositeOperation = 'source-over'

        // 격자 배경 채우기
        ctx.fillStyle = cfg.fill
        ctx.fillRect(left, top, w, h)

        // 빨간 테두리
        ctx.strokeStyle = cfg.border
        ctx.lineWidth = zoom >= 14 ? 2 : 1
        ctx.strokeRect(left + 0.5, top + 0.5, w - 1, h - 1)

        // ── 확대 시에만: 화염 애니메이션 ──
        if (showAnim && w > 8) {
          // 화염 파티클 관리
          if (!allFlames.has(gridId)) allFlames.set(gridId, [])
          const flames = allFlames.get(gridId)!

          // 부족한 파티클 보충
          while (flames.length < cfg.flameCount) {
            const f = spawnFlame(cfg)
            // 첫 생성 시 랜덤 높이로 흩뿌림 (시작부터 자연스럽게)
            f.ry = Math.random() * cfg.maxFlameH
            f.life = Math.random() * f.maxLife
            flames.push(f)
          }

          // 격자 영역으로 clip — 불꽃이 절대 밖으로 안 나감
          ctx.save()
          ctx.beginPath()
          ctx.rect(left, top, w, h)
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
