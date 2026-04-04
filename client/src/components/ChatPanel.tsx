/**
 * 채팅 바텀시트 Mock UI
 *
 * 격자 기반 휘발성 익명 채팅. 지도 위 바텀시트로 표시.
 * Mock 데이터 하드코딩, 실제 백엔드 연동은 후순위.
 * 24시간 후 자동 삭제 안내 포함.
 */

import { useState } from 'react'

interface ChatMessage {
  id: number
  nickname: string
  avatar: string
  avatarBg: string
  nameColor: string
  text: string
  time: string
  isMe: boolean
}

const MOCK_MESSAGES: ChatMessage[] = [
  {
    id: 1,
    nickname: '익명의 여우',
    avatar: '🦊',
    avatarBg: '#3a2d5c',
    nameColor: '#a78bfa',
    text: '여기 이미 불바다네 ㅋㅋㅋ',
    time: '14:23',
    isMe: false,
  },
  {
    id: 2,
    nickname: '익명의 곰',
    avatar: '🐻',
    avatarBg: '#2d3a5c',
    nameColor: '#7bb8fa',
    text: '테헤란로 전소 가보자',
    time: '14:23',
    isMe: false,
  },
  {
    id: 3,
    nickname: '',
    avatar: '',
    avatarBg: '',
    nameColor: '',
    text: 'ㅋㅋㅋ 우리 회사 불태운다',
    time: '14:24',
    isMe: true,
  },
  {
    id: 4,
    nickname: '익명의 개구리',
    avatar: '🐸',
    avatarBg: '#3a5c2d',
    nameColor: '#7bfa90',
    text: '방금 3단계 됐다!!! 화재!!',
    time: '14:24',
    isMe: false,
  },
  {
    id: 5,
    nickname: '익명의 여우',
    avatar: '🦊',
    avatarBg: '#3a2d5c',
    nameColor: '#a78bfa',
    text: '전소 달성하면 뭔가 터지나?',
    time: '14:25',
    isMe: false,
  },
]

interface ChatPanelProps {
  visible: boolean
  onClose: () => void
}

export function ChatPanel({ visible, onClose }: ChatPanelProps) {
  const [inputValue, setInputValue] = useState('')

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
              3단계
            </span>
            <span className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">
              강남 테헤란로
            </span>
          </div>
          <div className="flex items-center gap-1 text-[0.6875rem] text-[var(--color-text-secondary)]">
            <span className="w-1.5 h-1.5 bg-[var(--color-accent)] rounded-full" />
            12명 관전 중
          </div>
        </div>

        {/* TTL 안내 */}
        <div className="text-center text-[0.625rem] text-[var(--color-text-secondary)] py-1.5 flex items-center justify-center gap-1">
          <span>⏳</span>
          메시지는 24시간 후 자동 삭제됩니다
        </div>

        {/* 메시지 영역 */}
        <div className="flex-1 overflow-y-auto px-4 py-3 flex flex-col gap-1.5">
          {/* 시스템 메시지 */}
          <div className="flex items-center gap-2 justify-center py-2">
            <div className="flex-1 h-px bg-[rgba(255,255,255,0.06)]" />
            <span className="text-[0.6875rem] text-[var(--color-text-secondary)]">
              이 지역 채팅방에 입장했습니다
            </span>
            <div className="flex-1 h-px bg-[rgba(255,255,255,0.06)]" />
          </div>

          {MOCK_MESSAGES.map((msg) => (
            <div
              key={msg.id}
              className={`flex gap-2 items-end max-w-[85%] ${
                msg.isMe ? 'self-end flex-row-reverse' : 'self-start'
              }`}
            >
              {!msg.isMe && (
                <div
                  className="w-7 h-7 rounded-full flex items-center justify-center text-[0.75rem] flex-shrink-0 font-bold"
                  style={{ background: msg.avatarBg, color: msg.nameColor }}
                >
                  {msg.avatar}
                </div>
              )}
              <div>
                {!msg.isMe && (
                  <div
                    className="text-[0.6875rem] font-semibold mb-0.5"
                    style={{ color: msg.nameColor }}
                  >
                    {msg.nickname}
                  </div>
                )}
                <div
                  className={`px-3.5 py-2.5 text-[0.875rem] leading-[1.45] ${
                    msg.isMe
                      ? 'bg-[var(--color-accent)] text-[#000] font-medium rounded-[18px] rounded-br-[4px]'
                      : 'bg-[var(--color-bg-card)] text-[var(--color-text-base)] rounded-[18px] rounded-bl-[4px]'
                  }`}
                >
                  {msg.text}
                </div>
                <div
                  className={`text-[0.625rem] text-[var(--color-text-secondary)] mt-0.5 ${
                    msg.isMe ? 'text-right' : ''
                  }`}
                >
                  {msg.time}
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* 입력 영역 */}
        <div className="flex gap-2 items-end px-4 pt-2.5 pb-7 border-t border-[rgba(255,255,255,0.06)]">
          <input
            className="flex-1 px-4 py-2.5 bg-[var(--color-bg-elevated)] border border-transparent rounded-[22px] text-[var(--color-text-base)] text-[0.875rem] outline-none transition-colors placeholder:text-[#555] focus:border-[var(--color-accent)]"
            placeholder="메시지를 입력하세요..."
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
          />
          <button className="w-10 h-10 rounded-full bg-[var(--color-accent)] flex items-center justify-center flex-shrink-0 transition-transform active:scale-90">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="#000">
              <path d="M2 21l21-9L2 3v7l15 2-15 2v7z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  )
}
