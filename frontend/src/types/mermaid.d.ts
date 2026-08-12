declare module 'mermaid' {
  type MermaidRenderResult = {
    svg: string
    bindFunctions?: (element: Element) => void
  }

  type MermaidAPI = {
    initialize: (config: Record<string, unknown>) => void
    render: (id: string, source: string, svgContainingElement?: Element) => Promise<MermaidRenderResult>
  }

  const mermaid: MermaidAPI
  export default mermaid
}
