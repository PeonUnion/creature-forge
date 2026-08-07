<template>
  <div class="motion-preview">
    <div class="mp-stage" :class="{ dragging }" @mousedown="onDragDown">
      <div v-if="sprite" ref="spriteEl" class="mp-sprite" :style="spriteStyle"></div>
      <div v-else class="preview-empty">
        <p>{{ loading ? '渲染中…' : '暂无预览帧，请先渲染动作' }}</p>
      </div>
      <span v-if="frameCount" class="mp-badge">{{ frameIndex + 1 }} / {{ frameCount }}</span>
      <span v-if="dragging" class="mp-orbit-hint">拖动旋转视角…</span>
    </div>
    <div class="mp-toolbar">
      <el-button size="small" :disabled="!sprite" :icon="playing ? 'VideoPause' : 'VideoPlay'" @click="togglePlay">
        {{ playing ? '暂停' : '播放' }}
      </el-button>
      <el-button size="small" type="primary" :disabled="!sprite" :loading="gifLoading" icon="Download" @click="exportGif">
        导出 GIF
      </el-button>
      <span class="mp-hint">拖拽画面旋转视角 · GIF 由后端按当前相机视角逐帧合成</span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { api } from '../api.js'
import { ElMessage } from 'element-plus'
import { useOrbitDrag } from '../composables/useOrbitDrag.js'

const props = defineProps({
  /** 帧拼接大图（PNG data URL，后端 sprite=1 合成）——一次请求、一次解码，性能最优 */
  sprite: { type: String, default: '' },
  /** 帧数 / 单帧宽高（用于 CSS 逐帧动画与帧计数） */
  frameCount: { type: Number, default: 0 },
  frameW: { type: Number, default: 0 },
  frameH: { type: Number, default: 0 },
  /** 播放速度（帧/秒） */
  fps: { type: Number, default: 6 },
  /** 相机参数 {yaw,pitch,dist,panX,panY}（dist 为距离倍数），拖拽旋转 + 导出 GIF 用 */
  cam: { type: Object, default: () => ({}) },
  /** 物种 ID（导出 GIF 需要） */
  speciesId: { type: String, default: '' },
  /** 动作 ID（导出 GIF 需要） */
  motionId: { type: String, default: '' },
  /** 渲染中状态（无 sprite 时显示） */
  loading: { type: Boolean, default: false },
})
const emit = defineEmits(['update:cam'])

const spriteEl = ref(null)
const playing = ref(false)
const gifLoading = ref(false)
const frameIndex = ref(0)
let badgeRaf = null

// 播放：纯 CSS 动画（steps 逐帧 + background-position 位移）。
// 零 JS 定时器、单图单次解码、GPU 合成；暂停用 animation-play-state（完美定格）。
// ⚠️ background-position 百分比公式：偏移 = (容器宽 - 背景图宽) * p。背景图宽 = N*容器宽，
//    0% → 偏移 0（帧0）；100% → 偏移 -(N-1)*容器宽（帧 N-1）。steps(N-1) 每次精确跳一帧。
const spriteStyle = computed(() => {
  const n = props.frameCount || 0
  if (!props.sprite || !n) return {}
  const steps = Math.max(n - 1, 1)
  const dur = (n / Math.max(props.fps, 1)).toFixed(3)
  return {
    width: '100%',
    aspectRatio: `${props.frameW || 1} / ${props.frameH || 1}`,
    backgroundImage: `url(${props.sprite})`,
    backgroundSize: `${n * 100}% 100%`,
    backgroundRepeat: 'no-repeat',
    animation: `mp-play ${dur}s steps(${steps}) infinite`,
    animationPlayState: playing.value ? 'running' : 'paused',
  }
})

function togglePlay() {
  if (!props.sprite) return
  playing.value = !playing.value
  if (playing.value) startBadge()
  else stopBadge()
}

function stop() {
  playing.value = false
  stopBadge()
}

// 帧计数：Web Animations API 读当前播放时间 → 计算帧号（rAF 仅更新文本，不驱动播放）
function updateBadge() {
  badgeRaf = null
  if (!playing.value || !spriteEl.value) return
  const anim = spriteEl.value.getAnimations?.()[0]
  if (anim) {
    const d = anim.effect?.getTiming?.().duration ?? 1
    const t = anim.currentTime ?? 0
    const n = props.frameCount || 1
    frameIndex.value = Math.floor(((t % d) / d) * n) % n
  }
  badgeRaf = requestAnimationFrame(updateBadge)
}
function startBadge() { stopBadge(); badgeRaf = requestAnimationFrame(updateBadge) }
function stopBadge() { if (badgeRaf) { cancelAnimationFrame(badgeRaf); badgeRaf = null } }

// 新 sprite（重渲染/切动作）→ 帧计数归零
watch(() => props.sprite, () => { frameIndex.value = 0 })

// 轨道相机：拖拽预览图旋转视角（父级 watch cam 自动重渲染）
const { onMouseDown: onDragDown, isDragging } = useOrbitDrag({
  getCam: () => props.cam || {},
  setCam: (c) => emit('update:cam', c),
})
const dragging = isDragging

/** 导出 GIF：后端按当前相机视角逐帧合成，浏览器下载 */
async function exportGif() {
  if (!props.motionId || !props.speciesId) { ElMessage.warning('缺少动作信息，无法导出 GIF'); return }
  gifLoading.value = true
  try {
    const c = props.cam || {}
    const qs = `species=${encodeURIComponent(props.speciesId)}&yaw=${c.yaw ?? 0}&pitch=${c.pitch ?? 0}` +
               `&dist=${c.dist ?? 1}&pan_x=${c.panX ?? 0}&pan_y=${c.panY ?? 0}&grid=0&gif=1`
    const r = await api.renderMotion3d(props.motionId, qs)
    if (r.gif) {
      const a = document.createElement('a')
      a.href = r.gif
      a.download = `${props.motionId}.gif`
      document.body.appendChild(a)
      a.click()
      a.remove()
      ElMessage.success('GIF 已导出')
    } else {
      ElMessage.error('GIF 生成失败')
    }
  } catch (e) { ElMessage.error(e.message) }
  gifLoading.value = false
}

onBeforeUnmount(stop)
</script>

<style scoped>
.motion-preview { display: flex; flex-direction: column; gap: 10px; }
.mp-stage { position: relative; display: flex; justify-content: center; min-height: 200px;
  cursor: grab; user-select: none; }
.mp-stage.dragging { cursor: grabbing; }
.mp-sprite {
  max-width: 100%; border: 1px solid #111827; border-radius: 8px; background-color: #111827;
  pointer-events: none;
}
.mp-badge { position: absolute; right: 8px; bottom: 8px; background: rgba(0,0,0,.65); color: #fff;
  font-size: .75rem; padding: 2px 8px; border-radius: 999px; }
.mp-orbit-hint { position: absolute; left: 50%; top: 12px; transform: translateX(-50%);
  background: rgba(0,0,0,.65); color: #fff; font-size: .75rem; padding: 3px 10px; border-radius: 999px; }
.mp-toolbar { display: flex; gap: 8px; align-items: center; }
.mp-hint { font-size: .75rem; color: #c0c4cc; }
.preview-empty { text-align: center; color: #c0c4cc; padding: 40px; border: 2px dashed #e4e7ed;
  border-radius: 8px; width: 100%; }
</style>
<!-- @keyframes 必须全局（非 scoped）：内联 style 引用的动画名不做 scoped hash -->
<style>
@keyframes mp-play {
  0% { background-position: 0 0; }
  100% { background-position: 100% 0; }
}
</style>