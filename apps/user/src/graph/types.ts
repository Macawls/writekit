export type Visibility = 'public' | 'unlisted' | 'private'

export interface GraphNode {
  id: string
  slug: string
  title: string
  tags: string[]
  collection_id?: string | null
  url: string
  visibility: Visibility
}

export interface GraphEdge {
  source: string
  target: string
  weight: number
}

export interface GraphCollection {
  id: string
  slug: string
  title: string
}

export interface GraphResponse {
  nodes: GraphNode[]
  collections: GraphCollection[]
}

export interface GraphView {
  nodes: GraphNode[]
  edges: GraphEdge[]
  collections: GraphCollection[]
}
