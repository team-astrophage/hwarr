import { useRef, useCallback, useState, useEffect } from 'react'
import { useFireStore } from '../stores/fireStore'

const STAGE_LABELS: Record<number, { label: string; color: string }> = {
  0: { label: '안전', color: 'var(--color-text-secondary)' },
  1: { label: '1단계 · 불씨', color: 'var(--color-accent)' },
  2: { label: '2단계 · 모닥불', color: 'var(--color-warning)' },
  3: { label: '3단계 · 불기둥', color: '#e4531b' },
  4: { label: '4단계 · 불바다', color: 'var(--color-negative)' },
  5: { label: 'MAX · 불지옥', color: '#ff0040' },
}

interface BottomPanelProps {
  gridId: string | null
  onFire: () => void
  onVisit: () => void
  disabled: boolean
  noLocation?: boolean
}

export function BottomPanel({
  gridId,
  onFire,
  onVisit,
  disabled,
  noLocation = false,
}: BottomPanelProps) {
  const fires = useFireStore((s) => s.fires)
  const activeGrids = useFireStore((s) => s.globalActiveCount)

  const currentCell = gridId ? fires.get(gridId) : undefined
  const stage = currentCell?.stage ?? 0
  const stageInfo = STAGE_LABELS[stage] ?? STAGE_LABELS[0]

  // 단계 변경 감지 → Shake + Glow 이펙트
  const prevStageRef = useRef(stage)
  const [flashing, setFlashing] = useState(false)

  useEffect(() => {
    if (prevStageRef.current !== stage && stage > 0) {
      setFlashing(true)
    }
    prevStageRef.current = stage
  }, [stage])

  // 멀티터치 방지: 첫 번째 primary pointerId만 추적
  const activePointerIdRef = useRef<number | null>(null)

  const handlePointerDown = useCallback((e: React.PointerEvent<HTMLButtonElement>) => {
    e.stopPropagation()
    if (disabled || !e.isPrimary) return
    if (activePointerIdRef.current !== null) return
    activePointerIdRef.current = e.pointerId
    try {
      e.currentTarget.setPointerCapture(e.pointerId)
    } catch {
      // 일부 브라우저에서 실패 가능 — 무시
    }
  }, [disabled])

  const handlePointerUp = useCallback((e: React.PointerEvent<HTMLButtonElement>) => {
    e.stopPropagation()
    if (e.pointerId !== activePointerIdRef.current) return
    activePointerIdRef.current = null
    // 포인터가 버튼 영역 안에서 놓았는지 검증
    const rect = e.currentTarget.getBoundingClientRect()
    if (e.clientX < rect.left || e.clientX > rect.right || e.clientY < rect.top || e.clientY > rect.bottom) return
    onFire()
  }, [onFire])

  const handlePointerCancel = useCallback((e: React.PointerEvent<HTMLButtonElement>) => {
    e.stopPropagation()
    if (e.pointerId !== activePointerIdRef.current) return
    activePointerIdRef.current = null
  }, [])

  return (
    <div
      className="absolute bottom-0 left-0 right-0 z-[1000] p-4 pb-5"
      onPointerDown={(e) => e.stopPropagation()}
      onPointerUp={(e) => e.stopPropagation()}
      onPointerMove={(e) => e.stopPropagation()}
    >
      <div className="bg-[var(--color-bg-surface)] rounded-[20px] shadow-[var(--shadow-heavy)] p-5">
        {/* Stats row */}
        <div className="flex items-center gap-4 mb-4">
          <div
            className="flex-1 flex items-center gap-3"
            style={flashing ? { animation: 'stage-shake 500ms ease-out' } : undefined}
            onAnimationEnd={() => setFlashing(false)}
          >
            <div className="relative w-8 h-8 rounded-[10px] flex items-center justify-center" style={{ backgroundColor: `color-mix(in srgb, ${stageInfo.color} 15%, transparent)` }}>
              {flashing && (
                <div
                  className="absolute inset-[-4px] rounded-[12px] pointer-events-none"
                  style={{
                    boxShadow: `0 0 14px 4px ${stageInfo.color}`,
                    animation: 'stage-glow 700ms ease-out forwards',
                  }}
                />
              )}
              <svg width="14" height="14" viewBox="0 0 24 24" fill={stageInfo.color}>
                <path d="M12 23c-3.866 0-7-3.134-7-7 0-3.866 4-9 7-12 3 3 7 8.134 7 12 0 3.866-3.134 7-7 7z" />
              </svg>
            </div>
            <div>
              <p
                className="text-[0.8125rem] font-bold leading-none"
                style={{ color: stageInfo.color }}
              >
                {stageInfo.label}
              </p>
              <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-0.5">
                현재 위치 화재 단계
              </p>
            </div>
          </div>

          <div className="w-px h-8 bg-[var(--color-border)]" />

          <button
            className="flex-1 flex items-center gap-3 transition-opacity active:opacity-70"
            onClick={onVisit}
            disabled={activeGrids === 0}
          >
            <div className="w-8 h-8 bg-[var(--color-bg-elevated)] rounded-[10px] flex items-center justify-center">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 1 1 18 0z" />
                <circle cx="12" cy="10" r="3" />
              </svg>
            </div>
            <div className="text-left">
              <p
                className="text-[1rem] font-bold text-[var(--color-text-base)] leading-none"
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                {activeGrids}
              </p>
              <p className="text-[0.6875rem] text-[var(--color-accent)] mt-0.5">
                실시간 화재 지역 →
              </p>
            </div>
          </button>
        </div>

        {/* Fire button — tap으로 성냥 던지기 */}
        <button
          disabled={disabled}
          onPointerDown={handlePointerDown}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerCancel}
          onContextMenu={(e) => e.preventDefault()}
          className="flex h-12 w-full select-none touch-none items-center justify-center pl-2 pr-2 text-center no-underline gap-2 rounded-[14px] bg-[#e4531b] font-bold tracking-wide text-[0.8125rem] text-white shadow-[var(--shadow-medium)] transition-[transform,filter] duration-150 active:scale-[0.96] active:brightness-[0.92] disabled:cursor-not-allowed disabled:opacity-35"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden className="shrink-0 opacity-95">
            <path d="M12 23c-3.866 0-7-3.134-7-7 0-3.866 4-9 7-12 3 3 7 8.134 7 12 0 3.866-3.134 7-7 7z" />
          </svg>
          {noLocation ? '위치를 찾을 수 없어요' : gridId ? '실시간 내 위치에 불지르기' : '위치 감지 중...'}
        </button>
      </div>
    </div>
  )
}
