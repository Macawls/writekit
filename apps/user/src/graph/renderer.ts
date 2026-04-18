import {
  Scene,
  OrthographicCamera,
  PerspectiveCamera,
  WebGLRenderer,
  InstancedMesh,
  CircleGeometry,
  SphereGeometry,
  MeshBasicMaterial,
  MeshPhongMaterial,
  AmbientLight,
  DirectionalLight,
  Object3D,
  Color,
  Raycaster,
  Vector2,
  Vector3,
} from 'three'
import { CSS2DRenderer, CSS2DObject } from 'three/addons/renderers/CSS2DRenderer.js'
import { TrackballControls } from 'three/addons/controls/TrackballControls.js'
import { LineSegments2 } from 'three/addons/lines/LineSegments2.js'
import { LineSegmentsGeometry } from 'three/addons/lines/LineSegmentsGeometry.js'
import { LineMaterial } from 'three/addons/lines/LineMaterial.js'
import { forceLink } from 'd3-force-3d'
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
const NODE_DIM_COLOR = 0xd4d4d8

const EDGE_STRONG_THRESHOLD = 0.45
const EDGE_WEAK_COLOR = 0xd4d4d8
const EDGE_STRONG_COLOR = 0x71717a
const EDGE_FOCUS_COLOR = 0x18181b
const EDGE_WEAK_WIDTH = 1.4
const EDGE_STRONG_WIDTH = 2.4
const EDGE_FOCUS_WIDTH = 3.0
const EDGE_WEAK_OPACITY = 0.55
const EDGE_STRONG_OPACITY = 0.75
const EDGE_DIM_OPACITY = 0.08

const ZOOM_MIN = 0.15
const ZOOM_MAX = 8
const FIT_PADDING = 0.85

const DRAG_MOVE_THRESHOLD_PX = 4

const NODE_CAPACITY_GROWTH = 1.5

export type ViewMode = '2d' | '3d'

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
  private camera: OrthographicCamera | PerspectiveCamera
  private renderer: WebGLRenderer
  private labelRenderer: CSS2DRenderer
  private controls: TrackballControls | null = null

  private mode: ViewMode = '2d'

  private nodeMesh!: InstancedMesh
  private nodeCapacity = 0
  private lights: Object3D[] = []

  private weakLayer!: EdgeLayer
  private strongLayer!: EdgeLayer
  private focusLayer!: EdgeLayer

  private simNodes: SimNode[] = []
  private simLinks: SimLink[] = []
  private sim!: ReturnType<typeof buildSimulation>['sim']

  private labels: CSS2DObject[] = []
  private labelAnchors: Object3D[] = []
  private edgeLabels: CSS2DObject[] = []
  private edgeLabelAnchors: Object3D[] = []
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

  constructor(host: HTMLElement, nodes: GraphNode[], edges: GraphEdge[], callbacks: Callbacks, mode: ViewMode = '2d') {
    this.host = host
    this.callbacks = callbacks
    this.mode = mode

    const width = host.clientWidth
    const height = host.clientHeight

    this.camera = this.makeCamera(mode, width, height)

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
    labelEl.style.zIndex = '1'
    host.appendChild(labelEl)

    this.buildNodeMesh(Math.max(8, nodes.length))
    if (this.mode === '3d') this.addLights()

    const { sim, simNodes, simLinks } = buildSimulation(nodes, edges, this.mode)
    this.sim = sim
    this.simNodes = simNodes
    this.simLinks = simLinks

    simNodes.forEach((n, i) => this.idToIndex.set(n.id, i))
    this.buildEdgeLayers(edges)
    simNodes.forEach((n, i) => this.addLabelFor(n, i))

    this.paintNodes()

    sim.on('tick', () => this.applySimToScene())
    this.applySimToScene()

    this.attachInputHandlers()
    if (this.mode === '3d') this.enableTrackball()

    this.resizeObserver = new ResizeObserver(() => this.resize())
    this.resizeObserver.observe(host)

    this.loop()
  }

  private makeCamera(mode: ViewMode, width: number, height: number): OrthographicCamera | PerspectiveCamera {
    if (mode === '3d') {
      const cam = new PerspectiveCamera(55, width / height, 1, 10000)
      cam.position.set(0, 0, 600)
      return cam
    }
    const cam = new OrthographicCamera(-width / 2, width / 2, height / 2, -height / 2, -1000, 1000)
    cam.position.z = 10
    return cam
  }

  private enableTrackball() {
    const controls = new TrackballControls(this.camera, this.renderer.domElement)
    controls.rotateSpeed = 3.0
    controls.zoomSpeed = 1.2
    controls.panSpeed = 0.8
    controls.noZoom = false
    controls.noPan = false
    controls.staticMoving = false
    controls.dynamicDampingFactor = 0.2
    this.controls = controls
  }

  private attachInputHandlers() {
    this.host.addEventListener('pointermove', this.onPointerMove)
    this.host.addEventListener('click', this.onClick)
    this.host.addEventListener('pointerdown', this.onPointerDown)
    this.host.addEventListener('pointerup', this.onPointerUp)
    if (this.mode === '2d') this.host.addEventListener('wheel', this.onWheel, { passive: false })
  }

  private detachInputHandlers() {
    this.host.removeEventListener('pointermove', this.onPointerMove)
    this.host.removeEventListener('click', this.onClick)
    this.host.removeEventListener('pointerdown', this.onPointerDown)
    this.host.removeEventListener('pointerup', this.onPointerUp)
    this.host.removeEventListener('wheel', this.onWheel)
  }

  private buildNodeMesh(capacity: number) {
    const geom = this.mode === '3d'
      ? new SphereGeometry(1, 24, 20)
      : new CircleGeometry(1, NODE_SEGMENTS)
    const mat = this.mode === '3d'
      ? new MeshPhongMaterial({ color: 0xffffff, shininess: 32, specular: 0x222222 })
      : new MeshBasicMaterial({ color: 0xffffff })
    this.nodeMesh = new InstancedMesh(geom, mat, capacity)
    this.nodeMesh.count = 0
    this.nodeCapacity = capacity
    this.scene.add(this.nodeMesh)
  }

  private addLights() {
    if (this.lights.length > 0) return
    const ambient = new AmbientLight(0xffffff, 0.55)
    const key = new DirectionalLight(0xffffff, 0.9)
    key.position.set(200, 400, 600)
    const rim = new DirectionalLight(0xffffff, 0.35)
    rim.position.set(-300, -200, -400)
    this.scene.add(ambient, key, rim)
    this.lights.push(ambient, key, rim)
  }

  private removeLights() {
    for (const l of this.lights) this.scene.remove(l)
    this.lights = []
  }

  private ensureNodeCapacity(n: number) {
    if (n <= this.nodeCapacity) return
    const newCap = Math.max(n, Math.ceil(this.nodeCapacity * NODE_CAPACITY_GROWTH))
    this.scene.remove(this.nodeMesh)
    this.nodeMesh.geometry.dispose()
    ;(this.nodeMesh.material as { dispose?: () => void }).dispose?.()
    this.buildNodeMesh(newCap)
  }

  private addLabelFor(n: SimNode, _i: number) {
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
    label.position.set(0, -NODE_BASE_SCALE - 7, 0)
    anchor.add(label)
    this.labels.push(label)
  }

  private removeLabelAt(i: number) {
    const label = this.labels[i]
    if (label) label.element.remove()
    const anchor = this.labelAnchors[i]
    if (anchor) this.scene.remove(anchor)
    this.labels.splice(i, 1)
    this.labelAnchors.splice(i, 1)
  }

  setGraph(nodes: GraphNode[], edges: GraphEdge[]) {
    const incomingIds = new Set(nodes.map(n => n.id))
    const nodeById = new Map(nodes.map(n => [n.id, n]))

    let centroidX = 0, centroidY = 0, centroidZ = 0, kept = 0
    for (const sn of this.simNodes) {
      if (incomingIds.has(sn.id)) {
        centroidX += sn.x; centroidY += sn.y; centroidZ += sn.z; kept++
      }
    }
    if (kept > 0) {
      centroidX /= kept; centroidY /= kept; centroidZ /= kept
    }

    const surviving: SimNode[] = []
    for (let i = 0; i < this.simNodes.length; i++) {
      const sn = this.simNodes[i]
      if (incomingIds.has(sn.id)) {
        const updatedData = nodeById.get(sn.id)!
        sn.data = updatedData
        surviving.push(sn)
      } else {
        this.removeLabelAt(surviving.length)
      }
    }

    const survivingIds = new Set(surviving.map(n => n.id))
    for (const n of nodes) {
      if (!survivingIds.has(n.id)) {
        const jitter = () => (Math.random() - 0.5) * 40
        const newNode: SimNode = {
          id: n.id,
          data: n,
          x: centroidX + jitter(),
          y: centroidY + jitter(),
          z: this.mode === '3d' ? centroidZ + jitter() : 0,
          vx: 0, vy: 0, vz: 0,
        }
        surviving.push(newNode)
        this.addLabelFor(newNode, surviving.length - 1)
      }
    }

    this.simNodes = surviving
    this.idToIndex.clear()
    this.simNodes.forEach((sn, i) => this.idToIndex.set(sn.id, i))

    this.ensureNodeCapacity(this.simNodes.length)
    this.nodeMesh.count = this.simNodes.length
    this.sim.nodes(this.simNodes)

    this.setEdges(edges, { restart: true })
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
      arr[i * 6 + 2] = s.z ?? 0
      arr[i * 6 + 3] = t.x
      arr[i * 6 + 4] = t.y
      arr[i * 6 + 5] = t.z ?? 0
    }
    layer.geom.setPositions(arr)
  }

  private loop = () => {
    if (this.disposed) return
    if (this.controls) this.controls.update()
    this.renderer.render(this.scene, this.camera)
    this.labelRenderer.render(this.scene, this.camera)
    this.animationId = requestAnimationFrame(this.loop)
  }

  private resize() {
    const width = this.host.clientWidth
    const height = this.host.clientHeight
    if (this.camera instanceof OrthographicCamera) {
      this.camera.left = -width / 2
      this.camera.right = width / 2
      this.camera.top = height / 2
      this.camera.bottom = -height / 2
    } else {
      this.camera.aspect = width / height
    }
    this.camera.updateProjectionMatrix()
    this.renderer.setSize(width, height)
    this.labelRenderer.setSize(width, height)
    this.weakLayer?.mat.resolution.set(width, height)
    this.strongLayer?.mat.resolution.set(width, height)
    this.focusLayer?.mat.resolution.set(width, height)
    if (this.controls) this.controls.handleResize()
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
      const z = this.mode === '3d' ? n.z : 0
      this.dummy.position.set(n.x, n.y, z)
      this.dummy.scale.set(scale, scale, scale)
      this.dummy.updateMatrix()
      this.nodeMesh.setMatrixAt(i, this.dummy.matrix)

      const label = this.labels[i]?.element as HTMLElement | undefined
      if (label) label.style.display = hidden ? 'none' : ''
      if (this.labelAnchors[i]) this.labelAnchors[i].position.set(n.x, n.y, z)
    }
    this.nodeMesh.instanceMatrix.needsUpdate = true

    this.updateLayerPositions(this.weakLayer)
    this.updateLayerPositions(this.strongLayer)
    if (this.focusLayer.indices.length > 0) {
      this.updateLayerPositions(this.focusLayer)
      this.updateEdgeLabels()
    }
  }

  private updateEdgeLabels() {
    for (let i = 0; i < this.focusLayer.indices.length; i++) {
      const l = this.simLinks[this.focusLayer.indices[i]]
      const s = l.source as SimNode
      const t = l.target as SimNode
      const anchor = this.edgeLabelAnchors[i]
      if (!anchor) continue
      const z = this.mode === '3d' ? ((s.z + t.z) / 2) : 0
      anchor.position.set((s.x + t.x) / 2, (s.y + t.y) / 2, z)
    }
  }

  private isHighlighted(i: number): boolean {
    if (this.focusedIndex < 0) return true
    if (i === this.focusedIndex) return true
    return this.neighbors[this.focusedIndex]?.has(i) ?? false
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
    this.clearEdgeLabels()

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
    if (focusIdx.length > 0) {
      this.updateLayerPositions(this.focusLayer)
      this.createEdgeLabels(focusIdx)
    }
  }

  private createEdgeLabels(indices: number[]) {
    for (const ei of indices) {
      const l = this.simLinks[ei]
      const anchor = new Object3D()
      this.scene.add(anchor)
      this.edgeLabelAnchors.push(anchor)

      const div = document.createElement('div')
      div.className = 'graph-edge-label'
      div.textContent = Math.round(l.weight * 100) + '%'
      const label = new CSS2DObject(div)
      anchor.add(label)
      this.edgeLabels.push(label)
    }
    this.updateEdgeLabels()
  }

  private clearEdgeLabels() {
    for (const lbl of this.edgeLabels) lbl.element.remove()
    for (const anchor of this.edgeLabelAnchors) this.scene.remove(anchor)
    this.edgeLabels = []
    this.edgeLabelAnchors = []
  }

  private ndcToWorld(ndcX: number, ndcY: number): { x: number; y: number } {
    const rect = this.host.getBoundingClientRect()
    const cam = this.camera as OrthographicCamera
    const halfW = (rect.width / 2) / cam.zoom
    const halfH = (rect.height / 2) / cam.zoom
    return {
      x: cam.position.x + ndcX * halfW,
      y: cam.position.y + ndcY * halfH,
    }
  }

  private updatePointerFromEvent(ev: PointerEvent) {
    const rect = this.host.getBoundingClientRect()
    this.pointer.x = ((ev.clientX - rect.left) / rect.width) * 2 - 1
    this.pointer.y = -((ev.clientY - rect.top) / rect.height) * 2 + 1
    if (this.mode === '2d') {
      const w = this.ndcToWorld(this.pointer.x, this.pointer.y)
      this.pointerWorld.x = w.x
      this.pointerWorld.y = w.y
    }
  }

  private onPointerMove = (ev: PointerEvent) => {
    this.updatePointerFromEvent(ev)

    if (this.mode === '2d' && this.draggingIndex >= 0) {
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

    if (this.mode === '2d' && this.isPanning) {
      const dx = (ev.clientX - this.panStart.x) / (this.camera as OrthographicCamera).zoom
      const dy = (ev.clientY - this.panStart.y) / (this.camera as OrthographicCamera).zoom
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
      this.host.style.cursor = this.hoveredIndex >= 0 ? 'pointer' : (this.mode === '2d' ? 'grab' : 'default')
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
    if (this.mode === '2d') {
      this.isPanning = true
      this.panMoved = false
      this.panStart.x = ev.clientX
      this.panStart.y = ev.clientY
      this.cameraStart.x = this.camera.position.x
      this.cameraStart.y = this.camera.position.y
      this.host.style.cursor = 'grabbing'
    }
  }

  private onPointerUp = () => {
    if (this.draggingIndex >= 0) {
      if (this.mode === '2d' && this.dragMoved) {
        const n = this.simNodes[this.draggingIndex]
        n.fx = null
        n.fy = null
        this.sim.alphaTarget(0)
      }
      this.draggingIndex = -1
      this.host.style.cursor = this.hoveredIndex >= 0 ? 'pointer' : (this.mode === '2d' ? 'grab' : 'default')
      return
    }
    if (this.isPanning) {
      this.isPanning = false
      this.host.style.cursor = this.hoveredIndex >= 0 ? 'pointer' : 'grab'
    }
  }

  private onWheel = (ev: WheelEvent) => {
    if (this.mode !== '2d') return
    ev.preventDefault()
    const rect = this.host.getBoundingClientRect()
    const ndcX = ((ev.clientX - rect.left) / rect.width) * 2 - 1
    const ndcY = -((ev.clientY - rect.top) / rect.height) * 2 + 1
    const factor = Math.exp(-ev.deltaY * 0.0015)
    this.zoomAtNdc(factor, ndcX, ndcY)
  }

  private zoomAtNdc(factor: number, ndcX: number, ndcY: number) {
    if (!(this.camera instanceof OrthographicCamera)) return
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
    if (this.mode === '3d') {
      let best = -1
      let bestDist = Infinity
      const v = new Vector3()
      for (let i = 0; i < this.simNodes.length; i++) {
        if (this.hiddenNodes.has(i)) continue
        const n = this.simNodes[i]
        v.set(n.x, n.y, n.z)
        const d = this.raycaster.ray.distanceSqToPoint(v)
        const r = this.nodeScale(i) * 1.5
        if (d < r * r && d < bestDist) {
          bestDist = d
          best = i
        }
      }
      return best
    }
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
    if (this.mode === '2d') this.zoomAtNdc(factor, 0, 0)
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
    if (this.camera instanceof OrthographicCamera) {
      const spanX = Math.max(1, maxX - minX)
      const spanY = Math.max(1, maxY - minY)
      const rect = this.host.getBoundingClientRect()
      const zoomX = (rect.width * FIT_PADDING) / spanX
      const zoomY = (rect.height * FIT_PADDING) / spanY
      this.camera.zoom = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Math.min(zoomX, zoomY)))
      this.camera.position.x = cx
      this.camera.position.y = cy
      this.camera.updateProjectionMatrix()
    } else {
      this.camera.position.set(cx, cy, 600)
      this.camera.lookAt(cx, cy, 0)
      this.controls?.target.set(cx, cy, 0)
    }
  }

  setEdges(edges: GraphEdge[], opts: { restart?: boolean } = {}) {
    this.disposeEdgeLayer(this.weakLayer)
    this.disposeEdgeLayer(this.strongLayer)
    this.disposeEdgeLayer(this.focusLayer)

    this.buildEdgeLayers(edges)

    this.simLinks = edges.map(e => ({ source: e.source, target: e.target, weight: e.weight }))
    const linkForce = forceLink<SimNode, SimLink>(this.simLinks)
      .id((d: SimNode) => d.id)
      .distance((l: SimLink) => 180 / Math.max(0.35, l.weight))
      .strength((l: SimLink) => 0.45 * Math.min(1, l.weight))
    this.sim.force('link', linkForce)
    if (opts.restart ?? true) this.sim.alpha(0.3).restart()
    this.paintNodes()
    this.applyEdgeFocus()
  }

  setMode(mode: ViewMode) {
    if (mode === this.mode) return
    this.mode = mode

    if (this.controls) { this.controls.dispose(); this.controls = null }
    this.detachInputHandlers()

    const rect = this.host.getBoundingClientRect()
    this.camera = this.makeCamera(mode, rect.width, rect.height)

    this.scene.remove(this.nodeMesh)
    this.nodeMesh.geometry.dispose()
    ;(this.nodeMesh.material as { dispose?: () => void }).dispose?.()
    this.buildNodeMesh(Math.max(8, this.simNodes.length, this.nodeCapacity))
    this.nodeMesh.count = this.simNodes.length

    if (mode === '3d') {
      this.addLights()
      for (const n of this.simNodes) if (n.z === 0) n.z = (Math.random() - 0.5) * 200
    } else {
      this.removeLights()
      for (const n of this.simNodes) { n.z = 0; n.vz = 0 }
    }

    const edgesFromLinks: GraphEdge[] = this.simLinks.map(l => ({
      source: typeof l.source === 'string' ? l.source : (l.source as SimNode).id,
      target: typeof l.target === 'string' ? l.target : (l.target as SimNode).id,
      weight: l.weight,
    }))

    this.sim.stop()
    const { sim } = buildSimulation(this.simNodes.map(sn => sn.data), edgesFromLinks, mode)
    sim.nodes(this.simNodes)
    this.sim = sim

    this.setEdges(edgesFromLinks, { restart: true })

    this.sim.on('tick', () => this.applySimToScene())
    this.sim.alpha(0.5).restart()

    this.attachInputHandlers()
    if (mode === '3d') this.enableTrackball()

    this.paintNodes()
    this.applySimToScene()
  }

  private disposeEdgeLayer(layer: EdgeLayer | undefined) {
    if (!layer) return
    this.scene.remove(layer.line)
    layer.geom.dispose()
    layer.mat.dispose()
  }

  dispose() {
    this.disposed = true
    cancelAnimationFrame(this.animationId)
    this.sim.stop()
    this.resizeObserver.disconnect()
    this.detachInputHandlers()
    if (this.controls) this.controls.dispose()
    this.removeLights()
    for (const label of this.labels) label.element.remove()
    this.clearEdgeLabels()
    this.nodeMesh.geometry.dispose()
    ;(this.nodeMesh.material as { dispose?: () => void }).dispose?.()
    this.disposeEdgeLayer(this.weakLayer)
    this.disposeEdgeLayer(this.strongLayer)
    this.disposeEdgeLayer(this.focusLayer)
    this.renderer.dispose()
    if (this.renderer.domElement.parentNode) this.renderer.domElement.remove()
    const labelEl = this.labelRenderer.domElement
    if (labelEl.parentNode) labelEl.remove()
  }
}
