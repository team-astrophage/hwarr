/**
 * 피드백 제출 훅 (TanStack Query)
 *
 * POST /api/feedback → { ok: true }
 */

import { useMutation } from '@tanstack/react-query'
import { API_URL } from '@/lib/config'
import type { FeedbackRequest } from '../types'

async function submitFeedback(data: FeedbackRequest): Promise<{ ok: true }> {
  const res = await fetch(`${API_URL}/api/feedback`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) {
    if (res.status === 429) throw new Error('잠시 후 다시 시도해주세요')
    if (res.status === 422) throw new Error('입력값을 확인해주세요')
    throw new Error('전송에 실패했어요')
  }
  return res.json()
}

export function useFeedbackSubmit() {
  return useMutation({ mutationFn: submitFeedback })
}
