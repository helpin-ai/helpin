import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { configureSessionStorage, createCookieSessionStorage } from '@helpin-ai/support-core'
import { clearClientSession } from '@/stores/authStore'
import { installAppLoadRecovery } from '@/lib/appLoadRecovery'
import { initializeAppAnalytics } from '@/lib/analytics'
import { App } from './App'
import { isLogoutPath } from './appPaths'
import 'streamdown/styles.css'
import './index.css'

configureSessionStorage(createCookieSessionStorage())
const removeAppLoadRecovery = installAppLoadRecovery()
if (import.meta.hot) import.meta.hot.dispose(removeAppLoadRecovery)
initializeAppAnalytics()

if (isLogoutPath) {
  void clearClientSession().finally(() => {
    window.location.replace('/login')
  })
}

if (!isLogoutPath) {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}
