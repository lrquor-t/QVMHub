import request from '@/utils/request'

export function getVPCQuota(params) {
  return request({ url: '/vpc/quota', method: 'get', params })
}

export function getVPCSwitches(params) {
  return request({ url: '/vpc/switches', method: 'get', params })
}

export function createVPCSwitch(data) {
  return request({ url: '/vpc/switches', method: 'post', data })
}

export function updateVPCSwitch(id, data) {
  return request({ url: `/vpc/switches/${id}`, method: 'put', data })
}

export function deleteVPCSwitch(id, force = false) {
  return request({ url: `/vpc/switches/${id}`, method: 'delete', params: { force: force ? 'true' : '' } })
}

export function getVPCSwitchVMs(id) {
  return request({ url: `/vpc/switches/${id}/vms`, method: 'get' })
}

export function resetVPCSwitchTraffic(id) {
  return request({ url: `/vpc/switches/${id}/traffic/reset`, method: 'post' })
}

export function getVPCSecurityGroups(params) {
  return request({ url: '/vpc/security-groups', method: 'get', params })
}

export function createVPCSecurityGroup(data) {
  return request({ url: '/vpc/security-groups', method: 'post', data })
}

export function updateVPCSecurityGroup(id, data) {
  return request({ url: `/vpc/security-groups/${id}`, method: 'put', data })
}

export function deleteVPCSecurityGroup(id) {
  return request({ url: `/vpc/security-groups/${id}`, method: 'delete' })
}

export function addVPCSecurityGroupRule(id, data) {
  return request({ url: `/vpc/security-groups/${id}/rules`, method: 'post', data })
}

export function deleteVPCSecurityGroupRule(id) {
  return request({ url: `/vpc/security-groups/rules/${id}`, method: 'delete' })
}

export function previewVPCACL() {
  return request({ url: '/vpc/acl/preview', method: 'get' })
}

export function applyVPCACL() {
  return request({ url: '/vpc/acl/apply', method: 'post' })
}

export function getVMVPCBinding(name) {
  return request({ url: `/vm/${name}/vpc`, method: 'get' })
}

export function bindVMVPC(name, data) {
  return request({ url: `/vm/${name}/vpc`, method: 'put', data })
}

export function switchVMSecurityGroup(name, securityGroupID) {
  return request({
    url: `/vm/${name}/security-group`,
    method: 'put',
    data: { security_group_id: securityGroupID }
  })
}

// 多网口管理（仅管理员）
export function listVMInterfaces(name) {
  return request({ url: `/vm/${name}/interfaces`, method: 'get' })
}

export function addVMInterface(name, data) {
  return request({ url: `/vm/${name}/interfaces`, method: 'post', data })
}

export function removeVMInterface(name, order) {
  return request({ url: `/vm/${name}/interfaces/${order}`, method: 'delete' })
}

export function updateVMInterface(name, order, data) {
  return request({ url: `/vm/${name}/interfaces/${order}`, method: 'put', data })
}
