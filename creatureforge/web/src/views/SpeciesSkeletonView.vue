<template>
  <div class="skel-view">
    <div class="skel-head">
      <div class="crumb">
        <span class="crumb-root">物种</span><span class="crumb-sep">/</span>
        <span class="crumb-now">{{ wiz?.title || speciesId }}</span>
      </div>
      <div class="head-actions">
        <el-button size="small" @click="emit('back')">返回列表</el-button>
        <el-button size="small" type="primary" @click="save" :loading="saving" icon="Check">保存骨骼</el-button>
      </div>
    </div>

    <!-- 普通 / 高级 双页签，共享同一份草稿 draft -->
    <el-tabs v-model="mode" class="mode-tabs">
      <el-tab-pane label="🧩 普通（语义化）" name="normal" />
      <el-tab-pane label="⚙️ 高级 JSON" name="advanced" />
    </el-tabs>

    <!-- 普通模式：骨架 / 姿态 / 参数 -->
    <div v-if="mode === 'normal'" class="normal-body">
      <el-tabs v-model="sub">
        <el-tab-pane label="🦴 骨架结构" name="skeleton">
          <p class="hint">左侧 3D：左键拖关节=移动该关节、拖空白=移动整体，右键=旋转视角，滚轮=缩放；右侧编辑结构（树/父级/改名/镜像）+ 变换（位置 / 旋转角度 / 平移）。</p>
          <div class="wiz-layout">
            <div class="wiz-preview">
              <Skeleton3DViewer v-if="preview" :joints="preview.joints" :bones="preview.bones"
                :head-radius="12.5" :center="[480, 300, 0]"
                :highlight="highlightJoint" :editable="true" @pick="onPick" @dragend="onDragEnd" />
              <div v-else class="preview-empty"><p>添加关节后实时显示骨架</p></div>
            </div>
            <div class="wiz-controls">
              <div class="sec sec-xform">
                <div class="sec-t xform-head">
                  <span>变换</span>
                  <span v-if="selJoint" class="xform-sel">{{ selJoint }}</span>
                  <el-button v-if="selJoint" size="small" text type="primary" @click="clearSel">清除</el-button>
                </div>
                <p v-if="!selJoint" class="hint small">点击 3D 关节球或关节树选中；未选中时旋转/平移作用于整体。</p>
                <template v-else>
                  <div class="xform-block">
                    <div class="xform-t">位置 XYZ（直接设坐标）</div>
                    <div class="row3">
                      <el-input-number v-model="xf.pos.x" size="small" :step="5" />
                      <el-input-number v-model="xf.pos.y" size="small" :step="5" />
                      <el-input-number v-model="xf.pos.z" size="small" :step="5" />
                    </div>
                    <el-button size="small" type="primary" @click="applyPos">设坐标</el-button>
                  </div>
                  <div class="xform-block">
                    <div class="xform-t">旋转（绕该关节，带动子树改朝向）</div>
                    <div class="row3">
                      <el-select v-model="xf.axis" size="small" style="width: 64px">
                        <el-option v-for="a in ['x','y','z']" :key="a" :label="a.toUpperCase()" :value="a" />
                      </el-select>
                      <el-input-number v-model="xf.angle" size="small" :step="5" :min="-180" :max="180" />
                      <el-button size="small" type="primary" @click="applyRotate">旋转</el-button>
                    </div>
                    <div class="row3 wrap">
                      <el-button size="small" @click="rotateAxis(xf.axis, -15)">-15°</el-button>
                      <el-button size="small" @click="rotateAxis(xf.axis, 15)">+15°</el-button>
                      <el-button size="small" @click="rotateAxis(xf.axis, -90)">-90°</el-button>
                      <el-button size="small" @click="rotateAxis(xf.axis, 90)">+90°</el-button>
                    </div>
                  </div>
                  <div class="xform-block">
                    <div class="xform-t">平移（该关节及其子树）</div>
                    <div class="row3">
                      <el-input-number v-model="xf.dx" size="small" :step="5" />
                      <el-input-number v-model="xf.dy" size="small" :step="5" />
                      <el-input-number v-model="xf.dz" size="small" :step="5" />
                      <el-button size="small" type="primary" @click="applyTranslate">平移</el-button>
                    </div>
                    <p class="hint small">也可在 3D 预览直接按住该关节拖拽移动。</p>
                  </div>
                </template>
              </div>
              <div class="sec">
                <div class="sec-t">新增关节</div>
                <el-input v-model="nj.name" size="small" placeholder="关节名，如 head / wing_l" />
                <el-select v-model="nj.parent" size="small" placeholder="父关节（空 = 根）" clearable filterable style="width: 100%">
                  <el-option v-for="n in jointNames" :key="n" :label="n" :value="n" />
                </el-select>
                <el-input v-model="nj.posStr" size="small" placeholder="坐标 x,y,z（可留空 = 0,0,0）" />
                <el-button size="small" type="primary" @click="addJoint" icon="Plus">加关节</el-button>
              </div>
              <div class="sec">
                <div class="sec-t">关节树（{{ jointNames.length }}，缩进 = 子级，父级可下拉重接）</div>
                <p class="hint small">悬停/点击行 → 3D 高亮该关节及子树；在 3D 预览点关节球可反向定位。</p>
                <div class="joint-list">
                  <div v-for="jt in jointTree" :key="jt.name" class="joint-row"
                    :ref="el => rowEls[jt.name] = el"
                    :class="{ selected: selJoint === jt.name }"
                    :style="{ paddingLeft: (jt.depth ? 10 + jt.depth * 16 : 8) + 'px' }"
                    @mouseenter="hoverJoint = jt.name" @mouseleave="hoverJoint = selJoint"
                    @click="toggleSel(jt.name)">
                    <span class="tree-mark" :class="{ root: !jt.parent }">{{ jt.parent ? '└' : '●' }}</span>
                    <span class="mono">{{ jt.name }}</span>
                    <span class="parent-sel" @click.stop>
                      <el-select :model-value="jt.parent" size="small" placeholder="根"
                        clearable filterable @change="(v) => changeParent(jt.name, v)">
                        <el-option v-for="n in jointNames" :key="n" :label="n" :value="n" :disabled="n === jt.name" />
                      </el-select>
                    </span>
                    <span class="row-actions" @click.stop>
                      <el-button size="small" text type="primary" @click="renameJoint(jt.name)">改名</el-button>
                      <el-button size="small" text type="primary" @click="mirrorJoint(jt.name)">镜像</el-button>
                      <el-button size="small" text type="danger" @click="rmJoint(jt.name)">删</el-button>
                    </span>
                  </div>
                </div>
              </div>
              <div class="sec">
                <div class="sec-t">命名链（spine / tail / arm…）</div>
                <el-input v-model="chainName" size="small" placeholder="链名，如 spine" />
                <el-input v-model="chainJoints" size="small" placeholder="关节（逗号分隔），如 head,neck,chest" />
                <el-button size="small" type="primary" @click="addChain" icon="Link">建链</el-button>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="🧍 默认姿态" name="pose">
          <div class="wiz-layout">
            <div class="wiz-preview">
              <Skeleton3DViewer v-if="preview" :joints="preview.joints" :bones="preview.bones"
                :head-radius="12.5" :center="[480, 300, 0]"
                :highlight="highlightJoint" :editable="true" @pick="onPickPose" @dragend="onDragEnd" />
              <div v-else class="preview-empty"><p>骨架预览</p></div>
            </div>
            <div class="wiz-controls">
              <div class="sec">
                <div class="sec-t">快速操作（选关节 = 旋转/平移该关节及其子树；空 = 整体）</div>
                <el-select v-model="rotJoint" size="small" placeholder="作用于（空 = 整体）" clearable filterable style="width: 100%">
                  <el-option v-for="n in jointNames" :key="n" :label="n" :value="n" />
                </el-select>
                <div class="row3 wrap">
                  <el-button size="small" @click="rotate('x', -90)">X-90</el-button>
                  <el-button size="small" @click="rotate('x', 90)">X+90</el-button>
                  <el-button size="small" @click="rotate('y', -90)">Y-90</el-button>
                  <el-button size="small" @click="rotate('y', 90)">Y+90</el-button>
                  <el-button size="small" @click="rotate('z', -90)">Z-90</el-button>
                  <el-button size="small" @click="rotate('z', 90)">Z+90</el-button>
                </div>
                <div class="row3 wrap">
                  <el-button size="small" type="primary" plain @click="horizontalize">水平化</el-button>
                  <el-button size="small" @click="translate(0, -20, 0)">上移</el-button>
                  <el-button size="small" @click="translate(0, 20, 0)">下移</el-button>
                  <el-button size="small" @click="translate(-20, 0, 0)">左移</el-button>
                  <el-button size="small" @click="translate(20, 0, 0)">右移</el-button>
                </div>
              </div>
              <div class="sec">
                <div class="sec-t">关节坐标</div>
                <el-select v-model="poseJoint" size="small" placeholder="选关节" filterable style="width: 100%">
                  <el-option v-for="n in jointNames" :key="n" :label="n" :value="n" />
                </el-select>
                <el-input v-model="poseStr" size="small" :placeholder="posePlaceholder" />
                <el-button size="small" type="primary" @click="setPose">设坐标</el-button>
              </div>
              <div class="sec">
                <div class="sec-t">画布 / 地面</div>
                <div class="row3">
                  <el-input-number v-model="canvas.width" size="small" :step="50" />
                  <el-input-number v-model="canvas.height" size="small" :step="50" />
                  <el-input-number v-model="canvas.floor_y" size="small" :step="10" />
                </div>
                <el-button size="small" type="primary" @click="setCanvas">保存画布</el-button>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="📐 体型参数" name="params">
          <div class="wiz-layout">
            <div class="wiz-controls full">
              <div class="sec">
                <div class="sec-t">新增体型参数（可调部位）</div>
                <el-input v-model="pcName" size="small" placeholder="参数名，如 head_scale" />
                <el-input v-model="pcJoints" size="small" placeholder="关节（逗号分隔），如 head" />
                <el-input v-model="pcLabel" size="small" placeholder="中文名，如 头大小（可空）" />
                <el-button size="small" type="primary" @click="addParam" icon="Plus">加参数</el-button>
              </div>
              <div class="sec">
                <div class="sec-t">已定义体型参数（{{ Object.keys(paramChains).length }}）</div>
                <div class="joint-list">
                  <div v-for="(pc, name) in paramChains" :key="name" class="joint-row">
                    <span class="mono">{{ pc.param || name }}</span><span class="parent">{{ pc.label }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 高级模式：JSON 直接编辑（与普通模式共享 draft） -->
    <div v-else class="advanced-body">
      <p class="hint">直接编辑骨骼产物 JSON（skeleton.json / default.json）。点「应用 JSON」同步到普通模式；普通模式改动也会在此刷新。</p>
      <div class="json-grid">
        <div class="json-box">
          <div class="json-head">skeleton.json</div>
          <el-input v-model="skeletonJson" type="textarea" :rows="22" class="mono json-editor" spellcheck="false" />
        </div>
        <div class="json-box">
          <div class="json-head">default.json</div>
          <el-input v-model="defaultJson" type="textarea" :rows="22" class="mono json-editor" spellcheck="false" />
        </div>
      </div>
      <div class="json-actions">
        <el-button size="small" @click="loadFiles">重新加载</el-button>
        <el-button size="small" type="primary" @click="applyFiles">应用 JSON 到普通模式</el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api.js'
import Skeleton3DViewer from '../components/Skeleton3DViewer.vue'

const props = defineProps({ speciesId: { type: String, required: true } })
const emit = defineEmits(['back', 'saved'])

const mode = ref('normal')  // normal | advanced
const sub = ref('skeleton') // skeleton | pose | params
const saving = ref(false)
const wiz = ref(null)

const nodes = computed(() => wiz.value?.nodes || {})
const chains = computed(() => wiz.value?.chains || {})
const paramChains = computed(() => wiz.value?.param_chains || {})
const pos3d = computed(() => wiz.value?.positions_3d || {})
const jointNames = computed(() => Object.keys(nodes.value))

// 关节树：按父级组织（根在最前），DFS 层级排序；孤儿/断链子树的根兜底
const jointTree = computed(() => {
  const childrenOf = {}
  const names = Object.keys(nodes.value)
  for (const name of names) {
    const nd = nodes.value[name]
    const p = (nd && nd.parent) || ''
    ;(childrenOf[p] ||= []).push(name)
  }
  for (const arr of Object.values(childrenOf)) arr.sort()
  const out = []
  const visited = new Set()
  const walk = (p, depth) => {
    for (const name of childrenOf[p] || []) {
      if (visited.has(name)) continue
      visited.add(name)
      out.push({ name, depth, parent: p || null })
      walk(name, depth + 1)
    }
  }
  walk('', 0)
  for (const name of names) { if (!visited.has(name)) walk(name, 0) }
  return out
})

const preview = computed(() => {
  const joints = {}
  const bones = []
  for (const [name, nd] of Object.entries(nodes.value)) {
    joints[name] = pos3d.value[name] || [0, 0, 0]
    if (nd.parent) bones.push([nd.parent, name])
  }
  return { joints, bones }
})

const nj = ref({ name: '', parent: null, posStr: '' })
const chainName = ref('')
const chainJoints = ref('')
const poseJoint = ref('')
const poseStr = ref('')
const rotJoint = ref('')

// 预览 ↔ 关节树联动：hover/选中高亮 + 3D 点选反向定位
const hoverJoint = ref('')
const selJoint = ref('')
const rowEls = {}
const highlightJoint = computed(() => selJoint.value || hoverJoint.value || '')
function toggleSel(name) { selJoint.value = selJoint.value === name ? '' : name }
function onPick(name) {
  selJoint.value = name
  hoverJoint.value = name
  const el = rowEls[name]
  if (el) el.scrollIntoView({ block: 'nearest' })
}
function onPickPose(name) {
  selJoint.value = name
  rotJoint.value = name  // 3D 点选 = 姿态操作作用对象（旋转/平移即作用于该关节）
}
const posePlaceholder = computed(() =>
  poseJoint.value && pos3d.value[poseJoint.value]
    ? `当前 ${pos3d.value[poseJoint.value].join(',')}（输入 x,y,z）`
    : 'x,y,z（如 480,300,0）')

// 变换面板（选中关节的位置 / 旋转 / 平移）
const xf = ref({ axis: 'z', angle: 15, dx: 0, dy: 0, dz: 0, pos: { x: 0, y: 0, z: 0 } })
watch(selJoint, (name) => {
  if (name && pos3d.value[name]) {
    const [x, y, z] = pos3d.value[name]
    xf.value.pos = { x, y, z }
  }
})
function clearSel() { selJoint.value = ''; hoverJoint.value = '' }
async function applyPos() {
  if (!selJoint.value) return
  const { x, y, z } = xf.value.pos
  try { await api.wizardPoseSet(props.speciesId, selJoint.value, [x, y, z]); await refresh() }
  catch (e) { ElMessage.error(e.message) }
}
async function applyRotate() { await rotateAxis(xf.value.axis, xf.value.angle) }
async function rotateAxis(axis, angle) {
  try { await api.wizardRotate(props.speciesId, { axis, angle, joint: selJoint.value || null }); await refresh() }
  catch (e) { ElMessage.error(e.message) }
}
async function applyTranslate() {
  const { dx, dy, dz } = xf.value
  try {
    await api.wizardTranslate(props.speciesId, { dx, dy, dz, joint: selJoint.value || null })
    xf.value.dx = 0; xf.value.dy = 0; xf.value.dz = 0
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}
async function onDragEnd({ name, dx, dy, dz }) {
  try { await api.wizardTranslate(props.speciesId, { dx, dy, dz, joint: name }); await refresh() }
  catch (e) { ElMessage.error(e.message) }
}
const canvas = ref({ width: 960, height: 600, floor_y: 470 })
const pcName = ref('')
const pcJoints = ref('')
const pcLabel = ref('')

// 高级 JSON
const skeletonJson = ref('')
const defaultJson = ref('')

async function refresh() {
  wiz.value = await api.wizardGet(props.speciesId)
  canvas.value = { ...(wiz.value?.canvas || { width: 960, height: 600, floor_y: 470 }) }
}

async function loadFiles() {
  try {
    const r = await api.wizardFiles(props.speciesId)
    skeletonJson.value = JSON.stringify(r.skeleton, null, 2)
    defaultJson.value = JSON.stringify(r.default, null, 2)
  } catch (e) { ElMessage.error(e.message) }
}

async function applyFiles() {
  let sk, df
  try { sk = JSON.parse(skeletonJson.value) } catch (e) { ElMessage.error('skeleton JSON 语法错误: ' + e.message); return }
  try { df = JSON.parse(defaultJson.value) } catch (e) { ElMessage.error('default JSON 语法错误: ' + e.message); return }
  try {
    await api.wizardSaveFiles(props.speciesId, sk, df)
    ElMessage.success('已应用 JSON，普通模式已同步')
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}

watch(mode, (m) => { if (m === 'advanced') loadFiles() })

onMounted(async () => { await refresh() })

// -- 骨架结构 --
async function addJoint() {
  const name = nj.value.name.trim()
  if (!name) { ElMessage.warning('请输入关节名'); return }
  const pos = nj.value.posStr.trim() ? nj.value.posStr.split(',').map(Number) : null
  try {
    await api.wizardJointAdd(props.speciesId, { name, parent: nj.value.parent, pos })
    nj.value.name = ''; nj.value.posStr = ''
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}
async function rmJoint(name) {
  try {
    await ElMessageBox.confirm(`删除关节「${name}」及其后代？`, '确认', { type: 'warning' })
    await api.wizardJointRm(props.speciesId, name)
    await refresh()
  } catch (e) { if (e !== 'cancel') ElMessage.error(e.message || e) }
}
async function mirrorJoint(name) {
  try { await api.wizardMirror(props.speciesId, name); await refresh() }
  catch (e) { ElMessage.error(e.message) }
}
async function changeParent(name, parent) {
  try {
    await api.wizardJointParent(props.speciesId, name, parent || null)
    await refresh()
  } catch (e) { ElMessage.error(e.message); await refresh() }
}
async function renameJoint(oldName) {
  let value = null
  try {
    const r = await ElMessageBox.prompt(`重命名「${oldName}」为：`, '改名', {
      inputValue: oldName,
      inputValidator: (v) => (v && v.trim() && v.trim() !== oldName) || '请输入不同的新名称',
    })
    value = r.value.trim()
  } catch (e) { return }  // 取消
  try {
    await api.wizardJointRename(props.speciesId, oldName, value)
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}
async function addChain() {
  const name = chainName.value.trim()
  const joints = chainJoints.value.split(',').map(s => s.trim()).filter(Boolean)
  if (!name || !joints.length) { ElMessage.warning('请填链名和关节'); return }
  try { await api.wizardChainAdd(props.speciesId, name, joints); chainName.value = ''; chainJoints.value = ''; await refresh() }
  catch (e) { ElMessage.error(e.message) }
}
// -- 姿态 --
async function setPose() {
  if (!poseJoint.value) { ElMessage.warning('先选关节'); return }
  const pos = poseStr.value.split(',').map(Number)
  if (pos.length !== 3 || pos.some(isNaN)) { ElMessage.warning('坐标格式：x,y,z'); return }
  try { await api.wizardPoseSet(props.speciesId, poseJoint.value, pos); await refresh() }
  catch (e) { ElMessage.error(e.message) }
}
async function rotate(axis, angle) {
  try {
    await api.wizardRotate(props.speciesId, { axis, angle, joint: rotJoint.value || null })
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}
async function horizontalize() {
  try {
    // 竖直姿态 → 水平：绕 Z 轴 90°（把 Y 方向主轴转成 X 方向），整体
    await api.wizardRotate(props.speciesId, { axis: 'z', angle: 90, joint: null })
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}
async function translate(dx, dy, dz) {
  try {
    await api.wizardTranslate(props.speciesId, { dx, dy, dz, joint: rotJoint.value || null })
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}
async function setCanvas() {
  try { await api.wizardCanvas(props.speciesId, { ...canvas.value }); ElMessage.success('画布已保存') }
  catch (e) { ElMessage.error(e.message) }
}
// -- 参数 --
async function addParam() {
  const name = pcName.value.trim()
  const joints = pcJoints.value.split(',').map(s => s.trim()).filter(Boolean)
  if (!name || !joints.length) { ElMessage.warning('请填参数名和关节'); return }
  try {
    await api.wizardParamAdd(props.speciesId, name, joints, { label: pcLabel.value || null })
    pcName.value = ''; pcJoints.value = ''; pcLabel.value = ''
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}

async function save() {
  saving.value = true
  try {
    await api.wizardCommit(props.speciesId)
    ElMessage.success('骨骼已保存')
    emit('saved')
  } catch (e) { ElMessage.error('保存失败: ' + e.message) }
  saving.value = false
}
</script>

<style scoped>
.skel-view { padding: 4px 0; }
.skel-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.crumb { font-size: .9rem; }
.crumb-root { color: #909399; } .crumb-sep { color: #c0c4cc; } .crumb-now { font-weight: 600; }
.mode-tabs { margin-bottom: 8px; }
.hint { color: #909399; font-size: .82rem; margin: 0 0 12px; }
.wiz-layout { display: grid; grid-template-columns: minmax(0, 1.45fr) minmax(0, 1fr); gap: 18px; align-items: start; }
.wiz-layout .full { grid-column: 1 / -1; }
.wiz-preview { border-radius: 8px; overflow: hidden; border: 1px solid #111827; min-height: 420px; }
.wiz-controls { display: flex; flex-direction: column; gap: 14px; max-height: calc(100vh - 250px); overflow-y: auto; padding-right: 6px; }
.sec { border: 1px solid #ebeef5; border-radius: 8px; padding: 10px 12px; background: #fafbfc; display: flex; flex-direction: column; gap: 8px; }
.sec-t { font-size: .78rem; font-weight: 600; color: #606266; }
.sec-xform { background: #f0f7ff; border-color: #c6e2ff; }
.xform-head { display: flex; align-items: center; gap: 8px; }
.xform-sel { font-family: monospace; font-size: .8rem; color: #409eff; }
.xform-block { display: flex; flex-direction: column; gap: 6px; padding: 6px 0; border-top: 1px dashed #d9ecff; }
.xform-t { font-size: .74rem; color: #606266; }
.joint-list { max-height: 46vh; min-height: 180px; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.joint-row { display: flex; align-items: center; gap: 6px; font-size: .8rem; padding: 2px 4px; border-radius: 4px; cursor: pointer; }
.joint-row:hover { background: #f0f2f5; }
.joint-row.selected { background: #ecf5ff; outline: 1px solid #409eff; }
.hint.small { font-size: .72rem; margin: 0 0 6px; }
.tree-mark { color: #c0c4cc; font-size: .72rem; width: 10px; flex: 0 0 auto; }
.tree-mark.root { color: #67c23a; }
.parent-sel { width: 96px; flex: 0 0 auto; }
.row-actions { margin-left: auto; display: flex; gap: 2px; white-space: nowrap; }
.mono { font-family: monospace; }
.parent { color: #909399; font-size: .72rem; flex: 1; }
.row3 { display: flex; gap: 6px; }
.row3.wrap { flex-wrap: wrap; }
.preview-empty { border: 1px dashed #d9d9d9; border-radius: 8px; min-height: 300px; display: flex; align-items: center; justify-content: center; color: #c0c4cc; }
.json-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.json-box { border: 1px solid #ebeef5; border-radius: 8px; }
.json-head { padding: 6px 10px; font-size: .78rem; font-weight: 600; border-bottom: 1px solid #ebeef5; background: #fafbfc; border-radius: 8px 8px 0 0; }
.json-actions { margin-top: 10px; display: flex; gap: 8px; }
</style>
