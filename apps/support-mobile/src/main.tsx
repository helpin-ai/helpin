import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'

export function App() {
  return (
    <div className="flex min-h-dvh items-center justify-center bg-background text-foreground">
      <h1 className="text-xl font-semibold">Helpin Support</h1>
    </div>
  )
}

const rootEl = document.getElementById('root')
if (rootEl) {
  createRoot(rootEl).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}
