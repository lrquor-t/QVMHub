/**
 * 统一密码验证工具
 * 所有需要校验密码的地方都应调用此函数，确保验证规则一致。
 * 前端仅做本地常见弱密码快速检测 + 后端 API 异步泄露检测。
 * 最终校验以后端 ValidateStrongPassword 为准。
 */

import { checkPasswordBreach } from '@/api/auth'

const STRONG_PASSWORD_MIN_LENGTH = 12
const PASSWORD_ALLOWED_PATTERN = /^[A-Za-z0-9!@#$%^&*_\-+=?]+$/

/**
 * 本地常见弱密码列表（快速检测，无需网络请求）
 * 与后端 common_passwords.go 保持同步，此处仅保留最高频的子集
 */
const COMMON_PASSWORDS = new Set([
  // 数字类
  '123456', '123456789', '12345678', '1234567', '1234567890',
  '12345', '123123', '1234', '111111', '000000', '121212',
  '666666', '888888', '987654321', '654321', '54321', '696969',
  '112233', '00000000', '11111111', '222222', '333333', '456789',
  // 字母类
  'password', 'Password', 'PASSWORD', 'password1', 'Password1',
  'admin', 'Admin', 'ADMIN', 'root', 'Root', 'ROOT',
  'qwerty', 'Qwerty', 'qwerty123', 'abc123', 'Abc123',
  'test', 'Test', 'guest', 'Guest', 'user', 'User',
  'welcome', 'Welcome', 'letmein', 'monkey', 'dragon',
  'master', 'shadow', 'sunshine', 'princess', 'football',
  'iloveyou', 'trustno1', 'superman', 'batman', 'hello',
  // 混合类
  'passw0rd', 'P@ssw0rd', 'P@ssword', 'P@ssw0rd1', 'P@ssword1',
  'admin123', 'Admin123', 'root123', 'test123', 'Test123',
  'abc12345', 'password123', 'Password123', 'qwerty1',
  '1q2w3e', '1q2w3e4r', '1qaz2wsx', 'qazwsx', 'changeme',
  // 中文拼音/数字谐音
  'woaini', 'woaini520', 'woaini1314', '5201314', '1314520',
  'mima', 'mima123',
  // 键盘图案
  'qwertyuiop', 'asdfghjkl', 'zxcvbnm', 'asdfgh', 'zxcvbn',
])

/**
 * 本地快速检测：密码是否在常见弱密码列表中
 * @param {string} password - 待检测密码
 * @returns {boolean}
 */
function isCommonPassword(password) {
  if (!password) return false
  if (COMMON_PASSWORDS.has(password)) return true
  // 尝试全小写匹配
  if (COMMON_PASSWORDS.has(password.toLowerCase())) return true
  return false
}

/**
 * 校验密码（本地快速检测，不阻塞）
 * 前端不做强制限制，最终校验由后端完成。
 * @param {string} password - 待校验密码
 * @returns {{ valid: boolean, message: string }} 校验结果
 */
export function validatePassword(password) {
  if (isCommonPassword(password)) {
    return { valid: false, message: '该密码过于常见，请更换为更安全的密码' }
  }
  return { valid: true, message: '' }
}

/**
 * Element Plus 表单 validator（本地快速检测）
 */
export function passwordValidator(_rule, value, callback) {
  const result = validatePassword(value)
  if (!result.valid) {
    callback(new Error(result.message))
  } else {
    callback()
  }
}

/**
 * 异步检测密码是否在泄露数据库中
 * 调用后端 API，后端通过 HIBP API（k-匿名性）+ 本地常见密码列表检测
 * @param {string} password - 待检测密码
 * @returns {Promise<{ enabled: boolean, breached: boolean, warning?: string }>}
 */
export async function checkPasswordBreachAsync(password) {
  if (!password) {
    return { enabled: true, breached: false }
  }
  try {
    const res = await checkPasswordBreach(password)
    const data = res.data || {}
    return {
      enabled: data.enabled !== false,
      breached: !!data.breached,
      warning: data.warning || '',
    }
  } catch {
    // 网络错误时不阻断，仅返回未泄露
    return { enabled: true, breached: false, warning: '检测服务暂时不可用' }
  }
}

/**
 * 生成随机强密码
 * @param {number} length - 密码长度，默认 16
 * @returns {string}
 */
export function generatePassword(length = 16) {
  const charset = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%^&*_-+=?'
  const randomValues = new Uint32Array(length)
  window.crypto.getRandomValues(randomValues)
  let result = ''
  for (let i = 0; i < length; i++) {
    result += charset[randomValues[i] % charset.length]
  }
  return result
}

export { STRONG_PASSWORD_MIN_LENGTH, PASSWORD_ALLOWED_PATTERN }
