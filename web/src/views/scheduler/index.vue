<template>
  <div class="scheduler-page">
    <el-card class="scheduler-card" v-loading="overviewLoading">
      <template #header>
        <div class="section-header">
          <div>
            <h2>调度器概览</h2>
            <el-text type="info">当前先接入动态内存体系下的两类调度器，后续可继续扩展。</el-text>
          </div>
          <el-button type="primary" plain icon="Refresh" @click="fetchOverview" :loading="overviewLoading">刷新概览</el-button>
        </div>
      </template>

      <el-row :gutter="16" v-if="schedulerList.length">
        <el-col :xs="24" :sm="12" :lg="8" v-for="item in schedulerList" :key="item.key">
          <div class="overview-item">
            <div class="overview-top">
              <div>
                <div class="overview-title">{{ item.name }}</div>
                <div class="overview-key">{{ item.key }}</div>
              </div>
              <el-tag :type="item.enabled ? 'success' : 'info'" effect="plain">
                {{ item.enabled ? '已启用' : '已停用' }}
              </el-tag>
            </div>
            <div class="overview-group">分组：{{ item.group }}</div>
            <div class="overview-desc">{{ item.description || '暂无说明' }}</div>
            <div class="overview-time">最近事件：{{ formatTime(item.last_event_at) }}</div>
          </div>
        </el-col>
      </el-row>
      <el-empty v-else description="暂无已注册调度器" />
    </el-card>

    <el-card class="scheduler-card">
      <template #header>
        <div class="section-header">
          <div>
            <h2>调度事件</h2>
            <el-text type="info">仅记录实际发生调度动作尝试的事件，不记录普通轮询扫描。</el-text>
          </div>
          <div class="header-actions">
            <el-tag :type="sseStatusType" effect="dark">
              {{ sseStatusText }}
            </el-tag>
            <el-button type="success" icon="Refresh" @click="fetchEvents" :loading="tableLoading">刷新列表</el-button>
          </div>
        </div>
      </template>

      <div class="filter-bar">
        <el-select v-model="filters.scheduler_key" clearable placeholder="调度器" style="width: 220px;">
          <el-option v-for="item in schedulerList" :key="item.key" :label="item.name" :value="item.key" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="状态" style="width: 160px;">
          <el-option label="正在执行" value="running" />
          <el-option label="执行完毕" value="success" />
          <el-option label="执行失败" value="failed" />
        </el-select>
        <el-input v-model="filters.vm_name" clearable placeholder="虚拟机名称" style="width: 220px;" />
        <el-date-picker
          v-model="filters.range"
          type="datetimerange"
          range-separator="至"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          value-format="YYYY-MM-DD HH:mm:ss"
          style="width: 360px;"
        />
        <el-button type="primary" @click="handleSearch">查询</el-button>
        <el-button @click="handleReset">重置</el-button>
      </div>

      <el-table :data="tableData" border row-key="id" v-loading="tableLoading">
        <el-table-column label="触发时间" min-width="170" align="center">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="vm_name" label="虚拟机" min-width="160" />
        <el-table-column prop="scheduler_name" label="调度器类型" min-width="180" />
        <el-table-column label="调度状态" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="trigger_reason" label="调度原因" min-width="260" show-overflow-tooltip />
        <el-table-column label="执行结果 / 失败原因" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.error_message || row.result_message || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="完成时间" min-width="170" align="center">
          <template #default="{ row }">
            {{ formatTime(row.finished_at) }}
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-bar">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @size-change="fetchEvents"
          @current-change="fetchEvents"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref, computed } from 'vue'
import { createSchedulerEventSSE, getSchedulerEventList, getSchedulerList } from '@/api/scheduler'

const overviewLoading = ref(false)
const tableLoading = ref(false)
const sseConnected = ref('connecting')

const sseStatusType = computed(() => {
  const map = { connecting: 'warning', connected: 'success', disconnected: 'info' }
  return map[sseConnected.value] || 'info'
})

const sseStatusText = computed(() => {
  const map = { connecting: '连接中...', connected: '实时连接', disconnected: '连接断开' }
  return map[sseConnected.value] || '连接断开'
})
const schedulerList = ref([])
const tableData = ref([])
const filters = reactive({
  scheduler_key: '',
  status: '',
  vm_name: '',
  range: []
})
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

let eventSource = null

const statusText = (status) => {
  const map = {
    running: '正在执行',
    success: '执行完毕',
    failed: '执行失败'
  }
  return map[status] || status
}

const statusTagType = (status) => {
  const map = {
    running: 'primary',
    success: 'success',
    failed: 'danger'
  }
  return map[status] || 'info'
}

const formatTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

const buildParams = () => ({
  page: pagination.page,
  page_size: pagination.pageSize,
  scheduler_key: filters.scheduler_key || undefined,
  status: filters.status || undefined,
  vm_name: filters.vm_name || undefined,
  start: filters.range?.[0] || undefined,
  end: filters.range?.[1] || undefined
})

const fetchOverview = async () => {
  overviewLoading.value = true
  try {
    const res = await getSchedulerList()
    schedulerList.value = res.data || []
  } catch (err) {
    console.error(err)
  } finally {
    overviewLoading.value = false
  }
}

const fetchEvents = async () => {
  tableLoading.value = true
  try {
    const res = await getSchedulerEventList(buildParams())
    tableData.value = res.data?.list || []
    pagination.total = res.data?.total || 0
  } catch (err) {
    console.error(err)
  } finally {
    tableLoading.value = false
  }
}

const handleSearch = () => {
  pagination.page = 1
  fetchEvents()
}

const handleReset = () => {
  filters.scheduler_key = ''
  filters.status = ''
  filters.vm_name = ''
  filters.range = []
  pagination.page = 1
  fetchEvents()
}

const matchesCurrentFilters = (row) => {
  if (filters.scheduler_key && row.scheduler_key !== filters.scheduler_key) return false
  if (filters.status && row.status !== filters.status) return false
  if (filters.vm_name && !row.vm_name?.includes(filters.vm_name)) return false
  if (filters.range?.length === 2) {
    const createdAt = new Date(row.created_at).getTime()
    const start = new Date(filters.range[0]).getTime()
    const end = new Date(filters.range[1]).getTime()
    if (!Number.isNaN(createdAt) && (createdAt < start || createdAt > end)) return false
  }
  return true
}

const updateOverviewLastEvent = (event) => {
  const item = schedulerList.value.find(row => row.key === event.scheduler_key)
  if (item) {
    item.last_event_at = event.created_at
  }
}

const upsertEvent = (event) => {
  updateOverviewLastEvent(event)
  const index = tableData.value.findIndex(row => row.id === event.id)
  if (index !== -1) {
    tableData.value[index] = event
    return
  }
  if (pagination.page !== 1 || !matchesCurrentFilters(event)) {
    return
  }
  tableData.value.unshift(event)
  if (tableData.value.length > pagination.pageSize) {
    tableData.value.pop()
  }
  pagination.total += 1
}

const connectSSE = () => {
  sseConnected.value = 'connecting'
  eventSource = createSchedulerEventSSE()

  eventSource.addEventListener('connected', () => {
    sseConnected.value = 'connected'
  })

  eventSource.addEventListener('scheduler_event', (rawEvent) => {
    try {
      const payload = JSON.parse(rawEvent.data)
      if (payload?.event) {
        upsertEvent(payload.event)
      }
    } catch (err) {
      console.error('解析调度事件 SSE 失败', err)
    }
  })

  eventSource.onerror = () => {
    sseConnected.value = 'disconnected'
    if (eventSource) {
      eventSource.close()
    }
    setTimeout(connectSSE, 5000)
  }
}

const disconnectSSE = () => {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  sseConnected.value = 'disconnected'
}

onMounted(async () => {
  await Promise.all([fetchOverview(), fetchEvents()])
  connectSSE()
})

onBeforeUnmount(() => {
  disconnectSSE()
})
</script>

<style scoped>
.scheduler-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.scheduler-card {
  border-radius: 12px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.section-header h2 {
  margin: 0 0 6px;
  font-size: 18px;
  color: var(--el-text-color-primary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.overview-item {
  height: 100%;
  padding: 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: linear-gradient(180deg, var(--el-fill-color-extra-light), var(--el-bg-color));
}

.overview-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 10px;
}

.overview-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.overview-key {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  word-break: break-all;
}

.overview-group,
.overview-time,
.overview-desc {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
}

.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 16px;
}

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 768px) {
  .section-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .header-actions {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
