<template>
  <el-dialog
    v-model="visible"
    :title="isBatch ? '批量删除虚拟机' : `删除虚拟机 - ${vmName}`"
    width="600px"
    append-to-body
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <div v-loading="loadingDisks">
      <el-alert
        type="error"
        :closable="false"
        show-icon
        style="margin-bottom: 15px;"
      >
        <template #title>
          <span v-if="isBatch">确定要删除选中的 {{ vmList.length }} 台虚拟机吗？此操作不可恢复！</span>
          <span v-else>确定要删除虚拟机 <strong>{{ vmName }}</strong> 吗？此操作不可恢复！</span>
        </template>
      </el-alert>

      <!-- 单台删除时显示磁盘选择 -->
      <template v-if="!isBatch && diskList.length > 0">
        <p style="margin-bottom: 10px; font-weight: bold; color: #303133;">选择要删除的磁盘：</p>
        <el-checkbox-group v-model="selectedDisks">
          <div v-for="disk in diskList" :key="disk.path" style="margin-bottom: 8px;">
            <el-checkbox
              :value="disk.path"
              :disabled="disk.is_system"
            >
              <span style="font-family: monospace;">{{ disk.device }}</span>
              <el-tag size="small" type="info" style="margin-left: 8px;">{{ disk.format }}</el-tag>
              <span style="margin-left: 8px; color: #909399;">
                容量: {{ disk.capacity_gb }} GB
              </span>
              <el-tag v-if="disk.is_system" size="small" type="danger" style="margin-left: 8px;">系统盘</el-tag>
              <span style="margin-left: 8px; color: #C0C4CC; font-size: 12px;">
                {{ disk.path }}
              </span>
            </el-checkbox>
          </div>
        </el-checkbox-group>

        <!-- 未勾选磁盘的提示 -->
        <el-alert
          v-if="transferDisks.length > 0 && !storageInitialized && !isAdmin"
          type="error"
          :closable="false"
          show-icon
          style="margin-top: 15px;"
        >
          <template #title>
            您尚未开通「我的存储」，无法转移磁盘。请先前往「我的存储」页面初始化存储池，或勾选所有磁盘直接删除。
          </template>
        </el-alert>
        <el-alert
          v-else-if="transferDisks.length > 0"
          type="warning"
          :closable="false"
          show-icon
          style="margin-top: 15px;"
        >
          <template #title>
            以下磁盘将转移到“我的存储 - 虚拟磁盘”中：
          </template>
          <template #default>
            <div v-for="disk in transferDiskDetails" :key="disk.path" style="margin-top: 4px;">
              <span style="font-family: monospace;">{{ disk.device }}</span>
              <span style="margin-left: 8px; color: #606266;">{{ disk.path }}</span>
            </div>
          </template>
        </el-alert>
      </template>

      <!-- 批量删除时的简化提示 -->
      <template v-if="isBatch">
        <p style="color: #F56C6C;">将同时删除所有虚拟机的磁盘文件！</p>
        <div style="max-height: 200px; overflow-y: auto; padding: 10px; background: #fafafa; border-radius: 4px;">
          <el-tag
            v-for="vm in vmList"
            :key="vm.name"
            style="margin: 4px;"
          >
            {{ vm.name }}
          </el-tag>
        </div>
      </template>
    </div>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button
        type="danger"
        :loading="deleting"
        :disabled="transferDisks.length > 0 && !storageInitialized && !isAdmin"
        @click="handleConfirm"
      >
        确认删除
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed } from 'vue'
import { deleteVm, getVmQcow2Disks } from '@/api/vm'
import { selfDeleteVm, selfGetVmQcow2Disks } from '@/api/user'
import { getStorageInfo } from '@/api/storage'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/store/user'

const userStore = useUserStore()
const isAdmin = computed(() => userStore.role === 'admin')

const emit = defineEmits(['success'])

const visible = ref(false)
const loadingDisks = ref(false)
const deleting = ref(false)
const vmName = ref('')
const vmList = ref([]) // 批量删除时的VM列表
const isBatch = ref(false)
const diskList = ref([])
const selectedDisks = ref([]) // 勾选要删除的磁盘路径
const storageInitialized = ref(false) // 用户存储池是否已初始化

// 计算未勾选的磁盘（需要转移的）
const transferDisks = computed(() => {
  return diskList.value
    .filter(d => !selectedDisks.value.includes(d.path))
    .map(d => d.path)
})

const transferDiskDetails = computed(() => {
  return diskList.value.filter(d => !selectedDisks.value.includes(d.path))
})

// 打开对话框 - 单台删除
const open = async (name) => {
  isBatch.value = false
  vmName.value = name
  vmList.value = []
  diskList.value = []
  selectedDisks.value = []
  visible.value = true

  // 加载磁盘列表
  loadingDisks.value = true
  try {
    // 非管理员时同时检查存储池状态
    const promises = []
    if (isAdmin.value) {
      promises.push(getVmQcow2Disks(name))
    } else {
      promises.push(selfGetVmQcow2Disks(name))
      promises.push(getStorageInfo().catch(() => null))
    }
    const results = await Promise.all(promises)

    const diskRes = results[0]
    if (diskRes && diskRes.data && Array.isArray(diskRes.data)) {
      diskList.value = diskRes.data
      // 默认全部勾选
      selectedDisks.value = diskRes.data.map(d => d.path)
    }

    // 存储池状态
    if (isAdmin.value) {
      storageInitialized.value = true // 管理员不需要检查
    } else {
      const storageRes = results[1]
      storageInitialized.value = storageRes?.data?.initialized ?? false
    }
  } catch (e) {
    console.error('获取磁盘列表失败', e)
  } finally {
    loadingDisks.value = false
  }
}

// 打开对话框 - 批量删除
const openBatch = (vms) => {
  isBatch.value = true
  vmName.value = ''
  vmList.value = vms
  diskList.value = []
  selectedDisks.value = []
  visible.value = true
}

const handleClose = () => {
  diskList.value = []
  selectedDisks.value = []
  vmName.value = ''
  vmList.value = []
  storageInitialized.value = false
}

const handleConfirm = async () => {
  deleting.value = true
  try {
    if (isBatch.value) {
      // 批量删除：全部删除磁盘
      let promises = []
      if (isAdmin.value) {
        promises = vmList.value.map(vm => deleteVm(vm.name))
      } else {
        promises = vmList.value.map(vm => selfDeleteVm(vm.name))
      }
      const results = await Promise.allSettled(promises)
      const successCount = results.filter(r => r.status === 'fulfilled').length
      const failCount = results.filter(r => r.status === 'rejected').length
      ElMessage({
        type: failCount === 0 ? 'success' : 'warning',
        message: `批量删除完成。成功: ${successCount}, 失败: ${failCount}`
      })
    } else {
      // 单台删除：传递磁盘选择
      const data = {}
      if (diskList.value.length > 0) {
        data.delete_disks = selectedDisks.value
        data.transfer_disks = transferDisks.value
      }

      if (isAdmin.value) {
        await deleteVm(vmName.value, data)
      } else {
        await selfDeleteVm(vmName.value, data)
      }

      if (transferDisks.value.length > 0) {
        ElMessage.success('删除任务已提交，未勾选的磁盘将转移到"我的存储-虚拟磁盘"')
      } else {
        ElMessage.success('删除任务已提交')
      }
    }
    visible.value = false
    emit('success')
  } catch (e) {
    // API 错误由拦截器处理
  } finally {
    deleting.value = false
  }
}

defineExpose({ open, openBatch })
</script>
