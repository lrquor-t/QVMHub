<template>
  <div class="vm-monitor-panel">
    <div class="monitor-main" :class="{ 'content-blurred': showIntro }">
      <el-alert
        v-if="vmStatus === 'paused'"
        type="warning"
        :closable="false"
        style="margin-bottom: 16px;"
      >
        <template #title>
          当前虚拟机处于暂停状态。若启用了“虚拟机启动时自动冻结CPU（使用监视器命令c可继续启动过程）”，可直接执行 `c` 继续启动。
        </template>
      </el-alert>

      <el-alert
        type="info"
        :closable="false"
        style="margin-bottom: 16px;"
      >
        <template #title>
          {{ permissionHintText }}
        </template>
      </el-alert>

      <div class="monitor-toolbar">
        <div class="monitor-status-group">
          <el-tag :type="monitorStateTagType" size="large">{{ monitorStatus.domain_state || vmStatus || '-' }}</el-tag>
          <el-tag v-if="monitorStatus.monitor_status" type="warning" size="large">{{ monitorStatus.monitor_status }}</el-tag>
          <el-tag v-if="monitorStatus.monitor_available" type="success" size="large">监视器可用</el-tag>
          <el-tag v-else type="info" size="large">监视器不可用</el-tag>
        </div>
        <div class="monitor-actions">
          <el-button :loading="loading" @click="fetchStatus">刷新状态</el-button>
          <el-button
            type="primary"
            :disabled="!canContinue"
            :loading="executingQuick === 'c'"
            @click="runQuickCommand('c')"
          >
            继续启动（c）
          </el-button>
          <el-button
            type="warning"
            :disabled="!canStop"
            :loading="executingQuick === 'stop'"
            @click="runQuickCommand('stop')"
          >
            暂停（stop）
          </el-button>
        </div>
      </div>

      <el-row :gutter="16" class="monitor-grid">
        <el-col :span="10">
          <el-card shadow="never" class="monitor-card">
            <template #header>
              <div class="card-header">快捷命令</div>
            </template>
            <div class="quick-command-group">
              <el-button
                v-for="item in quickCommands"
                :key="item.value"
                class="quick-command-btn"
                :loading="executingQuick === item.value"
                @click="runQuickCommand(item.value)"
              >
                {{ item.label }}
              </el-button>
            </div>
            <el-divider>操作命令</el-divider>
            <div class="quick-command-group">
              <el-button
                v-for="item in operationCommands"
                :key="item.value"
                class="quick-command-btn"
                :loading="executingQuick === item.value"
                @click="runQuickCommand(item.value)"
              >
                {{ item.label }}
              </el-button>
            </div>
          </el-card>
        </el-col>
        <el-col :span="14">
          <el-card shadow="never" class="monitor-card">
            <template #header>
              <div class="card-header">执行监视器命令</div>
            </template>
            <div class="command-editor">
              <el-input
                v-model="customCommand"
                placeholder="例如：info status / system_reset / nmi / sendkey ctrl-alt-f1"
                clearable
                @keyup.enter="runCustomCommand"
              />
              <el-button type="primary" :loading="executingCustom" @click="runCustomCommand">执行</el-button>
            </div>
            <div class="command-hint">当前账号可使用：`c`、`stop`、`help`、`nmi`、`info ...`、`system_reset`、`system_powerdown`、`system_wakeup`、`sendkey ...`</div>
          </el-card>
        </el-col>
      </el-row>

      <el-card shadow="never" class="monitor-card output-card">
        <template #header>
          <div class="card-header">输出结果</div>
        </template>
        <el-empty v-if="!lastOutput" description="尚未执行监视器命令" />
        <template v-else>
          <div class="output-meta">
            <el-tag type="info">最后命令：{{ lastCommand || '-' }}</el-tag>
          </div>
          <pre class="monitor-output">{{ lastOutput }}</pre>
        </template>
      </el-card>
    </div>

    <div v-if="showIntro" class="monitor-intro-mask">
      <el-card shadow="always" class="monitor-intro-card">
        <template #header>
          <div class="card-header">首次使用说明</div>
        </template>
        <div class="intro-content">
          <p>开发者监视器用于执行 QEMU Monitor 调试命令，可配合“启动时冻结 CPU”观察虚拟机早期启动过程。</p>
          <p class="intro-warning">若您不是开发者或专业人士，请不要使用此功能，避免影响业务。</p>
          <p>当前仅开放只会作用于本虚拟机的安全命令，不开放会修改宿主机网络、磁盘链、快照链或调试暴露面的命令。请不要在业务高峰期随意暂停或重置虚拟机。</p>
          <div class="intro-actions">
            <el-button @click="dismissIntro">我已了解</el-button>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { executeVMMonitorCommand, getVMMonitorStatus } from '@/api/vm'
import { useUserStore } from '@/store/user'

const props = defineProps({
  vmName: { type: String, required: true },
  vmStatus: { type: String, default: '' }
})

const userStore = useUserStore()
const loading = ref(false)
const executingQuick = ref('')
const executingCustom = ref(false)
const customCommand = ref('')
const lastCommand = ref('')
const lastOutput = ref('')
const showIntro = ref(false)
const monitorStatus = ref({
  domain_state: '',
  monitor_available: false,
  monitor_status: '',
  raw_output: ''
})

const introStorageKey = computed(() => `vm-monitor-intro-seen:${userStore.username || 'default'}`)
const permissionHintText = computed(() => {
  return '当前账号可使用仅作用于本虚拟机的安全命令：`c`、`stop`、`help`、`nmi`、`info ...`、`system_reset`、`system_powerdown`、`system_wakeup`、`sendkey ...`。若您不是开发者或专业人士，请不要使用此功能，避免影响业务。'
})
const quickCommands = computed(() => {
  return [
    { label: 'help', value: 'help' },
    { label: 'info status', value: 'info status' },
    { label: 'info cpus', value: 'info cpus' },
    { label: 'info registers', value: 'info registers' },
    { label: 'info block', value: 'info block' },
    { label: 'info pci', value: 'info pci' },
    { label: 'info qtree', value: 'info qtree' }
  ]
})
const operationCommands = computed(() => [
  { label: '请求关机', value: 'system_powerdown' },
  { label: '唤醒虚拟机', value: 'system_wakeup' },
  { label: '重置虚拟机', value: 'system_reset' },
  { label: '发送 NMI', value: 'nmi' },
  { label: '发送 Ctrl+Alt+Delete', value: 'sendkey ctrl-alt-delete' }
])

const currentDomainState = computed(() => (monitorStatus.value.domain_state || props.vmStatus || '').toLowerCase())
const canContinue = computed(() => currentDomainState.value.startsWith('paused'))
const canStop = computed(() => currentDomainState.value.startsWith('running'))
const monitorStateTagType = computed(() => {
  if (currentDomainState.value.startsWith('running')) return 'success'
  if (currentDomainState.value.startsWith('paused')) return 'warning'
  return 'info'
})

const fetchStatus = async () => {
  loading.value = true
  try {
    const res = await getVMMonitorStatus(props.vmName)
    monitorStatus.value = res.data || monitorStatus.value
  } catch (error) {
    console.error('获取监视器状态失败', error)
  } finally {
    loading.value = false
  }
}

const dismissIntro = () => {
  showIntro.value = false
  localStorage.setItem(introStorageKey.value, '1')
}

const initIntro = () => {
  showIntro.value = localStorage.getItem(introStorageKey.value) !== '1'
}

const commandConfirmMessages = {
  stop: '确定要发送 stop 吗？这会暂停当前虚拟机，业务会立即停止响应。',
  nmi: '确定要发送 NMI 吗？这通常用于内核或系统异常调试。',
  system_reset: '确定要发送 system_reset 吗？这会立即重置虚拟机，效果类似硬重启。',
  system_powerdown: '确定要发送 system_powerdown 吗？这会向来宾系统发出关机请求。',
  system_wakeup: '确定要发送 system_wakeup 吗？这会尝试唤醒处于挂起状态的虚拟机。',
  'sendkey ctrl-alt-delete': '确定要向虚拟机发送 Ctrl+Alt+Delete 吗？'
}

const executeCommand = async (command, mode = 'custom') => {
  if (!command) {
    ElMessage.warning('请输入监视器命令')
    return
  }
  const normalizedCommand = command.trim().toLowerCase()
  let confirmMessage = commandConfirmMessages[normalizedCommand]
  if (!confirmMessage && normalizedCommand.startsWith('sendkey ')) {
    confirmMessage = '确定要发送该组按键到虚拟机吗？请确认这只会影响当前虚拟机内的输入。'
  }
  if (confirmMessage) {
    try {
      await ElMessageBox.confirm(confirmMessage, '操作确认', { type: 'warning' })
    } catch {
      return
    }
  }
  if (mode === 'quick') {
    executingQuick.value = command
  } else {
    executingCustom.value = true
  }
  try {
    const res = await executeVMMonitorCommand(props.vmName, command)
    const data = res.data || {}
    lastCommand.value = data.command || command
    lastOutput.value = data.output || ''
    if (data.status) {
      monitorStatus.value = data.status
    } else {
      await fetchStatus()
    }
    ElMessage.success(`监视器命令 ${lastCommand.value} 已执行`)
    if (mode === 'custom') {
      customCommand.value = ''
    }
  } catch (error) {
    console.error('执行监视器命令失败', error)
  } finally {
    if (mode === 'quick') {
      executingQuick.value = ''
    } else {
      executingCustom.value = false
    }
  }
}

const runQuickCommand = (command) => executeCommand(command, 'quick')
const runCustomCommand = () => executeCommand(customCommand.value, 'custom')

watch(() => props.vmName, () => {
  lastCommand.value = ''
  lastOutput.value = ''
  customCommand.value = ''
  initIntro()
  fetchStatus()
})

watch(() => props.vmStatus, () => {
  fetchStatus()
})

onMounted(() => {
  initIntro()
  fetchStatus()
})
</script>

<style scoped>
.vm-monitor-panel {
  position: relative;
}

.monitor-main {
  transition: filter 0.2s ease, opacity 0.2s ease;
}

.content-blurred {
  filter: blur(8px);
  opacity: 0.35;
  pointer-events: none;
  user-select: none;
}

.monitor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.monitor-status-group {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.monitor-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.monitor-grid {
  margin-bottom: 16px;
}

.monitor-card {
  border-radius: 8px;
}

.card-header {
  font-weight: 600;
  color: #303133;
}

.quick-command-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.quick-command-btn {
  margin-left: 0 !important;
}

.command-editor {
  display: flex;
  gap: 12px;
}

.command-editor :deep(.el-input) {
  flex: 1;
}

.command-hint {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
}

.output-card {
  min-height: 220px;
}

.output-meta {
  margin-bottom: 12px;
}

.monitor-output {
  margin: 0;
  padding: 16px;
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: Consolas, Monaco, monospace;
}

.monitor-intro-mask {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.monitor-intro-card {
  width: min(640px, 100%);
  border-radius: 12px;
}

.intro-content {
  color: #606266;
  line-height: 1.8;
}

.intro-content p {
  margin: 0 0 12px;
}

.intro-warning {
  color: #f56c6c;
  font-weight: 600;
}

.intro-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}

@media (max-width: 768px) {
  .monitor-toolbar {
    flex-direction: column;
    gap: 10px;
  }

  .command-editor {
    flex-direction: column;
    gap: 8px;
  }

  .command-editor :deep(.el-input) {
    width: 100%;
  }

  .output-card {
    min-height: 160px;
  }

  .monitor-output {
    font-size: 11px;
    padding: 12px;
  }

  .intro-content {
    font-size: 13px;
  }
}
</style>
