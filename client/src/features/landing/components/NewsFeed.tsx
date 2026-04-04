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

function SectionHeader() {
  return (
    <div className="flex items-center gap-2.5">
      <h2 className="text-[1rem] font-bold leading-none tracking-tight text-[var(--color-text-base)]">
        속보
      </h2>
      <span className="rounded-[var(--radius-pill)] bg-[rgba(255,68,68,0.12)] px-2.5 py-1 text-[0.6875rem] font-bold uppercase leading-none tracking-wide text-[#ff4444]">
        Live
      </span>
    </div>
  )
}

export function NewsFeed() {
  const { data: items, isLoading } = useNews()

  if (isLoading) {
    return (
      <section className="flex flex-col gap-4">
        <SectionHeader />
        <div className="flex flex-col gap-3">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-[72px] animate-pulse rounded-[12px] bg-[var(--color-bg-surface)]"
            />
          ))}
        </div>
      </section>
    )
  }

  if (!items || items.length === 0) {
    return (
      <section className="flex flex-col gap-4">
        <SectionHeader />
        <div className="rounded-[12px] bg-[var(--color-bg-surface)] p-4 text-center">
          <p className="text-[0.8125rem] text-[var(--color-text-secondary)]">
            현재 활성 화재가 없습니다
          </p>
        </div>
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-4">
      <SectionHeader />

      <div className="flex flex-col gap-3">
        {items.map((item) => (
          <div
            key={item.id}
            className="flex gap-3 items-start rounded-[12px] bg-[var(--color-bg-surface)] p-4 transition-colors duration-150 hover:bg-[var(--color-bg-elevated)]"
          >
            <div
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px] text-[0.875rem]"
              style={{ background: item.icon_bg }}
            >
              {item.icon}
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-[0.8125rem] font-medium leading-[1.5] text-[var(--color-text-base)]">
                <Headline parts={item.headline_parts} />
              </p>
              <div className="mt-1.5 flex items-center gap-1.5 text-[0.6875rem] text-[var(--color-text-secondary)]">
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
