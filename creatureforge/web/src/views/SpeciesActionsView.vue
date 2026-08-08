<template>
  <div class="actions-view">
    <div class="acts-head">
      <div class="crumb">
        <span class="crumb-root">物种</span><span class="crumb-sep">/</span>
        <span class="crumb-now">{{ speciesTitle }}</span><span class="crumb-sep">/</span>
        <span class="crumb-now">动作</span>
      </div>
      <div class="head-actions">
        <el-button size="small" @click="emit('back')">返回列表</el-button>
        <el-button size="small" type="primary" @click="startCreateAction" icon="Plus">新建动作</el-button>
      </div>
    </div>

    <!-- 动作列表 -->
    <div v-if="!actionEditor" class="acts-list">
      <p class="hint">动作存放于 species/{{ speciesId }}/actions3d/（3D 关节旋转，引擎插值驱动）。</p>
      <el-table :data="actionList" size="small" border>
        <el-table-column label="动作" min-width="180">
          <template #default="{row}">
            <div class="cell-main"><span class="cell-title">🎬 {{ row.title || row.motion_id }}</span><span class="cell-id mono">{{ row.motion_id }}</span></div>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="参数" width="180">
          <template #default="{row}">
            <el-tag v-for="p in Object.keys(row.params||{}).slice(0,3)" :key="p" size="small" effect="plain">{{ p }}</el-tag>
            <span v-if="Object.keys(row.params||{}).length>3" class="cell-id">+{{ Object.keys(row.params).length-3 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" align="center">
          <template #default="{row}">
            <el-button size="small" text type="primary" @click="openAction(row.motion_id)">编辑</el-button>
            <el-button size="small" text type="danger" @click="confirmDeleteAction(row.motion_id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!actionList.length" class="empty-inline">该物种暂无动作，点击「新建动作」创建</div>
    </div>

    <!-- 动作编辑器 -->
    <div v-else class="acts-editor">
      <div class="acts-head">
        <div class="crumb">
          <span class="crumb-root">动作</span><span class="crumb-sep">/</span>
          <span class="crumb-now">{{ actionEditor.motion_id || '新动作' }}</span>
        </div>
        <div class="head-actions">
          <el-button size="small" @click="actionEditor = null">关闭</el-button>
          <el-button size="small" type="primary" @click="saveAction" :loading="saving" icon="Check">
            保存动作<span v-if="dirty" class="dirty-dot" title="有未保存的修改" />
          </el-button>
        </div>
      </div>

      <div class="stat-cards">
        <div class="stat-card"><div class="stat-val">{{ actionEditor.motion_id || '-' }}</div><div class="stat-label">动作 ID</div></div>
        <div class="stat-card"><div class="stat-val">{{ actionEditor.frame_count || '-' }}</div><div class="stat-label">帧数</div></div>
        <div class="stat-card"><div class="stat-val">{{ Object.keys(actionEditor.params||{}).length }}</div><div class="stat-label">可调参数</div></div>
        <div class="stat-card"><div class="stat-val">{{ speciesId }}</div><div class="stat-label">所属物种</div></div>
      </div>

      <el-tabs v-model="atab">
        <!-- 普通：语义化编辑（基本信息 + 参数），非 JSON -->
        <el-tab-pane label="🧩 普通（语义化）" name="def">
          <el-form label-position="top" class="form-grid">
            <el-form-item label="动作 ID"><el-input v-model="actionEditor.motion_id" placeholder="如 fly3d" /></el-form-item>
            <el-form-item label="名称"><el-input v-model="actionEditor.title" placeholder="如 飞行" /></el-form-item>
            <el-form-item label="帧数"><el-input-number v-model="actionEditor.frame_count" :min="2" :max="120" /></el-form-item>
            <el-form-item label="描述"><el-input v-model="actionEditor.description" /></el-form-item>
          </el-form>
          <div class="sec">
            <div class="sec-t">动作参数（可调幅度等，随动作数据派生）</div>
            <div v-for="(spec, pkey) in (actionEditor.params||{})" :key="pkey" class="param-row">
              <span class="mono pkey">{{ pkey }}</span>
              <el-input v-model="spec.label" size="small" placeholder="中文名" style="width: 110px" />
              <el-input-number v-model="spec.min" size="small" :step="0.05" style="width: 88px" />
              <el-input-number v-model="spec.max" size="small" :step="0.05" style="width: 88px" />
              <el-input-number v-model="spec.default" size="small" :step="0.05" style="width: 88px" />
              <el-button size="small" text type="danger" @click="rmParam(pkey)">删</el-button>
            </div>
            <el-button size="small" @click="addParam" icon="Plus">加参数</el-button>
            <span class="hint">旋转/位移数据（fk3d）在高级 JSON 中；预览用物种默认动作模板，或由动作向导（后续）生成关键帧。</span>
          </div>
        </el-tab-pane>

        <!-- 高级：完整 JSON（与普通模式共享 actionEditor） -->
        <el-tab-pane label="⚙️ 高级 JSON" name="advanced">
          <el-input v-model="actionJson" type="textarea" :rows="18" class="mono json-editor" spellcheck="false" />
          <div class="json-actions">
            <el-button size="small" @click="syncJsonToEditor">应用 JSON 到普通模式</el-button>
          </div>
        </el-tab-pane>

        <!-- 逐帧编辑：一帧一帧编辑关节旋转，确认全部帧后保存 -->
        <el-tab-pane label="🎬 逐帧编辑" name="frame">
          <div v-if="frameJoints && frameJoints.length" class="frame-grid">
            <div class="wiz-preview">
              <Skeleton3DViewer v-if="frameData" :joints="frameData.joints" :bones="frameData.bones"
                :head-radius="frameData.head_radius" :center="frameData.center" />
              <div v-else class="preview-empty"><p>切换帧实时显示该帧姿态（未保存编辑）</p></div>
            </div>
            <div class="frame-controls">
              <div class="sec">
                <div class="sec-t">帧导航（第 {{ frameIdx + 1 }} / {{ frameCount }} 帧）</div>
                <div class="row3 wrap">
                  <el-button size="small" :disabled="frameIdx <= 0" @click="gotoFrame(0)">⏮ 首帧</el-button>
                  <el-button size="small" :disabled="frameIdx <= 0" @click="gotoFrame(frameIdx - 1)">◀</el-button>
                  <el-input-number v-model="frameIdx" size="small" :min="0" :max="frameCount - 1" @change="onFrameIdx" style="flex: 1" />
                  <el-button size="small" :disabled="frameIdx >= frameCount - 1" @click="gotoFrame(frameIdx + 1)">▶</el-button>
                  <el-button size="small" :disabled="frameIdx >= frameCount - 1" @click="gotoFrame(frameCount - 1)">末帧 ⏭</el-button>
                </div>
                <p class="hint small">逐帧编辑关节旋转（写回 fk3d.rotations3d 的 table 波形），确认全部帧后点「保存动作」。无 table 的轴为表达式驱动，只读。</p>
              </div>
              <div class="sec">
                <div class="sec-t">第 {{ frameIdx + 1 }} 帧 · 关节旋转（弧度，X / Y / Z）</div>
                <div class="fr-table">
                  <div v-for="r in frameRotRows" :key="r.joint" class="fr-row">
                    <span class="mono fr-joint">{{ r.joint }}</span>
                    <template v-for="ax in ['x', 'y', 'z']" :key="ax">
                      <el-input-number v-if="r.axes[ax]" v-model="r.axes[ax].val" size="small" :step="0.05"
                        @change="writeFrame(r.joint, ax)" />
                      <span v-else class="fr-na">—</span>
                    </template>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="empty-inline">该动作没有 fk3d.rotations3d 关键帧；可在高级 JSON 中定义后「应用 JSON」再逐帧编辑。</div>
        </el-tab-pane>

        <el-tab-pane label="👁 动作预览" name="preview">
          <div class="preview-controls">
            <CameraControls v-model="cam" compact />
            <el-button size="small" type="primary" @click="renderAction" :loading="motionRenderLoading" icon="Refresh">渲染</el-button>
            <el-button size="small" :disabled="!motionData" :loading="gifLoading" icon="Download" @click="exportMotionGif">导出 GIF</el-button>
          </div>
          <Skeleton3DViewer v-if="motionData" ref="motionViewer"
            :frames="motionData.frames" :bones="motionData.bones"
            :head-radius="motionData.head_radius" :center="motionData.center"
            :fps="motionData.fps"
            @view="cam = { ...cam, yaw: $event.yaw, pitch: $event.pitch }" />
          <div class="preview-empty" v-else><p>点击「渲染」加载 3D 动作预览（左键拖拽=转动手办 · 右键平移 · 滚轮缩放）</p></div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api.js'
import CameraControls from '../components/CameraControls.vue'
import Skeleton3DViewer from '../components/Skeleton3DViewer.vue'

const props = defineProps({ speciesId: { type: String, required: true } })
const emit = defineEmits(['back', 'saved'])

const speciesTitle = ref(props.speciesId)
const actionList = ref([])
const actionEditor = ref(null)
const actionJson = ref('')
const saving = ref(false)
const dirty = ref(false)  // 有未保存修改（本地暂存，点「保存动作」确认）
const atab = ref('def')

// 逐帧编辑状态
const frameIdx = ref(0)
const frameCount = ref(1)
const frameData = ref(null)
const frameRotRows = ref([])
const frameJoints = ref([])

const motionData = ref(null)
const motionViewer = ref(null)
const gifLoading = ref(false)
const motionRenderLoading = ref(false)
const cam = ref({ yaw: 30, pitch: 12, dist: 1, panX: 0, panY: 0 })
let lastMotionId = ''

// 普通/高级 共享 actionEditor；切页签同步 JSON
watch(atab, (m) => {
  if (m === 'advanced' && actionEditor.value) {
    actionJson.value = JSON.stringify(actionEditor.value, null, 2)
  }
  if (m === 'frame' && actionEditor.value) {
    frameIdx.value = 0
    loadFrameRows()
    refreshFramePreview()
  }
})

// 动作编辑本地暂存：任何修改标记 dirty（点保存才落盘）
watch(actionEditor, () => { if (actionEditor.value) dirty.value = true }, { deep: true })

// -- 逐帧编辑：table 波形读写 + 当前帧实时预览 --
function findTable(expr) {
  if (expr == null) return null
  if (Array.isArray(expr)) {
    for (const it of expr) { const r = findTable(it); if (r) return r }
    return null
  }
  if (typeof expr !== 'object') return null
  for (const [k, v] of Object.entries(expr)) {
    if (k === 'table' && Array.isArray(v)) return v
    const r = findTable(v)
    if (r) return r
  }
  return null
}
function loadFrameRows() {
  const act = actionEditor.value
  frameCount.value = Math.max(1, Number(act?.frame_count) || 1)
  const fk = act?.fk3d?.rotations3d || {}
  const joints = Object.keys(fk)
  frameJoints.value = joints
  if (!joints.length) { frameRotRows.value = []; frameData.value = null; return }
  frameRotRows.value = joints.map(j => {
    const comp = fk[j] || {}
    const axes = {}
    for (const ax of ['x', 'y', 'z']) {
      const tbl = findTable(comp[ax + '_rot'])
      axes[ax] = tbl ? { tbl, val: Number(tbl[frameIdx.value] ?? 0) } : null
    }
    return { joint: j, axes }
  })
}
function writeFrame(joint, axis) {
  const row = frameRotRows.value.find(r => r.joint === joint)
  const a = row?.axes?.[axis]
  if (!a?.tbl) return
  a.tbl[frameIdx.value] = Number(a.val) || 0   // 写回 table 波形
  dirty.value = true
  refreshFramePreview()                        // 当前帧旋转变化 → 刷新预览
}
async function gotoFrame(i) {
  frameIdx.value = Math.max(0, Math.min(Number(i) || 0, frameCount.value - 1))
  loadFrameRows()
  await refreshFramePreview()
}
async function onFrameIdx() { await gotoFrame(frameIdx.value) }
async function refreshFramePreview() {
  if (!actionEditor.value) return
  try {
    const r = await api.motion3dLive(actionEditor.value, props.speciesId, frameIdx.value)
    frameData.value = r && r.ok ? r : null
  } catch (e) { frameData.value = null }
}

function addParam() {
  if (!actionEditor.value) return
  actionEditor.value.params = actionEditor.value.params || {}
  const n = Object.keys(actionEditor.value.params).length + 1
  const key = `param${n}`
  actionEditor.value.params[key] = { label: `参数${n}`, min: 0.5, max: 1.5, step: 0.05, default: 1.0 }
}
function rmParam(pkey) {
  if (!actionEditor.value) return
  delete actionEditor.value.params[pkey]
}
function syncJsonToEditor() {
  try {
    const d = JSON.parse(actionJson.value)
    if (!d.motion_id) { ElMessage.warning('JSON 缺少 motion_id'); return }
    d.species = props.speciesId
    actionEditor.value = d
    ElMessage.success('已应用 JSON，普通模式已同步')
  } catch (e) { ElMessage.error('JSON 语法错误: ' + e.message) }
}

async function loadSpecies() {
  try {
    const d = await api.speciesDetail(props.speciesId)
    speciesTitle.value = d.title || props.speciesId
    actionList.value = d.actions || []
  } catch (e) { ElMessage.error(e.message) }
}

async function openAction(actionId) {
  try {
    const act = await api.actionDetail(props.speciesId, actionId)
    actionEditor.value = act
    actionJson.value = JSON.stringify(act, null, 2)
    atab.value = 'def'
    motionData.value = null
    frameData.value = null
    frameJoints.value = []
    cam.value = { ...cam.value, yaw: 0, pitch: 0 }
    await nextTick(); dirty.value = false
  } catch (e) { ElMessage.error(e.message) }
}

function startCreateAction() {
  actionEditor.value = {
    schema: 'creatureforge_motion3d_v1', motion_id: '', title: '', description: '',
    species: props.speciesId, frame_count: 8, params: {},
    root3d: { dy: { phase: true } }, offsets3d: {}, ik3d: {},
  }
  actionJson.value = JSON.stringify(actionEditor.value, null, 2)
  atab.value = 'def'
  motionData.value = null
  frameData.value = null
  frameJoints.value = []
  nextTick(() => { dirty.value = false })
}

async function saveAction() {
  saving.value = true
  try {
    // 普通模式编辑 actionEditor；若在高级 JSON 页签先应用
    if (atab.value === 'advanced') {
      const d = JSON.parse(actionJson.value)
      if (!d.motion_id) { ElMessage.warning('motion_id 不能为空'); saving.value = false; return }
      d.species = props.speciesId
      actionEditor.value = d
    }
    const data = JSON.parse(JSON.stringify(actionEditor.value))
    if (!data.motion_id) { ElMessage.warning('motion_id 不能为空'); saving.value = false; return }
    data.species = props.speciesId
    if (actionEditor.value?.motion_id && actionEditor.value.motion_id !== data.motion_id) {
      await api.createAction(props.speciesId, data)
      await api.deleteAction(props.speciesId, actionEditor.value.motion_id)
    } else if (actionEditor.value?.motion_id) {
      await api.updateAction(props.speciesId, data.motion_id, data)
    } else {
      await api.createAction(props.speciesId, data)
    }
    ElMessage.success('动作已保存')
    dirty.value = false
    actionEditor.value = null
    await loadSpecies()
    emit('saved')
  } catch (e) { ElMessage.error('保存失败: ' + e.message) }
  saving.value = false
}

async function confirmDeleteAction(actionId) {
  try {
    await ElMessageBox.confirm(`确定删除动作「${actionId}」吗？`, '确认', { type: 'warning' })
    await api.deleteAction(props.speciesId, actionId)
    ElMessage.success('已删除')
    await loadSpecies()
  } catch (e) { if (e !== 'cancel') ElMessage.error(e.message || e) }
}

async function renderAction() {
  if (!actionEditor.value?.motion_id) { ElMessage.warning('请先填写 motion_id 并保存'); return }
  motionRenderLoading.value = true
  try {
    const switching = lastMotionId && lastMotionId !== actionEditor.value.motion_id
    const trans = switching ? `&transition_from=${encodeURIComponent(lastMotionId)}` : ''
    const r = await api.motion3dData(actionEditor.value.motion_id, `species=${encodeURIComponent(props.speciesId)}${trans}`)
    if (r.ok && r.frames) motionData.value = r
    else { motionData.value = null; ElMessage.error('动作数据获取失败') }
    lastMotionId = actionEditor.value.motion_id
  } catch (e) { ElMessage.error(e.message) }
  motionRenderLoading.value = false
}

async function exportMotionGif() {
  const viewer = motionViewer.value
  if (!viewer || !motionData.value) { ElMessage.warning('请先渲染动作'); return }
  gifLoading.value = true
  try {
    const blob = await viewer.exportGif()
    if (blob) {
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${actionEditor.value.motion_id}.gif`
      document.body.appendChild(a); a.click(); a.remove()
      setTimeout(() => URL.revokeObjectURL(url), 3000)
      ElMessage.success('GIF 已导出（当前 Three.js 视图）')
    } else ElMessage.error('GIF 生成失败')
  } catch (e) { ElMessage.error(e.message) }
  gifLoading.value = false
}

watch(cam, () => {
  if (motionData.value && motionViewer.value) {
    motionViewer.value.setView(cam.value.yaw, cam.value.pitch, cam.value.dist, cam.value.panX, cam.value.panY)
  }
}, { deep: true })

onMounted(async () => { await loadSpecies() })
onBeforeUnmount(() => {
  if (motionViewer.value) motionViewer.value = null
})
</script>

<style scoped>
.actions-view { padding: 4px 0; }
.dirty-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #f56c6c; margin-left: 6px; vertical-align: middle; }
.frame-grid { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr); gap: 16px; align-items: start; }
.frame-controls { display: flex; flex-direction: column; gap: 14px; max-height: calc(100vh - 260px); overflow-y: auto; }
.fr-table { display: flex; flex-direction: column; gap: 4px; max-height: 55vh; overflow-y: auto; }
.fr-row { display: flex; align-items: center; gap: 6px; font-size: .8rem; }
.fr-joint { width: 150px; flex: 0 0 auto; }
.fr-row .el-input-number { width: 112px; flex: 0 0 auto; }
.fr-na { width: 112px; flex: 0 0 auto; display: inline-flex; justify-content: center; color: #c0c4cc; }
.acts-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.crumb { font-size: .9rem; }
.crumb-root { color: #909399; } .crumb-sep { color: #c0c4cc; } .crumb-now { font-weight: 600; }
.hint { color: #909399; font-size: .82rem; margin: 0 0 12px; }
.stat-cards { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 16px; }
.stat-card { background: #f8fafc; border: 1px solid #ebeef5; border-radius: 8px; padding: 10px 14px; text-align: center; }
.stat-val { font-size: 1.15rem; font-weight: 700; color: #409eff; }
.stat-label { font-size: .72rem; color: #909399; margin-top: 2px; }
.preview-controls { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; flex-wrap: wrap; }
.preview-empty { border: 1px dashed #d9d9d9; border-radius: 8px; min-height: 260px; display: flex; align-items: center; justify-content: center; color: #c0c4cc; }
.cell-main { display: flex; align-items: center; gap: 8px; }
.cell-title { font-weight: 600; font-size: .85rem; }
.cell-id { color: #909399; font-size: .72rem; font-family: monospace; }
.empty-inline { padding: 24px; text-align: center; color: #c0c4cc; }
.mono { font-family: monospace; }
.json-editor { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.form-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 8px; }
.sec { border: 1px solid #ebeef5; border-radius: 8px; padding: 10px 12px; background: #fafbfc; display: flex; flex-direction: column; gap: 8px; margin-top: 8px; }
.sec-t { font-size: .78rem; font-weight: 600; color: #606266; }
.param-row { display: flex; align-items: center; gap: 6px; }
.pkey { width: 110px; color: #606266; overflow: hidden; text-overflow: ellipsis; }
.json-actions { margin-top: 8px; }
</style>
