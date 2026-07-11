import request from '@/utils/request'

export function listNodes() {
  return request({
    url: '/nodes',
    method: 'get'
  })
}

export function createNode(data) {
  return request({
    url: '/nodes',
    method: 'post',
    data
  })
}

export function updateNode(id, data) {
  return request({
    url: `/nodes/${id}`,
    method: 'put',
    data
  })
}

export function deleteNode(id) {
  return request({
    url: `/nodes/${id}`,
    method: 'delete'
  })
}

export function probeNode(id) {
  return request({
    url: `/nodes/${id}/probe`,
    method: 'post',
    timeout: 120000
  })
}

export function getNodeMigrationOptions(id, params) {
  return request({
    url: `/nodes/${id}/migration-options`,
    method: 'get',
    params
  })
}
