interface LocationPermissionModalProps {
  permissionDenied: boolean
  onRetry: () => void
}

export function LocationPermissionModal({
  permissionDenied,
  onRetry,
}: LocationPermissionModalProps) {
  const handleClick = () => {
    if (permissionDenied) {
      window.location.reload()
    } else {
      onRetry()
    }
  }

  const icon = permissionDenied ? '🔒' : '📍'
  const title = permissionDenied
    ? '위치 권한이 차단되었어요'
    : '위치 권한이 필요해요'
  const description = permissionDenied
    ? '브라우저 주소창 왼쪽 자물쇠 아이콘 → 사이트 설정 → 위치 허용으로 변경 후 새로고침 해주세요'
    : '근처에서 불을 지르려면 위치 공유가 필요합니다'
  const cta = permissionDenied ? '새로고침' : '위치 권한 허용'

  return (
    <div className='fixed inset-0 z-[2000] bg-black/60 backdrop-blur-sm flex items-start justify-center pt-[40vh] px-4'>
      <div className='bg-[var(--color-bg-surface)] rounded-[16px] shadow-[var(--shadow-heavy)] w-full max-w-[320px] px-6 py-6 flex flex-col items-center text-center'>
        <div className='text-5xl mb-3' aria-hidden='true'>
          {icon}
        </div>
        <h2 className='text-[1.0625rem] font-bold text-[var(--color-text-primary)] mb-2'>
          {title}
        </h2>
        <p className='text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed mb-5'>
          {description}
        </p>
        <button
          onClick={handleClick}
          className='w-full bg-[var(--color-accent)] text-white rounded-[12px] py-3 text-[0.9375rem] font-bold transition-transform active:scale-95'
        >
          {cta}
        </button>
      </div>
    </div>
  )
}
