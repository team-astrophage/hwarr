/**
 * 불 상태 관리 (Zustand)
 *
 * 선택 이유:
 * - Map<gridId, FireCell> 구조: 격자 ID로 O(1) 조회
 * - Socket 이벤트 → Zustand 스토어 → React 컴포넌트 리렌더
 *   (컴포넌트가 직접 소켓을 구독하지 않음 → 관심사 분리)
 * - onlineUsers: 서버 소켓 접속자 수 (방화범 카운트)
 */

import { create } from 'zustand'
import { useAnimationStore } from './animationStore'

export interface FireCell {
  gridId: string
  activeCount: number
  stage: number // 0~5
}

interface FireState {
  fires: Map<string, FireCell>
  onlineUsers: number

  updateFire: (cell: FireCell) => void
  syncFires: (cells: FireCell[]) => void
  setOnlineUsers: (count: number) => void
}

export const useFireStore = create<FireState>((set) => ({
  fires: new Map(),
  onlineUsers: 0,

  updateFire: (cell) =>
    set((state) => {
      const next = new Map(state.fires)
      if (cell.activeCount > 0) {
        next.set(cell.gridId, cell)
        // 5단계(전소) 최초 도달 시 폭발 트리거
        if (cell.stage >= 5) {
          useAnimationStore.getState().triggerExplosion(cell.gridId)
        }
      } else {
        next.delete(cell.gridId)
      }
      return { fires: next }
    }),

  syncFires: (cells) =>
    set(() => {
      const next = new Map<string, FireCell>()
      for (const cell of cells) {
        if (cell.activeCount > 0) {
          next.set(cell.gridId, cell)
        }
      }
      return { fires: next }
    }),

  setOnlineUsers: (count) => set({ onlineUsers: count }),
}))
