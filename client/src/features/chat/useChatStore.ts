/**
 * 채팅 상태 저장소 (Zustand)
 *
 * 소켓 이벤트 → 스토어 → ChatPanel 렌더 파이프라인의 중간 계층.
 * messages 는 최대 MESSAGE_CAP 개까지만 유지 (오래된 것부터 drop).
 */

import { create } from 'zustand'

const MESSAGE_CAP = 200

export interface ChatMessage {
  id: string
  user_id: string
  nickname: string
  avatar: string
  avatar_bg: string
  name_color: string
  text: string
  timestamp: number
}

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected'

interface ChatState {
  messages: ChatMessage[]
  presenceCount: number
  connectionStatus: ConnectionStatus

  appendMessage: (msg: ChatMessage) => void
  setHistory: (msgs: ChatMessage[]) => void
  setPresence: (count: number) => void
  setStatus: (status: ConnectionStatus) => void
  reset: () => void
}

export const useChatStore = create<ChatState>((set) => ({
  messages: [],
  presenceCount: 0,
  connectionStatus: 'disconnected',

  appendMessage: (msg) =>
    set((state) => {
      // 동일 id 중복 방지 (자기 메시지 에코 + 서버 브로드캐스트 중복 대응)
      if (state.messages.some((m) => m.id === msg.id)) return state
      const next = [...state.messages, msg]
      if (next.length > MESSAGE_CAP) next.splice(0, next.length - MESSAGE_CAP)
      return { messages: next }
    }),

  setHistory: (msgs) => set({ messages: msgs.slice(-MESSAGE_CAP) }),

  setPresence: (count) => set({ presenceCount: count }),

  setStatus: (status) => set({ connectionStatus: status }),

  reset: () => set({ messages: [], presenceCount: 0 }),
}))
