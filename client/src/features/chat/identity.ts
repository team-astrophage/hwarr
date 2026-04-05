/**
 * 익명 채팅 아이덴티티
 *
 * 디바이스당 1회 동물 이모지 + 색상 조합을 뽑아 localStorage에 영속화한다.
 * 서버 계정 없이 "같은 사람"을 식별하기 위한 최소 수단.
 */

const STORAGE_KEY = 'hwarr:chat:identity'

export interface ChatIdentity {
  userId: string
  nickname: string
  avatar: string
  avatarBg: string
  nameColor: string
}

interface AnimalPreset {
  name: string
  avatar: string
  avatarBg: string
  nameColor: string
}

const ANIMALS: AnimalPreset[] = [
  { name: '여우', avatar: '🦊', avatarBg: '#3a2d5c', nameColor: '#a78bfa' },
  { name: '곰', avatar: '🐻', avatarBg: '#2d3a5c', nameColor: '#7bb8fa' },
  { name: '개구리', avatar: '🐸', avatarBg: '#3a5c2d', nameColor: '#7bfa90' },
  { name: '토끼', avatar: '🐰', avatarBg: '#5c2d3a', nameColor: '#fa7b9e' },
  { name: '고양이', avatar: '🐱', avatarBg: '#5c4a2d', nameColor: '#fac07b' },
  { name: '강아지', avatar: '🐶', avatarBg: '#3a4a2d', nameColor: '#c4fa7b' },
  { name: '판다', avatar: '🐼', avatarBg: '#333333', nameColor: '#e0e0e0' },
  { name: '호랑이', avatar: '🐯', avatarBg: '#5c3a1f', nameColor: '#ffa42b' },
  { name: '돼지', avatar: '🐷', avatarBg: '#5c2d4a', nameColor: '#fa9ec4' },
  { name: '원숭이', avatar: '🐵', avatarBg: '#4a3a2d', nameColor: '#d4a574' },
  { name: '사자', avatar: '🦁', avatarBg: '#5c4a2d', nameColor: '#ffd56b' },
  { name: '펭귄', avatar: '🐧', avatarBg: '#2d3a4a', nameColor: '#7bdffa' },
]

function genUserId(): string {
  // crypto.randomUUID 가 최신 브라우저에 존재
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `u-${Math.random().toString(36).slice(2, 10)}-${Date.now().toString(36)}`
}

function createIdentity(): ChatIdentity {
  const animal = ANIMALS[Math.floor(Math.random() * ANIMALS.length)]
  return {
    userId: genUserId(),
    nickname: `익명의 ${animal.name}`,
    avatar: animal.avatar,
    avatarBg: animal.avatarBg,
    nameColor: animal.nameColor,
  }
}

let cached: ChatIdentity | null = null

export function getChatIdentity(): ChatIdentity {
  if (cached) return cached

  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as ChatIdentity
      if (parsed && parsed.userId && parsed.avatar) {
        cached = parsed
        return cached
      }
    }
  } catch {
    // ignore parse errors — regenerate
  }

  const fresh = createIdentity()
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(fresh))
  } catch {
    // localStorage 사용 불가 환경 — 메모리에만 유지
  }
  cached = fresh
  return cached
}
