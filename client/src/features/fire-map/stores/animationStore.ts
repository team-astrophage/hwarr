/**
 * 성냥 던지기 애니메이션 상태 (Zustand)
 *
 * - matchThrow: 탭 시 성냥 포물선 애니메이션 트리거
 * - explosions: 전소 도달 시 폭발 이펙트
 * - trajectories: 500 하드캡 초과 시 이웃 그리드로 번지는 궤적
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

interface SpreadTrajectory {
  id: number
  startTime: number
  /** 소스 격자 ID (불이 출발한 곳) */
  fromGridId: string
  /** 타겟 격자 ID (불이 착륙한 곳) */
  toGridId: string
  /** 격자 중심 위도/경도 (서버가 내려준 값) */
  fromLat: number
  fromLng: number
  toLat: number
  toLng: number
}

/** 랜덤 폭발 간격 최소 (탭 횟수) */
const RANDOM_EXPLOSION_INTERVAL_MIN = 20
/** 랜덤 폭발 간격 최대 (탭 횟수) */
const RANDOM_EXPLOSION_INTERVAL_MAX = 30

interface AnimationState {
  /** 활성 성냥 애니메이션 목록 */
  matches: MatchThrow[]
  /** 전소 폭발 이펙트 목록 */
  explosions: Explosion[]
  /** 전소 달성한 격자 추적 (중복 방지) */
  explodedGrids: Set<string>
  /** 5단계(전소) 달성 이후 누적 탭 카운트 */
  postExplodeTapCount: number
  /** 다음 랜덤 폭발이 발동될 탭 카운트 (postExplodeTapCount 기준) */
  nextRandomExplosionAt: number
  /** 불 확산 궤적 애니메이션 목록 */
  trajectories: SpreadTrajectory[]

  throwMatch: (gridId: string) => void
  removeMatch: (id: number) => void
  triggerExplosion: (gridId: string) => void
  removeExplosion: (id: number) => void
  addTrajectory: (
    fromGridId: string,
    toGridId: string,
    fromLat: number,
    fromLng: number,
    toLat: number,
    toLng: number,
  ) => void
  removeTrajectory: (id: number) => void
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
  explosions: [],
  explodedGrids: new Set(),
  postExplodeTapCount: 0,
  nextRandomExplosionAt: pickNextRandomInterval(),
  trajectories: [],

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

  addTrajectory: (fromGridId, toGridId, fromLat, fromLng, toLat, toLng) =>
    set((state) => ({
      trajectories: [
        ...state.trajectories,
        {
          id: ++matchIdCounter,
          startTime: performance.now(),
          fromGridId,
          toGridId,
          fromLat,
          fromLng,
          toLat,
          toLng,
        },
      ],
    })),

  removeTrajectory: (id) =>
    set((state) => ({
      trajectories: state.trajectories.filter((t) => t.id !== id),
    })),
}))
