<template>
  <div class="lxc-monitor-panel">
    <el-alert
      v-if="status && !isRunning"
      class="lxc-monitor-notice"
      type="info"
      :closable="false"
      show-icon
      title="容器当前未运行，展示的是历史监控数据"
    />
    <ResourceCharts
      type="lxc"
      :name="name"
      :status="status"
      default-mode="history"
    />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import ResourceCharts from './ResourceCharts.vue'

const props = defineProps({
  name: { type: String, required: true },
  status: { type: String, default: '' }
})

// 停机遮罩：LXCCache.Status 为大写（RUNNING/STOPPED/...）
const isRunning = computed(() => (props.status || '').toLowerCase() === 'running')
</script>

<style scoped>
.lxc-monitor-panel {
  padding: 4px 2px;
}
.lxc-monitor-notice {
  margin-bottom: 12px;
}
</style>
