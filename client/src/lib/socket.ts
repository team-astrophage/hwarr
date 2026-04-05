/**
 * Socket.IO 클라이언트 re-export
 *
 * 실제 인스턴스/수명주기 관리는 ./socketManager.ts 에 있으며,
 * 이 파일은 기존 import 경로 (`lib/socket`) 호환을 위해 유지된다.
 */

export { socket } from './socketManager'
