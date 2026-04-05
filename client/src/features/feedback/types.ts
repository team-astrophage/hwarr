export type FeedbackCategory = 'bug' | 'idea' | 'etc'

export interface FeedbackRequest {
  category: FeedbackCategory
  message: string
  email?: string
  page?: string
}

export const FEEDBACK_MAX_LEN = 500
