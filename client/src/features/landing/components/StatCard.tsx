type StatTone = 'accent' | 'warning' | 'negative' | 'base'

const TONE_COLOR: Record<StatTone, string> = {
  accent: 'var(--color-accent)',
  warning: 'var(--color-warning)',
  negative: 'var(--color-negative)',
  base: 'var(--color-text-base)',
}

interface StatCardProps {
  label: string
  value: number | undefined
  unit: string
  tone: StatTone
}

export function StatCard({ label, value, unit, tone }: StatCardProps) {
  const displayValue = value !== undefined ? value.toLocaleString('ko-KR') : '--'
  const ariaLabel =
    value !== undefined
      ? `${label} ${displayValue}${unit}`
      : `${label} 불러오는 중`

  return (
    <div className="min-w-0 rounded-[12px] bg-[var(--color-bg-surface)] p-4 shadow-[var(--shadow-medium)]">
      <p className="mb-3 text-[0.8125rem] font-medium leading-snug text-[var(--color-text-base)]">
        {label}
      </p>
      <p
        className="flex flex-wrap items-baseline gap-x-1 leading-none"
        style={{ fontVariantNumeric: 'tabular-nums' }}
        aria-label={ariaLabel}
      >
        <span
          className="text-[1.5rem] font-bold"
          style={{ color: TONE_COLOR[tone] }}
        >
          {displayValue}
        </span>
        <span className="text-[0.8125rem] font-medium text-[var(--color-text-base)]">
          {unit}
        </span>
      </p>
    </div>
  )
}
