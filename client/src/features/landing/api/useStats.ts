/**
 * 전국 화재 통계 API 훅 (TanStack Query)
 *
 * GET /api/stats → { activeGrids, totalFires, onlineUsers }
 * 10초 간격 자동 리페치로 랜딩 페이지 수치 갱신
 */

import { useQuery } from '@tanstack/react-query'
import { API_URL } from '@/lib/config'

interface StatsData {
  activeGrids: number
  totalFires: number
  cumulativeFires: number
  dailyFires: number
  onlineUsers: number
}

async function fetchStats(): Promise<StatsData> {
  const res = await fetch(`${API_URL}/api/stats`)
  if (!res.ok) throw new Error('통계 API 호출 실패')
  return res.json()
}

export function useStats() {
  return useQuery({
    queryKey: ['stats'],
    queryFn: fetchStats,
    refetchInterval: 10_000,
  })
}
