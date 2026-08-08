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
          <el-button size="small" type="primary" @click="saveAction" :loading="saving" icon="Check">保存动作</el-button>
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
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
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
const atab = ref('def')

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
})

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
    cam.value = { ...cam.value, yaw: 0, pitch: 0 }
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
