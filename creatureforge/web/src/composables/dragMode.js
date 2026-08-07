import { ref } from 'vue'

/**
 * 预览拖拽手感模式（全局共享 + localStorage 持久化）：
 * - 'trackball'（默认，用户偏好）：物体表面跟随鼠标 → 右拖物体右转，像"抓取物体"
 * - 'orbit'：轨道相机（Three.js OrbitControls / Blender 默认）→ 右拖物体左转，像"拖动相机"
 */
const STORAGE_KEY = 'creatureforge_drag_mode'

function load() {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    return v === 'orbit' || v === 'trackball' ? v : 'trackball'
  } catch {
    return 'trackball'
  }
}

export const dragMode = ref(load())

export function setDragMode(mode) {
  if (mode !== 'orbit' && mode !== 'trackball') return
  dragMode.value = mode
  try { localStorage.setItem(STORAGE_KEY, mode) } catch { /* ignore */ }
}
