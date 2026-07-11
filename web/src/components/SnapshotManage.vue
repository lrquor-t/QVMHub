<template>
  <el-dialog title="快照管理" v-model="visible" width="700px" append-to-body>
    <SnapshotList v-if="visible" :vm-name="currentVmName" :vm-status="currentVmStatus" />
  </el-dialog>
</template>

<script setup>
import { ref } from 'vue'
import SnapshotList from './SnapshotList.vue'

const visible = ref(false)
const currentVmName = ref('')
const currentVmStatus = ref('')

const open = (vmName, vmStatus) => {
  currentVmName.value = vmName
  currentVmStatus.value = vmStatus
  visible.value = true
}

defineExpose({ open })
</script>
