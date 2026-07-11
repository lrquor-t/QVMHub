import { defineStore } from 'pinia'

// QVMHub 当前选中节点(P2 设计 §3.1):所有业务请求经 axios 拦截器拼上 /api/n/{selectedNodeId}。
// 持久化到 localStorage,刷新页面后保持选择。
export const useNodeStore = defineStore('qvmhub-node', {
  state: () => ({
    selectedNodeId: localStorage.getItem('qvmhub_selected_node_id') || '',
    selectedNodeName: localStorage.getItem('qvmhub_selected_node_name') || '',
    nodes: [] // [{id,name,online,status,...}]
  }),
  getters: {
    hasSelection: (state) => !!state.selectedNodeId
  },
  actions: {
    setSelected(node) {
      this.selectedNodeId = String(node?.id ?? '')
      this.selectedNodeName = node?.name ?? ''
      localStorage.setItem('qvmhub_selected_node_id', this.selectedNodeId)
      localStorage.setItem('qvmhub_selected_node_name', this.selectedNodeName)
    },
    clear() {
      this.selectedNodeId = ''
      this.selectedNodeName = ''
      localStorage.removeItem('qvmhub_selected_node_id')
      localStorage.removeItem('qvmhub_selected_node_name')
    },
    // 从总览列表里挑选一个默认节点(优先当前选择 → 第一个在线 → 第一个)。
    pickDefault() {
      if (this.selectedNodeId) {
        const stillExists = this.nodes.some((n) => String(n.id) === this.selectedNodeId)
        if (stillExists) return
      }
      const firstOnline = this.nodes.find((n) => n.online)
      const target = firstOnline || this.nodes[0]
      if (target) this.setSelected(target)
      else this.clear()
    }
  }
})
