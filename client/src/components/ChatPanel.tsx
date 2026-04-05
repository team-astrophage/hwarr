/**
 * 채팅 바텀시트
 *
 * 글로벌 익명 채팅 (chat:global). 지도 위 바텀시트로 표시.
 * 메시지는 서버에서 1시간 TTL + 최대 100개 보관.
 */

import { useEffect, useMemo, useRef, useState } from 'react'
import { getChatIdentity } from '../features/chat/identity'
import { useChat, useSendChatMessage } from '../features/chat/useChat'
import { useChatStore } from '../features/chat/useChatStore'

interface ChatPanelProps {
  visible: boolean
  onClose: () => void
}

function formatTime(ts: number): string {
  const d = new Date(ts * 1000)
  const h = d.getHours().toString().padStart(2, '0')
  const m = d.getMinutes().toString().padStart(2, '0')
  return `${h}:${m}`
}

export function ChatPanel({ visible, onClose }: ChatPanelProps) {
  useChat(visible)
  const sendChatMessage = useSendChatMessage()

  const messages = useChatStore((s) => s.messages)
  const presenceCount = useChatStore((s) => s.presenceCount)
  const connectionStatus = useChatStore((s) => s.connectionStatus)

  const identity = useMemo(() => getChatIdentity(), [])
  const [inputValue, setInputValue] = useState('')
  const [isComposing, setIsComposing] = useState(false)
  const messagesRef = useRef<HTMLDivElement>(null)

  // 새 메시지 도착 시 최하단으로 자동 스크롤
  useEffect(() => {
    const el = messagesRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [messages.length])

  const connected = connectionStatus === 'connected'
  const canSend = connected && inputValue.trim().length > 0 && inputValue.length <= 300

  const handleSend = async () => {
    if (!canSend) return
    const text = inputValue
    setInputValue('')
    const ack = await sendChatMessage(text)
    if (ack?.error) {
      console.warn('[chat] send failed:', ack.error)
      // 전송 실패 시 입력값 복원
      setInputValue(text)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !isComposing) {
      e.preventDefault()
      handleSend()
    }
  }

  if (!visible) return null

  return (
    <div className="absolute bottom-0 left-0 right-0 z-[1100] flex flex-col max-h-[55vh]">
      {/* 바텀시트 */}
      <div className="bg-[var(--color-bg-surface)] rounded-t-[24px] shadow-[0_-4px_24px_rgba(0,0,0,0.5)] flex flex-col overflow-hidden flex-1">
        {/* 드래그 핸들 + 닫기 */}
        <div className="flex justify-center pt-2.5 pb-1.5 relative">
          <div className="w-9 h-1 bg-[#444] rounded-[2px]" />
          <button
            onClick={onClose}
            className="absolute right-3 top-1.5 w-7 h-7 flex items-center justify-center text-[var(--color-text-secondary)] hover:text-[var(--color-text-base)]"
          >
            ✕
          </button>
        </div>

        {/* 헤더 */}
        <div className="flex items-center justify-between px-5 pb-3 border-b border-[rgba(255,255,255,0.06)]">
          <div className="flex items-center gap-2">
            <span className="text-[0.625rem] font-bold text-[var(--color-warning)] bg-[rgba(255,140,0,0.12)] px-2.5 py-0.5 rounded-[var(--radius-pill)] uppercase tracking-[0.5px]">
              LIVE
            </span>
            <span className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">
              전국 불판 채팅
            </span>
          </div>
          <div className="flex items-center gap-1 text-[0.6875rem] text-[var(--color-text-secondary)]">
            {connected ? (
              <>
                <span className="w-1.5 h-1.5 bg-[var(--color-accent)] rounded-full" />
                {presenceCount}명 관전 중
              </>
            ) : (
              <>
                <span className="w-1.5 h-1.5 bg-[var(--color-warning)] rounded-full animate-pulse" />
                연결 중…
              </>
            )}
          </div>
        </div>

        {/* TTL 안내 */}
        <div className="text-center text-[0.625rem] text-[var(--color-text-secondary)] py-1.5 flex items-center justify-center gap-1">
          <span>⏳</span>
          메시지는 1시간 후 자동 삭제됩니다
        </div>

        {/* 메시지 영역 */}
        <div
          ref={messagesRef}
          className="flex-1 overflow-y-auto px-4 py-3 flex flex-col gap-1.5"
        >
          {/* 시스템 메시지 */}
          <div className="flex items-center gap-2 justify-center py-2">
            <div className="flex-1 h-px bg-[rgba(255,255,255,0.06)]" />
            <span className="text-[0.6875rem] text-[var(--color-text-secondary)]">
              {identity.nickname}(으)로 입장했습니다
            </span>
            <div className="flex-1 h-px bg-[rgba(255,255,255,0.06)]" />
          </div>

          {messages.map((msg) => {
            const isMe = msg.user_id === identity.userId
            return (
              <div
                key={msg.id}
                className={`flex gap-2 items-end max-w-[85%] ${
                  isMe ? 'self-end flex-row-reverse' : 'self-start'
                }`}
              >
                <div
                  className="w-7 h-7 rounded-full flex items-center justify-center text-[0.75rem] flex-shrink-0 font-bold"
                  style={{ background: msg.avatar_bg, color: msg.name_color }}
                >
                  {msg.avatar}
                </div>
                <div>
                  <div
                    className={`text-[0.6875rem] font-semibold mb-0.5 ${isMe ? 'text-right' : ''}`}
                    style={{ color: msg.name_color }}
                  >
                    {msg.nickname}{isMe ? ' (나)' : ''}
                  </div>
                  <div
                    className={`px-3.5 py-2.5 text-[0.875rem] leading-[1.45] break-words ${
                      isMe
                        ? 'bg-[var(--color-accent)] text-[#000] font-medium rounded-[18px] rounded-br-[4px]'
                        : 'bg-[var(--color-bg-card)] text-[var(--color-text-base)] rounded-[18px] rounded-bl-[4px]'
                    }`}
                  >
                    {msg.text}
                  </div>
                  <div
                    className={`text-[0.625rem] text-[var(--color-text-secondary)] mt-0.5 ${
                      isMe ? 'text-right' : ''
                    }`}
                  >
                    {formatTime(msg.timestamp)}
                  </div>
                </div>
              </div>
            )
          })}
        </div>

        {/* 입력 영역 */}
        <div className="flex gap-2 items-end px-4 pt-2.5 pb-7 border-t border-[rgba(255,255,255,0.06)]">
          <input
            className="flex-1 px-4 py-2.5 bg-[var(--color-bg-elevated)] border border-transparent rounded-[22px] text-[var(--color-text-base)] text-[0.875rem] outline-none transition-colors placeholder:text-[#555] focus:border-[var(--color-accent)] disabled:opacity-50"
            placeholder={connected ? '메시지를 입력하세요...' : '연결 중입니다...'}
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value.slice(0, 300))}
            onKeyDown={handleKeyDown}
            onCompositionStart={() => setIsComposing(true)}
            onCompositionEnd={() => setIsComposing(false)}
            maxLength={300}
            disabled={!connected}
          />
          <button
            onClick={handleSend}
            disabled={!canSend}
            className="w-10 h-10 rounded-full bg-[var(--color-accent)] flex items-center justify-center flex-shrink-0 transition-transform active:scale-90 disabled:opacity-40 disabled:active:scale-100"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="#000">
              <path d="M2 21l21-9L2 3v7l15 2-15 2v7z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  )
}
