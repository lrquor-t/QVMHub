<template>
  <el-dialog
    v-model="dialogVisible"
    title="导入虚拟机"
    width="600px"
    append-to-body
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <el-form :model="form" label-width="100px" :rules="rules" ref="formRef">
      <el-form-item label="虚拟机名称" prop="name">
        <el-input v-model="form.name" placeholder="英文字母开头，支持字母/数字/下划线/连字符" />
      </el-form-item>

      <el-form-item label="磁盘文件" prop="disk_file">
        <el-select v-model="form.disk_file" placeholder="选择磁盘文件" style="width: 100%;" v-loading="diskLoading">
          <el-option
            v-for="file in diskFiles"
            :key="file.name"
            :label="file.name"
            :value="file.name"
          >
            <div style="display: flex; justify-content: space-between;">
              <span>{{ file.name }}</span>
              <span style="color: #999; font-size: 12px;">{{ file.size_text }}</span>
            </div>
          </el-option>
        </el-select>
        <div style="margin-top: 4px; color: #909399; font-size: 12px;">
          从我的存储 → 虚拟磁盘中选择文件（导入后文件会被移动到虚拟机目录）
        </div>
      </el-form-item>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="CPU 核心" prop="vcpu">
            <el-input-number v-model="form.vcpu" :min="1" :max="vcpuMax" style="width: 100%;" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="内存 (GB)" prop="ram">
            <el-input-number v-model="form.ram" :min="1" :max="128" style="width: 100%;" />
          </el-form-item>
        </el-col>
      </el-row>


      <!-- 高级选项 -->
      <el-collapse style="margin-top: 8px;">
        <el-collapse-item title="高级选项">
          <el-form-item label="启动类型">
            <el-radio-group v-model="form.boot_type">
              <el-radio value="bios">BIOS</el-radio>
              <el-radio value="uefi">UEFI</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="机器类型">
            <el-radio-group v-model="form.machine_type">
              <el-radio value="q35">Q35</el-radio>
              <el-radio value="pc">PC (i440FX)</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="网卡模型">
            <el-select v-model="form.nic_model" style="width: 100%;">
              <el-option label="VirtIO (推荐)" value="virtio" />
              <el-option label="e1000e" value="e1000e" />
              <el-option label="e1000" value="e1000" />
              <el-option label="rtl8139" value="rtl8139" />
            </el-select>
          </el-form-item>
          <el-form-item label="显示设备">
            <el-select v-model="form.video_model" style="width: 100%;">
              <el-option label="VirtIO（高性能）" value="virtio" />
              <el-option label="ramfb（ARM 兼容）" value="ramfb" />
              <el-option label="VGA（兼容模式）" value="vga" />
              <el-option label="VMVGA（VMware 嵌套）" value="vmvga" />
              <el-option label="Cirrus（排障）" value="cirrus" />
            </el-select>
            <div style="margin-top: 4px; color: #909399; font-size: 12px;">
              Windows 安装或 VMware 嵌套环境可优先尝试 VGA / VMVGA
            </div>
          </el-form-item>
          <el-form-item label="开机自启">
            <el-switch v-model="form.autostart" />
          </el-form-item>
        </el-collapse-item>
      </el-collapse>
    </el-form>

    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">确认导入</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch, computed } from 'vue'
import { importVM } from '@/api/storage'
import { getStorageFiles } from '@/api/storage'
import { getHostCPUCores } from '@/api/settings'
import { ElMessage } from 'element-plus'

const props = defineProps({
  modelValue: Boolean
})

const emit = defineEmits(['update:modelValue', 'success'])

const dialogVisible = ref(false)
const formRef = ref(null)
const submitting = ref(false)
const diskLoading = ref(false)
const diskFiles = ref([])
const hostCPUCores = ref(0)
const vcpuMax = computed(() => hostCPUCores.value > 0 ? hostCPUCores.value : 64)

const form = reactive({
  name: '',
  disk_file: '',
  vcpu: 2,
  ram: 2,
  hostname: '',
  user: '',
  password: '',
  template_user: '',
  boot_type: 'bios',
  machine_type: 'q35',
  nic_model: 'virtio',
  video_model: 'virtio',
  autostart: false
})

const rules = {
  name: [
    { required: true, message: '请输入虚拟机名称', trigger: 'blur' },
    { pattern: /^[a-zA-Z][a-zA-Z0-9_-]*$/, message: '以字母开头，只能包含字母/数字/下划线/连字符', trigger: 'blur' }
  ],
  disk_file: [
    { required: true, message: '请选择磁盘文件', trigger: 'change' }
  ],
  vcpu: [
    { required: true, message: '请设置 CPU 核心数', trigger: 'change' }
  ],
  ram: [
    { required: true, message: '请设置内存大小', trigger: 'change' }
  ]
}

// 同步 v-model
watch(() => props.modelValue, (val) => {
  dialogVisible.value = val
  if (val) {
    loadDiskFiles()
    loadHostCPU()
  }
})
watch(dialogVisible, (val) => {
  emit('update:modelValue', val)
})


// 获取宿主机 CPU 核心数
const loadHostCPU = async () => {
  try {
    const res = await getHostCPUCores()
    if (res.code === 200 && res.data?.cores) {
      hostCPUCores.value = res.data.cores
    }
  } catch (err) {
    console.error('获取宿主机 CPU 核心数失败:', err)
  }
}

// 加载磁盘文件列表
const loadDiskFiles = async () => {
  diskLoading.value = true
  try {
    const res = await getStorageFiles('disk')
    diskFiles.value = res.data || []
  } catch (err) {
    console.error(err)
  } finally {
    diskLoading.value = false
  }
}

// 提交导入
const handleSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    await importVM({
      name: form.name,
      disk_file: form.disk_file,
      vcpu: form.vcpu,
      ram: form.ram,
      hostname: form.hostname || form.name,
      user: form.user,
      password: form.password,
      template_user: form.template_user,
      boot_type: form.boot_type,
      machine_type: form.machine_type,
      nic_model: form.nic_model,
      video_model: form.video_model,
      autostart: form.autostart
    })
    ElMessage.success('导入任务已提交，请在任务中心查看进度')
    dialogVisible.value = false
    emit('success')
  } catch (err) {
    console.error(err)
  } finally {
    submitting.value = false
  }
}

// 关闭对话框时重置表单
const handleClose = () => {
  formRef.value?.resetFields()
  form.name = ''
  form.disk_file = ''
  form.vcpu = 2
  form.ram = 2
  form.hostname = ''
  form.user = ''
  form.password = ''
  form.template_user = ''
  form.boot_type = 'bios'
  form.machine_type = 'q35'
  form.nic_model = 'virtio'
  form.video_model = 'virtio'
  form.autostart = false
}
</script>
