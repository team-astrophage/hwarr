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
      <Link to="/" className="min-w-0 shrink-0">
        <span className="text-[1.125rem] font-extrabold text-[var(--color-text-base)] leading-none tracking-tight">
          화르르
        </span>
      </Link>

      {/* Live indicator */}
      <div className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--color-bg-surface)] py-1 pl-1.5 pr-2.5 shadow-[var(--shadow-medium)]">
        <span className="relative flex size-2 shrink-0">
          <span className="absolute inline-flex size-full animate-ping rounded-full bg-[var(--color-accent)] opacity-75" />
          <span className="relative inline-flex size-2 rounded-full bg-[var(--color-accent)]" />
        </span>
        <span className="text-[0.6875rem] font-bold uppercase leading-none tracking-wide text-[var(--color-text-secondary)]">
          Live
        </span>
      </div>
    </header>
  )
}
