import { Link } from '@tanstack/react-router'

export function NotFound() {
  return (
    <div className="flex min-h-dvh items-center justify-center bg-[var(--color-bg-base)] px-5">
      <div className="w-full max-w-[360px] text-center">
        <p className="text-[2.5rem] leading-none" aria-hidden>
          &#129519;
        </p>
        <h1 className="mt-4 text-[1.125rem] font-bold text-[var(--color-text-base)]">
          페이지를 찾을 수 없습니다
        </h1>
        <p className="mt-2 text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed">
          요청하신 페이지가 존재하지 않거나 이동되었습니다.
        </p>
        <Link
          to="/"
          className="mt-6 inline-block w-full rounded-[12px] bg-[var(--color-accent)] py-3 text-[0.9375rem] font-bold text-[#000000] transition-transform active:scale-[0.96]"
        >
          홈으로 돌아가기
        </Link>
      </div>
    </div>
  )
}
