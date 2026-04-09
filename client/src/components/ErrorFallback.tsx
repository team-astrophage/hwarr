import { useQueryErrorResetBoundary } from '@tanstack/react-query'
import { useRouter, type ErrorComponentProps } from '@tanstack/react-router'

export function ErrorFallback({ error, reset }: ErrorComponentProps) {
  const router = useRouter()
  const { reset: resetQuery } = useQueryErrorResetBoundary()

  const handleRetry = () => {
    resetQuery()
    reset()
  }

  const handleGoHome = () => {
    resetQuery()
    router.navigate({ to: '/' })
  }

  return (
    <div className="flex min-h-dvh items-center justify-center bg-[var(--color-bg-base)] px-5">
      <div className="w-full max-w-[360px] text-center">
        <p className="text-[2.5rem] leading-none" aria-hidden>
          &#128293;
        </p>
        <h1 className="mt-4 text-[1.125rem] font-bold text-[var(--color-text-base)]">
          문제가 발생했습니다
        </h1>
        <p className="mt-2 text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed">
          {error.message || '알 수 없는 오류가 발생했습니다.'}
        </p>
        <div className="mt-6 flex flex-col gap-3">
          <button
            onClick={handleRetry}
            className="w-full rounded-[12px] bg-[var(--color-accent)] py-3 text-[0.9375rem] font-bold text-[#000000] transition-transform active:scale-[0.96]"
          >
            다시 시도
          </button>
          <button
            onClick={handleGoHome}
            className="w-full rounded-[12px] bg-[var(--color-bg-surface)] py-3 text-[0.9375rem] font-bold text-[var(--color-text-secondary)] transition-transform active:scale-[0.96]"
          >
            홈으로 돌아가기
          </button>
        </div>
      </div>
    </div>
  )
}
