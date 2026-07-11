import request from '@/utils/request'

function withStageToken(token) {
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export function login(data) {
  return request({
    url: '/auth/login',
    method: 'post',
    data
  })
}

export function getUserInfo() {
  return request({
    url: '/auth/info',
    method: 'get'
  })
}

export function changePassword(data) {
  return request({
    url: '/auth/password',
    method: 'put',
    data
  })
}

export function changeUsername(data) {
  return request({
    url: '/auth/username',
    method: 'put',
    data
  })
}

export function sendLoginEmailCode(token) {
  return request({
    url: '/auth/login/email/send',
    method: 'post',
    headers: withStageToken(token)
  })
}

export function verifyLoginStage(token, data) {
  return request({
    url: '/auth/login/verify',
    method: 'post',
    data,
    headers: withStageToken(token)
  })
}

export function sendEmailCode(data, token = '') {
  return request({
    url: '/auth/email/code/send',
    method: 'post',
    data,
    headers: withStageToken(token)
  })
}

export function bindEmail(data, token = '') {
  return request({
    url: '/auth/email/bind',
    method: 'post',
    data,
    headers: withStageToken(token)
  })
}

export function setup2FA(token = '') {
  return request({
    url: '/auth/2fa/setup',
    method: 'post',
    headers: withStageToken(token)
  })
}

export function enable2FA(data, token = '') {
  return request({
    url: '/auth/2fa/enable',
    method: 'post',
    data,
    headers: withStageToken(token)
  })
}

export function disable2FA(data) {
  return request({
    url: '/auth/2fa/disable',
    method: 'post',
    data
  })
}

export function regenRecoveryCodes(data) {
  return request({
    url: '/auth/2fa/recovery/regen',
    method: 'post',
    data
  })
}

export function verifyHighRisk(data) {
  return request({
    url: '/auth/high-risk/verify',
    method: 'post',
    data,
    skipHighRiskHandler: true
  })
}

export function getInviteInfo(token) {
  return request({
    url: '/auth/invite',
    method: 'get',
    params: { token }
  })
}

export function completeInvite(data) {
  return request({
    url: '/auth/invite/complete',
    method: 'post',
    data
  })
}

export function forgotPassword(data) {
  return request({
    url: '/auth/password/forgot',
    method: 'post',
    data
  })
}

export function sendForgotPasswordCode(data) {
  return request({
    url: '/auth/password/forgot/send-code',
    method: 'post',
    data
  })
}

export function verifyForgotPasswordCode(data) {
  return request({
    url: '/auth/password/forgot/verify-code',
    method: 'post',
    data
  })
}

export function selectForgotPasswordAccount(data) {
  return request({
    url: '/auth/password/forgot/select-account',
    method: 'post',
    data
  })
}

export function resetPasswordByEmail(data) {
  return request({
    url: '/auth/password/reset',
    method: 'post',
    data
  })
}

// 管理员跳过安全初始化
export function skipBootstrap(token) {
  return request({
    url: '/auth/skip-bootstrap',
    method: 'post',
    data: { confirm: true },
    headers: withStageToken(token)
  })
}

// 检查密码是否在泄露数据库中（公开接口）
export function checkPasswordBreach(password) {
  return request({
    url: '/auth/check-password',
    method: 'post',
    data: { password }
  })
}
