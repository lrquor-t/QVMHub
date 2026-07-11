import request from '@/utils/request'

// 获取用户列表（管理员）
export function getUserList(params) {
  return request({
    url: '/user/list',
    method: 'get',
    params
  })
}

// 创建用户（管理员）
export function createUser(data) {
  return request({
    url: '/user',
    method: 'post',
    data
  })
}

// 分配虚拟机给用户（管理员）
export function assignVms(username, data) {
  return request({
    url: `/user/${username}/vms`,
    method: 'put',
    data
  })
}

// 登记轻量云待开通 VM（管理员）
export function createLightweightVmRegistrations(username, data) {
  return request({
    url: `/user/${username}/lightweight-registrations`,
    method: 'post',
    data
  })
}

// 删除轻量云待开通 VM 注册（管理员）
export function deleteLightweightVmRegistration(username, id) {
  return request({
    url: `/user/${username}/lightweight-registrations/${id}`,
    method: 'delete'
  })
}

// 移除已开通轻量云 VM 的注册列表记录和单 VM 配额（不删除虚拟机）
export function removeLightweightRegisteredVm(username, vmName) {
  return request({
    url: `/user/${username}/lightweight-vm/${encodeURIComponent(vmName)}`,
    method: 'delete'
  })
}

// 更新轻量云单 VM 配额（管理员）
export function updateLightweightVmQuota(username, data) {
  return request({
    url: `/user/${username}/lightweight-vm-quota`,
    method: 'put',
    data
  })
}

// 更新用户配额（管理员）
export function updateUserQuota(username, data) {
  return request({
    url: `/user/${username}/quota`,
    method: 'put',
    data
  })
}

// 更新用户状态（封禁/解封）
export function updateUserStatus(username, data) {
  return request({
    url: `/user/${username}/status`,
    method: 'put',
    data
  })
}

// 获取用户配额使用情况（管理员）
export function getUserQuotaUsage(username) {
  return request({
    url: `/user/${username}/quota`,
    method: 'get'
  })
}

// 删除用户（管理员）
export function deleteUser(username) {
  return request({
    url: `/user/${username}`,
    method: 'delete'
  })
}

export function resendInvite(username) {
  return request({
    url: `/user/${username}/resend-invite`,
    method: 'post'
  })
}

// 切换用户 SSH 访问权限（管理员）
export function toggleUserSSH(username, enabled) {
  return request({
    url: `/user/${username}/ssh`,
    method: 'put',
    data: { enabled }
  })
}

// 重置用户流量配额（管理员）
export function resetUserTraffic(username) {
  return request({
    url: `/user/${username}/traffic/reset`,
    method: 'post'
  })
}

// ==================== 用户自助 API ====================

// 获取当前用户的配额信息
export function getSelfQuota() {
  return request({
    url: '/self/quota',
    method: 'get'
  })
}

// 获取当前用户的VM列表
export function getSelfVMs(params) {
  return request({
    url: '/self/vms',
    method: 'get',
    params
  })
}

// 用户自助克隆VM
export function selfCloneVm(data) {
  return request({
    url: '/self/vm/clone',
    method: 'post',
    data
  })
}

// 用户自助删除VM
export function selfDeleteVm(name, data = {}) {
  return request({
    url: `/self/vm/${name}`,
    method: 'delete',
    data
  })
}

// 获取当前用户待确认的轻量云服务器
export function getSelfLightweightVmRegistrations() {
  return request({
    url: '/self/lightweight-registrations',
    method: 'get'
  })
}

// 确认并开通轻量云服务器
export function confirmSelfLightweightVmRegistration(id, data) {
  return request({
    url: `/self/lightweight-registrations/${id}/confirm`,
    method: 'post',
    data
  })
}

// 获取虚拟机的 qcow2 磁盘列表（用户自助-删除确认用）
export function selfGetVmQcow2Disks(name) {
  return request({
    url: `/self/vm/${name}/qcow2-disks`,
    method: 'get'
  })
}
