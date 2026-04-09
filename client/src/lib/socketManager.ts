/**
 * Socket.IO 연결 수명주기 관리
 *
 * 책임:
 * - 앱 전역에서 공유되는 단일 소켓 인스턴스 제공
 * - 서버 heartbeat 송신 (10초 간격, 서버 reaper timeout=30s)
 * - 연결/재접속/오프라인 상태를 Zustand 스토어로 노출
 * - 서버발 `io server disconnect` 시 수동 재접속 (Socket.IO 기본 동작 보정)
 *
 * 사용:
 * - 앱 진입 시 SocketProvider 가 `startSocket()` 을 1회 호출
 * - 기능별 훅(useFireSocket 등)은 리스너만 등록/해제하고
 *   소켓 자체의 connect/disconnect 는 건드리지 않음
 */

import { io, type Socket } from 'socket.io-client'
import { create } from 'zustand'
import { API_URL, SOCKET_URL, TTL_SECONDS } from './config'

export type ConnectionStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'offline'

interface SocketState {
  status: ConnectionStatus
  reconnectAttempt: number
  setStatus: (s: ConnectionStatus) => void
  setAttempt: (n: number) => void
}

export const useSocketStore = create<SocketState>((set) => ({
  status: 'idle',
  reconnectAttempt: 0,
  setStatus: (status) => set({ status }),
  setAttempt: (n) => set({ reconnectAttempt: n }),
}))

// 서버 HEARTBEAT_INTERVAL_SEC=10, TIMEOUT=30 에 맞춤
const HEARTBEAT_MS = 10_000
const MAX_RECONNECT_ATTEMPTS = 8
const USER_ID_STORAGE_KEY = 'hwarr:anonUserId'
const TOKEN_ERROR_RE = /token|expired|invalid|authentication/i

/**
 * Fetch an HMAC-signed auth token and server-assigned user_id from the API.
 * The token is short-lived and validated by the server on Socket.IO connect.
 */
async function fetchToken(): Promise<{ token: string; user_id: string }> {
  const res = await fetch(`${API_URL}/api/token`)
  if (!res.ok) throw new Error(`Token fetch failed: ${res.status}`)
  return res.json()
}

// Token cache: reuse the same token (and userID) across reconnections
// until it approaches expiry. Preserves userID for server-side session
// restoration via GetPreviousSession(userID).
const TOKEN_REFRESH_MARGIN_MS = 60_000

let cachedToken: {
  token: string
  user_id: string
  expiresAt: number
} | null = null

function isTokenFresh(): boolean {
  return (
    cachedToken !== null &&
    Date.now() < cachedToken.expiresAt - TOKEN_REFRESH_MARGIN_MS
  )
}

async function getToken(): Promise<{
  token: string
  user_id: string
} | null> {
  if (isTokenFresh()) return cachedToken!

  for (let i = 0; i < 2; i++) {
    try {
      const { token, user_id } = await fetchToken()
      cachedToken = {
        token,
        user_id,
        expiresAt: Date.now() + TTL_SECONDS * 1000,
      }
      localStorage.setItem(USER_ID_STORAGE_KEY, user_id)
      return cachedToken
    } catch (err) {
      console.error(`[Socket] Token fetch attempt ${i + 1} failed`, err)
      if (i === 0) await new Promise((r) => setTimeout(r, 1_000))
    }
  }
  return null
}

export function clearTokenCache(): void {
  cachedToken = null
}

export const socket: Socket = io(SOCKET_URL, {
  autoConnect: false,
  transports: ['polling', 'websocket'],
  reconnection: true,
  reconnectionAttempts: MAX_RECONNECT_ATTEMPTS,
  reconnectionDelay: 500,
  reconnectionDelayMax: 5_000,
  auth: async (cb) => {
    const authData = await getToken()
    if (authData) {
      cb({ user_id: authData.user_id, token: authData.token })
    } else {
      cb({})
    }
  },
})

let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let started = false

function startHeartbeat() {
  if (heartbeatTimer) return
  // 첫 박동을 즉시 보내 서버 last_heartbeat 를 connect 직후 갱신
  socket.emit('heartbeat', { ts: Date.now() })
  heartbeatTimer = setInterval(() => {
    if (socket.connected) socket.emit('heartbeat', { ts: Date.now() })
  }, HEARTBEAT_MS)
}

function stopHeartbeat() {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer)
    heartbeatTimer = null
  }
}

/**
 * 소켓 연결을 시작한다. 앱에서 1회만 호출.
 * StrictMode 더블마운트 방어를 위해 `started` 가드가 적용돼 있다.
 */
export function startSocket(): void {
  if (started) return
  started = true
  const { setStatus, setAttempt } = useSocketStore.getState()

  socket.on('connect', () => {
    console.log('[Socket] connected', socket.id)
    setStatus('connected')
    setAttempt(0)
    startHeartbeat()
  })

  socket.on('disconnect', (reason) => {
    console.log('[Socket] disconnected', reason)
    stopHeartbeat()
    setStatus('reconnecting')
    // Socket.IO 는 'io server disconnect' 시 자동 재접속하지 않음 → 수동 재접속
    if (reason === 'io server disconnect') {
      socket.connect()
    }
  })

  socket.on('connect_error', (err) => {
    console.warn('[Socket] connect_error', err.message)
    if (TOKEN_ERROR_RE.test(err.message)) {
      cachedToken = null
    }
  })

  socket.io.on('reconnect_attempt', (n) => {
    setStatus('reconnecting')
    setAttempt(n)
  })

  socket.io.on('reconnect_failed', () => {
    console.warn('[Socket] reconnect_failed — giving up')
    setStatus('offline')
  })

  setStatus('connecting')
  socket.connect()
}

/** 테스트/HMR 등에서만 사용. 일반 런타임에서는 호출할 일이 없다. */
export function stopSocket(): void {
  stopHeartbeat()
  socket.removeAllListeners()
  socket.disconnect()
  started = false
  useSocketStore.getState().setStatus('idle')
}
