/**
 * 속보 뉴스 피드 API 훅 (TanStack Query)
 *
 * GET /api/news → NewsItem[]
 * 10초 간격 자동 리페치로 랜딩 페이지 속보 갱신
 */

import { useQuery } from "@tanstack/react-query"
import { API_URL } from "@/lib/config"

export interface HeadlinePart {
  text: string
  highlight: "accent" | "warning" | null
}

export interface NewsItem {
  id: string
  icon: string
  icon_bg: string
  headline_parts: HeadlinePart[]
  time: string
  detail: string
}

async function fetchNews(): Promise<NewsItem[]> {
  const res = await fetch(`${API_URL}/api/news`)
  if (!res.ok) throw new Error("뉴스 API 호출 실패")
  return res.json()
}

export function useNews() {
  return useQuery({
    queryKey: ["news"],
    queryFn: fetchNews,
    refetchInterval: 10_000,
  })
}
