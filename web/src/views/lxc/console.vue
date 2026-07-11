<template>
  <div class="lxc-console-wrap">
    <div class="lxc-console-toolbar">
      <span class="title">LXC 控制台 · {{ name }}</span>
      <div class="toolbar-actions">
        <el-button size="small" :icon="RefreshRight" :loading="connecting" @click="reconnect">
          {{ connecting ? '连接中' : (connected ? '已连接' : '重连') }}
        </el-button>
        <el-button size="small" :icon="FullScreen" @click="fit">适应窗口</el-button>
      </div>
    </div>
    <div ref="termEl" class="lxc-terminal"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { ElButton } from 'element-plus'
import { RefreshRight, FullScreen } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { buildLXCConsoleWsUrl } from '@/api/lxc'
import 'xterm/css/xterm.css'

const route = useRoute()
const userStore = useUserStore()
const name = route.params.name

const termEl = ref(null)
const connecting = ref(false)
const connected = ref(false)
let term = null
let fitAddon = null
let ws = null

const sendResize = () => {
  if (!ws || ws.readyState !== WebSocket.OPEN || !fitAddon) return
  const cols = term.cols
  const rows = term.rows
  ws.send(JSON.stringify({ action: 'resize', cols, rows }))
}

const fit = () => {
  if (fitAddon) {
    fitAddon.fit()
    sendResize()
  }
}

const connect = () => {
  connecting.value = true
  ws = new WebSocket(buildLXCConsoleWsUrl(name, userStore.token))
  ws.binaryType = 'arraybuffer'
  ws.onopen = () => {
    connecting.value = false
    connected.value = true
    nextTick(() => { fit(); sendResize() })
  }
  ws.onmessage = (ev) => {
    // 后端 PTY 输出为 BinaryMessage
    if (typeof ev.data === 'string') return
    term.write(new Uint8Array(ev.data))
  }
  ws.onclose = () => { connected.value = false; connecting.value = false }
  ws.onerror = () => { connected.value = false; connecting.value = false }
}

const reconnect = () => {
  if (ws) { try { ws.close() } catch {} }
  connect()
}

const onResize = () => fit()

onMounted(() => {
  term = new Terminal({ cursorBlink: true, fontSize: 13 })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(termEl.value)
  term.onData((d) => {
    // 键入 → BinaryMessage 原始字节
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(new TextEncoder().encode(d))
    }
  })
  window.addEventListener('resize', onResize)
  connect()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  if (ws) { try { ws.close() } catch {} }
  if (term) term.dispose()
})
</script>

<style scoped>
.lxc-console-wrap { display: flex; flex-direction: column; height: 100vh; background: #000; }
.lxc-console-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 14px;
  background: #1f1f1f;            /* 固定深色（用户决策，保持终端感，不跟随主题） */
  border-bottom: 1px solid #2e2e2e;
  color: #ddd;
}
.lxc-console-toolbar .title { font-size: 13px; font-weight: 600; letter-spacing: 0.2px; }
.lxc-console-toolbar .toolbar-actions { margin-left: auto; display: flex; gap: 8px; }
.lxc-console-toolbar :deep(.el-button) { background: #2a2a2a; border-color: #3a3a3a; color: #ddd; }
.lxc-console-toolbar :deep(.el-button:hover) { background: #333; border-color: #555; color: #fff; }
.lxc-terminal { flex: 1; padding: 4px 8px; }
</style>
