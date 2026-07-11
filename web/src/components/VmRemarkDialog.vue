<template>
  <el-dialog
    v-model="visible"
    :title="dialogTitle"
    width="520px"
    append-to-body
    :close-on-click-modal="false"
    @closed="handleClosed"
  >
    <el-form label-width="80px">
      <el-form-item label="虚拟机">
        <el-input :model-value="vmName" disabled />
      </el-form-item>
      <el-form-item label="备注">
        <el-input
          v-model="remark"
          type="textarea"
          :rows="4"
          maxlength="200"
          show-word-limit
          placeholder="用于记录用途、环境或业务信息；留空保存将清空备注"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存备注</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { updateVm } from '@/api/vm'

const emit = defineEmits(['success'])

const visible = ref(false)
const submitting = ref(false)
const vmName = ref('')
const remark = ref('')

const dialogTitle = computed(() => vmName.value ? `编辑备注 - ${vmName.value}` : '编辑备注')

const open = (name, currentRemark = '') => {
  vmName.value = name || ''
  remark.value = currentRemark || ''
  visible.value = true
}

const handleClosed = () => {
  vmName.value = ''
  remark.value = ''
  submitting.value = false
}

const handleSubmit = async () => {
  if (!vmName.value) {
    return
  }

  submitting.value = true
  try {
    const nextRemark = remark.value.trim()
    const res = await updateVm(vmName.value, { remark: nextRemark })
    ElMessage.success(res.message || '备注已更新')
    emit('success', { name: vmName.value, remark: nextRemark })
    visible.value = false
  } finally {
    submitting.value = false
  }
}

defineExpose({
  open
})
</script>
