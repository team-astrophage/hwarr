import { Link } from '@tanstack/react-router'
import { Header } from '../../../components/Header'
import { useStats } from '../api/useStats'
import { NewsFeed } from './NewsFeed'

export function LandingPage() {
  const { data: stats } = useStats()

  return (
    <div className="flex flex-col min-h-svh bg-[var(--color-bg-base)]">
      <Header />

      <main className="flex-1 flex flex-col px-5 pt-6 pb-4 gap-8">
        {/* Hero */}
        <section className="pt-4">
          <p className="text-[0.75rem] font-bold text-[var(--color-accent)] uppercase tracking-[2px] mb-3">
            GPS 기반 실시간 스트레스 해소
          </p>
          <h1
            className="text-[2rem] font-extrabold text-[var(--color-text-base)] leading-[1.15] tracking-tight"
            style={{ textWrap: 'balance' }}
          >
            지금, 이 자리에서
            <br />
            불을 질러라.
          </h1>
          <p
            className="text-[0.9375rem] text-[var(--color-text-secondary)] mt-3 leading-relaxed"
            style={{ textWrap: 'pretty' }}
          >
            화르르는 전국 어디서든 GPS 좌표 위에
            <br />
            익명으로 불을 지르는 실시간 체험입니다.
          </p>
        </section>

        {/* Stats */}
        <section className="flex gap-3">
          <div className="flex-1 bg-[var(--color-bg-surface)] rounded-[16px] p-4 shadow-[var(--shadow-medium)]">
            <p className="text-[0.625rem] font-bold text-[var(--color-text-secondary)] uppercase tracking-[1.5px] mb-2">
              화재 구역
            </p>
            <p
              className="text-[1.75rem] font-extrabold text-[var(--color-accent)] leading-none"
              style={{ fontVariantNumeric: 'tabular-nums' }}
            >
              {stats?.activeGrids?.toLocaleString() ?? '--'}
              <span className="text-[0.75rem] font-medium text-[var(--color-text-secondary)] ml-1">
                곳
              </span>
            </p>
          </div>
          <div className="flex-1 bg-[var(--color-bg-surface)] rounded-[16px] p-4 shadow-[var(--shadow-medium)]">
            <p className="text-[0.625rem] font-bold text-[var(--color-text-secondary)] uppercase tracking-[1.5px] mb-2">
              방화 건수
            </p>
            <p
              className="text-[1.75rem] font-extrabold text-[var(--color-warning)] leading-none"
              style={{ fontVariantNumeric: 'tabular-nums' }}
            >
              {stats?.totalFires?.toLocaleString() ?? '--'}
              <span className="text-[0.75rem] font-medium text-[var(--color-text-secondary)] ml-1">
                건
              </span>
            </p>
          </div>
        </section>

        {/* News */}
        <NewsFeed />
      </main>

      {/* Footer CTA */}
      <footer className="px-5 pt-2 pb-10">
        <Link
          to="/map"
          className="flex items-center justify-center w-full h-14 bg-[var(--color-accent)] text-[#000000] font-extrabold text-[0.9375rem] tracking-[1px] rounded-[14px] transition-[transform] duration-150 active:scale-[0.96] shadow-[var(--shadow-medium)]"
        >
          불 지르러 가기
        </Link>
        <p
          className="text-[0.6875rem] text-[var(--color-text-secondary)] text-center mt-3"
          style={{ textWrap: 'pretty' }}
        >
          완전 익명 · 로그인 불필요
        </p>
      </footer>
    </div>
  )
}
