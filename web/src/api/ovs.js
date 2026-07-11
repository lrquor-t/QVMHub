import request from '@/utils/request'

export function getOVSStatus() {
  return request({ url: '/ovs/status', method: 'get' })
}

export function getOVSPorts() {
  return request({ url: '/ovs/ports', method: 'get' })
}

export function getOVSLeases() {
  return request({ url: '/ovs/leases', method: 'get' })
}

export function checkOVSNetwork() {
  return request({ url: '/ovs/check', method: 'post' })
}

export function repairOVSNetwork() {
  return request({ url: '/ovs/repair', method: 'post' })
}
