import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from '@tanstack/react-router'
import { setCustomerPortalApiBase } from '@/lib/services/customerPortalService'
import { PortalUnavailable } from '@/components/customer-portal/CustomerPortal'
import { readPortalMount } from './mount'
import { createPortalRouter } from './router'
import 'streamdown/styles.css'
import '@/index.css'

// The portal-only app served by help centers at /requests. The help center
// server injects which workspace's portal this host serves.
const mount = readPortalMount()
const root = createRoot(document.getElementById('root')!)

if (!mount) {
  root.render(<StrictMode><PortalUnavailable /></StrictMode>)
} else {
  // The help center proxies the portal API on its own origin.
  setCustomerPortalApiBase(`${window.location.origin}${mount.basepath}/api`)
  const router = createPortalRouter(mount)
  root.render(
    <StrictMode>
      <RouterProvider router={router} />
    </StrictMode>,
  )
}
