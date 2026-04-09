import { useEffect, useCallback } from 'react'
import { Link, useRouterState } from '@tanstack/react-router'
import { useStats } from '../features/landing/api/useStats'

interface MenuDrawerProps {
  open: boolean
  onClose: () => void
}

const NAV_ITEMS = [
  {
    to: '/map' as const,
    label: '실시간 지도',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <polygon points="1 6 1 22 8 18 16 22 23 18 23 2 16 6 8 2 1 6" />
        <line x1="8" y1="2" x2="8" y2="18" />
        <line x1="16" y1="6" x2="16" y2="22" />
      </svg>
    ),
  },
  {
    to: '/' as const,
    label: '상황판',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="7" height="7" rx="1" />
        <rect x="14" y="3" width="7" height="7" rx="1" />
        <rect x="3" y="14" width="7" height="7" rx="1" />
        <rect x="14" y="14" width="7" height="7" rx="1" />
      </svg>
    ),
  },
  {
    to: '/guide' as const,
    label: '이용안내',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="10" />
        <path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" />
        <line x1="12" y1="17" x2="12.01" y2="17" />
      </svg>
    ),
  },
  {
    to: '/archive' as const,
    label: '아카이브',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <line x1="18" y1="20" x2="18" y2="10" />
        <line x1="12" y1="20" x2="12" y2="4" />
        <line x1="6" y1="20" x2="6" y2="14" />
      </svg>
    ),
  },
]

function formatNumber(n: number): string {
  if (n >= 10000) return `${(n / 10000).toFixed(1)}만`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}천`
  return n.toLocaleString()
}

export function MenuDrawer({ open, onClose }: MenuDrawerProps) {
  const { location } = useRouterState()
  const { data: stats } = useStats()

  // Close on route change
  useEffect(() => {
    if (open) onClose()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname])

  // Close on ESC
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    },
    [onClose],
  )

  useEffect(() => {
    if (open) {
      document.addEventListener('keydown', handleKeyDown)
      return () => document.removeEventListener('keydown', handleKeyDown)
    }
  }, [open, handleKeyDown])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-[1100]" role="dialog" aria-modal="true" aria-label="내비게이션 메뉴">
      {/* Overlay */}
      <div
        className="absolute inset-0 bg-black/50 animate-[drawer-overlay-in_200ms_ease-out]"
        onClick={onClose}
      />

      {/* Panel */}
      <aside className="absolute top-0 left-0 h-full w-[72vw] max-w-[280px] bg-[var(--color-bg-surface)] shadow-[var(--shadow-heavy)] flex flex-col animate-[drawer-slide-in_200ms_ease-out]">
        {/* Header */}
        <div className="flex items-center justify-between px-5 pt-5 pb-4">
          <div>
            <p className="text-[1.125rem] font-extrabold text-[var(--color-text-base)] leading-none">화르르</p>
            <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-1">실시간 가상 불 지도</p>
          </div>
          <button
            onClick={onClose}
            className="flex items-center justify-center w-8 h-8 rounded-[8px] transition-colors hover:bg-[var(--color-bg-elevated)]"
            aria-label="메뉴 닫기"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-secondary)" strokeWidth="2" strokeLinecap="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        {/* Divider */}
        <div className="mx-5 h-px bg-[var(--color-bg-elevated)]" />

        {/* Navigation */}
        <nav className="flex-1 px-3 py-3 space-y-1">
          {NAV_ITEMS.map(({ to, label, icon }) => {
            const isActive = location.pathname === to
            return (
              <Link
                key={to}
                to={to}
                className={`flex items-center gap-3 px-3 py-2.5 rounded-[10px] transition-colors text-[0.875rem] ${
                  isActive
                    ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text-base)] font-bold'
                    : 'text-[var(--color-text-secondary)] font-medium hover:bg-[var(--color-bg-elevated)]'
                }`}
              >
                {icon}
                {label}
              </Link>
            )
          })}
        </nav>

        {/* Mini Stats Card */}
        {stats && (
          <>
            <div className="mx-5 h-px bg-[var(--color-bg-elevated)]" />
            <div className="px-5 py-4">
              <p className="text-[0.6875rem] font-bold text-[var(--color-text-secondary)] mb-2 tracking-wide">실시간 상황판</p>
              <div className="grid grid-cols-2 gap-2">
                <div className="bg-[var(--color-bg-elevated)] rounded-[8px] px-3 py-2">
                  <p className="text-[0.625rem] text-[var(--color-text-secondary)]">화재 구역</p>
                  <p className="text-[0.9375rem] font-bold text-[var(--color-accent)]" style={{ fontVariantNumeric: 'tabular-nums' }}>
                    {formatNumber(stats.activeGrids)}
                  </p>
                </div>
                <div className="bg-[var(--color-bg-elevated)] rounded-[8px] px-3 py-2">
                  <p className="text-[0.625rem] text-[var(--color-text-secondary)]">오늘 방화</p>
                  <p className="text-[0.9375rem] font-bold text-[var(--color-warning)]" style={{ fontVariantNumeric: 'tabular-nums' }}>
                    {formatNumber(stats.dailyFires)}
                  </p>
                </div>
              </div>
              <Link
                to="/"
                className="mt-2 flex items-center justify-center gap-1 text-[0.6875rem] font-medium text-[var(--color-accent)] transition-colors hover:text-[var(--color-text-base)]"
              >
                상황판 보기
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="9 18 15 12 9 6" />
                </svg>
              </Link>
            </div>
          </>
        )}

        {/* Footer */}
        <div className="px-5 py-3 border-t border-[var(--color-bg-elevated)]">
          <p className="text-[0.625rem] text-[var(--color-text-secondary)]">화르르 v1.0</p>
        </div>
      </aside>
    </div>
  )
}
