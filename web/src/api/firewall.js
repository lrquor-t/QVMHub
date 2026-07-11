import request from '@/utils/request'

export function getFirewallStatus() {
  return request({ url: '/firewall/status', method: 'get' })
}

export function getFirewallPolicy() {
  return request({ url: '/firewall/policy', method: 'get' })
}

export function saveFirewallPolicy(data) {
  return request({ url: '/firewall/policy', method: 'put', data })
}

export function previewFirewallPolicy(data) {
  return request({ url: '/firewall/preview', method: 'post', data })
}

export function applyFirewallPolicy(policy) {
  return request({ url: '/firewall/apply', method: 'post', data: { policy } })
}

export function disableFirewall() {
  return request({ url: '/firewall/disable', method: 'post' })
}

export function rollbackFirewall() {
  return request({ url: '/firewall/rollback', method: 'post' })
}

export function importFirewallRegion(data) {
  return request({ url: '/firewall/geoip/import', method: 'post', data })
}

export function updateFirewallGeoIP(data) {
  return request({ url: '/firewall/geoip/update', method: 'post', data })
}

export function getHostFirewallStatus() {
  return request({ url: '/firewall/host/status', method: 'get' })
}

export function previewEnableHostFirewall(data) {
  return request({ url: '/firewall/host/enable/preview', method: 'post', data })
}

export function enableHostFirewall(data) {
  return request({ url: '/firewall/host/enable', method: 'post', data })
}

export function disableHostFirewall() {
  return request({ url: '/firewall/host/disable', method: 'post' })
}

export function getHostFirewallRules() {
  return request({ url: '/firewall/host/rules', method: 'get' })
}

export function createHostFirewallRule(data) {
  return request({ url: '/firewall/host/rules', method: 'post', data })
}

export function updateHostFirewallRule(id, data) {
  return request({ url: `/firewall/host/rules/${id}`, method: 'put', data })
}

export function deleteHostFirewallRule(id) {
  return request({ url: `/firewall/host/rules/${id}`, method: 'delete' })
}

export function addHostFirewallVNCDefaultRule() {
  return request({ url: '/firewall/host/rules/vnc-default', method: 'post' })
}

export function previewHostFirewallConnections(mode) {
  return request({ url: '/firewall/host/connections/preview', method: 'get', params: { mode } })
}

export function closeHostFirewallConnections(data) {
  return request({ url: '/firewall/host/connections/close', method: 'post', data })
}
