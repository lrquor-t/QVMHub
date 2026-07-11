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
      <el-form-item label="分组名称">
        <el-select
          v-model="groupName"
          filterable
          allow-create
          clearable
          default-first-option
          placeholder="输入或选择分组名称，留空则取消分组"
          style="width: 100%;"
        >
          <el-option
            v-for="g in existingGroups"
            :key="g"
            :label="g"
            :value="g"
          />
        </el-select>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存分组</el-button>
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
const groupName = ref('')
const existingGroups = ref([])

const dialogTitle = computed(() => vmName.value ? `编辑分组 - ${vmName.value}` : '编辑分组')

const open = (name, currentGroup = '', groups = []) => {
  vmName.value = name || ''
  groupName.value = currentGroup || ''
  existingGroups.value = groups
  visible.value = true
}

const handleClosed = () => {
  vmName.value = ''
  groupName.value = ''
  existingGroups.value = []
  submitting.value = false
}

const handleSubmit = async () => {
  if (!vmName.value) {
    return
  }

  submitting.value = true
  try {
    const nextGroup = (groupName.value || '').trim()
    const res = await updateVm(vmName.value, { group: nextGroup })
    ElMessage.success(res.message || '分组已更新')
    emit('success', { name: vmName.value, group: nextGroup })
    visible.value = false
  } finally {
    submitting.value = false
  }
}

defineExpose({
  open
})
</script>
