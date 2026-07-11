<template>
  <el-dialog
    v-model="visible"
    :title="`Dataset 属性 - ${dataset}`"
    width="560px"
    append-to-body
    :close-on-click-modal="false"
    @closed="handleClosed"
  >
    <div v-loading="loading" class="zfs-prop-body">
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="池可用">{{ formatBytes(info.pool_avail) }}</el-descriptions-item>
        <el-descriptions-item label="提示">配额可超可用空间（超分合法）</el-descriptions-item>
      </el-descriptions>

      <el-form label-width="100px" class="prop-form">
        <el-form-item label="压缩">
          <el-select v-model="form.compression" style="width:100%">
            <el-option v-for="c in ['off','lz4','zstd','gzip','lzjb']" :key="c" :label="c" :value="c" />
            <el-option label="gzip-1" value="gzip-1" />
          </el-select>
          <div class="prop-source">当前: {{ info.compression.value || '-' }}（{{ sourceText(info.compression.source) }}）</div>
        </el-form-item>
        <el-form-item label="atime">
          <el-select v-model="form.atime" style="width:100%">
            <el-option label="on" value="on" /><el-option label="off" value="off" />
          </el-select>
          <div class="prop-source">当前: {{ info.atime.value || '-' }}（{{ sourceText(info.atime.source) }}）</div>
        </el-form-item>
        <el-form-item label="quota">
          <el-input v-model="form.quota" placeholder="如 50G / 1024M / none" />
          <div class="prop-source">当前: {{ info.quota.value || '-' }}（{{ sourceText(info.quota.source) }}）</div>
        </el-form-item>
        <el-form-item label="refquota">
          <el-input v-model="form.refquota" placeholder="如 50G / 1024M / none（磁盘大小语义）" />
          <div class="prop-source">当前: {{ info.refquota.value || '-' }}（{{ sourceText(info.refquota.source) }}）</div>
        </el-form-item>
      </el-form>
    </div>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { getZFSProperty, setZFSProperty } from '@/api/infra'

const visible = ref(false)
const loading = ref(false)
const saving = ref(false)
const dataset = ref('')
const info = ref({ compression: {}, atime: {}, quota: {}, refquota: {}, pool_avail: 0 })
const form = reactive({ compression: '', atime: '', quota: '', refquota: '' })

const sourceText = (s) => {
  if (!s) return '-'
  if (s === 'local') return '本地'
  if (s === 'default') return '默认'
  if (s.startsWith('inherited from ')) return '继承自 ' + s.replace('inherited from ', '')
  if (s.startsWith('inherited')) return '继承自父级'
  return s
}
const formatBytes = (n) => {
  if (!n) return '0B'
  const u = ['B', 'K', 'M', 'G', 'T']
  let i = 0, v = n
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v >= 100 ? 0 : 1)}${u[i]}`
}

async function fetchInfo() {
  loading.value = true
  try {
    const res = await getZFSProperty(dataset.value)
    info.value = res.data || {}
    form.compression = info.value.compression?.value && info.value.compression.value !== 'none' ? info.value.compression.value : 'off'
    form.atime = info.value.atime?.value || 'off'
    form.quota = info.value.quota?.value && info.value.quota.value !== 'none' ? info.value.quota.value : ''
    form.refquota = info.value.refquota?.value && info.value.refquota.value !== 'none' ? info.value.refquota.value : ''
  } catch (e) {
    ElMessage.error(e?.message || '读取属性失败')
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const targets = [
      ['compression', form.compression],
      ['atime', form.atime],
      ['quota', form.quota || 'none'],
      ['refquota', form.refquota || 'none']
    ]
    for (const [prop, val] of targets) {
      const raw = info.value[prop]?.value || ''
      // 压缩/atime 空值归一为 off；quota/refquota 空值归一为 none——两侧同口径比较
      const cur = (prop === 'compression' || prop === 'atime') ? (raw || 'off') : (raw || 'none')
      if ((val || '') === (cur || '')) continue // 未变跳过
      await setZFSProperty(dataset.value, prop, val)
      ElMessage.success(prop + ' 已更新')
    }
    await fetchInfo()
  } catch (e) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

function open(ds) {
  dataset.value = ds || ''
  visible.value = true
  fetchInfo()
}
function handleClosed() { dataset.value = '' }
defineExpose({ open })
</script>

<style scoped>
.zfs-prop-body { display: flex; flex-direction: column; gap: 14px; }
.prop-form { margin-top: 4px; }
.prop-source { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
</style>
