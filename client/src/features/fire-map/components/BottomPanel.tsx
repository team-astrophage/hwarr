import { useRef, useCallback } from 'react'
import { useFireStore } from '../stores/fireStore'

/** long press 판정 기준 (ms) */
const LONG_PRESS_MS = 500

interface BottomPanelProps {
  gridId: string | null
  onFire: () => void
  onLongPressFire: () => void
  onLongPressEnd: () => void
  disabled: boolean
}

export function BottomPanel({
  gridId,
  onFire,
  onLongPressFire,
  onLongPressEnd,
  disabled,
}: BottomPanelProps) {
  const fires = useFireStore((s) => s.fires)
  const onlineUsers = useFireStore((s) => s.onlineUsers)

  const activeGrids = fires.size

  // tap vs long press 분기
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isLongRef = useRef(false)

  const handlePointerDown = useCallback(() => {
    if (disabled) return
    isLongRef.current = false
    timerRef.current = setTimeout(() => {
      isLongRef.current = true
      onLongPressFire()
    }, LONG_PRESS_MS)
  }, [disabled, onLongPressFire])

  const handlePointerUp = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    if (isLongRef.current) {
      // long press 종료
      onLongPressEnd()
      isLongRef.current = false
    } else {
      // tap → 성냥 던지기
      onFire()
    }
  }, [onFire, onLongPressEnd])

  const handlePointerLeave = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    if (isLongRef.current) {
      onLongPressEnd()
      isLongRef.current = false
    }
  }, [onLongPressEnd])

  return (
    <div className="absolute bottom-0 left-0 right-0 z-[1000] p-4 pb-8">
      <div className="bg-[var(--color-bg-surface)] rounded-[20px] shadow-[var(--shadow-heavy)] p-5">
        {/* Stats row */}
        <div className="flex items-center gap-4 mb-5">
          <div className="flex-1 flex items-center gap-3">
            <div className="w-9 h-9 bg-[var(--color-bg-elevated)] rounded-[10px] flex items-center justify-center">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="var(--color-accent)">
                <path d="M12 23c-3.866 0-7-3.134-7-7 0-3.866 4-9 7-12 3 3 7 8.134 7 12 0 3.866-3.134 7-7 7z" />
              </svg>
            </div>
            <div>
              <p
                className="text-[1.125rem] font-bold text-[var(--color-text-base)] leading-none"
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                {onlineUsers}
              </p>
              <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-0.5">
                방화범
              </p>
            </div>
          </div>

          <div className="w-px h-8 bg-[var(--color-border)]" />

          <div className="flex-1 flex items-center gap-3">
            <div className="w-9 h-9 bg-[var(--color-bg-elevated)] rounded-[10px] flex items-center justify-center">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--color-accent)" strokeWidth="2">
                <rect x="3" y="3" width="18" height="18" rx="2" />
                <path d="M3 9h18M9 3v18" />
              </svg>
            </div>
            <div>
              <p
                className="text-[1.125rem] font-bold text-[var(--color-text-base)] leading-none"
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                {activeGrids}
              </p>
              <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-0.5">
                화재 지역
              </p>
            </div>
          </div>
        </div>

        {/* Fire button — tap: 성냥, long press: 화염방사기 */}
        <button
          disabled={disabled}
          onPointerDown={handlePointerDown}
          onPointerUp={handlePointerUp}
          onPointerLeave={handlePointerLeave}
          onContextMenu={(e) => e.preventDefault()}
          className="flex h-12 w-full select-none touch-none items-center justify-center gap-2 rounded-[14px] bg-gradient-to-b from-[#ff5c3d] via-[#ff3b1f] to-[#c41e12] font-bold uppercase tracking-[2px] text-[0.875rem] text-white shadow-[0_4px_22px_rgba(255,55,30,0.45)] ring-1 ring-inset ring-white/20 transition-[transform,filter] duration-150 active:scale-[0.96] active:brightness-95 disabled:cursor-not-allowed disabled:opacity-35 disabled:shadow-none"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" className="text-[#fff5e6] drop-shadow-[0_0_6px_rgba(255,200,120,0.9)]">
            <path d="M12 23c-3.866 0-7-3.134-7-7 0-3.866 4-9 7-12 3 3 7 8.134 7 12 0 3.866-3.134 7-7 7z" />
          </svg>
          {gridId ? '불 지르기' : '위치 감지 중...'}
        </button>
      </div>
    </div>
  )
}
