export interface GraphNode {
  id: string
  slug: string
  title: string
  tags: string[]
  collection_id?: string | null
  url: string
}

export interface GraphEdge {
  source: string
  target: string
  weight: number
}

export interface GraphResponse {
  nodes: GraphNode[]
  edges: GraphEdge[]
  model: string
  embedded_count: number
  total_page_count: number
}
