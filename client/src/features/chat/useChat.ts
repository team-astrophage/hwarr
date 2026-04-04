/**
 * 전역 익명 채팅 훅 + Zustand 스토어
 *
 * - 채팅 패널 활성화 시 chat:join → presence count 수신
 * - chat:message / chat:presence 리스너 등록 (재접속 시 자동 재조인)
 * - sendChatMessage() 로 chat:send emit, 오프라인이면 낙관적 큐잉
 * - 언마운트 시 chat:leave 및 리스너 정리
 */

import { useEffect } from 'react'
import { create } from 'zustand'
import { socket } from '../../lib/socket'
import { getOrCreateChatIdentity } from './identity'

export type ChatMessageStatus = 'sent' | 'pending' | 'failed'

export interface ChatMessage {
  id: string
  userId: string
  nickname: string
  avatar: string
  avatarBg: string
  nameColor: string
  text: string
  timestamp: number
  status?: ChatMessageStatus
}

interface ChatState {
  messages: ChatMessage[]
  presenceCount: number
  /** 아직 서버로 송신 확정되지 않은 메시지의 클라이언트 측 id 집합 */
  pendingIds: string[]
  appendMessage: (msg: ChatMessage) => void
  /** 서버 echo 가 도착했을 때 동일 내용의 pending 메시지를 실 메시지로 교체 */
  replacePending: (incoming: ChatMessage) => void
  markFailed: (id: string) => void
  removeMessage: (id: string) => void
  setPresence: (count: number) => void
  addPendingId: (id: string) => void
  removePendingId: (id: string) => void
  reset: () => void
}

const MAX_MESSAGES = 200

export const useChatStore = create<ChatState>((set) => ({
  messages: [],
  presenceCount: 0,
  pendingIds: [],
  appendMessage: (msg) =>
    set((state) => {
      const next = state.messages.concat(msg)
      if (next.length > MAX_MESSAGES) next.splice(0, next.length - MAX_MESSAGES)
      return { messages: next }
    }),
  replacePending: (incoming) =>
    set((state) => {
      // 같은 유저가 보낸 동일 텍스트의 첫 pending 메시지를 서버 echo 로 교체
      const idx = state.messages.findIndex(
        (m) =>
          m.status === 'pending' &&
          m.userId === incoming.userId &&
          m.text === incoming.text,
      )
      if (idx === -1) {
        const next = state.messages.concat(incoming)
        if (next.length > MAX_MESSAGES)
          next.splice(0, next.length - MAX_MESSAGES)
        return { messages: next }
      }
      const replaced = state.messages.slice()
      const oldId = replaced[idx].id
      replaced[idx] = incoming
      return {
        messages: replaced,
        pendingIds: state.pendingIds.filter((pid) => pid !== oldId),
      }
    }),
  markFailed: (id) =>
    set((state) => ({
      messages: state.messages.map((m) =>
        m.id === id ? { ...m, status: 'failed' } : m,
      ),
    })),
  removeMessage: (id) =>
    set((state) => ({
      messages: state.messages.filter((m) => m.id !== id),
      pendingIds: state.pendingIds.filter((pid) => pid !== id),
    })),
  setPresence: (count) => set({ presenceCount: count }),
  addPendingId: (id) =>
    set((state) => ({ pendingIds: state.pendingIds.concat(id) })),
  removePendingId: (id) =>
    set((state) => ({
      pendingIds: state.pendingIds.filter((pid) => pid !== id),
    })),
  reset: () => set({ messages: [], presenceCount: 0, pendingIds: [] }),
}))

// Server wire format (snake_case) → client ChatMessage
interface ServerChatMessage {
  id: string
  user_id: string
  nickname: string
  avatar: string
  avatar_bg: string
  name_color: string
  text: string
  timestamp: number
}

function fromServer(m: ServerChatMessage): ChatMessage {
  return {
    id: m.id,
    userId: m.user_id,
    nickname: m.nickname,
    avatar: m.avatar,
    avatarBg: m.avatar_bg,
    nameColor: m.name_color,
    text: m.text,
    timestamp: m.timestamp,
    status: 'sent',
  }
}

function clientId(): string {
  const rand =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID().slice(0, 12)
      : Math.random().toString(36).slice(2, 14)
  return `pending-${rand}`
}

/** 오프라인 큐를 순차적으로 서버에 emit 한다 (연결 복구 직후 호출). */
function flushPendingQueue() {
  if (!socket.connected) return
  const { pendingIds, messages } = useChatStore.getState()
  if (pendingIds.length === 0) return
  const identity = getOrCreateChatIdentity()
  for (const pid of pendingIds) {
    const msg = messages.find((m) => m.id === pid)
    if (!msg) continue
    socket.emit('chat:send', {
      text: msg.text,
      user_id: identity.userId,
      nickname: identity.nickname,
      avatar: identity.avatar,
      avatar_bg: identity.avatarBg,
      name_color: identity.nameColor,
    })
  }
}

export function useChat(active: boolean) {
  const replacePending = useChatStore((s) => s.replacePending)
  const setPresence = useChatStore((s) => s.setPresence)

  useEffect(() => {
    if (!active) return

    const onMessage = (data: ServerChatMessage) => {
      replacePending(fromServer(data))
    }
    const onPresence = (data: { count: number }) => {
      setPresence(data.count)
    }
    const joinRoom = () => {
      socket.emit(
        'chat:join',
        {},
        (ack: { status: string; count?: number } | undefined) => {
          if (ack?.count != null) setPresence(ack.count)
        },
      )
      // 방 복귀 직후 오프라인 큐 전송
      flushPendingQueue()
    }

    socket.on('chat:message', onMessage)
    socket.on('chat:presence', onPresence)
    // 초기 및 재접속 시 자동 재조인
    if (socket.connected) joinRoom()
    socket.on('connect', joinRoom)

    return () => {
      socket.off('chat:message', onMessage)
      socket.off('chat:presence', onPresence)
      socket.off('connect', joinRoom)
      if (socket.connected) {
        socket.emit('chat:leave', {})
      }
    }
  }, [active, replacePending, setPresence])
}

/**
 * 채팅 메시지를 전송한다.
 * - 연결됨: 즉시 emit (서버 echo 로 UI 에 반영됨)
 * - 끊김: pending 메시지로 낙관적 append + 큐에 적재, 복구 시 자동 전송
 *
 * 반환값:
 * - true: 전송 또는 큐잉 성공 (입력창 비우기 가능)
 * - false: 유효성 검증 실패
 */
export function sendChatMessage(text: string): boolean {
  const trimmed = text.trim()
  if (trimmed.length < 1 || trimmed.length > 300) return false

  const identity = getOrCreateChatIdentity()
  const store = useChatStore.getState()

  if (socket.connected) {
    socket.emit('chat:send', {
      text: trimmed,
      user_id: identity.userId,
      nickname: identity.nickname,
      avatar: identity.avatar,
      avatar_bg: identity.avatarBg,
      name_color: identity.nameColor,
    })
    return true
  }

  // 오프라인: 낙관적으로 append + 큐잉
  const id = clientId()
  store.appendMessage({
    id,
    userId: identity.userId,
    nickname: identity.nickname,
    avatar: identity.avatar,
    avatarBg: identity.avatarBg,
    nameColor: identity.nameColor,
    text: trimmed,
    timestamp: Date.now() / 1000,
    status: 'pending',
  })
  store.addPendingId(id)
  return true
}
