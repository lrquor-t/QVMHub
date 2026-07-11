<template>
  <div class="vnc-window">
    <!-- 顶部工具栏 -->
    <div class="vnc-window-toolbar">
      <div class="toolbar-left">
        <el-icon :size="20" color="#409EFF"><Monitor /></el-icon>
        <span class="vm-name">{{ vmName }}</span>
        <el-tag :type="connected ? 'success' : 'info'" size="small" style="margin-left: 12px;">
          {{ connected ? '已连接' : '未连接' }}
        </el-tag>
      </div>
      <div class="toolbar-right">
        <el-button v-if="!connected" type="success" @click="connect" :loading="connecting" size="small">
          <el-icon><Monitor /></el-icon>
          连接
        </el-button>
        <el-button v-else type="danger" @click="disconnect" size="small">
          <el-icon><SwitchButton /></el-icon>
          断开
        </el-button>
        <el-button @click="sendPrimaryShortcut" :disabled="!connected" size="small">
          {{ primaryShortcut.label }}
        </el-button>
        <el-dropdown trigger="click" :disabled="!connected">
          <el-button :disabled="!connected" size="small">
            常用组合键 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="shortcut in commonShortcuts"
                :key="shortcut.id"
                @click="sendCommonShortcut(shortcut)"
              >
                {{ shortcut.label }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-tooltip :content="pastePasswordTip" placement="top">
          <span class="toolbar-inline-block">
            <el-button
              @click="pasteGuestPassword"
              :disabled="!canPasteGuestPassword"
              :loading="pastingGuestPassword"
              size="small"
            >
              粘贴密码
            </el-button>
          </span>
        </el-tooltip>
        <el-tooltip :content="sendTextTip" placement="top">
          <span class="toolbar-inline-block">
            <el-button @click="showSendTextDialog = true" :disabled="!connected" size="small">
              发送文本
            </el-button>
          </span>
        </el-tooltip>
        <el-button @click="toggleFullscreen" :disabled="!connected" size="small">
          <el-icon><FullScreen /></el-icon>
          全屏
        </el-button>
        <el-divider direction="vertical" />
        <el-button @click="handleClose" size="small" type="info" plain>
          <el-icon><Close /></el-icon>
          关闭窗口
        </el-button>
      </div>
    </div>

    <!-- VNC 画布区域 -->
    <div ref="vncContainer" class="vnc-window-screen" :class="{ 'vnc-connected': connected, 'vnc-fullscreen': isFullscreen }">
      <div v-if="!connected" class="vnc-placeholder">
        <el-icon :size="80" color="#4a4a5a"><Monitor /></el-icon>
        <p v-if="connecting">正在连接中...</p>
        <p v-else>点击工具栏「连接」按钮开始远程控制</p>
        <p v-if="errorMsg" class="error-msg">{{ errorMsg }}</p>
      </div>
    </div>

    <el-dialog v-model="showSendTextDialog" title="发送文本到 VNC" width="520px" :close-on-click-modal="false" append-to-body>
      <el-form label-width="100px">
        <el-form-item label="文本内容">
          <el-input
            v-model="customText"
            type="textarea"
            :autosize="{ minRows: 4, maxRows: 10 }"
            resize="vertical"
            placeholder="请输入要发送到虚拟机当前焦点位置的文本"
          />
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false" style="margin-top: 10px;">
        <template #title>
          文本会按字符逐个输入到虚拟机当前焦点位置，适合粘贴密码、命令、激活码或多行文本。
        </template>
      </el-alert>
      <template #footer>
        <el-button @click="closeSendTextDialog">取消</el-button>
        <el-button type="primary" @click="handleSendCustomText" :loading="sendingCustomText">确认发送</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getVmDetail } from '@/api/vm'
import { useUserStore } from '@/store/user'
import RFB from '@novnc/novnc'
import { applyDocumentTitle } from '@/utils/site'
import { buildVncWsUrl, COMMON_VNC_SHORTCUTS, PRIMARY_VNC_SHORTCUT, refreshVncViewport, sendTextToVnc, sendVncShortcut } from '@/utils/vnc'

const route = useRoute()
const vmName = route.params.id

// 状态
const connected = ref(false)
const connecting = ref(false)
const errorMsg = ref('')
const isFullscreen = ref(false)
const guestPassword = ref('')
const pastingGuestPassword = ref(false)
const sendingCustomText = ref(false)
const showSendTextDialog = ref(false)
const customText = ref('')
const canPasteGuestPassword = computed(() => connected.value && !!guestPassword.value)
const pastePasswordTip = computed(() => {
  if (!connected.value) {
    return '请先连接 VNC'
  }
  if (!guestPassword.value) {
    return '当前未保存虚拟机登录密码'
  }
  return '将已保存的登录密码输入到虚拟机当前焦点位置'
})
const sendTextTip = computed(() => {
  if (!connected.value) {
    return '请先连接 VNC'
  }
  return '手动输入一段文本并发送到虚拟机当前焦点位置'
})
const primaryShortcut = PRIMARY_VNC_SHORTCUT
const commonShortcuts = COMMON_VNC_SHORTCUTS

// noVNC
const vncContainer = ref(null)
let rfb = null

// 构建 WebSocket URL
const getWsUrl = () => {
  const userStore = useUserStore()
  return buildVncWsUrl(vmName, userStore.token)
}

// 获取已保存的登录密码
const fetchGuestPassword = async () => {
  try {
    const res = await getVmDetail(vmName)
    guestPassword.value = res.data?.credential?.password || ''
  } catch (err) {
    console.error('获取虚拟机登录凭据失败', err)
  }
}

// 连接 VNC
const connect = async () => {
  if (!vncContainer.value) return

  connecting.value = true
  errorMsg.value = ''

  try {
    const url = getWsUrl()

    // 清除旧的画布元素
    const existingCanvas = vncContainer.value.querySelector('canvas')
    if (existingCanvas) {
      vncContainer.value.removeChild(existingCanvas)
    }

    rfb = new RFB(vncContainer.value, url)

    rfb.viewOnly = false
    rfb.scaleViewport = true
    rfb.resizeSession = false

    rfb.addEventListener('connect', () => {
      connected.value = true
      connecting.value = false
      errorMsg.value = ''
      // 更新窗口标题
      applyDocumentTitle(`VNC - ${vmName}`)
      // 连接成功后延迟触发画布重新缩放
      nextTick(() => {
        setTimeout(() => {
          refreshVncViewport(rfb)
        }, 200)
      })
    })

    rfb.addEventListener('disconnect', (e) => {
      connected.value = false
      connecting.value = false
      if (!e.detail.clean) {
        errorMsg.value = '连接已断开（异常）'
      }
      rfb = null
    })

    rfb.addEventListener('credentialsrequired', () => {
      ElMessageBox.prompt('请输入 VNC 密码', 'VNC 认证', {
        confirmButtonText: '确认',
        cancelButtonText: '取消',
        inputType: 'password',
        inputPlaceholder: '请输入 VNC 密码',
      }).then(({ value }) => {
        if (rfb) {
          rfb.sendCredentials({ password: value })
        }
      }).catch(() => {
        disconnect()
      })
    })

    rfb.addEventListener('securityfailure', (e) => {
      errorMsg.value = `认证失败: ${e.detail.reason || '密码错误'}`
      connecting.value = false
    })

  } catch (err) {
    errorMsg.value = `连接失败: ${err.message}`
    connecting.value = false
  }
}

// 断开 VNC
const disconnect = () => {
  if (rfb) {
    rfb.disconnect()
    rfb = null
  }
  connected.value = false
  closeSendTextDialog()
}

// 发送主快捷键
const sendPrimaryShortcut = () => {
  if (!rfb) {
    return
  }
  sendVncShortcut(rfb, primaryShortcut)
  ElMessage.success(`已发送 ${primaryShortcut.label}`)
}

// 发送常用组合键
const sendCommonShortcut = (shortcut) => {
  if (!rfb) {
    return
  }
  sendVncShortcut(rfb, shortcut)
  ElMessage.success(`已发送 ${shortcut.label}`)
}

// 输入已保存密码
const pasteGuestPassword = async () => {
  if (!connected.value) {
    ElMessage.warning('请先连接 VNC')
    return
  }
  if (!guestPassword.value) {
    ElMessage.warning('当前未保存虚拟机登录密码')
    return
  }

  pastingGuestPassword.value = true
  try {
    await sendTextToVnc(rfb, guestPassword.value)
    ElMessage.success('已将登录密码输入到虚拟机当前焦点位置')
  } finally {
    pastingGuestPassword.value = false
  }
}

const closeSendTextDialog = () => {
  showSendTextDialog.value = false
  customText.value = ''
}

const handleSendCustomText = async () => {
  if (!connected.value || !rfb) {
    ElMessage.warning('请先连接 VNC')
    return
  }
  if (!customText.value) {
    ElMessage.warning('请输入要发送的文本')
    return
  }

  sendingCustomText.value = true
  try {
    await sendTextToVnc(rfb, customText.value)
    ElMessage.success('文本已发送到虚拟机当前焦点位置')
    closeSendTextDialog()
  } finally {
    sendingCustomText.value = false
  }
}

// 全屏
const toggleFullscreen = () => {
  if (!vncContainer.value) return

  if (!document.fullscreenElement) {
    vncContainer.value.requestFullscreen().then(() => {
      isFullscreen.value = true
    }).catch(() => {
      ElMessage.warning('浏览器不支持全屏或被阻止')
    })
  } else {
    document.exitFullscreen()
    isFullscreen.value = false
  }
}

// 监听全屏变化
const handleFullscreenChange = () => {
  isFullscreen.value = !!document.fullscreenElement
}

// 窗口大小变化时重新缩放
const handleResize = () => {
  refreshVncViewport(rfb)
}

// 关闭窗口
const handleClose = () => {
  disconnect()
  window.close()
}

onMounted(() => {
  applyDocumentTitle(`VNC - ${vmName}`)
  fetchGuestPassword()
  document.addEventListener('fullscreenchange', handleFullscreenChange)
  window.addEventListener('resize', handleResize)
  // 自动连接
  setTimeout(connect, 300)
})

onBeforeUnmount(() => {
  disconnect()
  closeSendTextDialog()
  document.removeEventListener('fullscreenchange', handleFullscreenChange)
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.vnc-window {
  width: 100vw;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #0d0d1a;
  overflow: hidden;
}

.vnc-window-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
  border-bottom: 1px solid #2a2a3e;
  flex-shrink: 0;
  z-index: 10;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.vm-name {
  color: #e0e0e0;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0.5px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 6px;
}

.toolbar-inline-block {
  display: inline-block;
}

.vnc-window-screen {
  flex: 1;
  background: #0d0d1a;
  overflow: hidden;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.vnc-window-screen.vnc-connected {
  display: block;
}

.vnc-window-screen.vnc-fullscreen {
  border-radius: 0;
}

.vnc-placeholder {
  text-align: center;
  color: #6b6b7b;
}

.vnc-placeholder p {
  margin-top: 20px;
  font-size: 16px;
  color: #8b8b9b;
}

.vnc-placeholder .error-msg {
  color: #F56C6C;
  font-size: 14px;
}

/* noVNC 画布样式 */
.vnc-window-screen :deep(canvas) {
  max-width: 100%;
  max-height: 100%;
}

/* Element Plus 在暗色模式下的适配 */
.vnc-window :deep(.el-divider--vertical) {
  border-color: #3a3a4e;
  margin: 0 8px;
}

@media (max-width: 768px) {
  .vnc-window-toolbar {
    padding: 6px 10px;
    flex-wrap: wrap;
    gap: 6px;
  }

  .toolbar-left {
    flex: 1 1 100%;
  }

  .toolbar-right {
    flex-wrap: wrap;
    gap: 4px;
  }

  .toolbar-right .el-button {
    font-size: 11px;
    padding: 5px 8px;
  }

  .vm-name {
    font-size: 13px;
  }

  .toolbar-inline-block {
    display: none;
  }
}

@media (max-width: 480px) {
  .toolbar-right .el-button {
    font-size: 10px;
    padding: 4px 6px;
  }

  .vnc-window-toolbar {
    padding: 4px 8px;
    gap: 4px;
  }
}
</style>
