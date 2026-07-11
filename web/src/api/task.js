import request from '@/utils/request'

// 获取任务列表（支持筛选）
export function getTaskList(params) {
  return request({
    url: '/task/list',
    method: 'get',
    params
  })
}

// 获取任务详情
export function getTaskDetail(id) {
  return request({
    url: `/task/${id}`,
    method: 'get'
  })
}

// 取消任务
export function cancelTask(id) {
  return request({
    url: `/task/${id}/cancel`,
    method: 'post'
  })
}

// 清理已完成任务
export function clearFinishedTasks() {
  return request({
    url: '/task/clear',
    method: 'delete'
  })
}

/**
 * 创建 SSE 连接用于实时接收任务进度
 * @returns {EventSource}
 */
export function createTaskSSE() {
  const token = localStorage.getItem('token')
  const baseURL = import.meta.env.VITE_APP_BASE_API || '/api'
  const url = `${baseURL}/task/sse?token=${encodeURIComponent(token)}`
  return new EventSource(url)
}
