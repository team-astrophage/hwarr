import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useFeedbackSubmit } from '../api/useFeedbackSubmit'
import type { FeedbackCategory } from '../types'
import { FEEDBACK_MAX_LEN } from '../types'

interface FeedbackModalProps {
  onClose: () => void
}

const CATEGORIES: { value: FeedbackCategory; label: string; icon: string }[] = [
  { value: 'bug', label: '버그', icon: '🐛' },
  { value: 'idea', label: '제안', icon: '💡' },
  { value: 'etc', label: '기타', icon: '💬' },
]

export function FeedbackModal({ onClose }: FeedbackModalProps) {
  const [category, setCategory] = useState<FeedbackCategory>('bug')
  const [message, setMessage] = useState('')
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const submit = useFeedbackSubmit()

  // Lock body scroll
  useEffect(() => {
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [])

  // Autofocus textarea
  useEffect(() => {
    textareaRef.current?.focus()
  }, [])

  // ESC to close
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') requestClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [message])

  // Auto-close after success
  useEffect(() => {
    if (!sent) return
    const t = setTimeout(onClose, 1500)
    return () => clearTimeout(t)
  }, [sent, onClose])

  const requestClose = () => {
    if (message.trim().length > 0 && !sent) {
      const ok = window.confirm('작성 중인 내용이 사라집니다. 닫을까요?')
      if (!ok) return
    }
    onClose()
  }

  const over = message.length > FEEDBACK_MAX_LEN
  const canSubmit = message.trim().length > 0 && !over && !submit.isPending

  const handleSubmit = () => {
    if (!canSubmit) return
    submit.mutate(
      {
        category,
        message: message.trim(),
        email: email.trim() || undefined,
        page: typeof window !== 'undefined' ? window.location.pathname : undefined,
      },
      {
        onSuccess: () => setSent(true),
      },
    )
  }

  // Render via portal so `fixed inset-0` is anchored to the viewport,
  // not to any ancestor with transform/filter/backdrop-filter (e.g. landing
  // footer which would otherwise clip the modal).
  if (typeof document === 'undefined') return null

  return createPortal(
    <div
      className='fixed inset-0 z-[2000] bg-black/60 backdrop-blur-sm flex items-start justify-center pt-[20vh] px-4'
      onClick={requestClose}
    >
      <div
        className='bg-[var(--color-bg-surface)] rounded-[16px] shadow-[var(--shadow-heavy)] w-full max-w-[360px] px-5 py-5 flex flex-col'
        onClick={(e) => e.stopPropagation()}
      >
        {sent ? (
          <div className='py-10 flex flex-col items-center text-center'>
            <div className='text-5xl mb-3' aria-hidden='true'>🔥</div>
            <p className='text-[1.0625rem] font-bold text-[var(--color-text-base)]'>
              의견 감사합니다
            </p>
            <p className='text-[0.8125rem] text-[var(--color-text-secondary)] mt-1'>
              개발자에게 전달됐어요
            </p>
          </div>
        ) : (
          <>
            <div className='flex items-center justify-between mb-4'>
              <h2 className='text-[1.0625rem] font-bold text-[var(--color-text-base)]'>
                의견 보내기
              </h2>
              <button
                onClick={requestClose}
                aria-label='닫기'
                className='w-8 h-8 flex items-center justify-center rounded-full text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-elevated)] transition-colors'
              >
                <svg width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2.5' strokeLinecap='round'>
                  <path d='M18 6L6 18M6 6l12 12' />
                </svg>
              </button>
            </div>

            {/* Category chips */}
            <div className='flex gap-2 mb-3'>
              {CATEGORIES.map((c) => {
                const active = category === c.value
                return (
                  <button
                    key={c.value}
                    onClick={() => setCategory(c.value)}
                    className={`flex-1 h-9 rounded-[10px] text-[0.8125rem] font-bold transition-colors ${
                      active
                        ? 'bg-[var(--color-accent)] text-black'
                        : 'bg-[var(--color-bg-elevated)] text-[var(--color-text-secondary)]'
                    }`}
                  >
                    <span className='mr-1' aria-hidden='true'>{c.icon}</span>
                    {c.label}
                  </button>
                )
              })}
            </div>

            {/* Message textarea */}
            <textarea
              ref={textareaRef}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder='무엇이든 자유롭게 적어주세요'
              rows={5}
              className='w-full bg-[var(--color-bg-elevated)] text-[var(--color-text-base)] rounded-[12px] px-3 py-3 text-[0.9375rem] leading-relaxed resize-none outline-none placeholder:text-[var(--color-text-secondary)] focus:ring-2 focus:ring-[var(--color-accent)]/50'
            />

            {/* Counter */}
            <div className='flex items-center justify-between mt-1 mb-3 min-h-[18px]'>
              {submit.isError ? (
                <p className='text-[0.75rem] text-[var(--color-negative)]'>
                  {(submit.error as Error).message}
                </p>
              ) : (
                <span />
              )}
              {message.length >= 400 && (
                <span
                  className={`text-[0.75rem] font-medium ${
                    over ? 'text-[var(--color-negative)]' : 'text-[var(--color-text-secondary)]'
                  }`}
                >
                  {message.length}/{FEEDBACK_MAX_LEN}
                </span>
              )}
            </div>

            {/* Optional email */}
            <details className='mb-4 group'>
              <summary className='cursor-pointer list-none text-[0.8125rem] text-[var(--color-text-secondary)] flex items-center gap-1 select-none'>
                <svg
                  width='12'
                  height='12'
                  viewBox='0 0 24 24'
                  fill='none'
                  stroke='currentColor'
                  strokeWidth='2.5'
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  className='transition-transform group-open:rotate-90'
                >
                  <path d='M9 18l6-6-6-6' />
                </svg>
                답변받을 이메일 (선택)
              </summary>
              <input
                type='email'
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder='you@example.com'
                className='mt-2 w-full bg-[var(--color-bg-elevated)] text-[var(--color-text-base)] rounded-[10px] px-3 py-2.5 text-[0.875rem] outline-none placeholder:text-[var(--color-text-secondary)] focus:ring-2 focus:ring-[var(--color-accent)]/50'
              />
            </details>

            {/* Submit */}
            <button
              onClick={handleSubmit}
              disabled={!canSubmit}
              className='w-full bg-[var(--color-accent)] text-black rounded-[12px] py-3 text-[0.9375rem] font-bold transition-transform active:scale-[0.98] disabled:opacity-30 disabled:active:scale-100'
            >
              {submit.isPending ? '보내는 중...' : '보내기'}
            </button>
          </>
        )}
      </div>
    </div>,
    document.body,
  )
}
