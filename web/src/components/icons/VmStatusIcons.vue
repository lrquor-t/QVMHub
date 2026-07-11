<template>
  <span class="vm-status-icon" :class="`vm-status-${statusKey}`">
    <svg v-if="status === 'running'" :width="size" :height="size" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="12" cy="12" r="10" :stroke="resolvedColor" stroke-width="2" class="pulse-ring-anim" />
      <polygon points="10,7.5 17,12 10,16.5" :fill="resolvedColor" />
    </svg>

    <svg v-else-if="status === 'paused'" :width="size" :height="size" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="12" cy="12" r="10" :stroke="resolvedColor" stroke-width="2" />
      <rect x="9" y="8" width="2.2" height="8" rx="1" :fill="resolvedColor" />
      <rect x="12.8" y="8" width="2.2" height="8" rx="1" :fill="resolvedColor" />
    </svg>

    <svg v-else-if="status === 'migrating'" :width="size" :height="size" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="12" cy="12" r="10" :stroke="resolvedColor" stroke-width="2" />
      <polyline points="7,12 10,9" :stroke="resolvedColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="migrate-arrow-l" />
      <polyline points="7,12 10,15" :stroke="resolvedColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="migrate-arrow-l" />
      <line x1="7" y1="12" x2="17" y2="12" :stroke="resolvedColor" stroke-width="2" stroke-linecap="round" class="migrate-dash" />
      <polyline points="17,9 14,12 17,15" :stroke="resolvedColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="migrate-arrow-r" />
    </svg>

    <svg v-else :width="size" :height="size" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="12" cy="12" r="10" :stroke="resolvedColor" stroke-width="2" />
      <line x1="12" y1="8" x2="12" y2="14" :stroke="resolvedColor" stroke-width="2.2" stroke-linecap="round" />
    </svg>
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, required: true },
  size: { type: [Number, String], default: 24 },
  color: { type: String, default: '' }
})

const defaultColors = {
  running: '#67c23a',
  'shut off': '#909399',
  paused: '#e6a23c',
  migrating: '#f56c6c'
}

const resolvedColor = computed(() => props.color || defaultColors[props.status] || '#909399')
const statusKey = computed(() => (props.status || 'shut off').replace(' ', '-'))
</script>

<style scoped>
.vm-status-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  line-height: 0;
}

.pulse-ring-anim {
  animation: vm-svg-pulse 2s ease-out infinite;
  transform-origin: center;
}

@keyframes vm-svg-pulse {
  0% { opacity: 0.5; stroke-width: 2; }
  100% { opacity: 0; stroke-width: 7; }
}

.migrate-dash {
  animation: vm-migrate-dash-fade 1.5s ease-in-out infinite;
}

.migrate-arrow-r {
  animation: vm-migrate-slide-r 1.5s ease-in-out infinite;
}

.migrate-arrow-l {
  animation: vm-migrate-slide-l 1.5s ease-in-out infinite;
}

@keyframes vm-migrate-dash-fade {
  0%, 100% { opacity: 0.35; }
  50% { opacity: 1; }
}

@keyframes vm-migrate-slide-r {
  0%, 100% { opacity: 0.4; transform: translateX(0); }
  50% { opacity: 1; transform: translateX(1.5px); }
}

@keyframes vm-migrate-slide-l {
  0%, 100% { opacity: 0.4; transform: translateX(0); }
  50% { opacity: 1; transform: translateX(-1.5px); }
}
</style>
