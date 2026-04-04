/**
 * 익명 채팅 identity (동물 이모지 + 색상)
 *
 * 첫 진입 시 동물/색상 풀에서 랜덤 선택 후 localStorage 에 보관.
 * 이후 소켓 재접속에도 같은 닉네임이 유지된다.
 */

const STORAGE_KEY = 'hwarr:chat:identity'

interface AnimalDef {
  animal: string
  emoji: string
  bg: string
  color: string
}

const ANIMALS: AnimalDef[] = [
  { animal: '여우', emoji: '🦊', bg: '#3a2d5c', color: '#a78bfa' },
  { animal: '곰', emoji: '🐻', bg: '#2d3a5c', color: '#7bb8fa' },
  { animal: '개구리', emoji: '🐸', bg: '#3a5c2d', color: '#7bfa90' },
  { animal: '토끼', emoji: '🐰', bg: '#5c2d3a', color: '#fa7ba8' },
  { animal: '고양이', emoji: '🐱', bg: '#5c4a2d', color: '#fac97b' },
  { animal: '강아지', emoji: '🐶', bg: '#5c3a2d', color: '#faa37b' },
  { animal: '판다', emoji: '🐼', bg: '#3a3a3a', color: '#e0e0e0' },
  { animal: '호랑이', emoji: '🐯', bg: '#5c3d2d', color: '#ffb347' },
  { animal: '돼지', emoji: '🐷', bg: '#5c2d4f', color: '#ff9ec7' },
  { animal: '원숭이', emoji: '🐵', bg: '#4a3a2d', color: '#d4a574' },
  { animal: '사자', emoji: '🦁', bg: '#5c4d2d', color: '#ffd966' },
  { animal: '펭귄', emoji: '🐧', bg: '#2d3d4a', color: '#89c4e8' },
]

export interface ChatIdentity {
  userId: string
  nickname: string
  avatar: string
  avatarBg: string
  nameColor: string
}

function randomUuid(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `u-${Math.random().toString(36).slice(2)}-${Date.now().toString(36)}`
}

function createIdentity(): ChatIdentity {
  const def = ANIMALS[Math.floor(Math.random() * ANIMALS.length)]
  return {
    userId: randomUuid(),
    nickname: `익명의 ${def.animal}`,
    avatar: def.emoji,
    avatarBg: def.bg,
    nameColor: def.color,
  }
}

let cached: ChatIdentity | null = null

export function getOrCreateChatIdentity(): ChatIdentity {
  if (cached) return cached
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as ChatIdentity
      if (parsed?.userId && parsed?.nickname && parsed?.avatar) {
        cached = parsed
        return parsed
      }
    }
  } catch {
    // ignore, fall through to create
  }
  const fresh = createIdentity()
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(fresh))
  } catch {
    // localStorage unavailable — still return in-memory identity
  }
  cached = fresh
  return fresh
}
