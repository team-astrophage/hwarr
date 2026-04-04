import { Link } from '@tanstack/react-router'
import { Header } from '../../../components/Header'
import { useStats } from '../api/useStats'
import { NewsFeed } from './NewsFeed'

export function LandingPage() {
  const { data: stats } = useStats()

  return (
    <div className="relative flex min-h-svh flex-col bg-[var(--color-bg-base)]">
      <Header />

      <main className="mt-5 flex flex-1 flex-col gap-8 px-5 pt-6 pb-[calc(7.75rem+env(safe-area-inset-bottom,0px))]">
        {/* Hero */}
        <section className="pt-20 pb-2">
          <h1
            className="text-[2.375rem] font-bold text-[var(--color-text-max)] leading-[1.08] tracking-[-0.035em]"
            style={{ textWrap: 'balance' }}
          >
            지금, 이 자리에서
            <br />
            <span className="text-[var(--color-accent)]">불을 질러라.</span>
          </h1>
          <p
            className="mt-7 mb-1 max-w-[28ch] text-[0.9375rem] text-[var(--color-text-secondary)] leading-[1.65]"
            style={{ textWrap: 'pretty' }}
          >
            이 자리의 좌표가 출발점이에요.
            <br />
            지도 위에 익명으로 불을 올리고, 전국과 같은 맵을 실시간으로 함께 봅니다.
          </p>
        </section>

        {/* 상황판 */}
        <section className="flex flex-col gap-4">
          <h2 className="text-[1rem] font-bold leading-none tracking-tight text-[var(--color-text-base)]">
            상황판
          </h2>
          <div className="flex gap-2.5">
            <div className="min-w-0 flex-1 rounded-[12px] bg-[var(--color-bg-surface)] p-4 shadow-[var(--shadow-medium)]">
              <p className="mb-3 text-[0.8125rem] font-medium leading-snug text-[var(--color-text-base)]">
                실시간 화재 구역
              </p>
              <p
                className="flex flex-wrap items-baseline gap-x-1 leading-none"
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                <span className="text-[1.875rem] font-bold text-[var(--color-accent)]">
                  {stats?.activeGrids?.toLocaleString() ?? '--'}
                </span>
                <span className="text-[0.8125rem] font-medium text-[var(--color-text-base)]">
                  곳
                </span>
              </p>
            </div>
            <div className="min-w-0 flex-1 rounded-[12px] bg-[var(--color-bg-surface)] p-4 shadow-[var(--shadow-medium)]">
              <p className="mb-3 text-[0.8125rem] font-medium leading-snug text-[var(--color-text-base)]">
                지금까지 방화 건수
              </p>
              <p
                className="flex flex-wrap items-baseline gap-x-1 leading-none"
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                <span className="text-[1.875rem] font-bold text-[var(--color-warning)]">
                  {stats?.totalFires?.toLocaleString() ?? '--'}
                </span>
                <span className="text-[0.8125rem] font-medium text-[var(--color-text-base)]">
                  건
                </span>
              </p>
            </div>
          </div>
        </section>

        {/* News */}
        <NewsFeed />
      </main>

      <footer className="pointer-events-none fixed bottom-0 left-1/2 z-[100] w-full max-w-[430px] -translate-x-1/2">
        <div
          className="pointer-events-auto border-t border-[color-mix(in_srgb,var(--color-border)_35%,transparent)] bg-[color-mix(in_srgb,var(--color-bg-base)_92%,transparent)] px-5 pt-3 shadow-[0_-12px_40px_rgba(0,0,0,0.35)] backdrop-blur-md supports-[backdrop-filter]:bg-[color-mix(in_srgb,var(--color-bg-base)_88%,transparent)] pb-[max(0.75rem,env(safe-area-inset-bottom,0px))]"
        >
          <Link
            to="/map"
            className="flex h-14 w-full items-center justify-center rounded-[14px] bg-[var(--color-accent)] text-[0.9375rem] font-extrabold tracking-[1px] text-[#000000] shadow-[var(--shadow-medium)] transition-[transform] duration-150 active:scale-[0.96]"
          >
            실시간 지도에서 불 지르기
          </Link>
          <p
            className="mt-2 text-center text-[0.6875rem] text-[var(--color-text-secondary)]"
            style={{ textWrap: 'pretty' }}
          >
            완전 익명 · 로그인 불필요
          </p>
        </div>
      </footer>
    </div>
  )
}
