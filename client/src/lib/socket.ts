/**
 * Socket.IO 클라이언트 싱글톤
 *
 * 선택 이유:
 * - 싱글톤 패턴: 앱 전체에서 하나의 소켓 인스턴스 공유
 * - autoConnect: false → 지도 페이지 진입 시에만 연결 (랜딩에서는 소켓 안 씀)
 * - transports: ["polling", "websocket"] → polling으로 handshake 후 WebSocket 업그레이드
 *   (CloudFront가 WebSocket-only handshake를 지원하지 않음)
 */

import { io } from 'socket.io-client'
import { SOCKET_URL } from './config'

export const socket = io(SOCKET_URL, {
  autoConnect: false,
  transports: ['polling', 'websocket'],
})
