import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '@/store/user'
import router from '@/router'
import NProgress from 'nprogress'

const baseURL = import.meta.env.VITE_APP_BASE_API || '/api'

let requestCount = 0
const startLoading = () => {
  if (requestCount === 0) {
    NProgress.start()
  }
  requestCount++
}
const stopLoading = () => {
  if (requestCount > 0) {
    requestCount--
  }
  if (requestCount === 0) {
    NProgress.done()
  }
}

const service = axios.create({
  baseURL,
  timeout: 60000
})

const rawClient = axios.create({
  baseURL,
  timeout: 60000
})

function createHighRiskCancelledError(action = 'cancel') {
  const error = new Error('已取消高风险验证')
  error.isHighRiskCancelled = true
  error.highRiskAction = action
  return error
}

function isHighRiskCancelledError(error) {
  return !!error?.isHighRiskCancelled
}

service.interceptors.request.use(
  config => {
    if (!config.silent) {
      startLoading()
    }
    const userStore = useUserStore()
    config.headers = config.headers || {}
    if (userStore.token && !config.headers.Authorization) {
      config.headers.Authorization = `Bearer ${userStore.token}`
    }
    return config
  },
  error => {
    stopLoading()
    return Promise.reject(error)
  }
)

async function promptHighRiskChallenge(data) {
  const method = data?.method
  if (method === 'totp') {
    const hasRecovery = data?.has_recovery
    const recoveryHint = hasRecovery ? '\n如果您无法使用 2FA 验证器，可使用恢复码。' : ''
    const result = await ElMessageBox.prompt(
      `请输入 2FA 验证码${recoveryHint}`,
      '高风险验证',
      {
        confirmButtonText: '验证',
        cancelButtonText: '取消',
        inputPattern: /^\S{6,16}$/,
        inputErrorMessage: '请输入 6 位验证码或 16 位恢复码'
      }
    )
    const inputValue = result.value?.trim() || ''
    // 自动判断是 6 位 TOTP 验证码还是 16 位恢复码
    if (hasRecovery && inputValue.length >= 16) {
      return {
        method: 'recovery',
        code: inputValue,
        operation: data.operation
      }
    }
    return {
      method: 'totp',
      code: inputValue,
      operation: data.operation
    }
  }

  const result = await ElMessageBox.prompt(
    `验证码已发送至 ${data?.masked_email || '您的邮箱'}，请输入邮箱验证码`,
    '邮箱验证',
    {
      confirmButtonText: '验证',
      cancelButtonText: '取消',
      inputPattern: /^\d{6}$/,
      inputErrorMessage: '请输入 6 位验证码'
    }
  )
  return {
    method: 'email',
    code: result.value,
    challenge_id: data?.challenge_id,
    operation: data?.operation
  }
}

async function verifyHighRiskChallenge(payload, authHeader) {
  const response = await rawClient.post('/auth/high-risk/verify', payload, {
    headers: authHeader ? { Authorization: authHeader } : {},
    silent: true
  })
  return response.data
}

let highRiskLock = false

async function handleHighRisk(error) {
  if (highRiskLock) {
    // 已有验证弹窗在进行中，取消本次请求避免双弹窗
    throw createHighRiskCancelledError('concurrent')
  }

  const originalConfig = error.config
  const responseData = error.response?.data?.data
  if (!originalConfig || originalConfig._highRiskRetried || originalConfig.skipHighRiskHandler) {
    throw error
  }

  highRiskLock = true
  const authHeader = originalConfig.headers?.Authorization || (useUserStore().token ? `Bearer ${useUserStore().token}` : '')
  let verifyPayload
  try {
    verifyPayload = await promptHighRiskChallenge(responseData)
  } catch (promptAction) {
    highRiskLock = false
    if (promptAction === 'cancel' || promptAction === 'close') {
      throw createHighRiskCancelledError(promptAction)
    }
    throw promptAction
  }
  try {
    const verifyRes = await verifyHighRiskChallenge(verifyPayload, authHeader)
    originalConfig._highRiskRetried = true
    originalConfig.headers = originalConfig.headers || {}
    if (verifyRes?.data?.verification_token) {
      originalConfig.headers['X-High-Risk-Token'] = verifyRes.data.verification_token
    }
    return service(originalConfig)
  } finally {
    highRiskLock = false
  }
}

service.interceptors.response.use(
  response => {
    if (!response.config.silent) {
      stopLoading()
    }
    const res = response.data
    if (res.code !== 200 && res.code !== 0) {
      if (!response.config?.skipErrorHandler) {
        ElMessage({
          message: res.message || '请求失败',
          type: 'error',
          duration: 5000
        })
      }
      if (res.code === 401) {
        const userStore = useUserStore()
        userStore.logout()
        if ((res.message || '').includes('登录环境发生变化')) {
          ElMessage.warning('登录环境发生变化，请重新登录')
        }
        router.push('/login')
      }
      return Promise.reject(new Error(res.message || '请求失败'))
    }
    return res
  },
  async error => {
    if (!error.config || !error.config.silent) {
      stopLoading()
    }

    if (error.response?.status === 428) {
      try {
        return await handleHighRisk(error)
      } catch (challengeError) {
        if (!isHighRiskCancelledError(challengeError) && challengeError !== 'cancel' && challengeError !== 'close' && challengeError?.message) {
          ElMessage({
            message: challengeError.response?.data?.message || challengeError.message || '验证失败',
            type: 'error',
            duration: 5000
          })
        }
        return Promise.reject(challengeError)
      }
    }

    const serverMessage = error.response?.data?.message
    const displayMessage = serverMessage || error.message || '请求失败'
    if (!error.response?.data?.data?.require_nvram_fix && !error.config?.skipErrorHandler) {
      ElMessage({
        message: displayMessage,
        type: 'error',
        duration: 5000
      })
    }

    if (error.response?.status === 401) {
      const userStore = useUserStore()
      const serverMsg = error.response?.data?.message || ''
      userStore.logout()
      if (serverMsg.includes('登录环境发生变化')) {
        ElMessage.warning('登录环境发生变化，请重新登录')
      }
      router.push('/login')
    }

    return Promise.reject(error)
  }
)

export default service
