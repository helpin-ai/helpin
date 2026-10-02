// @vitest-environment jsdom
import React, { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { CoverageAnalysisActions } from '../CoverageAnalysisActions'
import type { CoveragePipelineHealthV2 } from '@/lib/supportCoverageTypes'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const health: CoveragePipelineHealthV2 = { latest_batch:null,failures:[],healthy:true,rollout:{requested_mode:'v2_write',capture_enabled:true,assignment_enabled:true,read_v2_enabled:true,write_v2_enabled:true} }
let root: ReturnType<typeof createRoot>
let container: HTMLDivElement
function render(canReanalyze=true,status?:'queued'|'running') {
 container=document.createElement('div'); document.body.appendChild(container);root=createRoot(container)
 const onReanalyze=vi.fn()
 act(()=>root.render(<TooltipProvider><CoverageAnalysisActions health={{...health,reanalysis_status:status}} canReanalyze={canReanalyze} submitting={false} loading={false} onReanalyze={onReanalyze}/></TooltipProvider>))
 return onReanalyze
}
afterEach(()=>{act(()=>root?.unmount());container?.remove()})
it('offers reassessment without routine status noise',()=>{
 const action=render();const button=container.querySelector('button')!
 expect(button.textContent).toBe('Re-analyze');expect(container.querySelector('[role="status"]')).toBeNull()
 act(()=>button.click());expect(action).toHaveBeenCalledOnce()
})
it.each(['queued','running'] as const)('prevents a duplicate %s request',status=>{
 const action=render(true,status);const button=container.querySelector('button')!
 expect(button.disabled).toBe(true);act(()=>button.click());expect(action).not.toHaveBeenCalled()
 expect(container.querySelector('[role="status"]')?.textContent).toBe(status==='queued'?'Queued':'Analyzing')
})
it('keeps progress visible to people without manage permission',()=>{
 render(false,'running');expect(container.querySelector('button')).toBeNull()
 expect(container.querySelector('[role="status"]')?.textContent).toBe('Analyzing')
})
