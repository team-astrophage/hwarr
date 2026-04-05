/**
 * 성냥 던지기 & 화염방사기 애니메이션 상태 (Zustand)
 *
 * - matchThrow: 탭 시 성냥 포물선 애니메이션 트리거
 * - flamethrower: long press 시 스트림형 파티클 분사 상태
 */

import { create } from 'zustand'

interface MatchThrow {
  id: number
  /** 시작 시각 (performance.now) */
  startTime: number
  /** 대상 격자 ID */
  gridId: string
}

interface Explosion {
  id: number
  startTime: number
  gridId: string
}

/** 랜덤 폭발 간격 최소 (탭 횟수) */
const RANDOM_EXPLOSION_INTERVAL_MIN = 20
/** 랜덤 폭발 간격 최대 (탭 횟수) */
const RANDOM_EXPLOSION_INTERVAL_MAX = 30

interface AnimationState {
  /** 활성 성냥 애니메이션 목록 */
  matches: MatchThrow[]
  /** 화염방사기 활성 여부 */
  flamethrowerActive: boolean
  /** 대상 격자 ID */
  targetGridId: string | null
  /** 전소 폭발 이펙트 목록 */
  explosions: Explosion[]
  /** 전소 달성한 격자 추적 (중복 방지) */
  explodedGrids: Set<string>
  /** 5단계(전소) 달성 이후 누적 탭 카운트 */
  postExplodeTapCount: number
  /** 다음 랜덤 폭발이 발동될 탭 카운트 (postExplodeTapCount 기준) */
  nextRandomExplosionAt: number

  throwMatch: (gridId: string) => void
  removeMatch: (id: number) => void
  startFlamethrower: (gridId: string) => void
  stopFlamethrower: () => void
  triggerExplosion: (gridId: string) => void
  removeExplosion: (id: number) => void
}

const pickNextRandomInterval = () =>
  RANDOM_EXPLOSION_INTERVAL_MIN +
  Math.floor(
    Math.random() *
      (RANDOM_EXPLOSION_INTERVAL_MAX - RANDOM_EXPLOSION_INTERVAL_MIN + 1),
  )

let matchIdCounter = 0

export const useAnimationStore = create<AnimationState>((set) => ({
  matches: [],
  flamethrowerActive: false,
  targetGridId: null,
  explosions: [],
  explodedGrids: new Set(),
  postExplodeTapCount: 0,
  nextRandomExplosionAt: pickNextRandomInterval(),

  throwMatch: (gridId) =>
    set((state) => {
      const nextMatches = [
        ...state.matches,
        {
          id: ++matchIdCounter,
          startTime: performance.now(),
          gridId,
        },
      ]

      // 5단계(전소) 달성 이전에는 기본 동작만
      if (state.explodedGrids.size === 0) {
        return { matches: nextMatches }
      }

      // 5단계 이후: 탭 20~30회 랜덤 간격으로 폭발 이펙트 트리거
      const nextCount = state.postExplodeTapCount + 1
      if (nextCount >= state.nextRandomExplosionAt) {
        return {
          matches: nextMatches,
          postExplodeTapCount: 0,
          nextRandomExplosionAt: pickNextRandomInterval(),
          explosions: [
            ...state.explosions,
            {
              id: ++matchIdCounter,
              startTime: performance.now(),
              gridId,
            },
          ],
        }
      }

      return {
        matches: nextMatches,
        postExplodeTapCount: nextCount,
      }
    }),

  removeMatch: (id) =>
    set((state) => ({
      matches: state.matches.filter((m) => m.id !== id),
    })),

  startFlamethrower: (gridId) =>
    set({ flamethrowerActive: true, targetGridId: gridId }),

  stopFlamethrower: () =>
    set({ flamethrowerActive: false }),

  triggerExplosion: (gridId) =>
    set((state) => {
      if (state.explodedGrids.has(gridId)) return state
      const next = new Set(state.explodedGrids)
      next.add(gridId)
      return {
        explodedGrids: next,
        explosions: [
          ...state.explosions,
          {
            id: ++matchIdCounter,
            startTime: performance.now(),
            gridId,
          },
        ],
      }
    }),

  removeExplosion: (id) =>
    set((state) => ({
      explosions: state.explosions.filter((e) => e.id !== id),
    })),
}))
