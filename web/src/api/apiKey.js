import request from '@/utils/request'

export function getAPIKeyInfo() {
  return request({
    url: '/auth/api-key',
    method: 'get'
  })
}

export function rotateAPIKey() {
  return request({
    url: '/auth/api-key',
    method: 'post'
  })
}

export function revokeAPIKey() {
  return request({
    url: '/auth/api-key',
    method: 'delete'
  })
}
