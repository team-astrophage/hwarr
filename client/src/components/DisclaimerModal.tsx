import { useState, useEffect, useCallback } from 'react'

const STORAGE_KEY = 'hwarr_disclaimer_dismissed'
const SESSION_KEY = 'hwarr_disclaimer_session'

export function isDismissedToday(): boolean {
  if (sessionStorage.getItem(SESSION_KEY)) return true
  const stored = localStorage.getItem(STORAGE_KEY)
  if (!stored) return false
  const dismissedDate = new Date(stored).toDateString()
  const today = new Date().toDateString()
  return dismissedDate === today
}

interface DisclaimerModalProps {
  onAccept: () => void
}

export function DisclaimerModal({ onAccept }: DisclaimerModalProps) {
  const [hideToday, setHideToday] = useState(false)
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    if (isDismissedToday()) {
      onAccept()
    } else {
      setVisible(true)
    }
  }, [onAccept])

  const handleAccept = useCallback(() => {
    sessionStorage.setItem(SESSION_KEY, '1')
    if (hideToday) {
      localStorage.setItem(STORAGE_KEY, new Date().toISOString())
    }
    setVisible(false)
    onAccept()
  }, [hideToday, onAccept])

  if (!visible) return null

  return (
    <div className="fixed inset-0 z-[3000] flex items-center justify-center bg-black/70 backdrop-blur-sm px-5">
      <div className="w-full max-w-[360px] rounded-[16px] bg-[var(--color-bg-surface)] shadow-[var(--shadow-heavy)] px-6 py-6">
        {/* Header */}
        <div className="flex items-center gap-2 mb-4">
          <span className="text-2xl" aria-hidden>&#9888;&#65039;</span>
          <h2 className="text-[1.0625rem] font-bold text-[var(--color-text-base)]">
            서비스 이용 안내
          </h2>
        </div>

        {/* Body */}
        <div className="space-y-3 text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed">
          <p>
            <strong className="text-[var(--color-text-base)]">화르르는 지도 위에서 스트레스를 해소하는 엔터테인먼트 서비스</strong>이며,
            실제 화재 · 방화와는 어떠한 관련도 없습니다.
          </p>
          <p>
            본 서비스는 실제 방화 행위를 조장, 권장, 교사하지 않습니다.
            지도 위의 '불' 표현은 스트레스 해소를 위한 상징적 연출이며, 실제 화재 · 재난 정보가 아닙니다.
          </p>
          <p>
            실제 방화는 <strong className="text-[var(--color-negative)]">형법 제164조</strong>에 의해
            무기 또는 3년 이상의 징역에 처해지는 중대 범죄입니다.
            절대로 실제 방화 행위를 시도하지 마십시오.
          </p>
          <p>
            서비스 이용 중 발생하는 모든 행위에 대한 법적 책임은 이용자 본인에게 있으며,
            운영자는 이용자의 서비스 외부 행위에 대해 책임을 지지 않습니다.
          </p>
        </div>

        {/* Hide today checkbox */}
        <label className="mt-5 flex items-center gap-2 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={hideToday}
            onChange={(e) => setHideToday(e.target.checked)}
            className="size-4 rounded accent-[var(--color-accent)]"
          />
          <span className="text-[0.75rem] text-[var(--color-text-secondary)]">
            오늘 하루 보지 않기
          </span>
        </label>

        {/* CTA */}
        <button
          onClick={handleAccept}
          className="mt-4 w-full rounded-[12px] bg-[var(--color-accent)] py-3 text-[0.9375rem] font-bold text-[#000000] transition-transform active:scale-[0.96]"
        >
          확인했습니다
        </button>
      </div>
    </div>
  )
}
