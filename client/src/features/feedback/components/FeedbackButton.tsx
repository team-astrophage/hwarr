import { useState } from 'react'
import { FeedbackModal } from './FeedbackModal'

interface FeedbackButtonProps {
  /** Optional custom trigger renderer. Receives onClick handler. */
  children?: (open: () => void) => React.ReactNode
}

export function FeedbackButton({ children }: FeedbackButtonProps) {
  const [open, setOpen] = useState(false)

  return (
    <>
      {children ? (
        children(() => setOpen(true))
      ) : (
        <button
          onClick={() => setOpen(true)}
          aria-label='의견 보내기'
          className='w-12 h-12 bg-[var(--color-bg-surface)] rounded-full shadow-[var(--shadow-heavy)] flex items-center justify-center transition-transform active:scale-90 hover:bg-[var(--color-bg-elevated)]'
        >
          <svg
            width='20'
            height='20'
            viewBox='0 0 24 24'
            fill='none'
            stroke='var(--color-text-secondary)'
            strokeWidth='2'
            strokeLinecap='round'
            strokeLinejoin='round'
          >
            <path d='M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z' />
            <polyline points='22,6 12,13 2,6' />
          </svg>
        </button>
      )}
      {open && <FeedbackModal onClose={() => setOpen(false)} />}
    </>
  )
}
