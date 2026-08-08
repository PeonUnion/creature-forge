import { test, expect } from '@playwright/test'

// 物种管理全量 E2E（ToB 布局：表格列表 + 详情页，骨骼/动作独立页面）
test.describe('物种管理（全量 E2E）', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/#/species')
  })

  test('物种列表：表格加载并显示 human', async ({ page }) => {
    await expect(page.locator('.el-table__row', { hasText: '人类骨骼拓扑' })).toBeVisible()
    // ToB：无左侧 sidebar 列表
    await expect(page.locator('.sidebar')).toHaveCount(0)
  })

  test('骨骼页：进入详情 → 骨架维护渲染（普通/高级双页签）', async ({ page }) => {
    const row = page.locator('.el-table__row', { hasText: '人类骨骼拓扑' }).first()
    await row.getByRole('button', { name: '骨骼' }).click()
    await expect(page.locator('.detail-tabs .el-tabs__item', { hasText: '骨骼' })).toBeVisible()
    // 普通/高级双页签
    await expect(page.locator('.mode-tabs .el-tabs__item', { hasText: '普通（语义化）' })).toBeVisible()
    await expect(page.locator('.mode-tabs .el-tabs__item', { hasText: '高级 JSON' })).toBeVisible()
    // 骨架结构 3D 预览（WebGL canvas 非空白）
    await expect(page.locator('.sk3d canvas')).toBeVisible({ timeout: 30_000 })
    await expect.poll(() => page.evaluate(() => {
      const c = document.querySelector('.sk3d canvas')
      const gl = c && (c.getContext('webgl2') || c.getContext('webgl'))
      if (!c || !gl) return 0
      const w = gl.drawingBufferWidth, h = gl.drawingBufferHeight
      const px = new Uint8Array(w * h * 4)
      gl.readPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, px)
      let n = 0
      for (let i = 0; i < px.length; i += 4) {
        if (px[i] !== 0x11 || px[i + 1] !== 0x18 || px[i + 2] !== 0x27) n++
      }
      return n
    }), { timeout: 10_000 }).toBeGreaterThan(100)
    // 高级 JSON：切页签 → skeleton/default 两个编辑器
    await page.locator('.mode-tabs .el-tabs__item', { hasText: '高级 JSON' }).click()
    await expect(page.locator('.json-box textarea')).toHaveCount(2)
    await expect(page.locator('.json-box .json-head', { hasText: 'skeleton.json' })).toBeVisible()
  })

  test('动作页：walk3d 编辑 → 预览渲染 → GIF 导出下载', async ({ page }) => {
    const row = page.locator('.el-table__row', { hasText: '人类骨骼拓扑' }).first()
    await row.getByRole('button', { name: '动作' }).click()
    await expect(page.locator('.cell-title', { hasText: 'Walk 3D' })).toBeVisible()
    // 动作表格内「编辑」walk3d
    await page.locator('.el-table .el-table__row', { hasText: 'walk3d' })
      .getByRole('button', { name: '编辑' }).click()
    await expect(page.locator('.crumb-now', { hasText: 'walk3d' })).toBeVisible({ timeout: 20_000 })
    // 预览 tab → 渲染
    await page.locator('.el-tabs__item', { hasText: '动作预览' }).click()
    await page.locator('.preview-controls button', { hasText: '渲染' }).click()
    await expect(page.locator('.sk3d canvas')).toBeVisible({ timeout: 40_000 })
    await expect(page.locator('.sk3d-badge', { hasText: '16' })).toBeVisible()
    // GIF 导出（浏览器下载）
    const dl = page.waitForEvent('download', { timeout: 60_000 })
    await page.locator('.preview-controls button', { hasText: '导出 GIF' }).click()
    const download = await dl
    expect(download.suggestedFilename()).toMatch(/\.gif$/)
  })

  test('动作预览：拖拽旋转视角（WebGL 把玩手办）', async ({ page }) => {
    const row = page.locator('.el-table__row', { hasText: '人类骨骼拓扑' }).first()
    await row.getByRole('button', { name: '动作' }).click()
    await page.locator('.el-table .el-table__row', { hasText: 'walk3d' })
      .getByRole('button', { name: '编辑' }).click()
    await page.locator('.el-tabs__item', { hasText: '动作预览' }).click()
    await page.locator('.preview-controls button', { hasText: '渲染' }).click()
    await expect(page.locator('.sk3d canvas')).toBeVisible({ timeout: 40_000 })
    // 初始：正面高亮（openAction 默认 yaw=0）
    await expect(page.locator('button', { hasText: '正面' })).toHaveClass(/primary/)
    // 拖拽 → TrackballControls 转动 → 相机角度变化
    const stage = page.locator('.sk3d')
    await stage.scrollIntoViewIfNeeded()
    const box = await stage.boundingBox()
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
    await page.mouse.down()
    await page.mouse.move(box.x + box.width / 2 + 70, box.y + box.height / 2 + 20, { steps: 8 })
    await page.mouse.up()
    await expect(page.locator('button', { hasText: '正面' })).not.toHaveClass(/primary/, { timeout: 5_000 })
  })

  test('物种 CRUD：新建向导（从 0）→ 列表 → 删除', async ({ page }) => {
    const sid = `e2e_sp_${Date.now()}`
    // 新建（向导）
    await page.locator('.page-header button', { hasText: '新建物种（向导）' }).click()
    // step1 基本信息
    await page.getByPlaceholder('如 dragon / humanoid_x').fill(sid)
    await page.getByPlaceholder('如 深渊幼龙').fill('E2E 测试物种')
    await page.getByRole('button', { name: '下一步' }).click()
    // step2 模板：默认 custom（从 0 开始）→ 下一步
    await page.getByRole('button', { name: '下一步' }).click()
    // step3 骨架：加根关节
    await page.getByPlaceholder('关节名，如 head / wing_l').fill('root')
    await page.getByRole('button', { name: '加关节' }).click()
    await expect(page.locator('.joint-row .mono', { hasText: 'root' })).toBeVisible()
    // 姿态 → 参数 → 完成并创建
    await page.getByRole('button', { name: '下一步' }).click()
    await page.getByRole('button', { name: '下一步' }).click()
    await page.getByRole('button', { name: '完成并创建' }).click()
    await expect(page.locator('.el-table__row', { hasText: sid })).toBeVisible({ timeout: 10_000 })

    // 删除
    const row = page.locator('.el-table__row', { hasText: sid }).first()
    await row.getByRole('button', { name: '删除' }).click()
    await page.locator('.el-message-box__btns button', { hasText: '删除' }).click()
    await expect(page.locator('.el-table__row', { hasText: sid })).toHaveCount(0)
  })

  test('动作 CRUD：新建动作 → 列表 → 删除', async ({ page }) => {
    const aid = `e2e_act_${Date.now()}`
    const row = page.locator('.el-table__row', { hasText: '人类骨骼拓扑' }).first()
    await row.getByRole('button', { name: '动作' }).click()
    await page.locator('button', { hasText: '新建动作' }).click()
    // 编辑 JSON 定义：填 motion_id
    const editor = page.locator('.acts-editor textarea')
    const json = JSON.parse(await editor.inputValue())
    json.motion_id = aid
    json.title = 'E2E 测试动作'
    await editor.fill(JSON.stringify(json, null, 2))
    await page.locator('.acts-editor button', { hasText: '保存动作' }).click()
    await expect(page.locator('.el-table__row', { hasText: aid })).toBeVisible({ timeout: 10_000 })
    // 删除
    await page.locator('.el-table .el-table__row', { hasText: aid })
      .getByRole('button', { name: '删除' }).click()
    await page.locator('.el-message-box__btns button', { hasText: '确定' }).click()
    await expect(page.locator('.el-table__row', { hasText: aid })).toHaveCount(0)
  })
})
