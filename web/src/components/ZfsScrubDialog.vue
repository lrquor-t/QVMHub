<template>
  <el-dialog
    v-model="visible"
    :title="`Scrub / 健康 - ${poolName}`"
    width="640px"
    append-to-body
    :close-on-click-modal="false"
    @closed="handleClosed"
  >
    <div v-loading="loading" class="zfs-scrub-body">
      <!-- 健康度 + scrub 状态 -->
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="池健康">
          <el-tag :type="healthTagType" size="small">{{ status.health || '-' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="Scrub 状态">{{ scrubStateText }}</el-descriptions-item>
        <el-descriptions-item label="错误计数" :span="2">
          READ {{ status.read_err }} / WRITE {{ status.write_err }} / CKSUM {{ status.checksum_err }}
          <el-button
            v-if="(status.checksum_err || 0) > 0 || (status.read_err || 0) > 0 || (status.write_err || 0) > 0"
            link type="warning" size="small" :loading="clearing" @click="handleClear"
          >清除错误</el-button>
        </el-descriptions-item>
      </el-descriptions>

      <!-- 进度（运行中） -->
      <div v-if="status.scrub_state === 'running'" class="scrub-progress">
        <el-progress :percentage="progressInt" status="warning" />
        <div class="scrub-meta">
          <span>已扫描 {{ formatBytes(status.scanned) }} / {{ formatBytes(status.total) }}</span>
          <span v-if="status.time_remaining">剩余 {{ status.time_remaining }}</span>
          <span v-else>估算剩余中…</span>
        </div>
      </div>

      <!-- 上次 scrub（完成/取消） -->
      <el-alert
        v-if="status.scrub_state === 'finished'"
        :title="`上次 scrub：用时 ${status.duration || '-'}，发现 ${status.errors_found} 个错误（${status.finished_at || '-'}）`"
        :type="status.errors_found > 0 ? 'error' : 'success'"
        :closable="false" show-icon class="scrub-alert"
      />
      <el-alert
        v-if="status.scrub_state === 'canceled'"
        title="上次 scrub 已取消"
        type="warning" :closable="false" show-icon class="scrub-alert"
      />
      <el-alert
        v-if="status.scrub_state === 'none'"
        title="该池从未执行过 scrub"
        type="info" :closable="false" show-icon class="scrub-alert"
      />

      <!-- 错误清单 -->
      <div class="scrub-errors">
        <div class="scrub-errors-head">
          <span>永久错误文件清单</span>
          <el-button link type="primary" size="small" :loading="loadingErrors" @click="loadErrors">查看</el-button>
        </div>
        <div v-if="errorsLoaded" class="scrub-errors-body">
          <template v-if="errors.files && errors.files.length">
            <div v-for="f in errors.files" :key="f" class="mono-text error-file">{{ f }}</div>
            <div v-if="errors.truncated" class="error-truncated">仅显示前 {{ errors.files.length }} 条，共 {{ errors.total }} 条</div>
            <div v-if="errors.timed_out" class="error-truncated">错误清单暂不可用（读取超时或权限不足），请稍后重试或登录宿主机执行 zpool status -v</div>
          </template>
          <div v-else class="error-empty">无永久错误</div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button v-if="status.scrub_state !== 'running'" type="primary" :loading="acting" @click="handleStart">启动 Scrub</el-button>
      <el-button v-if="status.scrub_state === 'running'" type="warning" :loading="acting" @click="handleStop">停止 Scrub</el-button>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getZFSScrubStatus, startZFSScrub, stopZFSScrub, clearZFSErrors, getZFSErrors } from '@/api/infra'

const visible = ref(false)
const loading = ref(false)
const acting = ref(false)
const clearing = ref(false)
const loadingErrors = ref(false)
const errorsLoaded = ref(false)
const poolName = ref('')
const status = ref({})
const errors = ref({ files: [], total: 0, truncated: false, timed_out: false })
let pollTimer = null

const healthTagType = computed(() => {
  const h = (status.value.health || '').toUpperCase()
  if (h === 'ONLINE') return 'success'
  if (h === 'DEGRADED') return 'warning'
  if (h === 'FAULTED' || h === 'SUSPENDED' || h === 'UNAVAIL') return 'danger'
  return 'info'
})

const scrubStateText = computed(() => ({
  running: '运行中', finished: '已完成', canceled: '已取消', none: '从未执行'
}[status.value.scrub_state] || status.value.scrub_state || '-'))

const progressInt = computed(() => Math.min(100, Math.round(status.value.progress_pct || 0)))

function formatBytes(n) {
  if (!n) return '0B'
  const u = ['B', 'K', 'M', 'G', 'T', 'P']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v >= 100 ? 0 : 1)}${u[i]}`
}

async function fetchStatus() {
  if (!poolName.value) return
  loading.value = true
  try {
    const res = await getZFSScrubStatus(poolName.value)
    status.value = res.data || {}
    schedulePoll()
  } finally {
    loading.value = false
  }
}

function schedulePoll() {
  stopPoll()
  if (status.value.scrub_state === 'running') {
    pollTimer = setInterval(async () => {
      try {
        const res = await getZFSScrubStatus(poolName.value, { skipErrorHandler: true })
        status.value = res.data || {}
        if (status.value.scrub_state !== 'running') {
          stopPoll()
        }
      } catch { /* 轮询失败静默，下轮重试 */ }
    }, 5000)
  }
}

function stopPoll() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

async function handleStart() {
  try {
    await ElMessageBox.confirm(`确认对存储池 ${poolName.value} 启动 scrub 数据校验？`, '启动 Scrub', { type: 'warning' })
  } catch { return }
  acting.value = true
  try {
    const res = await startZFSScrub(poolName.value)
    ElMessage.success(res.message || 'scrub 已启动')
    await fetchStatus()
  } catch (e) {
    ElMessage.error(e?.message || '启动失败')
  } finally {
    acting.value = false
  }
}

async function handleStop() {
  acting.value = true
  try {
    const res = await stopZFSScrub(poolName.value)
    ElMessage.success(res.message || 'scrub 已停止')
    await fetchStatus()
  } catch (e) {
    ElMessage.error(e?.message || '停止失败')
  } finally {
    acting.value = false
  }
}

async function handleClear() {
  clearing.value = true
  try {
    const res = await clearZFSErrors(poolName.value)
    ElMessage.success(res.message || '错误已清除')
    await fetchStatus()
  } catch (e) {
    ElMessage.error(e?.message || '清除失败')
  } finally {
    clearing.value = false
  }
}

async function loadErrors() {
  loadingErrors.value = true
  try {
    const res = await getZFSErrors(poolName.value)
    errors.value = res.data || { files: [], total: 0, truncated: false, timed_out: false }
    errorsLoaded.value = true
  } catch (e) {
    ElMessage.error(e?.message || '读取错误清单失败')
  } finally {
    loadingErrors.value = false
  }
}

function open(name) {
  poolName.value = name || ''
  status.value = {}
  errors.value = { files: [], total: 0, truncated: false, timed_out: false }
  errorsLoaded.value = false
  visible.value = true
  fetchStatus().catch(() => {})
}

function handleClosed() {
  stopPoll()
  poolName.value = ''
  status.value = {}
  errorsLoaded.value = false
}

onBeforeUnmount(stopPoll)

defineExpose({ open })
</script>

<style scoped>
.zfs-scrub-body { display: flex; flex-direction: column; gap: 14px; }
.scrub-progress { display: flex; flex-direction: column; gap: 6px; }
.scrub-meta { display: flex; gap: 18px; font-size: 12px; color: #666; }
.scrub-alert { margin: 0; }
.scrub-errors-head { display: flex; align-items: center; justify-content: space-between; font-size: 13px; }
.scrub-errors-body { max-height: 180px; overflow: auto; border: 1px solid #ebeef5; border-radius: 4px; padding: 6px 8px; }
.error-file { font-size: 12px; line-height: 1.8; word-break: break-all; }
.error-truncated { font-size: 12px; color: #e6a23c; margin-top: 6px; }
.error-empty { font-size: 12px; color: #999; }
.mono-text { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
</style>
