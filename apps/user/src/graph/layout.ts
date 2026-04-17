import { forceSimulation, forceManyBody, forceLink, forceCenter, forceCollide, type Simulation } from 'd3-force'
import type { GraphEdge, GraphNode } from './types'

export interface SimNode {
  id: string
  data: GraphNode
  x: number
  y: number
  vx: number
  vy: number
  fx?: number | null
  fy?: number | null
}

export interface SimLink {
  source: SimNode | string
  target: SimNode | string
  weight: number
}

export function buildSimulation(nodes: GraphNode[], edges: GraphEdge[]) {
  const simNodes: SimNode[] = nodes.map(n => ({
    id: n.id,
    data: n,
    x: (Math.random() - 0.5) * 200,
    y: (Math.random() - 0.5) * 200,
    vx: 0,
    vy: 0,
  }))

  const simLinks: SimLink[] = edges.map(e => ({
    source: e.source,
    target: e.target,
    weight: e.weight,
  }))

  const sim: Simulation<SimNode, SimLink> = forceSimulation(simNodes)
    .force('charge', forceManyBody<SimNode>().strength(-120))
    .force('link', forceLink<SimNode, SimLink>(simLinks)
      .id(d => d.id)
      .distance(l => 60 / Math.max(0.35, l.weight))
      .strength(l => Math.min(1, l.weight)))
    .force('center', forceCenter(0, 0))
    .force('collide', forceCollide<SimNode>().radius(14))
    .alphaDecay(0.02)

  return { sim, simNodes, simLinks }
}
