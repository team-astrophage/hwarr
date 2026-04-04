import { useFireStore } from '../stores/fireStore'

interface BottomPanelProps {
  gridId: string | null
  onFire: () => void
  disabled: boolean
}

export function BottomPanel({
  gridId,
  onFire,
  disabled,
}: BottomPanelProps) {
  const fires = useFireStore((s) => s.fires)
  const onlineUsers = useFireStore((s) => s.onlineUsers)

  const activeGrids = fires.size

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

        {/* Fire button — no cooldown */}
        <button
          disabled={disabled}
          onClick={onFire}
          className="w-full h-12 bg-[var(--color-accent)] disabled:opacity-30 disabled:cursor-not-allowed rounded-[14px] flex items-center justify-center gap-2 text-[#000000] font-bold text-[0.875rem] uppercase tracking-[2px] transition-[transform] duration-150 active:scale-[0.96]"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="#000000">
            <path d="M12 23c-3.866 0-7-3.134-7-7 0-3.866 4-9 7-12 3 3 7 8.134 7 12 0 3.866-3.134 7-7 7z" />
          </svg>
          {gridId ? '불 지르기' : '위치 감지 중...'}
        </button>
      </div>
    </div>
  )
}
