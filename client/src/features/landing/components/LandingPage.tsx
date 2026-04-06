import { Link } from '@tanstack/react-router'
import { Header } from '../../../components/Header'
import { FeedbackButton } from '../../feedback/components/FeedbackButton'
import { useStats } from '../api/useStats'
import { RankingFeed } from './RankingFeed'
import { StatCard } from './StatCard'

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
            스트레스, 여기서 마음껏 불태우세요.
            <br />
            내 위치에 불을 지르고,
            <br />
            전국에서 불타는 곳을 구경하세요.
          </p>
        </section>

        {/* 상황판 */}
        <section className="flex flex-col gap-4">
          <h2 className="text-[1rem] font-bold leading-none tracking-tight text-[var(--color-text-base)]">
            상황판
          </h2>
          <div className="grid grid-cols-2 gap-2.5">
            <StatCard
              label="실시간 화재 구역"
              value={stats?.activeGrids}
              unit="곳"
              tone="accent"
            />
            <StatCard
              label="실시간 방화 시도"
              value={stats?.totalFires}
              unit="회"
              tone="warning"
            />
            <StatCard
              label="오늘의 방화 시도"
              value={stats?.dailyFires}
              unit="회"
              tone="negative"
            />
            <StatCard
              label="누적 방화 시도"
              value={stats?.cumulativeFires}
              unit="회"
              tone="base"
            />
          </div>
        </section>

        {/* Today's ranking */}
        <RankingFeed />
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
          <div className="mt-1 flex justify-center">
            <FeedbackButton>
              {(open) => (
                <button
                  onClick={open}
                  className="text-[0.6875rem] text-[var(--color-text-secondary)] underline-offset-2 hover:underline"
                >
                  의견 보내기
                </button>
              )}
            </FeedbackButton>
          </div>
        </div>
      </footer>
    </div>
  )
}
