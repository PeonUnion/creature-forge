import { ref, onBeforeUnmount } from 'vue'
import { dragMode } from './dragMode.js'

/**
 * 3D 预览拖拽交互：
 * - 左键拖拽 = 旋转（水平→yaw、垂直→pitch）
 *   - trackball（默认）：物体表面跟随鼠标 = 转动手办本体（右拖→手办右转=yaw 增）
 *   - orbit：轨道相机（右拖→手办左转=yaw 减，像拖动相机；Three.js/Blender 可切换）
 * - 上拖（dy<0）→ 俯视看到顶部（pitch 增），两种手感一致
 * - **Shift + 左键拖拽 = 平移观察目标**（水平→panX、垂直→panY，不改变观测角度）
 *
 * 视角变化通过 setCam 回调交给父级（通常配合 watch cam → debounce 重渲染）。
 *
 * @param {Object} opts
 * @param {() => Object} opts.getCam   读取当前相机 {yaw, pitch, panX, panY, ...}
 * @param {(cam: Object) => void} opts.setCam  更新相机
 * @param {(e: MouseEvent) => void} [opts.onDragStart]
 * @param {(e: MouseEvent) => void} [opts.onDragEnd]
 * @returns {{ onMouseDown: (e: MouseEvent) => void, isDragging: import('vue').Ref<boolean> }}
 */
export function useOrbitDrag({ getCam, setCam, onDragStart, onDragEnd }) {
  const dragging = ref(false)
  let sx = 0, sy = 0, syaw = 0, spitch = 0, spanX = 0, spanY = 0, mode = 'rotate'

  const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v))

  function onMouseMove(e) {
    if (!dragging.value) return
    const dx = e.clientX - sx
    const dy = e.clientY - sy
    const cam = { ...getCam() }
    if (mode === 'pan') {
      // 平移：拖拽方向 = 画面移动方向（场景跟随鼠标），保持 yaw/pitch 不变
      setCam({ ...cam, panX: spanX + dx, panY: spanY + dy })
    } else {
      // trackball：物体表面跟随鼠标（右拖→手办右转=yaw 增）；orbit：拖相机（右拖→yaw 减）
      const dir = dragMode.value === 'trackball' ? 1 : -1
      const yaw = (((syaw + dx * 0.5 * dir) % 360) + 360) % 360
      const pitch = clamp(spitch - dy * 0.5, -90, 90)  // 上拖→俯视看到顶部，两种一致
      setCam({ ...cam, yaw, pitch })
    }
  }

  function end(e) {
    if (!dragging.value) return
    dragging.value = false
    window.removeEventListener('mousemove', onMouseMove)
    window.removeEventListener('mouseup', end)
    document.body.style.cursor = ''
    if (onDragEnd) onDragEnd(e)
  }

  function onMouseDown(e) {
    if (e.button !== 0) return
    dragging.value = true
    sx = e.clientX; sy = e.clientY
    const c = getCam() || {}
    syaw = c.yaw ?? 0
    spitch = c.pitch ?? 0
    spanX = c.panX ?? 0
    spanY = c.panY ?? 0
    mode = e.shiftKey ? 'pan' : 'rotate'
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', end)
    document.body.style.cursor = 'grabbing'
    e.preventDefault()
    if (onDragStart) onDragStart(e)
  }

  onBeforeUnmount(() => {
    window.removeEventListener('mousemove', onMouseMove)
    window.removeEventListener('mouseup', end)
  })

  return { onMouseDown, isDragging: dragging }
}
