/**
 * 속보 뉴스 피드 — 백엔드 /api/news 연동
 *
 * 랜딩 페이지에 뉴스 형식으로 실시간 화재 현황 표시.
 * 서버에서 템플릿 + Redis 데이터를 조합한 headline_parts를 받아 렌더링.
 */

import { useNews } from "../api/useNews"
import type { HeadlinePart } from "../api/useNews"

const HIGHLIGHT_CLASS: Record<string, string> = {
  accent: "text-[var(--color-accent)] font-bold",
  warning: "text-[var(--color-warning)] font-bold",
}

function Headline({ parts }: { parts: HeadlinePart[] }) {
  return (
    <>
      {parts.map((part, i) =>
        part.highlight ? (
          <span key={i} className={HIGHLIGHT_CLASS[part.highlight]}>
            {part.text}
          </span>
        ) : (
          <span key={i}>{part.text}</span>
        ),
      )}
    </>
  )
}

export function NewsFeed() {
  const { data: items, isLoading } = useNews()

  if (isLoading) {
    return (
      <div className="px-5 mb-5">
        <div className="flex items-center gap-2 mb-3">
          <h2 className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">속보</h2>
          <span className="text-[0.5625rem] font-bold text-[#ff4444] bg-[rgba(255,68,68,0.12)] px-2 py-0.5 rounded-[var(--radius-pill)] uppercase tracking-[0.5px]">
            Live
          </span>
        </div>
        <div className="flex flex-col gap-2">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-[72px] bg-[var(--color-bg-surface)] rounded-[12px] animate-pulse"
            />
          ))}
        </div>
      </div>
    )
  }

  if (!items || items.length === 0) {
    return (
      <div className="px-5 mb-5">
        <div className="flex items-center gap-2 mb-3">
          <h2 className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">속보</h2>
          <span className="text-[0.5625rem] font-bold text-[#ff4444] bg-[rgba(255,68,68,0.12)] px-2 py-0.5 rounded-[var(--radius-pill)] uppercase tracking-[0.5px]">
            Live
          </span>
        </div>
        <div className="bg-[var(--color-bg-surface)] rounded-[12px] p-4 text-center">
          <p className="text-[0.8125rem] text-[var(--color-text-secondary)]">
            현재 활성 화재가 없습니다
          </p>
        </div>
      </div>
    )
  }

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
        {items.map((item) => (
          <div
            key={item.id}
            className="flex gap-3 items-start bg-[var(--color-bg-surface)] rounded-[12px] p-3 transition-colors duration-150 hover:bg-[var(--color-bg-elevated)]"
          >
            <div
              className="w-8 h-8 rounded-[8px] flex items-center justify-center shrink-0 text-[0.875rem]"
              style={{ background: item.icon_bg }}
            >
              {item.icon}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-[0.8125rem] font-medium leading-[1.5] text-[var(--color-text-base)]">
                <Headline parts={item.headline_parts} />
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
