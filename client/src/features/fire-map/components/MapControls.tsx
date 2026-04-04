import { useMap } from 'react-leaflet'

export function MapControls({ onLocate }: { onLocate: () => void }) {
  const map = useMap()

  return (
    <div className="absolute right-4 top-1/2 -translate-y-1/2 z-[1000] flex flex-col gap-2">
      {/* Zoom In */}
      <button
        onClick={() => map.zoomIn()}
        className="w-10 h-10 bg-[var(--color-bg-surface)] rounded-[12px] shadow-[var(--shadow-heavy)] flex items-center justify-center text-[var(--color-text-base)] text-lg font-bold transition-[transform] duration-150 active:scale-[0.96] hover:bg-[var(--color-bg-elevated)]"
      >
        +
      </button>

      {/* Zoom Out */}
      <button
        onClick={() => map.zoomOut()}
        className="w-10 h-10 bg-[var(--color-bg-surface)] rounded-[12px] shadow-[var(--shadow-heavy)] flex items-center justify-center text-[var(--color-text-base)] text-lg font-bold transition-[transform] duration-150 active:scale-[0.96] hover:bg-[var(--color-bg-elevated)]"
      >
        &minus;
      </button>

      {/* Divider */}
      <div className="h-px mx-1.5 bg-[var(--color-border)]" />

      {/* My Location */}
      <button
        onClick={onLocate}
        className="w-10 h-10 bg-[var(--color-bg-surface)] rounded-[12px] shadow-[var(--shadow-heavy)] flex items-center justify-center transition-[transform] duration-150 active:scale-[0.96] hover:bg-[var(--color-bg-elevated)]"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--color-accent)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v4M12 18v4M2 12h4M18 12h4" />
        </svg>
      </button>
    </div>
  )
}
