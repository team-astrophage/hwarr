/**
 * 오늘의 방화 지역 Top 10 — 랜딩 페이지 리스트.
 *
 * 백엔드 /api/ranking/today 연동. "{구} {동} NNN회" 형식.
 */

import { useRanking, type RankingItem } from '../api/useRanking'

type RankTone = 'warning' | 'accent' | 'negative' | 'muted'

const TONE_BG: Record<RankTone, string> = {
  warning: 'var(--color-warning)',
  accent: 'var(--color-accent)',
  negative: 'var(--color-negative)',
  muted: 'var(--color-bg-elevated)',
}

const TONE_TEXT: Record<RankTone, string> = {
  warning: 'var(--color-warning)',
  accent: 'var(--color-accent)',
  negative: 'var(--color-negative)',
  muted: 'var(--color-text-base)',
}

function toneFor(rank: number): RankTone {
  if (rank === 1) return 'warning'
  if (rank === 2) return 'accent'
  if (rank === 3) return 'negative'
  return 'muted'
}

function SectionHeader() {
  return (
    <div className="flex items-center gap-2.5">
      <h2 className="text-[1rem] font-bold leading-none tracking-tight text-[var(--color-text-base)]">
        오늘의 방화 지역 TOP 10
      </h2>
      <span className="rounded-[var(--radius-pill)] bg-[rgba(255,68,68,0.12)] px-2.5 py-1 text-[0.6875rem] font-bold uppercase leading-none tracking-wide text-[#ff4444]">
        Live
      </span>
    </div>
  )
}

function RankRow({ item }: { item: RankingItem }) {
  const tone = toneFor(item.rank)
  const rankBadgeColor = tone === 'muted' ? 'var(--color-text-secondary)' : '#000000'
  const displayCount = item.count.toLocaleString('ko-KR')

  return (
    <div
      className="flex items-center gap-3 rounded-[12px] bg-[var(--color-bg-surface)] px-4 py-3"
      aria-label={`${item.rank}위 ${item.region} ${displayCount}회`}
    >
      <div
        className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-[0.75rem] font-bold"
        style={{ background: TONE_BG[tone], color: rankBadgeColor }}
      >
        {item.rank}
      </div>
      <p className="min-w-0 flex-1 truncate text-[0.875rem] font-medium text-[var(--color-text-base)]">
        {item.region}
      </p>
      <p
        className="flex items-baseline gap-0.5 leading-none"
        style={{ fontVariantNumeric: 'tabular-nums' }}
      >
        <span
          className="text-[0.9375rem] font-bold"
          style={{ color: TONE_TEXT[tone] }}
        >
          {displayCount}
        </span>
        <span className="text-[0.75rem] font-medium text-[var(--color-text-secondary)]">
          회
        </span>
      </p>
    </div>
  )
}

export function RankingFeed() {
  const { data, isLoading } = useRanking()

  if (isLoading) {
    return (
      <section className="flex flex-col gap-4">
        <SectionHeader />
        <div className="flex flex-col gap-2">
          {Array.from({ length: 10 }).map((_, i) => (
            <div
              key={i}
              className="h-[46px] animate-pulse rounded-[12px] bg-[var(--color-bg-surface)]"
            />
          ))}
        </div>
      </section>
    )
  }

  if (!data || data.items.length === 0) {
    return (
      <section className="flex flex-col gap-4">
        <SectionHeader />
        <div className="rounded-[12px] bg-[var(--color-bg-surface)] p-4 text-center">
          <p className="text-[0.8125rem] text-[var(--color-text-secondary)]">
            오늘은 아직 집계된 지역이 없습니다
          </p>
        </div>
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-4">
      <SectionHeader />
      <div className="flex flex-col gap-2">
        {data.items.map((item) => (
          <RankRow key={item.rank} item={item} />
        ))}
      </div>
    </section>
  )
}
