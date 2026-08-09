<template>
  <div class="skel-view">
    <div class="skel-head">
      <div class="crumb">
        <span class="crumb-root">物种</span><span class="crumb-sep">/</span>
        <span class="crumb-now">{{ wiz?.title || speciesId }}</span>
      </div>
      <div class="head-actions">
        <el-button size="small" @click="emit('back')">返回列表</el-button>
        <el-button size="small" type="primary" @click="save" :loading="saving" icon="Check">
          保存骨骼<span v-if="dirty" class="dirty-dot" title="有未保存的姿态修改" />
        </el-button>
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
              <div class="edit-plane-bar">
                <span class="plane-label">编辑</span>
                <el-radio-group v-model="editPlane" size="small">
                  <el-radio-button value="front">正面</el-radio-button>
                  <el-radio-button value="back">背面</el-radio-button>
                  <el-radio-button value="left">左侧视</el-radio-button>
                  <el-radio-button value="right">右侧视</el-radio-button>
                  <el-radio-button value="top">俯视</el-radio-button>
                  <el-radio-button value="bottom">仰视</el-radio-button>
                </el-radio-group>
                <el-switch v-model="snapEnabled" size="small" active-text="网格" inactive-text="自由" />
                <el-input-number v-model="gridStep" size="small" :min="1" :max="100" :disabled="!snapEnabled" style="width: 96px" />
                <span class="plane-legend">x 左右 · y 上下(高度) · z 前后(纵深)</span>
                <el-button size="small" text :disabled="!undoStack.length" @click="doUndo" title="撤销 (Ctrl+Z)">↶ 撤销</el-button>
                <el-button size="small" text :disabled="!redoStack.length" @click="doRedo" title="重做 (Ctrl+Y)">↷ 重做</el-button>
                <span class="plane-hint">落点吸附网格交叉点</span>
              </div>
              <Skeleton3DViewer v-if="preview" :joints="preview.joints" :bones="preview.bones"
                :head-radius="12.5" :center="[480, 300, 0]"
                :highlight="highlightJoint" :editable="true" :drag-plane="editPlane"
                :grid-step="snapEnabled ? gridStep : 0" @pick="onPick" @dragend="onDragEnd" @ready="(a) => (skeletonViewerApi = a)" />
              <div v-else class="preview-empty"><p>添加关节后实时显示骨架</p></div>
            </div>
            <div class="wiz-controls">
              <div class="sec sec-xform">
                <div class="sec-t xform-head">
                  <span>变换</span>
                  <span v-if="selJoint" class="xform-sel">{{ selJoint }}</span>
                  <el-button v-if="selJoint" size="small" text type="primary" @click="clearSel">清除</el-button>
                </div>
                <div v-if="!selJoint" class="xform-block">
                  <div class="xform-t">整体平移（全部关节）</div>
                  <div class="row3">
                    <el-input-number v-model="xf.dx" size="small" :step="5" />
                    <el-input-number v-model="xf.dy" size="small" :step="5" />
                    <el-input-number v-model="xf.dz" size="small" :step="5" />
                    <el-button size="small" type="primary" @click="applyTranslate">平移</el-button>
                  </div>
                  <div class="xform-t">整体旋转（绕质心）</div>
                  <div class="row3">
                    <el-select v-model="xf.axis" size="small" style="width: 64px">
                      <el-option v-for="a in ['x','y','z']" :key="a" :label="a.toUpperCase()" :value="a" />
                    </el-select>
                    <el-input-number v-model="xf.angle" size="small" :step="5" :min="-180" :max="180" />
                    <el-button size="small" type="primary" @click="applyRotate">旋转</el-button>
                  </div>
                  <p class="hint small">点击 3D 关节球或关节树选中关节后，可单独编辑位置/旋转/平移。</p>
                </div>
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
                    <div class="xform-t">坐标分量参数化（引用坐标参数，对称共享）</div>
                    <div v-for="ax in ['x','y','z']" :key="ax" class="axis-row">
                      <span class="mono axis-t">{{ ax.toUpperCase() }}</span>
                      <el-select v-model="axMode[ax].kind" size="small" style="width: 64px" @change="applyAxis(ax)">
                        <el-option value="const" label="常量" />
                        <el-option value="param" label="参数" />
                      </el-select>
                      <template v-if="axMode[ax].kind === 'param'">
                        <el-select v-model="axMode[ax].param" size="small" placeholder="参数" style="width: 122px" filterable @change="applyAxis(ax)">
                          <el-option v-for="(s, pn) in (wiz?.params || {})" :key="pn" :label="`${pn} · ${s.label || ''}`" :value="pn" />
                        </el-select>
                        <el-select v-model="axMode[ax].op" size="small" style="width: 70px" @change="applyAxis(ax)">
                          <el-option value="direct" label="直接" />
                          <el-option value="neg" label="取负" />
                          <el-option value="mul" label="倍数" />
                          <el-option value="add" label="偏移" />
                        </el-select>
                        <el-input-number v-if="axMode[ax].op === 'mul' || axMode[ax].op === 'add' || axMode[ax].op === 'neg'" v-model="axMode[ax].k" size="small" :step="1" @change="applyAxis(ax)" style="width: 84px" />
                        <el-button size="small" text type="danger" @click="clearAxis(ax)">固化常量</el-button>
                      </template>
                      <el-input-number v-else v-model="axMode[ax].val" size="small" :step="1" @change="applyAxis(ax)" style="width: 110px" />
                    </div>
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
                <div class="sec-t xform-head">
                  <span>全部关节坐标</span>
                  <el-button size="small" text type="primary" @click="showCoords = !showCoords">{{ showCoords ? '收起' : '展开' }}</el-button>
                </div>
                <div v-if="showCoords" class="coord-table">
                  <div v-for="n in jointNames" :key="n" class="coord-row">
                    <span class="mono coord-name">{{ n }}</span>
                    <el-input-number v-model="coordVals[n].x" size="small" :step="5" @change="applyCoord(n)" />
                    <el-input-number v-model="coordVals[n].y" size="small" :step="5" @change="applyCoord(n)" />
                    <el-input-number v-model="coordVals[n].z" size="small" :step="5" @change="applyCoord(n)" />
                  </div>
                </div>
              </div>
              <div class="sec">
                <div class="sec-t xform-head">
                  <span>坐标参数（暴露给预设）</span>
                  <el-button size="small" text type="primary" :loading="extracting" @click="extractSym">提取对称</el-button>
                </div>
                <div class="param-table">
                  <div v-for="(spec, pname) in (wiz?.params || {})" :key="pname" class="param-row">
                    <span class="mono pkey">{{ pname }}</span>
                    <el-input v-model="spec.label" size="small" placeholder="中文名" style="width: 100px" @change="touchParams" />
                    <el-input-number v-model="spec.default" size="small" :step="1" @change="touchParams" />
                  </div>
                </div>
                <div class="row3 wrap">
                  <el-input v-model="npName" size="small" placeholder="引用名" style="width: 96px" />
                  <el-input v-model="npLabel" size="small" placeholder="中文名" style="width: 96px" />
                  <el-input-number v-model="npDefault" size="small" :step="1" />
                  <el-button size="small" @click="addCoordParam">加参数</el-button>
                </div>
                <p class="hint small">参数化分量存为引用表达式，改默认值对称侧同步；保存后暴露给预设快速建变体。</p>
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
                :highlight="highlightJoint" :editable="true" :drag-plane="editPlane"
                :grid-step="snapEnabled ? gridStep : 0" @pick="onPickPose" @dragend="onDragEnd" @ready="(a) => (poseViewerApi = a)" />
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
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api.js'
import Skeleton3DViewer from '../components/Skeleton3DViewer.vue'

const props = defineProps({ speciesId: { type: String, required: true } })
const emit = defineEmits(['back', 'saved'])

const mode = ref('normal')  // normal | advanced
const sub = ref('skeleton') // skeleton | pose | params
const saving = ref(false)
const dirty = ref(false)  // 有未保存的姿态/坐标修改（本地暂存，保存按钮统一提交）
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

// 坐标表达式求值（数值=常量；dict=参数引用/计算，复用后端 motion DSL 子集）
function evalCoordExpr(v, params) {
  if (typeof v === 'number') return v
  if (v && typeof v === 'object' && !Array.isArray(v)) {
    const k = Object.keys(v)[0]
    const a = v[k]
    switch (k) {
      case 'const': return a
      case 'param': return params[a] ?? 0
      case 'neg': return -evalCoordExpr(a, params)
      case 'add': return a.reduce((s, x) => s + evalCoordExpr(x, params), 0)
      case 'sub': return evalCoordExpr(a[0], params) - evalCoordExpr(a[1], params)
      case 'mul': return a.reduce((s, x) => s * evalCoordExpr(x, params), 1)
      case 'div': return evalCoordExpr(a[0], params) / (evalCoordExpr(a[1], params) || 1)
      default: return 0
    }
  }
  return 0
}
// 坐标参数值（skeleton 顶层 params 默认值；可被预设 body 覆盖）
const coordParamsVal = computed(() => {
  const out = {}
  for (const [k, s] of Object.entries(wiz.value?.params || {})) out[k] = Number(s?.default ?? 0)
  return out
})
const preview = computed(() => {
  const joints = {}
  const bones = []
  const P = coordParamsVal.value
  for (const [name, nd] of Object.entries(nodes.value)) {
    const raw = pos3d.value[name]
    if (Array.isArray(raw)) joints[name] = raw.map(v => evalCoordExpr(v, P))
    else if (raw && typeof raw === 'object') joints[name] = ['x', 'y', 'z'].map(a => evalCoordExpr(raw[a], P))
    else joints[name] = [0, 0, 0]
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

// -- 姿态本地编辑（暂存内存，点「保存骨骼」才写入草稿并落盘） --
const r2 = (v) => Math.round(v * 100) / 100
function subtreeOf(name) {
  const out = [name]
  const nds = wiz.value?.nodes || {}
  for (let i = 0; i < out.length; i++) {
    const cur = out[i]
    for (const [n, nd] of Object.entries(nds)) {
      if (nd?.parent === cur && !out.includes(n)) out.push(n)
    }
  }
  return out
}

// -- 撤销 / 重做（本地姿态编辑，Ctrl+Z / Ctrl+Y；栈存 positions_3d 快照） --
const undoStack = ref([])
const redoStack = ref([])
const MAX_UNDO = 50
function posSnapshot() {
  const p = wiz.value?.positions_3d || {}
  return JSON.parse(JSON.stringify(p))
}
function pushUndo() {
  if (!wiz.value) return
  undoStack.value.push(posSnapshot())
  if (undoStack.value.length > MAX_UNDO) undoStack.value.shift()
  redoStack.value = []
}
function restorePos(snap) {
  if (!wiz.value) return
  wiz.value.positions_3d = snap
  dirty.value = true
  syncCoordVals()   // 同步「全部关节坐标」总表
  if (selJoint.value && snap[selJoint.value]) {
    const [x, y, z] = snap[selJoint.value]; xf.value.pos = { x, y, z }
  }
}
function doUndo() {
  if (!undoStack.value.length) { ElMessage.info('没有可撤销的操作'); return }
  redoStack.value.push(posSnapshot())
  restorePos(undoStack.value.pop())
}
function doRedo() {
  if (!redoStack.value.length) { ElMessage.info('没有可重做的操作'); return }
  undoStack.value.push(posSnapshot())
  restorePos(redoStack.value.pop())
}

function localRotate(axis, angle, joint) {
  if (!wiz.value) return
  pushUndo()
  const p = wiz.value.positions_3d ||= {}
  const names = joint ? subtreeOf(joint) : Object.keys(p)
  const pts = joint && p[joint] ? [p[joint]] : Object.values(p).filter(Boolean)
  const cx = pts.reduce((a, b) => a + b[0], 0) / pts.length
  const cy = pts.reduce((a, b) => a + b[1], 0) / pts.length
  const cz = pts.reduce((a, b) => a + b[2], 0) / pts.length
  const rad = angle * Math.PI / 180, c = Math.cos(rad), s = Math.sin(rad)
  for (const n of names) {
    if (!Array.isArray(p[n])) continue
    const [x, y, z] = p[n]
    const dx = x - cx, dy = y - cy, dz = z - cz
    let nx = x, ny = y, nz = z
    if (axis === 'z') { nx = dx * c - dy * s + cx; ny = dx * s + dy * c + cy }
    else if (axis === 'y') { nx = dx * c + dz * s + cx; nz = -dx * s + dz * c + cz }
    else if (axis === 'x') { ny = dy * c - dz * s + cy; nz = dy * s + dz * c + cz }
    p[n] = [r2(nx), r2(ny), r2(nz)]
  }
  if (joint && p[joint]) { const [x, y, z] = p[joint]; xf.value.pos = { x, y, z } }
  dirty.value = true
}
function localTranslate(dx, dy, dz, joint) {
  if (!wiz.value) return
  pushUndo()
  const p = wiz.value.positions_3d ||= {}
  const names = joint ? subtreeOf(joint) : Object.keys(p)
  for (const n of names) if (Array.isArray(p[n])) p[n] = [r2(p[n][0] + dx), r2(p[n][1] + dy), r2(p[n][2] + dz)]
  if (joint && joint === selJoint.value && Array.isArray(p[joint])) {
    const [x, y, z] = p[joint]; xf.value.pos = { x, y, z }
  }
  dirty.value = true
}

// -- 变换面板（选中关节的位置 / 旋转 / 平移） --
const xf = ref({ axis: 'z', angle: 15, dx: 0, dy: 0, dz: 0, pos: { x: 0, y: 0, z: 0 } })

// 坐标分量参数化：把关节某轴绑定到坐标参数（引用名+计算方式），或固化为常量
const axMode = ref({ x: { kind: 'const', val: 0, param: '', op: 'direct', k: 1 }, y: { kind: 'const', val: 0, param: '', op: 'direct', k: 1 }, z: { kind: 'const', val: 0, param: '', op: 'direct', k: 1 } })
function parseAxis(v, P) {
  if (v && typeof v === 'object' && !Array.isArray(v)) {
    if (v.param) return { kind: 'param', param: v.param, op: 'direct', k: 1, val: evalCoordExpr(v, P) }
    if (v.neg && v.neg.param) return { kind: 'param', param: v.neg.param, op: 'neg', k: 1, val: evalCoordExpr(v, P) }
    if (v.mul && v.mul[0]?.param && v.mul[1]?.const) return { kind: 'param', param: v.mul[0].param, op: 'mul', k: v.mul[1].const, val: evalCoordExpr(v, P) }
    if (v.add && Array.isArray(v.add)) {
      // 兼容两种顺序：{"add":[{"param":p},{"const":k}]} 或 中心互补 {"add":[{"const":c},{"param":p}]}
      const pIdx = v.add.findIndex(x => x && x.param)
      const cIdx = v.add.findIndex(x => x && x.const !== undefined)
      if (pIdx >= 0 && cIdx >= 0) return { kind: 'param', param: v.add[pIdx].param, op: 'add', k: v.add[cIdx].const, val: evalCoordExpr(v, P) }
      // 中心互补取反：{"add":[{"const":c},{"neg":{"param":p}}]} → x = c - p
      const nIdx = v.add.findIndex(x => x && x.neg && x.neg.param)
      if (nIdx >= 0 && cIdx >= 0) return { kind: 'param', param: v.add[nIdx].neg.param, op: 'neg', k: v.add[cIdx].const, val: evalCoordExpr(v, P) }
    }
  }
  return { kind: 'const', val: typeof v === 'number' ? v : 0, param: '', op: 'direct', k: 1 }
}
function syncAxMode() {
  const name = selJoint.value
  const raw = name ? (wiz.value?.positions_3d?.[name]) : null
  const P = coordParamsVal.value
  if (Array.isArray(raw)) {
    axMode.value = { x: parseAxis(raw[0], P), y: parseAxis(raw[1], P), z: parseAxis(raw[2], P) }
  } else if (raw && typeof raw === 'object') {
    axMode.value = { x: parseAxis(raw.x, P), y: parseAxis(raw.y, P), z: parseAxis(raw.z, P) }
  } else {
    axMode.value = { x: { kind: 'const', val: 0, param: '', op: 'direct', k: 1 }, y: { kind: 'const', val: 0, param: '', op: 'direct', k: 1 }, z: { kind: 'const', val: 0, param: '', op: 'direct', k: 1 } }
  }
}
function applyAxis(ax) {
  if (!wiz.value || !selJoint.value) return
  const m = axMode.value[ax]
  const p = wiz.value.positions_3d ||= {}
  const cur = p[selJoint.value]
  const obj = Array.isArray(cur) ? { x: cur[0], y: cur[1], z: cur[2] } : (cur && typeof cur === 'object' ? { ...cur } : {})
  if (m.kind === 'const') {
    obj[ax] = m.val ?? 0
  } else {
    if (!m.param) return
    const ref = { param: m.param }
    if (m.op === 'neg') obj[ax] = (m.k && m.k !== 0) ? { add: [{ const: m.k }, { neg: ref }] } : { neg: ref }
    else if (m.op === 'mul') obj[ax] = { mul: [ref, { const: m.k ?? 1 }] }
    else if (m.op === 'add') obj[ax] = { add: [ref, { const: m.k ?? 0 }] }
    else obj[ax] = ref
  }
  p[selJoint.value] = obj
  dirty.value = true
  syncCoordVals()
  const P = coordParamsVal.value
  const xyz = Array.isArray(obj) ? obj.map(v => evalCoordExpr(v, P)) : ['x', 'y', 'z'].map(a => evalCoordExpr(obj[a], P))
  xf.value.pos = { x: xyz[0], y: xyz[1], z: xyz[2] }
}
function clearAxis(ax) {
  const m = axMode.value[ax]
  const name = selJoint.value
  const raw = name ? wiz.value?.positions_3d?.[name] : null
  const P = coordParamsVal.value
  const v = Array.isArray(raw) ? raw[['x', 'y', 'z'].indexOf(ax)] : (raw && typeof raw === 'object') ? raw[ax] : 0
  m.kind = 'const'
  m.val = Math.round(evalCoordExpr(v, P) * 100) / 100
  applyAxis(ax)
}
watch(selJoint, (name) => {
  syncAxMode()
  if (name) {
    const P = coordParamsVal.value
    const raw = pos3d.value[name]
    const xyz = Array.isArray(raw) ? raw.map(v => evalCoordExpr(v, P))
      : (raw && typeof raw === 'object') ? ['x', 'y', 'z'].map(a => evalCoordExpr(raw[a], P))
      : [0, 0, 0]
    xf.value.pos = { x: xyz[0], y: xyz[1], z: xyz[2] }
  }
})
function clearSel() { selJoint.value = ''; hoverJoint.value = '' }
function applyPos() {
  if (!selJoint.value || !wiz.value) return
  if (wiz.value.positions_3d?.[selJoint.value] && !Array.isArray(wiz.value.positions_3d[selJoint.value])) {
    ElMessage.info('该关节为参数化坐标，请通过「坐标参数」面板修改'); return
  }
  pushUndo()
  const { x, y, z } = xf.value.pos
  wiz.value.positions_3d ||= {}
  wiz.value.positions_3d[selJoint.value] = [r2(x), r2(y), r2(z)]
  dirty.value = true
}
function applyRotate() { rotateAxis(xf.value.axis, xf.value.angle) }
function rotateAxis(axis, angle) { localRotate(axis, angle, selJoint.value || null) }
function applyTranslate() {
  localTranslate(xf.value.dx, xf.value.dy, xf.value.dz, selJoint.value || null)
  xf.value.dx = 0; xf.value.dy = 0; xf.value.dz = 0
}
function onDragEnd({ name, dx, dy, dz }) {
  if (!wiz.value) return
  pushUndo()
  const p = wiz.value.positions_3d ||= {}
  if (name) {
    if (Array.isArray(p[name])) p[name] = [r2(p[name][0] + dx), r2(p[name][1] + dy), r2(p[name][2] + dz)]
  } else {
    for (const k of Object.keys(p)) if (Array.isArray(p[k])) p[k] = [r2(p[k][0] + dx), r2(p[k][1] + dy), r2(p[k][2] + dz)]
  }
  if (name && selJoint.value === name && Array.isArray(p[name])) {
    const [x, y, z] = p[name]; xf.value.pos = { x, y, z }
  }
  dirty.value = true
}
// 编辑视图与网格吸附
const editPlane = ref('front')    // front/back=正/背面(锁z) / left/right=左右侧视(锁x) / top/bottom=俯/仰视(锁y)

// 坐标参数（物种级，暴露给预设；引用名+label）
const extracting = ref(false)
const npName = ref('')
const npLabel = ref('')
const npDefault = ref(0)
function touchParams() { dirty.value = true }
async function extractSym() {
  extracting.value = true
  try {
    await api.wizardCoordExtract(props.speciesId)
    await refresh()
    ElMessage.success('已提取对称参数')
  } catch (e) { ElMessage.error(e.message) }
  extracting.value = false
}
function addCoordParam() {
  const name = npName.value.trim()
  if (!name || !wiz.value) { ElMessage.warning('请填引用名'); return }
  wiz.value.params ||= {}
  wiz.value.params[name] = { label: npLabel.value.trim() || name, default: npDefault.value }
  npName.value = ''; npLabel.value = ''
  dirty.value = true
}

// 全部关节坐标总表（批量查看/编辑 XYZ，直接修错位关节）
const showCoords = ref(false)
const coordVals = ref({})
function syncCoordVals() {
  const out = {}
  const P = coordParamsVal.value
  for (const n of jointNames.value) {
    const raw = pos3d.value[n]
    const xyz = Array.isArray(raw) ? raw.map(v => evalCoordExpr(v, P))
      : (raw && typeof raw === 'object') ? ['x', 'y', 'z'].map(a => evalCoordExpr(raw[a], P))
      : [0, 0, 0]
    out[n] = { x: xyz[0], y: xyz[1], z: xyz[2] }
  }
  coordVals.value = out
}
function applyCoord(n) {
  const v = coordVals.value[n]
  if (!wiz.value || !v) return
  if (wiz.value.positions_3d?.[n] && !Array.isArray(wiz.value.positions_3d[n])) return  // 参数化关节不覆盖表达式
  pushUndo()
  wiz.value.positions_3d ||= {}
  wiz.value.positions_3d[n] = [r2(v.x), r2(v.y), r2(v.z)]
  dirty.value = true
  if (n === selJoint.value) xf.value.pos = { x: r2(v.x), y: r2(v.y), z: r2(v.z) }
}
const snapEnabled = ref(true)    // 网格吸附开关
const gridStep = ref(5)          // 网格精度（落点吸附步长）
const skeletonViewerApi = ref(null)
const poseViewerApi = ref(null)
// 编辑视图 → 相机对齐为对应 2D 正交视角（正面/背面/左侧视/右侧视/俯视/仰视）
const PLANE_VIEW = {
  front: { yaw: 0, pitch: 0 }, back: { yaw: 180, pitch: 0 },
  left: { yaw: 270, pitch: 0 }, right: { yaw: 90, pitch: 0 },
  top: { yaw: 0, pitch: 90 }, bottom: { yaw: 0, pitch: -90 },
}
watch(editPlane, (pl) => {
  const cfg = PLANE_VIEW[pl] || { yaw: 0, pitch: 0 }
  skeletonViewerApi.value?.setView(cfg.yaw, cfg.pitch, 1)
  poseViewerApi.value?.setView(cfg.yaw, cfg.pitch, 1)
})
// 方向键微调移动选中关节（按当前编辑平面解释方向，步长=网格精度）
function onKeyMove(e) {
  if (mode.value !== 'normal' || (sub.value !== 'skeleton' && sub.value !== 'pose')) return
  const t = e.target
  const itype = t && t.tagName === 'INPUT' ? (t.type || '') : ''
  const isText = (t && t.tagName === 'INPUT' && !['radio', 'checkbox', 'button'].includes(itype)) ||
    (t && t.tagName === 'TEXTAREA') || (t && t.isContentEditable)
  if (isText) return   // 文本输入框内不拦截方向键；radio/checkbox 等允许
  // 撤销/重做快捷键：Ctrl+Z 撤销、Ctrl+Y / Ctrl+Shift+Z 重做
  const mod = e.ctrlKey || e.metaKey
  if (mod && (e.key.toLowerCase() === 'z' || e.key.toLowerCase() === 'y')) {
    e.preventDefault()
    if (e.key.toLowerCase() === 'y' || e.shiftKey) doRedo(); else doUndo()
    return
  }
  if (!selJoint.value) return
  const step = snapEnabled.value ? gridStep.value : 5
  const side = editPlane.value === 'left' || editPlane.value === 'right'  // 左右侧视：水平=z；正/背面：水平=x
  const topView = editPlane.value === 'top' || editPlane.value === 'bottom'  // 俯/仰视：水平面 x/z
  let dx = 0, dy = 0, dz = 0
  switch (e.key) {
    case 'ArrowLeft': side ? (dz = -step) : (dx = -step); break
    case 'ArrowRight': side ? (dz = step) : (dx = step); break
    case 'ArrowUp': topView ? (dz = step) : (dy = -step); break   // 俯视：屏幕上移=+z；垂直面：上移=高度 y 减小
    case 'ArrowDown': topView ? (dz = -step) : (dy = step); break
    default: return
  }
  e.preventDefault()
  localTranslate(dx, dy, dz, selJoint.value)
}
const canvas = ref({ width: 960, height: 600, floor_y: 470 })
const pcName = ref('')
const pcJoints = ref('')
const pcLabel = ref('')

// 高级 JSON
const skeletonJson = ref('')
const defaultJson = ref('')

async function refresh() {
  // 有未保存的姿态修改时，保留本地坐标（结构类操作 refresh 不会冲掉拖拽结果）
  const localPos = dirty.value && wiz.value ? { ...(wiz.value.positions_3d || {}) } : null
  const localParams = dirty.value && wiz.value ? { ...(wiz.value.params || {}) } : null
  wiz.value = await api.wizardGet(props.speciesId)
  if (localPos) wiz.value.positions_3d = { ...(wiz.value.positions_3d || {}), ...localPos }
  if (localParams) wiz.value.params = { ...(wiz.value.params || {}), ...localParams }
  canvas.value = { ...(wiz.value?.canvas || { width: 960, height: 600, floor_y: 470 }) }
  syncCoordVals()
  syncAxMode()
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
    dirty.value = false   // JSON 已覆盖草稿，本地姿态随 JSON 为准
    await refresh()
  } catch (e) { ElMessage.error(e.message) }
}

watch(mode, (m) => { if (m === 'advanced') loadFiles() })

onMounted(async () => {
  await refresh()
  window.addEventListener('keydown', onKeyMove)
})
onBeforeUnmount(() => window.removeEventListener('keydown', onKeyMove))

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
// -- 姿态（默认姿态 tab 快速操作，同样本地暂存） --
function setPose() {
  if (!poseJoint.value) { ElMessage.warning('先选关节'); return }
  const pos = poseStr.value.split(',').map(Number)
  if (pos.length !== 3 || pos.some(isNaN)) { ElMessage.warning('坐标格式：x,y,z'); return }
  if (!wiz.value) return
  pushUndo()
  wiz.value.positions_3d ||= {}
  if (!Array.isArray(wiz.value.positions_3d[poseJoint.value])) {
    ElMessage.info('该关节为参数化坐标，请通过「坐标参数」面板修改'); return
  }
  wiz.value.positions_3d[poseJoint.value] = [r2(pos[0]), r2(pos[1]), r2(pos[2])]
  dirty.value = true
}
function rotate(axis, angle) { localRotate(axis, angle, rotJoint.value || null) }
function horizontalize() { localRotate('z', 90, null) }
function translate(dx, dy, dz) { localTranslate(dx, dy, dz, rotJoint.value || null) }
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
    // 先批量写入草稿（仅当有未保存姿态修改），再落盘正式文件
    if (dirty.value) await api.wizardPoseApply(props.speciesId, wiz.value?.positions_3d || {})
    await api.wizardCommit(props.speciesId)
    dirty.value = false
    undoStack.value = []; redoStack.value = []
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
.dirty-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #f56c6c; margin-left: 6px; vertical-align: middle; }
.edit-plane-bar { display: flex; align-items: center; gap: 10px; padding: 8px 10px; background: #f7f9fc; border-bottom: 1px solid #ebeef5; flex-wrap: wrap; }
.plane-label { font-size: .76rem; color: #606266; font-weight: 600; }.plane-legend { font-size: .72rem; color: #67c23a; font-family: monospace; }.plane-hint { font-size: .72rem; color: #909399; margin-left: auto; }
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
.axis-row { display: flex; align-items: center; gap: 6px; font-size: .8rem; }
.axis-t { width: 16px; flex: 0 0 auto; color: #909399; font-weight: 600; }
.param-table { display: flex; flex-direction: column; gap: 4px; max-height: 30vh; overflow-y: auto; }
.param-row { display: flex; align-items: center; gap: 6px; font-size: .8rem; }
.param-row .pkey { width: 96px; flex: 0 0 auto; }
.param-row .el-input-number { width: 110px; flex: 0 0 auto; }
.coord-table { display: flex; flex-direction: column; gap: 3px; max-height: 50vh; overflow-y: auto; }
.coord-row { display: flex; align-items: center; gap: 4px; font-size: .8rem; }
.coord-name { width: 130px; flex: 0 0 auto; }
.coord-row .el-input-number { width: 96px; flex: 0 0 auto; }
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
