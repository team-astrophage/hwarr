/**
 * 채팅 구독 훅
 *
 * active=true 일 때만 채팅방에 join + 리스너 등록. 해제 시 chat:leave + listener off.
 * 소켓 싱글톤(`lib/socket.ts`)을 공유하되, 채팅 전용 lifecycle 만 이 훅에서 관리한다.
 */

import { useCallback, useEffect } from 'react'
import { socket } from '../../lib/socket'
import { getChatIdentity } from './identity'
import type { ChatMessage } from './useChatStore'
import { useChatStore } from './useChatStore'

interface ChatJoinAck {
  status?: string
  history?: ChatMessage[]
  presence?: number
  error?: string
}

interface ChatSendAck {
  status?: string
  id?: string
  error?: string
}

export function useChat(active: boolean) {
  const appendMessage = useChatStore((s) => s.appendMessage)
  const setHistory = useChatStore((s) => s.setHistory)
  const setPresence = useChatStore((s) => s.setPresence)
  const setStatus = useChatStore((s) => s.setStatus)
  const reset = useChatStore((s) => s.reset)

  useEffect(() => {
    if (!active) return

    const emitJoin = () => {
      socket.emit('chat:join', {}, (ack: ChatJoinAck) => {
        if (ack?.error) {
          console.warn('[chat] join failed:', ack.error)
          return
        }
        if (Array.isArray(ack?.history)) setHistory(ack.history)
        if (typeof ack?.presence === 'number') setPresence(ack.presence)
      })
    }

    const onConnect = () => {
      setStatus('connected')
      emitJoin()
    }
    const onDisconnect = () => setStatus('connecting')
    const onConnectError = () => setStatus('connecting')
    const onMessage = (msg: ChatMessage) => appendMessage(msg)
    const onPresence = (data: { count: number }) => setPresence(data.count)

    socket.on('connect', onConnect)
    socket.on('disconnect', onDisconnect)
    socket.on('connect_error', onConnectError)
    socket.on('chat:message', onMessage)
    socket.on('chat:presence', onPresence)

    if (socket.connected) {
      setStatus('connected')
      emitJoin()
    } else {
      setStatus('connecting')
      socket.connect()
    }

    return () => {
      if (socket.connected) {
        socket.emit('chat:leave', {})
      }
      socket.off('connect', onConnect)
      socket.off('disconnect', onDisconnect)
      socket.off('connect_error', onConnectError)
      socket.off('chat:message', onMessage)
      socket.off('chat:presence', onPresence)
      reset()
    }
  }, [active, appendMessage, setHistory, setPresence, setStatus, reset])
}

export function useSendChatMessage() {
  return useCallback((text: string): Promise<ChatSendAck> => {
    const trimmed = text.trim()
    if (trimmed.length === 0) {
      return Promise.resolve({ error: 'text_too_short' })
    }
    if (trimmed.length > 300) {
      return Promise.resolve({ error: 'text_too_long' })
    }
    const identity = getChatIdentity()
    return new Promise((resolve) => {
      socket.emit(
        'chat:send',
        {
          text: trimmed,
          user_id: identity.userId,
          nickname: identity.nickname,
          avatar: identity.avatar,
          avatar_bg: identity.avatarBg,
          name_color: identity.nameColor,
        },
        (ack: ChatSendAck) => resolve(ack ?? {}),
      )
    })
  }, [])
}
