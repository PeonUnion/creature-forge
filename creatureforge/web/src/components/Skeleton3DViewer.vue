<template>
  <div class="sk3d" ref="mountEl">
    <!-- 动作动画播放控制（仅 frames 模式显示） -->
    <div v-if="hasFrames" class="sk3d-bar">
      <button class="sk3d-btn" @click="togglePlay">{{ playing ? '⏸ 暂停' : '▶ 播放' }}</button>
      <span class="sk3d-badge">{{ frameIndex + 1 }} / {{ frames.length }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import * as THREE from 'three'
import { TrackballControls } from 'three/examples/jsm/controls/TrackballControls.js'
import { GIFEncoder, quantize, applyPalette } from 'gifenc'

/**
 * WebGL 实时 3D 查看器（Three.js）。
 *
 * - 数据：后端 skeleton3d_data（joints）或 motion3d_data（frames 每帧 joints）
 *   （Y-down 项目坐标，此处翻转为 Y-up）
 * - 交互：TrackballControls（左键=转动模型本体=把玩手办，右键=平移，滚轮/中键=缩放）
 * - 地面网格：GridHelper（脚部平面）；动画模式有播放/暂停 + 帧计数
 * - setView(yaw, pitch, dist, panX, panY)：快捷视角 / 相机面板控制（与后端 PNG 同语义）
 * - emit('view', {yaw, pitch})：拖拽结束时同步相机角度（父组件用于快捷按钮高亮）
 */
const props = defineProps({
  joints: { type: Object, default: () => ({}) },  // 静态骨架（无 frames 时）
  bones: { type: Array, default: () => [] },
  center: { type: Array, default: () => [0, 0, 0] },
  headRadius: { type: Number, default: 22 },
  frames: { type: Array, default: () => [] },    // 动作帧：每帧 joints（WebGL 动画）
  fps: { type: Number, default: 6 },
  highlight: { type: String, default: '' },      // 高亮关节（含其后代子树），''=无
  editable: { type: Boolean, default: false },   // 编辑模式：可按住高亮关节拖拽平移
})
const emit = defineEmits(['ready', 'view', 'pick', 'dragend'])

const hasFrames = computed(() => props.frames && props.frames.length > 0)
const mountEl = ref(null)
let renderer, scene, camera, controls, skeletonGroup = null, grid = null
let jointsMeshes = new Map()
let meshBaseScale = new Map()      // 每个关节球的基础 scale（高亮放大用）
let bonesLine = null
let bonesGeo = null
let bonesLineHi = null
let bonesGeoHi = null
let hiSet = new Set()              // 当前高亮关节集合（选中关节 + 后代）
let dragging = null                // 拖拽中：{ name, lastX, lastY }
let dragAcc = { dx: 0, dy: 0, dz: 0 }
let rafId = null, resizeObs = null
let fitDist = 300, fitTarget = new THREE.Vector3()

// 播放状态（frames 模式）
const playing = ref(false)
const frameIndex = ref(0)
let animTimer = null

// 后端 Y-down → Three.js Y-up：y 取负（X/Z 不变）
const flip = (p) => [p[0], -p[1], p[2]]

function init() {
  const el = mountEl.value
  const w = el.clientWidth || 640, h = el.clientHeight || 480

  scene = new THREE.Scene()
  scene.background = new THREE.Color(0x111827)

  camera = new THREE.PerspectiveCamera(45, w / h, 1, 10000)

  renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.setSize(w, h)
  el.appendChild(renderer.domElement)

  // 把玩手办：左键 rotate（物体跟随鼠标）、右键 pan、滚轮 zoom
  controls = new TrackballControls(camera, renderer.domElement)
  controls.rotateSpeed = 1.0
  controls.zoomSpeed = 1.2
  controls.panSpeed = 0.8
  controls.staticMoving = true
  controls.addEventListener('change', () => renderer.render(scene, camera))

  // 拖拽编辑 / 视角旋转 / 点击拾取 三合一：
  //   - editable && 按住高亮关节 → 拖拽平移该关节（禁用视角旋转）
  //   - 未拖拽的短点击 → 拾取关节 emit('pick')
  //   - 拖拽视角 → 结束 emit('view')
  let pointerDown = false, downX = 0, downY = 0
  renderer.domElement.addEventListener('pointerdown', (e) => {
    pointerDown = true; downX = e.clientX; downY = e.clientY
    if (props.editable && props.highlight && hitJoint(e) === props.highlight) {
      dragging = { name: props.highlight, lastX: e.clientX, lastY: e.clientY }
      dragAcc = { dx: 0, dy: 0, dz: 0 }
      controls.enabled = false            // 拖拽期间禁止手办旋转
      renderer.domElement.style.cursor = 'grabbing'
    }
  })
  renderer.domElement.addEventListener('pointermove', (e) => {
    if (!dragging) return
    const dxPx = e.clientX - dragging.lastX
    const dyPx = e.clientY - dragging.lastY
    dragging.lastX = e.clientX; dragging.lastY = e.clientY
    const w = screenToWorld(dxPx, dyPx)
    dragAcc.dx += w.dx; dragAcc.dy += w.dy; dragAcc.dz += w.dz
    const mesh = jointsMeshes.get(dragging.name)
    if (mesh) mesh.position.add(new THREE.Vector3(w.dx, w.dy, w.dz)) // 本地即时视觉
    renderer.render(scene, camera)
  })
  renderer.domElement.addEventListener('pointerup', (e) => {
    if (dragging) {
      emit('dragend', { name: dragging.name, dx: dragAcc.dx, dy: dragAcc.dy, dz: dragAcc.dz })
      dragging = null; dragAcc = { dx: 0, dy: 0, dz: 0 }
      controls.enabled = true
      renderer.domElement.style.cursor = ''
      return
    }
    if (!pointerDown) return
    pointerDown = false
    if (Math.hypot(e.clientX - downX, e.clientY - downY) < 6) pickAt(e)
    emit('view', viewFromCamera())
  })

  buildSkeleton()
  fitCamera()
  animate()

  resizeObs = new ResizeObserver(() => {
    const ww = el.clientWidth, hh = el.clientHeight
    if (!ww || !hh) return
    camera.aspect = ww / hh
    camera.updateProjectionMatrix()
    renderer.setSize(ww, hh)
    renderer.render(scene, camera)
  })
  resizeObs.observe(el)

  emit('ready', { setView })
}

function buildSkeleton() {
  if (skeletonGroup) { scene.remove(skeletonGroup); skeletonGroup = null }
  jointsMeshes = new Map()
  meshBaseScale = new Map()
  skeletonGroup = new THREE.Group()

  // 骨骼线：普通（蓝）+ 高亮（橙，选中关节及其子树）
  bonesGeo = new THREE.BufferGeometry()
  bonesLine = new THREE.LineSegments(bonesGeo, new THREE.LineBasicMaterial({ color: 0x9dd6ff }))
  bonesLine.frustumCulled = false
  skeletonGroup.add(bonesLine)
  bonesGeoHi = new THREE.BufferGeometry()
  bonesLineHi = new THREE.LineSegments(bonesGeoHi,
    new THREE.LineBasicMaterial({ color: 0xffb020, transparent: true, opacity: 0.95 }))
  bonesLineHi.frustumCulled = false
  skeletonGroup.add(bonesLineHi)

  // 关节球（位置随动画逐帧更新；头为椭圆，同 2D 渲染）
  const base = hasFrames.value ? props.frames[0] : props.joints
  for (const [name, p] of Object.entries(base)) {
    const r = isHead(name) ? props.headRadius : Math.max(props.headRadius * 0.35, 3)
    const mesh = new THREE.Mesh(
      new THREE.SphereGeometry(r, 12, 12),
      new THREE.MeshBasicMaterial({ color: baseColor(name) }),
    )
    if (isHead(name)) mesh.scale.set(0.78, 1, 0.78) // 高 > 宽 的椭圆（Y-up：Y 为高）
    meshBaseScale.set(name, mesh.scale.clone())
    jointsMeshes.set(name, mesh)
    skeletonGroup.add(mesh)
  }
  scene.add(skeletonGroup)
  applyHighlight()
}

/** 增量更新骨架到指定帧（动画时只改 position，不重建几何，GPU 友好） */
function updateSkeleton(joints) {
  if (!skeletonGroup) return
  for (const [name, mesh] of jointsMeshes) {
    const p = joints[name]
    if (p) { mesh.visible = true; mesh.position.set(...flip(p)) }
    else mesh.visible = false
  }
  const positions = [], positionsHi = []
  for (const [a, b] of props.bones) {
    const pa = joints[a], pb = joints[b]
    if (!pa || !pb) continue
    const hi = hiSet.has(a) || hiSet.has(b)
    ;(hi ? positionsHi : positions).push(...flip(pa), ...flip(pb))
  }
  bonesGeo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3))
  bonesGeo.attributes.position.needsUpdate = true
  bonesGeoHi.setAttribute('position', new THREE.Float32BufferAttribute(positionsHi, 3))
  bonesGeoHi.attributes.position.needsUpdate = true
}

const isHead = (n) => n === 'head' || n === 'head_left' || n === 'head_right'
const baseColor = (n) => (isHead(n) ? 0x9dd6ff : 0xfff1a8)

/** 计算高亮集合：选中关节 + 其全部后代（bones 父→子边推导） */
function computeHighlightSet(name) {
  const set = new Set()
  if (!name) return set
  set.add(name)
  let changed = true
  while (changed) {
    changed = false
    for (const [a, b] of props.bones) {
      if (set.has(a) && !set.has(b)) { set.add(b); changed = true }
    }
  }
  return set
}

/** 应用高亮：选中关节及其子树关节球变橙放大，相关骨头线切到橙色高亮线 */
function applyHighlight() {
  if (!skeletonGroup) return
  hiSet = computeHighlightSet(props.highlight)
  for (const [name, mesh] of jointsMeshes) {
    const hi = hiSet.has(name)
    mesh.material.color.set(hi ? 0xffb020 : baseColor(name))
    const base = meshBaseScale.get(name)
    if (base) {
      const f = hi ? 1.45 : 1
      mesh.scale.set(base.x * f, base.y * f, base.z * f)
    }
  }
  updateSkeleton(hasFrames.value ? props.frames[frameIndex.value] : props.joints)
}

/** 命中检测：返回点击/按下处最近的关节名（射线到球心距离 + 放宽阈值，小球也好点中） */
function hitJoint(e) {
  if (!renderer || !jointsMeshes.size) return null
  const rect = renderer.domElement.getBoundingClientRect()
  const ndc = new THREE.Vector2(
    ((e.clientX - rect.left) / rect.width) * 2 - 1,
    -((e.clientY - rect.top) / rect.height) * 2 + 1,
  )
  const raycaster = new THREE.Raycaster()
  raycaster.setFromCamera(ndc, camera)
  const ray = raycaster.ray
  const tmp = new THREE.Vector3()
  let best = null, bestD = Infinity
  for (const [name, mesh] of jointsMeshes) {
    if (!mesh.visible) continue
    mesh.getWorldPosition(tmp)
    const d = ray.distanceToPoint(tmp)
    const base = meshBaseScale.get(name) || mesh.scale
    const radius = (mesh.geometry.parameters?.radius || 4) * Math.max(base.x, base.y, base.z)
    const hitR = Math.max(radius * 2.5, 8)
    if (d < hitR && d < bestD) { bestD = d; best = name }
  }
  return best
}

/** 点击拾取 → emit('pick', name) */
function pickAt(e) {
  const name = hitJoint(e)
  if (name) emit('pick', name)
}

/** 屏幕像素位移 → 世界位移（沿相机右/上向量映射，随视角方向移动，符合直觉） */
function screenToWorld(dxPx, dyPx) {
  const vh = renderer.domElement.clientHeight || 1
  const k = (2 * fitDist * Math.tan(THREE.MathUtils.degToRad(camera.fov / 2))) / vh
  const right = new THREE.Vector3().setFromMatrixColumn(camera.matrixWorld, 0)
  const up = new THREE.Vector3().setFromMatrixColumn(camera.matrixWorld, 1)
  return {
    dx: (right.x * dxPx + up.x * -dyPx) * k,
    dy: (right.y * dxPx + up.y * -dyPx) * k,
    dz: (right.z * dxPx + up.z * -dyPx) * k,
  }
}

function fitCamera() {
  // 所有帧（或静态骨架）包围球 → 相机适配距离 + 目标
  const pts = []
  const src = hasFrames.value ? props.frames : [props.joints]
  for (const j of src) for (const p of Object.values(j)) pts.push(flip(p))
  if (!pts.length) return
  const min = [Infinity, Infinity, Infinity]
  const max = [-Infinity, -Infinity, -Infinity]
  for (const p of pts) {
    for (let i = 0; i < 3; i++) {
      min[i] = Math.min(min[i], p[i]); max[i] = Math.max(max[i], p[i])
    }
  }
  fitTarget.set((min[0] + max[0]) / 2, (min[1] + max[1]) / 2, (min[2] + max[2]) / 2)
  const radius = Math.max(...pts.map(p => Math.hypot(p[0] - fitTarget.x, p[1] - fitTarget.y, p[2] - fitTarget.z)), 10)
  fitDist = radius / Math.tan(THREE.MathUtils.degToRad(45) / 2) / 0.76
  controls.target.copy(fitTarget)

  // 地面网格（脚部平面，Y-up = -脚部Y；取全部帧脚部最低点保证模型不穿地）
  if (grid) scene.remove(grid)
  const groundY = -Math.max(...src.map(j => Math.max(...Object.values(j).map(p => p[1]))))
  const size = radius * 2.8
  grid = new THREE.GridHelper(size, 12, 0x4b5e7a, 0x3a4a5f)
  grid.position.y = groundY
  grid.position.x = fitTarget.x
  grid.position.z = fitTarget.z
  scene.add(grid)

  setView(30, 12, 1)
}

/** 从当前相机位置推导 yaw/pitch（拖拽结束同步用） */
function viewFromCamera() {
  const p = camera.position, t = controls.target
  const dx = p.x - t.x, dy = p.y - t.y, dz = p.z - t.z
  const dist = Math.hypot(dx, dy, dz) || 1
  const pitch = THREE.MathUtils.radToDeg(Math.asin(dy / dist))
  const yaw = ((THREE.MathUtils.radToDeg(Math.atan2(dx, dz)) % 360) + 360) % 360
  return { yaw, pitch }
}

/**
 * 外部视角控制（快捷视角 / 相机面板）：yaw/pitch/dist(倍数)/pan 与后端 PNG 渲染同语义。
 */
function setView(yaw = 30, pitch = 12, dist = 1, panX = 0, panY = 0) {
  const d = fitDist * Math.max(dist, 0.01)
  const yawR = THREE.MathUtils.degToRad(yaw)
  const pitchR = THREE.MathUtils.degToRad(pitch)
  const cp = Math.cos(pitchR)
  // Y-up 球坐标：pitch 正=相机上移俯视
  camera.position.set(
    fitTarget.x + d * cp * Math.sin(yawR),
    fitTarget.y + d * Math.sin(pitchR),
    fitTarget.z + d * cp * Math.cos(yawR),
  )
  camera.lookAt(fitTarget)
  controls.target.copy(fitTarget)
  controls.update()
  renderer.render(scene, camera)
}

// -- 动作动画播放 --

function togglePlay() {
  if (!hasFrames.value) return
  playing.value = !playing.value
  if (playing.value) {
    const ms = Math.max(50, Math.round(1000 / (props.fps || 6)))
    animTimer = setInterval(() => {
      frameIndex.value = (frameIndex.value + 1) % props.frames.length
      updateSkeleton(props.frames[frameIndex.value])
    }, ms)
  } else if (animTimer) {
    clearInterval(animTimer); animTimer = null
  }
}

/**
 * 导出 GIF（所见即所得：当前 Three.js 视图 = 当前相机 + 地面网格 + 骨架逐帧截帧）。
 * 返回 GIF Blob；null 表示无动画帧。
 */
async function exportGif({ fps, maxWidth = 640, onProgress } = {}) {
  if (!hasFrames.value || !renderer) return null
  const fpsNum = fps || props.fps || 6
  const delay = Math.max(20, Math.round(1000 / fpsNum))
  // 暂停播放避免冲突，导出后恢复
  const wasPlaying = playing.value
  if (wasPlaying) togglePlay()
  try {
    const srcW = renderer.domElement.width
    const srcH = renderer.domElement.height
    const scale = Math.min(1, maxWidth / (srcW || 1))
    const w = Math.max(1, Math.round(srcW * scale))
    const h = Math.max(1, Math.round(srcH * scale))
    const canvas = document.createElement('canvas')
    canvas.width = w; canvas.height = h
    const ctx = canvas.getContext('2d')
    const gif = GIFEncoder()
    for (let i = 0; i < props.frames.length; i++) {
      updateSkeleton(props.frames[i])
      renderer.render(scene, camera)
      const img = new Image()
      const url = renderer.domElement.toDataURL('image/png')
      await new Promise((res, rej) => { img.onload = res; img.onerror = rej; img.src = url })
      ctx.clearRect(0, 0, w, h)
      ctx.drawImage(img, 0, 0, w, h)
      const { data } = ctx.getImageData(0, 0, w, h)
      const palette = quantize(data, 256)
      const index = applyPalette(data, palette)
      gif.writeFrame(index, w, h, { palette, delay })
      if (onProgress) onProgress(i + 1, props.frames.length)
      await new Promise(r => setTimeout(r, 0)) // 让 UI 保持响应
    }
    gif.finish()
    return new Blob([gif.bytes()], { type: 'image/gif' })
  } finally {
    if (wasPlaying) togglePlay()
  }
}

function animate() {
  rafId = requestAnimationFrame(animate)
  controls.update()
  renderer.render(scene, camera)
}

// 动作帧变化 → 重置并渲染首帧
watch(() => props.frames, (f) => {
  if (!scene) return
  if (f && f.length) {
    frameIndex.value = 0
    if (animTimer) { clearInterval(animTimer); animTimer = null }
    buildSkeleton()
    fitCamera()
    renderer.render(scene, camera)
  }
}, { deep: true })

// 静态骨架变化（无 frames 时）
watch(() => props.joints, () => {
  if (!scene || hasFrames.value) return
  buildSkeleton()
  fitCamera()
  renderer.render(scene, camera)
}, { deep: true })

// 高亮变化 → 应用高亮（不重建场景）
watch(() => props.highlight, () => { if (scene) applyHighlight() })

onMounted(init)

onBeforeUnmount(() => {
  if (animTimer) clearInterval(animTimer)
  if (rafId) cancelAnimationFrame(rafId)
  if (resizeObs) resizeObs.disconnect()
  if (controls) { controls.dispose() }
  if (renderer) {
    renderer.dispose()
    renderer.domElement.remove()
  }
})

defineExpose({ setView, exportGif })
</script>

<style scoped>
.sk3d { position: relative; width: 100%; min-height: 380px; border: 1px solid #111827; border-radius: 8px;
  overflow: hidden; background: #111827; }
.sk3d :deep(canvas) { display: block; }
.sk3d-bar { position: absolute; left: 10px; top: 10px; z-index: 2; display: flex; align-items: center; gap: 8px; }
.sk3d-btn {
  background: rgba(0,0,0,.65); color: #fff; border: 1px solid #4b5e7a; border-radius: 6px;
  padding: 4px 12px; font-size: .8rem; cursor: pointer;
}
.sk3d-btn:hover { background: rgba(75,94,122,.6); }
.sk3d-badge { background: rgba(0,0,0,.65); color: #fff; font-size: .75rem;
  padding: 3px 8px; border-radius: 999px; }
</style>
