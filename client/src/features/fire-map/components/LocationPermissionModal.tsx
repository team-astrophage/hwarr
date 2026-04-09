type ModalMode = 'pre-permission' | 'loading' | 'error'

interface LocationPermissionModalProps {
  mode: ModalMode
  permissionDenied: boolean
  onRetry: () => void
  onDismiss?: () => void
}

const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent)

function getContent(mode: ModalMode, permissionDenied: boolean) {
  if (mode === 'pre-permission') {
    return {
      icon: '🔥',
      title: '내 위치에 불을 지르세요',
      description: 'GPS 위치로 지도에 가상의 불을 피울 수 있어요.\n위치 권한을 허용해주세요.',
      cta: '위치 허용하기',
    }
  }

  if (mode === 'loading') {
    return {
      icon: null,
      title: '위치를 찾고 있어요',
      description: 'GPS 신호를 수신 중입니다...',
      cta: null,
    }
  }

  // mode === 'error'
  if (permissionDenied) {
    const description = isIOS
      ? '설정 앱 → 개인정보 보호 및 보안 → 위치 서비스 → Safari(또는 사용 중인 브라우저) → 허용으로 변경해주세요.'
      : '브라우저 주소창 왼쪽 자물쇠 아이콘 → 사이트 설정 → 위치 → 허용으로 변경 후 새로고침 해주세요.'

    return {
      icon: '🔒',
      title: '위치 권한이 차단되었어요',
      description,
      cta: '새로고침',
    }
  }

  return {
    icon: '📍',
    title: '위치 권한이 필요해요',
    description: '내 GPS 위치에 불을 질러 스트레스를 푸세요.\n위치 공유를 허용해주세요.',
    cta: '위치 권한 허용',
  }
}

export function LocationPermissionModal({
  mode,
  permissionDenied,
  onRetry,
  onDismiss,
}: LocationPermissionModalProps) {
  const { icon, title, description, cta } = getContent(mode, permissionDenied)

  const handleClick = () => {
    if (mode === 'error' && permissionDenied) {
      window.location.reload()
    } else {
      onRetry()
    }
  }

  return (
    <div className='fixed inset-0 z-[2000] bg-black/60 backdrop-blur-sm flex items-start justify-center pt-[40vh] px-4'>
      <div className='bg-[var(--color-bg-surface)] rounded-[16px] shadow-[var(--shadow-heavy)] w-full max-w-[320px] px-6 py-6 flex flex-col items-center text-center'>
        {icon ? (
          <div className='text-5xl mb-3' aria-hidden='true'>
            {icon}
          </div>
        ) : (
          <div className='w-12 h-12 mb-3 rounded-full border-4 border-[var(--color-accent)] border-t-transparent animate-spin' />
        )}
        <h2 className='text-[1.0625rem] font-bold text-[var(--color-text-primary)] mb-2'>
          {title}
        </h2>
        <p className='text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed mb-5 whitespace-pre-line'>
          {description}
        </p>
        {cta && (
          <button
            onClick={handleClick}
            className='w-full bg-[var(--color-accent)] text-white rounded-[12px] py-3 text-[0.9375rem] font-bold transition-transform active:scale-95'
          >
            {cta}
          </button>
        )}
        {onDismiss && (
          <button
            onClick={onDismiss}
            className={`mt-2 w-full py-2.5 text-[0.8125rem] transition-colors ${
              cta
                ? 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-base)]'
                : 'bg-[var(--color-bg-elevated)] text-[var(--color-text-secondary)] rounded-[12px] hover:text-[var(--color-text-base)]'
            }`}
          >
            위치 없이 구경하기
          </button>
        )}
      </div>
    </div>
  )
}
