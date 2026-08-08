<template>
  <div class="wizard">
    <el-steps :active="step" align-center finish-status="success" class="wiz-steps">
      <el-step title="基本信息" />
      <el-step title="身体形态" />
      <el-step title="骨架结构" />
      <el-step title="默认姿态" />
      <el-step title="体型参数" />
    </el-steps>

    <!-- Step 1 基本信息 -->
    <div v-if="step === 0" class="wiz-panel">
      <h4>1. 基本信息</h4>
      <p class="hint">先给这个生物起个名字（ID 用于数据目录，如 dragon / my_beast）。</p>
      <el-form label-position="top" class="form-grid">
        <el-form-item label="物种 ID"><el-input v-model="form.species_id" placeholder="如 dragon / humanoid_x" /></el-form-item>
        <el-form-item label="名称"><el-input v-model="form.title" placeholder="如 深渊幼龙" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" /></el-form-item>
      </el-form>
    </div>

    <!-- Step 2 模板选择 -->
    <div v-if="step === 1" class="wiz-panel">
      <h4>2. 选择身体形态</h4>
      <p class="hint">模板只是起点，之后每一步都可自由修改；「从 0 开始」= 空骨架，逐步构建任意形态（龙/蛇/多足/触手…）。</p>
      <div class="template-grid">
        <div v-for="t in templateList" :key="t.morph_id" class="template-card"
             :class="{ selected: form.morph_id === t.morph_id }" @click="form.morph_id = t.morph_id">
          <div class="t-name">{{ t.title }}</div>
          <div class="t-id">{{ t.morph_id }}</div>
          <div class="t-meta">关节 {{ t.joint_count }} · 链 {{ t.chain_count }} · 动作 {{ t.actions.length }}</div>
          <div class="t-desc">{{ t.description }}</div>
        </div>
      </div>
    </div>

    <!-- Step 3 骨架结构 -->
    <div v-if="step === 2" class="wiz-panel">
      <h4>3. 骨架结构（可视化 + 表单，无需 JSON）</h4>
      <p class="hint">第一个关节即「根」；可「一键镜像」对称肢（如 wing_l → wing_r）。</p>
      <div class="wiz-layout">
        <div class="wiz-preview">
          <Skeleton3DViewer v-if="preview" :joints="preview.joints" :bones="preview.bones"
            :head-radius="12.5" :center="[480, 300, 0]" />
          <div v-else class="preview-empty"><p>添加关节后实时显示骨架</p></div>
        </div>
        <div class="wiz-controls">
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
            <div class="sec-t">关节列表（{{ jointNames.length }}）</div>
            <div class="joint-list">
              <div v-for="n in jointNames" :key="n" class="joint-row">
                <span class="mono">{{ n }}</span>
                <span class="parent">{{ nodes[n]?.parent ? '← ' + nodes[n].parent : '根' }}</span>
                <el-button size="small" text type="primary" @click="mirrorJoint(n)">镜像</el-button>
                <el-button size="small" text type="danger" @click="rmJoint(n)">删</el-button>
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
    </div>

    <!-- Step 4 默认姿态 -->
    <div v-if="step === 3" class="wiz-panel">
      <h4>4. 默认姿态（模板已带姿态可跳过；从 0 开始建议逐个摆开关节）</h4>
      <div class="wiz-layout">
        <div class="wiz-preview">
          <Skeleton3DViewer v-if="preview" :joints="preview.joints" :bones="preview.bones"
            :head-radius="12.5" :center="[480, 300, 0]" />
          <div v-else class="preview-empty"><p>骨架预览</p></div>
        </div>
        <div class="wiz-controls">
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
              <el-input-number v-model="canvas.width" size="small" :step="50" placeholder="宽" />
              <el-input-number v-model="canvas.height" size="small" :step="50" placeholder="高" />
              <el-input-number v-model="canvas.floor_y" size="small" :step="10" placeholder="地面" />
            </div>
            <el-button size="small" type="primary" @click="setCanvas">保存画布</el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- Step 5 体型参数 -->
    <div v-if="step === 4" class="wiz-panel">
      <h4>5. 体型参数（哪些部位可调长短胖瘦）</h4>
      <div class="wiz-layout">
        <div class="wiz-controls full">
          <div class="sec">
            <div class="sec-t">新增体型参数</div>
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
    </div>

    <div class="wiz-actions">
      <el-button @click="cancel">取消</el-button>
      <el-button v-if="isEdit" @click="emit('switch-json')">高级 JSON</el-button>
      <el-button v-if="step > (isEdit ? 2 : 0)" @click="step--">上一步</el-button>
      <el-button v-if="step < 4" type="primary" @click="next">下一步</el-button>
      <el-button v-if="step === 4" type="primary" @click="finish" :loading="committing">{{ isEdit ? '保存修改' : '完成并创建' }}</el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api.js'
import Skeleton3DViewer from '../components/Skeleton3DViewer.vue'

const emit = defineEmits(['done', 'cancel', 'switch-json'])

// 编辑模式：existingId 非空时加载已有物种进行语义化编辑（非 JSON）
const props = defineProps({ existingId: { type: String, default: '' } })
const isEdit = computed(() => !!props.existingId)

const step = ref(0)
const templateList = ref([])
const committing = ref(false)
const form = ref({ species_id: '', title: '', description: '', morph_id: 'custom' })

// 草稿状态
const wiz = ref(null)
const nodes = computed(() => wiz.value?.nodes || {})
const chains = computed(() => wiz.value?.chains || {})
const paramChains = computed(() => wiz.value?.param_chains || {})
const pos3d = computed(() => wiz.value?.positions_3d || {})
const jointNames = computed(() => Object.keys(nodes.value))

// 骨架预览：positions_3d + parent → Skeleton3DViewer 数据（Y-down，组件内部翻转）
const preview = computed(() => {
  const joints = {}
  const bones = []
  for (const [name, nd] of Object.entries(nodes.value)) {
    joints[name] = pos3d.value[name] || [0, 0, 0]
    if (nd.parent) bones.push([nd.parent, name])
  }
  return { joints, bones }
})

// 新增关节表单
const nj = ref({ name: '', parent: null, posStr: '' })
// 链表单
const chainName = ref('')
const chainJoints = ref('')
// 姿态
const poseJoint = ref('')
const poseStr = ref('')
const posePlaceholder = computed(() =>
  poseJoint.value && pos3d.value[poseJoint.value]
    ? `当前 ${pos3d.value[poseJoint.value].join(',')}（输入 x,y,z）`
    : 'x,y,z（如 480,300,0）')
const canvas = ref({ width: 960, height: 600, floor_y: 470 })
// 参数
const pcName = ref('')
const pcJoints = ref('')
const pcLabel = ref('')

onMounted(async () => {
  try {
    const r = await api.templates()
    templateList.value = r.templates || []
  } catch (e) { ElMessage.error('加载模板失败: ' + e.message) }
  // 编辑模式：加载已有物种 → 直接进入骨架结构步骤
  if (props.existingId) {
    try {
      const v = await api.wizardGet(props.existingId)
      wiz.value = v
      form.value.species_id = props.existingId
      form.value.title = v.title || ''
      form.value.description = v.description || ''
      canvas.value = { ...(v.canvas || { width: 960, height: 600, floor_y: 470 }) }
      step.value = 2
    } catch (e) { ElMessage.error('加载物种失败: ' + e.message) }
  }
})

function next() {
  if (step.value === 0) {
    if (!form.value.species_id.trim()) { ElMessage.warning('请填写物种 ID'); return }
  }
  step.value++
  if (step.value === 2) initWizard()
}

async function initWizard() {
  try {
    const r = await api.wizardInit(form.value.species_id.trim(), {
      morph_id: form.value.morph_id, title: form.value.title, description: form.value.description,
    })
    wiz.value = r
    canvas.value = { ...(r.canvas || { width: 960, height: 600, floor_y: 470 }) }
  } catch (e) { ElMessage.error(e.message); step.value = 1 }
}

async function refresh() {
  wiz.value = await api.wizardGet(form.value.species_id.trim())
}

async function addJoint() {
  const name = nj.value.name.trim()
  if (!name) { ElMessage.warning('请输入关节名'); return }
  const pos = nj.value.posStr.trim() ? nj.value.posStr.split(',').map(Number) : null
  try {
    await api.wizardJointAdd(form.value.species_id.trim(), { name, parent: nj.value.parent, pos })
    nj.value.name = ''; nj.value.posStr = ''
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}

async function rmJoint(name) {
  try {
    await ElMessageBox.confirm(`删除关节「${name}」及其后代？`, '确认', { type: 'warning' })
    await api.wizardJointRm(form.value.species_id.trim(), name)
    await refresh()
  } catch (e) { if (e !== 'cancel') ElMessage.error(e.message || e) }
}

async function mirrorJoint(name) {
  try {
    await api.wizardMirror(form.value.species_id.trim(), name)
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}

async function addChain() {
  const name = chainName.value.trim()
  const joints = chainJoints.value.split(',').map(s => s.trim()).filter(Boolean)
  if (!name || !joints.length) { ElMessage.warning('请填链名和关节'); return }
  try {
    await api.wizardChainAdd(form.value.species_id.trim(), name, joints)
    chainName.value = ''; chainJoints.value = ''
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}

async function setPose() {
  if (!poseJoint.value) { ElMessage.warning('先选关节'); return }
  const pos = poseStr.value.split(',').map(Number)
  if (pos.length !== 3 || pos.some(isNaN)) { ElMessage.warning('坐标格式：x,y,z'); return }
  try {
    await api.wizardPoseSet(form.value.species_id.trim(), poseJoint.value, pos)
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}

async function setCanvas() {
  try {
    await api.wizardCanvas(form.value.species_id.trim(), { ...canvas.value })
    ElMessage.success('画布已保存')
  } catch (e) { ElMessage.error(e.message) }
}

async function addParam() {
  const name = pcName.value.trim()
  const joints = pcJoints.value.split(',').map(s => s.trim()).filter(Boolean)
  if (!name || !joints.length) { ElMessage.warning('请填参数名和关节'); return }
  try {
    await api.wizardParamAdd(form.value.species_id.trim(), name, joints, { label: pcLabel.value || null })
    pcName.value = ''; pcJoints.value = ''; pcLabel.value = ''
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}

async function finish() {
  committing.value = true
  try {
    await api.wizardCommit(form.value.species_id.trim())
    ElMessage.success('物种已创建')
    emit('done')
  } catch (e) { ElMessage.error('提交失败: ' + e.message) }
  committing.value = false
}

async function cancel() {
  if (wiz.value) {
    try { await api.wizardDiscard(form.value.species_id.trim()) } catch (e) { /* 忽略 */ }
  }
  emit('cancel')
}
</script>

<style scoped>
.wizard { padding: 8px 0; }
.wiz-steps { margin-bottom: 20px; }
.wiz-panel { min-height: 380px; }
.wiz-panel h4 { margin: 0 0 6px; }
.hint { color: #909399; font-size: .82rem; margin: 0 0 12px; }
.template-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 12px; }
.template-card { border: 1px solid #e4e7ed; border-radius: 10px; padding: 12px 14px; cursor: pointer; background: #fff; transition: all .15s; }
.template-card:hover { border-color: #409eff; box-shadow: 0 2px 8px rgba(64,158,255,.12); }
.template-card.selected { border-color: #409eff; background: #ecf5ff; }
.t-name { font-weight: 600; font-size: .95rem; }
.t-id { font-family: monospace; font-size: .75rem; color: #909399; margin: 2px 0 6px; }
.t-meta { font-size: .72rem; color: #67c23a; margin-bottom: 6px; }
.t-desc { font-size: .75rem; color: #909399; line-height: 1.5; }
.wiz-layout { display: grid; grid-template-columns: 1fr 300px; gap: 16px; align-items: start; }
.wiz-layout .full { grid-column: 1 / -1; }
.wiz-preview { border-radius: 8px; overflow: hidden; border: 1px solid #111827; }
.wiz-controls { display: flex; flex-direction: column; gap: 14px; }
.sec { border: 1px solid #ebeef5; border-radius: 8px; padding: 10px 12px; background: #fafbfc; display: flex; flex-direction: column; gap: 8px; }
.sec-t { font-size: .78rem; font-weight: 600; color: #606266; }
.joint-list { max-height: 200px; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.joint-row { display: flex; align-items: center; gap: 8px; font-size: .8rem; padding: 2px 4px; border-radius: 4px; }
.joint-row:hover { background: #f0f2f5; }
.mono { font-family: monospace; }
.parent { color: #909399; font-size: .72rem; flex: 1; }
.row3 { display: flex; gap: 6px; }
.wiz-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 18px; padding-top: 14px; border-top: 1px solid #ebeef5; }
.preview-empty { border: 1px dashed #d9d9d9; border-radius: 8px; min-height: 300px; display: flex; align-items: center; justify-content: center; color: #c0c4cc; }
</style>
