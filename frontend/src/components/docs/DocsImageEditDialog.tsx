import { useEffect, useRef, useState } from 'react'
import { Loading01Icon, PaintBoardIcon } from '@/lib/icons'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { LoadingImage } from '@/components/ui/loading-image'
import { docsService } from '@/lib/services/docsService'
import { pmAttachmentService } from '@/lib/services/pmAttachmentService'
import { toast } from 'sonner'

interface DocsImageEditDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: string
  documentId: string
  sourceAttachmentId: string
  sourceUrl: string
  onUse: (image: { attachmentId: string; url: string }) => void
}

// DocsImageEditDialog provides conversational, iterative editing of one docs image.
export function DocsImageEditDialog({ open, onOpenChange, workspaceId, documentId, sourceAttachmentId, sourceUrl, onUse }: DocsImageEditDialogProps) {
  const [prompt, setPrompt] = useState('')
  const [editing, setEditing] = useState(false)
  const [hasGenerated, setHasGenerated] = useState(false)
  const [annotating, setAnnotating] = useState(false)
  const [hasAnnotation, setHasAnnotation] = useState(false)
  const [drawing, setDrawing] = useState(false)
  const [candidate, setCandidate] = useState<{ attachmentId: string; url: string }>({ attachmentId: sourceAttachmentId, url: sourceUrl })
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const imageRef = useRef<HTMLImageElement>(null)
  const annotationSeededRef = useRef(false)

  useEffect(() => {
    if (open) {
      setPrompt('')
      setHasGenerated(false)
      setAnnotating(false)
      setHasAnnotation(false)
      annotationSeededRef.current = false
      setCandidate({ attachmentId: sourceAttachmentId, url: sourceUrl })
      canvasRef.current?.getContext('2d')?.clearRect(0, 0, canvasRef.current.width, canvasRef.current.height)
    }
  }, [open, sourceAttachmentId, sourceUrl])

  const prepareAnnotationCanvas = () => {
    const canvas = canvasRef.current
    const image = imageRef.current
    if (!canvas || !image || !image.naturalWidth || !image.naturalHeight) return false
    canvas.width = image.naturalWidth
    canvas.height = image.naturalHeight
    const context = canvas.getContext('2d')
    if (!context) return false
    context.clearRect(0, 0, canvas.width, canvas.height)
    context.drawImage(image, 0, 0, canvas.width, canvas.height)
    annotationSeededRef.current = true
    return true
  }

  const point = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current!
    const rect = canvas.getBoundingClientRect()
    return { x: (event.clientX - rect.left) * (canvas.width / rect.width), y: (event.clientY - rect.top) * (canvas.height / rect.height) }
  }
  const startStroke = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current
    if (!canvas) return
    if (!annotationSeededRef.current && !prepareAnnotationCanvas()) return
    const ctx = canvas.getContext('2d')!
    const { x, y } = point(event)
    canvas.setPointerCapture(event.pointerId)
    ctx.beginPath(); ctx.moveTo(x, y); ctx.strokeStyle = '#ef4444'; ctx.lineWidth = 14; ctx.lineCap = 'round'; ctx.lineJoin = 'round'
    setDrawing(true); setHasAnnotation(true)
  }
  const draw = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawing) return
    const ctx = canvasRef.current?.getContext('2d')
    if (!ctx) return
    const { x, y } = point(event)
    ctx.lineTo(x, y); ctx.stroke()
  }
  const clearAnnotation = () => {
    const canvas = canvasRef.current
    if (canvas) prepareAnnotationCanvas()
    setHasAnnotation(false)
  }

  const generate = async () => {
    if (!prompt.trim()) return
    setEditing(true)
    try {
      const annotation = hasAnnotation ? canvasRef.current?.toDataURL('image/png') : undefined
      const result = await docsService.editImage(workspaceId, documentId, { source_attachment_id: candidate.attachmentId, prompt: prompt.trim(), ...(annotation ? { annotation_data_url: annotation } : {}) })
      if (result.error || !result.data) throw new Error(result.error ?? 'Could not edit image')
      setCandidate({ attachmentId: result.data.attachment_id, url: pmAttachmentService.contentUrl(result.data.attachment_id) })
      setHasGenerated(true); setPrompt(''); setAnnotating(false); clearAnnotation()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not edit image')
    } finally { setEditing(false) }
  }

  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="flex max-h-[88vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-4xl">
      <DialogHeader className="border-b px-5 py-4 text-left sm:text-left">
        <DialogTitle>Edit image with AI</DialogTitle>
        <DialogDescription className="mt-1 max-w-2xl">Describe a change or annotate the area you want to modify. Your original image is preserved.</DialogDescription>
      </DialogHeader>
      <div className="min-h-0 flex-1 overflow-y-auto p-5">
        <div className="flex min-h-72 max-h-[52vh] items-center justify-center overflow-hidden rounded-lg border bg-muted/30 p-2">
          <div className="relative inline-flex max-h-[52vh] max-w-full">
            <LoadingImage ref={imageRef} crossOrigin="anonymous" src={candidate.url} alt="AI image edit preview" containerClassName="max-h-[52vh] max-w-full" className="max-h-[52vh] max-w-full object-contain" />
            {(annotating || hasAnnotation) && <canvas ref={canvasRef} className={`absolute inset-0 h-full w-full touch-none ${annotating ? 'cursor-crosshair' : 'pointer-events-none'}`} onPointerDown={startStroke} onPointerMove={draw} onPointerUp={() => setDrawing(false)} onPointerLeave={() => setDrawing(false)} />}
            {annotating && <div className="absolute left-3 top-3 rounded-md bg-red-500 px-2.5 py-1 text-xs font-medium text-white shadow-sm">Draw in red</div>}
            {!annotating && hasAnnotation && <div className="absolute left-3 top-3 rounded-md border border-red-200 dark:border-red-800 bg-background/95 px-2.5 py-1 text-xs font-medium text-red-600 dark:text-red-400 shadow-sm backdrop-blur">Target area selected</div>}
          </div>
        </div>
      </div>
      <div className="border-t px-5 py-4">
        <div className="flex items-center gap-2 pb-3">
          <Button type="button" variant={annotating ? 'secondary' : 'outline'} size="sm" onClick={() => setAnnotating(value => !value)}><PaintBoardIcon className="mr-2 h-4 w-4" />{annotating ? 'Finish annotation' : hasAnnotation ? 'Edit annotation' : 'Annotate'}</Button>
          {annotating && hasAnnotation && <Button type="button" variant="ghost" size="sm" onClick={clearAnnotation}>Clear</Button>}
          <span className="text-xs text-muted-foreground">{hasAnnotation ? 'The highlighted area will be sent with your prompt.' : 'Mark an area for a targeted edit.'}</span>
        </div>
        <Textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} onKeyDown={(event) => { if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') void generate() }} placeholder="Describe the change you want…" rows={3} disabled={editing} autoFocus className="min-h-20 resize-none" />
        <DialogFooter className="mt-4 sm:justify-between"><Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button><div className="flex gap-2"><Button onClick={() => void generate()} disabled={editing || !prompt.trim()}>{editing && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />}Generate</Button>{hasGenerated && <Button onClick={() => { onUse(candidate); onOpenChange(false) }}>Use this image</Button>}</div></DialogFooter>
      </div>
    </DialogContent>
  </Dialog>
}
