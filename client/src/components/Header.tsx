import { Link, useRouterState } from '@tanstack/react-router'
import { useSocketStore, type ConnectionStatus } from '../lib/socketManager'
import { useFireStore } from '../features/fire-map/stores/fireStore'

type Indicator = {
  label: string
  /** CSS 변수 토큰 이름 (배경/펄스 색상) */
  colorVar: string
  /** 끊겼을 때는 pulse 애니메이션을 멈춰 사용자가 '죽은 점'으로 인지하게 한다 */
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

  return (
    <header
      className={`
        flex items-center justify-between px-5 h-14
        ${isMap ? 'absolute top-0 left-0 right-0 z-[1000]' : 'bg-[var(--color-bg-base)]'}
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
        <span className="text-[0.6875rem] font-bold leading-none tracking-wide text-[var(--color-text-secondary)]">
          {indicator.label}
        </span>
        {isMap && onlineUsers > 0 && (
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
    </header>
  )
}
