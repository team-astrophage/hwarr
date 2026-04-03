/**
 * AI 뉴스 피드 Mock UI
 *
 * 랜딩 페이지에 뉴스 형식 화재 현황 표시.
 * Mock 데이터 하드코딩 → 나중에 AI 연동.
 */

const MOCK_NEWS = [
  {
    id: 1,
    icon: '🔥',
    iconBg: 'rgba(255,140,0,0.2)',
    text: (
      <>
        <span className="text-[var(--color-accent)] font-bold">강남 테헤란로</span> 일대{' '}
        <span className="text-[var(--color-warning)] font-bold">대형화재</span> 발생 — 14시부터
        4단계 지속 중
      </>
    ),
    time: '2분 전',
    detail: '활성 방화범 89명',
  },
  {
    id: 2,
    icon: '🔥',
    iconBg: 'rgba(255,68,68,0.15)',
    text: (
      <>
        <span className="text-[var(--color-accent)] font-bold">구로 디지털단지</span>역 반경
        300m 3단계{' '}
        <span className="text-[var(--color-warning)] font-bold">화재</span> 확산 중
      </>
    ),
    time: '5분 전',
    detail: '활성 방화범 34명',
  },
  {
    id: 3,
    icon: '🧯',
    iconBg: 'rgba(30,215,96,0.15)',
    text: (
      <>
        <span className="text-[var(--color-accent)] font-bold">판교 테크노밸리</span> 화재 진압
        완료 — 47분간 불탔다
      </>
    ),
    time: '12분 전',
    detail: '총 화재 127건',
  },
]

export function NewsFeed() {
  return (
    <div className="px-5 mb-5">
      {/* 헤더 */}
      <div className="flex items-center gap-2 mb-3">
        <span className="text-[0.875rem] font-bold text-[var(--color-text-base)]">속보</span>
        <span className="text-[0.625rem] font-bold text-[#ff4444] bg-[rgba(255,68,68,0.15)] px-2 py-0.5 rounded-[var(--radius-pill)] uppercase tracking-[0.5px]">
          Live
        </span>
      </div>

      {/* 뉴스 아이템 */}
      <div className="flex flex-col gap-2">
        {MOCK_NEWS.map((item) => (
          <div
            key={item.id}
            className="flex gap-3 items-start bg-[var(--color-bg-surface)] rounded-[14px] p-3.5 transition-colors hover:bg-[var(--color-bg-elevated)]"
          >
            <div
              className="w-9 h-9 rounded-[10px] flex items-center justify-center flex-shrink-0 text-[1rem]"
              style={{ background: item.iconBg }}
            >
              {item.icon}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-[0.8125rem] font-semibold leading-[1.45] text-[var(--color-text-base)]">
                {item.text}
              </p>
              <div className="flex gap-2 items-center mt-1 text-[0.6875rem] text-[var(--color-text-secondary)]">
                <span>{item.time}</span>
                <span>·</span>
                <span>{item.detail}</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
