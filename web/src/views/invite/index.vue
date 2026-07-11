<template>
  <div class="auth-page">
    <el-card class="auth-card" v-loading="loading">
      <template v-if="detail">
        <h2>邀请注册</h2>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="用户名">{{ detail.username }}</el-descriptions-item>
          <el-descriptions-item label="邮箱">{{ detail.email }}</el-descriptions-item>
          <el-descriptions-item label="角色">{{ detail.role === 'admin' ? '管理员' : '普通用户' }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.role !== 'admin'" label="用户类型">{{ isLightweightInvite ? '轻量云' : '弹性云' }}</el-descriptions-item>
          <el-descriptions-item label="有效期">{{ detail.expires_at }}</el-descriptions-item>
          <template v-if="!isLightweightInvite">
            <el-descriptions-item label="CPU 配额">{{ detail.max_cpu || '不限' }}</el-descriptions-item>
            <el-descriptions-item label="内存配额">{{ detail.max_memory ? `${detail.max_memory} GB` : '不限' }}</el-descriptions-item>
            <el-descriptions-item label="磁盘配额">{{ detail.max_disk ? `${detail.max_disk} GB` : '不限' }}</el-descriptions-item>
            <el-descriptions-item label="虚拟机数量">{{ detail.max_vm || '不限' }}</el-descriptions-item>
            <el-descriptions-item label="存储配额">{{ detail.max_storage ? `${detail.max_storage} GB` : '不限' }}</el-descriptions-item>
            <el-descriptions-item label="运行时长配额">{{ detail.max_runtime_hours ? `${detail.max_runtime_hours} 小时` : '不限' }}</el-descriptions-item>
            <el-descriptions-item label="端口转发功能">{{ detail.enable_port_forward ? '已开通' : '未开通' }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.enable_port_forward" label="端口转发配额">{{ detail.max_port_forwards || '不限' }}</el-descriptions-item>
          </template>
          <template v-else>
            <el-descriptions-item label="资源模式">管理员分配云服务器</el-descriptions-item>
            <el-descriptions-item label="配额模式">流量、带宽、运行时长和端口转发按单台服务器配置</el-descriptions-item>
            <el-descriptions-item label="专用 VPC">{{ detail.dedicated_vpc_switch_id ? `#${detail.dedicated_vpc_switch_id}` : '管理员已配置' }}</el-descriptions-item>
          </template>
        </el-descriptions>
        <el-alert
          v-if="isLightweightInvite"
          type="info"
          show-icon
          :closable="false"
          class="lightweight-tip"
          title="轻量云账号注册后不会显示用户级额度。管理员会直接分配服务器，并为每台服务器设置流量、带宽和端口转发上限。"
        />
        <div v-if="isLightweightInvite && (detail.lightweight_vm_registrations || []).length" class="registration-list">
          <h3>待确认开通服务器</h3>
          <el-table :data="detail.lightweight_vm_registrations" border size="small">
            <el-table-column prop="vm_name" label="名称" min-width="120" />
            <el-table-column prop="template" label="模板" min-width="140" show-overflow-tooltip />
            <el-table-column label="规格" width="130">
              <template #default="{ row }">{{ row.vcpu }}C / {{ row.ram }}GB / {{ row.disk_size }}GB</template>
            </el-table-column>
            <el-table-column label="网络配额" min-width="190">
              <template #default="{ row }">{{ formatRegistrationQuota(row) }}</template>
            </el-table-column>
          </el-table>
          <div class="registration-note">注册完成并登录面板后，请在虚拟机列表中逐台补全登录凭据并确认开通。</div>
        </div>

        <el-form :model="form" label-width="100px" style="margin-top: 20px;">
          <el-form-item label="密码">
            <el-input v-model="form.password" type="password" show-password placeholder="请输入密码（至少12位）" />
          </el-form-item>
          <el-form-item label="确认密码">
            <el-input v-model="form.confirm_password" type="password" show-password placeholder="请再次输入密码" />
          </el-form-item>
          <div class="password-tip">
            密码至少 12 位。
          </div>
          <el-form-item>
            <el-button type="primary" :loading="submitting" @click="handleSubmit">完成注册</el-button>
            <el-button @click="router.push('/login')">返回登录</el-button>
          </el-form-item>
        </el-form>
      </template>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { completeInvite, getInviteInfo } from '@/api/auth'
import { useUserStore } from '@/store/user'
import { validatePassword, checkPasswordBreachAsync } from '@/utils/validate'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const token = route.query.token || ''

const loading = ref(false)
const submitting = ref(false)
const detail = ref(null)
const isLightweightInvite = computed(() => detail.value?.cloud_type === 'lightweight')
const form = reactive({
  password: '',
  confirm_password: ''
})

const formatRegistrationQuota = (row) => {
  const traffic = `${row.traffic_down_gb || 0}/${row.traffic_up_gb || 0}GB`
  const bandwidth = `${row.bandwidth_down_mbps || 0}/${row.bandwidth_up_mbps || 0}Mbps`
  const runtime = row.max_runtime_hours ? `${row.max_runtime_hours}小时` : '不限'
  return `流量 ${traffic}，带宽 ${bandwidth}，端口 ${row.max_port_forwards ?? 10}，运行 ${runtime}`
}

const fetchDetail = async () => {
  if (!token) {
    ElMessage.error('邀请令牌不存在')
    router.push('/login')
    return
  }
  loading.value = true
  try {
    const res = await getInviteInfo(token)
    detail.value = res.data
  } finally {
    loading.value = false
  }
}

const handleSubmit = async () => {
  if (!form.password || !form.confirm_password) {
    ElMessage.warning('请完整填写密码信息')
    return
  }
  // 本地常见弱密码检测
  const check = validatePassword(form.password)
  if (!check.valid) {
    ElMessage.error(check.message)
    return
  }
  // 异步泄露密码检测（HIBP API）
  const breach = await checkPasswordBreachAsync(form.password)
  if (breach.enabled && breach.breached) {
    ElMessage.error('该密码已在已知泄露数据库中发现，请更换为更安全的密码')
    return
  }
  submitting.value = true
  try {
    const res = await completeInvite({
      token,
      password: form.password,
      confirm_password: form.confirm_password
    })
    userStore.setToken(res.data.token)
    userStore.setUserInfo(res.data.username, res.data.role, res.data.security || null, res.data.cloud_type || 'elastic')
    ElMessage.success('注册完成，已自动登录')
    router.push('/')
  } finally {
    submitting.value = false
  }
}

onMounted(fetchDetail)
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: linear-gradient(135deg, #f3f7ff 0%, #eef8f1 100%);
}

.auth-card {
  width: min(760px, 100%);
}

h2 {
  margin-top: 0;
  margin-bottom: 18px;
}

.password-tip {
  margin: -6px 0 16px 100px;
  color: #606266;
  font-size: 12px;
  line-height: 1.6;
}

.lightweight-tip {
  margin-top: 16px;
}

.registration-list {
  margin-top: 18px;
}

.registration-list h3 {
  margin: 0 0 10px;
  font-size: 16px;
}

.registration-note {
  margin-top: 10px;
  color: #606266;
  font-size: 12px;
}

@media (max-width: 768px) {
  .auth-card :deep(.el-form-item__label) {
    width: 80px !important;
    font-size: 13px;
  }

  .password-tip {
    margin-left: 0;
  }

  .registration-list .el-table {
    font-size: 12px;
  }

  .registration-list .el-table :deep(.cell) {
    padding: 0 4px;
  }
}
</style>
