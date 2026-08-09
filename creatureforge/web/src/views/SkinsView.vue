<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2>🧍 皮肤管理</h2>
        <p class="page-desc">皮肤 = 基于预设的外观实例：肤色 / 体脂 / 肌肉 + 材质 + 部件。表格 + 详情页维护。</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="startCreate" icon="Plus">新建皮肤</el-button>
      </div>
    </div>

    <!-- 列表表格 -->
    <section class="list-view" v-if="!current && !creating">
      <el-table :data="skinList" border stripe>
        <el-table-column label="皮肤" min-width="200">
          <template #default="{row}">
            <div class="cell-main"><span class="cell-title">🧍 {{ row.title || row.skin_id }}</span><span class="cell-id mono">{{ row.skin_id }}</span></div>
          </template>
        </el-table-column>
        <el-table-column label="预设" width="150">
          <template #default="{row}"><el-tag size="small" effect="plain">🎨 {{ row.preset }}</el-tag></template>
        </el-table-column>
        <el-table-column label="物种" width="120">
          <template #default="{row}"><el-tag size="small" effect="plain" type="info">🦴 {{ row.species }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="操作" width="180" align="center">
          <template #default="{row}">
            <el-button size="small" text type="primary" @click="openSkin(row)">编辑</el-button>
            <el-button size="small" text type="danger" @click="confirmDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!skinList.length" class="empty-list">
        <div class="empty-icon">🧍</div><p>暂无皮肤</p>
        <el-button type="primary" @click="startCreate">新建皮肤</el-button>
      </div>
    </section>

    <!-- 详情：参数 / 材质 / 部件 / 预览 -->
    <section class="detail-view" v-else-if="current">
      <div class="detail-head">
        <div class="crumb">
          <span class="crumb-root">皮肤</span><span class="crumb-sep">/</span><span class="crumb-now">{{ current.title || current.skin_id }}</span>
        </div>
        <div class="head-actions">
          <el-button size="small" @click="close">返回列表</el-button>
          <el-button size="small" type="primary" @click="save" :loading="saving" icon="Check">保存皮肤</el-button>
        </div>
      </div>

      <el-form label-position="top" class="form-grid">
        <el-form-item label="皮肤 ID"><el-input v-model="current.skin_id" :disabled="!isNew" placeholder="如 sk_warrior" /></el-form-item>
        <el-form-item label="名称"><el-input v-model="current.title" placeholder="如 战士皮肤" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="current.description" /></el-form-item>
        <el-form-item label="基于预设"><el-tag effect="plain">🎨 {{ current.preset }}</el-tag></el-form-item>
      </el-form>

      <el-tabs v-model="tab">
        <el-tab-pane label="🎚 皮肤参数" name="params">
          <p class="hint">调整皮肤外观（来自物种 skin_params.json 派生 schema，default 为物种默认）。</p>
          <div v-if="paramItems.length" class="param-grid">
            <div v-for="it in paramItems" :key="it.key" class="param-item">
              <div class="param-head">
                <label :title="it.key">{{ it.label }}</label>
                <span class="val">{{ round(current.params[it.key] ?? it.def) }}</span>
              </div>
              <el-slider :min="it.min" :max="it.max" :step="it.step" :show-tooltip="false"
                         :model-value="current.params[it.key] ?? it.def" @update:model-value="setParam(it.key, $event)" />
              <div v-if="it.desc" class="param-desc">{{ it.desc }}</div>
            </div>
          </div>
          <div v-else class="preview-empty"><p>该物种没有皮肤参数</p></div>
        </el-tab-pane>

        <el-tab-pane label="🎨 材质" name="materials">
          <p class="hint">材质参数（albedo 基础色 / roughness 粗糙度 / metallic 金属度），应用到蒙皮网格。</p>
          <div class="param-grid">
            <div class="param-item">
              <div class="param-head"><label>基础色（albedo）</label></div>
              <el-color-picker v-model="current.materials.albedo" show-alpha :predefine="predefColors" />
            </div>
            <div class="param-item">
              <div class="param-head"><label>粗糙度 roughness</label><span class="val">{{ round(current.materials.roughness) }}</span></div>
              <el-slider :min="0" :max="1" :step="0.01" :model-value="current.materials.roughness"
                         @update:model-value="current.materials.roughness = $event" />
            </div>
            <div class="param-item">
              <div class="param-head"><label>金属度 metallic</label><span class="val">{{ round(current.materials.metallic) }}</span></div>
              <el-slider :min="0" :max="1" :step="0.01" :model-value="current.materials.metallic"
                         @update:model-value="current.materials.metallic = $event" />
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="🧩 部件" name="parts">
          <div class="parts-head">
            <p class="hint">皮肤部件 = 画师导出部件（网格 + 贴图 + 材质）附着到骨架。上传 glTF/GLB/OBJ/JSON 网格 + 图片贴图，选绑定骨骼并调整偏移。</p>
            <el-button size="small" type="primary" icon="Plus" @click="startPart">添加部件</el-button>
          </div>
          <div v-if="!(current.parts && current.parts.length)" class="preview-empty">
            <p>暂无部件。点击「添加部件」，上传画师部件文件并附着到骨架。</p>
          </div>
          <div v-else class="parts-list">
            <div v-for="pt in current.parts" :key="pt.part_id" class="part-card">
              <div class="part-head">
                <span class="part-name">🧩 {{ pt.title || pt.part_id }}</span>
                <span class="part-id">{{ pt.part_id }}</span>
                <el-tag size="small" effect="plain">{{ pt.kind === 'skinned' ? '蒙皮' : '装饰' }}</el-tag>
                <el-button size="small" text type="danger" @click="deletePart(pt)">删除</el-button>
              </div>
              <div class="part-row">
                <label>网格文件</label>
                <input type="file" accept=".glb,.gltf,.obj,.json" @change="uploadMesh(pt, $event)" />
                <span v-if="pt.mesh_file" class="ok">✓ {{ pt.mesh_file }}</span>
                <span v-else-if="pt.mesh" class="ok">内嵌 {{ pt.mesh.vertex_count }} 顶点</span>
                <span v-else class="missing">未上传</span>
              </div>
              <div class="part-row">
                <label>贴图 albedo</label>
                <input type="file" accept="image/*" @change="uploadTex(pt, 'albedo', $event)" />
                <span v-if="pt.textures && pt.textures.albedo" class="ok">✓ 已上传</span>
              </div>
              <div class="part-row">
                <label>绑定骨骼</label>
                <el-select v-model="pt.bone" size="small" filterable style="width: 180px" @change="schedulePartPatch">
                  <el-option v-for="b in boneList" :key="b" :label="b" :value="b" />
                </el-select>
              </div>
              <div class="part-row">
                <label>位置 x/y/z</label>
                <el-input-number v-model="pt.transform.position[0]" size="small" :step="2" @change="schedulePartPatch" />
                <el-input-number v-model="pt.transform.position[1]" size="small" :step="2" @change="schedulePartPatch" />
                <el-input-number v-model="pt.transform.position[2]" size="small" :step="2" @change="schedulePartPatch" />
              </div>
              <div class="part-row">
                <label>旋转 r/p/y</label>
                <el-input-number v-model="pt.transform.rotation[0]" size="small" :step="5" @change="schedulePartPatch" />
                <el-input-number v-model="pt.transform.rotation[1]" size="small" :step="5" @change="schedulePartPatch" />
                <el-input-number v-model="pt.transform.rotation[2]" size="small" :step="5" @change="schedulePartPatch" />
              </div>
              <div class="part-row">
                <label>缩放 x/y/z</label>
                <el-input-number v-model="pt.transform.scale[0]" size="small" :step="0.1" @change="schedulePartPatch" />
                <el-input-number v-model="pt.transform.scale[1]" size="small" :step="0.1" @change="schedulePartPatch" />
                <el-input-number v-model="pt.transform.scale[2]" size="small" :step="0.1" @change="schedulePartPatch" />
              </div>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="👁 预览" name="preview">
          <div class="preview-controls">
            <CameraControls v-model="cam" />
            <el-select v-model="previewAction" placeholder="选择动作" clearable filterable style="width: 170px">
              <el-option v-for="(a, aid) in previewActions" :key="aid" :label="a.title || aid" :value="aid" />
            </el-select>
            <span class="hint">胖瘦因子 {{ round(bodyScale) }}</span>
          </div>
          <SkinnedViewer v-if="previewData" ref="previewViewer"
            :mesh="previewData.mesh" :frames="previewData.frames" :fps="previewData.fps"
            :center="previewData.center" :material="current.materials"
            :bind-joints="previewData.bindJoints" :parts="previewData.parts"
            :part-bone-frames="previewData.part_bone_frames"
            :part-skin-frames="previewData.part_skin_frames"
            @view="cam = { ...cam, yaw: $event.yaw, pitch: $event.pitch }" />
          <div v-else class="preview-empty"><p>{{ rendering ? '渲染中…' : '选择动作加载蒙皮预览（应用当前皮肤参数）' }}</p></div>
        </el-tab-pane>
      </el-tabs>
    </section>

    <!-- 新建：选预设 -->
    <section class="detail-view" v-else-if="creating">
      <h4 class="panel-title">新建皮肤 — 选择预设</h4>
      <p class="hint">皮肤基于预设：预设提供物种（网格/权重基底 + 皮肤参数 schema）与动作参数；在此之上调整皮肤外观。</p>
      <div class="create-row">
        <el-select v-model="newPresetId" placeholder="选择预设" style="width: 300px" filterable>
          <el-option v-for="p in presetList" :key="p.preset_id"
                     :label="`${p.title} (${p.preset_id}) — ${p.species}`" :value="p.preset_id" />
        </el-select>
        <el-button type="primary" :disabled="!newPresetId" @click="initNew" icon="Right">初始化皮肤</el-button>
        <el-button @click="creating = false">取消</el-button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { api } from '../api.js'
import { ElMessage, ElMessageBox } from 'element-plus'
import CameraControls from '../components/CameraControls.vue'
import SkinnedViewer from '../components/SkinnedViewer.vue'

const predefColors = ['#c9a58c', '#8a5a3a', '#5d8a3a', '#6b8a5a', '#d8b8a0', '#b06a4a', '#3a6b8a', '#c9c9c9']

const loading = ref(true)
const saving = ref(false)
const skinList = ref([])
const presetList = ref([])
const current = ref(null)
const isNew = ref(false)
const creating = ref(false)
const newPresetId = ref('')
const tab = ref('params')
const cam = ref({ yaw: 30, pitch: 12, dist: 1, panX: 0, panY: 0 })
const previewAction = ref('')
const previewData = ref(null)
const previewViewer = ref(null)
const rendering = ref(false)
let renderTimer = null
const boneList = ref([])
let partTimer = null

const schema = computed(() => current.value?.schema_info || { params: {}, materials: {} })
const paramItems = computed(() => {
  const p = schema.value.params || {}
  return Object.entries(p).map(([key, spec]) => ({
    key, label: spec.label || key, min: spec.min ?? 0, max: spec.max ?? 1,
    step: spec.step || 0.01, def: spec.default ?? 0, desc: spec.desc || '',
  }))
})
const bodyScale = computed(() => {
  const bs = schema.value.body_scale
  if (!bs) return null
  const p = current.value?.params || {}
  let s = bs.base ?? 1
  for (const [k, cfg] of Object.entries(bs.params || {})) {
    const v = p[k] ?? cfg.offset ?? 0
    s += (cfg.coef ?? 0) * (v - (cfg.offset ?? 0))
  }
  return Math.max(bs.min ?? 0.6, Math.min(bs.max ?? 1.6, s))
})
const round = (v) => (typeof v === 'number' ? Math.round(v * 100) / 100 : v)

onMounted(async () => {
  await Promise.all([loadSkins(), loadPresets()])
  loading.value = false
})

async function loadSkins() {
  try { const r = await api.skins(); skinList.value = r.skins || [] }
  catch (e) { ElMessage.error('加载皮肤失败: ' + e.message) }
}
async function loadPresets() {
  try { const r = await api.presets(); presetList.value = r.presets || [] }
  catch (e) { ElMessage.error('加载预设失败: ' + e.message) }
}

// -- 预览动作：来自皮肤所基于的预设（预设已选动作优先，否则物种全部动作）--
const presetActions = ref({})   // 预设已选动作 {aid: {title, params}}
const speciesActions = ref({})  // 物种全部动作 {aid: {title, params}}
const previewActions = computed(() => {
  const picked = Object.keys(presetActions.value)
  if (picked.length) return presetActions.value
  return speciesActions.value
})
async function loadPresetActions(presetId) {
  presetActions.value = {}
  speciesActions.value = {}
  if (!presetId) return
  try {
    const d = await api.presetDetail(presetId)
    const all = d.schema_info?.actions || {}
    speciesActions.value = all
    const picked = {}
    for (const aid of Object.keys(d.actions || {})) {
      picked[aid] = all[aid] || { title: aid, params: {} }
    }
    presetActions.value = picked
  } catch (e) { /* 预设详情不可用 → 保持空，预览回退骨架 */ }
}

async function openSkin(s) {
  creating.value = false
  isNew.value = false
  try {
    current.value = await api.skinDetail(s.skin_id)
    tab.value = 'params'
    previewAction.value = ''
    previewData.value = null
    await loadPresetActions(current.value?.preset)
    await loadBones()
  } catch (e) { ElMessage.error(e.message) }
}

function startCreate() { creating.value = true; newPresetId.value = '' }

async function initNew() {
  if (!newPresetId.value) return
  try {
    current.value = await api.skinNew(newPresetId.value)
    isNew.value = true
    creating.value = false
    tab.value = 'params'
    previewAction.value = ''
    current.value.parts = current.value.parts || []
    await loadPresetActions(current.value?.preset)
    await loadBones()
  } catch (e) { ElMessage.error(e.message) }
}

function close() { current.value = null; isNew.value = false; previewData.value = null; presetActions.value = {}; speciesActions.value = {} }

async function save() {
  if (!current.value?.skin_id) { ElMessage.warning('皮肤 ID 不能为空'); return }
  saving.value = true
  try {
    const data = JSON.parse(JSON.stringify(current.value))
    if (isNew.value) await api.createSkin(data)
    else await api.updateSkin(current.value.skin_id, data)
    ElMessage.success('皮肤已保存')
    await loadSkins()
  } catch (e) { ElMessage.error('保存失败: ' + e.message) }
  saving.value = false
}

async function confirmDelete(s) {
  try {
    await ElMessageBox.confirm(`确定删除皮肤「${s.title || s.skin_id}」吗？`, '确认', { type: 'warning' })
    await api.deleteSkin(s.skin_id)
    if (current.value?.skin_id === s.skin_id) current.value = null
    await loadSkins()
  } catch (e) { if (e !== 'cancel') ElMessage.error(e.message || e) }
}

function setParam(key, val) {
  current.value.params = { ...(current.value.params || {}), [key]: val }
}

watch(previewAction, () => scheduleRender())
watch(() => current.value?.params, () => { /* SkinnedViewer 响应 bodyScale */ }, { deep: true })
watch(cam, () => {
  if (previewData.value && previewViewer.value) {
    previewViewer.value.setView(cam.value.yaw, cam.value.pitch, cam.value.dist, cam.value.panX, cam.value.panY)
  }
}, { deep: true })

function scheduleRender() {
  if (!current.value) return
  if (renderTimer) clearTimeout(renderTimer)
  renderTimer = setTimeout(renderLive, 400)
}

async function renderLive() {
  const c = current.value
  if (!c) return
  rendering.value = true
  try {
    if (previewAction.value) {
      const r = await api.skin3dData(previewAction.value,
        `preset=${encodeURIComponent(c.preset)}&skin_id=${encodeURIComponent(c.skin_id)}`)
      if (r.ok && r.frames) previewData.value = r
      else previewData.value = null
    } else {
      previewData.value = null
    }
  } catch (e) { previewData.value = null }
  rendering.value = false
}

// -- 部件 --

async function loadBones() {
  if (!current.value?.species) { boneList.value = []; return }
  try {
    const r = await api.skeleton3dData(current.value.species)
    boneList.value = Object.keys(r.joints || {})
  } catch (e) { boneList.value = [] }
}

function fileToB64(file) {
  return new Promise((res, rej) => {
    const r = new FileReader()
    r.onload = () => res(String(r.result).split(',')[1])
    r.onerror = rej
    r.readAsDataURL(file)
  })
}

async function refreshCurrent() {
  if (!current.value) return
  current.value = await api.skinDetail(current.value.skin_id)
}

async function startPart() {
  if (!current.value) return
  const n = (current.value.parts || []).length + 1
  try {
    await api.skinPartAdd(current.value.skin_id, {
      part_id: `p_${Date.now().toString(36)}`,
      title: `部件${n}`,
      kind: 'bone',
      bone: boneList.value[0] || '',
      transform: { position: [0, 0, 0], rotation: [0, 0, 0], scale: [1, 1, 1] },
      mesh: null, mesh_file: null, textures: {}, materials: {}, weights: null,
    })
    await refreshCurrent()
    ElMessage.success('部件已添加，上传网格文件并选择绑定骨骼')
  } catch (e) { ElMessage.error(e.message) }
}

async function uploadMesh(pt, e) {
  const f = e.target.files && e.target.files[0]
  e.target.value = ''
  if (!f || !current.value) return
  try {
    const b64 = await fileToB64(f)
    const r = await api.skinPartUploadMesh(current.value.skin_id, pt.part_id, f.name, b64)
    ElMessage.success(`网格已上传（${f.name}）`)
    await refreshCurrent()
    if (previewAction.value) scheduleRender()
  } catch (e) { ElMessage.error(e.message) }
}

async function uploadTex(pt, field, e) {
  const f = e.target.files && e.target.files[0]
  e.target.value = ''
  if (!f || !current.value) return
  try {
    const b64 = await fileToB64(f)
    await api.skinPartUploadTexture(current.value.skin_id, pt.part_id, field, f.name, b64)
    ElMessage.success(`贴图已上传（${field}）`)
    await refreshCurrent()
    if (previewAction.value) scheduleRender()
  } catch (e) { ElMessage.error(e.message) }
}

async function deletePart(pt) {
  if (!current.value) return
  try {
    await api.skinPartDelete(current.value.skin_id, pt.part_id)
    await refreshCurrent()
    if (previewAction.value) scheduleRender()
  } catch (e) { ElMessage.error(e.message) }
}

function schedulePartPatch() {
  if (!current.value || partTimer) return
  partTimer = setTimeout(() => { partTimer = null }, 0)
}

watch(() => current.value?.parts, () => {
  if (!current.value || !current.value.skin_id) return
  if (partTimer) clearTimeout(partTimer)
  partTimer = setTimeout(async () => {
    partTimer = null
    try {
      await api.updateSkin(current.value.skin_id, current.value)
      if (previewAction.value) scheduleRender()
    } catch (e) { /* 静默：保存皮肤时兜底 */ }
  }, 600)
}, { deep: true })
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
.param-desc { font-size: .72rem; color: #909399; margin-top: 4px; }
.preview-controls { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; flex-wrap: wrap; }
.preview-empty { border: 1px dashed #d9d9d9; border-radius: 8px; min-height: 260px; display: flex; align-items: center; justify-content: center; color: #c0c4cc; }
.hint { color: #909399; font-size: .82rem; margin: 0 0 12px; }
.panel-title { margin: 0 0 8px; }
.create-row { display: flex; align-items: center; gap: 10px; }
.parts-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; margin-bottom: 10px; }
.parts-list { display: flex; flex-direction: column; gap: 10px; }
.part-card { border: 1px solid #ebeef5; border-radius: 8px; padding: 10px 12px; background: #fafbfc; }
.part-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.part-name { font-weight: 600; font-size: .85rem; }
.part-id { color: #909399; font-size: .72rem; font-family: monospace; }
.part-row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; font-size: .8rem; }
.part-row label { width: 76px; color: #606266; flex-shrink: 0; }
.part-row .ok { color: #67c23a; font-size: .75rem; }
.part-row .missing { color: #f56c6c; font-size: .75rem; }
</style>
