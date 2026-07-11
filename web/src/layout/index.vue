<template>
  <el-container class="app-wrapper">
    <!-- 侧边栏 -->
    <el-aside 
      :width="isMobile ? '260px' : (isCollapse ? '64px' : '220px')" 
      class="sidebar"
      :class="{ 'mobile-show': !isCollapse && isMobile, 'is-collapsed': isCollapse && !isMobile }"
    >
      <div class="logo">
        <img class="sidebar-logo" src="@/assets/logo.png" alt="logo" />
        <el-icon v-if="isMobile" class="mobile-sidebar-close" @click="isCollapse = true"><Close /></el-icon>
      </div>
      <el-menu
        :default-active="activeMenu"
        class="el-menu-vertical"
        router
        :collapse="isCollapse && !isMobile"
        :collapse-transition="false"
        @select="handleMenuSelect"
      >
        <template v-for="node in menuNodes" :key="node.type === 'group' ? ('g:' + node.id) : ('i:' + node.route)">
          <el-menu-item v-if="node.type === 'item'" :index="node.route">
            <SidebarIcons :icon="node.icon" />
            <template #title>{{ node.title }}</template>
          </el-menu-item>
          <el-sub-menu v-else :index="'g:' + node.id">
            <template #title>
              <SidebarIcons :icon="node.icon" />
              <span>{{ node.title }}</span>
            </template>
            <el-menu-item v-for="child in node.children" :key="child.route" :index="child.route">
              <SidebarIcons :icon="child.icon" />
              <template #title>{{ child.title }}</template>
            </el-menu-item>
          </el-sub-menu>
        </template>
      </el-menu>
    </el-aside>

    <!-- 移动端遮罩 -->
    <div v-if="isMobile && !isCollapse" class="mobile-mask" @click="isCollapse = true"></div>

    <!-- 主体内容 -->
    <el-container class="main-container">
      <!-- 导航栏 -->
      <el-header class="navbar">
        <div class="left-menu">
          <el-icon class="fold-btn" @click="toggleCollapse">
            <component :is="isCollapse ? Expand : Fold" />
          </el-icon>
          <span class="page-title">{{ route.meta.title || displaySiteTitle }}</span>
        </div>
        <div class="right-menu">
          <el-badge :value="activeTaskCount" :hidden="activeTaskCount === 0" :max="99" class="task-badge">
            <el-button text circle @click="toggleRecentTaskPanel" title="近期任务" class="task-toggle-btn">
              <el-icon size="18"><List /></el-icon>
            </el-button>
          </el-badge>
          <el-switch
            v-model="isDark"
            inline-prompt
            :active-icon="Moon"
            :inactive-icon="Sunny"
            @change="toggleDark"
            class="dark-switch"
          />
          <el-link 
            href="https://github.com/QVMConsole/QVMConsole" 
            target="_blank" 
            :underline="false"
            class="oss-link"
          >
            <el-icon><Link /></el-icon>
            开源版
          </el-link>
          <el-tag v-if="!isAdmin" type="success" size="small" class="cloud-tag">
            {{ isLightweight ? '轻量云' : '弹性云' }}
          </el-tag>
          <el-dropdown trigger="click" @command="handleCommand">
            <span class="el-dropdown-link user-info">
              <el-avatar :size="32" :icon="UserFilled" />
              <span class="username">{{ userStore.username || 'Admin' }}</span>
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="security">
                  <el-icon><Lock /></el-icon>
                  安全设置
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- 路由占位层 -->
      <el-main class="app-main">
        <router-view v-slot="{ Component }">
          <transition name="fade-transform" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>

      <!-- 近期任务面板 -->
      <RecentTaskPanel
        ref="recentTaskPanelRef"
        :class="{ 'task-panel-hidden': !isLogged }"
        @open-full="openFullTaskCenter"
      />
    </el-container>

    <!-- 安全设置对话框 -->
    <el-dialog
      v-model="securityDialogVisible"
      title="安全设置"
      width="620px"
      :close-on-click-modal="false"
      destroy-on-close
      append-to-body
    >
      <el-tabs v-model="securityTab" class="security-tabs">
        <el-tab-pane label="邮箱" name="email">
          <el-form label-width="110px" class="security-form">
            <el-alert
              v-if="userStore.security?.must_bind_email"
              title="当前账户尚未完成邮箱绑定，部分安全能力不可用。"
              type="warning"
              :closable="false"
              style="margin-bottom: 16px;"
            />
            <el-form-item label="当前邮箱">
              <el-input :model-value="userStore.security?.email || '未绑定'" disabled />
            </el-form-item>
            <el-form-item label="验证状态">
              <el-tag :type="userStore.security?.email_verified ? 'success' : 'warning'">
                {{ userStore.security?.email_verified ? '已验证' : '未验证' }}
              </el-tag>
            </el-form-item>
            <el-form-item label="新邮箱">
              <el-input v-model="emailForm.email" placeholder="请输入邮箱" />
            </el-form-item>
            <el-form-item label="验证码">
              <el-input v-model="emailForm.code" maxlength="6" show-word-limit placeholder="请输入邮箱验证码" />
            </el-form-item>
            <div class="security-tip">
              邮箱验证码 10 分钟内有效，验证通过后会立即更新账户绑定邮箱。
            </div>
            <el-form-item>
              <el-button @click="handleSendEmailCode" :loading="emailCodeLoading">发送验证码</el-button>
              <el-button type="primary" @click="handleBindEmail" :loading="emailBindingLoading">保存邮箱</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="2FA" name="totp">
          <el-alert
            :title="userStore.security?.totp_enabled ? '已启用 2FA 验证' : '建议启用 2FA 验证增强账户安全'"
            :type="userStore.security?.totp_enabled ? 'success' : 'warning'"
            :closable="false"
            style="margin-bottom: 16px;"
          />

          <template v-if="userStore.security?.totp_enabled">
            <el-form label-width="110px" class="security-form">
              <el-form-item label="当前密码">
                <el-input v-model="disable2FAForm.password" type="password" show-password placeholder="请输入当前密码" />
              </el-form-item>
              <el-form-item label="2FA 验证码">
                <el-input v-model="disable2FAForm.code" maxlength="6" show-word-limit placeholder="请输入 6 位验证码" />
              </el-form-item>
              <el-form-item>
                <el-button type="danger" :loading="disable2FALoading" @click="handleDisable2FA">关闭 2FA</el-button>
              </el-form-item>
            </el-form>

            <!-- 恢复码管理 -->
            <el-divider />
            <el-alert
              v-if="userStore.security?.has_recovery_codes"
              type="success"
              :closable="false"
              style="margin-bottom: 12px;"
              title="您有可用的恢复码，若 2FA 设备不可用可使用恢复码登录"
            />
            <el-alert
              v-else
              type="warning"
              :closable="false"
              style="margin-bottom: 12px;"
              title="暂无可用恢复码，建议生成新的恢复码以备用"
            />
            <div class="security-tip" style="margin-bottom: 12px;">
              恢复码用于在 2FA 验证器不可用时登录。重新生成后旧恢复码将立即失效。
            </div>
            <el-form label-width="110px" class="security-form">
              <el-form-item label="当前密码">
                <el-input v-model="disable2FAForm.password" type="password" show-password placeholder="请输入当前密码" />
              </el-form-item>
              <el-form-item label="2FA 验证码">
                <el-input v-model="disable2FAForm.code" maxlength="6" show-word-limit placeholder="请输入 6 位验证码" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="regenRecoveryLoading" @click="handleRegenRecovery">重新生成恢复码</el-button>
              </el-form-item>
            </el-form>
          </template>

          <template v-else>
            <div class="totp-actions">
              <el-button @click="handleGenerate2FA" :loading="totpLoading">生成 2FA 配置</el-button>
            </div>
            <div v-if="totpSetup.secret" class="totp-panel">
              <img :src="totpSetup.qrCodeData" alt="2FA QR" class="qr-image" />
              <p class="totp-secret">密钥：{{ totpSetup.secret }}</p>
              <div class="security-tip">
                请使用支持 TOTP 的验证器应用扫描二维码，输入 6 位动态验证码完成绑定。
              </div>
              <el-input v-model="totpSetup.code" maxlength="6" show-word-limit placeholder="请输入 6 位验证码" />
              <el-button type="primary" style="margin-top: 12px;" :loading="enable2FALoading" @click="handleEnable2FA">启用 2FA</el-button>
            </div>
          </template>
        </el-tab-pane>

        <el-tab-pane label="API" name="api">
          <el-alert
            title="API Key 可用于外部程序调用面板接口，请只保存在可信环境中。重新生成后旧 Key 会立即失效。"
            type="warning"
            :closable="false"
            style="margin-bottom: 16px;"
          />
          <el-form v-loading="apiKeyLoading" label-width="110px" class="security-form">
            <el-form-item label="状态">
              <el-tag :type="apiKeyInfo?.enabled ? 'success' : 'info'">
                {{ apiKeyInfo?.enabled ? '已启用' : '未生成' }}
              </el-tag>
            </el-form-item>
            <el-form-item label="API ID">
              <el-input :model-value="apiKeyInfo?.api_key_id || '未生成'" disabled>
                <template #append>
                  <el-button :disabled="!apiKeyInfo?.api_key_id" @click="copySecurityText(apiKeyInfo.api_key_id)">复制</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="Key 标识">
              <el-input :model-value="apiKeyInfo?.key_prefix || '未生成'" disabled />
            </el-form-item>
            <el-form-item label="创建时间">
              <el-input :model-value="formatDateTime(apiKeyInfo?.created_at)" disabled />
            </el-form-item>
            <el-form-item label="最后使用">
              <el-input :model-value="formatDateTime(apiKeyInfo?.last_used_at)" disabled />
            </el-form-item>
            <el-form-item v-if="generatedAPIKey" label="API Key">
              <el-input :model-value="generatedAPIKey" type="password" show-password readonly>
                <template #append>
                  <el-button @click="copySecurityText(generatedAPIKey)">复制</el-button>
                </template>
              </el-input>
              <div class="security-tip">
                API Key 只会在本次生成后显示一次，关闭窗口后无法再次查看。
              </div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="apiKeyGenerating" @click="handleRotateAPIKey">
                {{ apiKeyInfo?.enabled ? '重新生成' : '生成 Key 和 ID' }}
              </el-button>
              <el-button :disabled="!apiKeyInfo?.enabled" :loading="apiKeyRevoking" type="danger" plain @click="handleRevokeAPIKey">
                撤销
              </el-button>
              <el-button @click="openAPIDocs">接口文档</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 修改密码 Tab -->
        <el-tab-pane label="修改密码" name="password">
          <el-form
            ref="passwordFormRef"
            :model="passwordForm"
            :rules="passwordRules"
            label-width="100px"
            class="security-form"
          >
            <el-form-item label="当前密码" prop="oldPassword">
              <el-input
                v-model="passwordForm.oldPassword"
                type="password"
                show-password
                placeholder="请输入当前密码"
              />
            </el-form-item>
            <el-form-item label="新密码" prop="newPassword">
              <el-input
                v-model="passwordForm.newPassword"
                type="password"
                show-password
                placeholder="请输入新密码（至少12位）"
              />
            </el-form-item>
            <el-form-item label="确认密码" prop="confirmPassword">
              <el-input
                v-model="passwordForm.confirmPassword"
                type="password"
                show-password
                placeholder="请再次输入新密码"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="submitPasswordChange" :loading="passwordLoading">
                确认修改
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 修改用户名 Tab -->
        <el-tab-pane label="修改用户名" name="username">
          <el-form
            ref="usernameFormRef"
            :model="usernameForm"
            :rules="usernameRules"
            label-width="100px"
            class="security-form"
          >
            <el-form-item label="当前用户名">
              <el-input :model-value="userStore.username" disabled />
            </el-form-item>
            <el-form-item label="新用户名" prop="newUsername">
              <el-input
                v-model="usernameForm.newUsername"
                placeholder="请输入新用户名（3-32个字符）"
              />
            </el-form-item>
            <el-form-item label="确认密码" prop="password">
              <el-input
                v-model="usernameForm.password"
                type="password"
                show-password
                placeholder="请输入密码以确认身份"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="submitUsernameChange" :loading="usernameLoading">
                确认修改
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

  </el-container>
</template>

<script setup>
import { computed, ref, reactive, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import QRCode from 'qrcode'
import { useUserStore } from '@/store/user'
import { getAPIKeyInfo, revokeAPIKey, rotateAPIKey } from '@/api/apiKey'
import { bindEmail, changePassword, changeUsername, disable2FA, enable2FA, getUserInfo, regenRecoveryCodes, sendEmailCode, setup2FA } from '@/api/auth'
import SidebarIcons from '@/components/icons/SidebarIcons.vue'
import {
  ArrowDown,
  Close,
  Expand,
  Fold,
  Link,
  List,
  Moon,
  Sunny,
  SwitchButton,
  UserFilled
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { copyTextWithFallback } from '@/utils/clipboard'
import { siteTitle } from '@/utils/site'
import { menuLayoutRaw } from '@/utils/site'
import { composeMenu } from '@/utils/menu'
import { passwordValidator, checkPasswordBreachAsync } from '@/utils/validate'

// 导入近期任务面板组件
import RecentTaskPanel from '@/components/RecentTaskPanel.vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const displaySiteTitle = computed(() => siteTitle.value)

const isAdmin = computed(() => userStore.role === 'admin')
const isLightweight = computed(() => userStore.role !== 'admin' && userStore.cloudType === 'lightweight')

const menuNodes = computed(() =>
  composeMenu(menuLayoutRaw.value, { isAdmin: isAdmin.value, isLightweight: isLightweight.value })
)

const activeMenu = computed(() => route.path)

// 响应式和侧边栏逻辑
const isCollapse = ref(false)
const isMobile = ref(false)

const checkMobile = () => {
  const isCurrentlyMobile = window.innerWidth <= 768
  if (isCurrentlyMobile !== isMobile.value) {
    isMobile.value = isCurrentlyMobile
    isCollapse.value = isCurrentlyMobile // 移动端默认收起
  }
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  // 暗黑模式检测
  if (isDark.value) {
    document.documentElement.classList.add('dark')
  }
  refreshSecurityInfo()

  // 监听弹窗打开，自动收起异步任务面板
  dialogObserver = new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      for (const node of mutation.addedNodes) {
        if (node.nodeType === 1 && (
          node.classList?.contains('el-overlay') ||
          node.classList?.contains('el-message-box__wrapper') ||
          node.classList?.contains('el-overlay-message-box')
        )) {
          recentTaskPanelRef.value?.collapse()
          return
        }
      }
    }
  })
  dialogObserver.observe(document.body, { childList: true, subtree: false })
})

let dialogObserver = null

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  if (dialogObserver) {
    dialogObserver.disconnect()
    dialogObserver = null
  }
})

const toggleCollapse = () => {
  isCollapse.value = !isCollapse.value
}

const handleMenuSelect = () => {
  if (isMobile.value) {
    isCollapse.value = true // 移动端点击菜单后自动收起
  }
}

// 暗黑模式逻辑
const isDark = ref(localStorage.getItem('theme-dark') === 'true')
const toggleDark = (val) => {
  if (val) {
    document.documentElement.classList.add('dark')
    localStorage.setItem('theme-dark', 'true')
  } else {
    document.documentElement.classList.remove('dark')
    localStorage.setItem('theme-dark', 'false')
  }
}

// ==================== 安全设置相关 ====================
const securityDialogVisible = ref(false)
const securityTab = ref('password')
const emailCodeLoading = ref(false)
const emailBindingLoading = ref(false)
const emailForm = reactive({
  email: '',
  code: '',
  challenge_id: 0
})
const totpLoading = ref(false)
const enable2FALoading = ref(false)
const disable2FALoading = ref(false)
const totpSetup = reactive({
  secret: '',
  otpauth_url: '',
  qrCodeData: '',
  code: ''
})
const disable2FAForm = reactive({
  password: '',
  code: ''
})
const recoveryCodes = ref([])
const regenRecoveryLoading = ref(false)
const apiKeyInfo = ref(null)
const generatedAPIKey = ref('')
const apiKeyLoading = ref(false)
const apiKeyGenerating = ref(false)
const apiKeyRevoking = ref(false)

// ==================== 异步任务面板 ====================
const recentTaskPanelRef = ref(null)
const activeTaskCount = ref(0)
const isLogged = computed(() => !!localStorage.getItem('token'))

const toggleRecentTaskPanel = () => {
  recentTaskPanelRef.value?.toggleExpand()
}

const openFullTaskCenter = () => {
  router.push('/task/recent')
}

const refreshSecurityInfo = async () => {
  try {
    const res = await getUserInfo()
    userStore.setUserInfo(res.data.username, res.data.role, res.data.security || null, res.data.cloud_type || 'elastic')
    if (
      userStore.role === 'user' &&
      !userStore.security?.development_mode &&
      !userStore.security?.totp_enabled &&
      !sessionStorage.getItem('2fa_recommended')
    ) {
      sessionStorage.setItem('2fa_recommended', '1')
      ElMessageBox.confirm('建议尽快绑定 2FA 以增强账户安全，是否现在前往安全设置？', '安全提示', {
        confirmButtonText: '立即设置',
        cancelButtonText: '稍后',
        type: 'warning'
      }).then(() => {
        handleCommand('security')
      }).catch(() => {})
    }
  } catch (err) {
    // 交给请求拦截器处理
  }
}

// 修改密码
const passwordFormRef = ref(null)
const passwordLoading = ref(false)
const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const passwordRules = {
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
        if (value !== passwordForm.newPassword) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

// 修改用户名
const usernameFormRef = ref(null)
const usernameLoading = ref(false)
const usernameForm = reactive({
  newUsername: '',
  password: ''
})

const usernameRules = {
  newUsername: [
    { required: true, message: '请输入新用户名', trigger: 'blur' },
    { min: 3, max: 32, message: '用户名长度在3-32个字符之间', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码以确认身份', trigger: 'blur' }
  ]
}

// 提交修改密码
const submitPasswordChange = async () => {
  const valid = await passwordFormRef.value.validate().catch(() => false)
  if (!valid) return

  // 异步泄露密码检测（HIBP API）
  const breach = await checkPasswordBreachAsync(passwordForm.newPassword)
  if (breach.enabled && breach.breached) {
    ElMessage.error('该密码已在已知泄露数据库中发现，请更换为更安全的密码')
    return
  }

  passwordLoading.value = true
  try {
    const res = await changePassword({
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword
    })
    ElMessage.success(res.message || '密码修改成功，请重新登录')
    securityDialogVisible.value = false
    // 密码修改成功后需要重新登录
    userStore.logout()
    router.push('/login')
  } catch (err) {
    // 拦截器已经弹出了错误提示，这里不再重复
  } finally {
    passwordLoading.value = false
  }
}

// 提交修改用户名
const submitUsernameChange = async () => {
  const valid = await usernameFormRef.value.validate().catch(() => false)
  if (!valid) return

  usernameLoading.value = true
  try {
    const res = await changeUsername({
      new_username: usernameForm.newUsername,
      password: usernameForm.password
    })
    const { token, username } = res.data
    // 更新本地存储的 token 和用户名
    userStore.setToken(token)
    userStore.setUserInfo(username, userStore.role, userStore.security, userStore.cloudType)
    ElMessage.success(res.message || '用户名修改成功')
    securityDialogVisible.value = false
    // 重置表单
    usernameForm.newUsername = ''
    usernameForm.password = ''
  } catch (err) {
    // 拦截器已经弹出了错误提示，这里不再重复
  } finally {
    usernameLoading.value = false
  }
}

const handleSendEmailCode = async () => {
  if (!emailForm.email) {
    ElMessage.warning('请输入要绑定的邮箱')
    return
  }
  emailCodeLoading.value = true
  try {
    const res = await sendEmailCode({ email: emailForm.email })
    emailForm.challenge_id = res.data.challenge_id
    ElMessage.success('验证码已发送')
  } finally {
    emailCodeLoading.value = false
  }
}

const handleBindEmail = async () => {
  if (!emailForm.challenge_id) {
    ElMessage.warning('请先发送邮箱验证码')
    return
  }
  emailBindingLoading.value = true
  try {
    await bindEmail({
      email: emailForm.email,
      code: emailForm.code,
      challenge_id: emailForm.challenge_id
    })
    await refreshSecurityInfo()
    ElMessage.success('邮箱已更新')
  } finally {
    emailBindingLoading.value = false
  }
}

const handleGenerate2FA = async () => {
  totpLoading.value = true
  try {
    const res = await setup2FA()
    totpSetup.secret = res.data.secret
    totpSetup.otpauth_url = res.data.otpauth_url
    totpSetup.qrCodeData = await QRCode.toDataURL(res.data.otpauth_url)
    totpSetup.code = ''
  } finally {
    totpLoading.value = false
  }
}

const handleEnable2FA = async () => {
  if (!totpSetup.secret) {
    ElMessage.warning('请先生成 2FA 配置')
    return
  }
  enable2FALoading.value = true
  try {
    const res = await enable2FA({
      secret: totpSetup.secret,
      code: totpSetup.code
    })
    if (res.recovery?.recovery_codes?.length) {
      recoveryCodes.value = res.recovery.recovery_codes
      ElMessageBox.alert(
        formatRecoveryCodesMessage(res.recovery.recovery_codes),
        '请保存恢复码',
        {
          dangerouslyUseHTMLString: true,
          confirmButtonText: '我已安全保存',
          type: 'warning',
          beforeClose: (action, instance, done) => {
            recoveryCodes.value = []
            done()
          }
        }
      )
    }
    await refreshSecurityInfo()
    totpSetup.secret = ''
    totpSetup.otpauth_url = ''
    totpSetup.qrCodeData = ''
    totpSetup.code = ''
    ElMessage.success('2FA 已启用')
  } finally {
    enable2FALoading.value = false
  }
}

const handleDisable2FA = async () => {
  disable2FALoading.value = true
  try {
    await disable2FA({
      password: disable2FAForm.password,
      code: disable2FAForm.code
    })
    await refreshSecurityInfo()
    disable2FAForm.password = ''
    disable2FAForm.code = ''
    ElMessage.success('2FA 已关闭')
  } finally {
    disable2FALoading.value = false
  }
}

const formatRecoveryCodesMessage = (codes) => {
  const codeItems = codes.map((c, i) => `<div style="font-family:monospace;padding:2px 0;">${String(i + 1).padStart(2, '0')}. <b>${c}</b></div>`).join('')
  return `<div style="font-size:14px;"><p style="color:#e53e3e;font-weight:bold;margin-bottom:8px;">以下恢复码仅在本次显示，请立即复制/保存：</p><div style="background:#f5f5f5;padding:12px;border-radius:4px;margin-bottom:8px;">${codeItems}</div><p style="font-size:12px;color:#666;">当 2FA 验证器不可用时，可使用恢复码登录。每个码只能使用一次。</p></div>`
}

const handleRegenRecovery = async () => {
  if (!disable2FAForm.password || !disable2FAForm.code) {
    ElMessage.warning('请先输入当前密码和 2FA 验证码')
    return
  }
  regenRecoveryLoading.value = true
  try {
    const res = await regenRecoveryCodes({
      password: disable2FAForm.password,
      code: disable2FAForm.code
    })
    if (res.recovery?.recovery_codes?.length) {
      recoveryCodes.value = res.recovery.recovery_codes
      ElMessageBox.alert(
        formatRecoveryCodesMessage(res.recovery.recovery_codes),
        '新的恢复码',
        {
          dangerouslyUseHTMLString: true,
          confirmButtonText: '我已安全保存',
          type: 'warning',
          beforeClose: () => {
            recoveryCodes.value = []
            disable2FAForm.password = ''
            disable2FAForm.code = ''
          }
        }
      )
    }
    await refreshSecurityInfo()
  } finally {
    regenRecoveryLoading.value = false
  }
}

const formatDateTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

const loadAPIKeyInfo = async () => {
  apiKeyLoading.value = true
  try {
    const res = await getAPIKeyInfo()
    apiKeyInfo.value = res.data || null
  } finally {
    apiKeyLoading.value = false
  }
}

const handleRotateAPIKey = async () => {
  try {
    await ElMessageBox.confirm(
      apiKeyInfo.value?.enabled
        ? '确定要重新生成 API Key 吗？旧 Key 会立即失效。'
        : '确定要生成 API Key 和 ID 吗？生成后请立即复制保存 Key。',
      'API 凭证',
      {
        confirmButtonText: apiKeyInfo.value?.enabled ? '重新生成' : '生成',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
  } catch (action) {
    if (action === 'cancel' || action === 'close') return
    throw action
  }
  apiKeyGenerating.value = true
  try {
    const res = await rotateAPIKey()
    apiKeyInfo.value = res.data || null
    generatedAPIKey.value = res.data?.api_key || ''
    ElMessage.success(res.message || 'API 凭证已生成')
  } finally {
    apiKeyGenerating.value = false
  }
}

const handleRevokeAPIKey = async () => {
  try {
    await ElMessageBox.confirm('确定要撤销当前 API Key 吗？撤销后外部程序将无法继续调用接口。', '撤销 API 凭证', {
      confirmButtonText: '撤销',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch (action) {
    if (action === 'cancel' || action === 'close') return
    throw action
  }
  apiKeyRevoking.value = true
  try {
    await revokeAPIKey()
    generatedAPIKey.value = ''
    await loadAPIKeyInfo()
    ElMessage.success('API 凭证已撤销')
  } finally {
    apiKeyRevoking.value = false
  }
}

const copySecurityText = async (text) => {
  try {
    await copyTextWithFallback(text)
    ElMessage.success('已复制')
  } catch (err) {
    ElMessage.error(err.message || '复制失败')
  }
}

const openAPIDocs = () => {
  securityDialogVisible.value = false
  router.push('/api-docs')
}

watch(securityTab, (tab) => {
  if (tab === 'api') {
    loadAPIKeyInfo()
  }
})

// 下拉菜单命令处理
const handleCommand = (command) => {
  if (command === 'logout') {
    userStore.logout()
    router.push('/login')
  } else if (command === 'security') {
    // 重置表单数据
    emailForm.email = userStore.security?.email || ''
    emailForm.code = ''
    emailForm.challenge_id = 0
    totpSetup.secret = ''
    totpSetup.otpauth_url = ''
    totpSetup.qrCodeData = ''
    totpSetup.code = ''
    disable2FAForm.password = ''
    disable2FAForm.code = ''
    passwordForm.oldPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''
    usernameForm.newUsername = ''
    usernameForm.password = ''
    securityTab.value = 'email'
    refreshSecurityInfo()
    loadAPIKeyInfo()
    securityDialogVisible.value = true
  }
}

</script>

<style scoped>
.app-wrapper {
  height: 100vh;
  width: 100%;
}

/* ===== 侧边栏优化 ===== */
.sidebar {
  background-color: var(--el-bg-color-overlay);
  border-right: 1px solid var(--app-border-light, var(--el-border-color-light));
  transition: width 0.28s cubic-bezier(0.4, 0, 0.2, 1), transform 0.28s cubic-bezier(0.4, 0, 0.2, 1);
  overflow-x: hidden;
  overflow-y: auto;
  z-index: 1001;
}

.mobile-sidebar-close {
  position: absolute;
  right: 14px;
  top: 50%;
  transform: translateY(-50%);
  cursor: pointer;
  font-size: 20px;
  color: var(--el-text-color-regular);
  display: none;
  transition: color var(--app-transition-fast, 0.15s ease);
}

.mobile-sidebar-close:hover {
  color: var(--el-text-color-primary);
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 0 12px;
  color: var(--el-text-color-primary);
  font-size: 16px;
  font-weight: 700;
  background-color: var(--el-bg-color-overlay);
  border-bottom: 1px solid var(--app-border-light, var(--el-border-color-light));
  white-space: nowrap;
  overflow: hidden;
  position: relative;
  letter-spacing: -0.01em;
}

.sidebar-logo {
  width: 42px;
  height: 42px;
  flex-shrink: 0;
  object-fit: contain;
}

.el-menu-vertical {
  border-right: none;
}

.el-menu-vertical .sidebar-icon,
.el-menu-vertical .el-menu-item .sidebar-icon,
.el-menu-vertical .el-sub-menu .sidebar-icon {
  font-size: 18px;
  margin-right: 5px;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.el-menu--collapse .sidebar-icon {
  display: inline-flex !important;
  height: auto !important;
  width: auto !important;
  overflow: visible !important;
  visibility: visible !important;
}

/* ===== 主容器优化 ===== */
.main-container {
  display: flex;
  flex-direction: column;
  background-color: var(--app-bg-page, var(--el-bg-color-page));
}

/* ===== 导航栏 - 玻璃态效果 ===== */
.navbar {
  height: 60px;
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: var(--app-navbar-blur, blur(12px));
  -webkit-backdrop-filter: var(--app-navbar-blur, blur(12px));
  border-bottom: 1px solid var(--app-border-light, var(--el-border-color-light));
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 24px;
  z-index: 999;
  position: sticky;
  top: 0;
  transition: background var(--app-transition-base, 0.25s ease);
}

html.dark .navbar {
  background: rgba(30, 30, 34, 0.88);
}

/* ===== 导航栏左侧 ===== */
.left-menu {
  display: flex;
  align-items: center;
  min-width: 0;
}

.fold-btn {
  font-size: 20px;
  cursor: pointer;
  margin-right: 16px;
  color: var(--el-text-color-regular);
  transition: color var(--app-transition-fast, 0.15s ease), transform var(--app-transition-fast, 0.15s ease);
  display: flex;
  align-items: center;
  padding: 4px;
  border-radius: 6px;
}

.fold-btn:hover {
  color: var(--el-color-primary);
  background: var(--app-bg-hover, rgba(64, 158, 255, 0.04));
}

.page-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  letter-spacing: -0.01em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* ===== 导航栏右侧 ===== */
.right-menu {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.user-info {
  display: flex;
  align-items: center;
  cursor: pointer;
  padding: 6px 10px;
  border-radius: 8px;
  transition: background var(--app-transition-fast, 0.15s ease);
}

.user-info:hover {
  background: var(--app-bg-hover, rgba(64, 158, 255, 0.04));
}

.username {
  margin-left: 8px;
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

/* ===== 主内容区优化 ===== */
.app-main {
  padding: 24px;
  background-color: var(--app-bg-page, var(--el-bg-color-page));
  overflow-y: auto;
  flex: 1 1 0%;
  min-height: 0;
  position: relative;
}

/* 内容区顶部微妙渐变，增强层次感 */
.app-main::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 80px;
  background: var(--app-gradient-subtle, linear-gradient(180deg, rgba(64, 158, 255, 0.02) 0%, transparent 100%));
  pointer-events: none;
  z-index: 0;
}

.app-main > * {
  position: relative;
  z-index: 1;
}

/* ===== 页面过渡动画 ===== */
.fade-transform-enter-active,
.fade-transform-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.fade-transform-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}

.fade-transform-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

/* ===== 安全设置对话框优化 ===== */
.security-tabs {
  margin-top: -10px;
}

.security-form {
  padding: 20px 20px 0 0;
}

.security-form :deep(.el-form-item:last-child) {
  margin-bottom: 0;
  margin-top: 10px;
}

.security-tip {
  margin: -4px 0 14px 110px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
  display: flex;
  align-items: center;
  gap: 4px;
}

.totp-actions {
  margin-bottom: 12px;
}

.totp-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px;
  background: var(--el-fill-color-light);
  border-radius: var(--app-radius-lg, 14px);
  border: 1px solid var(--app-border-extralight, var(--el-border-color-extra-light));
}

.qr-image {
  width: 180px;
  height: 180px;
  padding: 10px;
  border-radius: var(--app-radius-lg, 14px);
  background: #fff;
  box-shadow: var(--app-shadow-lg, 0 8px 18px rgba(15, 23, 42, 0.08));
}

.totp-secret {
  margin: 14px 0 10px;
  color: var(--el-text-color-primary);
  word-break: break-all;
  font-family: 'SF Mono', SFMono-Regular, Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 13px;
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .sidebar {
    position: fixed;
    height: 100vh;
    left: 0;
    top: 0;
    transform: translateX(-100%);
    box-shadow: 4px 0 20px rgba(0, 0, 0, 0.12);
  }

  .sidebar.mobile-show {
    transform: translateX(0);
  }

  .mobile-sidebar-close {
    display: inline-flex;
  }

  .mobile-mask {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.45);
    backdrop-filter: blur(2px);
    -webkit-backdrop-filter: blur(2px);
    z-index: 1000;
  }

  .security-tip {
    margin-left: 0;
  }

  .navbar {
    padding: 0 14px !important;
    height: 52px !important;
  }

  .page-title {
    font-size: 14px !important;
  }

  .username {
    display: none;
  }

  .dark-switch {
    margin-right: 8px;
  }

  .cloud-tag {
    margin-right: 8px;
  }

  .app-main {
    padding: 14px;
  }

  .right-menu {
    gap: 2px;
  }
}

/* ===== 异步任务面板导航栏入口 ===== */
.task-badge :deep(.el-badge__content) {
  top: 4px;
  right: 8px;
}

.task-toggle-btn {
  font-size: 18px;
  color: var(--el-text-color-regular);
  transition: color var(--app-transition-fast, 0.15s ease);
  display: flex;
  align-items: center;
  padding: 4px;
  border-radius: 6px;
}

.task-toggle-btn:hover {
  color: var(--el-color-primary);
  background: var(--app-bg-hover, rgba(64, 158, 255, 0.04));
}

/* ===== 开源版链接 ===== */
.oss-link {
  margin-right: 8px;
  font-size: 13px;
  font-weight: 500;
}

/* 异步任务面板隐藏（未登录时） */
.task-panel-hidden {
  display: none !important;
}

</style>
