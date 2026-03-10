import { useCallback } from 'react'
import { toast } from 'sonner'

/**
 * Reusable hook for common image actions: copy to clipboard, download, open in new tab.
 */
export function useImageActions() {
  const copyImage = useCallback(async (src: string) => {
    try {
      // Load image onto a canvas to get a PNG blob — avoids CORS fetch issues
      // since we load via an Image element with crossOrigin.
      const blob = await imageSrcToPngBlob(src)

      await navigator.clipboard.write([
        new ClipboardItem({ 'image/png': blob }),
      ])
      toast.success('Image copied to clipboard')
    } catch {
      // Fallback: copy the URL
      try {
        await navigator.clipboard.writeText(src)
        toast.success('Image URL copied to clipboard')
      } catch {
        toast.error('Failed to copy image')
      }
    }
  }, [])

  const downloadImage = useCallback(async (src: string, filename?: string) => {
    try {
      const response = await fetch(src)
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename || getFilenameFromUrl(src)
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch {
      // Fallback: open in new tab
      window.open(src, '_blank')
    }
  }, [])

  const openInNewTab = useCallback((src: string) => {
    window.open(src, '_blank')
  }, [])

  return { copyImage, downloadImage, openInNewTab }
}

/** Load an image URL onto a canvas and return a PNG blob for clipboard use. */
function imageSrcToPngBlob(src: string): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => {
      const canvas = document.createElement('canvas')
      canvas.width = img.naturalWidth
      canvas.height = img.naturalHeight
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        reject(new Error('Canvas context unavailable'))
        return
      }
      ctx.drawImage(img, 0, 0)
      canvas.toBlob((blob) => {
        if (blob) resolve(blob)
        else reject(new Error('Failed to create PNG blob'))
      }, 'image/png')
    }
    img.onerror = () => reject(new Error('Failed to load image'))
    img.src = src
  })
}

function getFilenameFromUrl(url: string): string {
  try {
    const pathname = new URL(url).pathname
    const name = pathname.split('/').pop()
    return name || 'image'
  } catch {
    return 'image'
  }
}
