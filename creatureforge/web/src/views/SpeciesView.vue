<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2>🦴 物种管理</h2>
        <p class="page-desc">物种 = 骨骼拓扑 + 动作。表格 + 详情页维护：骨骼与动作各自独立页面，聚焦操作。</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="startWizard" icon="MagicStick">新建物种（向导）</el-button>
      </div>
    </div>

    <!-- 列表表格（ToB 风格，无左侧列表） -->
    <section class="list-view" v-if="!selectedSpecies && !wizardMode">
      <el-table :data="speciesList" border stripe>
        <el-table-column label="物种" min-width="200">
          <template #default="{row}">
            <div class="cell-main"><span class="cell-title">🦴 {{ row.title }}</span><span class="cell-id mono">{{ row.id }}</span></div>
          </template>
        </el-table-column>
        <el-table-column label="关节" width="80" align="center"><template #default="{row}">{{ row.joint_count }}</template></el-table-column>
        <el-table-column label="骨骼" width="80" align="center"><template #default="{row}">{{ row.bone_count }}</template></el-table-column>
        <el-table-column label="动作" width="80" align="center"><template #default="{row}">{{ (row.actions||[]).length }}</template></el-table-column>
        <el-table-column prop="description" label="描述" min-width="220" show-overflow-tooltip />
        <el-table-column label="操作" width="230" align="center">
          <template #default="{row}">
            <el-button size="small" text type="primary" @click="openSpecies(row, 'skeleton')">骨骼</el-button>
            <el-button size="small" text type="primary" @click="openSpecies(row, 'actions')">动作</el-button>
            <el-button size="small" text type="danger" @click="confirmDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!speciesList.length" class="empty-list">
        <div class="empty-icon">🦴</div>
        <p>暂无物种</p>
        <el-button type="primary" @click="startWizard">创建第一个物种</el-button>
      </div>
    </section>

    <!-- 详情：骨骼 / 动作 独立页面 -->
    <section class="detail-view" v-else-if="selectedSpecies">
      <div class="detail-head">
        <div class="crumb">
          <span class="crumb-root">物种</span><span class="crumb-sep">/</span><span class="crumb-now">{{ selectedSpecies.title }}</span>
        </div>
        <el-button size="small" @click="backToList">返回列表</el-button>
      </div>
      <el-tabs v-model="selectedTab" class="detail-tabs">
        <el-tab-pane label="🦴 骨骼" name="skeleton">
          <SpeciesSkeletonView :species-id="selectedSpecies.id" @back="backToList" @saved="loadSpecies" />
        </el-tab-pane>
        <el-tab-pane label="🎬 动作" name="actions">
          <SpeciesActionsView :species-id="selectedSpecies.id" @back="backToList" @saved="loadSpecies" />
        </el-tab-pane>
      </el-tabs>
    </section>

    <!-- 新建向导 -->
    <section class="detail-view" v-else-if="wizardMode">
      <SpeciesWizard @done="onWizardDone" @cancel="wizardMode = false" />
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api.js'
import { ElMessage, ElMessageBox } from 'element-plus'
import SpeciesWizard from './SpeciesWizard.vue'
import SpeciesSkeletonView from './SpeciesSkeletonView.vue'
import SpeciesActionsView from './SpeciesActionsView.vue'

const loading = ref(true)
const speciesList = ref([])
const selectedSpecies = ref(null)
const selectedTab = ref('skeleton')
const wizardMode = ref(false)

async function loadSpecies() {
  try {
    const r = await api.species()
    speciesList.value = r.species || []
  } catch (e) { ElMessage.error('加载物种失败: ' + e.message) }
}

function openSpecies(sp, tab) {
  selectedSpecies.value = sp
  selectedTab.value = tab
  wizardMode.value = false
}
function backToList() { selectedSpecies.value = null; wizardMode.value = false }

function startWizard() { selectedSpecies.value = null; wizardMode.value = true }
async function onWizardDone() { wizardMode.value = false; await loadSpecies() }

async function confirmDelete(sp) {
  try {
    await ElMessageBox.confirm(`确定删除物种「${sp.title}」吗？此操作不可恢复。`, '确认删除',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    await api.deleteSpecies(sp.id)
    ElMessage.success('已删除')
    if (selectedSpecies.value?.id === sp.id) selectedSpecies.value = null
    await loadSpecies()
  } catch (e) { if (e !== 'cancel') ElMessage.error(e.message || e) }
}

onMounted(loadSpecies)
</script>

<style scoped>
.page { max-width: 1280px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-header h2 { margin: 0 0 4px; }
.page-desc { color: #909399; font-size: .85rem; margin: 0; }
.header-actions { display: flex; gap: 8px; }

.list-view { background: #fff; border: 1px solid #e4e7ed; border-radius: 10px; padding: 8px; }
.cell-main { display: flex; align-items: center; gap: 8px; }
.cell-title { font-weight: 600; font-size: .9rem; }
.cell-id { color: #909399; font-size: .72rem; font-family: monospace; }
.empty-list { padding: 48px 20px; text-align: center; color: #c0c4cc; display: flex; flex-direction: column; align-items: center; gap: 8px; }
.empty-icon { font-size: 2rem; }

.detail-view { background: #fff; border: 1px solid #e4e7ed; border-radius: 10px; padding: 16px 20px; }
.detail-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.crumb { font-size: .95rem; }
.crumb-root { color: #909399; } .crumb-sep { color: #c0c4cc; } .crumb-now { font-weight: 600; color: #303133; }
.detail-tabs { margin-top: 4px; }
</style>
