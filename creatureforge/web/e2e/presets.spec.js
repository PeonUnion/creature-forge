import { test, expect } from '@playwright/test'

// 预设管理全量 E2E（ToB 布局：表格列表 + 详情页）
test.describe('预设管理（全量 E2E）', () => {
  test('新建预设：选物种 → 初始化 → 体型/动作/预览 → 保存 → 列表 → 删除', async ({ page }) => {
    const pid = `e2e_preset_${Date.now()}`

    await page.goto('/#/presets')
    await expect(page.locator('.page-header h2', { hasText: '预设管理' })).toBeVisible()

    // 新建 → 选物种
    await page.locator('button', { hasText: '新建预设' }).first().click()
    await expect(page.locator('.panel-title', { hasText: '新建预设' })).toBeVisible()
    await page.locator('.el-select__wrapper').first().click()
    await page.locator('.el-select-dropdown__item', { hasText: '人类骨骼拓扑' }).last()
      .evaluate((el) => el.click())
    await page.locator('button', { hasText: '初始化预设' }).click()

    await expect(page.locator('.el-form-item', { hasText: '物种（schema 来源）' })).toContainText('human')

    // 填 preset_id + 名称
    const inputs = page.locator('.el-input__inner')
    await inputs.nth(0).fill(pid)
    await inputs.nth(1).fill('E2E 测试预设')

    // 体型参数 tab
    await expect(page.locator('.param-item', { hasText: '头大小' })).toBeVisible()
    const headSlider = page.locator('.param-item', { hasText: '头大小' }).locator('.el-slider__runway').first()
    const hb = await headSlider.boundingBox()
    await page.mouse.click(hb.x + hb.width * 0.8, hb.y + hb.height / 2)
    await expect(page.locator('.param-item', { hasText: '头大小' }).locator('.val')).toHaveText(/.+/)

    // 动作管理 tab：从物种动作中选择添加 → 表格 → 配置详情
    await page.locator('.el-tabs__item', { hasText: '动作管理' }).click()
    await expect(page.locator('.act-table + .empty-inline', { hasText: '尚未添加动作' })).toBeVisible()
    await page.locator('.act-toolbar .el-select').click()
    await page.locator('.el-select-dropdown__item', { hasText: 'walk3d' }).last()
      .evaluate((el) => el.click())
    await page.locator('.act-toolbar button', { hasText: '添加动作' }).click()
    await expect(page.locator('.act-table .el-table__row', { hasText: 'walk3d' })).toBeVisible()
    await page.locator('.act-table .el-table__row', { hasText: 'walk3d' })
      .getByRole('button', { name: '配置' }).click()
    await expect(page.locator('.act-detail', { hasText: 'Walk 3D' })).toBeVisible()

    // 预览 tab：骨架实时渲染（限定可见 canvas，避开非激活 tab）
    await page.locator('.el-tabs__item', { hasText: '预览' }).click()
    await expect(page.locator('.sk3d:visible canvas')).toBeVisible({ timeout: 30_000 })
    await page.locator('.preview-controls .el-select__wrapper').first().click()
    await page.locator('.el-select-dropdown__item', { hasText: 'Walk 3D' }).last()
      .evaluate((el) => el.click())
    await expect(page.locator('.el-tab-pane:visible .preview-controls .el-select__selected-item').filter({ hasText: 'Walk 3D' })).toHaveCount(1)

    // 保存 → 返回列表 → 表格出现 pid
    await page.locator('button', { hasText: '保存预设' }).click()
    await expect(page.locator('button', { hasText: '返回列表' })).toBeVisible({ timeout: 10_000 })
    await page.locator('button', { hasText: '返回列表' }).click()
    await expect(page.locator('.el-table__row .cell-id', { hasText: pid })).toBeVisible({ timeout: 10_000 })

    // 删除
    const row = page.locator('.el-table__row', { hasText: pid }).first()
    await row.getByRole('button', { name: '删除' }).click()
    await page.locator('.el-message-box__btns button', { hasText: '确定' }).click()
    await expect(page.locator('.el-table__row', { hasText: pid })).toHaveCount(0)
  })

  test('预设列表：无预设时显示空状态', async ({ page }) => {
    await page.goto('/#/presets')
    await expect(page.locator('.empty-list', { hasText: '暂无预设' })).toBeVisible()
  })

  test('预设查看/编辑：打开已有 → 调整体型 → 保存 → 重新打开验证 → 删除', async ({ page }) => {
    const pid = `e2e_edit_${Date.now()}`
    await page.goto('/#/presets')
    // 新建 + 保存
    await page.locator('button', { hasText: '新建预设' }).first().click()
    await page.locator('.el-select__wrapper').first().click()
    await page.locator('.el-select-dropdown__item', { hasText: '人类骨骼拓扑' }).last()
      .evaluate((el) => el.click())
    await page.locator('button', { hasText: '初始化预设' }).click()
    const inputs = page.locator('.el-input__inner')
    await inputs.nth(0).fill(pid)
    await inputs.nth(1).fill('E2E 编辑预设')
    await page.locator('button', { hasText: '保存预设' }).click()
    await page.locator('button', { hasText: '返回列表' }).click()
    await expect(page.locator('.el-table__row .cell-id', { hasText: pid })).toBeVisible({ timeout: 10_000 })

    // 打开已有（表格「编辑」）→ 改名 → 保存 → 返回 → 验证
    const row = page.locator('.el-table__row', { hasText: pid }).first()
    await row.getByRole('button', { name: '编辑' }).click()
    await expect(page.locator('.crumb-now', { hasText: 'E2E 编辑预设' })).toBeVisible()
    await page.locator('.el-form-item', { hasText: '名称' }).locator('input').fill('E2E 改名后')
    await page.locator('button', { hasText: '保存预设' }).click()
    await page.locator('button', { hasText: '返回列表' }).click()
    await expect(page.locator('.el-table__row .cell-title', { hasText: 'E2E 改名后' })).toBeVisible({ timeout: 10_000 })

    // 重新打开：名称已持久化
    await page.locator('.el-table__row', { hasText: pid }).first().getByRole('button', { name: '编辑' }).click()
    await expect(page.locator('.crumb-now', { hasText: 'E2E 改名后' })).toBeVisible()
    await page.locator('button', { hasText: '返回列表' }).click()

    // 删除
    await page.locator('.el-table__row', { hasText: pid }).first().getByRole('button', { name: '删除' }).click()
    await page.locator('.el-message-box__btns button', { hasText: '确定' }).click()
    await expect(page.locator('.el-table__row', { hasText: pid })).toHaveCount(0)
  })
})
