import { useEffect, useCallback } from 'react'
import { Link, useRouterState } from '@tanstack/react-router'
import { useStats } from '../features/landing/api/useStats'

interface MenuPopoverProps {
  open: boolean
  onClose: () => void
}

const NAV_ITEMS = [
  {
    to: '/map' as const,
    label: '실시간 지도',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
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
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
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
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
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
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
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

export function MenuDrawer({ open, onClose }: MenuPopoverProps) {
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
    <>
      {/* Invisible overlay to catch outside clicks */}
      <div className="fixed inset-0 z-[1100]" onClick={onClose} />

      {/* Popover panel — anchored top-right below header */}
      <div className="absolute top-11 right-3 z-[1100] w-[220px] bg-[var(--color-bg-surface)] rounded-[14px] shadow-[var(--shadow-heavy)] border border-[var(--color-bg-elevated)] overflow-hidden animate-[menu-pop-in_150ms_ease-out]">
        {/* Navigation */}
        <nav className="px-2 py-2 space-y-0.5">
          {NAV_ITEMS.map(({ to, label, icon }) => {
            const isActive = location.pathname === to
            return (
              <Link
                key={to}
                to={to}
                className={`flex items-center gap-2.5 px-3 py-2.5 rounded-[10px] transition-colors text-[0.8125rem] ${
                  isActive
                    ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text-base)] font-bold'
                    : 'text-[var(--color-text-secondary)] font-medium active:bg-[var(--color-bg-elevated)]'
                }`}
              >
                {icon}
                {label}
              </Link>
            )
          })}
        </nav>

        {/* Mini Stats */}
        {stats && (
          <>
            <div className="mx-3 h-px bg-[var(--color-bg-elevated)]" />
            <Link to="/" className="block px-3 py-3">
              <div className="flex items-center gap-3">
                <div className="flex-1">
                  <p className="text-[0.625rem] text-[var(--color-text-secondary)]">화재 구역</p>
                  <p className="text-[0.8125rem] font-bold text-[var(--color-accent)]" style={{ fontVariantNumeric: 'tabular-nums' }}>
                    {formatNumber(stats.activeGrids)}
                  </p>
                </div>
                <div className="w-px h-6 bg-[var(--color-bg-elevated)]" />
                <div className="flex-1">
                  <p className="text-[0.625rem] text-[var(--color-text-secondary)]">오늘 방화</p>
                  <p className="text-[0.8125rem] font-bold text-[var(--color-warning)]" style={{ fontVariantNumeric: 'tabular-nums' }}>
                    {formatNumber(stats.dailyFires)}
                  </p>
                </div>
              </div>
              <p className="mt-1.5 text-[0.625rem] text-[var(--color-accent)] text-center">
                상황판 보기 →
              </p>
            </Link>
          </>
        )}
      </div>
    </>
  )
}
