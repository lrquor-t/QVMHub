import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '@/store/user'
import { useNodeStore } from '@/store/node'
import router from '@/router'
import NProgress from 'nprogress'

const baseURL = import.meta.env.VITE_APP_BASE_API || '/api'

// QVMHub 控制器自有路径(不经节点代理):auth/nodes/overview/public/settings。
// 其余业务路径(vm/lxc/host/network/storage/task/...)按选中节点拼 /api/n/{nodeId} 前缀。
const CONTROLLER_NATIVE_PREFIXES = ['/auth', '/nodes', '/overview', '/public', '/settings']

function resolveNodePrefix(url) {
  if (!url) return ''
  if (url.startsWith('http://') || url.startsWith('https://')) return '' // 绝对地址不动
  for (const p of CONTROLLER_NATIVE_PREFIXES) {
    if (url === p || url.startsWith(p + '/')) return ''
  }
  const nodeStore = useNodeStore()
  const id = nodeStore.selectedNodeId
  if (!id) return ''
  // 设计 §3.1:最终 URL 为 /api/n/{id}/api/<业务路径>,代理剥 /api/n/{id} 后
  // 转发到节点 /api/<业务路径>。此处补 "/api"(节点侧路由前缀必须保留)。
  return '/n/' + id + '/api'
}

// sseURL 为浏览器原生 EventSource 构造节点感知的完整 URL(EventSource 不走 axios 拦截器)。
// path 形如 "/host/stats/sse";query 形如 "token=xxx"(可空)。
export function sseURL(path, query = '') {
  const nodeStore = useNodeStore()
  const id = nodeStore.selectedNodeId
  const prefix = id ? '/n/' + id + '/api' : ''
  const qs = query ? (query.startsWith('?') ? query : '?' + query) : ''
  return baseURL + prefix + path + qs
}

// wsURL 为浏览器原生 WebSocket 构造节点感知的绝对地址(与 sseURL 同理,但产出 ws(s)://host/api/...)。
// 控制台 WS(VNC/LXC 终端)同样经控制器 /api/n/{id}/api 中继到节点(§6.1/§6.3)。
// path 形如 "/vm/<name>/vnc/ws";query 形如 "token=xxx"(可空)。
export function wsURL(path, query = '') {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const nodeStore = useNodeStore()
  const id = nodeStore.selectedNodeId
  const prefix = id ? '/n/' + id + '/api' : ''
  const qs = query ? (query.startsWith('?') ? query : '?' + query) : ''
  return `${protocol}//${host}${baseURL}${prefix}${path}${qs}`
}


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
    // QVMHub:业务请求按选中节点拼前缀(/api/n/{nodeId}/...)。
    const prefix = resolveNodePrefix(config.url)
    if (prefix) {
      config.url = prefix + config.url
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
