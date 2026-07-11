<template>
  <el-drawer
    v-model="visible"
    :title="currentName ? `管理 · ${currentName}` : '管理'"
    :size="drawerWidth + 'px'"
    append-to-body
    @closed="onClosed"
  >
    <!-- Teleport 到 body：避开 el-drawer 面板上的 transform，让 fixed 把手真正相对视口定位 -->
    <Teleport to="body">
      <div
        v-if="visible"
        class="resize-handle"
        title="拖动调节宽度（双击恢复默认）"
        :style="{ left: handleLeftPx + 'px' }"
        @mousedown="startResize"
        @dblclick="resetWidth"
      ><span class="resize-handle__grip"></span></div>
    </Teleport>
    <div class="drawer-body">
      <!-- 容器概览 hero -->
      <div v-if="currentName" class="container-hero">
        <el-icon class="hero-icon"><Monitor /></el-icon>
        <div class="hero-info">
          <div class="hero-name">{{ currentName }}</div>
          <div class="hero-meta">
            <el-tag :type="heroStatusType" size="small" effect="light">{{ heroStatusText }}</el-tag>
            <span class="hero-meta-item">{{ currentBacking || 'dir' }} backing</span>
            <span v-if="currentConfig.cpu_shares" class="hero-meta-item">{{ currentConfig.cpu_shares }} cpu</span>
            <span v-if="currentConfig.memory_mb" class="hero-meta-item">{{ currentConfig.memory_mb }}MB</span>
          </div>
        </div>
      </div>

      <el-tabs v-model="activeTab" class="lxc-tabs">
        <el-tab-pane name="snapshot">
          <template #label>
            <span class="lxc-tab-label"><el-icon><Camera /></el-icon> 快照</span>
          </template>
          <LxcSnapshotPanel
            v-if="visible && currentName && activeTab === 'snapshot'"
            :name="currentName"
            :status="currentStatus"
            :backing="currentBacking"
          />
        </el-tab-pane>
        <el-tab-pane name="config" lazy>
          <template #label>
            <span class="lxc-tab-label"><el-icon><Setting /></el-icon> 配置</span>
          </template>
          <LxcConfigPanel
            v-if="visible && currentName"
            :name="currentName"
            :backing="currentBacking"
            :initial-config="currentConfig"
            @saved="onConfigSaved"
          />
        </el-tab-pane>
        <el-tab-pane name="network" lazy>
          <template #label>
            <span class="lxc-tab-label"><el-icon><Connection /></el-icon> 网络</span>
          </template>
          <LxcNetworkPanel
            v-if="visible && currentName"
            :name="currentName"
          />
        </el-tab-pane>
        <el-tab-pane name="monitor" lazy>
          <template #label>
            <span class="lxc-tab-label"><el-icon><TrendCharts /></el-icon> 监控</span>
          </template>
          <LxcMonitorPanel
            v-if="visible && currentName && activeTab === 'monitor'"
            :name="currentName"
            :status="currentStatus"
          />
        </el-tab-pane>
        <el-tab-pane name="schedule" lazy>
          <template #label>
            <span class="lxc-tab-label"><el-icon><AlarmClock /></el-icon> 定时任务</span>
          </template>
          <LxcSchedulePanel
            v-if="visible && currentName && activeTab === 'schedule'"
            :name="currentName"
          />
        </el-tab-pane>
      </el-tabs>
    </div>
  </el-drawer>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Camera, Setting, Monitor, Connection, TrendCharts, AlarmClock } from '@element-plus/icons-vue'
import LxcSnapshotPanel from './LxcSnapshotPanel.vue'
import LxcConfigPanel from './LxcConfigPanel.vue'
import LxcNetworkPanel from './LxcNetworkPanel.vue'
import LxcMonitorPanel from './LxcMonitorPanel.vue'
import LxcSchedulePanel from './LxcSchedulePanel.vue'

const emit = defineEmits(['refresh'])

const visible = ref(false)
const activeTab = ref('snapshot')
const currentName = ref('')
const currentStatus = ref('')
const currentBacking = ref('')
const currentConfig = ref({})

// —— 抽屉宽度可拖拽调节（localStorage 持久化，写法沿用 RecentTaskPanel）——
const STORAGE_KEY_WIDTH = 'lxc-manage-drawer-width'
const DEFAULT_WIDTH = 900
const MIN_WIDTH = 480

const viewportWidth = ref(window.innerWidth)
const drawerWidth = ref(
  localStorage.getItem(STORAGE_KEY_WIDTH)
    ? Number(localStorage.getItem(STORAGE_KEY_WIDTH))
    : DEFAULT_WIDTH
)
// 把手 fixed 钉在抽屉左边缘（= 视口宽度 − 抽屉宽度），不随内容滚动
const handleLeftPx = computed(() => viewportWidth.value - drawerWidth.value)
const clampWidth = (w) => Math.max(MIN_WIDTH, Math.min(w, viewportWidth.value))

let resizing = false
let startX = 0
let startWidth = 0

const onResize = (e) => {
  if (!resizing) return
  // 抽屉右侧吸附：宽度 = 起始宽度 + (起始 x − 当前 x)，向左拖变宽
  drawerWidth.value = clampWidth(startWidth + (startX - e.clientX))
}

const stopResize = () => {
  if (!resizing) return
  resizing = false
  document.removeEventListener('mousemove', onResize)
  document.removeEventListener('mouseup', stopResize)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  localStorage.setItem(STORAGE_KEY_WIDTH, String(drawerWidth.value))
}

const startResize = (e) => {
  e.preventDefault()
  resizing = true
  startX = e.clientX
  startWidth = drawerWidth.value
  document.addEventListener('mousemove', onResize)
  document.addEventListener('mouseup', stopResize)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

// 双击把手恢复默认宽度
const resetWidth = () => {
  drawerWidth.value = clampWidth(DEFAULT_WIDTH)
  localStorage.setItem(STORAGE_KEY_WIDTH, String(drawerWidth.value))
}

// 视口缩小时把超出宽度的抽屉收回来，并同步把手位置
const handleViewportResize = () => {
  viewportWidth.value = window.innerWidth
  if (drawerWidth.value > viewportWidth.value) {
    drawerWidth.value = viewportWidth.value
    localStorage.setItem(STORAGE_KEY_WIDTH, String(drawerWidth.value))
  }
}

onMounted(() => {
  window.addEventListener('resize', handleViewportResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleViewportResize)
  stopResize()
})

const open = (row, tab = 'snapshot') => {
  currentName.value = row.name
  currentStatus.value = row.status
  currentBacking.value = row.backing || ''
  currentConfig.value = {
    cpu_shares: row.cpu_shares,
    memory_mb: row.memory_mb,
    autostart: row.autostart,
    group_name: row.group_name,
    remark: row.remark
  }
  activeTab.value = tab
  drawerWidth.value = clampWidth(drawerWidth.value)
  visible.value = true
}

const onClosed = () => {
  currentName.value = ''
}

// 配置保存后通知父组件刷新容器列表
const onConfigSaved = () => {
  emit('refresh')
}

const heroStatusType = computed(() => {
  if (currentStatus.value === 'RUNNING') return 'success'
  if (currentStatus.value === 'FROZEN') return 'warning'
  return 'info'
})
const heroStatusText = computed(() => {
  const map = { RUNNING: '运行中', STOPPED: '已停止', FROZEN: '已冻结', STARTING: '启动中', ABORTING: '异常' }
  return map[currentStatus.value] || (currentStatus.value || '未知')
})

defineExpose({ open })
</script>

<style scoped>
/* 抽屉主体：铺页面底色，让 hero/卡片浮起来（负 margin 抵消 el-drawer__body 默认 20px padding） */
.drawer-body {
  background: var(--app-bg-page);
  margin: -20px;
  padding: 18px 24px 24px;
  min-height: 100%;
  box-sizing: border-box;
}

/* 左边缘拖拽把手：position:fixed 钉在抽屉左边缘，不随内容滚动 */
.resize-handle {
  position: fixed;
  top: 0;
  bottom: 0;
  width: 8px;
  margin-left: -4px;
  z-index: 3001;
  cursor: col-resize;
  display: flex;
  align-items: center;
  justify-content: center;
}
.resize-handle__grip {
  width: 2px;
  height: 40px;
  border-radius: 2px;
  background: var(--app-border-light);
  transition: background 0.15s, height 0.15s;
}
.resize-handle:hover .resize-handle__grip,
.resize-handle:active .resize-handle__grip {
  background: var(--el-color-primary);
  height: 60px;
}

/* 容器概览 hero */
.container-hero {
  display: flex;
  align-items: center;
  gap: 14px;
  background: var(--app-bg-card);
  border: 1px solid var(--app-border-light);
  border-radius: 12px;
  padding: 14px 18px;
  margin-bottom: 18px;
  box-shadow: var(--app-shadow-sm);
}
.hero-icon {
  font-size: 28px;
  color: var(--el-color-primary);
  flex-shrink: 0;
}
.hero-name {
  font-size: 17px;
  font-weight: 700;
  color: var(--el-text-color-primary);
  line-height: 1.3;
}
.hero-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-wrap: wrap;
}

.lxc-tab-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
/* tabs 设计感：更大标签 + 加粗激活态 + 加粗活动条 */
.lxc-tabs :deep(.el-tabs__header) {
  margin: 0 0 20px;
}
.lxc-tabs :deep(.el-tabs__nav-wrap::after) {
  background: var(--app-border-light);
}
.lxc-tabs :deep(.el-tabs__item) {
  font-size: 15px;
  height: 44px;
  padding: 0 20px;
}
.lxc-tabs :deep(.el-tabs__item.is-active) {
  font-weight: 600;
  color: var(--el-color-primary);
}
.lxc-tabs :deep(.el-tabs__active-bar) {
  height: 3px;
  border-radius: 2px;
}
</style>
