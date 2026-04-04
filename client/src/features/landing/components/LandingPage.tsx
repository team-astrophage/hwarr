import { Link } from '@tanstack/react-router'
import { Header } from '../../../components/Header'

export function LandingPage() {
  return (
    <div className="flex flex-col min-h-svh bg-[var(--color-bg-base)]">
      {/* Shared header */}
      <Header />

      {/* Main Content */}
      <main className="flex-1 flex flex-col justify-center px-5 gap-3">
        {/* Hero text */}
        <div className="mb-6">
          <h2
            className="text-[1.5rem] font-bold text-[var(--color-text-base)] leading-tight"
            style={{ textWrap: 'balance' }}
          >
            지금, 이 자리에서
            <br />
            <span className="text-[var(--color-accent)]">불을 질러라</span>
          </h2>
          <p className="text-[0.875rem] text-[var(--color-text-secondary)] mt-2">
            GPS 기반 전국 실시간 스트레스 해소
          </p>
        </div>

        {/* Stat widgets — rounded card style like reference */}
        <div className="bg-[var(--color-bg-surface)] rounded-[20px] shadow-[var(--shadow-medium)] p-5">
          <div className="flex items-center gap-4">
            {/* 실시간 방화범 */}
            <div className="flex-1">
              <p className="text-[0.6875rem] font-bold text-[var(--color-text-secondary)] uppercase tracking-[1.5px] mb-2">
                실시간 방화범
              </p>
              <p
                className="text-[2rem] font-bold text-[var(--color-accent)] leading-none"
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                --
                <span className="text-[0.875rem] font-normal text-[var(--color-text-secondary)] ml-1">
                  명
                </span>
              </p>
            </div>

            {/* Divider */}
            <div className="w-px h-12 bg-[var(--color-border)]" />

            {/* 전국 화재 */}
            <div className="flex-1">
              <p className="text-[0.6875rem] font-bold text-[var(--color-text-secondary)] uppercase tracking-[1.5px] mb-2">
                전국 화재
              </p>
              <p
                className="text-[2rem] font-bold text-[var(--color-accent)] leading-none"
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                --
                <span className="text-[0.875rem] font-normal text-[var(--color-text-secondary)] ml-1">
                  건
                </span>
              </p>
            </div>
          </div>
        </div>
      </main>

      {/* Footer — CTA */}
      <footer className="px-5 pt-6 pb-10 flex flex-col gap-3">
        <Link
          to="/map"
          className="flex items-center justify-center w-full h-14 bg-[var(--color-accent)] text-[#000000] font-bold text-[0.875rem] uppercase tracking-[2px] rounded-[14px] transition-[transform] duration-150 active:scale-[0.96]"
        >
          불 지르러 가기
        </Link>

        <p
          className="text-[0.75rem] text-[var(--color-text-secondary)] text-center"
          style={{ textWrap: 'pretty' }}
        >
          완전 익명 · 로그인 불필요
        </p>
      </footer>
    </div>
  )
}
