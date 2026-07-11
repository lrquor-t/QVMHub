import { defineStore } from 'pinia'

/**
 * 虚拟机列表缓存 Store
 * 用于在页面导航（如从详情页返回列表页）时避免重复加载
 */
export const useVmStore = defineStore('vm', {
  state: () => ({
    // 缓存的虚拟机列表数据
    vmList: [],
    // 上次获取数据的时间戳
    lastFetchTime: 0,
    // 最近访问的虚拟机列表，用于侧边栏展示
    visitedVms: [],
  }),

  getters: {
    // 是否已有缓存数据
    hasCachedData: (state) => state.vmList.length > 0 && state.lastFetchTime > 0,
  },

  actions: {
    // 更新虚拟机列表缓存
    setVmList(data) {
      if (Array.isArray(data)) {
        this.vmList = data
        this.lastFetchTime = Date.now()
      }
    },
    
    // 添加访问的虚拟机记录
    addVisitedVm(vm) {
      if (!vm || !vm.id) return
      
      const vmRecord = {
        id: String(vm.id),
        name: vm.name || String(vm.id)
      }
      
      const index = this.visitedVms.findIndex(v => v.id === vmRecord.id)
      if (index === -1) {
        this.visitedVms.push(vmRecord)
      } else {
        this.visitedVms[index].name = vmRecord.name
      }
    },
    
    // 移除访问的虚拟机记录
    removeVisitedVm(id) {
      this.visitedVms = this.visitedVms.filter(v => v.id !== id)
    },

    // 清除缓存（用于登出或需要强制刷新时）
    clearCache() {
      this.vmList = []
      this.lastFetchTime = 0
      this.visitedVms = []
    },
  },
})
