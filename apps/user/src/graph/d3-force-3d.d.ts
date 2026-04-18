declare module 'd3-force-3d' {
  export * from 'd3-force'
  import type { Force, Simulation, SimulationNodeDatum, SimulationLinkDatum } from 'd3-force'
  export function forceSimulation<NodeDatum extends SimulationNodeDatum, LinkDatum extends SimulationLinkDatum<NodeDatum> = SimulationLinkDatum<NodeDatum>>(
    nodes?: NodeDatum[],
    numDimensions?: 1 | 2 | 3,
  ): Simulation<NodeDatum, LinkDatum>
  export function forceZ<NodeDatum extends SimulationNodeDatum>(z?: number | ((d: NodeDatum, i: number, data: NodeDatum[]) => number)): {
    (alpha: number): void
    initialize(nodes: NodeDatum[]): void
    strength(): number | ((d: NodeDatum, i: number, data: NodeDatum[]) => number)
    strength(s: number | ((d: NodeDatum, i: number, data: NodeDatum[]) => number)): ReturnType<typeof forceZ<NodeDatum>>
    z(): number | ((d: NodeDatum, i: number, data: NodeDatum[]) => number)
    z(z: number | ((d: NodeDatum, i: number, data: NodeDatum[]) => number)): ReturnType<typeof forceZ<NodeDatum>>
  } & Force<NodeDatum, any>
}
