import { defineStore } from 'pinia'
import { useVmStore } from './vm'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    username: localStorage.getItem('username') || '',
    role: localStorage.getItem('role') || '',
    cloudType: localStorage.getItem('cloud_type') || 'elastic',
    security: JSON.parse(localStorage.getItem('security') || 'null')
  }),
  actions: {
    setToken(token) {
      this.token = token
      localStorage.setItem('token', token)
    },
    setUserInfo(username, role, security = null, cloudType = 'elastic') {
      this.username = username
      this.role = role
      this.security = security
      this.cloudType = cloudType || 'elastic'
      localStorage.setItem('username', username)
      localStorage.setItem('role', role)
      localStorage.setItem('security', JSON.stringify(security || null))
      localStorage.setItem('cloud_type', this.cloudType)
    },
    setSecurity(security) {
      this.security = security
      localStorage.setItem('security', JSON.stringify(security || null))
    },
    logout() {
      this.token = ''
      this.username = ''
      this.role = ''
      this.cloudType = 'elastic'
      this.security = null
      localStorage.removeItem('token')
      localStorage.removeItem('username')
      localStorage.removeItem('role')
      localStorage.removeItem('cloud_type')
      localStorage.removeItem('security')
      sessionStorage.removeItem('2fa_recommended')
      // 清除虚拟机列表缓存
      const vmStore = useVmStore()
      vmStore.clearCache()
    }
  }
})
