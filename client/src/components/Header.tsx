import { useState } from 'react'
import { Link, useRouterState } from '@tanstack/react-router'
import { useSocketStore, type ConnectionStatus } from '../lib/socketManager'
import { useFireStore } from '../features/fire-map/stores/fireStore'
import { MenuDrawer } from './MenuDrawer'

type Indicator = {
  label: string
  colorVar: string
  pulse: boolean
}

function toIndicator(status: ConnectionStatus, isMap: boolean): Indicator {
  switch (status) {
    case 'connected':
      return { label: isMap ? '실시간 접속자' : 'Live', colorVar: '--color-accent', pulse: true }
    case 'connecting':
    case 'reconnecting':
      return { label: isMap ? '재연결 중' : 'Reconnecting', colorVar: '--color-warning', pulse: true }
    case 'offline':
      return { label: isMap ? '오프라인' : 'Offline', colorVar: '--color-negative', pulse: false }
    case 'idle':
    default:
      return { label: isMap ? '대기' : 'Idle', colorVar: '--color-text-secondary', pulse: false }
  }
}

export function Header() {
  const { location } = useRouterState()
  const isMap = location.pathname === '/map'
  const status = useSocketStore((s) => s.status)
  const indicator = toIndicator(status, isMap)
  const onlineUsers = useFireStore((s) => s.onlineUsers)
  const [menuOpen, setMenuOpen] = useState(false)

  return (
    <>
      <header
        className={`
          flex items-center justify-between px-3 h-11
          ${isMap ? 'absolute top-0 left-0 right-0 z-[1000] bg-[var(--color-bg-surface)]/85 backdrop-blur-md' : 'bg-[var(--color-bg-surface)]'}
        `}
      >
        {/* Logo */}
        <Link to="/" className="min-w-0 shrink-0">
          <span className="text-[1rem] font-extrabold text-[var(--color-text-base)] leading-none tracking-tight">
            화르르
          </span>
        </Link>

        {/* Right side: Live badge (map only) + Hamburger */}
        <div className="flex items-center gap-2">
          {/* Socket connection indicator — map only */}
          {isMap && (
            <div className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--color-bg-elevated)] py-1 pl-1.5 pr-2.5">
              <span className="relative flex size-2 shrink-0">
                {indicator.pulse && (
                  <span
                    className="absolute inline-flex size-full animate-ping rounded-full opacity-75"
                    style={{ backgroundColor: `var(${indicator.colorVar})` }}
                  />
                )}
                <span
                  className="relative inline-flex size-2 rounded-full"
                  style={{ backgroundColor: `var(${indicator.colorVar})` }}
                />
              </span>
              <span className="text-[0.6875rem] font-bold leading-none tracking-wide text-[var(--color-text-secondary)]">
                {indicator.label}
              </span>
              {onlineUsers > 0 && (
                <>
                  <span className="h-2.5 w-px bg-[var(--color-border)]" />
                  <span
                    className="text-[0.6875rem] font-bold leading-none text-[var(--color-text-base)]"
                    style={{ fontVariantNumeric: 'tabular-nums' }}
                  >
                    {onlineUsers}
                  </span>
                </>
              )}
            </div>
          )}

          {/* Hamburger menu button */}
          <button
            onClick={() => setMenuOpen(true)}
            className="flex items-center justify-center w-8 h-8 rounded-[8px] transition-colors hover:bg-[var(--color-bg-elevated)] active:scale-[0.92]"
            aria-label="메뉴 열기"
            aria-expanded={menuOpen}
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-secondary)" strokeWidth="2" strokeLinecap="round">
              <line x1="4" y1="6" x2="20" y2="6" />
              <line x1="4" y1="12" x2="20" y2="12" />
              <line x1="4" y1="18" x2="20" y2="18" />
            </svg>
          </button>
        </div>
      </header>

      <MenuDrawer open={menuOpen} onClose={() => setMenuOpen(false)} />
    </>
  )
}
