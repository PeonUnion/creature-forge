import { ref, onBeforeUnmount } from 'vue'

/**
 * 轨道相机拖拽交互（业界 OrbitControls 语义）：
 * - 左键拖拽 = 旋转（水平→yaw、垂直→pitch）
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
      const yaw = (((syaw + dx * 0.5) % 360) + 360) % 360
      const pitch = clamp(spitch + dy * 0.5, -90, 90)
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
