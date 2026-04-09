/**
 * 히트맵 아카이브 페이지 Mock UI
 *
 * 일별/주별/월별 탭 + 캘린더 히트맵 + TOP 화재 지역 랭킹.
 * 하드코딩 Mock 데이터, 백엔드 연동 후순위.
 */

import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Header } from '../../../components/Header'

const TABS = ['일별', '주별', '월별'] as const

const DAY_LABELS = ['일', '월', '화', '수', '목', '금', '토']

// Mock 히트맵 데이터 (0~5 레벨)
const HEATMAP_DATA = [
  // Week 1 (수~토)
  -1, -1, -1, 2, 3, 5, 4,
  // Week 2
  1, 2, 3, 4, 2, 5, 3,
  // Week 3
  1, 3, 2, 4, 5, 3, 2,
  // Week 4
  2, 4, 3, 1, 0, 0, 0,
]

const TOP_LOCATIONS = [
  { rank: 1, name: '강남 테헤란로', count: 2847, pct: 100 },
  { rank: 2, name: '구로 디지털단지', count: 2221, pct: 78 },
  { rank: 3, name: '판교 테크노밸리', count: 1850, pct: 65 },
  { rank: 4, name: '여의도 IFC', count: 1480, pct: 52 },
  { rank: 5, name: '성수동 뚝섬', count: 1167, pct: 41 },
]

const LEVEL_COLORS: Record<number, string> = {
  0: 'bg-[var(--color-bg-elevated)]',
  1: 'bg-[#3b1a1a]',
  2: 'bg-[#6b2020]',
  3: 'bg-[#a33030]',
  4: 'bg-[#e04040]',
  5: 'bg-[#ff6b35] shadow-[0_0_8px_rgba(255,107,53,0.4)]',
}

const RANK_COLORS: Record<number, string> = {
  1: 'text-[#ffd700]',
  2: 'text-[#c0c0c0]',
  3: 'text-[#cd7f32]',
}

export function ArchivePage() {
  const [activeTab, setActiveTab] = useState<(typeof TABS)[number]>('일별')
  return (
    <div className="flex flex-col min-h-svh bg-[var(--color-bg-base)]">
      <title>주간 리포트 — 화르르</title>
      <meta name="description" content="화르르 주간 화재 히트맵과 지역별 통계를 확인하세요." />
      <Header />

      {/* Coming Soon overlay */}
      <div className="fixed inset-0 z-[2000] flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="mx-6 w-full max-w-[340px] bg-[var(--color-bg-surface)] rounded-[20px] shadow-[var(--shadow-heavy)] p-6 text-center">
            <div className="w-14 h-14 mx-auto mb-4 rounded-full bg-[var(--color-accent)]/10 flex items-center justify-center">
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--color-accent)" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <line x1="18" y1="20" x2="18" y2="10" />
                <line x1="12" y1="20" x2="12" y2="4" />
                <line x1="6" y1="20" x2="6" y2="14" />
              </svg>
            </div>
            <h2 className="text-[1.125rem] font-extrabold text-[var(--color-text-base)] mb-2">
              주간 리포트 준비 중
            </h2>
            <p className="text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed mb-1">
              지역별 히트맵, 주간·월간 화재 통계,
            </p>
            <p className="text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed mb-5">
              TOP 화재 지역 랭킹이 곧 찾아옵니다.
            </p>
            <Link
              to="/"
              className="block w-full rounded-[12px] bg-[var(--color-accent)] py-3 text-[0.875rem] font-bold text-black transition-transform active:scale-[0.96]"
            >
              상황판 보러가기
            </Link>
          </div>
        </div>

      {/* 탭 */}
      <div className="flex gap-1 mx-5 mt-4 bg-[var(--color-bg-surface)] rounded-[12px] p-1">
        {TABS.map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`flex-1 py-2.5 text-center text-[0.8125rem] font-bold rounded-[10px] transition-all uppercase tracking-[1px] ${
              activeTab === tab
                ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text-base)]'
                : 'text-[var(--color-text-secondary)]'
            }`}
          >
            {tab}
          </button>
        ))}
      </div>

      {/* 오늘의 화재 현황 */}
      <div className="mx-5 mt-5 bg-[var(--color-bg-surface)] rounded-[20px] shadow-[var(--shadow-medium)] p-5">
        <p className="text-[0.6875rem] font-bold text-[var(--color-text-secondary)] uppercase tracking-[1.5px] mb-4">
          오늘의 화재 현황
        </p>
        <div className="grid grid-cols-3 gap-3">
          <div className="text-center">
            <p className="text-[1.5rem] font-bold text-[var(--color-warning)]" style={{ fontVariantNumeric: 'tabular-nums' }}>
              1,842
            </p>
            <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-1">총 화재</p>
          </div>
          <div className="text-center">
            <p className="text-[1.5rem] font-bold text-[var(--color-accent)]" style={{ fontVariantNumeric: 'tabular-nums' }}>
              247
            </p>
            <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-1">방화범</p>
          </div>
          <div className="text-center">
            <p className="text-[1.5rem] font-bold text-[var(--color-warning)]" style={{ fontVariantNumeric: 'tabular-nums' }}>
              3
            </p>
            <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-1">전소 달성</p>
          </div>
        </div>
      </div>

      {/* 히트맵 캘린더 */}
      <div className="mx-5 mt-5">
        <h3 className="text-[1rem] font-bold text-[var(--color-text-base)] mb-3">2026년 4월</h3>
        <div className="grid grid-cols-7 gap-[3px]">
          {DAY_LABELS.map((d) => (
            <div key={d} className="text-[0.625rem] text-[var(--color-text-secondary)] text-center pb-1">
              {d}
            </div>
          ))}
          {HEATMAP_DATA.map((level, i) => (
            <div
              key={i}
              className={`aspect-square rounded-[4px] transition-transform hover:scale-[1.15] ${
                level === -1
                  ? 'bg-transparent'
                  : LEVEL_COLORS[level] ?? LEVEL_COLORS[0]
              } ${level >= 0 ? 'cursor-pointer' : ''}`}
            />
          ))}
        </div>
        {/* 범례 */}
        <div className="flex items-center gap-1 justify-end mt-2.5 text-[0.625rem] text-[var(--color-text-secondary)]">
          적음
          {[0, 1, 2, 3, 4, 5].map((l) => (
            <div
              key={l}
              className={`w-3 h-3 rounded-[2px] ${LEVEL_COLORS[l]}`}
            />
          ))}
          많음
        </div>
      </div>

      {/* TOP 화재 지역 */}
      <div className="mx-5 mt-5 mb-8">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-[0.875rem] font-bold text-[var(--color-text-base)]">
            역대 TOP 화재 지역
          </h3>
          <span className="text-[0.6875rem] text-[var(--color-text-secondary)] bg-[var(--color-bg-elevated)] px-3 py-1 rounded-[var(--radius-pill)]">
            이번 주
          </span>
        </div>
        <div className="flex flex-col gap-1.5">
          {TOP_LOCATIONS.map((loc) => (
            <div
              key={loc.rank}
              className="flex items-center gap-3 p-3 px-4 bg-[var(--color-bg-surface)] rounded-[14px] transition-colors hover:bg-[var(--color-bg-elevated)]"
            >
              <div
                className={`w-6 text-[1rem] font-bold text-center ${
                  RANK_COLORS[loc.rank] ?? 'text-[var(--color-text-secondary)]'
                }`}
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                {loc.rank}
              </div>
              <div className="flex-1">
                <p className="text-[0.8125rem] font-semibold text-[var(--color-text-base)] mb-1">
                  {loc.name}
                </p>
                <div className="h-1.5 bg-[var(--color-bg-elevated)] rounded-[3px] overflow-hidden">
                  <div
                    className="h-full rounded-[3px]"
                    style={{
                      width: `${loc.pct}%`,
                      background: 'linear-gradient(90deg, #ff4444, #ff8c00)',
                    }}
                  />
                </div>
              </div>
              <div
                className="text-[0.875rem] font-bold text-[var(--color-warning)] min-w-[50px] text-right"
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                {loc.count.toLocaleString()}건
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
