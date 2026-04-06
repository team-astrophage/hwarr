import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { App } from './app/App'

// Dev only: GPS가 안 잡히는 환경(macOS 등)에서 서울역 좌표로 고정
if (import.meta.env.DEV) {
  window.__MELTTOWN_GPS = { lat: 37.5547, lng: 126.9707 }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
