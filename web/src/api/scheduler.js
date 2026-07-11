import request, { sseURL } from '@/utils/request'

// 获取调度器概览
export function getSchedulerList() {
  return request({
    url: '/scheduler/list',
    method: 'get'
  })
}

// 获取调度事件列表
export function getSchedulerEventList(params) {
  return request({
    url: '/scheduler/events',
    method: 'get',
    params
  })
}

// 创建调度事件 SSE 连接
export function createSchedulerEventSSE() {
  const token = localStorage.getItem('token')
  return new EventSource(sseURL('/scheduler/events/sse', 'token=' + encodeURIComponent(token)))
}
