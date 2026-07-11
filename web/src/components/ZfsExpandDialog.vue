<template>
  <el-dialog
    v-model="visible"
    :title="`扩容存储池 - ${poolName}`"
    width="620px"
    append-to-body
    :close-on-click-modal="false"
    @closed="handleClosed"
  >
    <div v-loading="loading" class="expand-body">
      <el-alert v-if="lockedType === 'mixed'" type="warning" :closable="false" show-icon class="expand-alert">
        该池已混合多种 vdev 类型，扩容类型可自选，但可能加剧不一致。
      </el-alert>

      <el-form label-width="100px" class="expand-form">
        <el-form-item label="vdev 类型">
          <el-select v-model="form.vdev_type" :disabled="lockedType !== 'mixed'" style="width:100%">
            <el-option v-for="v in vdevOptions" :key="v.value" :label="v.label" :value="v.value" />
          </el-select>
          <div v-if="lockedType !== 'mixed'" class="expand-hint">已锁定为与现有池一致（{{ vdevLabel(lockedType) }}）</div>
        </el-form-item>

        <el-form-item label="成员磁盘">
          <div style="width:100%">
            <el-alert v-if="!pvTargets.length && !loading" type="info" :closable="false">未找到可用磁盘设备。</el-alert>
            <el-checkbox-group v-model="form.device_ids">
              <el-card v-for="disk in pvTargets" :key="disk.id" shadow="never" class="pv-disk-item" style="margin-bottom:8px">
                <el-checkbox :value="disk.id" style="width:100%">
                  <div style="display:flex;justify-content:space-between;align-items:center;width:100%">
                    <span style="font-weight:500">{{ disk.display_name }}</span>
                    <span style="color:#999;font-size:12px">{{ disk.device_path }} · {{ formatBytes(disk.size) }}</span>
                  </div>
                </el-checkbox>
              </el-card>
            </el-checkbox-group>
            <div class="expand-hint" v-if="pvTargets.length">
              当前选择 {{ form.device_ids.length }} 块，{{ vdevLabel(form.vdev_type) }} 至少需要 {{ vdevMinDisks(form.vdev_type) }} 块
              <span v-if="form.device_ids.length < vdevMinDisks(form.vdev_type)" style="color:#f56c6c">（不足）</span>
            </div>
          </div>
        </el-form-item>
      </el-form>

      <el-alert type="warning" :closable="false" show-icon class="expand-alert">
        加入的磁盘上的现有数据将被清除；此操作不可撤销。
      </el-alert>
    </div>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" :disabled="!canSubmit" @click="handleSubmit">提交扩容</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { expandZFSPool, getAvailablePVTargets } from '@/api/infra'

const emit = defineEmits(['success'])

const visible = ref(false)
const loading = ref(false)
const submitting = ref(false)
const poolName = ref('')
const lockedType = ref('stripe') // 纯池=该类型；'mixed'=混合放行
const pvTargets = ref([])
const form = reactive({ vdev_type: 'stripe', device_ids: [] })

const vdevOptions = [
  { value: 'stripe', label: '条带/单盘 (stripe)' },
  { value: 'mirror', label: '镜像 (mirror)' },
  { value: 'raidz1', label: 'RAIDZ1' },
  { value: 'raidz2', label: 'RAIDZ2' },
  { value: 'raidz3', label: 'RAIDZ3' }
]
const vdevMinDisks = (v) => ({ stripe: 1, mirror: 2, raidz1: 3, raidz2: 4, raidz3: 5 }[v] || 1)
const vdevLabel = (v) => (vdevOptions.find(o => o.value === v)?.label) || v

const canSubmit = computed(() => form.device_ids.length >= vdevMinDisks(form.vdev_type))

const formatBytes = (n) => {
  if (!n) return '0B'
  const u = ['B', 'K', 'M', 'G', 'T']
  let i = 0, v = n
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v >= 100 ? 0 : 1)}${u[i]}`
}

const fetchPvTargets = async () => {
  loading.value = true
  try { const r = await getAvailablePVTargets(); pvTargets.value = r.data || [] }
  catch (e) { ElMessage.error('获取可用磁盘失败') }
  finally { loading.value = false }
}

const open = (pool, expandVdevType) => {
  poolName.value = pool || ''
  lockedType.value = expandVdevType || 'mixed'
  form.vdev_type = (lockedType.value !== 'mixed') ? lockedType.value : 'stripe'
  form.device_ids = []
  visible.value = true
  fetchPvTargets()
}

const handleSubmit = async () => {
  if (!canSubmit.value) { ElMessage.warning('磁盘数量不足'); return }
  submitting.value = true
  try {
    const res = await expandZFSPool({ pool: poolName.value, vdev_type: form.vdev_type, device_ids: form.device_ids })
    ElMessage.success(res.message || '扩容任务已完成')
    emit('success')
    visible.value = false
  } catch (e) {
    // high-risk 二次确认或业务错误由 request 拦截器处理；此处仅兜底
  } finally {
    submitting.value = false
  }
}

const handleClosed = () => { poolName.value = ''; form.device_ids = [] }
defineExpose({ open })
</script>

<style scoped>
.expand-body { display: flex; flex-direction: column; gap: 12px; }
.expand-form { margin-top: 8px; }
.expand-alert { margin: 0; }
.expand-hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px; }
.pv-disk-item { border: none; }
</style>
