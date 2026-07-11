import request from '@/utils/request'
import axios from 'axios'

function withStageToken(token) {
  return token ? { Authorization: `Bearer ${token}` } : {}
}

// 获取公开系统设置
export function getPublicSettings() {
  return request({
    url: '/public/settings',
    method: 'get'
  })
}

// 获取系统版本信息
export function getPublicVersion() {
  return request({
    url: '/public/version',
    method: 'get'
  })
}

// 获取系统运行环境信息（需登录）
export function getPublicSystemInfo() {
  return request({
    url: '/system-info',
    method: 'get'
  })
}

// 获取系统设置
export function getSettings(token = '') {
  return request({
    url: '/settings',
    method: 'get',
    headers: withStageToken(token)
  })
}

// 更新系统设置
export function updateSettings(data, token = '') {
  return request({
    url: '/settings',
    method: 'put',
    data,
    headers: withStageToken(token)
  })
}

// 测试 SMTP 发信
export function testSMTP(data, token = '') {
  return request({
    url: '/settings/smtp/test',
    method: 'post',
    data,
    headers: withStageToken(token)
  })
}

// 手动轮换 JWT 密钥
export function rotateJWTSecret(data, token = '') {
  return request({
    url: '/settings/jwt-secret/rotate',
    method: 'post',
    data,
    headers: withStageToken(token)
  })
}

// 获取宿主机 Intel KVM unrestricted_guest 状态
export function getHostKVMUnrestrictedGuestStatus() {
  return request({
    url: '/host/kvm-intel-unrestricted-guest',
    method: 'get'
  })
}

// 获取宿主机 CPU 核心总数（用于 CPU 热添加上限）
export function getHostCPUCores() {
  return request({
    url: '/host/cpus',
    method: 'get'
  })
}

// 设置宿主机 Intel KVM unrestricted_guest
export function updateHostKVMUnrestrictedGuest(data) {
  return request({
    url: '/host/kvm-intel-unrestricted-guest',
    method: 'put',
    data,
    timeout: 30000
  })
}

// 获取宿主机 KSM 状态
export function getHostKSMStatus() {
  return request({
    url: '/host/ksm',
    method: 'get'
  })
}

// 设置宿主机 KSM 挡位
export function updateHostKSMProfile(data) {
  return request({
    url: '/host/ksm',
    method: 'put',
    data,
    timeout: 30000
  })
}

// 获取宿主机 zRAM 状态
export function getHostZRAMStatus() {
  return request({
    url: '/host/zram',
    method: 'get'
  })
}

// 设置宿主机 zRAM 挡位
export function updateHostZRAMProfile(data) {
  return request({
    url: '/host/zram',
    method: 'put',
    data,
    timeout: 30000
  })
}

// 获取硬件直通状态
export function getHardwarePassthroughStatus() {
  return request({
    url: '/host/hardware-passthrough/status',
    method: 'get'
  })
}

// 一键开启 IOMMU（写入 grub + update-grub）
export function enableIommu() {
  return request({
    url: '/host/hardware-passthrough/enable-iommu',
    method: 'post',
    timeout: 60000
  })
}

// 一键加载 vfio-pci 模块
export function loadVfioPci() {
  return request({
    url: '/host/hardware-passthrough/load-vfio',
    method: 'post',
    timeout: 30000
  })
}

// 获取 CPU 亲和性预设列表
export function getCPUAffinityPresets() {
  return request({
    url: '/cpu-affinity-presets',
    method: 'get'
  })
}

// 获取当前用户存储 ISO 目录路径（用于一键修改系统 ISO 存放位置）
export function getUserStorageISOPath() {
  return request({
    url: '/settings/user-storage-iso-path',
    method: 'get'
  })
}

// 保存 CPU 亲和性预设列表（管理员）
export function saveCPUAffinityPresets(data) {
  return request({
    url: '/settings/cpu-affinity-presets',
    method: 'put',
    data
  })
}

// 获取日志状态（文件列表、磁盘占用）
export function getLogStatus() {
  return request({
    url: '/settings/log/status',
    method: 'get'
  })
}

// 删除日志文件
export function deleteLogs(data) {
  return request({
    url: '/settings/log/delete',
    method: 'post',
    data
  })
}

// 导出日志文件（返回 blob，使用原始 axios 实例避免 JSON 拦截器干扰）
export async function exportLogs(data) {
  const baseURL = import.meta.env.VITE_APP_BASE_API || '/api'
  const { useUserStore } = await import('@/store/user')
  const userStore = useUserStore()
  const headers = {}
  if (userStore.token) {
    headers.Authorization = `Bearer ${userStore.token}`
  }
  const response = await axios({
    url: baseURL + '/settings/log/export',
    method: 'post',
    data,
    responseType: 'blob',
    headers
  })
  return response.data
}

// 执行存储回收（fstrim + fallocate --dig-holes）
export function trimUserStorage() {
  return request({
    url: '/settings/storage/trim',
    method: 'post'
  })
}

// 获取诊断类别列表
export function getDiagnosticCategories() {
  return request({
    url: '/settings/diagnostics/categories',
    method: 'get'
  })
}

// 导出诊断信息（返回 blob，使用原始 axios 实例避免 JSON 拦截器干扰）
export async function exportDiagnostics(data) {
  const baseURL = import.meta.env.VITE_APP_BASE_API || '/api'
  const { useUserStore } = await import('@/store/user')
  const userStore = useUserStore()
  const headers = {}
  if (userStore.token) {
    headers.Authorization = `Bearer ${userStore.token}`
  }
  const response = await axios({
    url: baseURL + '/settings/diagnostics/export',
    method: 'post',
    data,
    responseType: 'blob',
    headers,
    timeout: 120000 // 诊断收集可能耗时较长
  })
  return response.data
}
