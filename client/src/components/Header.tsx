import { Link, useRouterState } from '@tanstack/react-router'
import { useSocketStore, type ConnectionStatus } from '../lib/socketManager'

type Indicator = {
  label: string
  /** CSS 변수 토큰 이름 (배경/펄스 색상) */
  colorVar: string
  /** 끊겼을 때는 pulse 애니메이션을 멈춰 사용자가 '죽은 점'으로 인지하게 한다 */
  pulse: boolean
}

function toIndicator(status: ConnectionStatus): Indicator {
  switch (status) {
    case 'connected':
      return { label: 'Live', colorVar: '--color-accent', pulse: true }
    case 'connecting':
    case 'reconnecting':
      return { label: 'Reconnecting', colorVar: '--color-warning', pulse: true }
    case 'offline':
      return { label: 'Offline', colorVar: '--color-negative', pulse: false }
    case 'idle':
    default:
      return { label: 'Idle', colorVar: '--color-text-secondary', pulse: false }
  }
}

export function Header() {
  const { location } = useRouterState()
  const isMap = location.pathname === '/map'
  const status = useSocketStore((s) => s.status)
  const indicator = toIndicator(status)

  return (
    <header
      className={`
        flex items-center justify-between px-5 h-14
        ${isMap ? 'absolute top-0 left-0 right-0 z-[1000]' : 'bg-[var(--color-bg-base)] border-b border-[var(--color-border)]'}
      `}
    >
      <Link to="/" className="min-w-0 shrink-0">
        <span className="text-[1.125rem] font-extrabold text-[var(--color-text-base)] leading-none tracking-tight">
          화르르
        </span>
      </Link>

      {/* Socket connection indicator */}
      <div className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--color-bg-surface)] py-1 pl-1.5 pr-2.5 shadow-[var(--shadow-medium)]">
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
        <span className="text-[0.6875rem] font-bold uppercase leading-none tracking-wide text-[var(--color-text-secondary)]">
          {indicator.label}
        </span>
      </div>
    </header>
  )
}
