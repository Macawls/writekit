import {
  forceSimulation,
  forceManyBody,
  forceLink,
  forceCollide,
  forceX,
  forceY,
  forceZ,
  type Simulation,
} from 'd3-force-3d'
import type { GraphEdge, GraphNode } from './types'

export interface SimNode {
  id: string
  data: GraphNode
  x: number
  y: number
  z: number
  vx: number
  vy: number
  vz: number
  fx?: number | null
  fy?: number | null
  fz?: number | null
}

export interface SimLink {
  source: SimNode | string
  target: SimNode | string
  weight: number
}

const GRAVITY = 0.12
const REPEL = -400
const LINK_DIST = 180
const LINK_STRENGTH = 0.45
const COLLIDE_RADIUS = 14

export function buildSimulation(nodes: GraphNode[], edges: GraphEdge[], mode: '2d' | '3d' = '2d') {
  const simNodes: SimNode[] = nodes.map(n => ({
    id: n.id,
    data: n,
    x: (Math.random() - 0.5) * 200,
    y: (Math.random() - 0.5) * 200,
    z: mode === '3d' ? (Math.random() - 0.5) * 200 : 0,
    vx: 0,
    vy: 0,
    vz: 0,
  }))

  const simLinks: SimLink[] = edges.map(e => ({
    source: e.source,
    target: e.target,
    weight: e.weight,
  }))

  const sim: Simulation<SimNode, SimLink> = forceSimulation(simNodes, mode === '3d' ? 3 : 2)
    .force('charge', forceManyBody<SimNode>().strength(REPEL))
    .force('link', forceLink<SimNode, SimLink>(simLinks)
      .id(d => d.id)
      .distance(l => LINK_DIST / Math.max(0.35, l.weight))
      .strength(l => LINK_STRENGTH * Math.min(1, l.weight)))
    .force('x', forceX<SimNode>(0).strength(GRAVITY))
    .force('y', forceY<SimNode>(0).strength(GRAVITY))
    .force('collide', forceCollide<SimNode>().radius(COLLIDE_RADIUS))
    .alphaDecay(0.02)

  if (mode === '3d') {
    sim.force('z', forceZ<SimNode>(0).strength(GRAVITY))
  }

  return { sim, simNodes, simLinks }
}
