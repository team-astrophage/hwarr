import { Link, useRouterState } from '@tanstack/react-router'
import { useSocketStore, type ConnectionStatus } from '../lib/socketManager'
import { useFireStore } from '../features/fire-map/stores/fireStore'

interface ChipConfig {
  dotClass: string
  label: string
  pulse: boolean
}

function getChipConfig(
  status: ConnectionStatus,
  onlineUsers: number,
): ChipConfig {
  switch (status) {
    case 'connected':
      return {
        dotClass: 'bg-[var(--color-accent)]',
        label: onlineUsers > 0 ? `LIVE · ${onlineUsers}` : 'LIVE',
        pulse: true,
      }
    case 'connecting':
      return { dotClass: 'bg-amber-500', label: '연결 중…', pulse: true }
    case 'reconnecting':
      return { dotClass: 'bg-amber-500', label: '재연결 중…', pulse: true }
    case 'offline':
      return { dotClass: 'bg-zinc-500', label: '오프라인', pulse: false }
    case 'idle':
    default:
      return { dotClass: 'bg-zinc-500', label: '—', pulse: false }
  }
}

export function Header() {
  const { location } = useRouterState()
  const isMap = location.pathname === '/map'
  const status = useSocketStore((s) => s.status)
  const onlineUsers = useFireStore((s) => s.onlineUsers)
  const chip = getChipConfig(status, onlineUsers)

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

      {/* 연결 상태 chip */}
      <div
        className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--color-bg-surface)] py-1 pl-1.5 pr-2.5 shadow-[var(--shadow-medium)] transition-colors"
        aria-label={`연결 상태: ${chip.label}`}
      >
        <span className="relative flex size-2 shrink-0">
          {chip.pulse && (
            <span
              className={`absolute inline-flex size-full animate-ping rounded-full ${chip.dotClass} opacity-75`}
            />
          )}
          <span
            className={`relative inline-flex size-2 rounded-full ${chip.dotClass}`}
          />
        </span>
        <span className="text-[0.6875rem] font-bold uppercase leading-none tracking-wide text-[var(--color-text-secondary)]">
          {chip.label}
        </span>
      </div>
    </header>
  )
}
