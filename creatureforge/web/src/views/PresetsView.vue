<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2>🎨 预设管理</h2>
        <p class="page-desc">预设 = 基于物种的具体实例：体型（骨骼尺寸）+ 动作（幅度）。表格 + 详情页维护。</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="startCreate" icon="Plus">新建预设</el-button>
      </div>
    </div>

    <!-- 列表表格 -->
    <section class="list-view" v-if="!current && !creating">
      <el-table :data="presetList" border stripe>
        <el-table-column label="预设" min-width="200">
          <template #default="{row}">
            <div class="cell-main"><span class="cell-title">🎨 {{ row.title || row.preset_id }}</span><span class="cell-id mono">{{ row.preset_id }}</span></div>
          </template>
        </el-table-column>
        <el-table-column label="物种" width="140">
          <template #default="{row}"><el-tag size="small" effect="plain">🦴 {{ row.species }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="220" show-overflow-tooltip />
        <el-table-column label="操作" width="180" align="center">
          <template #default="{row}">
            <el-button size="small" text type="primary" @click="openPreset(row)">编辑</el-button>
            <el-button size="small" text type="danger" @click="confirmDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!presetList.length" class="empty-list">
        <div class="empty-icon">🎨</div><p>暂无预设</p>
        <el-button type="primary" @click="startCreate">新建预设</el-button>
      </div>
    </section>

    <!-- 详情：体型 / 动作 / 预览 / 蒙皮 -->
    <section class="detail-view" v-else-if="current">
      <div class="detail-head">
        <div class="crumb">
          <span class="crumb-root">预设</span><span class="crumb-sep">/</span><span class="crumb-now">{{ current.title || current.preset_id }}</span>
        </div>
        <div class="head-actions">
          <el-button size="small" @click="close">返回列表</el-button>
          <el-button size="small" type="primary" @click="save" :loading="saving" icon="Check">保存预设</el-button>
        </div>
      </div>

      <el-form label-position="top" class="form-grid">
        <el-form-item label="预设 ID"><el-input v-model="current.preset_id" :disabled="!isNew" placeholder="如 model_male" /></el-form-item>
        <el-form-item label="名称"><el-input v-model="current.title" placeholder="如 模特男" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="current.description" /></el-form-item>
        <el-form-item label="物种（schema 来源）"><el-tag effect="plain">🦴 {{ current.species }}</el-tag></el-form-item>
      </el-form>

      <el-tabs v-model="tab">
        <el-tab-pane label="📐 体型参数" name="body">
          <p class="hint">调整骨骼尺寸（来自物种骨架 param_chains 派生 schema，default 为物种默认）。</p>
          <div v-if="bodyParamItems.length" class="param-grid">
            <div v-for="it in bodyParamItems" :key="it.key" class="param-item">
              <div class="param-head">
                <label :title="it.key">{{ it.label }}</label>
                <span class="val">{{ round(current.body[it.key] ?? it.def) }}</span>
              </div>
              <el-slider :min="it.min" :max="it.max" :step="it.step" :show-tooltip="false"
                         :model-value="current.body[it.key] ?? it.def" @update:model-value="setBody(it.key, $event)" />
            </div>
          </div>
          <div v-else class="preview-empty"><p>该物种没有体型参数</p></div>
        </el-tab-pane>

        <el-tab-pane label="🏃 动作幅度" name="actions">
          <p class="hint">调整各动作幅度（来自动作 JSON params 派生，default 为真实数据值）。可切「表达式」绑定体型/坐标参数（如 <span class="mono">param:overall_scale</span> 或 <span class="mono">mul:shoulder_width*1.2</span>），渲染时求值。</p>
          <div v-for="(a, aid) in schema.actions" :key="aid" class="action-card">
            <div class="action-head"><span>{{ a.title || aid }}</span><span class="mono">{{ aid }}</span></div>
            <div v-if="Object.keys(a.params||{}).length" class="param-grid">
              <div v-for="(spec, pkey) in a.params" :key="pkey" class="param-item">
                <div class="param-head">
                  <label :title="pkey">{{ spec.label || pkey }}</label>
                  <span class="val">{{ actExprMode[aid + ':' + pkey] ? '🔗 表达式' : actDisplay(aid, pkey, spec) }}</span>
                  <el-button size="small" text type="primary" @click="toggleActExpr(aid, pkey)">
                    {{ actExprMode[aid + ':' + pkey] ? '数值' : '表达式' }}
                  </el-button>
                </div>
                <el-slider v-if="!actExprMode[aid + ':' + pkey]" :min="spec.min" :max="spec.max" :step="spec.step||0.01" :show-tooltip="false"
                           :model-value="actNum(aid, pkey, spec)" @update:model-value="setAction(aid, pkey, $event)" />
                <el-input v-else size="small" :model-value="actExprDraft[aid + ':' + pkey]"
                          @update:model-value="onActExpr(aid, pkey, $event)"
                          placeholder="数值 或 param:p / neg:p / mul:p*k / add:p+k" />
              </div>
            </div>
            <span v-else class="no-params">该动作无可调参数（数据驱动，无预设可调项）</span>
          </div>
        </el-tab-pane>

        <el-tab-pane label="👁 预览" name="preview">
          <div class="preview-controls">
            <CameraControls v-model="cam" />
            <el-select v-model="previewAction" placeholder="骨架（应用体型）" clearable filterable style="width: 200px">
              <el-option v-for="(a, aid) in schema.actions" :key="aid" :label="`动作：${a.title||aid}`" :value="aid" />
            </el-select>
          </div>
          <Skeleton3DViewer v-if="previewData" ref="previewViewer"
            :joints="previewData.joints" :frames="previewData.frames" :bones="previewData.bones"
            :head-radius="previewData.head_radius" :center="previewData.center"
            :fps="previewData.fps"
            @view="cam = { ...cam, yaw: $event.yaw, pitch: $event.pitch }" />
          <div v-else class="preview-empty"><p>{{ rendering ? '渲染中…' : '调整参数自动渲染预览' }}</p></div>
        </el-tab-pane>

        <el-tab-pane label="🧍 蒙皮" name="skinning">
          <div class="preview-controls">
            <CameraControls v-model="cam" />
            <el-select v-model="previewSkinId" placeholder="选择皮肤" clearable filterable style="width: 210px">
              <el-option v-for="s in currentSkins" :key="s.skin_id"
                         :label="`${s.title||s.skin_id} (${s.skin_id})`" :value="s.skin_id" />
            </el-select>
            <el-select v-model="previewAction" placeholder="选择动作" clearable filterable style="width: 150px">
              <el-option v-for="(a, aid) in schema.actions" :key="aid" :label="a.title||aid" :value="aid" />
            </el-select>
            <el-button size="small" type="primary" :loading="exporting" icon="Download"
                       :disabled="!previewSkinId || !previewAction" @click="exportGlb">导出 GLB</el-button>
            <span class="hint">预设（体型+动作）+ 皮肤（材质+体态）→ 蒙皮 → 导出</span>
          </div>
          <SkinnedViewer v-if="skinPreviewData" ref="skinViewer"
            :mesh="skinPreviewData.mesh" :frames="skinPreviewData.frames" :fps="skinPreviewData.fps"
            :center="skinPreviewData.center"
            @view="cam = { ...cam, yaw: $event.yaw, pitch: $event.pitch }" />
          <div v-else class="preview-empty"><p>{{ skinRendering ? '渲染中…' : '选择皮肤 + 动作加载蒙皮预览（后端 LBS，应用预设体型/动作 + 皮肤材质/体态）' }}</p></div>
        </el-tab-pane>
      </el-tabs>
    </section>

    <!-- 新建：选物种 -->
    <section class="detail-view" v-else-if="creating">
      <h4 class="panel-title">新建预设 — 选择物种</h4>
      <p class="hint">物种提供体型参数 schema（骨骼尺寸）与各动作参数 schema（动作幅度）。</p>
      <div class="create-row">
        <el-select v-model="newSpeciesId" placeholder="选择物种" style="width: 300px" filterable>
          <el-option v-for="s in speciesList" :key="s.id"
                     :label="`${s.title} (${s.id}) · ${(s.actions||[]).length} 动作`" :value="s.id" />
        </el-select>
        <el-button type="primary" :disabled="!newSpeciesId" @click="initNew" icon="Right">初始化预设</el-button>
        <el-button @click="creating = false">取消</el-button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { api } from '../api.js'
import { ElMessage, ElMessageBox } from 'element-plus'
import CameraControls from '../components/CameraControls.vue'
import Skeleton3DViewer from '../components/Skeleton3DViewer.vue'
import SkinnedViewer from '../components/SkinnedViewer.vue'

const loading = ref(true)
const saving = ref(false)
const presetList = ref([])
const speciesList = ref([])
const skinList = ref([])
const current = ref(null)
const isNew = ref(false)
const creating = ref(false)
const newSpeciesId = ref('')
const tab = ref('body')

const currentSkins = computed(() => skinList.value.filter(s => s.preset === current.value?.preset_id))

const cam = ref({ yaw: 30, pitch: 12, dist: 1, panX: 0, panY: 0 })
const previewAction = ref('')
const lastPreviewAction = ref('')
const previewData = ref(null)
const previewViewer = ref(null)
const rendering = ref(false)
let renderTimer = null

const previewSkinId = ref('')
const skinPreviewData = ref(null)
const skinViewer = ref(null)
const skinRendering = ref(false)
const exporting = ref(false)
let skinTimer = null

const camQS = () => `yaw=${cam.value.yaw}&pitch=${cam.value.pitch}&dist=${cam.value.dist}&pan_x=${cam.value.panX}&pan_y=${cam.value.panY}`

const schema = computed(() => current.value?.schema_info || { body_params: {}, actions: {} })
const bodyParamItems = computed(() => {
  const bp = schema.value.body_params || {}
  return Object.entries(bp).map(([key, spec]) => ({
    key, label: spec.label || key, min: spec.min, max: spec.max,
    step: spec.step || 0.01, def: spec.default ?? 1.0,
  }))
})

const round = (v) => (typeof v === 'number' ? Math.round(v * 100) / 100 : v)

// -- 动作幅度表达式（与坐标参数化一致：数值=常量，语法 param:p/neg:p/mul:p*k/add:p+k/const:v/JSON=表达式） --
const actExprMode = ref({})    // {aid:pkey: bool} 表达式输入模式
const actExprDraft = ref({})   // {aid:pkey: 输入文本}

function parseExpr(s) {
  s = (s || '').trim()
  if (!s) return null
  if (s[0] === '{') { try { return JSON.parse(s) } catch (e) { return null } }
  if (s.startsWith('const:')) { const v = Number(s.slice(6)); return Number.isFinite(v) ? v : null }
  if (s.startsWith('param:')) return { param: s.slice(6).trim() }
  if (s.startsWith('neg:')) return { neg: { param: s.slice(4).trim() } }
  if (s.startsWith('mul:')) {
    const [p, k] = s.slice(4).split('*')
    const kv = Number(k); return (p && Number.isFinite(kv)) ? { mul: [{ param: p.trim() }, { const: kv }] } : null
  }
  if (s.startsWith('add:')) {
    const [p, k] = s.slice(4).split('+')
    const kv = Number(k); return (p && Number.isFinite(kv)) ? { add: [{ param: p.trim() }, { const: kv }] } : null
  }
  const n = Number(s); return Number.isFinite(n) ? n : null
}
function exprText(v) {
  if (typeof v === 'number') return String(v)
  if (v && typeof v === 'object') {
    if (v.param) return 'param:' + v.param
    if (v.neg && v.neg.param) return 'neg:' + v.neg.param
    if (v.mul && v.mul[0] && v.mul[0].param) return `mul:${v.mul[0].param}*${v.mul[1] ? v.mul[1].const : 1}`
    if (Array.isArray(v.add)) {
      const p = v.add.find(x => x && x.param), c = v.add.find(x => x && x.const !== undefined)
      if (p) return `add:${p.param}+${c ? c.const : 0}`
    }
    return JSON.stringify(v)
  }
  return String(v)
}
function actVal(aid, pkey, spec) {
  const v = (current.value?.actions?.[aid] || {})[pkey]
  return v == null ? spec.default : v
}
function actNum(aid, pkey, spec) {
  const v = actVal(aid, pkey, spec)
  return typeof v === 'number' ? v : spec.default
}
function actDisplay(aid, pkey, spec) {
  const v = actVal(aid, pkey, spec)
  return typeof v === 'number' ? round(v) : '🔗 表达式'
}
function toggleActExpr(aid, pkey) {
  const k = aid + ':' + pkey
  const on = !actExprMode.value[k]
  actExprMode.value[k] = on
  if (on) {
    const spec = (schema.value.actions[aid]?.params || {})[pkey] || { default: 1.0 }
    actExprDraft.value[k] = exprText(actVal(aid, pkey, spec))
  } else if (typeof actVal(aid, pkey, (schema.value.actions[aid]?.params || {})[pkey] || {}) !== 'number') {
    // 切回数值模式：表达式值还原为默认（slider 可调）
    const spec = (schema.value.actions[aid]?.params || {})[pkey] || { default: 1.0 }
    setAction(aid, pkey, spec.default)
  }
}
function onActExpr(aid, pkey, text) {
  actExprDraft.value[aid + ':' + pkey] = text
  const parsed = parseExpr(text)
  if (parsed !== null) setAction(aid, pkey, parsed)
}

onMounted(async () => {
  await Promise.all([loadPresets(), loadSpecies(), loadSkins()])
  loading.value = false
})

async function loadSkins() {
  try { const r = await api.skins(); skinList.value = r.skins || [] }
  catch (e) { /* 皮肤列表加载失败不阻塞 */ }
}
async function loadPresets() {
  try { const r = await api.presets(); presetList.value = r.presets || [] }
  catch (e) { ElMessage.error('加载预设失败: ' + e.message) }
}
async function loadSpecies() {
  try { const r = await api.species(); speciesList.value = r.species || [] }
  catch (e) { ElMessage.error('加载物种失败: ' + e.message) }
}

async function openPreset(p) {
  creating.value = false
  isNew.value = false
  try {
    current.value = await api.presetDetail(p.preset_id)
    tab.value = 'body'
    previewAction.value = ''
    lastPreviewAction.value = ''
    previewData.value = null
    previewSkinId.value = ''
    skinPreviewData.value = null
    await loadSkins()
  } catch (e) { ElMessage.error(e.message) }
}

function startCreate() { creating.value = true; newSpeciesId.value = '' }

async function initNew() {
  if (!newSpeciesId.value) return
  try {
    current.value = await api.presetNew(newSpeciesId.value)
    isNew.value = true
    creating.value = false
    tab.value = 'body'
    previewAction.value = ''
    lastPreviewAction.value = ''
  } catch (e) { ElMessage.error(e.message) }
}

function close() { current.value = null; isNew.value = false; previewData.value = null; lastPreviewAction.value = '' }

async function save() {
  if (!current.value?.preset_id) { ElMessage.warning('预设 ID 不能为空'); return }
  saving.value = true
  try {
    const data = JSON.parse(JSON.stringify(current.value))
    if (isNew.value) await api.createPreset(data)
    else await api.updatePreset(current.value.preset_id, data)
    ElMessage.success('预设已保存')
    await loadPresets()
  } catch (e) { ElMessage.error('保存失败: ' + e.message) }
  saving.value = false
}

async function confirmDelete(p) {
  try {
    await ElMessageBox.confirm(`确定删除预设「${p.title || p.preset_id}」吗？`, '确认', { type: 'warning' })
    await api.deletePreset(p.preset_id)
    if (current.value?.preset_id === p.preset_id) current.value = null
    await loadPresets()
  } catch (e) { if (e !== 'cancel') ElMessage.error(e.message || e) }
}

function setBody(key, val) {
  current.value.body = { ...(current.value.body || {}), [key]: val }
}
function setAction(aid, pkey, val) {
  current.value.actions = {
    ...(current.value.actions || {}),
    [aid]: { ...((current.value.actions || {})[aid] || {}), [pkey]: val },
  }
}

watch([() => current.value?.body, () => current.value?.actions, previewAction],
      () => scheduleRender(), { deep: true })
watch(cam, () => {
  if (previewData.value && previewViewer.value) {
    previewViewer.value.setView(cam.value.yaw, cam.value.pitch, cam.value.dist, cam.value.panX, cam.value.panY)
  }
  if (skinPreviewData.value && skinViewer.value) {
    skinViewer.value.setView(cam.value.yaw, cam.value.pitch, cam.value.dist, cam.value.panX, cam.value.panY)
  }
}, { deep: true })

watch([previewSkinId, previewAction], () => {
  if (tab.value === 'skinning') scheduleSkinRender()
})

function scheduleRender() {
  if (!current.value) return
  if (renderTimer) clearTimeout(renderTimer)
  renderTimer = setTimeout(renderLive, 500)
}

async function renderLive() {
  const c = current.value
  if (!c || !c.species) return
  rendering.value = true
  try {
    const body = encodeURIComponent(JSON.stringify(c.body || {}))
    if (previewAction.value) {
      // 切换动作直接播新动作帧（不加 transition_from 过渡段，避免循环播放时
      // 动画开头/结尾重复出现旧动作形态）
      const params = encodeURIComponent(JSON.stringify((c.actions || {})[previewAction.value] || {}))
      const r = await api.motion3dData(previewAction.value,
        `species=${encodeURIComponent(c.species)}&body=${body}&params=${params}`)
      if (r.ok && r.frames) previewData.value = r
      else previewData.value = null
      lastPreviewAction.value = previewAction.value
    } else {
      const r = await api.skeleton3dData(c.species, `data=1&body=${body}`)
      if (r.ok && r.joints) previewData.value = r
      else previewData.value = null
    }
  } catch (e) { ElMessage.error(e.message) }
  rendering.value = false
}

function scheduleSkinRender() {
  if (!current.value) return
  if (skinTimer) clearTimeout(skinTimer)
  skinTimer = setTimeout(renderSkinLive, 500)
}

async function renderSkinLive() {
  const c = current.value
  if (!c || !c.preset_id || !previewSkinId.value || !previewAction.value) {
    skinPreviewData.value = null
    return
  }
  skinRendering.value = true
  try {
    const r = await api.skin3dData(previewAction.value,
      `preset=${encodeURIComponent(c.preset_id)}&skin_id=${encodeURIComponent(previewSkinId.value)}`)
    if (r.ok && r.frames) skinPreviewData.value = r
    else skinPreviewData.value = null
  } catch (e) { skinPreviewData.value = null }
  skinRendering.value = false
}

async function exportGlb() {
  const c = current.value
  if (!c || !previewSkinId.value || !previewAction.value) return
  exporting.value = true
  try {
    const url = `/api/skin3d/export/${encodeURIComponent(previewAction.value)}?preset=${encodeURIComponent(c.preset_id)}&skin_id=${encodeURIComponent(previewSkinId.value)}`
    const resp = await fetch(url)
    if (!resp.ok) {
      const j = await resp.json().catch(() => ({}))
      throw new Error(j.error || 'GLB 导出失败')
    }
    const blob = await resp.blob()
    const dl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = dl
    a.download = `${c.preset_id}_${previewAction.value}.glb`
    document.body.appendChild(a)
    a.click()
    a.remove()
    setTimeout(() => URL.revokeObjectURL(dl), 3000)
    ElMessage.success('GLB 已导出（预设+皮肤 → 蒙皮 → 动画）')
  } catch (e) { ElMessage.error(e.message) }
  exporting.value = false
}

onBeforeUnmount(() => {
  if (renderTimer) clearTimeout(renderTimer)
  if (skinTimer) clearTimeout(skinTimer)
})
</script>

<style scoped>
.page { max-width: 1280px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-header h2 { margin: 0 0 4px; }
.page-desc { color: #909399; font-size: .85rem; margin: 0; }
.header-actions { display: flex; gap: 8px; }

.list-view { background: #fff; border: 1px solid #e4e7ed; border-radius: 10px; padding: 8px; }
.detail-view { background: #fff; border: 1px solid #e4e7ed; border-radius: 10px; padding: 16px 20px; }
.detail-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.crumb { font-size: .95rem; }
.crumb-root { color: #909399; } .crumb-sep { color: #c0c4cc; } .crumb-now { font-weight: 600; }
.head-actions { display: flex; gap: 8px; }
.cell-main { display: flex; align-items: center; gap: 8px; }
.cell-title { font-weight: 600; font-size: .9rem; }
.cell-id { color: #909399; font-size: .72rem; font-family: monospace; }
.empty-list { padding: 48px 20px; text-align: center; color: #c0c4cc; display: flex; flex-direction: column; align-items: center; gap: 8px; }
.empty-icon { font-size: 2rem; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr 1fr 1fr; gap: 12px; margin-bottom: 8px; }
.param-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 14px; }
.param-item { border: 1px solid #ebeef5; border-radius: 8px; padding: 10px 12px; background: #fafbfc; }
.param-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.param-head label { font-size: .8rem; color: #606266; }
.val { font-family: monospace; font-size: .8rem; color: #409eff; }
.action-card { border: 1px solid #ebeef5; border-radius: 8px; padding: 10px 14px; margin-bottom: 10px; background: #fafbfc; }
.action-head { display: flex; justify-content: space-between; margin-bottom: 8px; font-weight: 600; font-size: .85rem; }
.no-params { color: #909399; font-size: .78rem; }
.preview-controls { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; flex-wrap: wrap; }
.preview-empty { border: 1px dashed #d9d9d9; border-radius: 8px; min-height: 260px; display: flex; align-items: center; justify-content: center; color: #c0c4cc; }
.hint { color: #909399; font-size: .82rem; margin: 0 0 12px; }
.panel-title { margin: 0 0 8px; }
.create-row { display: flex; align-items: center; gap: 10px; }
</style>
