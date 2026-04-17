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

const NODE_BASE_SCALE = 8
const NODE_HOVER_SCALE = 12
const NODE_SEGMENTS = 24
const BG_COLOR = 0xfafafa
const NODE_COLOR = 0x18181b
const NODE_HOVER_COLOR = 0x2563eb
const EDGE_COLOR = 0xd4d4d8

export class GraphRenderer {
  private host: HTMLElement
  private scene = new Scene()
  private camera: OrthographicCamera
  private renderer: WebGLRenderer
  private labelRenderer: CSS2DRenderer

  private nodeMesh: InstancedMesh
  private edgeGeom: BufferGeometry
  private edgeLines: LineSegments
  private edgeWeights: Float32Array

  private simNodes: SimNode[]
  private simLinks: SimLink[]
  private sim: ReturnType<typeof buildSimulation>['sim']

  private labels: CSS2DObject[] = []
  private labelAnchors: Object3D[] = []

  private hoveredIndex = -1
  private onNodeClick: (node: GraphNode) => void

  private dummy = new Object3D()
  private tmpColor = new Color()
  private raycaster = new Raycaster()
  private pointer = new Vector2()

  private animationId = 0
  private resizeObserver: ResizeObserver
  private disposed = false

  private isPanning = false
  private panStart = { x: 0, y: 0 }
  private cameraStart = { x: 0, y: 0 }

  constructor(host: HTMLElement, nodes: GraphNode[], edges: GraphEdge[], onNodeClick: (n: GraphNode) => void) {
    this.host = host
    this.onNodeClick = onNodeClick

    const width = host.clientWidth
    const height = host.clientHeight

    this.camera = new OrthographicCamera(-width / 2, width / 2, height / 2, -height / 2, -1000, 1000)
    this.camera.position.z = 10

    this.renderer = new WebGLRenderer({ antialias: true, alpha: true })
    this.renderer.setPixelRatio(window.devicePixelRatio)
    this.renderer.setSize(width, height)
    this.renderer.setClearColor(BG_COLOR, 1)
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

    const geom = new CircleGeometry(1, NODE_SEGMENTS)
    const mat = new MeshBasicMaterial({ color: 0xffffff })
    this.nodeMesh = new InstancedMesh(geom, mat, Math.max(1, nodes.length))
    this.scene.add(this.nodeMesh)

    const { sim, simNodes, simLinks } = buildSimulation(nodes, edges)
    this.sim = sim
    this.simNodes = simNodes
    this.simLinks = simLinks

    this.edgeGeom = new BufferGeometry()
    const positions = new Float32Array(edges.length * 6)
    this.edgeGeom.setAttribute('position', new BufferAttribute(positions, 3))
    this.edgeWeights = new Float32Array(edges.length)
    for (let i = 0; i < edges.length; i++) this.edgeWeights[i] = edges[i].weight

    const edgeMat = new LineBasicMaterial({ color: EDGE_COLOR, transparent: true, opacity: 0.6 })
    this.edgeLines = new LineSegments(this.edgeGeom, edgeMat)
    this.scene.add(this.edgeLines)

    for (const n of simNodes) {
      const anchor = new Object3D()
      this.scene.add(anchor)
      this.labelAnchors.push(anchor)

      const div = document.createElement('div')
      div.className = 'graph-label'
      div.textContent = n.data.title || n.data.slug
      const label = new CSS2DObject(div)
      label.position.set(0, -NODE_BASE_SCALE - 6, 0)
      anchor.add(label)
      this.labels.push(label)
    }

    this.paintNodeColors()

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

  private applySimToScene() {
    for (let i = 0; i < this.simNodes.length; i++) {
      const n = this.simNodes[i]
      const scale = i === this.hoveredIndex ? NODE_HOVER_SCALE : NODE_BASE_SCALE
      this.dummy.position.set(n.x, n.y, 0)
      this.dummy.scale.set(scale, scale, 1)
      this.dummy.updateMatrix()
      this.nodeMesh.setMatrixAt(i, this.dummy.matrix)
      this.labelAnchors[i].position.set(n.x, n.y, 0)
    }
    this.nodeMesh.instanceMatrix.needsUpdate = true

    const pos = this.edgeGeom.getAttribute('position') as BufferAttribute
    const arr = pos.array as Float32Array
    for (let i = 0; i < this.simLinks.length; i++) {
      const l = this.simLinks[i]
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

  private paintNodeColors() {
    for (let i = 0; i < this.simNodes.length; i++) {
      const color = i === this.hoveredIndex ? NODE_HOVER_COLOR : NODE_COLOR
      this.tmpColor.setHex(color)
      this.nodeMesh.setColorAt(i, this.tmpColor)
    }
    if (this.nodeMesh.instanceColor) this.nodeMesh.instanceColor.needsUpdate = true
  }

  private onPointerMove = (ev: PointerEvent) => {
    if (this.isPanning) {
      const dx = (ev.clientX - this.panStart.x) / this.camera.zoom
      const dy = (ev.clientY - this.panStart.y) / this.camera.zoom
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
      this.paintNodeColors()
      this.applySimToScene()
      this.host.style.cursor = this.hoveredIndex >= 0 ? 'pointer' : 'grab'
    }
  }

  private onPointerDown = (ev: PointerEvent) => {
    if (this.hoveredIndex >= 0) return
    this.isPanning = true
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

    const prevZoom = this.camera.zoom
    const halfW = (rect.width / 2) / prevZoom
    const halfH = (rect.height / 2) / prevZoom
    const wx = this.camera.position.x + ndcX * halfW
    const wy = this.camera.position.y + ndcY * halfH

    const factor = Math.exp(-ev.deltaY * 0.0015)
    const next = Math.min(8, Math.max(0.2, prevZoom * factor))
    if (next === prevZoom) return
    this.camera.zoom = next
    this.camera.updateProjectionMatrix()

    const newHalfW = (rect.width / 2) / next
    const newHalfH = (rect.height / 2) / next
    this.camera.position.x = wx - ndcX * newHalfW
    this.camera.position.y = wy - ndcY * newHalfH
  }

  private onClick = () => {
    if (this.isPanning) return
    if (this.hoveredIndex < 0) return
    const node = this.simNodes[this.hoveredIndex]
    this.onNodeClick(node.data)
  }

  private hitTest(): number {
    this.raycaster.setFromCamera(this.pointer, this.camera)
    const worldPoint = new Vector2()
    const origin = this.raycaster.ray.origin
    worldPoint.set(origin.x, origin.y)

    let best = -1
    let bestDist = Infinity
    for (let i = 0; i < this.simNodes.length; i++) {
      const n = this.simNodes[i]
      const dx = n.x - worldPoint.x
      const dy = n.y - worldPoint.y
      const d2 = dx * dx + dy * dy
      if (d2 < NODE_HOVER_SCALE * NODE_HOVER_SCALE && d2 < bestDist) {
        bestDist = d2
        best = i
      }
    }
    return best
  }

  setEdges(edges: GraphEdge[]) {
    this.scene.remove(this.edgeLines)
    this.edgeGeom.dispose()
    ;(this.edgeLines.material as LineBasicMaterial).dispose()

    this.edgeGeom = new BufferGeometry()
    const positions = new Float32Array(Math.max(1, edges.length) * 6)
    this.edgeGeom.setAttribute('position', new BufferAttribute(positions, 3))
    this.edgeWeights = new Float32Array(edges.length)
    for (let i = 0; i < edges.length; i++) this.edgeWeights[i] = edges[i].weight

    const edgeMat = new LineBasicMaterial({ color: EDGE_COLOR, transparent: true, opacity: 0.6 })
    this.edgeLines = new LineSegments(this.edgeGeom, edgeMat)
    this.scene.add(this.edgeLines)

    this.simLinks = edges.map(e => ({ source: e.source, target: e.target, weight: e.weight }))
    const linkForce = forceLink<SimNode, SimLink>(this.simLinks)
      .id(d => d.id)
      .distance(l => 60 / Math.max(0.35, l.weight))
      .strength(l => Math.min(1, l.weight))
    this.sim.force('link', linkForce)
    this.sim.alpha(0.3).restart()
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
    for (const label of this.labels) {
      label.element.remove()
    }
    this.nodeMesh.geometry.dispose()
    ;(this.nodeMesh.material as MeshBasicMaterial).dispose()
    this.edgeGeom.dispose()
    ;(this.edgeLines.material as LineBasicMaterial).dispose()
    this.renderer.dispose()
    if (this.renderer.domElement.parentNode) this.renderer.domElement.remove()
    const labelEl = this.labelRenderer.domElement
    if (labelEl.parentNode) labelEl.remove()
  }
}
