import request from '@/utils/request'

// ==================== 用户存储池 API ====================

// 获取存储池信息
export function getStorageInfo() {
  return request({
    url: '/self/storage/info',
    method: 'get'
  })
}

// 初始化存储池
export function initStorage() {
  return request({
    url: '/self/storage/init',
    method: 'post'
  })
}

// 获取文件列表
export function getStorageFiles(category) {
  return request({
    url: `/self/storage/files/${category}`,
    method: 'get'
  })
}

// ---- 分片上传（断点续传 + 秒传）----

// 初始化/恢复上传会话（含秒传判断）
export function storageUploadInit(data) {
  return request({
    url: '/self/storage/upload/init',
    method: 'post',
    data
  })
}

// 上传单个分片（multipart: file, session_key, index）
export function storageUploadChunk(formData) {
  return request({
    url: '/self/storage/upload/chunk',
    method: 'post',
    data: formData,
    timeout: 0, // 单分片不超时
    maxContentLength: Infinity,
    maxBodyLength: Infinity
  })
}

// 全部分片到齐后完成校验
export function storageUploadComplete(data) {
  return request({
    url: '/self/storage/upload/complete',
    method: 'post',
    data
  })
}

// 查询上传进度（断点续传/继续上传）
export function storageUploadStatus(path) {
  return request({
    url: '/self/storage/upload/status',
    method: 'get',
    params: { path }
  })
}

// 取消上传（删除未完成文件与会话）
export function storageUploadCancel(path) {
  return request({
    url: '/self/storage/upload',
    method: 'delete',
    params: { path }
  })
}

// 列出未完成的上传会话（主动恢复）
export function getPendingUploads() {
  return request({
    url: '/self/storage/upload/pending',
    method: 'get'
  })
}

// 删除文件
export function deleteStorageFile(category, filename) {
  return request({
    url: `/self/storage/file/${category}/${encodeURIComponent(filename)}`,
    method: 'delete'
  })
}

// 下载文件
export function getStorageDownloadUrl(category, filename) {
  const token = localStorage.getItem('token')
  const baseUrl = import.meta.env.VITE_API_BASE || '/api'
  return `${baseUrl}/self/storage/download/${category}/${encodeURIComponent(filename)}?token=${token}`
}

// 获取用户ISO列表（VM创建用）
export function getUserISOs() {
  return request({
    url: '/self/storage/isos',
    method: 'get'
  })
}

// 获取用户所有VM的挂载列表
export function getUserMounts() {
  return request({
    url: '/self/storage/mounts',
    method: 'get'
  })
}

// 挂载存储池到VM
export function mountStorage(data) {
  return request({
    url: '/self/storage/mount',
    method: 'post',
    data
  })
}

// 卸载存储池
export function unmountStorage(vmName, tag) {
  return request({
    url: `/self/storage/mount/${vmName}/${tag}`,
    method: 'delete'
  })
}

// 用户自助创建VM
export function selfCreateVm(data) {
  return request({
    url: '/self/vm/create',
    method: 'post',
    data
  })
}

// 导出虚拟机
export function exportVM(data) {
  return request({
    url: '/self/vm/export',
    method: 'post',
    data
  })
}

// 导入虚拟机
export function importVM(data) {
  return request({
    url: '/self/vm/import',
    method: 'post',
    data
  })
}
