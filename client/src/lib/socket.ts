/**
 * Socket.IO 클라이언트 싱글톤
 *
 * 선택 이유:
 * - 싱글톤 패턴: 앱 전체에서 하나의 소켓 인스턴스 공유
 * - autoConnect: false → 지도 페이지 진입 시에만 연결 (랜딩에서는 소켓 안 씀)
 * - transports: ["websocket"] → polling 폴백 없이 바로 WebSocket
 *   (해커톤 환경에서 polling 불필요, 연결 속도 빠름)
 */

import { io } from 'socket.io-client'
import { SOCKET_URL } from './config'

export const socket = io(SOCKET_URL, {
  autoConnect: false,
  transports: ['websocket'],
})
