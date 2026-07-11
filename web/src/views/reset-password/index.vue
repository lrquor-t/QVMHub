<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <h2>重置密码</h2>
      <el-form :model="form" label-width="100px">
        <el-form-item label="新密码">
          <el-input v-model="form.password" type="password" show-password placeholder="请输入密码（至少12位）" />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="form.confirm_password" type="password" show-password placeholder="请再次输入密码" />
        </el-form-item>
        <div class="password-tip">
          密码至少 12 位。
        </div>
        <el-form-item>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">重置密码</el-button>
          <el-button @click="router.push('/login')">返回登录</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { resetPasswordByEmail } from '@/api/auth'
import { validatePassword, checkPasswordBreachAsync } from '@/utils/validate'

const route = useRoute()
const router = useRouter()
const token = route.query.token || ''

const submitting = ref(false)
const form = reactive({
  password: '',
  confirm_password: ''
})

const handleSubmit = async () => {
  if (!token) {
    ElMessage.error('重置令牌不存在')
    return
  }
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
    await resetPasswordByEmail({
      token,
      password: form.password,
      confirm_password: form.confirm_password
    })
    ElMessage.success('密码已重置，请重新登录')
    router.push('/login')
  } finally {
    submitting.value = false
  }
}
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
  width: min(520px, 100%);
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

@media (max-width: 768px) {
  .auth-card :deep(.el-form-item__label) {
    width: 80px !important;
    font-size: 13px;
  }

  .password-tip {
    margin-left: 0;
  }
}
</style>
