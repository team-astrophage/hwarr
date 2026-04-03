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

interface AnimationState {
  /** 활성 성냥 애니메이션 목록 */
  matches: MatchThrow[]
  /** 화염방사기 활성 여부 */
  flamethrowerActive: boolean
  /** 대상 격자 ID */
  targetGridId: string | null

  throwMatch: (gridId: string) => void
  removeMatch: (id: number) => void
  startFlamethrower: (gridId: string) => void
  stopFlamethrower: () => void
}

let matchIdCounter = 0

export const useAnimationStore = create<AnimationState>((set) => ({
  matches: [],
  flamethrowerActive: false,
  targetGridId: null,

  throwMatch: (gridId) =>
    set((state) => ({
      matches: [
        ...state.matches,
        {
          id: ++matchIdCounter,
          startTime: performance.now(),
          gridId,
        },
      ],
    })),

  removeMatch: (id) =>
    set((state) => ({
      matches: state.matches.filter((m) => m.id !== id),
    })),

  startFlamethrower: (gridId) =>
    set({ flamethrowerActive: true, targetGridId: gridId }),

  stopFlamethrower: () =>
    set({ flamethrowerActive: false }),
}))
