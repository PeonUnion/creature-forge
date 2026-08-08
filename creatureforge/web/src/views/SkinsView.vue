<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2>🧍 皮肤管理</h2>
        <p class="page-desc">皮肤 = 基于物种的外观实例：肤色 / 体脂 / 肌肉等皮肤参数 + 材质参数（albedo / 粗糙度 / 金属度）。参数面板由物种皮肤定义派生，网格/权重为物种基底（skin/），皮肤只存外观覆盖。</p>
      </div>
      <el-button type="primary" icon="Plus" @click="startCreate">新建皮肤</el-button>
    </div>

    <div class="layout">
      <!-- 左侧：皮肤列表（独立入口，多个皮肤可选） -->
      <aside class="sidebar">
        <div class="sidebar-header">
          <span>皮肤列表</span>
          <el-tag size="small" type="info" effect="plain">{{ skinList.length }}</el-tag>
        </div>
        <div v-if="loading" class="panel-loading"><el-skeleton :rows="5" animated /></div>
        <div v-else class="sidebar-list">
          <div v-for="s in skinList" :key="s.skin_id" class="list-item"
               :class="{ active: current?.skin_id === s.skin_id }" @click="openSkin(s)">
            <div class="item-main">
              <span class="item-name">🧍 {{ s.title || s.skin_id }}</span>
              <span class="item-id">{{ s.skin_id }}</span>
            </div>
            <div class="item-meta">
              <span class="meta-chip">🦴 {{ s.species }}</span>
              <span class="meta-chip">{{ s.description }}</span>
            </div>
            <div class="item-actions">
              <el-button size="small" text type="danger" @click.stop="confirmDelete(s)">删除</el-button>
            </div>
          </div>
          <div v-if="!skinList.length" class="empty-list"><p>暂无皮肤，点击「新建皮肤」</p></div>
        </div>
      </aside>

      <!-- 右侧 -->
      <section class="content">
        <!-- 新建：先选物种（皮肤 schema 来源） -->
        <div v-if="creating" class="panel">
          <h4 class="panel-title">新建皮肤 — 选择物种</h4>
          <p class="hint">物种提供皮肤参数 schema（肤色/体脂/肌肉）与材质基底（skin/materials）。</p>
          <div class="create-row">
            <el-select v-model="newSpeciesId" placeholder="选择物种" style="width: 300px" filterable>
              <el-option v-for="s in speciesList" :key="s.id"
                         :label="`${s.title} (${s.id})`" :value="s.id" />
            </el-select>
            <el-button type="primary" :disabled="!newSpeciesId" @click="initNew" icon="Right">初始化皮肤</el-button>
            <el-button @click="creating = false">取消</el-button>
          </div>
        </div>

        <!-- 编辑 -->
        <div v-else-if="current" class="panel">
          <div class="content-header">
            <div class="crumb"><span class="crumb-root">皮肤</span><span class="crumb-sep">/</span><span class="crumb-now">{{ current.title || current.skin_id }}</span></div>
            <div class="content-actions">
              <el-button @click="close">关闭</el-button>
              <el-button type="primary" @click="save" :loading="saving" icon="Check">保存皮肤</el-button>
            </div>
          </div>

          <el-form label-position="top" class="form-grid">
            <el-form-item label="皮肤 ID"><el-input v-model="current.skin_id" :disabled="!isNew" placeholder="如 sk_warrior" /></el-form-item>
            <el-form-item label="名称"><el-input v-model="current.title" placeholder="如 战士皮肤" /></el-form-item>
            <el-form-item label="描述"><el-input v-model="current.description" /></el-form-item>
            <el-form-item label="物种（schema 来源）"><el-tag effect="plain">🦴 {{ current.species }}</el-tag></el-form-item>
          </el-form>

          <el-tabs v-model="tab">
            <!-- 皮肤参数：肤色/体脂/肌肉 -->
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

            <!-- 材质：albedo 颜色 / 粗糙度 / 金属度 -->
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

            <!-- 预览：蒙皮（应用皮肤材质 + 胖瘦） -->
            <el-tab-pane label="👁 预览" name="preview">
              <div class="preview-controls">
                <CameraControls v-model="cam" />
                <el-select v-model="previewAction" placeholder="选择动作" clearable filterable style="width: 170px">
                  <el-option v-for="a in actions" :key="a" :label="actionTitle(a)" :value="a" />
                </el-select>
                <span class="hint">胖瘦因子 {{ round(bodyScale) }}</span>
              </div>
              <SkinnedViewer v-if="previewData" ref="previewViewer"
                :mesh="previewData.mesh" :frames="previewData.frames" :fps="previewData.fps"
                :center="previewData.center" :material="current.materials" :body-scale="bodyScale"
                @view="cam = { ...cam, yaw: $event.yaw, pitch: $event.pitch }" />
              <div v-else class="preview-empty"><p>{{ rendering ? '渲染中…' : '选择动作加载蒙皮预览（应用当前皮肤参数）' }}</p></div>
            </el-tab-pane>
          </el-tabs>
        </div>

        <div v-else class="panel empty-state">
          <div class="empty-icon">🧍</div>
          <h3>选择或创建皮肤</h3>
          <p>皮肤是基于物种的外观实例：肤色 / 体脂 / 肌肉 + 材质，可多个并存。</p>
          <el-button type="primary" @click="startCreate">新建皮肤</el-button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { api } from '../api.js'
import { ElMessage, ElMessageBox } from 'element-plus'
import CameraControls from '../components/CameraControls.vue'
import SkinnedViewer from '../components/SkinnedViewer.vue'

const actions = ['walk3d', 'run3d', 'jump3d', 'crawl3d', 'idle3d']
const actionTitle = (id) => ({ walk3d: 'Walk', run3d: 'Run', jump3d: 'Jump', crawl3d: 'Crawl', idle3d: 'Idle' }[id] || id)
const predefColors = ['#c9a58c', '#8a5a3a', '#5d8a3a', '#6b8a5a', '#d8b8a0', '#b06a4a', '#3a6b8a', '#c9c9c9']

const loading = ref(true)
const saving = ref(false)
const skinList = ref([])
const speciesList = ref([])
const current = ref(null)
const isNew = ref(false)
const creating = ref(false)
const newSpeciesId = ref('')
const tab = ref('params')
const cam = ref({ yaw: 30, pitch: 12, dist: 1, panX: 0, panY: 0 })
const previewAction = ref('')
const previewData = ref(null)
const previewViewer = ref(null)
const rendering = ref(false)
let renderTimer = null

const schema = computed(() => current.value?.schema_info || { params: {}, materials: {} })
const paramItems = computed(() => {
  const p = schema.value.params || {}
  return Object.entries(p).map(([key, spec]) => ({
    key, label: spec.label || key, min: spec.min ?? 0, max: spec.max ?? 1,
    step: spec.step || 0.01, def: spec.default ?? 0, desc: spec.desc || '',
  }))
})
// 胖瘦因子：体脂/肌肉 → 网格 x/z 缩放（增宽/收窄，保持身高）
const bodyScale = computed(() => {
  const p = current.value?.params || {}
  const fat = p.fat ?? 0.3
  const muscle = p.muscle ?? 0.5
  return Math.max(0.6, Math.min(1.6, 1 + (fat - 0.3) * 0.35 + (muscle - 0.5) * 0.12))
})
const round = (v) => (typeof v === 'number' ? Math.round(v * 100) / 100 : v)

onMounted(async () => {
  await Promise.all([loadSkins(), loadSpecies()])
  loading.value = false
})

async function loadSkins() {
  try { const r = await api.skins(); skinList.value = r.skins || [] }
  catch (e) { ElMessage.error('加载皮肤失败: ' + e.message) }
}
async function loadSpecies() {
  try { const r = await api.species(); speciesList.value = r.species || [] }
  catch (e) { ElMessage.error('加载物种失败: ' + e.message) }
}

async function openSkin(s) {
  creating.value = false
  isNew.value = false
  try {
    current.value = await api.skinDetail(s.skin_id)
    tab.value = 'params'
    previewAction.value = ''
    previewData.value = null
  } catch (e) { ElMessage.error(e.message) }
}

function startCreate() { creating.value = true; newSpeciesId.value = '' }

async function initNew() {
  if (!newSpeciesId.value) return
  try {
    current.value = await api.skinNew(newSpeciesId.value)
    isNew.value = true
    creating.value = false
    tab.value = 'params'
    previewAction.value = ''
  } catch (e) { ElMessage.error(e.message) }
}

function close() { current.value = null; isNew.value = false; previewData.value = null }

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

// 动作切换 → 重新加载蒙皮顶点；皮肤参数/材质变化 → SkinnedViewer 响应式更新（材质/胖瘦）
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
      const r = await api.skin3dData(previewAction.value, `species=${encodeURIComponent(c.species)}`)
      if (r.ok && r.frames) previewData.value = r
      else previewData.value = null
    } else {
      previewData.value = null
    }
  } catch (e) { previewData.value = null }
  rendering.value = false
}
</script>

<style scoped>
.page { max-width: 1280px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-header h2 { margin: 0 0 4px; }
.page-desc { color: #909399; font-size: .85rem; margin: 0; }
.layout { display: grid; grid-template-columns: 260px 1fr; gap: 16px; align-items: start; }

.sidebar { background: #fff; border-radius: 10px; border: 1px solid #e4e7ed; overflow: hidden; position: sticky; top: 76px; }
.sidebar-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 14px; border-bottom: 1px solid #f0f0f0; font-weight: 600; }
.panel-loading { padding: 14px; }
.sidebar-list { max-height: calc(100vh - 180px); overflow-y: auto; }
.list-item { padding: 10px 14px; border-bottom: 1px solid #f5f5f5; cursor: pointer; }
.list-item:hover { background: #f7f9fc; }
.list-item.active { background: #ecf5ff; }
.item-main { display: flex; align-items: center; gap: 6px; }
.item-name { font-weight: 600; font-size: .9rem; }
.item-id { font-size: .72rem; color: #909399; }
.item-meta { display: flex; gap: 6px; margin-top: 4px; flex-wrap: wrap; }
.meta-chip { font-size: .7rem; color: #909399; background: #f5f7fa; padding: 1px 8px; border-radius: 999px; }
.item-actions { margin-top: 6px; }
.empty-list { padding: 30px; text-align: center; color: #c0c4cc; }

.content { background: #fff; border-radius: 10px; border: 1px solid #e4e7ed; padding: 16px 20px; min-height: 480px; }
.panel { }
.panel-title { margin: 0 0 8px; }
.content-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
.crumb { color: #909399; font-size: .85rem; }
.crumb-root, .crumb-sep { color: #c0c4cc; }
.crumb-now { color: #303133; font-weight: 600; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0 20px; }
.hint { color: #909399; font-size: .8rem; margin: 0 0 12px; }
.create-row { display: flex; gap: 8px; align-items: center; margin-top: 8px; }

.param-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px 24px; }
.param-item { padding: 8px 10px; border: 1px solid #f0f0f0; border-radius: 8px; }
.param-head { display: flex; justify-content: space-between; align-items: center; }
.param-head label { font-size: .85rem; color: #606266; }
.val { font-family: monospace; color: #909399; font-size: .75rem; }
.param-desc { font-size: .72rem; color: #c0c4cc; margin-top: 4px; }

.preview-controls { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-bottom: 12px; }
.preview-empty { text-align: center; color: #c0c4cc; padding: 40px; }

.empty-state { text-align: center; padding: 60px 20px; }
.empty-state .empty-icon { font-size: 3rem; }
.empty-state h3 { margin: 8px 0 6px; }
.empty-state p { color: #909399; margin-bottom: 14px; }
</style>
