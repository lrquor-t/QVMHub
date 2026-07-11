<template>
  <div class="lxc-list-container">
    <!-- 页面头 -->
    <div class="page-header-bar">
      <div class="page-header-left">
        <el-icon class="page-icon"><Monitor /></el-icon>
        <div class="page-header-text">
          <h2>LXC 容器</h2>
          <p>共 {{ total }} 个 · 运行中 {{ running }}</p>
        </div>
      </div>
      <div class="page-header-right">
        <el-input
          v-model="lxcSearchText"
          placeholder="搜索名称、模板、备注、分组..."
          clearable
          :prefix-icon="Search"
          class="header-search"
        />
        <el-switch v-model="autoRefresh" active-text="自动刷新" />
        <el-button type="success" :icon="Refresh" :loading="loading" @click="fetchData">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="lxcCreateRef?.open()">创建容器</el-button>
      </div>
    </div>

    <!-- KPI 统计 -->
    <div class="kpi-row">
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-accent" style="background:var(--el-color-primary)"></div>
        <div class="kpi-body">
          <div class="kpi-head"><el-icon><Monitor /></el-icon><span>容器总数</span></div>
          <div class="kpi-value">{{ total }}</div>
        </div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-accent" style="background:var(--el-color-success)"></div>
        <div class="kpi-body">
          <div class="kpi-head"><el-icon><VideoPlay /></el-icon><span>运行中</span></div>
          <div class="kpi-value">{{ running }}</div>
        </div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-accent" style="background:var(--el-color-info)"></div>
        <div class="kpi-body">
          <div class="kpi-head"><el-icon><VideoPause /></el-icon><span>已停止</span></div>
          <div class="kpi-value">{{ stopped }}</div>
        </div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-accent" style="background:var(--el-color-danger)"></div>
        <div class="kpi-body">
          <div class="kpi-head"><el-icon><Warning /></el-icon><span>异常</span></div>
          <div class="kpi-value">{{ abnormal }}</div>
        </div>
      </el-card>
    </div>

    <!-- 批量操作条（仅选中时出现） -->
    <div v-show="hasSelection" class="lxc-batch-bar">
      <span class="lxc-batch-info">已选 {{ selected.length }} 个容器</span>
      <el-button size="small" type="success" :loading="batchOperating" @click="handleBatchOperate('start')">开机</el-button>
      <el-button size="small" type="warning" :loading="batchOperating" @click="handleBatchOperate('stop')">关机</el-button>
      <el-button size="small" type="info" :loading="batchOperating" @click="handleBatchOperate('restart')">重启</el-button>
      <el-button size="small" type="danger" :loading="batchOperating" @click="handleBatchOperate('delete')">删除</el-button>
    </div>

    <!-- 表格 -->
    <div class="lxc-table-wrap">
      <el-table
        :data="filteredData"
        v-loading="loading"
        style="width: 100%"
        @selection-change="selected = $event"
      >
        <el-table-column type="selection" width="45" align="center" />
        <el-table-column label="名称" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="lxc-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <span class="lxc-status" :class="'is-' + statusType(row.status)">
              <span class="lxc-status-dot"></span>
              <el-tag :type="statusType(row.status)" size="small" effect="light">{{ statusText(row.status) }}</el-tag>
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="template" label="模板" width="130" show-overflow-tooltip>
          <template #default="{ row }">{{ row.template || '-' }}</template>
        </el-table-column>
        <el-table-column label="规格" width="130" align="center">
          <template #default="{ row }">
            <span class="lxc-spec">{{ row.cpu_shares || '-' }} / {{ row.memory_mb ? row.memory_mb + 'MB' : '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="IP 地址" width="140">
          <template #default="{ row }">
            <span class="lxc-ip">{{ row.cached_ip || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="分组 / 备注" min-width="190">
          <template #default="{ row }">
            <div class="lxc-remark-cell">
              <div class="lxc-remark-left">
                <el-tag v-if="row.group_name" size="small" type="warning" effect="plain">{{ row.group_name }}</el-tag>
                <span class="lxc-remark-text" :title="row.remark || ''">{{ row.remark || '-' }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right" align="center">
          <template #default="{ row }">
            <el-tooltip :content="row.status === 'RUNNING' ? '关机' : '开机'" placement="top">
              <el-button
                size="small"
                circle
                class="lxc-op-btn"
                :type="row.status === 'RUNNING' ? 'warning' : 'success'"
                :icon="row.status === 'RUNNING' ? SwitchButton : VideoPlay"
                :loading="!!operatingMap[row.name]"
                @click="operate(row, row.status === 'RUNNING' ? 'stop' : 'start')"
              />
            </el-tooltip>
            <el-tooltip :content="row.status === 'RUNNING' ? '终端' : '终端（容器未运行）'" placement="top">
              <el-button size="small" circle type="primary" plain :icon="Monitor" :disabled="row.status !== 'RUNNING'" @click="openConsole(row)" />
            </el-tooltip>
            <el-tooltip content="管理" placement="top">
              <el-button size="small" circle type="primary" :icon="Operation" @click="openManage(row)" />
            </el-tooltip>
            <el-dropdown trigger="click" style="margin-left: 12px;" @command="cmd => handleMore(cmd, row)">
              <span class="el-dropdown-link">
                <el-tooltip content="更多" placement="top">
                  <el-button size="small" circle :icon="MoreFilled" />
                </el-tooltip>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="restart">重启</el-dropdown-item>
                  <el-dropdown-item command="clone">克隆</el-dropdown-item>
                  <el-dropdown-item v-if="isAdmin" command="template" :disabled="!isStopped(row)">制作模板</el-dropdown-item>
                  <el-dropdown-item command="delete" divided class="lxc-dropdown-danger">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无容器">
            <el-button type="primary" :icon="Plus" @click="lxcCreateRef?.open()">创建第一个容器</el-button>
          </el-empty>
        </template>
      </el-table>
    </div>

    <!-- 创建向导 -->
    <LxcCreateForm ref="lxcCreateRef" @success="fetchData" />


    <!-- 管理抽屉 -->
    <LxcManageDrawer ref="manageDrawerRef" @refresh="fetchData" />

    <!-- 制作模板对话框 -->
    <LxcMakeTemplateForm ref="lxcMakeTplRef" />

    <!-- 克隆对话框 -->
    <LxcCloneForm ref="lxcCloneRef" @success="fetchData" @goto-snapshot="onGotoSnapshot" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Plus, Refresh, VideoPlay, VideoPause, Warning, SwitchButton, Monitor, MoreFilled, Operation } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import {
  getLXCList, operateLXC, deleteLXC, batchOperateLXC
} from '@/api/lxc'
import LxcManageDrawer from '@/components/LxcManageDrawer.vue'
import LxcCreateForm from '@/components/LxcCreateForm.vue'
import LxcMakeTemplateForm from '@/components/LxcMakeTemplateForm.vue'
import LxcCloneForm from '@/components/LxcCloneForm.vue'

const router = useRouter()
const userStore = useUserStore()
const isAdmin = computed(() => userStore.role === 'admin')
const isStopped = (row) => (row.status || '').toUpperCase() === 'STOPPED'
const tableData = ref([])
const loading = ref(false)
const selected = ref([])
const autoRefresh = ref(false)
const lxcSearchText = ref('')
const operatingMap = ref({})
const batchOperating = ref(false)
const manageDrawerRef = ref(null)
const lxcCreateRef = ref(null)
const lxcMakeTplRef = ref(null)
const lxcCloneRef = ref(null)
const openMakeTemplate = (row) => lxcMakeTplRef.value?.open(row)
const openClone = (row) => lxcCloneRef.value?.open(row)
const onGotoSnapshot = (row) => manageDrawerRef.value?.open(row, 'snapshot')
const openManage = (row) => manageDrawerRef.value?.open(row)
let timer = null

const hasSelection = computed(() => selected.value.length > 0)

const filteredData = computed(() => {
  if (!lxcSearchText.value) return tableData.value
  const q = lxcSearchText.value.toLowerCase()
  return tableData.value.filter(r =>
    (r.name || '').toLowerCase().includes(q) ||
    (r.template || '').toLowerCase().includes(q) ||
    (r.remark || '').toLowerCase().includes(q) ||
    (r.group_name || '').toLowerCase().includes(q)
  )
})

// KPI 统计（从列表数据派生）
const total = computed(() => tableData.value.length)
const running = computed(() => tableData.value.filter(r => r.status === 'RUNNING').length)
const stopped = computed(() => tableData.value.filter(r => r.status === 'STOPPED').length)
const abnormal = computed(() => tableData.value.filter(r => r.status !== 'RUNNING' && r.status !== 'STOPPED').length)

const statusType = (status) => {
  if (status === 'RUNNING') return 'success'
  if (status === 'FROZEN') return 'warning'
  return 'info'
}
const statusText = (status) => {
  const map = { RUNNING: '运行中', STOPPED: '已停止', FROZEN: '已冻结', STARTING: '启动中', ABORTING: '异常' }
  return map[status] || (status ? status : '已停止')
}

const fetchData = async () => {
  loading.value = true
  try {
    const res = await getLXCList()
    tableData.value = res.data || []
  } catch (e) {} finally { loading.value = false }
}

const operate = async (row, action) => {
  operatingMap.value = { ...operatingMap.value, [row.name]: true }
  try { await operateLXC(row.name, action); ElMessage.success('操作已提交'); fetchData() } catch (e) {} finally {
    operatingMap.value = { ...operatingMap.value, [row.name]: false }
  }
}
const remove = async (row) => {
  await ElMessageBox.confirm(`确认删除容器 ${row.name}？`, '删除', { type: 'warning' })
  try { await deleteLXC(row.name); ElMessage.success('删除任务已提交'); fetchData() } catch (e) {}
}
const handleBatchOperate = async (action) => {
  const names = selected.value.map(r => r.name)
  const label = { start: '开机', stop: '关机', restart: '重启', delete: '删除' }[action]
  await ElMessageBox.confirm(`对选中的 ${names.length} 个容器执行「${label}」？`, '批量操作', { type: 'warning' })
  batchOperating.value = true
  try {
    await batchOperateLXC(names, action)
    ElMessage.success('已执行')
    fetchData()
  } catch (e) {} finally { batchOperating.value = false }
}
const handleMore = async (cmd, row) => {
  if (cmd === 'restart') operate(row, 'restart')
  else if (cmd === 'delete') remove(row)
  else if (cmd === 'template') openMakeTemplate(row)
  else if (cmd === 'clone') openClone(row)
}
const openConsole = (row) => {
  // 新标签页打开终端（与 VM VNC 窗口一致；路由守卫放行，WS 鉴权由后端 LXCAccessMiddleware + token query）
  const { href } = router.resolve(`/lxc/console/${encodeURIComponent(row.name)}`)
  window.open(href, '_blank')
}


onMounted(() => { fetchData(); timer = setInterval(() => { if (autoRefresh.value) fetchData() }, 5000) })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.lxc-list-container {
  padding: 10px;
}

/* 页面头 page-header-bar */
.page-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 4px 16px;
}
.page-header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}
.page-icon {
  font-size: 22px;
  color: var(--el-color-primary);
}
.page-header-text h2 {
  margin: 0;
  font-size: 19px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--el-text-color-primary);
}
.page-header-text p {
  margin: 2px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.page-header-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
  flex-wrap: wrap;
}
.header-search {
  width: 240px;
}

/* KPI 统计卡 */
.kpi-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
  padding: 0 4px 16px;
}
.kpi-card {
  border-radius: 12px;
  border: none;
  transition: transform 0.2s var(--app-transition-fast, 0.15s), box-shadow 0.2s;
}
.kpi-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--app-shadow-lg);
}
.kpi-card :deep(.el-card__body) {
  padding: 0;
}
.kpi-accent {
  height: 3px;
  border-radius: 12px 12px 0 0;
}
.kpi-body {
  padding: 14px 16px;
}
.kpi-head {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.kpi-head .el-icon {
  font-size: 15px;
}
.kpi-value {
  font-size: 24px;
  font-weight: 800;
  line-height: 1.2;
  margin-top: 4px;
  color: var(--el-text-color-primary);
}

/* 批量操作条 */
.lxc-batch-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  margin-bottom: 12px;
  border-radius: 12px;
  background: var(--el-color-warning-light-9);
  border: 1px solid var(--el-color-warning-light-7);
  box-shadow: var(--app-shadow-md);
}
.lxc-batch-info {
  margin-right: auto;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

/* 表格容器（hover-lift） */
.lxc-table-wrap {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  background: var(--app-bg-card);
  border-radius: 12px;
  box-shadow: var(--app-shadow-sm);
  border: 1px solid var(--app-border-light);
  padding: 2px;
  transition: box-shadow 0.2s var(--app-transition-fast, 0.15s);
}
.lxc-table-wrap:hover {
  box-shadow: var(--app-shadow-lg);
}

/* 单元格样式 */
.lxc-name {
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.lxc-spec {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.lxc-ip {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  color: var(--el-text-color-regular);
}
.lxc-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.lxc-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--el-color-info);
  flex-shrink: 0;
}
.lxc-status.is-success .lxc-status-dot {
  background: var(--el-color-success);
  box-shadow: 0 0 0 3px rgba(103, 194, 58, 0.18);
}
.lxc-status.is-warning .lxc-status-dot {
  background: var(--el-color-warning);
  box-shadow: 0 0 0 3px rgba(230, 162, 60, 0.18);
}
.lxc-remark-cell {
  display: flex;
  align-items: center;
  min-width: 0;
}
.lxc-remark-left {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  min-width: 0;
}
.lxc-remark-text {
  font-size: 13px;
  color: var(--el-text-color-regular);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.lxc-op-btn {
  transition: all var(--app-transition-fast, 0.15s);
}

/* 下拉“删除”红色 */
:deep(.lxc-dropdown-danger) {
  color: var(--el-color-danger);
}

/* ===== 深色模式 ===== */
html.dark .lxc-batch-bar {
  background: rgba(230, 162, 60, 0.12);
  border-color: rgba(230, 162, 60, 0.3);
}
html.dark .lxc-status.is-success .lxc-status-dot {
  box-shadow: 0 0 0 3px rgba(103, 194, 58, 0.22);
}

/* ===== 移动端 ===== */
@media (max-width: 768px) {
  .page-header-bar {
    flex-wrap: wrap;
    gap: 10px;
  }
  .page-header-right {
    order: 3;
    width: 100%;
  }
  .header-search {
    width: 100%;
  }
  .kpi-row {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
