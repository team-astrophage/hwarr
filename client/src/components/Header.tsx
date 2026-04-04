import { Link, useRouterState } from '@tanstack/react-router'

export function Header() {
  const { location } = useRouterState()
  const isMap = location.pathname === '/map'

  return (
    <header
      className={`
        flex items-center justify-between px-5 h-14
        ${isMap ? 'absolute top-0 left-0 right-0 z-[1000]' : 'bg-[var(--color-bg-base)]'}
      `}
    >
      <Link to="/" className="flex items-center gap-1.5">
        <span className="text-[1.125rem] font-extrabold text-[var(--color-text-base)] leading-none tracking-tight">
          화르르
        </span>
        <span className="text-[0.625rem] font-bold text-[var(--color-text-secondary)] leading-none uppercase tracking-[1.5px]">
          hwarr
        </span>
      </Link>

      {/* Live indicator */}
      <div className="flex items-center gap-2 h-8 px-3 bg-[var(--color-bg-surface)] rounded-[var(--radius-pill)] shadow-[var(--shadow-medium)]">
        <span className="relative flex h-2 w-2">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--color-accent)] opacity-75" />
          <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--color-accent)]" />
        </span>
        <span className="text-[0.6875rem] font-bold text-[var(--color-text-secondary)] uppercase tracking-[1px]">
          Live
        </span>
      </div>
    </header>
  )
}
