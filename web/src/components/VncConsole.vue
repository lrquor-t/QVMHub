<template>
  <div class="vnc-console">
    <!-- VNC 管理区域 -->
    <div class="vnc-toolbar">
      <div class="toolbar-left">
        <el-tag :type="vncInfo.enabled ? 'success' : 'info'" size="large">
          {{ vncInfo.enabled ? 'VNC 已开启' : 'VNC 未开启' }}
        </el-tag>
        <el-tag v-if="vncInfo.enabled && vncInfo.auth" type="warning" size="small" style="margin-left: 8px;">
          认证: {{ vncInfo.auth }}
        </el-tag>
        <el-tag v-if="vncInfo.enabled && vncInfo.exposed" type="danger" size="small" style="margin-left: 8px;">
          ☢ 已对外暴露
        </el-tag>
      </div>
      <div class="toolbar-right">
        <!-- VNC 未开启时 -->
        <template v-if="!vncInfo.enabled">
          <el-button type="primary" @click="showEnableDialog = true" :disabled="vmStatus !== 'running' && vmStatus !== 'shut off'">
            <el-icon><VideoCamera /></el-icon>
            开启 VNC
          </el-button>
        </template>

        <!-- VNC 已开启时 -->
        <template v-else>
          <el-button v-if="!connected" type="success" @click="connect" :disabled="!canConnectVnc" :loading="connecting">
            <el-icon><Monitor /></el-icon>
            连接
          </el-button>
          <el-button v-else type="danger" @click="disconnect">
            <el-icon><SwitchButton /></el-icon>
            断开
          </el-button>
          <el-button @click="sendPrimaryShortcut" :disabled="!connected">
            {{ primaryShortcut.label }}
          </el-button>
          <el-dropdown trigger="click" :disabled="!connected">
            <el-button :disabled="!connected">
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
              >
                粘贴密码
              </el-button>
            </span>
          </el-tooltip>
          <el-tooltip :content="sendTextTip" placement="top">
            <span class="toolbar-inline-block">
              <el-button @click="showSendTextDialog = true" :disabled="!connected">
                发送文本
              </el-button>
            </span>
          </el-tooltip>
          <el-button @click="toggleFullscreen" :disabled="!connected">
            <el-icon><FullScreen /></el-icon>
          </el-button>
          <el-tooltip content="在新窗口中打开" placement="top">
            <el-button @click="openInNewWindow">
              <el-icon><TopRight /></el-icon>
            </el-button>
          </el-tooltip>
          <el-dropdown trigger="click" style="margin-left: 8px;">
            <el-button>
              管理 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="showPasswordDialog = true">修改密码</el-dropdown-item>
                <el-dropdown-item v-if="!vncInfo.exposed" @click="handleExposeVnc(true)" divided>
                  <span style="color: #E6A23C;">☢ 开启对外暴露</span>
                </el-dropdown-item>
                <el-dropdown-item v-else @click="handleExposeVnc(false)" divided>
                  关闭对外暴露
                </el-dropdown-item>
                <el-dropdown-item @click="handleDisableVnc" divided>
                  <span style="color: #F56C6C;">关闭 VNC</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </div>
    </div>

    <!-- noVNC 画布区域 -->
    <div ref="vncContainer" class="vnc-screen" :class="{ 'vnc-connected': connected, 'vnc-fullscreen': isFullscreen, 'vnc-active': connected }">
      <div v-if="!connected" class="vnc-placeholder">
        <el-icon :size="64" color="#909399"><Monitor /></el-icon>
        <p v-if="!vncInfo.enabled">请先开启 VNC</p>
        <p v-else-if="!canConnectVnc">虚拟机未运行</p>
        <p v-else-if="vmStatus === 'paused'">虚拟机当前处于暂停状态，可直接连接查看画面</p>
        <p v-else>点击「连接」按钮开始远程控制</p>
        <p v-if="errorMsg" class="error-msg">{{ errorMsg }}</p>
      </div>
    </div>

    <!-- 开启 VNC 对话框 -->
    <el-dialog v-model="showEnableDialog" title="开启 VNC" width="420px" :close-on-click-modal="false" append-to-body>
      <el-form label-width="100px">
        <el-form-item label="VNC 密码">
          <el-input v-model="enablePassword" type="password" show-password placeholder="留空则无密码（最长8位）" maxlength="8" />
        </el-form-item>
      </el-form>
      <el-alert type="warning" :closable="false" style="margin-top: 10px;">
        <template #title>
          开启 VNC 需要重启虚拟机才能生效。{{ vmStatus === 'running' ? '虚拟机将会自动重启。' : '请在开启后启动虚拟机。' }}
        </template>
      </el-alert>
      <template #footer>
        <el-button @click="showEnableDialog = false">取消</el-button>
        <el-button type="primary" @click="handleEnableVnc" :loading="enableLoading">确认开启</el-button>
      </template>
    </el-dialog>

    <!-- 修改密码对话框 -->
    <el-dialog v-model="showPasswordDialog" title="修改 VNC 密码" width="420px" :close-on-click-modal="false" append-to-body>
      <el-form label-width="100px">
        <el-form-item label="新密码">
          <el-input v-model="newPassword" type="password" show-password placeholder="请输入新密码（最长8位）" maxlength="8" />
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false" style="margin-top: 10px;">
        <template #title>
          密码修改即时生效，无需重启虚拟机。
        </template>
      </el-alert>
      <template #footer>
        <el-button @click="showPasswordDialog = false">取消</el-button>
        <el-button type="primary" @click="handleChangePassword" :loading="passwdLoading">确认修改</el-button>
      </template>
    </el-dialog>

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
import { computed, ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getVncStatus, enableVnc, disableVnc, changeVncPassword, exposeVnc } from '@/api/vm'
import { useUserStore } from '@/store/user'
import RFB from '@novnc/novnc'
import { buildVncWsUrl, COMMON_VNC_SHORTCUTS, PRIMARY_VNC_SHORTCUT, refreshVncViewport, sendTextToVnc, sendVncShortcut } from '@/utils/vnc'

const router = useRouter()

const props = defineProps({
  vmName: { type: String, required: true },
  vmStatus: { type: String, default: '' },
  guestPassword: { type: String, default: '' }
})

const canConnectVnc = computed(() => props.vmStatus === 'running' || props.vmStatus === 'paused')
const canPasteGuestPassword = computed(() => connected.value && !!props.guestPassword)
const pastePasswordTip = computed(() => {
  if (!connected.value) {
    return '请先连接 VNC'
  }
  if (!props.guestPassword) {
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

// VNC 状态
const vncInfo = ref({ enabled: false, port: '', auth: '', has_password: false, exposed: false })
const connected = ref(false)
const connecting = ref(false)
const errorMsg = ref('')
const isFullscreen = ref(false)
const pastingGuestPassword = ref(false)
const sendingCustomText = ref(false)

// 对话框
const showEnableDialog = ref(false)
const showPasswordDialog = ref(false)
const showSendTextDialog = ref(false)
const enablePassword = ref('')
const newPassword = ref('')
const customText = ref('')
const enableLoading = ref(false)
const passwdLoading = ref(false)

// noVNC
const vncContainer = ref(null)
let rfb = null

// 获取 VNC 状态
const fetchVncStatus = async () => {
  try {
    const res = await getVncStatus(props.vmName)
    vncInfo.value = res.data
  } catch (err) {
    console.error('获取 VNC 状态失败', err)
  }
}

// 构建 WebSocket URL
const getWsUrl = () => {
  const userStore = useUserStore()
  return buildVncWsUrl(props.vmName, userStore.token)
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
    rfb.resizeSession = false // QEMU 不支持客户端调整分辨率

    rfb.addEventListener('connect', () => {
      connected.value = true
      connecting.value = false
      errorMsg.value = ''
      // 连接成功后延迟触发画布重新缩放，解决初始空白问题
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
      // VNC 需要密码认证
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
  if (!props.guestPassword) {
    ElMessage.warning('当前未保存虚拟机登录密码')
    return
  }

  pastingGuestPassword.value = true
  try {
    await sendTextToVnc(rfb, props.guestPassword)
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

// 在新窗口中打开 VNC
const openInNewWindow = () => {
  const url = router.resolve({
    name: 'VncWindow',
    params: { id: props.vmName }
  }).href
  // 计算窗口大小和位置（居中）
  const width = Math.min(1280, screen.availWidth - 100)
  const height = Math.min(800, screen.availHeight - 100)
  const left = Math.round((screen.availWidth - width) / 2)
  const top = Math.round((screen.availHeight - height) / 2)
  window.open(
    url,
    `vnc_${props.vmName}`,
    `width=${width},height=${height},left=${left},top=${top},menubar=no,toolbar=no,location=no,status=no,resizable=yes,scrollbars=no`
  )
}

// 开启 VNC
const handleEnableVnc = async () => {
  enableLoading.value = true
  try {
    await enableVnc(props.vmName, enablePassword.value)
    ElMessage.success('VNC 已开启' + (props.vmStatus === 'running' ? '，虚拟机正在重启' : ''))
    showEnableDialog.value = false
    enablePassword.value = ''
    // 等待虚拟机重启后刷新状态
    setTimeout(fetchVncStatus, 3000)
  } catch (err) {
    console.error(err)
  } finally {
    enableLoading.value = false
  }
}

// 关闭 VNC
const handleDisableVnc = async () => {
  try {
    await ElMessageBox.confirm(
      '关闭 VNC 需要重启虚拟机才能生效，确定要关闭吗？',
      '关闭 VNC',
      { confirmButtonText: '确认关闭', cancelButtonText: '取消', type: 'warning' }
    )
    disconnect()
    await disableVnc(props.vmName)
    ElMessage.success('VNC 已关闭')
    setTimeout(fetchVncStatus, 3000)
  } catch (err) {
    if (err !== 'cancel') console.error(err)
  }
}

// 切换 VNC 对外暴露
const handleExposeVnc = async (expose) => {
  try {
    if (expose) {
      await ElMessageBox.confirm(
        '⚠️ 严重安全风险！\n\n' +
        '开启对外暴露后，VNC 端口将监听 0.0.0.0，任何人都可以通过网络直接连接虚拟机 VNC。\n\n' +
        '强烈建议：\n' +
        '• 务必设置 VNC 密码\n' +
        '• 使用防火墙限制访问来源\n' +
        '• 仅在必要时临时开启\n\n' +
        '此操作需要重启虚拟机才能生效。确定要开启吗？',
        '☢ 开启 VNC 对外暴露',
        { confirmButtonText: '我已了解风险，确认开启', cancelButtonText: '取消', type: 'error', dangerouslyUseHTMLString: false }
      )
    } else {
      await ElMessageBox.confirm(
        '关闭对外暴露后，VNC 将仅监听 127.0.0.1，只能通过面板 WebSocket 代理访问。\n\n此操作需要重启虚拟机才能生效。',
        '关闭 VNC 对外暴露',
        { confirmButtonText: '确认关闭', cancelButtonText: '取消', type: 'info' }
      )
    }
    disconnect()
    await exposeVnc(props.vmName, expose)
    ElMessage.success(expose ? 'VNC 已开启对外暴露，虚拟机正在重启' : 'VNC 已关闭对外暴露，虚拟机正在重启')
    setTimeout(fetchVncStatus, 5000)
  } catch (err) {
    if (err !== 'cancel') console.error(err)
  }
}

// 修改 VNC 密码
const handleChangePassword = async () => {
  if (!newPassword.value) {
    ElMessage.warning('请输入新密码')
    return
  }
  passwdLoading.value = true
  try {
    await changeVncPassword(props.vmName, newPassword.value)
    ElMessage.success('VNC 密码已修改（即时生效）')
    showPasswordDialog.value = false
    newPassword.value = ''
    fetchVncStatus()
  } catch (err) {
    console.error(err)
  } finally {
    passwdLoading.value = false
  }
}

// 监听 vmName 变化
watch(() => props.vmName, () => {
  disconnect()
  fetchVncStatus()
})

onMounted(() => {
  fetchVncStatus()
  document.addEventListener('fullscreenchange', handleFullscreenChange)
})

onBeforeUnmount(() => {
  disconnect()
  closeSendTextDialog()
  document.removeEventListener('fullscreenchange', handleFullscreenChange)
})
</script>

<style scoped>
.vnc-console {
  width: 100%;
}

.vnc-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
  gap: 10px;
}

.toolbar-left {
  display: flex;
  align-items: center;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.toolbar-inline-block {
  display: inline-block;
}

.vnc-screen {
  width: 100%;
  min-height: 480px;
  background: #1a1a2e;
  border-radius: 8px;
  overflow: hidden;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 连接后取消 flex 居中，避免与 noVNC 内部布局冲突 */
.vnc-screen.vnc-active {
  display: block;
}

.vnc-screen.vnc-connected {
  min-height: 600px;
}

.vnc-screen.vnc-fullscreen {
  border-radius: 0;
}

.vnc-placeholder {
  text-align: center;
  color: #909399;
}

.vnc-placeholder p {
  margin-top: 16px;
  font-size: 16px;
}

.vnc-placeholder .error-msg {
  color: #F56C6C;
  font-size: 14px;
}

/* noVNC 画布样式 */
.vnc-screen :deep(canvas) {
  max-width: 100%;
  max-height: 100%;
}

@media (max-width: 768px) {
  .vnc-toolbar {
    padding: 8px 12px;
    gap: 6px;
  }

  .vnc-screen {
    height: 50vh;
    min-height: 300px;
  }

  .toolbar-right .el-button {
    font-size: 12px;
    padding: 5px 10px;
  }

  .toolbar-inline-block {
    display: none;
  }
}
</style>
