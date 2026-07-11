<template>
  <el-dialog title="制作模板" v-model="visible" width="500px" :close-on-click-modal="false" append-to-body>
    <el-form :model="form" label-width="120px">
      <el-form-item label="源虚拟机">
        <el-input :model-value="vmName" disabled />
      </el-form-item>

      <el-form-item label="模板名称" required>
        <el-input v-model="form.name" placeholder="管理员侧名称（字母、数字、点、下划线、短横线）" />
      </el-form-item>

      <el-form-item label="用户侧显示">
        <el-input v-model="form.display_name" placeholder="从模板克隆下拉框中显示的标题" />
      </el-form-item>

      <el-form-item label="模板类型">
        <el-radio-group v-model="form.type">
          <el-radio value="linux">
            <span style="display: flex; align-items: center; gap: 4px;">🐧 Linux</span>
          </el-radio>
          <el-radio value="windows">
            <span style="display: flex; align-items: center; gap: 4px;">🪟 Windows</span>
          </el-radio>
          <el-radio value="fnos">
            <span style="display: flex; align-items: center; gap: 4px;">📦 FnOS</span>
          </el-radio>
          <el-radio value="openwrt">
            <span style="display: flex; align-items: center; gap: 4px;">🌐 OpenWrt</span>
          </el-radio>
        </el-radio-group>
      </el-form-item>

      <template v-if="form.type === 'linux' || form.type === 'windows' || form.type === 'openwrt'">
        <el-form-item :label="form.type === 'windows' ? 'Windows 分类' : form.type === 'openwrt' ? 'OpenWrt 分类' : 'Linux 分类'">
          <el-select
            v-model="form.category"
            filterable
            :placeholder="categoryPlaceholder"
            style="width: 100%;"
          >
            <el-option
              v-for="item in categoryOptions"
              :key="item"
              :label="item"
              :value="item"
            />
          </el-select>
          <div class="form-tip">
            <el-icon><InfoFilled /></el-icon>
            {{ categoryTip }}
          </div>
        </el-form-item>
      </template>

      <!-- 初始化方式：所有系统类型通用 -->
      <template v-if="form.type !== 'other'">
        <el-divider content-position="left" style="margin: 12px 0;">
          {{ initDividerTitle }}
        </el-divider>

        <el-form-item label="初始化方式">
          <el-radio-group v-model="form.init_mode" @change="onInitModeChange">
            <template v-if="form.type === 'linux'">
              <el-radio value="nocloud">
                <span>☁️ cloud-init（推荐）</span>
              </el-radio>
              <el-radio value="none">
                <span>🚫 不初始化</span>
              </el-radio>
            </template>
            <template v-else-if="form.type === 'windows'">
              <el-radio value="configdrive">
                <span>🪟 ConfigDrive cloudbase-init（推荐）</span>
              </el-radio>
              <el-radio value="none">
                <span>🚫 不初始化</span>
              </el-radio>
            </template>
            <template v-else-if="form.type === 'fnos'">
              <el-radio value="fnos">
                <span>🛠️ virt-customize 离线初始化（推荐）</span>
              </el-radio>
              <el-radio value="none">
                <span>🚫 不初始化</span>
              </el-radio>
            </template>
            <template v-else-if="form.type === 'openwrt'">
              <el-radio value="openwrt">
                <span>🌐 UCI 配置注入（推荐）</span>
              </el-radio>
              <el-radio value="none">
                <span>🚫 不初始化</span>
              </el-radio>
            </template>
          </el-radio-group>
          <div class="form-tip">
            <el-icon><InfoFilled /></el-icon>
            <span v-if="form.init_mode === 'nocloud'">
              模板内需预装 cloud-init，克隆时自动扩容磁盘、设置 hostname，无需 SSH 连接
            </span>
            <span v-else-if="form.init_mode === 'configdrive'">
              克隆时通过 ConfigDrive 注入 cloudbase-init 配置，自动设置密码和网络
            </span>
            <span v-else-if="form.init_mode === 'fnos'">
              克隆时通过 virt-customize 注入用户名、密码、hostname、设备 ID 等 FnOS 首次启动配置
            </span>
            <span v-else-if="form.init_mode === 'openwrt'">
              克隆时通过 virt-customize 注入静态 IP、网关、DNS 和主机名等 OpenWrt UCI 配置
            </span>
            <span v-else>
              克隆时将直接完整复制模板磁盘，不做任何初始化操作
            </span>
          </div>
        </el-form-item>

        <el-form-item v-if="form.type === 'linux' && form.init_mode !== 'none'" label="模板用户名">
          <el-input v-model="form.template_user" placeholder="模板中已有的普通用户名" />
          <div class="form-tip">
            <el-icon><InfoFilled /></el-icon>
            克隆时若目标用户名与模板用户名不同，自动离线重命名
          </div>
        </el-form-item>

        <el-form-item v-if="form.type === 'linux' && form.init_mode !== 'none'" label="启动后命令">
          <el-input
            v-model="form.post_boot_command"
            type="textarea"
            :rows="3"
            placeholder="克隆后首次启动时执行的自定义 Shell 命令（可多行）"
          />
          <div class="form-tip">
            <el-icon><InfoFilled /></el-icon>
            命令将以 root 权限执行，仅首次启动时运行
          </div>
          <el-checkbox v-model="form.post_boot_blocking" :disabled="!form.post_boot_command" style="margin-top: 6px;">
            等待命令执行完毕后再启动 SSH
          </el-checkbox>
          <div v-if="form.post_boot_blocking" class="form-tip" style="color: #E6A23C;">
            <el-icon><InfoFilled /></el-icon>
            启用后系统启动期间将显示“正在启动 QVM 初始化服务”，用户在此期间无法通过 SSH 登录
          </div>
        </el-form-item>
      </template>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="loading" @click="handleSubmit">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref, reactive, watch } from 'vue'
import { prepareTemplate } from '@/api/vm'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  DEFAULT_LINUX_TEMPLATE_CATEGORY,
  DEFAULT_WINDOWS_TEMPLATE_CATEGORY,
  DEFAULT_OPENWRT_TEMPLATE_CATEGORY,
  LINUX_TEMPLATE_CATEGORY_OPTIONS,
  WINDOWS_TEMPLATE_CATEGORY_OPTIONS,
  OPENWRT_TEMPLATE_CATEGORY_OPTIONS,
  normalizeTemplateCategory,
} from '@/utils/templateCategory'

const emit = defineEmits(['success'])

const visible = ref(false)
const loading = ref(false)
const vmName = ref('')

const form = reactive({
  name: '',
  display_name: '',
  type: 'linux',
  category: DEFAULT_LINUX_TEMPLATE_CATEGORY,
  init_mode: 'nocloud',
  template_user: '',
  post_boot_command: '',
  post_boot_blocking: false,
})

const categoryOptions = computed(() => {
  if (form.type === 'windows') return WINDOWS_TEMPLATE_CATEGORY_OPTIONS
  if (form.type === 'openwrt') return OPENWRT_TEMPLATE_CATEGORY_OPTIONS
  return LINUX_TEMPLATE_CATEGORY_OPTIONS
})
const categoryPlaceholder = computed(() => {
  if (form.type === 'windows') return '默认归入 WindowsServer2022，可选择 WindowsServer2025 / Windows11 / Windows10 / WindowsServer2012R2 / 其它'
  if (form.type === 'openwrt') return '默认归入 OpenWrt'
  return '默认归入 Ubuntu，可选择 Debian、CentOS'
})
const categoryTip = computed(() => {
  if (form.type === 'windows') return 'Windows 模板按版本分类展示，2012 R2 会保留 BIOS/SATA 等默认配置用于克隆'
  if (form.type === 'openwrt') return 'OpenWrt 模板克隆时支持配置静态 IP、网关和密码'
  return 'Linux 模板按发行版分类展示'
})

watch(() => form.type, (type) => {
  if (type === 'windows') {
    form.category = normalizeTemplateCategory('windows', form.category || DEFAULT_WINDOWS_TEMPLATE_CATEGORY)
    form.init_mode = 'configdrive'
  } else if (type === 'linux') {
    form.category = normalizeTemplateCategory('linux', form.category || DEFAULT_LINUX_TEMPLATE_CATEGORY)
    form.init_mode = 'nocloud'
  } else if (type === 'fnos') {
    form.category = ''
    form.init_mode = 'fnos'
  } else if (type === 'openwrt') {
    form.category = normalizeTemplateCategory('openwrt', DEFAULT_OPENWRT_TEMPLATE_CATEGORY)
    form.init_mode = 'openwrt'
  }
})

const open = (name) => {
  vmName.value = name
  form.name = name + '-tpl'
  form.display_name = form.name
  form.type = 'linux'
  form.category = DEFAULT_LINUX_TEMPLATE_CATEGORY
  form.init_mode = 'nocloud'
  form.template_user = ''
  form.post_boot_command = ''
  form.post_boot_blocking = false
  visible.value = true
}

const initDividerTitle = computed(() => {
  if (form.type === 'linux') return 'Linux 模板配置'
  if (form.type === 'windows') return 'Windows 模板配置'
  if (form.type === 'fnos') return 'FnOS 模板配置'
  if (form.type === 'openwrt') return 'OpenWrt 模板配置'
  return '模板配置'
})

const onInitModeChange = async (value) => {
  if (value !== 'none') return
  if (form.type === 'linux') {
    try {
      await ElMessageBox.confirm(
        '选择「不初始化」意味着克隆此模板时不会进行任何系统初始化操作（不会设置 hostname、不会扩容磁盘、不会注入密码），克隆出的虚拟机将完全保留模板的原始状态。\n\n请确保：\n1. 模板内已自行完成通用化处理（如删除 SSH 主机密钥、清理 machine-id 等）\n2. 模板磁盘大小已满足最终需求，后续不会自动扩容\n3. 你清楚克隆后需自行登录虚拟机进行个性化配置',
        '⚠️ 风险确认：不初始化模板',
        { confirmButtonText: '我已知晓风险，继续', cancelButtonText: '取消', type: 'warning', dangerouslyUseHTMLString: true }
      )
    } catch {
      form.init_mode = form.type === 'windows' ? 'configdrive' : 'nocloud'
    }
  } else if (form.type === 'windows') {
    try {
      await ElMessageBox.confirm(
        '选择「不初始化」意味着克隆此模板时不会进行任何系统初始化操作（不会注入 ConfigDrive、不会设置密码、不会执行 cloudbase-init）。克隆出的虚拟机将完全保留模板的原始状态。\n\n<strong>⚠ 请在制作模板前务必对源虚拟机执行 sysprep 通用化：</strong>\n1. 运行 sysprep.exe 并勾选「通用」选项（/generalize）\n2. 关机后制作模板，确保 SID 和其他唯一标识已被清除\n3. 克隆后的 Windows 将在首次启动时重新进入 OOBE 初始化流程\n\n未通用化的 Windows 模板将导致克隆虚拟机出现 SID 冲突、域加入失败等问题。',
        '⚠️ 风险确认：不初始化模板',
        { confirmButtonText: '已通用化，继续', cancelButtonText: '取消', type: 'warning', dangerouslyUseHTMLString: true }
      )
    } catch {
      form.init_mode = 'configdrive'
    }
  } else if (form.type === 'fnos') {
    try {
      await ElMessageBox.confirm(
        '选择「不初始化」意味着克隆此模板时不会进行任何系统初始化操作。克隆出的虚拟机将完全保留模板的原始状态。\n\n请确保模板已完成必要的通用化处理。',
        '⚠️ 风险确认：不初始化模板',
        { confirmButtonText: '我已知晓风险，继续', cancelButtonText: '取消', type: 'warning' }
      )
    } catch {
      form.init_mode = 'fnos'
    }
  } else if (form.type === 'openwrt') {
    try {
      await ElMessageBox.confirm(
        '选择「不初始化」意味着克隆此模板时不会注入任何网络配置。克隆出的 OpenWrt 将保留模板原始 IP 配置。\n\n请确保模板已完成必要的通用化处理。',
        '⚠️ 风险确认：不初始化模板',
        { confirmButtonText: '我已知晓风险，继续', cancelButtonText: '取消', type: 'warning' }
      )
    } catch {
      form.init_mode = 'openwrt'
    }
  }
}

const handleSubmit = async () => {
  if (form.name.includes('..')) {
    ElMessage.warning('模板名称不能包含连续的点')
    return
  }

  if (!/^[a-zA-Z0-9._-]+$/.test(form.name)) {
    ElMessage.warning('模板名称只能包含字母、数字、点、下划线和短横线')
    return
  }

  loading.value = true
  try {
    const type = form.type
    await prepareTemplate({
      vm_name: vmName.value,
      template_name: form.name,
      display_name: form.display_name || form.name,
      type: type,
      category: ['linux', 'windows', 'openwrt'].includes(type) ? normalizeTemplateCategory(type, form.category) : undefined,
      cloud_init_mode: form.init_mode === 'none' ? 'none' : (form.init_mode || undefined),
      template_user: form.template_user || undefined,
      post_boot_command: form.post_boot_command || undefined,
      post_boot_blocking: form.post_boot_blocking || undefined,
    })
    ElMessage.success('制作模板任务已提交，请在任务中查看进度')
    visible.value = false
    emit('success')
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

defineExpose({ open })
</script>

<style scoped>
.form-tip {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}
</style>
