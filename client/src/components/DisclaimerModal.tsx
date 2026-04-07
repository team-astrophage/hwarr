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
            <strong className="text-[var(--color-text-base)]">① 서비스 성격</strong>
            <br />
            화르르는 GPS 기반 지도 위에서 가상의 불꽃 이펙트를 통해 스트레스를 해소하는 엔터테인먼트 서비스입니다.
            화면에 표시되는 모든 '불' 표현은 시각적 연출이며, 실제 화재 · 재난 정보가 아닙니다.
          </p>
          <p>
            <strong className="text-[var(--color-text-base)]">② 가상 콘텐츠 고지</strong>
            <br />
            본 서비스의 모든 콘텐츠(불꽃 이펙트, 화재 현황, 순위 등)는 100% 가상이며,
            현실의 어떠한 사건 · 장소 · 인물과도 관련이 없습니다.
          </p>
          <p>
            <strong className="text-[var(--color-text-base)]">③ 방화 행위 경고</strong>
            <br />
            본 서비스는 실제 방화 행위를 조장 · 권장 · 교사하지 않습니다.
            실제 방화는 <strong className="text-[var(--color-negative)]">형법 제164조~제167조</strong>에 의해 처벌되는 중대 범죄입니다.
            절대로 실제 방화 행위를 시도하지 마십시오.
          </p>
          <p>
            <strong className="text-[var(--color-text-base)]">④ 이용 연령</strong>
            <br />
            본 서비스는 만 14세 이상을 대상으로 합니다.
            만 14세 미만의 이용자는 법정대리인의 동의 없이 서비스를 이용할 수 없습니다.
          </p>
          <p>
            <strong className="text-[var(--color-text-base)]">⑤ 이용자 책임</strong>
            <br />
            운영자는 서비스 내 가상 콘텐츠 제공에 한하여 책임을 부담하며,
            서비스 이용 중 또는 이용 후 발생하는 이용자의 오프라인 행위에 대한 법적 책임은 전적으로 이용자 본인에게 있습니다.
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
