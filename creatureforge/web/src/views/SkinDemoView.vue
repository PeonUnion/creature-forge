<template>
  <div class="skin-demo">
    <header class="page-header">
      <div>
        <h2>🧍 蒙皮 Demo（LBS 顶点蒙皮）</h2>
        <p class="sub">
          网格 + 权重外挂于 <code>data/species/human/skin/</code>；后端按现有真实动捕动作做 LBS 蒙皮
          （每顶点绑 ≤4 骨），前端 WebGL 逐帧更新顶点。切换动作 / 播放 / 拖拽旋转 / 滚轮缩放。
        </p>
      </div>
    </header>

    <div class="demo-controls">
      <label class="ctl-label">动作</label>
      <el-select v-model="actionId" size="small" style="width: 170px" @change="load">
        <el-option v-for="a in actions" :key="a" :label="actionTitle(a)" :value="a" />
      </el-select>
      <el-button size="small" type="primary" :loading="exporting" icon="Download" @click="exportGlb">导出 GLB</el-button>
      <span class="hint">动作数据来自 CMU 真实动捕（fk3d 每帧真实旋转）→ 蒙皮顶点跟随骨骼</span>
    </div>

    <SkinnedViewer v-if="data"
      :mesh="data.mesh" :frames="data.frames" :fps="data.fps"
      :center="data.center" :bones="data.bones" :bindJoints="data.bindJoints"
      ref="viewer" />

    <div class="stats" v-if="data">
      <span class="stat-chip">顶点 {{ data.mesh.vertex_count.toLocaleString() }}</span>
      <span class="stat-chip">三角形 {{ (data.mesh.indices.length / 3).toLocaleString() }}</span>
      <span class="stat-chip">骨骼 {{ (data.boneNames || []).length }}</span>
      <span class="stat-chip">帧 {{ data.frame_count }} @{{ data.fps }}fps</span>
      <span class="stat-chip">数据源 skin/（外挂）+ actions3d/（真实动捕）</span>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import * as THREE from 'three'
import { GLTFExporter } from 'three/examples/jsm/exporters/GLTFExporter.js'
import { api } from '../api'
import SkinnedViewer from '../components/SkinnedViewer.vue'

const actions = ['walk3d', 'run3d', 'jump3d', 'crawl3d', 'idle3d']
const actionTitle = (id) => ({
  walk3d: 'Walk 走路', run3d: 'Run 跑步', jump3d: 'Jump 跳跃',
  crawl3d: 'Crawl 爬行', idle3d: 'Idle 待机呼吸',
}[id] || id)

const actionId = ref('walk3d')
const data = ref(null)
const viewer = ref(null)
const exporting = ref(false)

async function load() {
  try {
    const r = await api.skin3dData(actionId.value, 'species=human')
    if (r.ok && r.frames) data.value = r
    else { data.value = null; ElMessage.error('蒙皮数据获取失败') }
  } catch (e) { data.value = null; ElMessage.error(e.message) }
}

// Y-down → Y-up（导出 glTF 是 Y-up）
function flipY(arr) {
  const a = new Float32Array(arr.length)
  for (let i = 0; i < arr.length; i += 3) { a[i] = arr[i]; a[i + 1] = -arr[i + 1]; a[i + 2] = arr[i + 2] }
  return a
}

/** 用外挂 skin 数据重建 THREE.SkinnedMesh（骨骼层级 + 蒙皮权重），供 GLTFExporter 导出 */
function buildSkinned(d) {
  const fkTree = d.fk_tree || {}
  const bind = d.bindJoints || {}
  const bn = d.boneNames || []
  const m = d.mesh || {}
  const nv = m.vertex_count || 0
  const indexOf = {}
  bn.forEach((n, i) => { indexOf[n] = i })
  const bones = bn.map((name) => { const b = new THREE.Bone(); b.name = name; return b })
  const byParent = {}
  for (const n of bn) { const p = fkTree[n]; if (p != null) (byParent[p] = byParent[p] || []).push(n) }
  function place(j, parentIdx) {
    const bi = indexOf[j]
    const p = bind[j]
    if (parentIdx != null) {
      const pp = bind[bn[parentIdx]]
      bones[bi].position.set(p[0] - pp[0], -(p[1] - pp[1]), p[2] - pp[2])
      bones[parentIdx].add(bones[bi])
    } else {
      bones[bi].position.set(p[0], -p[1], p[2])
    }
    for (const c of (byParent[j] || [])) place(c, bi)
  }
  for (const n of bn) if (fkTree[n] == null) place(n, null)
  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(flipY(d.frames[0] || []), 3))
  if (m.uvs && m.uvs.length) geo.setAttribute('uv', new THREE.BufferAttribute(new Float32Array(m.uvs), 2))
  if (m.normals && m.normals.length) geo.setAttribute('normal', new THREE.BufferAttribute(flipY(m.normals), 3))
  if (m.indices && m.indices.length) geo.setIndex(m.indices)
  const si = new Float32Array(nv * 4), sw = new Float32Array(nv * 4)
  ;(d.weights || []).forEach((w, vi) => {
    for (let k = 0; k < (w.indices || []).length && k < 4; k++) {
      si[vi * 4 + k] = w.indices[k]
      sw[vi * 4 + k] = w.weights[k]
    }
  })
  geo.setAttribute('skinIndex', new THREE.BufferAttribute(si, 4))
  geo.setAttribute('skinWeight', new THREE.BufferAttribute(sw, 4))
  const mat = new THREE.MeshStandardMaterial({
    color: (m.materials && m.materials.albedo) || 0xc9a58c,
    roughness: (m.materials && m.materials.roughness) || 0.6,
    metalness: 0.0, side: THREE.DoubleSide,
  })
  const skinned = new THREE.SkinnedMesh(geo, mat)
  skinned.name = 'creatureforge'  // 动画 track 前缀（GLTFExporter 解析 bones[...] 需所属节点名）
  const skeleton = new THREE.Skeleton(bones)
  skinned.add(bones[0])
  skinned.bind(skeleton)
  skeleton.update()
  skinned.updateMatrixWorld(true)
  return skinned
}

/** 从动作每帧 TRS（后端 Y-up 欧拉 + 根位移）构建 AnimationClip（骨骼局部旋转 + 根位移） */
function buildClip(d) {
  const trs = d.trs || []
  const n = trs.length
  if (!n) return null
  const fps = d.fps || 6
  const times = []
  for (let i = 0; i < n; i++) times.push(i / fps)
  const tracks = []
  const euler = new THREE.Euler()
  const quat = new THREE.Quaternion()
  const prefix = 'creatureforge'  // 与 buildSkinned 的 skinned.name 一致
  // 每骨骼局部旋转 track（quaternion keyframes）
  for (const name of (d.boneNames || [])) {
    const vals = new Float32Array(n * 4)
    trs.forEach((fr, i) => {
      const r = (fr.rot && fr.rot[name]) || [0, 0, 0]
      euler.set(r[0], r[1], r[2], 'XYZ')
      quat.setFromEuler(euler)
      vals[i * 4] = quat.x; vals[i * 4 + 1] = quat.y
      vals[i * 4 + 2] = quat.z; vals[i * 4 + 3] = quat.w
    })
    tracks.push(new THREE.QuaternionKeyframeTrack(`${prefix}.bones[${name}].quaternion`, times, vals))
  }
  // 根位移 track（position = 绑定根位置 + root3d，Y-up）
  const rootName = (d.boneNames || [])[0]
  const bindRoot = d.bindJoints && d.bindJoints[rootName]
  if (bindRoot && trs[0] && trs[0].root) {
    const vals = new Float32Array(n * 3)
    trs.forEach((fr, i) => {
      vals[i * 3] = bindRoot[0] + fr.root[0]
      vals[i * 3 + 1] = -bindRoot[1] + fr.root[1]
      vals[i * 3 + 2] = bindRoot[2] + fr.root[2]
    })
    tracks.push(new THREE.VectorKeyframeTrack(`${prefix}.bones[${rootName}].position`, times, vals))
  }
  return new THREE.AnimationClip(actionId.value, n / fps, tracks)
}

/** 导出 .glb：含骨骼 + 蒙皮网格 + 动作动画，Godot/Unity/Blender 可直接导入播放 */
async function exportGlb() {
  const d = data.value
  if (!d) { ElMessage.warning('请先加载蒙皮数据'); return }
  exporting.value = true
  try {
    const skinned = buildSkinned(d)
    const clip = buildClip(d)
    const exporter = new GLTFExporter()
    const result = await new Promise((res, rej) =>
      exporter.parse(skinned, res, (e) => rej(e),
        { binary: true, animations: clip ? [clip] : undefined }))
    const blob = result instanceof Blob ? result : new Blob([result], { type: 'model/gltf-binary' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `creatureforge_${actionId.value}.glb`
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 3000)  // 延迟回收，避免下载被取消
    const animInfo = clip ? `+${clip.tracks.length} 动画轨道` : '无动画'
    ElMessage.success(`已导出 ${actionId.value}.glb（${(blob.size / 1024).toFixed(0)}KB，含骨骼+蒙皮${animInfo}，可导入 Godot/Unity/Blender）`)
  } catch (e) { ElMessage.error('导出失败: ' + e.message) }
  exporting.value = false
}

onMounted(load)
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 14px; }
.page-header h2 { font-size: 1.2rem; margin: 0 0 6px; }
.sub { color: #86909c; font-size: .86rem; line-height: 1.6; max-width: 860px; }
.sub code { background: #f2f3f5; padding: 1px 5px; border-radius: 4px; font-size: .8rem; }
.demo-controls { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.ctl-label { font-size: .86rem; color: #4e5969; font-weight: 600; }
.hint { font-size: .8rem; color: #86909c; }
.stats { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
.stat-chip { background: #fff; border: 1px solid #e4e7ed; border-radius: 6px; padding: 4px 10px;
  font-size: .78rem; color: #4e5969; font-family: ui-monospace, monospace; }
</style>
