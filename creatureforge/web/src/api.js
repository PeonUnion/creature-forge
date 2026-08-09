/**
 * CreatureForge API 层
 *
 * @module api
 */

const BASE = ''  // 同源

/**
 * 底层请求
 * @template T
 * @param {string} path
 * @param {RequestInit} [opts]
 * @returns {Promise<T>}
 */
async function raw(path, opts = {}) {
  const res = await fetch(BASE + path, opts)
  const body = await res.json().catch(() => ({}))
  if (body.ok === false) throw new Error(body.error || JSON.stringify(body))
  return /** @type {T} */ (body)
}

/** JSON 请求辅助 */
const json = (body) => ({
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})
const json_put = (body) => ({
  method: 'PUT',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})

// =========================================================================
// API 方法
// =========================================================================

export const api = {
  // -- 物种 Species --

  /** 物种列表 @returns {Promise<{species: import('../../models').SpeciesListItem[]}>} */
  species: () => raw('/api/species'),

  /** 物种详情（含动作） @returns {Promise<import('../../models').SpeciesDetail>} */
  speciesDetail: (id) => raw(`/api/species/${encodeURIComponent(id)}`),

  /** 物种预设 schema（随物种自动派生，创建预设的清单） @returns {Promise<object>} */
  presetSchema: (id) => raw(`/api/species/${encodeURIComponent(id)}/preset_schema`),

  /** 创建物种 */
  createSpecies: (data) => raw('/api/species', json(data)),

  /** 更新物种 */
  updateSpecies: (id, data) => raw(`/api/species/${encodeURIComponent(id)}`, json_put(data)),

  /** 删除物种 */
  deleteSpecies: (id) => raw(`/api/species/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  // -- 物种动作 Action --

  /** 动作详情 @returns {Promise<import('../../models').Motion>} */
  actionDetail: (speciesId, actionId) => raw(`/api/species/${encodeURIComponent(speciesId)}/actions/${encodeURIComponent(actionId)}`),

  /** 创建动作 */
  createAction: (speciesId, data) => raw(`/api/species/${encodeURIComponent(speciesId)}/actions`, json(data)),

  /** 更新动作 */
  updateAction: (speciesId, actionId, data) => raw(`/api/species/${encodeURIComponent(speciesId)}/actions/${encodeURIComponent(actionId)}`, json_put(data)),

  /** 删除动作 */
  deleteAction: (speciesId, actionId) => raw(`/api/species/${encodeURIComponent(speciesId)}/actions/${encodeURIComponent(actionId)}`, { method: 'DELETE' }),

  /** 动作参数提取：单一 intensity → 整体+部位/维度多参数（写回动作 JSON） */
  actionExtractParams: (speciesId, actionId) =>
    raw(`/api/species/${encodeURIComponent(speciesId)}/actions/${encodeURIComponent(actionId)}/extract-params`, json({})),

  // -- 物种默认参数（物种自带姿态/体型；动作与骨架预览的基础） --

  /** 读物种默认参数 @returns {Promise<object>} */
  speciesDefault: (id) => raw(`/api/species/${encodeURIComponent(id)}/default`),

  /** 保存物种默认参数 */
  saveSpeciesDefault: (id, data) => raw(`/api/species/${encodeURIComponent(id)}/default`, json(data)),

  // -- 物种分步向导（模板可选择 + custom 从 0 开始） --

  /** 形态模板列表（含 custom 从 0 开始） @returns {Promise<{templates: object[]}>} */
  templates: () => raw('/api/templates'),

  /** 向导草稿视图 @returns {Promise<object>} */
  wizardGet: (id) => raw(`/api/wizard/${encodeURIComponent(id)}`),

  /** 向导初始化（选模板/从 0） */
  wizardInit: (id, { morph_id = 'custom', title = '', description = '' } = {}) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/init`, json({ morph_id, title, description })),

  /** 新增关节 */
  wizardJointAdd: (id, { name, parent = null, pos = null, sym = null } = {}) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/joint/add`, json({ name, parent, pos, sym })),

  /** 删除关节 */
  wizardJointRm: (id, name) => raw(`/api/wizard/${encodeURIComponent(id)}/joint/rm`, json({ name })),

  /** 重命名关节 */
  wizardJointRename: (id, old, name) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/joint/rename`, json({ old, new: name })),

  /** 修改父级 */
  wizardJointParent: (id, name, parent) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/joint/parent`, json({ name, parent })),

  /** 一键镜像对称肢 */
  wizardMirror: (id, source, to_prefix = null) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/limb/mirror`, json({ source, to_prefix })),

  /** 新增链 */
  wizardChainAdd: (id, name, joints) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/chain/add`, json({ name, joints })),

  /** 删除链 */
  wizardChainRm: (id, name) => raw(`/api/wizard/${encodeURIComponent(id)}/chain/rm`, json({ name })),

  /** 设关节默认姿态 */
  wizardPoseSet: (id, name, pos) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/pose/set`, json({ name, pos })),

  /** 整体写入默认姿态坐标（保存按钮统一提交） */
  wizardPoseApply: (id, positions) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/apply_pose`, json({ positions })),

  /** 定义/更新物种级坐标参数（引用名+label，暴露给预设） */
  wizardCoordParam: (id, name, spec = {}) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/coord/param`, json({ name, ...spec })),

  /** 整体写入坐标（含表达式）与坐标参数定义（保存按钮统一提交） */
  wizardCoordApply: (id, positions, params) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/coord/apply`, json({ positions, params })),

  /** 配置关节某轴坐标为表达式（数值=常量；{"param":..}/{"neg":..}=计算参数引用） */
  wizardCoordExpr: (id, joint, axis, expr) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/coord/expr`, json({ joint, axis, expr })),

  /** 对称参数提取：扫描对称对，相同/相反值提取为共享参数 */
  wizardCoordExtract: (id, prefix = 'sym') =>
    raw(`/api/wizard/${encodeURIComponent(id)}/coord/extract`, json({ prefix })),

  /** 姿态快速操作：旋转（axis x/y/z，joint 空=整体） */
  wizardRotate: (id, { axis = 'z', angle = 90, joint = null } = {}) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/rotate`, json({ axis, angle, joint })),

  /** 姿态快速操作：平移（joint 空=整体） */
  wizardTranslate: (id, { dx = 0, dy = 0, dz = 0, joint = null } = {}) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/translate`, json({ dx, dy, dz, joint })),

  /** 画布/地面 */
  wizardCanvas: (id, { width = null, height = null, floor_y = null } = {}) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/canvas`, json({ width, height, floor_y })),

  /** 新增体型参数链 */
  wizardParamAdd: (id, name, joints, { anchor = null, label = null } = {}) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/param/add`, json({ name, joints, anchor, label })),

  /** 完成向导（落盘 skeleton/default/preset_schema） */
  wizardCommit: (id) => raw(`/api/wizard/${encodeURIComponent(id)}/commit`, json({})),

  /** 放弃草稿 */
  wizardDiscard: (id) => raw(`/api/wizard/${encodeURIComponent(id)}/discard`, json({})),

  /** 草稿派生文件（skeleton/default），高级 JSON 模式查看 */
  wizardFiles: (id) => raw(`/api/wizard/${encodeURIComponent(id)}/files`),

  /** 高级 JSON 模式保存（重建草稿，普通模式同步） */
  wizardSaveFiles: (id, skeleton, defaultData) =>
    raw(`/api/wizard/${encodeURIComponent(id)}/files`, json({ skeleton, default: defaultData })),

  // -- 3D 预览（3D 坐标 + 投影；基于物种默认参数） --

  /** 3D 骨架：任意角度(yaw/pitch) + 距离(透视) 渲染 PNG（species_id 路径） */
  renderSkeleton3d: (id, qs) => raw(`/api/skeleton3d/${encodeURIComponent(id)}?${qs}`),

  /** 骨架 3D 数据（WebGL 实时渲染用）：应用体型后的 joints/bones/center */
  skeleton3dData: (id, qs = 'data=1') => raw(`/api/skeleton3d/${encodeURIComponent(id)}?${qs}`),

  /** 3D 动作：任意角度 + 距离 单帧渲染 PNG（动作按 species 获取与渲染） */
  renderMotion3d: (id, qs) => raw(`/api/motion3d/${encodeURIComponent(id)}?${qs}`),
  motion3dData: (id, qs = '') => raw(`/api/motion3d/${encodeURIComponent(id)}?${qs}&data=1`),

  /** 实时求动作某一帧（逐帧编辑预览；动作数据直接传未保存的编辑） */
  motion3dLive: (action, species, index = 0) =>
    raw('/api/motion3d/live', json({ action, species, index })),

  /** 蒙皮：网格 + 动作每帧变形顶点（WebGL 蒙皮预览，LBS 后端计算） */
  skin3dData: (id, qs = '') => raw(`/api/skin3d/${encodeURIComponent(id)}?${qs}`),

  // -- 预设 Preset（独立入口：预设 = 物种实例，调整体型 + 动作幅度） --

  /** 预设列表 @returns {Promise<{presets: import('../../models').PresetSummary[]}>} */
  presets: () => raw('/api/presets'),

  /** 预设详情（预设值 + schema_info） @returns {Promise<import('../../models').PresetDetail>} */
  presetDetail: (id) => raw(`/api/presets/${encodeURIComponent(id)}`),

  /** 新建预设空白表单（含 schema） */
  presetNew: (speciesId) => raw(`/api/presets/new?species=${encodeURIComponent(speciesId)}`),

  /** 创建预设 */
  createPreset: (data) => raw('/api/presets', json(data)),

  /** 更新预设 */
  updatePreset: (id, data) => raw(`/api/presets/${encodeURIComponent(id)}`, json_put(data)),

  /** 删除预设 */
  deletePreset: (id) => raw(`/api/presets/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  /** 预设渲染：骨架/动作（应用体型 + 动作参数） */
  preset3dRender: (id, qs) => raw(`/api/preset3d/${encodeURIComponent(id)}?${qs}`),

  /** 预设实时渲染（live：未保存的 body/actions 预览） */
  preset3dLive: (qs) => raw(`/api/preset3d/live?${qs}`),

  // -- 皮肤 Skin（独立入口：皮肤 = 基于预设的外观实例，皮肤参数 + 材质） --

  /** 皮肤列表 @returns {Promise<{skins: import('../../models').SkinSummary[]}>} */
  skins: () => raw('/api/skins'),

  /** 皮肤详情（皮肤值 + schema_info） */
  skinDetail: (id) => raw(`/api/skins/${encodeURIComponent(id)}`),

  /** 新建皮肤空白表单（基于预设，含 schema） */
  skinNew: (presetId) => raw(`/api/skins/new?preset=${encodeURIComponent(presetId)}`),

  /** 创建皮肤 */
  createSkin: (data) => raw('/api/skins', json(data)),

  /** 更新皮肤 */
  updateSkin: (id, data) => raw(`/api/skins/${encodeURIComponent(id)}`, json_put(data)),

  /** 删除皮肤 */
  deleteSkin: (id) => raw(`/api/skins/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  // -- 皮肤部件（游戏皮肤式：上传画师文件 → 拆解部件 → 附着到骨架） --

  /** 添加部件 */
  skinPartAdd: (id, part) => raw(`/api/skins/${encodeURIComponent(id)}/parts`, json(part)),

  /** 更新部件（增量 patch） */
  skinPartUpdate: (id, pid, patch) => raw(`/api/skins/${encodeURIComponent(id)}/parts/${encodeURIComponent(pid)}`, json_put(patch)),

  /** 删除部件 */
  skinPartDelete: (id, pid) => raw(`/api/skins/${encodeURIComponent(id)}/parts/${encodeURIComponent(pid)}`, { method: 'DELETE' }),

  /** 上传部件网格文件（.glb/.gltf/.obj/.json，base64） */
  skinPartUploadMesh: (id, pid, filename, dataB64) =>
    raw(`/api/skins/${encodeURIComponent(id)}/parts/${encodeURIComponent(pid)}/mesh`,
        json({ filename, data_b64: dataB64 })),

  /** 上传部件贴图（field: albedo/normal/metallicRoughness，base64） */
  skinPartUploadTexture: (id, pid, field, filename, dataB64) =>
    raw(`/api/skins/${encodeURIComponent(id)}/parts/${encodeURIComponent(pid)}/texture`,
        json({ field, filename, data_b64: dataB64 })),
}
