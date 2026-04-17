import {
  Scene,
  OrthographicCamera,
  WebGLRenderer,
  InstancedMesh,
  CircleGeometry,
  MeshBasicMaterial,
  Object3D,
  Color,
  BufferGeometry,
  BufferAttribute,
  LineBasicMaterial,
  LineSegments,
  Raycaster,
  Vector2,
} from 'three'
import { CSS2DRenderer, CSS2DObject } from 'three/addons/renderers/CSS2DRenderer.js'
import { forceLink } from 'd3-force'
import { buildSimulation, type SimLink, type SimNode } from './layout'
import type { GraphEdge, GraphNode } from './types'

const NODE_BASE_SCALE = 6
const NODE_MAX_DEGREE_BONUS = 9
const NODE_DEGREE_STEP = 0.7
const NODE_FOCUS_MULTIPLIER = 1.5
const NODE_HOVER_MULTIPLIER = 1.25
const NODE_SEGMENTS = 48

const BG_COLOR = 0xfafafa
const NODE_COLOR = 0x18181b
const NODE_DIM_COLOR = 0xc4c4c8
const EDGE_COLOR = 0x9ca3af
const EDGE_FOCUS_COLOR = 0x18181b
const EDGE_STRONG_THRESHOLD = 0.7
const EDGE_STRONG_OPACITY = 0.75
const EDGE_WEAK_OPACITY = 0.4
const EDGE_DIM_OPACITY = 0.08
const EDGE_FOCUS_OPACITY = 0.9

const ZOOM_MIN = 0.15
const ZOOM_MAX = 8
const FIT_PADDING = 0.85

interface Callbacks {
  onNodeClick: (node: GraphNode) => void
  onBackgroundClick: () => void
}

export class GraphRenderer {
  private host: HTMLElement
  private scene = new Scene()
  private camera: OrthographicCamera
  private renderer: WebGLRenderer
  private labelRenderer: CSS2DRenderer

  private nodeMesh: InstancedMesh
  private edgesStrong!: LineSegments
  private edgesWeak!: LineSegments
  private edgesFocus!: LineSegments
  private edgeStrongGeom!: BufferGeometry
  private edgeWeakGeom!: BufferGeometry
  private edgeFocusGeom!: BufferGeometry
  private strongIdx: number[] = []
  private weakIdx: number[] = []
  private focusEdgeIdx: number[] = []

  private simNodes: SimNode[]
  private simLinks: SimLink[]
  private sim: ReturnType<typeof buildSimulation>['sim']

  private labels: CSS2DObject[] = []
  private labelAnchors: Object3D[] = []
  private idToIndex = new Map<string, number>()
  private neighbors: Set<number>[] = []
  private degrees: number[] = []

  private hoveredIndex = -1
  private focusedIndex = -1
  private callbacks: Callbacks

  private dummy = new Object3D()
  private tmpColor = new Color()
  private raycaster = new Raycaster()
  private pointer = new Vector2()

  private animationId = 0
  private resizeObserver: ResizeObserver
  private disposed = false

  private isPanning = false
  private panMoved = false
  private panStart = { x: 0, y: 0 }
  private cameraStart = { x: 0, y: 0 }

  constructor(host: HTMLElement, nodes: GraphNode[], edges: GraphEdge[], callbacks: Callbacks) {
    this.host = host
    this.callbacks = callbacks

    const width = host.clientWidth
    const height = host.clientHeight

    this.camera = new OrthographicCamera(-width / 2, width / 2, height / 2, -height / 2, -1000, 1000)
    this.camera.position.z = 10

    this.renderer = new WebGLRenderer({ antialias: true, alpha: true })
    this.renderer.setPixelRatio(window.devicePixelRatio)
    this.renderer.setSize(width, height)
    this.renderer.setClearColor(BG_COLOR, 0)
    this.renderer.domElement.style.position = 'absolute'
    this.renderer.domElement.style.inset = '0'
    host.appendChild(this.renderer.domElement)

    this.labelRenderer = new CSS2DRenderer()
    this.labelRenderer.setSize(width, height)
    const labelEl = this.labelRenderer.domElement
    labelEl.style.position = 'absolute'
    labelEl.style.inset = '0'
    labelEl.style.pointerEvents = 'none'
    host.appendChild(labelEl)

    nodes.forEach((n, i) => this.idToIndex.set(n.id, i))

    const geom = new CircleGeometry(1, NODE_SEGMENTS)
    const mat = new MeshBasicMaterial({ color: 0xffffff })
    this.nodeMesh = new InstancedMesh(geom, mat, Math.max(1, nodes.length))
    this.scene.add(this.nodeMesh)

    const { sim, simNodes, simLinks } = buildSimulation(nodes, edges)
    this.sim = sim
    this.simNodes = simNodes
    this.simLinks = simLinks

    this.rebuildNeighborsAndEdges(edges)

    simNodes.forEach((n, i) => {
      const anchor = new Object3D()
      this.scene.add(anchor)
      this.labelAnchors.push(anchor)

      const div = document.createElement('div')
      div.className = 'graph-label'
      div.textContent = n.data.title || n.data.slug
      const label = new CSS2DObject(div)
      const baseScale = NODE_BASE_SCALE + Math.min(this.degrees[i] * NODE_DEGREE_STEP, NODE_MAX_DEGREE_BONUS)
      label.position.set(0, -baseScale - 7, 0)
      anchor.add(label)
      this.labels.push(label)
    })

    this.paintNodes()

    sim.on('tick', () => this.applySimToScene())
    this.applySimToScene()

    host.addEventListener('pointermove', this.onPointerMove)
    host.addEventListener('click', this.onClick)
    host.addEventListener('pointerdown', this.onPointerDown)
    host.addEventListener('pointerup', this.onPointerUp)
    host.addEventListener('pointerleave', this.onPointerUp)
    host.addEventListener('wheel', this.onWheel, { passive: false })

    this.resizeObserver = new ResizeObserver(() => this.resize())
    this.resizeObserver.observe(host)

    this.loop()
  }

  private rebuildNeighborsAndEdges(edges: GraphEdge[]) {
    const n = this.simNodes.length
    this.neighbors = Array.from({ length: n }, () => new Set<number>())
    this.degrees = new Array(n).fill(0)
    this.strongIdx = []
    this.weakIdx = []

    edges.forEach((e, ei) => {
      const a = this.idToIndex.get(e.source)
      const b = this.idToIndex.get(e.target)
      if (a === undefined || b === undefined) return
      this.neighbors[a].add(b)
      this.neighbors[b].add(a)
      this.degrees[a]++
      this.degrees[b]++
      if (e.weight >= EDGE_STRONG_THRESHOLD) this.strongIdx.push(ei)
      else this.weakIdx.push(ei)
    })

    this.edgeStrongGeom = this.buildEdgeGeom(this.strongIdx.length)
    this.edgeWeakGeom = this.buildEdgeGeom(this.weakIdx.length)
    this.edgeFocusGeom = this.buildEdgeGeom(0)

    const strongMat = new LineBasicMaterial({ color: EDGE_COLOR, transparent: true, opacity: EDGE_STRONG_OPACITY })
    const weakMat = new LineBasicMaterial({ color: EDGE_COLOR, transparent: true, opacity: EDGE_WEAK_OPACITY })
    const focusMat = new LineBasicMaterial({ color: EDGE_FOCUS_COLOR, transparent: true, opacity: EDGE_FOCUS_OPACITY })

    this.edgesStrong = new LineSegments(this.edgeStrongGeom, strongMat)
    this.edgesWeak = new LineSegments(this.edgeWeakGeom, weakMat)
    this.edgesFocus = new LineSegments(this.edgeFocusGeom, focusMat)
    this.edgesFocus.visible = false
    this.scene.add(this.edgesStrong)
    this.scene.add(this.edgesWeak)
    this.scene.add(this.edgesFocus)
  }

  private buildEdgeGeom(count: number): BufferGeometry {
    const g = new BufferGeometry()
    const positions = new Float32Array(Math.max(1, count) * 6)
    g.setAttribute('position', new BufferAttribute(positions, 3))
    return g
  }

  private loop = () => {
    if (this.disposed) return
    this.renderer.render(this.scene, this.camera)
    this.labelRenderer.render(this.scene, this.camera)
    this.animationId = requestAnimationFrame(this.loop)
  }

  private resize() {
    const width = this.host.clientWidth
    const height = this.host.clientHeight
    this.camera.left = -width / 2
    this.camera.right = width / 2
    this.camera.top = height / 2
    this.camera.bottom = -height / 2
    this.camera.updateProjectionMatrix()
    this.renderer.setSize(width, height)
    this.labelRenderer.setSize(width, height)
  }

  private nodeScale(i: number): number {
    const base = NODE_BASE_SCALE + Math.min(this.degrees[i] * NODE_DEGREE_STEP, NODE_MAX_DEGREE_BONUS)
    if (i === this.focusedIndex) return base * NODE_FOCUS_MULTIPLIER
    if (i === this.hoveredIndex) return base * NODE_HOVER_MULTIPLIER
    return base
  }

  private applySimToScene() {
    for (let i = 0; i < this.simNodes.length; i++) {
      const n = this.simNodes[i]
      const scale = this.nodeScale(i)
      this.dummy.position.set(n.x, n.y, 0)
      this.dummy.scale.set(scale, scale, 1)
      this.dummy.updateMatrix()
      this.nodeMesh.setMatrixAt(i, this.dummy.matrix)
      this.labelAnchors[i].position.set(n.x, n.y, 0)
    }
    this.nodeMesh.instanceMatrix.needsUpdate = true

    this.writeEdgePositions(this.edgeStrongGeom, this.strongIdx)
    this.writeEdgePositions(this.edgeWeakGeom, this.weakIdx)
    if (this.focusEdgeIdx.length > 0) {
      this.writeEdgePositions(this.edgeFocusGeom, this.focusEdgeIdx)
    }
  }

  private writeEdgePositions(geom: BufferGeometry, indices: number[]) {
    const pos = geom.getAttribute('position') as BufferAttribute
    const arr = pos.array as Float32Array
    for (let i = 0; i < indices.length; i++) {
      const l = this.simLinks[indices[i]]
      const s = l.source as SimNode
      const t = l.target as SimNode
      arr[i * 6 + 0] = s.x
      arr[i * 6 + 1] = s.y
      arr[i * 6 + 2] = 0
      arr[i * 6 + 3] = t.x
      arr[i * 6 + 4] = t.y
      arr[i * 6 + 5] = 0
    }
    pos.needsUpdate = true
  }

  private isHighlighted(i: number): boolean {
    if (this.focusedIndex < 0) return true
    if (i === this.focusedIndex) return true
    return this.neighbors[this.focusedIndex].has(i)
  }

  private paintNodes() {
    for (let i = 0; i < this.simNodes.length; i++) {
      const hex = this.isHighlighted(i) ? NODE_COLOR : NODE_DIM_COLOR
      this.tmpColor.setHex(hex)
      this.nodeMesh.setColorAt(i, this.tmpColor)
    }
    if (this.nodeMesh.instanceColor) this.nodeMesh.instanceColor.needsUpdate = true

    for (let i = 0; i < this.labels.length; i++) {
      const el = this.labels[i].element as HTMLElement
      if (this.focusedIndex >= 0 && !this.isHighlighted(i)) el.classList.add('dim')
      else el.classList.remove('dim')
    }
  }

  private applyEdgeFocus() {
    const strongMat = this.edgesStrong.material as LineBasicMaterial
    const weakMat = this.edgesWeak.material as LineBasicMaterial

    if (this.focusedIndex < 0) {
      strongMat.opacity = EDGE_STRONG_OPACITY
      weakMat.opacity = EDGE_WEAK_OPACITY
      this.focusEdgeIdx = []
      this.edgesFocus.visible = false
      return
    }

    strongMat.opacity = EDGE_DIM_OPACITY
    weakMat.opacity = EDGE_DIM_OPACITY

    this.focusEdgeIdx = []
    for (let i = 0; i < this.simLinks.length; i++) {
      const l = this.simLinks[i]
      const sid = typeof l.source === 'string' ? l.source : l.source.id
      const tid = typeof l.target === 'string' ? l.target : l.target.id
      const a = this.idToIndex.get(sid)
      const b = this.idToIndex.get(tid)
      if (a === this.focusedIndex || b === this.focusedIndex) {
        this.focusEdgeIdx.push(i)
      }
    }

    this.edgeFocusGeom.dispose()
    this.edgeFocusGeom = this.buildEdgeGeom(this.focusEdgeIdx.length)
    this.edgesFocus.geometry = this.edgeFocusGeom
    this.edgesFocus.visible = this.focusEdgeIdx.length > 0
    this.writeEdgePositions(this.edgeFocusGeom, this.focusEdgeIdx)
  }

  private onPointerMove = (ev: PointerEvent) => {
    if (this.isPanning) {
      const dx = (ev.clientX - this.panStart.x) / this.camera.zoom
      const dy = (ev.clientY - this.panStart.y) / this.camera.zoom
      if (Math.abs(ev.clientX - this.panStart.x) + Math.abs(ev.clientY - this.panStart.y) > 3) {
        this.panMoved = true
      }
      this.camera.position.x = this.cameraStart.x - dx
      this.camera.position.y = this.cameraStart.y + dy
      return
    }
    const rect = this.host.getBoundingClientRect()
    this.pointer.x = ((ev.clientX - rect.left) / rect.width) * 2 - 1
    this.pointer.y = -((ev.clientY - rect.top) / rect.height) * 2 + 1
    const prev = this.hoveredIndex
    this.hoveredIndex = this.hitTest()
    if (prev !== this.hoveredIndex) {
      this.applySimToScene()
      this.host.style.cursor = this.hoveredIndex >= 0 ? 'pointer' : 'grab'
    }
  }

  private onPointerDown = (ev: PointerEvent) => {
    if (this.hoveredIndex >= 0) return
    this.isPanning = true
    this.panMoved = false
    this.panStart.x = ev.clientX
    this.panStart.y = ev.clientY
    this.cameraStart.x = this.camera.position.x
    this.cameraStart.y = this.camera.position.y
    this.host.style.cursor = 'grabbing'
  }

  private onPointerUp = () => {
    if (this.isPanning) {
      this.isPanning = false
      this.host.style.cursor = this.hoveredIndex >= 0 ? 'pointer' : 'grab'
    }
  }

  private onWheel = (ev: WheelEvent) => {
    ev.preventDefault()
    const rect = this.host.getBoundingClientRect()
    const ndcX = ((ev.clientX - rect.left) / rect.width) * 2 - 1
    const ndcY = -((ev.clientY - rect.top) / rect.height) * 2 + 1
    const factor = Math.exp(-ev.deltaY * 0.0015)
    this.zoomAtNdc(factor, ndcX, ndcY)
  }

  private zoomAtNdc(factor: number, ndcX: number, ndcY: number) {
    const rect = this.host.getBoundingClientRect()
    const prevZoom = this.camera.zoom
    const halfW = (rect.width / 2) / prevZoom
    const halfH = (rect.height / 2) / prevZoom
    const wx = this.camera.position.x + ndcX * halfW
    const wy = this.camera.position.y + ndcY * halfH

    const next = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, prevZoom * factor))
    if (next === prevZoom) return
    this.camera.zoom = next
    this.camera.updateProjectionMatrix()

    const newHalfW = (rect.width / 2) / next
    const newHalfH = (rect.height / 2) / next
    this.camera.position.x = wx - ndcX * newHalfW
    this.camera.position.y = wy - ndcY * newHalfH
  }

  private onClick = () => {
    if (this.isPanning && this.panMoved) return
    if (this.hoveredIndex >= 0) {
      const node = this.simNodes[this.hoveredIndex]
      this.callbacks.onNodeClick(node.data)
      return
    }
    this.callbacks.onBackgroundClick()
  }

  private hitTest(): number {
    this.raycaster.setFromCamera(this.pointer, this.camera)
    const origin = this.raycaster.ray.origin
    let best = -1
    let bestDist = Infinity
    for (let i = 0; i < this.simNodes.length; i++) {
      const n = this.simNodes[i]
      const dx = n.x - origin.x
      const dy = n.y - origin.y
      const d2 = dx * dx + dy * dy
      const r = this.nodeScale(i) * 1.3
      if (d2 < r * r && d2 < bestDist) {
        bestDist = d2
        best = i
      }
    }
    return best
  }

  setFocusedNode(id: string | null) {
    const next = id ? (this.idToIndex.get(id) ?? -1) : -1
    if (next === this.focusedIndex) return
    this.focusedIndex = next
    this.paintNodes()
    this.applyEdgeFocus()
    this.applySimToScene()
    if (next >= 0) this.sim.alpha(0.1).restart()
  }

  zoomBy(factor: number) {
    this.zoomAtNdc(factor, 0, 0)
  }

  fit() {
    if (this.simNodes.length === 0) return
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
    for (const n of this.simNodes) {
      if (n.x < minX) minX = n.x
      if (n.y < minY) minY = n.y
      if (n.x > maxX) maxX = n.x
      if (n.y > maxY) maxY = n.y
    }
    const cx = (minX + maxX) / 2
    const cy = (minY + maxY) / 2
    const spanX = Math.max(1, maxX - minX)
    const spanY = Math.max(1, maxY - minY)
    const rect = this.host.getBoundingClientRect()
    const zoomX = (rect.width * FIT_PADDING) / spanX
    const zoomY = (rect.height * FIT_PADDING) / spanY
    this.camera.zoom = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Math.min(zoomX, zoomY)))
    this.camera.position.x = cx
    this.camera.position.y = cy
    this.camera.updateProjectionMatrix()
  }

  setEdges(edges: GraphEdge[]) {
    this.scene.remove(this.edgesStrong)
    this.scene.remove(this.edgesWeak)
    this.scene.remove(this.edgesFocus)
    this.edgeStrongGeom.dispose()
    this.edgeWeakGeom.dispose()
    this.edgeFocusGeom.dispose()
    ;(this.edgesStrong.material as LineBasicMaterial).dispose()
    ;(this.edgesWeak.material as LineBasicMaterial).dispose()
    ;(this.edgesFocus.material as LineBasicMaterial).dispose()

    this.rebuildNeighborsAndEdges(edges)

    this.simLinks = edges.map(e => ({ source: e.source, target: e.target, weight: e.weight }))
    const linkForce = forceLink<SimNode, SimLink>(this.simLinks)
      .id(d => d.id)
      .distance(l => 60 / Math.max(0.35, l.weight))
      .strength(l => Math.min(1, l.weight))
    this.sim.force('link', linkForce)
    this.sim.alpha(0.3).restart()
    this.paintNodes()
    this.applyEdgeFocus()
  }

  dispose() {
    this.disposed = true
    cancelAnimationFrame(this.animationId)
    this.sim.stop()
    this.resizeObserver.disconnect()
    this.host.removeEventListener('pointermove', this.onPointerMove)
    this.host.removeEventListener('click', this.onClick)
    this.host.removeEventListener('pointerdown', this.onPointerDown)
    this.host.removeEventListener('pointerup', this.onPointerUp)
    this.host.removeEventListener('pointerleave', this.onPointerUp)
    this.host.removeEventListener('wheel', this.onWheel)
    for (const label of this.labels) label.element.remove()
    this.nodeMesh.geometry.dispose()
    ;(this.nodeMesh.material as MeshBasicMaterial).dispose()
    this.edgeStrongGeom.dispose()
    this.edgeWeakGeom.dispose()
    this.edgeFocusGeom.dispose()
    ;(this.edgesStrong.material as LineBasicMaterial).dispose()
    ;(this.edgesWeak.material as LineBasicMaterial).dispose()
    ;(this.edgesFocus.material as LineBasicMaterial).dispose()
    this.renderer.dispose()
    if (this.renderer.domElement.parentNode) this.renderer.domElement.remove()
    const labelEl = this.labelRenderer.domElement
    if (labelEl.parentNode) labelEl.remove()
  }
}
