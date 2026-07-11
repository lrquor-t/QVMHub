<template>
  <div class="vm-schedule-panel">
    <div class="panel-toolbar">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        class="schedule-alert"
        title="可为当前虚拟机设置一次性、每日或每周定时任务。删除虚拟机属于高风险动作，仅支持一次性任务，并会触发二次验证。"
      />
      <el-button type="primary" @click="openCreateDialog">新增定时任务</el-button>
    </div>

    <el-table :data="scheduleList" border v-loading="loading" empty-text="暂无定时任务">
      <el-table-column label="事件类型" width="120" align="center">
        <template #default="{ row }">
          <el-tag :type="row.event_type === 'power' ? 'primary' : 'warning'" effect="plain">
            {{ eventTypeText(row.event_type) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="执行动作" width="140" align="center">
        <template #default="{ row }">
          <el-tag :type="row.action === 'delete' ? 'danger' : (row.action === 'start' ? 'success' : 'info')">
            {{ actionText(row.action) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="执行计划" min-width="220">
        <template #default="{ row }">
          <div class="schedule-plan">{{ schedulePlanText(row) }}</div>
          <div class="schedule-timezone">时区：{{ row.timezone || browserTimezone }}</div>
        </template>
      </el-table-column>
      <el-table-column label="下次执行" width="180" align="center">
        <template #default="{ row }">
          {{ row.enabled ? formatDateTime(row.next_run_at, row.timezone || browserTimezone) : '已停用' }}
        </template>
      </el-table-column>
      <el-table-column label="最近执行" width="180" align="center">
        <template #default="{ row }">
          {{ formatDateTime(row.last_triggered_at, row.timezone || browserTimezone) }}
        </template>
      </el-table-column>
      <el-table-column label="最近结果" min-width="220">
        <template #default="{ row }">
          <div class="schedule-result-row">
            <el-tag size="small" :type="lastStatusTagType(row.last_status)">
              {{ lastStatusText(row.last_status) }}
            </el-tag>
            <span v-if="row.last_task_id" class="task-id-text">任务 #{{ row.last_task_id }}</span>
          </div>
          <div class="schedule-result-text">{{ row.last_message || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="100" align="center">
        <template #default="{ row }">
          <el-switch
            :model-value="row.enabled"
            :loading="Boolean(switchLoadingMap[row.id])"
            @change="value => handleToggle(row, value)"
          />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160" align="center" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEditDialog(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑定时任务' : '新增定时任务'"
      append-to-body
      width="620px"
      :close-on-click-modal="false"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" label-width="110px">
        <el-form-item label="事件类型">
          <el-radio-group v-model="form.eventType" @change="handleEventTypeChange">
            <el-radio-button label="power">电源事件</el-radio-button>
            <el-radio-button label="vm">虚拟机事件</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="执行动作">
          <el-select v-model="form.action" style="width: 100%;" @change="handleActionChange">
            <el-option v-for="item in actionOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>

        <el-form-item label="执行计划">
          <el-radio-group v-model="form.scheduleType" @change="handleScheduleTypeChange">
            <el-radio-button v-for="item in scheduleTypeOptions" :key="item.value" :label="item.value">
              {{ item.label }}
            </el-radio-button>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="form.scheduleType === 'once'" label="执行时间">
          <el-date-picker
            v-model="form.runAt"
            type="datetime"
            style="width: 100%;"
            placeholder="请选择执行时间"
            format="YYYY-MM-DD HH:mm"
            :disabled-date="disabledPastDate"
          />
        </el-form-item>

        <template v-else>
          <el-form-item label="执行时刻">
            <el-time-picker
              v-model="form.timeOfDay"
              format="HH:mm"
              value-format="HH:mm"
              style="width: 100%;"
              placeholder="请选择时间"
            />
          </el-form-item>

          <el-form-item v-if="form.scheduleType === 'weekly'" label="执行日期">
            <el-checkbox-group v-model="form.weekdays">
              <el-checkbox v-for="item in weekdayOptions" :key="item.value" :label="item.value">
                {{ item.label }}
              </el-checkbox>
            </el-checkbox-group>
          </el-form-item>
        </template>

        <el-form-item label="浏览器时区">
          <div class="timezone-text">{{ browserTimezone }}</div>
        </el-form-item>

        <el-form-item label="状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>

        <el-alert
          v-if="form.action === 'delete'"
          type="warning"
          :closable="false"
          show-icon
          title="删除虚拟机会走异步任务队列执行。任务触发后会自动清理该虚拟机关联的定时任务。"
        />
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createVmSchedule,
  deleteVmSchedule,
  getVmSchedules,
  updateVmSchedule
} from '@/api/vm'

const props = defineProps({
  vmName: {
    type: String,
    required: true
  }
})

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editingId = ref(0)
const formRef = ref(null)
const scheduleList = ref([])
const switchLoadingMap = reactive({})

const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

const weekdayOptions = [
  { value: 1, label: '周一' },
  { value: 2, label: '周二' },
  { value: 3, label: '周三' },
  { value: 4, label: '周四' },
  { value: 5, label: '周五' },
  { value: 6, label: '周六' },
  { value: 7, label: '周日' }
]

const form = reactive({
  eventType: 'power',
  action: 'start',
  scheduleType: 'once',
  runAt: null,
  timeOfDay: '08:00',
  weekdays: [1, 2, 3, 4, 5],
  enabled: true
})

const actionOptions = computed(() => {
  if (form.eventType === 'vm') {
    return [{ value: 'delete', label: '删除此虚拟机' }]
  }
  return [
    { value: 'start', label: '定时开机' },
    { value: 'shutdown', label: '定时关机' }
  ]
})

const scheduleTypeOptions = computed(() => {
  if (form.action === 'delete') {
    return [{ value: 'once', label: '一次性' }]
  }
  return [
    { value: 'once', label: '一次性' },
    { value: 'daily', label: '每天' },
    { value: 'weekly', label: '每周' }
  ]
})

const resetForm = () => {
  editingId.value = 0
  form.eventType = 'power'
  form.action = 'start'
  form.scheduleType = 'once'
  form.runAt = null
  form.timeOfDay = '08:00'
  form.weekdays = [1, 2, 3, 4, 5]
  form.enabled = true
}

const eventTypeText = (value) => {
  return value === 'vm' ? '虚拟机事件' : '电源事件'
}

const actionText = (value) => {
  const map = {
    start: '开机',
    shutdown: '关机',
    delete: '删除虚拟机'
  }
  return map[value] || value
}

const lastStatusText = (value) => {
  const map = {
    pending: '等待中',
    running: '执行中',
    success: '执行成功',
    failed: '执行失败'
  }
  return map[value] || '未执行'
}

const lastStatusTagType = (value) => {
  const map = {
    pending: 'info',
    running: 'primary',
    success: 'success',
    failed: 'danger'
  }
  return map[value] || 'info'
}

const formatDateTime = (value, tz) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  const options = {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  }
  // 使用定时任务配置的时区格式化，确保与用户预期一致
  if (tz) {
    try {
      return new Intl.DateTimeFormat('zh-CN', { ...options, timeZone: tz }).format(date)
    } catch {
      // 时区不可用时回退到浏览器时区
    }
  }
  return date.toLocaleString('zh-CN', options)
}

const formatWeekdays = (values = []) => {
  const labels = weekdayOptions
    .filter(item => Array.isArray(values) && values.includes(item.value))
    .map(item => item.label)
  return labels.join('、')
}

const schedulePlanText = (row) => {
  const tz = row.timezone || browserTimezone
  if (row.schedule_type === 'once') {
    return `一次性：${formatDateTime(row.run_at, tz)}`
  }
  if (row.schedule_type === 'daily') {
    return `每天 ${row.time_of_day || '--:--'}`
  }
  return `每周 ${formatWeekdays(row.weekdays)} ${row.time_of_day || '--:--'}`
}

const fetchSchedules = async () => {
  if (!props.vmName) return
  loading.value = true
  try {
    const res = await getVmSchedules(props.vmName)
    scheduleList.value = Array.isArray(res.data) ? res.data : []
  } finally {
    loading.value = false
  }
}

const normalizeFormForEvent = () => {
  if (form.eventType === 'vm') {
    form.action = 'delete'
    form.scheduleType = 'once'
  } else {
    if (!['start', 'shutdown'].includes(form.action)) {
      form.action = 'start'
    }
    if (!['once', 'daily', 'weekly'].includes(form.scheduleType)) {
      form.scheduleType = 'once'
    }
  }
}

const handleEventTypeChange = () => {
  normalizeFormForEvent()
}

const handleActionChange = () => {
  if (form.action === 'delete') {
    form.eventType = 'vm'
    form.scheduleType = 'once'
  } else {
    form.eventType = 'power'
  }
}

const handleScheduleTypeChange = () => {
  if (form.scheduleType !== 'weekly') {
    form.weekdays = [1, 2, 3, 4, 5]
  }
}

const openCreateDialog = () => {
  resetForm()
  dialogVisible.value = true
}

const openEditDialog = (row) => {
  editingId.value = row.id
  form.eventType = row.event_type || 'power'
  form.action = row.action || 'start'
  form.scheduleType = row.schedule_type || 'once'
  form.runAt = row.run_at ? new Date(row.run_at) : null
  form.timeOfDay = row.time_of_day || '08:00'
  form.weekdays = Array.isArray(row.weekdays) && row.weekdays.length ? [...row.weekdays] : [1, 2, 3, 4, 5]
  form.enabled = row.enabled !== false
  normalizeFormForEvent()
  dialogVisible.value = true
}

const buildPayload = (override = {}) => {
  return {
    event_type: override.eventType || form.eventType,
    action: override.action || form.action,
    schedule_type: override.scheduleType || form.scheduleType,
    run_at: (override.runAt ?? form.runAt) ? new Date(override.runAt ?? form.runAt).toISOString() : '',
    timezone: browserTimezone,
    time_of_day: override.timeOfDay || form.timeOfDay || '',
    weekdays: override.weekdays || (form.scheduleType === 'weekly' ? form.weekdays : []),
    enabled: override.enabled ?? form.enabled
  }
}

// 一次性任务禁止选择今天之前的日期（与后端 schedule.go「执行时间必须晚于当前时间」一致）
const disabledPastDate = (date) => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  return date.getTime() < today.getTime()
}

const validateForm = () => {
  if (form.scheduleType === 'once') {
    if (!form.runAt) {
      ElMessage.warning('请选择执行时间')
      return false
    }
    if (new Date(form.runAt).getTime() <= Date.now()) {
      ElMessage.warning('执行时间必须晚于当前时间')
      return false
    }
  }
  if (form.scheduleType !== 'once' && !form.timeOfDay) {
    ElMessage.warning('请选择执行时刻')
    return false
  }
  if (form.scheduleType === 'weekly' && (!Array.isArray(form.weekdays) || form.weekdays.length === 0)) {
    ElMessage.warning('请选择每周执行日期')
    return false
  }
  return true
}

const handleSubmit = async () => {
  if (!validateForm()) return
  submitting.value = true
  try {
    const payload = buildPayload()
    if (editingId.value) {
      await updateVmSchedule(props.vmName, editingId.value, payload)
      ElMessage.success('定时任务已更新')
    } else {
      await createVmSchedule(props.vmName, payload)
      ElMessage.success('定时任务已创建')
    }
    dialogVisible.value = false
    await fetchSchedules()
  } finally {
    submitting.value = false
  }
}

const buildPayloadFromRow = (row, enabled) => {
  return {
    event_type: row.event_type,
    action: row.action,
    schedule_type: row.schedule_type,
    run_at: row.run_at || '',
    timezone: row.timezone || browserTimezone,
    time_of_day: row.time_of_day || '',
    weekdays: Array.isArray(row.weekdays) ? row.weekdays : [],
    enabled
  }
}

const handleToggle = async (row, enabled) => {
  switchLoadingMap[row.id] = true
  try {
    await updateVmSchedule(props.vmName, row.id, buildPayloadFromRow(row, enabled))
    row.enabled = enabled
    if (!enabled) {
      row.next_run_at = null
    }
    ElMessage.success(enabled ? '定时任务已启用' : '定时任务已停用')
    await fetchSchedules()
  } finally {
    switchLoadingMap[row.id] = false
  }
}

const handleDelete = async (row) => {
  await ElMessageBox.confirm(`确定删除“${schedulePlanText(row)} / ${actionText(row.action)}”这条定时任务吗？`, '删除定时任务', {
    type: 'warning'
  })
  await deleteVmSchedule(props.vmName, row.id)
  ElMessage.success('定时任务已删除')
  await fetchSchedules()
}

watch(() => props.vmName, () => {
  fetchSchedules()
}, { immediate: true })

onMounted(() => {
  normalizeFormForEvent()
})
</script>

<style scoped>
.vm-schedule-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.panel-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.schedule-alert {
  flex: 1;
}

.schedule-plan {
  color: #303133;
  font-weight: 600;
}

.schedule-timezone {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
}

.schedule-result-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.schedule-result-text {
  margin-top: 6px;
  color: #606266;
  line-height: 1.5;
}

.task-id-text {
  color: #909399;
  font-size: 12px;
}

.timezone-text {
  color: #606266;
}

@media (max-width: 768px) {
  .panel-toolbar {
    flex-direction: column;
  }
}
</style>
