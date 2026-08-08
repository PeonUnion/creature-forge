<template>
  <div class="sk3d" ref="mountEl">
    <!-- 动画播放控制 -->
    <div class="sk3d-bar">
      <button class="sk3d-btn" @click="togglePlay">{{ playing ? '⏸ 暂停' : '▶ 播放' }}</button>
      <span class="sk3d-badge">{{ frameIndex + 1 }} / {{ frames.length }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import * as THREE from 'three'
import { TrackballControls } from 'three/examples/jsm/controls/TrackballControls.js'

/**
 * 蒙皮预览（WebGL，Three.js Mesh + 每帧更新顶点）。
 *
 * - 数据：后端 skin3d_data（mesh 绑定姿态网格 + frames 每帧 flat 顶点，LBS 后端算好）
 *   （Y-down 项目坐标，此处翻转为 Y-up）
 * - 蒙皮：BufferGeometry 每帧更新 position + 重算法线（顶点跟随骨骼，由后端动作驱动）
 * - 交互：TrackballControls（左键=转动、右键=平移、滚轮=缩放）+ 播放/暂停
 * - 可选叠加绑定骨架线（半透明）辅助观察形变
 */
const props = defineProps({
  mesh: { type: Object, default: () => ({}) },        // {indices, uvs, normals, vertex_count, materials}
  frames: { type: Array, default: () => [] },          // 每帧 flat 顶点 [x,y,z,...]
  fps: { type: Number, default: 6 },
  center: { type: Array, default: () => [0, 0, 0] },
  bindJoints: { type: Object, default: () => ({}) },   // 绑定骨架（叠加参考线）
  bones: { type: Array, default: () => [] },
})
const emit = defineEmits(['ready', 'view'])

const hasFrames = computed(() => props.frames && props.frames.length > 0)
const mountEl = ref(null)
let renderer, scene, camera, controls, skinMesh = null, boneLine = null, boneGeo = null
let posAttr = null, fitTarget = new THREE.Vector3(), fitDist = 300
let rafId = null, resizeObs = null, animTimer = null
const playing = ref(false)
const frameIndex = ref(0)

// 后端 Y-down → Three.js Y-up：y 取负
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

  controls = new TrackballControls(camera, renderer.domElement)
  controls.rotateSpeed = 1.0
  controls.zoomSpeed = 1.2
  controls.panSpeed = 0.8
  controls.staticMoving = true
  controls.addEventListener('change', () => renderer.render(scene, camera))

  let pointerDown = false
  renderer.domElement.addEventListener('pointerdown', () => { pointerDown = true })
  renderer.domElement.addEventListener('pointerup', () => {
    if (pointerDown) { pointerDown = false; emit('view', viewFromCamera()) }
  })

  buildSkin()
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

function buildSkin() {
  // 清理旧网格
  if (skinMesh) { scene.remove(skinMesh); skinMesh.geometry.dispose(); skinMesh.material.dispose(); skinMesh = null }
  if (boneLine) { scene.remove(boneLine); boneGeo = null; boneLine = null }
  const m = props.mesh || {}
  const first = props.frames[0] || []
  const nv = m.vertex_count || Math.floor(first.length / 3) || 0
  if (!nv) return
  const geo = new THREE.BufferGeometry()
  posAttr = new THREE.Float32BufferAttribute(new Float32Array(first), 3)
  geo.setAttribute('position', posAttr)
  if (m.uvs && m.uvs.length) geo.setAttribute('uv', new THREE.Float32BufferAttribute(m.uvs, 2))
  if (m.normals && m.normals.length) geo.setAttribute('normal', new THREE.Float32BufferAttribute(m.normals, 3))
  if (m.indices && m.indices.length) geo.setIndex(m.indices)
  // 皮肤材质（albedo 来自外挂 materials.json，默认肤色）
  const albedo = (m.materials && m.materials.albedo) || '#c9a58c'
  const mat = new THREE.MeshStandardMaterial({
    color: albedo, roughness: (m.materials && m.materials.roughness) || 0.6,
    metalness: (m.materials && m.materials.metallic) || 0.0, side: THREE.DoubleSide,
  })
  skinMesh = new THREE.Mesh(geo, mat)
  // 灯光
  scene.add(new THREE.AmbientLight(0xffffff, 0.65))
  const dl = new THREE.DirectionalLight(0xffffff, 0.85)
  dl.position.set(300, 400, 300)
  scene.add(dl)
  scene.add(skinMesh)
  // 绑定骨架参考线（静态半透明，帮助观察蒙皮形变）
  if (props.bones && props.bones.length && Object.keys(props.bindJoints).length) {
    boneGeo = new THREE.BufferGeometry()
    const pts = []
    for (const [a, b] of props.bones) {
      if (props.bindJoints[a] && props.bindJoints[b]) {
        pts.push(...flip(props.bindJoints[a]), ...flip(props.bindJoints[b]))
      }
    }
    boneGeo.setAttribute('position', new THREE.Float32BufferAttribute(pts, 3))
    boneLine = new THREE.LineSegments(boneGeo, new THREE.LineBasicMaterial({
      color: 0x9dd6ff, transparent: true, opacity: 0.45,
    }))
    boneLine.frustumCulled = false
    scene.add(boneLine)
  }
}

/** 增量更新顶点到指定帧（只改 position + 重算法线，不重建几何） */
function updateFrame(i) {
  const f = props.frames[i]
  if (!f || !posAttr) return
  const arr = posAttr.array
  for (let k = 0; k < f.length; k++) {
    arr[k] = (k % 3 === 1) ? -f[k] : f[k]  // Y-down → Y-up
  }
  posAttr.needsUpdate = true
  if (skinMesh) skinMesh.geometry.computeVertexNormals()
  renderer.render(scene, camera)
}

function fitCamera() {
  const pts = []
  for (const f of props.frames) {
    for (let k = 0; k < f.length; k += 3) pts.push([f[k], -f[k + 1], f[k + 2]])
  }
  if (!pts.length) return
  const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity]
  for (const p of pts) for (let i = 0; i < 3; i++) {
    min[i] = Math.min(min[i], p[i]); max[i] = Math.max(max[i], p[i])
  }
  fitTarget.set((min[0] + max[0]) / 2, (min[1] + max[1]) / 2, (min[2] + max[2]) / 2)
  const radius = Math.max(...pts.map(p => Math.hypot(p[0] - fitTarget.x, p[1] - fitTarget.y, p[2] - fitTarget.z)), 10)
  fitDist = radius / Math.tan(THREE.MathUtils.degToRad(45) / 2) / 0.76
  controls.target.copy(fitTarget)
  setView(30, 12, 1)
}

function viewFromCamera() {
  const p = camera.position, t = controls.target
  const dx = p.x - t.x, dy = p.y - t.y, dz = p.z - t.z
  const dist = Math.hypot(dx, dy, dz) || 1
  const pitch = THREE.MathUtils.radToDeg(Math.asin(dy / dist))
  const yaw = ((THREE.MathUtils.radToDeg(Math.atan2(dx, dz)) % 360) + 360) % 360
  return { yaw, pitch }
}

function setView(yaw = 30, pitch = 12, dist = 1, panX = 0, panY = 0) {
  const d = fitDist * Math.max(dist, 0.01)
  const yawR = THREE.MathUtils.degToRad(yaw)
  const pitchR = THREE.MathUtils.degToRad(pitch)
  const cp = Math.cos(pitchR)
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

function togglePlay() {
  if (!hasFrames.value || !props.frames.length) return
  playing.value = !playing.value
  if (playing.value) {
    const ms = Math.max(50, Math.round(1000 / (props.fps || 6)))
    animTimer = setInterval(() => {
      frameIndex.value = (frameIndex.value + 1) % props.frames.length
      updateFrame(frameIndex.value)
    }, ms)
  } else if (animTimer) {
    clearInterval(animTimer); animTimer = null
  }
}

function animate() {
  rafId = requestAnimationFrame(animate)
  controls.update()
  renderer.render(scene, camera)
}

watch(() => props.frames, (f) => {
  if (!scene) return
  if (f && f.length) {
    frameIndex.value = 0
    if (animTimer) { clearInterval(animTimer); animTimer = null }
    buildSkin()
    fitCamera()
    renderer.render(scene, camera)
  }
}, { deep: true })

onMounted(init)
onBeforeUnmount(() => {
  if (animTimer) clearInterval(animTimer)
  if (rafId) cancelAnimationFrame(rafId)
  if (resizeObs) resizeObs.disconnect()
  if (controls) controls.dispose()
  if (renderer) { renderer.dispose(); renderer.domElement.remove() }
})

defineExpose({ setView })
</script>

<style scoped>
.sk3d { position: relative; width: 100%; min-height: 420px; border: 1px solid #111827; border-radius: 8px;
  overflow: hidden; background: #111827; }
.sk3d-bar { position: absolute; left: 10px; top: 10px; z-index: 2; display: flex; align-items: center; gap: 8px; }
.sk3d-btn { background: rgba(255,255,255,.08); color: #e5e7eb; border: 1px solid rgba(255,255,255,.15);
  border-radius: 6px; padding: 4px 12px; cursor: pointer; font-size: .82rem; }
.sk3d-btn:hover { background: rgba(255,255,255,.16); }
.sk3d-badge { color: #9ca3af; font-size: .78rem; font-family: ui-monospace, monospace; }
</style>
