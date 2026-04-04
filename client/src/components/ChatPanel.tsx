/**
 * 전역 익명 채팅 바텀시트
 *
 * 전 세계 접속자 모두가 공유하는 하나의 방("chat:global").
 * Socket.IO 실시간 송수신 + 접속자 수 표시.
 * 메시지는 서버에서 1시간 TTL 로 Redis 에 보관되지만,
 * 본 UI 는 세션 중 수신된 메시지만 렌더링한다 (히스토리 로드 out-of-scope).
 */

import { useEffect, useMemo, useRef, useState } from 'react'
import { getOrCreateChatIdentity } from '../features/chat/identity'
import {
  sendChatMessage,
  useChat,
  useChatStore,
} from '../features/chat/useChat'

interface ChatPanelProps {
  visible: boolean
  onClose: () => void
}

function formatTime(ts: number): string {
  const d = new Date(ts * 1000)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
}

export function ChatPanel({ visible, onClose }: ChatPanelProps) {
  const [inputValue, setInputValue] = useState('')
  const identity = useMemo(() => getOrCreateChatIdentity(), [])
  const messages = useChatStore((s) => s.messages)
  const presenceCount = useChatStore((s) => s.presenceCount)
  const listRef = useRef<HTMLDivElement | null>(null)

  useChat(visible)

  // 새 메시지 도착 시 하단 자동 스크롤
  useEffect(() => {
    if (!visible) return
    const el = listRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [messages, visible])

  if (!visible) return null

  const handleSend = () => {
    const ok = sendChatMessage(inputValue)
    if (ok) setInputValue('')
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
      e.preventDefault()
      handleSend()
    }
  }

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
            <span className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">
              🌍 전국 공용 채팅
            </span>
          </div>
          <div className="flex items-center gap-1 text-[0.6875rem] text-[var(--color-text-secondary)]">
            <span className="w-1.5 h-1.5 bg-[var(--color-accent)] rounded-full" />
            {presenceCount}명 접속 중
          </div>
        </div>

        {/* TTL 안내 */}
        <div className="text-center text-[0.625rem] text-[var(--color-text-secondary)] py-1.5 flex items-center justify-center gap-1">
          <span>⏳</span>
          메시지는 1시간 후 자동 삭제됩니다
        </div>

        {/* 메시지 영역 */}
        <div
          ref={listRef}
          className="flex-1 overflow-y-auto px-4 py-3 flex flex-col gap-1.5"
        >
          {/* 시스템 메시지 */}
          <div className="flex items-center gap-2 justify-center py-2">
            <div className="flex-1 h-px bg-[rgba(255,255,255,0.06)]" />
            <span className="text-[0.6875rem] text-[var(--color-text-secondary)]">
              채팅방에 입장했습니다 — {identity.avatar} {identity.nickname}
            </span>
            <div className="flex-1 h-px bg-[rgba(255,255,255,0.06)]" />
          </div>

          {messages.map((msg) => {
            const isMe = msg.userId === identity.userId
            return (
              <div
                key={msg.id}
                className={`flex gap-2 items-end max-w-[85%] ${
                  isMe ? 'self-end flex-row-reverse' : 'self-start'
                }`}
              >
                {!isMe && (
                  <div
                    className="w-7 h-7 rounded-full flex items-center justify-center text-[0.75rem] flex-shrink-0 font-bold"
                    style={{ background: msg.avatarBg, color: msg.nameColor }}
                  >
                    {msg.avatar}
                  </div>
                )}
                <div>
                  {!isMe && (
                    <div
                      className="text-[0.6875rem] font-semibold mb-0.5"
                      style={{ color: msg.nameColor }}
                    >
                      {msg.nickname}
                    </div>
                  )}
                  <div
                    className={`px-3.5 py-2.5 text-[0.875rem] leading-[1.45] ${
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
            className="flex-1 px-4 py-2.5 bg-[var(--color-bg-elevated)] border border-transparent rounded-[22px] text-[var(--color-text-base)] text-[0.875rem] outline-none transition-colors placeholder:text-[#555] focus:border-[var(--color-accent)]"
            placeholder="메시지를 입력하세요..."
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            maxLength={300}
          />
          <button
            onClick={handleSend}
            className="w-10 h-10 rounded-full bg-[var(--color-accent)] flex items-center justify-center flex-shrink-0 transition-transform active:scale-90"
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
