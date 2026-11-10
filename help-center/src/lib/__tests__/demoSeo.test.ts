import {expect, it} from 'vitest'
import {buildRootHead} from '../seo'
import type {RootRouteData} from '../rootLoader'
it('emits initial noindex metadata for demos and explicit opt-out only',()=>{
 for(const [config,expected] of [[{demo_mode:true,search_indexing_enabled:true},true],[{search_indexing_enabled:false},true],[{search_indexing_enabled:true},false],[{},false],[undefined,false],[null,false]] as const){
  const head=buildRootHead({config} as RootRouteData)
  expect(head.meta.some(meta=>meta.name==='robots'&&meta.content==='noindex')).toBe(expected)
 }
})
it('does not infer an indexing policy when the root loader fails', () => {
 expect(buildRootHead().meta.some(meta => meta.name === 'robots')).toBe(false)
})
