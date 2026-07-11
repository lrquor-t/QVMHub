<template>
  <div class="login-container">
      <div class="login-box">
      <div class="card-header">
        <img class="login-logo" src="@/assets/logo.png" alt="logo" />
      </div>

      <template v-if="stage === 'login'">
        <el-form :model="form" :rules="rules" ref="loginFormRef" label-width="0">
          <el-form-item prop="username">
            <el-input v-model="form.username" size="large" placeholder="用户名" prefix-icon="User" />
          </el-form-item>
          <el-form-item prop="password">
            <el-input v-model="form.password" size="large" type="password" placeholder="密码" prefix-icon="Lock" show-password @keyup.enter="handleLogin" />
          </el-form-item>
          <el-form-item class="agreement-item">
            <el-checkbox v-model="agreementChecked" class="agreement-checkbox">
              <span>我已阅读并同意</span>
              <a href="https://qvmcdocs.xiaozhuhouses.asia/agreement?return=%2Fdocs%2Finstall%2F" target="_blank" rel="noopener noreferrer" class="agreement-link" @click.stop>《用户协议》</a>
            </el-checkbox>
          </el-form-item>
          <el-form-item class="submit-item">
            <el-button type="primary" size="large" class="login-btn" :loading="loading" :disabled="!agreementChecked" @click="handleLogin">登 录</el-button>
          </el-form-item>
        </el-form>
      </template>

      <template v-else-if="stage === 'login_verify'">
        <div class="stage-header">
          <h3>登录验证</h3>
          <p>账户：{{ stageUsername }}</p>
        </div>
        <el-radio-group v-if="allowedMethods.length > 1" v-model="selectedMethod" class="method-switch">
          <el-radio-button v-if="allowedMethods.includes('totp')" label="totp">2FA</el-radio-button>
          <el-radio-button v-if="allowedMethods.includes('recovery')" label="recovery">恢复码</el-radio-button>
          <el-radio-button v-if="allowedMethods.includes('email')" label="email">邮箱</el-radio-button>
        </el-radio-group>
        <el-alert
          v-if="selectedMethod === 'email'"
          type="info"
          :closable="false"
          style="margin-bottom: 16px;"
          :title="`验证码将发送至 ${stageSecurity.masked_email || '已绑定邮箱'}`"
        />
        <el-alert
          v-if="selectedMethod === 'recovery'"
          type="warning"
          :closable="false"
          style="margin-bottom: 16px;"
          title="每个恢复码只能使用一次，使用后即失效"
        />
        <el-form label-width="0">
          <el-form-item>
            <el-input
              v-if="selectedMethod === 'recovery'"
              v-model="loginVerifyForm.code"
              maxlength="16"
              show-word-limit
              placeholder="请输入 16 位恢复码"
              @keyup.enter="submitLoginVerify"
            />
            <el-input
              v-else
              v-model="loginVerifyForm.code"
              maxlength="6"
              show-word-limit
              placeholder="请输入 6 位验证码"
              @keyup.enter="submitLoginVerify"
            />
          </el-form-item>
          <el-form-item v-if="selectedMethod === 'email'">
            <el-button @click="sendLoginCode" :loading="sendingLoginCode">发送邮箱验证码</el-button>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="loading" @click="submitLoginVerify">完成验证</el-button>
            <el-button @click="resetStage">返回登录</el-button>
          </el-form-item>
        </el-form>
      </template>

      <template v-else-if="stage === 'bootstrap_security'">
        <div class="stage-header">
          <h3>安全初始化</h3>
          <p>请先完成必要的安全配置</p>
        </div>

        <el-card v-if="stageRole === 'admin' && !stageSecurity.smtp_configured" shadow="never" class="section-card">
          <template #header>SMTP 配置</template>
          <el-alert
            v-if="!smtpTested"
            title="请先填写 SMTP 配置信息和测试收件邮箱，发送测试邮件验证通过后才能保存配置。"
            type="warning"
            :closable="false"
            style="margin-bottom: 16px;"
          />
          <el-alert
            v-else
            title="测试邮件发送成功！请确认无误后点击保存按钮完成 SMTP 配置。"
            type="success"
            :closable="false"
            style="margin-bottom: 16px;"
          />
          <el-form label-width="110px">
            <el-form-item label="SMTP 主机">
              <el-input v-model="smtpForm.smtp_host" />
            </el-form-item>
            <el-form-item label="SMTP 端口">
              <el-input-number v-model="smtpForm.smtp_port" :min="1" :max="65535" style="width: 100%;" />
            </el-form-item>
            <el-form-item label="用户名">
              <el-input v-model="smtpForm.smtp_username" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="smtpForm.smtp_password" type="password" show-password placeholder="留空表示不修改已有密码" />
            </el-form-item>
            <el-form-item label="发件人名称">
              <el-input v-model="smtpForm.smtp_from_name" />
            </el-form-item>
            <el-form-item label="发件邮箱">
              <el-input v-model="smtpForm.smtp_from_address" />
            </el-form-item>
            <el-form-item label="加密方式">
              <el-select v-model="smtpForm.smtp_security" style="width: 100%;">
                <el-option label="STARTTLS" value="starttls" />
                <el-option label="SSL/TLS" value="ssl" />
                <el-option label="无加密" value="none" />
              </el-select>
            </el-form-item>
            <el-form-item label="超时秒数">
              <el-input-number v-model="smtpForm.smtp_timeout_seconds" :min="5" :max="120" style="width: 100%;" />
            </el-form-item>
            <el-form-item label="测试收件邮箱">
              <el-input v-model="smtpTestEmail" placeholder="请输入用于接收测试邮件的邮箱地址" :disabled="smtpTested" />
            </el-form-item>
            <el-form-item>
              <el-button type="success" :loading="testingSMTP" :disabled="smtpTested" @click="sendSMTPTest">发送测试邮件</el-button>
              <el-button v-if="smtpTested" type="primary" :loading="savingSMTP" @click="saveSMTP">保存 SMTP</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card v-if="stageSecurity.must_bind_email" shadow="never" class="section-card">
          <template #header>绑定邮箱</template>
          <el-alert
            v-if="!stageSecurity.smtp_configured"
            title="当前系统尚未配置 SMTP，暂时无法完成邮箱绑定，请联系管理员处理。"
            type="warning"
            :closable="false"
            style="margin-bottom: 16px;"
          />
          <el-form label-width="100px">
            <el-form-item label="邮箱">
              <el-input v-model="emailForm.email" placeholder="请输入邮箱" />
            </el-form-item>
            <el-form-item label="验证码">
              <el-input v-model="emailForm.code" maxlength="6" show-word-limit placeholder="请输入验证码" />
            </el-form-item>
            <el-form-item>
              <el-button @click="sendBindEmailCode" :loading="sendingEmailCode" :disabled="!stageSecurity.smtp_configured">发送验证码</el-button>
              <el-button type="primary" :loading="bindingEmail" @click="submitBindEmail" :disabled="!stageSecurity.smtp_configured">绑定邮箱</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card v-if="stageSecurity.must_bind_2fa" shadow="never" class="section-card">
          <template #header>绑定 2FA</template>
          <div style="margin-bottom: 12px;">
            <el-button @click="generate2FA" :loading="generating2FA">生成 2FA 配置</el-button>
          </div>
          <div v-if="totpSetup.secret" class="totp-box">
            <img :src="totpSetup.qrCodeData" alt="2FA QR" class="qr-image" />
            <div class="secret-line">
              <span>密钥：{{ totpSetup.secret }}</span>
            </div>
            <el-input v-model="totpCode" maxlength="6" show-word-limit placeholder="请输入 6 位验证码" style="margin-top: 12px;" />
            <el-button type="primary" style="margin-top: 12px;" :loading="binding2FA" @click="submitBind2FA">启用 2FA</el-button>
          </div>
        </el-card>

        <div class="action-row">
          <el-button type="warning" text @click="handleSkipBootstrap" style="margin-right: auto;">跳过安全设置</el-button>
          <el-button @click="resetStage">返回登录</el-button>
        </div>
      </template>

      <div class="helper-links">
        <el-link type="primary" :underline="false" @click="forgotVisible = true">忘记密码</el-link>
      </div>
    </div>

    <el-dialog v-model="forgotVisible" title="找回密码" width="420px" append-to-body destroy-on-close @closed="resetForgotState">
      <el-form label-width="80px">
        <template v-if="forgotStep === 'email'">
          <el-form-item label="邮箱">
            <el-input v-model="forgotForm.email" placeholder="请输入账号绑定邮箱" />
          </el-form-item>
          <div class="password-tip" style="margin-left: 80px;">
            系统会先向该邮箱发送验证码，验证成功后可选择要重置的账号。
          </div>
        </template>

        <template v-else-if="forgotStep === 'verify'">
          <el-alert
            :title="`验证码已发送至 ${forgotMaskedEmail || '目标邮箱'}`"
            type="info"
            :closable="false"
            style="margin-bottom: 16px;"
          />
          <el-form-item label="验证码">
            <el-input v-model="forgotForm.code" maxlength="6" show-word-limit placeholder="请输入 6 位验证码" />
          </el-form-item>
        </template>

        <template v-else>
          <el-alert
            :title="`请选择 ${forgotMaskedEmail || '该邮箱'} 下要重置的账号`"
            type="success"
            :closable="false"
            style="margin-bottom: 16px;"
          />
          <el-form-item label="账号">
            <el-select v-model="forgotForm.username" placeholder="请选择账号" style="width: 100%;">
              <el-option
                v-for="item in forgotAccounts"
                :key="item.username"
                :label="item.role === 'admin' ? `${item.username}（管理员）` : item.username"
                :value="item.username"
              />
            </el-select>
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="handleForgotCancel">{{ forgotStep === 'email' ? '取消' : '返回' }}</el-button>
        <el-button
          type="primary"
          :loading="sendingForgot"
          @click="submitForgot"
        >
          {{ forgotStep === 'email' ? '发送验证码' : forgotStep === 'verify' ? '验证邮箱' : '继续重置' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="dialogVisible"
      :title="`关于 ${displaySiteTitle}`"
      width="550px"
      append-to-body
      destroy-on-close
    >
      <div class="about-content">
        <p>{{ displaySiteTitle }}公有云平台，由开发者又菜有爱玩的小朱独立研发设计。底层采用 KVM+QEMU 支持软硬件虚拟机并自行研发模板和开通流程，所有的模板均为官方镜像并非二次魔改，并且采用标准的开通方式。</p>
        <p>同时我自行研发弹性云和轻量云，弹性云面向开发者群体和企业用户，采用配额计费体系，用户可以在配额内灵活自行开通虚拟机VPS并支持VPC内网通信、网段隔离和ACL安全组策略，轻量云则面向个人用户提供最简单的服务器使用方式，由管理员直接开通分配。</p>
        <p class="contact">如需体验试用，可联系 QQ：<strong>3354416548</strong>。</p>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button type="primary" @click="dialogVisible = false">确 定</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 首次登录强制修改默认密码弹窗 -->
    <el-dialog
      v-model="forcePwdVisible"
      title="首次登录请修改默认密码"
      width="460px"
      append-to-body
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      destroy-on-close
    >
      <el-alert
        type="warning"
        :closable="false"
        style="margin-bottom: 16px;"
        title="检测到您正在使用默认密码，为保障账户安全，请立即修改密码。"
      />
      <el-form
        ref="forcePwdFormRef"
        :model="forcePwdForm"
        :rules="forcePwdRules"
        label-width="80px"
      >
        <el-form-item label="当前密码" prop="oldPassword">
          <el-input
            v-model="forcePwdForm.oldPassword"
            type="password"
            show-password
            placeholder="请输入当前密码"
          />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input
            v-model="forcePwdForm.newPassword"
            type="password"
            show-password
            placeholder="请输入新密码"
          />
        </el-form-item>
        <el-form-item label="确认新密码" prop="confirmPassword">
          <el-input
            v-model="forcePwdForm.confirmPassword"
            type="password"
            show-password
            placeholder="请再次输入新密码"
            @keyup.enter="submitForcePasswordChange"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="handleForcePwdLogout">退出登录</el-button>
        <el-button type="primary" :loading="forcePwdLoading" @click="submitForcePasswordChange">
          修改密码并登录
        </el-button>
      </template>
    </el-dialog>

    <!-- 恢复码展示弹窗（仅在绑定 2FA 成功后显示一次） -->
    <el-dialog
      v-model="recoveryVisible"
      title="请保存恢复码"
      width="520px"
      append-to-body
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
    >
      <el-alert
        type="error"
        :closable="false"
        style="margin-bottom: 16px;"
        title="以下恢复码仅在本次显示，关闭后将无法再次查看。"
      />
      <div style="margin-bottom: 8px; font-size: 13px; color: var(--el-text-color-secondary);">
        当您的 2FA 验证器设备不可用时，可使用恢复码登录。每个恢复码只能使用一次。
      </div>
      <div class="recovery-codes-grid">
        <div v-for="(code, idx) in recoveryCodes" :key="idx" class="recovery-code-item">
          <span class="code-index">{{ String(idx + 1).padStart(2, '0') }}</span>
          <code class="code-text">{{ code }}</code>
        </div>
      </div>
      <div style="margin-top: 12px;">
        <el-button type="primary" @click="copyRecoveryCodes">一键复制所有恢复码</el-button>
        <el-button @click="downloadRecoveryCodes">下载为文本文件</el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="confirmRecoveryCodes">我已安全保存</el-button>
      </template>
    </el-dialog>

    <div class="footer-ack">
      <div class="footer-copy">
        &copy; <a href="https://github.com/QVMConsole/QVMConsole" target="_blank" rel="noopener noreferrer"><svg class="github-icon" viewBox="0 0 16 16" width="14" height="14" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg> QVMConsole (Open source Apache 2.0)</a>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import QRCode from 'qrcode'
import { ElMessage, ElMessageBox } from 'element-plus'
import { copyTextWithFallback } from '@/utils/clipboard'
import { useUserStore } from '@/store/user'
import { siteTitle } from '@/utils/site'
import {
  bindEmail,
  changePassword,
  enable2FA,
  login,
  selectForgotPasswordAccount,
  sendEmailCode,
  sendForgotPasswordCode,
  sendLoginEmailCode,
  setup2FA,
  verifyForgotPasswordCode,
  verifyLoginStage,
  skipBootstrap
} from '@/api/auth'
import { getSettings, testSMTP, updateSettings } from '@/api/settings'
import { passwordValidator, checkPasswordBreachAsync } from '@/utils/validate'

const router = useRouter()
const userStore = useUserStore()
const loginFormRef = ref(null)
const displaySiteTitle = computed(() => siteTitle.value)

// 用户协议勾选状态，勾选后通过 localStorage 记住
const agreementChecked = ref(false)
const AGREEMENT_STORAGE_KEY = 'qvm_agreement_accepted'

onMounted(() => {
  if (localStorage.getItem(AGREEMENT_STORAGE_KEY) === 'true') {
    agreementChecked.value = true
  }
})

watch(agreementChecked, (val) => {
  if (val) {
    localStorage.setItem(AGREEMENT_STORAGE_KEY, 'true')
  } else {
    localStorage.removeItem(AGREEMENT_STORAGE_KEY)
  }
})

const loading = ref(false)
const dialogVisible = ref(false)
const forgotVisible = ref(false)
const sendingForgot = ref(false)
const forgotStep = ref('email')
const forgotMaskedEmail = ref('')
const forgotAccounts = ref([])
const forgotForm = reactive({
  email: '',
  code: '',
  challenge_id: 0,
  selection_token: '',
  username: ''
})

const form = reactive({
  username: '',
  password: ''
})

const rules = reactive({
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
})

const stage = ref('login')
const stageToken = ref('')
const stageUsername = ref('')
const stageRole = ref('')
const stageSecurity = reactive({
  email: '',
  masked_email: '',
  must_bind_email: false,
  must_bind_2fa: false,
  smtp_configured: false
})
const allowedMethods = ref([])
const selectedMethod = ref('totp')

const loginVerifyForm = reactive({
  code: '',
  challenge_id: 0
})
const emailForm = reactive({
  email: '',
  code: '',
  challenge_id: 0
})
const smtpForm = reactive({
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  smtp_from_name: 'QVMConsole',
  smtp_from_address: '',
  smtp_security: 'starttls',
  smtp_timeout_seconds: 15
})
const smtpTestEmail = ref('')
const smtpTested = ref(false)
const totpSetup = reactive({
  secret: '',
  otpauth_url: '',
  qrCodeData: ''
})
const totpCode = ref('')

const sendingLoginCode = ref(false)
const sendingEmailCode = ref(false)
const bindingEmail = ref(false)
const savingSMTP = ref(false)
const testingSMTP = ref(false)
const generating2FA = ref(false)
const binding2FA = ref(false)

// 恢复码相关
const recoveryVisible = ref(false)
const recoveryCodes = ref([])
const pendingSession = ref(null)

// 强制修改默认密码弹窗
const forcePwdVisible = ref(false)
const forcePwdLoading = ref(false)
const forcePwdFormRef = ref(null)
const pendingForcePwdSession = ref(null)  // 暂存登录会话，密码修改成功后应用
const forcePwdForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})
const forcePwdRules = {
  oldPassword: [
    { required: true, message: '请输入当前密码', trigger: 'blur' }
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (!value) {
          callback(new Error('请输入新密码'))
          return
        }
        passwordValidator(rule, value, callback)
      },
      trigger: 'blur'
    }
  ],
  confirmPassword: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== forcePwdForm.newPassword) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const holdRecovery = (codes, sessionData) => {
  recoveryCodes.value = codes || []
  if (sessionData) {
    pendingSession.value = sessionData
  }
  if (recoveryCodes.value.length > 0) {
    recoveryVisible.value = true
  }
}

const copyRecoveryCodes = async () => {
  const text = recoveryCodes.value.join('\n')
  await copyTextWithFallback(text)
  ElMessage.success('恢复码已复制到剪贴板')
}

const downloadRecoveryCodes = () => {
  const text = recoveryCodes.value.join('\n')
  const blob = new Blob([text], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'qvmconsole-recovery-codes.txt'
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success('恢复码已下载')
}

const confirmRecoveryCodes = () => {
  recoveryVisible.value = false
  recoveryCodes.value = []
  if (pendingSession.value) {
    applySession(pendingSession.value)
    pendingSession.value = null
  } else {
    ElMessage.success('2FA 绑定已完成，请妥善保管恢复码')
  }
}

const applySession = (data) => {
  userStore.setToken(data.token)
  userStore.setUserInfo(data.username, data.role, data.security || null, data.cloud_type || 'elastic')
  ElMessage.success('登录成功')
  router.push('/')
}

const applyStage = async (data) => {
  stage.value = data.stage
  stageToken.value = data.token || ''
  stageUsername.value = data.username || ''
  stageRole.value = data.role || ''
  Object.assign(stageSecurity, data.security || {})
  allowedMethods.value = data.allowed_methods || []
  selectedMethod.value = allowedMethods.value.includes('totp') ? 'totp' : 'email'
  loginVerifyForm.code = ''
  loginVerifyForm.challenge_id = 0
  emailForm.email = data.security?.email || ''
  emailForm.code = ''
  emailForm.challenge_id = 0
  totpSetup.secret = ''
  totpSetup.otpauth_url = ''
  totpSetup.qrCodeData = ''
  totpCode.value = ''

  if (stage.value === 'bootstrap_security' && stageRole.value === 'admin' && !stageSecurity.smtp_configured) {
    smtpTestEmail.value = ''
    smtpTested.value = false
    await loadSMTPSettings()
  }
}

const handleLogin = async () => {
  if (!loginFormRef.value) return
  if (!agreementChecked.value) {
    ElMessage.warning('请先阅读并同意用户协议')
    return
  }
  const valid = await loginFormRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await login({ username: form.username, password: form.password })
    if (res.data.stage === 'success') {
      if (res.data.force_password_change) {
        // 首次登录需修改默认密码：先设置 token，弹出修改密码弹窗
        pendingForcePwdSession.value = res.data
        userStore.setToken(res.data.token)
        forcePwdForm.oldPassword = form.password  // 预填当前密码
        forcePwdForm.newPassword = ''
        forcePwdForm.confirmPassword = ''
        forcePwdVisible.value = true
      } else {
        applySession(res.data)
      }
    } else {
      await applyStage(res.data)
    }
  } finally {
    loading.value = false
  }
}

const sendLoginCode = async () => {
  sendingLoginCode.value = true
  try {
    const res = await sendLoginEmailCode(stageToken.value)
    loginVerifyForm.challenge_id = res.data.challenge_id
    ElMessage.success('验证码已发送')
  } finally {
    sendingLoginCode.value = false
  }
}

const submitLoginVerify = async () => {
  loading.value = true
  try {
    const payload = {
      method: selectedMethod.value,
      code: loginVerifyForm.code,
      challenge_id: loginVerifyForm.challenge_id
    }
    const res = await verifyLoginStage(stageToken.value, payload)
    applySession(res.data)
  } finally {
    loading.value = false
  }
}

const loadSMTPSettings = async () => {
  const res = await getSettings(stageToken.value)
  Object.assign(smtpForm, {
    smtp_host: res.data.smtp_host || '',
    smtp_port: res.data.smtp_port || 587,
    smtp_username: res.data.smtp_username || '',
    smtp_password: '',
    smtp_from_name: res.data.smtp_from_name || 'QVMConsole',
    smtp_from_address: res.data.smtp_from_address || '',
    smtp_security: res.data.smtp_security || 'starttls',
    smtp_timeout_seconds: res.data.smtp_timeout_seconds || 15
  })
}

const saveSMTP = async () => {
  savingSMTP.value = true
  try {
    await updateSettings({ ...smtpForm }, stageToken.value)
    stageSecurity.smtp_configured = true
    ElMessage.success('SMTP 配置已保存')
  } finally {
    savingSMTP.value = false
  }
}

const sendSMTPTest = async () => {
  if (!smtpTestEmail.value) {
    ElMessage.error('请先输入测试收件邮箱')
    return
  }
  testingSMTP.value = true
  try {
    await testSMTP({
      email: smtpTestEmail.value,
      ...smtpForm
    }, stageToken.value)
    smtpTested.value = true
    ElMessage.success('测试邮件已发送，请检查收件箱。确认无误后点击保存完成配置。')
  } finally {
    testingSMTP.value = false
  }
}

const sendBindEmailCode = async () => {
  if (!emailForm.email) {
    ElMessage.warning('请输入要绑定的邮箱')
    return
  }
  sendingEmailCode.value = true
  try {
    const res = await sendEmailCode({ email: emailForm.email }, stageToken.value)
    emailForm.challenge_id = res.data.challenge_id
    ElMessage.success('验证码已发送')
  } finally {
    sendingEmailCode.value = false
  }
}

const submitBindEmail = async () => {
  if (!emailForm.challenge_id) {
    ElMessage.warning('请先发送邮箱验证码')
    return
  }
  bindingEmail.value = true
  try {
    const res = await bindEmail({
      email: emailForm.email,
      code: emailForm.code,
      challenge_id: emailForm.challenge_id
    }, stageToken.value)
    if (res.data.stage === 'success') {
      applySession(res.data)
      return
    }
    Object.assign(stageSecurity, res.data.security || {})
    ElMessage.success('邮箱绑定成功')
  } finally {
    bindingEmail.value = false
  }
}

const generate2FA = async () => {
  generating2FA.value = true
  try {
    const res = await setup2FA(stageToken.value)
    totpSetup.secret = res.data.secret
    totpSetup.otpauth_url = res.data.otpauth_url
    totpSetup.qrCodeData = await QRCode.toDataURL(res.data.otpauth_url)
  } finally {
    generating2FA.value = false
  }
}

const submitBind2FA = async () => {
  if (!totpSetup.secret) {
    ElMessage.warning('请先生成 2FA 配置')
    return
  }
  binding2FA.value = true
  try {
    const res = await enable2FA({
      secret: totpSetup.secret,
      code: totpCode.value
    }, stageToken.value)
    if (res.data.stage === 'success') {
      holdRecovery(res.recovery?.recovery_codes, res.data)
      if (!recoveryVisible.value) {
        applySession(res.data)
      }
      return
    }
    Object.assign(stageSecurity, res.data.security || {})
    holdRecovery(res.recovery?.recovery_codes)
  } finally {
    binding2FA.value = false
  }
}

const submitForgot = async () => {
  if (forgotStep.value === 'email') {
    if (!forgotForm.email) {
      ElMessage.warning('请输入邮箱')
      return
    }
    sendingForgot.value = true
    try {
      const res = await sendForgotPasswordCode({ email: forgotForm.email })
      forgotForm.challenge_id = res.data.challenge_id
      forgotMaskedEmail.value = res.data.masked_email || ''
      forgotStep.value = 'verify'
      ElMessage.success('验证码已发送，请检查邮箱')
    } finally {
      sendingForgot.value = false
    }
    return
  }

  if (forgotStep.value === 'verify') {
    if (!forgotForm.challenge_id) {
      ElMessage.warning('请先发送验证码')
      return
    }
    if (!forgotForm.code) {
      ElMessage.warning('请输入验证码')
      return
    }
    sendingForgot.value = true
    try {
      const res = await verifyForgotPasswordCode({
        email: forgotForm.email,
        code: forgotForm.code,
        challenge_id: forgotForm.challenge_id
      })
      forgotAccounts.value = res.data.accounts || []
      forgotForm.selection_token = res.data.selection_token || ''
      forgotForm.username = forgotAccounts.value.length === 1 ? forgotAccounts.value[0].username : ''
      if (!forgotAccounts.value.length) {
        ElMessage.warning('该邮箱下暂无可重置的已激活账号')
        resetForgotState()
        return
      }
      forgotStep.value = 'select'
      ElMessage.success('邮箱验证成功，请选择账号')
    } finally {
      sendingForgot.value = false
    }
    return
  }

  if (!forgotForm.selection_token) {
    ElMessage.warning('账号选择状态已失效，请重新验证邮箱')
    resetForgotState()
    return
  }
  if (!forgotForm.username) {
    ElMessage.warning('请选择要重置的账号')
    return
  }
  sendingForgot.value = true
  try {
    const res = await selectForgotPasswordAccount({
      selection_token: forgotForm.selection_token,
      username: forgotForm.username
    })
    ElMessage.success(`已确认账号 ${res.data.username}，请设置新密码`)
    forgotVisible.value = false
    resetForgotState()
    router.push(`/reset-password?token=${encodeURIComponent(res.data.reset_token)}`)
  } finally {
    sendingForgot.value = false
  }
}

const handleSkipBootstrap = () => {
  ElMessageBox.confirm(
    '跳过安全设置后，SMTP 邮件服务、邮箱绑定和 2FA 双因素认证均不会配置。' +
    '相关功能（邀请注册、找回密码、邮箱验证等）将不可用，敏感操作将无法进行二次验证。' +
    '请在确保当前处于安全可信的网络环境中使用，并尽快完成安全配置。',
    '跳过安全初始化风险提示',
    {
      confirmButtonText: '我已知晓风险，跳过',
      cancelButtonText: '返回继续配置',
      type: 'warning',
      confirmButtonClass: 'el-button--danger',
      dangerouslyUseHTMLString: false
    }
  ).then(async () => {
    loading.value = true
    try {
      const res = await skipBootstrap(stageToken.value)
      applySession(res.data)
    } catch (err) {
      // error handled by interceptor
    } finally {
      loading.value = false
    }
  }).catch(() => {})
}

const resetStage = () => {
  stage.value = 'login'
  stageToken.value = ''
  stageUsername.value = ''
  stageRole.value = ''
  Object.assign(stageSecurity, {
    email: '',
    masked_email: '',
    must_bind_email: false,
    must_bind_2fa: false,
    smtp_configured: false
  })
  allowedMethods.value = []
  selectedMethod.value = 'totp'
  emailForm.email = ''
  emailForm.code = ''
  emailForm.challenge_id = 0
  loginVerifyForm.code = ''
  loginVerifyForm.challenge_id = 0
  totpSetup.secret = ''
  totpSetup.otpauth_url = ''
  totpSetup.qrCodeData = ''
  totpCode.value = ''
  smtpTestEmail.value = ''
  smtpTested.value = false
}

// 提交强制修改默认密码
const submitForcePasswordChange = async () => {
  const valid = await forcePwdFormRef.value?.validate().catch(() => false)
  if (!valid) return

  // 异步泄露密码检测（HIBP API）
  const breach = await checkPasswordBreachAsync(forcePwdForm.newPassword)
  if (breach.enabled && breach.breached) {
    ElMessage.error('该密码已在已知泄露数据库中发现，请更换为更安全的密码')
    return
  }

  forcePwdLoading.value = true
  try {
    const res = await changePassword({
      old_password: forcePwdForm.oldPassword,
      new_password: forcePwdForm.newPassword
    })
    ElMessage.success(res.message || '密码修改成功')
    forcePwdVisible.value = false
    // 密码修改成功，应用之前暂存的登录会话
    if (pendingForcePwdSession.value) {
      applySession(pendingForcePwdSession.value)
      pendingForcePwdSession.value = null
    }
  } catch (err) {
    // 拦截器已经弹出了错误提示，这里不再重复
  } finally {
    forcePwdLoading.value = false
  }
}

// 退出登录（放弃修改密码）
const handleForcePwdLogout = () => {
  userStore.logout()
  forcePwdVisible.value = false
  pendingForcePwdSession.value = null
  forcePwdForm.oldPassword = ''
  forcePwdForm.newPassword = ''
  forcePwdForm.confirmPassword = ''
  resetStage()
}

const resetForgotState = () => {
  forgotStep.value = 'email'
  forgotMaskedEmail.value = ''
  forgotAccounts.value = []
  forgotForm.email = ''
  forgotForm.code = ''
  forgotForm.challenge_id = 0
  forgotForm.selection_token = ''
  forgotForm.username = ''
}

const handleForgotCancel = () => {
  if (forgotStep.value === 'email') {
    forgotVisible.value = false
    resetForgotState()
    return
  }
  if (forgotStep.value === 'select') {
    forgotStep.value = 'verify'
    forgotForm.username = ''
    return
  }
  forgotStep.value = 'email'
  forgotForm.code = ''
  forgotForm.challenge_id = 0
  forgotMaskedEmail.value = ''
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  padding: 32px 20px 72px;
  box-sizing: border-box;
  background-color: #071728;
  background-image:
    linear-gradient(135deg, rgba(3, 14, 28, 0.72), rgba(10, 36, 61, 0.38)),
    url('@/assets/login-bg-cloud-tech.png');
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
  position: relative;
  overflow: hidden;
}

.login-container::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at top center, rgba(97, 186, 255, 0.22), transparent 42%),
    linear-gradient(180deg, rgba(4, 13, 25, 0.15), rgba(4, 13, 25, 0.44));
  pointer-events: none;
}

.login-box {
  position: relative;
  z-index: 1;
  width: 520px;
  max-width: 100%;
  max-height: 88vh;
  overflow-y: auto;
  padding: 36px 32px;
  background: rgba(243, 248, 255, 0.68);
  backdrop-filter: blur(18px);
  border-radius: 20px;
  box-shadow: 0 20px 60px rgba(4, 20, 38, 0.28);
  border: 1px solid rgba(255, 255, 255, 0.42);
}

.card-header {
  text-align: center;
  margin-bottom: 28px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.login-logo {
  width: 84px;
  height: 84px;
  object-fit: contain;
}

.card-header span {
  font-size: 26px;
  font-weight: bold;
  color: #16324f;
  letter-spacing: 1px;
  text-shadow: 0 1px 0 rgba(255, 255, 255, 0.28);
}

.submit-item {
  margin-top: 24px;
  margin-bottom: 0;
}

.login-btn {
  width: 100%;
  font-size: 16px;
  font-weight: bold;
  border-radius: 8px;
  letter-spacing: 4px;
  height: 44px;
}

.stage-header h3 {
  margin: 0 0 8px;
  font-size: 20px;
  color: #173554;
}

.stage-header p {
  margin: 0 0 18px;
  color: #606266;
}

.section-card {
  margin-bottom: 16px;
  background: rgba(255, 255, 255, 0.7);
}

.method-switch {
  margin-bottom: 16px;
}

.helper-links {
  margin-top: 16px;
  text-align: center;
  display: flex;
  justify-content: center;
  gap: 16px;
}

.totp-box {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.qr-image {
  width: 180px;
  height: 180px;
  border-radius: 12px;
  background: white;
  padding: 8px;
}

.secret-line {
  margin-top: 12px;
  color: #303133;
  word-break: break-all;
}

.action-row {
  display: flex;
  justify-content: flex-end;
}

.password-tip {
  margin-top: -10px;
  margin-bottom: 12px;
  color: #606266;
  font-size: 12px;
  line-height: 1.6;
}

.about-content p {
  line-height: 1.8;
  margin-bottom: 15px;
  color: #444;
  text-align: justify;
  font-size: 15px;
  text-indent: 2em;
}

.about-content .contact {
  margin-top: 25px;
  color: #333;
  font-size: 16px;
  text-indent: 0;
  text-align: center;
}

.footer-ack {
  position: absolute;
  bottom: 20px;
  left: 20px;
  right: 20px;
  text-align: center;
  font-size: 13px;
  color: rgba(235, 244, 255, 0.88);
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.55);
  z-index: 10;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.footer-copy,
.footer-thanks {
  line-height: 1.5;
}

.github-icon {
  vertical-align: -2px;
  margin-right: 2px;
}

.footer-ack a {
  color: rgba(255, 255, 255, 0.96);
  text-decoration: none;
  font-weight: 500;
  border-bottom: 1px dotted rgba(255, 255, 255, 0.5);
}

.agreement-item {
  margin-bottom: 8px;
}

.agreement-checkbox {
  white-space: normal;
  line-height: 1.6;
}

.agreement-link {
  color: var(--el-color-primary);
  text-decoration: none;
}

.agreement-link:hover {
  text-decoration: underline;
}

:deep(.el-input__wrapper) {
  background-color: rgba(255, 255, 255, 0.74) !important;
}

@media (max-width: 640px) {
  .login-container {
    padding: 20px 14px 56px;
    align-items: flex-start;
  }

  .login-box {
    margin-top: 20px;
    padding: 28px 20px;
    border-radius: 18px;
  }

  .login-logo {
    width: 69px;
    height: 69px;
  }

  .card-header span {
    font-size: 22px;
  }

  .about-content p {
    font-size: 13px;
    text-indent: 1.2em;
  }

  .action-row {
    justify-content: center;
  }

  .section-card :deep(.el-form-item__label) {
    width: 80px !important;
    font-size: 12px;
  }
}

@media (max-width: 480px) {
  .login-box {
    margin-top: 10px;
    padding: 22px 14px;
    border-radius: 14px;
  }

  .login-logo {
    width: 60px;
    height: 60px;
  }

  .card-header span {
    font-size: 20px;
  }

  .stage-header h3 {
    font-size: 17px;
  }

  .footer-ack {
    font-size: 11px;
  }
}

/* 恢复码网格 */
.recovery-codes-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 6px;
  margin-top: 8px;
}
.recovery-code-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  border: 1px solid var(--el-border-color-light);
}
.code-index {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  min-width: 22px;
}
.code-text {
  font-family: 'Courier New', monospace;
  font-size: 13px;
  color: var(--el-text-color-primary);
  letter-spacing: 0.5px;
  word-break: break-all;
}

/* 深色模式适配 */
.dark .recovery-code-item {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.1);
}
</style>
