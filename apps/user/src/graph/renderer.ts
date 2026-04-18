import {
  Scene,
  OrthographicCamera,
  WebGLRenderer,
  InstancedMesh,
  CircleGeometry,
  MeshBasicMaterial,
  Object3D,
  Color,
  Raycaster,
  Vector2,
  AdditiveBlending,
} from 'three'
import { CSS2DRenderer, CSS2DObject } from 'three/addons/renderers/CSS2DRenderer.js'
import { LineSegments2 } from 'three/addons/lines/LineSegments2.js'
import { LineSegmentsGeometry } from 'three/addons/lines/LineSegmentsGeometry.js'
import { LineMaterial } from 'three/addons/lines/LineMaterial.js'
import { forceLink } from 'd3-force'
import { buildSimulation, type SimLink, type SimNode } from './layout'
import type { GraphEdge, GraphNode } from './types'

const NODE_BASE_SCALE = 6
const NODE_MAX_DEGREE_BONUS = 9
const NODE_DEGREE_STEP = 0.7
const NODE_FOCUS_MULTIPLIER = 1.5
const NODE_HOVER_MULTIPLIER = 1.25
const NODE_SEGMENTS = 48
const GLOW_SCALE = 2.4

const BG_COLOR = 0xfafafa
const NODE_COLOR = 0x18181b
const NODE_DIM_COLOR = 0xc4c4c8
const GLOW_COLOR = 0x4f46e5

const EDGE_STRONG_THRESHOLD = 0.75
const EDGE_WEAK_COLOR = 0xc4c4c8
const EDGE_STRONG_COLOR = 0x4f46e5
const EDGE_FOCUS_COLOR = 0x4f46e5
const EDGE_WEAK_WIDTH = 1.4
const EDGE_STRONG_WIDTH = 2.6
const EDGE_FOCUS_WIDTH = 3.8
const EDGE_WEAK_OPACITY = 0.55
const EDGE_STRONG_OPACITY = 0.85
const EDGE_DIM_OPACITY = 0.07

const ZOOM_MIN = 0.15
const ZOOM_MAX = 8
const FIT_PADDING = 0.85

const DRAG_MOVE_THRESHOLD_PX = 4

interface Callbacks {
  onNodeClick: (node: GraphNode) => void
  onBackgroundClick: () => void
}

interface EdgeLayer {
  indices: number[]
  line: LineSegments2
  geom: LineSegmentsGeometry
  mat: LineMaterial
  positions: Float32Array
}

export class GraphRenderer {
  private host: HTMLElement
  private scene = new Scene()
  private camera: OrthographicCamera
  private renderer: WebGLRenderer
  private labelRenderer: CSS2DRenderer

  private nodeMesh: InstancedMesh
  private glowMesh: InstancedMesh

  private weakLayer!: EdgeLayer
  private strongLayer!: EdgeLayer
  private focusLayer!: EdgeLayer

  private simNodes: SimNode[]
  private simLinks: SimLink[]
  private sim: ReturnType<typeof buildSimulation>['sim']

  private labels: CSS2DObject[] = []
  private labelAnchors: Object3D[] = []
  private idToIndex = new Map<string, number>()
  private neighbors: Set<number>[] = []
  private degrees: number[] = []

  private hoveredIndex = -1
  private externalHoverIndex = -1
  private focusedIndex = -1
  private hiddenNodes: Set<number> = new Set()
  private callbacks: Callbacks

  private dummy = new Object3D()
  private tmpColor = new Color()
  private raycaster = new Raycaster()
  private pointer = new Vector2()
  private pointerWorld = new Vector2()

  private animationId = 0
  private resizeObserver: ResizeObserver
  private disposed = false

  private isPanning = false
  private panMoved = false
  private panStart = { x: 0, y: 0 }
  private cameraStart = { x: 0, y: 0 }

  private draggingIndex = -1
  private dragMoved = false
  private dragStart = { x: 0, y: 0 }

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

    const glowGeom = new CircleGeometry(1, NODE_SEGMENTS)
    const glowMat = new MeshBasicMaterial({
      color: GLOW_COLOR,
      transparent: true,
      opacity: 0.0,
      blending: AdditiveBlending,
      depthWrite: false,
    })
    this.glowMesh = new InstancedMesh(glowGeom, glowMat, Math.max(1, nodes.length))
    this.glowMesh.visible = false
    this.scene.add(this.glowMesh)

    const geom = new CircleGeometry(1, NODE_SEGMENTS)
    const mat = new MeshBasicMaterial({ color: 0xffffff })
    this.nodeMesh = new InstancedMesh(geom, mat, Math.max(1, nodes.length))
    this.scene.add(this.nodeMesh)

    const { sim, simNodes, simLinks } = buildSimulation(nodes, edges)
    this.sim = sim
    this.simNodes = simNodes
    this.simLinks = simLinks

    this.buildEdgeLayers(edges)

    simNodes.forEach((n, i) => {
      const anchor = new Object3D()
      this.scene.add(anchor)
      this.labelAnchors.push(anchor)

      const div = document.createElement('div')
      div.className = 'graph-label'
      if (n.data.visibility && n.data.visibility !== 'public') {
        const dot = document.createElement('span')
        dot.className = `graph-label-dot graph-label-dot--${n.data.visibility}`
        dot.title = n.data.visibility
        div.appendChild(dot)
      }
      const text = document.createElement('span')
      text.textContent = n.data.title || n.data.slug
      div.appendChild(text)
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
    host.addEventListener('wheel', this.onWheel, { passive: false })

    this.resizeObserver = new ResizeObserver(() => this.resize())
    this.resizeObserver.observe(host)

    this.loop()
  }

  private createEdgeLayer(color: number, width: number, opacity: number): EdgeLayer {
    const mat = new LineMaterial({
      color,
      linewidth: width,
      transparent: true,
      opacity,
      worldUnits: false,
      depthTest: false,
    })
    const rect = this.host.getBoundingClientRect()
    mat.resolution.set(rect.width, rect.height)
    const geom = new LineSegmentsGeometry()
    const line = new LineSegments2(geom, mat)
    line.renderOrder = 0
    this.scene.add(line)
    return { indices: [], line, geom, mat, positions: new Float32Array(0) }
  }

  private buildEdgeLayers(edges: GraphEdge[]) {
    const n = this.simNodes.length
    this.neighbors = Array.from({ length: n }, () => new Set<number>())
    this.degrees = new Array(n).fill(0)

    this.weakLayer = this.createEdgeLayer(EDGE_WEAK_COLOR, EDGE_WEAK_WIDTH, EDGE_WEAK_OPACITY)
    this.strongLayer = this.createEdgeLayer(EDGE_STRONG_COLOR, EDGE_STRONG_WIDTH, EDGE_STRONG_OPACITY)
    this.focusLayer = this.createEdgeLayer(EDGE_FOCUS_COLOR, EDGE_FOCUS_WIDTH, 1.0)
    this.focusLayer.line.visible = false
    this.focusLayer.line.renderOrder = 1

    edges.forEach((e, ei) => {
      const a = this.idToIndex.get(e.source)
      const b = this.idToIndex.get(e.target)
      if (a === undefined || b === undefined) return
      this.neighbors[a].add(b)
      this.neighbors[b].add(a)
      this.degrees[a]++
      this.degrees[b]++
      if (e.weight >= EDGE_STRONG_THRESHOLD) this.strongLayer.indices.push(ei)
      else this.weakLayer.indices.push(ei)
    })

    this.weakLayer.positions = new Float32Array(this.weakLayer.indices.length * 6)
    this.strongLayer.positions = new Float32Array(this.strongLayer.indices.length * 6)
    this.focusLayer.positions = new Float32Array(0)
  }

  private updateLayerPositions(layer: EdgeLayer) {
    if (layer.indices.length === 0) return
    const arr = layer.positions
    for (let i = 0; i < layer.indices.length; i++) {
      const l = this.simLinks[layer.indices[i]]
      const s = l.source as SimNode
      const t = l.target as SimNode
      arr[i * 6 + 0] = s.x
      arr[i * 6 + 1] = s.y
      arr[i * 6 + 2] = 0
      arr[i * 6 + 3] = t.x
      arr[i * 6 + 4] = t.y
      arr[i * 6 + 5] = 0
    }
    layer.geom.setPositions(arr)
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
    this.weakLayer?.mat.resolution.set(width, height)
    this.strongLayer?.mat.resolution.set(width, height)
    this.focusLayer?.mat.resolution.set(width, height)
  }

  private nodeScale(i: number): number {
    const base = NODE_BASE_SCALE + Math.min(this.degrees[i] * NODE_DEGREE_STEP, NODE_MAX_DEGREE_BONUS)
    if (i === this.focusedIndex) return base * NODE_FOCUS_MULTIPLIER
    if (i === this.hoveredIndex || i === this.externalHoverIndex) return base * NODE_HOVER_MULTIPLIER
    return base
  }

  private applySimToScene() {
    for (let i = 0; i < this.simNodes.length; i++) {
      const n = this.simNodes[i]
      const hidden = this.hiddenNodes.has(i)
      const scale = hidden ? 0.0001 : this.nodeScale(i)
      this.dummy.position.set(n.x, n.y, 0)
      this.dummy.scale.set(scale, scale, 1)
      this.dummy.updateMatrix()
      this.nodeMesh.setMatrixAt(i, this.dummy.matrix)

      if (!hidden && (i === this.focusedIndex || i === this.externalHoverIndex)) {
        this.dummy.scale.set(scale * GLOW_SCALE, scale * GLOW_SCALE, 1)
        this.dummy.updateMatrix()
      } else {
        this.dummy.scale.set(0.0001, 0.0001, 1)
        this.dummy.updateMatrix()
      }
      this.glowMesh.setMatrixAt(i, this.dummy.matrix)

      const label = this.labels[i].element as HTMLElement
      label.style.display = hidden ? 'none' : ''
      this.labelAnchors[i].position.set(n.x, n.y, 0)
    }
    this.nodeMesh.instanceMatrix.needsUpdate = true
    this.glowMesh.instanceMatrix.needsUpdate = true
    this.glowMesh.visible = this.focusedIndex >= 0 || this.externalHoverIndex >= 0
    ;(this.glowMesh.material as MeshBasicMaterial).opacity = 0.22

    this.updateLayerPositions(this.weakLayer)
    this.updateLayerPositions(this.strongLayer)
    if (this.focusLayer.indices.length > 0) {
      this.updateLayerPositions(this.focusLayer)
    }
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
    if (this.focusedIndex < 0) {
      this.weakLayer.mat.opacity = EDGE_WEAK_OPACITY
      this.strongLayer.mat.opacity = EDGE_STRONG_OPACITY
      this.focusLayer.indices = []
      this.focusLayer.line.visible = false
      return
    }

    this.weakLayer.mat.opacity = EDGE_DIM_OPACITY
    this.strongLayer.mat.opacity = EDGE_DIM_OPACITY

    const focusIdx: number[] = []
    for (let i = 0; i < this.simLinks.length; i++) {
      const l = this.simLinks[i]
      const sid = typeof l.source === 'string' ? l.source : l.source.id
      const tid = typeof l.target === 'string' ? l.target : l.target.id
      const a = this.idToIndex.get(sid)
      const b = this.idToIndex.get(tid)
      if (a === this.focusedIndex || b === this.focusedIndex) {
        focusIdx.push(i)
      }
    }

    this.focusLayer.indices = focusIdx
    this.focusLayer.positions = new Float32Array(focusIdx.length * 6)
    this.focusLayer.line.visible = focusIdx.length > 0
    if (focusIdx.length > 0) this.updateLayerPositions(this.focusLayer)
  }

  private ndcToWorld(ndcX: number, ndcY: number): { x: number; y: number } {
    const rect = this.host.getBoundingClientRect()
    const halfW = (rect.width / 2) / this.camera.zoom
    const halfH = (rect.height / 2) / this.camera.zoom
    return {
      x: this.camera.position.x + ndcX * halfW,
      y: this.camera.position.y + ndcY * halfH,
    }
  }

  private updatePointerFromEvent(ev: PointerEvent) {
    const rect = this.host.getBoundingClientRect()
    this.pointer.x = ((ev.clientX - rect.left) / rect.width) * 2 - 1
    this.pointer.y = -((ev.clientY - rect.top) / rect.height) * 2 + 1
    const w = this.ndcToWorld(this.pointer.x, this.pointer.y)
    this.pointerWorld.x = w.x
    this.pointerWorld.y = w.y
  }

  private onPointerMove = (ev: PointerEvent) => {
    this.updatePointerFromEvent(ev)

    if (this.draggingIndex >= 0) {
      const moved = Math.abs(ev.clientX - this.dragStart.x) + Math.abs(ev.clientY - this.dragStart.y)
      if (!this.dragMoved && moved < DRAG_MOVE_THRESHOLD_PX) return
      if (!this.dragMoved) {
        this.dragMoved = true
        const n = this.simNodes[this.draggingIndex]
        n.fx = n.x
        n.fy = n.y
        this.sim.alphaTarget(0.25).restart()
        this.host.style.cursor = 'grabbing'
      }
      const n = this.simNodes[this.draggingIndex]
      n.fx = this.pointerWorld.x
      n.fy = this.pointerWorld.y
      return
    }

    if (this.isPanning) {
      const dx = (ev.clientX - this.panStart.x) / this.camera.zoom
      const dy = (ev.clientY - this.panStart.y) / this.camera.zoom
      if (Math.abs(ev.clientX - this.panStart.x) + Math.abs(ev.clientY - this.panStart.y) > DRAG_MOVE_THRESHOLD_PX) {
        this.panMoved = true
      }
      this.camera.position.x = this.cameraStart.x - dx
      this.camera.position.y = this.cameraStart.y + dy
      return
    }

    const prev = this.hoveredIndex
    this.hoveredIndex = this.hitTest()
    if (prev !== this.hoveredIndex) {
      this.host.style.cursor = this.hoveredIndex >= 0 ? 'pointer' : 'grab'
    }
  }

  private onPointerDown = (ev: PointerEvent) => {
    this.updatePointerFromEvent(ev)
    if (this.hoveredIndex >= 0) {
      this.draggingIndex = this.hoveredIndex
      this.dragMoved = false
      this.dragStart.x = ev.clientX
      this.dragStart.y = ev.clientY
      this.host.setPointerCapture?.(ev.pointerId)
      return
    }
    this.isPanning = true
    this.panMoved = false
    this.panStart.x = ev.clientX
    this.panStart.y = ev.clientY
    this.cameraStart.x = this.camera.position.x
    this.cameraStart.y = this.camera.position.y
    this.host.style.cursor = 'grabbing'
  }

  private onPointerUp = () => {
    if (this.draggingIndex >= 0) {
      if (this.dragMoved) {
        const n = this.simNodes[this.draggingIndex]
        n.fx = null
        n.fy = null
        this.sim.alphaTarget(0)
      }
      this.draggingIndex = -1
      this.host.style.cursor = this.hoveredIndex >= 0 ? 'pointer' : 'grab'
      return
    }
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
    if (this.dragMoved) { this.dragMoved = false; return }
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
      if (this.hiddenNodes.has(i)) continue
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
    if (next >= 0) this.sim.alpha(0.25).restart()
  }

  setHiddenNodes(ids: Set<string>) {
    const next = new Set<number>()
    ids.forEach(id => {
      const idx = this.idToIndex.get(id)
      if (idx !== undefined) next.add(idx)
    })
    this.hiddenNodes = next
    this.applySimToScene()
  }

  setHoverHighlight(id: string | null) {
    const next = id ? (this.idToIndex.get(id) ?? -1) : -1
    if (next === this.externalHoverIndex) return
    this.externalHoverIndex = next
    this.applySimToScene()
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
    this.disposeEdgeLayer(this.weakLayer)
    this.disposeEdgeLayer(this.strongLayer)
    this.disposeEdgeLayer(this.focusLayer)

    this.buildEdgeLayers(edges)

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

  private disposeEdgeLayer(layer: EdgeLayer) {
    this.scene.remove(layer.line)
    layer.geom.dispose()
    layer.mat.dispose()
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
    this.host.removeEventListener('wheel', this.onWheel)
    for (const label of this.labels) label.element.remove()
    this.nodeMesh.geometry.dispose()
    ;(this.nodeMesh.material as MeshBasicMaterial).dispose()
    this.glowMesh.geometry.dispose()
    ;(this.glowMesh.material as MeshBasicMaterial).dispose()
    this.disposeEdgeLayer(this.weakLayer)
    this.disposeEdgeLayer(this.strongLayer)
    this.disposeEdgeLayer(this.focusLayer)
    this.renderer.dispose()
    if (this.renderer.domElement.parentNode) this.renderer.domElement.remove()
    const labelEl = this.labelRenderer.domElement
    if (labelEl.parentNode) labelEl.remove()
  }
}
