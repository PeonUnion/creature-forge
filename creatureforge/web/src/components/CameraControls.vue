<template>
  <div class="cam-controls" :class="{ compact }">
    <!-- 常驻：快捷视角按钮（轨道相机：一个点 → 看模型） -->
    <div class="cam-presets">
      <el-button v-for="p in presets" :key="p.label" size="small" text
                 :type="activePreset === p.label ? 'primary' : ''"
                 @click="applyPreset(p)">{{ p.label }}</el-button>
    </div>
    <!-- 常驻：相机设置按钮 → 弹出细调面板 -->
    <el-popover placement="bottom-end" :width="300" trigger="click" popper-class="cam-popover"
                :visible="panelOpen" @update:visible="panelOpen = $event">
      <template #reference>
        <el-button size="small" icon="Setting" :type="panelOpen ? 'primary' : ''">相机</el-button>
      </template>
      <div class="cam-panel">
        <div class="cam-panel-head">
          <span class="cam-panel-title">相机设置</span>
          <el-button size="small" text type="primary" @click="reset" icon="RefreshLeft">重置</el-button>
        </div>
        <!-- 轨道（绕模型中心）：角度/距离/缩放/平移 → yaw+pitch 任意角度、dist 任意距离 -->
        <div class="cam-group">
          <div class="cam-group-title">相机（yaw/pitch 任意角度 · dist 任意距离）</div>
          <div class="cam-row" v-for="item in orbitItems" :key="item.key">
            <span class="cam-label">{{ item.label }}</span>
            <el-slider class="cam-slider" :min="item.min" :max="item.max" :step="item.step"
                       :model-value="cam[item.key]" @update:model-value="set(item.key, $event)" />
            <span class="cam-val">{{ fmt(item, cam[item.key]) }}</span>
          </div>
        </div>
        <!-- 拖拽手感：轨迹球（物体跟随鼠标）/ 轨道（拖动相机） -->
        <div class="cam-row">
          <span class="cam-label">拖拽手感</span>
          <el-radio-group v-model="dragModeModel" size="small">
            <el-radio-button value="trackball">轨迹球</el-radio-button>
            <el-radio-button value="orbit">轨道</el-radio-button>
          </el-radio-group>
        </div>
        <div class="cam-tip">💡 左键拖拽 = 旋转 · Shift+左键拖拽 = 平移观察点（角度不变）</div>
      </div>
    </el-popover>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { dragMode, setDragMode } from '../composables/dragMode.js'

/**
 * 3D 相机控制（轨道相机：yaw/pitch/dist 决定相机位置，固定 FOV，任意角度+距离查看）。
 * 拖拽手感（轨迹球/轨道）全局共享，可在面板切换。
 * - 常驻：快捷视角按钮（正面/侧面/背面/45°/俯视/微仰）
 * - 面板：相机（yaw/pitch/dist/pan）+ 重置
 * - 配合预览图拖拽旋转（见 useOrbitDrag）
 * v-model 绑定相机状态 { yaw, pitch, dist, panX, panY }（dist 为距离倍数，1=自动适配）。
 */
const props = defineProps({
  modelValue: { type: Object, required: true },
  compact: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const panelOpen = ref(false)
const cam = computed(() => props.modelValue)
const set = (key, val) => emit('update:modelValue', { ...props.modelValue, [key]: val })
// 拖拽手感（轨迹球/轨道）全局共享，面板切换 + localStorage 持久化
const dragModeModel = computed({
  get: () => dragMode.value,
  set: (v) => setDragMode(v),
})

const DEFAULT_CAM = { yaw: 30, pitch: 12, dist: 1, panX: 0, panY: 0 }
function reset() { emit('update:modelValue', { ...DEFAULT_CAM }) }

const presets = [
  { label: '正面', yaw: 0, pitch: 0 },
  { label: '侧面', yaw: 90, pitch: 0 },
  { label: '背面', yaw: 180, pitch: 0 },
  { label: '斜侧', yaw: 45, pitch: 10 },
  { label: '俯视', yaw: 30, pitch: 30 },
  { label: '微仰', yaw: 30, pitch: -15 },
]
const applyPreset = (p) => emit('update:modelValue', { ...props.modelValue, yaw: p.yaw, pitch: p.pitch })

const activePreset = computed(() => {
  const p = presets.find(x => x.yaw === cam.value.yaw && x.pitch === cam.value.pitch)
  return p ? p.label : ''
})

// 统一面板项（骨架预览与动作预览一致，与 compact 无关；compact 仅影响样式）
// yaw+pitch 覆盖球面任意角度；dist 为距离倍数（1=自动适配，>1 拉远、<1 拉近），固定 FOV 不畸变
const orbitItems = [
  { key: 'yaw', label: '水平角', unit: '°', min: 0, max: 360, step: 1 },
  { key: 'pitch', label: '俯仰角', unit: '°', min: -90, max: 90, step: 1 },
  { key: 'dist', label: '距离', unit: '×', min: 0.2, max: 5, step: 0.05 },
  { key: 'panX', label: '平移 X', unit: '', min: -600, max: 600, step: 10 },
  { key: 'panY', label: '平移 Y', unit: '', min: -400, max: 400, step: 10 },
]

const fmt = (item, v) => (typeof v === 'number' ? Math.round(v * 100) / 100 : v) + item.unit
</script>

<style scoped>
.cam-controls { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.cam-presets { display: flex; flex-wrap: wrap; gap: 2px; }
.cam-panel { display: flex; flex-direction: column; gap: 6px; }
.cam-panel-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.cam-panel-title { font-weight: 600; color: #303133; font-size: .9rem; }
.cam-group { display: flex; flex-direction: column; gap: 4px; }
.cam-group-title { font-size: .72rem; color: #909399; margin-top: 2px; border-top: 1px dashed #e4e7ed; padding-top: 6px; }
.cam-row { display: flex; align-items: center; gap: 8px; }
.cam-label { width: 52px; flex-shrink: 0; color: #606266; font-size: .8rem; }
.cam-slider { flex: 1; }
.cam-val { width: 56px; text-align: right; font-family: monospace; color: #606266; font-size: .75rem; }
.cam-tip { font-size: .75rem; color: #909399; margin-top: 4px; }
</style>
