/**
 * 오늘의 방화 지역 랭킹 API 훅 (TanStack Query)
 *
 * GET /api/ranking/today → { date, items: [{ rank, region, count }, ...] }
 * 15초 간격 자동 리페치 (stats 10초와 어긋나게).
 */

import { useQuery } from '@tanstack/react-query'
import { API_URL } from '@/lib/config'

export interface RankingItem {
  rank: number
  region: string
  count: number
}

interface RankingResponse {
  date: string
  items: RankingItem[]
}

async function fetchRanking(): Promise<RankingResponse> {
  const res = await fetch(`${API_URL}/api/ranking/today`)
  if (!res.ok) throw new Error('랭킹 API 호출 실패')
  return res.json()
}

export function useRanking() {
  return useQuery({
    queryKey: ['ranking', 'today'],
    queryFn: fetchRanking,
    refetchInterval: 15_000,
  })
}
