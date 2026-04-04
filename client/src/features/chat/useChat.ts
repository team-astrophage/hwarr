/**
 * 전역 익명 채팅 훅 + Zustand 스토어
 *
 * - 마운트 시 chat:join → presence count 수신
 * - chat:message / chat:presence 리스너 등록
 * - sendMessage() 로 chat:send emit
 * - 언마운트 시 chat:leave 및 리스너 정리
 */

import { useEffect } from 'react'
import { create } from 'zustand'
import { socket } from '../../lib/socket'
import { getOrCreateChatIdentity } from './identity'

export interface ChatMessage {
  id: string
  userId: string
  nickname: string
  avatar: string
  avatarBg: string
  nameColor: string
  text: string
  timestamp: number
}

interface ChatState {
  messages: ChatMessage[]
  presenceCount: number
  appendMessage: (msg: ChatMessage) => void
  setPresence: (count: number) => void
  reset: () => void
}

const MAX_MESSAGES = 200

export const useChatStore = create<ChatState>((set) => ({
  messages: [],
  presenceCount: 0,
  appendMessage: (msg) =>
    set((state) => {
      const next = state.messages.concat(msg)
      if (next.length > MAX_MESSAGES) next.splice(0, next.length - MAX_MESSAGES)
      return { messages: next }
    }),
  setPresence: (count) => set({ presenceCount: count }),
  reset: () => set({ messages: [], presenceCount: 0 }),
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
  }
}

export function useChat(active: boolean) {
  const appendMessage = useChatStore((s) => s.appendMessage)
  const setPresence = useChatStore((s) => s.setPresence)

  useEffect(() => {
    if (!active) return

    const onMessage = (data: ServerChatMessage) => {
      appendMessage(fromServer(data))
    }
    const onPresence = (data: { count: number }) => {
      setPresence(data.count)
    }

    socket.on('chat:message', onMessage)
    socket.on('chat:presence', onPresence)

    const joinRoom = () => {
      socket.emit(
        'chat:join',
        {},
        (ack: { status: string; count?: number } | undefined) => {
          if (ack?.count != null) setPresence(ack.count)
        },
      )
    }

    if (socket.connected) {
      joinRoom()
    } else {
      socket.once('connect', joinRoom)
    }

    return () => {
      socket.off('chat:message', onMessage)
      socket.off('chat:presence', onPresence)
      socket.off('connect', joinRoom)
      if (socket.connected) {
        socket.emit('chat:leave', {})
      }
    }
  }, [active, appendMessage, setPresence])
}

export function sendChatMessage(text: string): boolean {
  const trimmed = text.trim()
  if (trimmed.length < 1 || trimmed.length > 300) return false
  if (!socket.connected) return false
  const identity = getOrCreateChatIdentity()
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
