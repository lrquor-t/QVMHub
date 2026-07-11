import request from '@/utils/request'

export function getHostStats() {
  return request({
    url: '/host/stats',
    method: 'get'
  })
}

export function getHostStatsHistory(params) {
  return request({
    url: '/host/stats/history',
    method: 'get',
    params
  })
}

export function getHostDisks() {
  return request({
    url: '/host/disks',
    method: 'get'
  })
}

export function getHostKSMStatus() {
  return request({
    url: '/host/ksm',
    method: 'get'
  })
}

export function createHostStatsSSE(token) {
  const baseUrl = import.meta.env.VITE_APP_BASE_API || '/api'
  return new EventSource(`${baseUrl}/host/stats/sse?token=${token}`)
}
