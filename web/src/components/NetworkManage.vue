<template>
  <el-dialog :title="'网络管理 - ' + currentVmName" v-model="visible" width="900px" append-to-body>
    <NetworkList v-if="visible" :vm-name="currentVmName" />
  </el-dialog>
</template>

<script setup>
import { ref } from 'vue'
import NetworkList from './NetworkList.vue'

const visible = ref(false)
const currentVmName = ref('')

const open = (vmName) => {
  currentVmName.value = vmName
  visible.value = true
}

defineExpose({ open })
</script>
