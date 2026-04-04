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
    iconBg: 'rgba(255,140,0,0.15)',
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
    iconBg: 'rgba(255,68,68,0.12)',
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
    iconBg: 'rgba(30,215,96,0.12)',
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
    <section>
      {/* 섹션 헤더 */}
      <div className="flex items-center gap-2 mb-3">
        <h2 className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">속보</h2>
        <span className="text-[0.5625rem] font-bold text-[#ff4444] bg-[rgba(255,68,68,0.12)] px-2 py-0.5 rounded-[var(--radius-pill)] uppercase tracking-[0.5px]">
          Live
        </span>
      </div>

      {/* 뉴스 리스트 */}
      <div className="flex flex-col gap-2">
        {MOCK_NEWS.map((item) => (
          <div
            key={item.id}
            className="flex gap-3 items-start bg-[var(--color-bg-surface)] rounded-[12px] p-3 transition-colors duration-150 hover:bg-[var(--color-bg-elevated)]"
          >
            <div
              className="w-8 h-8 rounded-[8px] flex items-center justify-center shrink-0 text-[0.875rem]"
              style={{ background: item.iconBg }}
            >
              {item.icon}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-[0.8125rem] font-medium leading-[1.5] text-[var(--color-text-base)]">
                {item.text}
              </p>
              <div className="flex gap-1.5 items-center mt-1.5 text-[0.6875rem] text-[var(--color-text-secondary)]">
                <span>{item.time}</span>
                <span className="text-[var(--color-border)]">·</span>
                <span>{item.detail}</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}
