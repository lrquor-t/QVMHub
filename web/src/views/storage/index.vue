<template>
  <div class="storage-container">
    <h2>我的存储</h2>

    <!-- 未初始化提示 -->
    <el-card v-if="!storageInfo.initialized && !initLoading" class="init-card">
      <el-empty description="存储池尚未初始化">
        <el-button type="primary" @click="handleInit" :loading="initLoading">开通存储池</el-button>
      </el-empty>
    </el-card>

    <!-- 已初始化 -->
    <template v-if="storageInfo.initialized">
      <!-- 未完成的上传（断点续传 · 主动恢复） -->
      <el-card v-if="pendingUploads.length" class="quota-card">
        <template #header>
          <span>未完成的上传（可继续或取消）</span>
        </template>
        <div v-for="item in pendingUploads" :key="item.session_key" style="display:flex; align-items:center; gap:12px; padding:6px 0;">
          <el-tag size="small" :type="item.category === 'iso' ? 'success' : item.category === 'disk' ? 'warning' : 'info'">
            {{ categoryLabel(item.category) }}
          </el-tag>
          <span style="flex-shrink:0; max-width:240px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" :title="item.file_name">{{ item.file_name }}</span>
          <el-progress :percentage="item.progress" :stroke-width="10" style="flex:1; min-width:120px;" />
          <el-button size="small" type="primary" @click="resumeUpload(item)">继续</el-button>
          <el-button size="small" type="danger" plain @click="cancelPending(item)">取消</el-button>
        </div>
      </el-card>
      <input ref="resumeFileInput" type="file" style="display:none" @change="onResumeFileChange" />

      <!-- 配额概览 -->
      <el-card class="quota-card">
        <template #header>
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <span><el-icon style="margin-right: 4px;"><Coin /></el-icon>存储配额</span>
            <el-tag v-if="storageInfo.readonly" type="danger" effect="dark">只读模式</el-tag>
          </div>
        </template>
        <el-alert v-if="storageInfo.readonly" type="error" :closable="false" style="margin-bottom: 16px;">
          存储空间已超出配额，当前处于只读模式。请删除部分文件后恢复正常使用。
        </el-alert>
        <div class="quota-info">
          <div class="quota-item">
            <span class="quota-label">已用空间</span>
            <span class="quota-value">{{ storageInfo.used_display || '0 B' }}</span>
          </div>
          <div class="quota-item">
            <span class="quota-label">配额上限</span>
            <span class="quota-value">{{ storageInfo.max_storage > 0 ? storageInfo.max_storage + ' GB' : '不限' }}</span>
          </div>
        </div>
        <el-progress
          v-if="storageInfo.max_storage > 0"
          :percentage="usagePercentage"
          :color="usagePercentage > 90 ? '#F56C6C' : usagePercentage > 70 ? '#E6A23C' : '#409EFF'"
          :stroke-width="12"
          style="margin-top: 12px;"
        />
      </el-card>

      <!-- 文件管理 -->
      <el-card style="margin-top: 16px;">
        <el-tabs v-model="activeTab" @tab-change="handleTabChange">
          <!-- ISO 标签页 -->
          <el-tab-pane label="ISO 镜像" name="iso">
            <div style="margin-bottom: 16px; display: flex; gap: 8px;">
              <el-upload
                :action="''"
                :auto-upload="false"
                :show-file-list="false"
                :on-change="(file) => handleUpload(file, 'iso')"
                accept=".iso"
                :disabled="storageInfo.readonly"
              >
                <el-button type="primary" icon="Upload" :disabled="storageInfo.readonly">上传 ISO</el-button>
              </el-upload>
              <el-button icon="Refresh" @click="loadFiles('iso')">刷新</el-button>
              <el-button type="success" icon="Connection" @click="showMountDialog('iso')">挂载到虚拟机</el-button>
            </div>
            <el-table :data="isoFiles" border v-loading="filesLoading" empty-text="暂无 ISO 文件">
              <el-table-column prop="name" label="文件名" min-width="200" show-overflow-tooltip />
              <el-table-column label="系统类型" width="120">
                <template #default="{ row }">
                  <el-tag v-if="row.os_type" :type="row.os_type === 'windows' ? 'warning' : 'success'" size="small">
                    {{ row.os_type === 'windows' ? 'Windows' : 'Linux' }}
                  </el-tag>
                  <span v-else style="color: #999;">-</span>
                </template>
              </el-table-column>
              <el-table-column prop="size_text" label="大小" width="120" />
              <el-table-column prop="mod_time" label="上传时间" width="180" />
              <el-table-column label="操作" width="100" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="danger" plain @click="handleDelete(row, 'iso')">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <!-- 文件共享标签页 -->
          <el-tab-pane label="文件共享" name="share">
            <div style="margin-bottom: 16px; display: flex; gap: 8px;">
              <el-upload
                :action="''"
                :auto-upload="false"
                :show-file-list="false"
                :on-change="(file) => handleUpload(file, 'share')"
                :disabled="storageInfo.readonly"
              >
                <el-button type="primary" icon="Upload" :disabled="storageInfo.readonly">上传文件</el-button>
              </el-upload>
              <el-button icon="Refresh" @click="loadFiles('share')">刷新</el-button>
              <el-button type="success" icon="Connection" @click="showMountDialog('share')">挂载到虚拟机</el-button>
            </div>
            <el-table :data="shareFiles" border v-loading="filesLoading" empty-text="暂无共享文件">
              <el-table-column prop="name" label="文件名" min-width="200" show-overflow-tooltip />
              <el-table-column prop="size_text" label="大小" width="120" />
              <el-table-column prop="mod_time" label="上传时间" width="180" />
              <el-table-column label="操作" width="150" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="primary" plain @click="handleDownload(row)">下载</el-button>
                  <el-button size="small" type="danger" plain @click="handleDelete(row, 'share')">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <!-- 虚拟磁盘标签页 -->
          <el-tab-pane label="虚拟磁盘" name="disk">
            <div style="margin-bottom: 16px; display: flex; gap: 8px;">
              <el-upload
                :action="''"
                :auto-upload="false"
                :show-file-list="false"
                :on-change="(file) => handleUpload(file, 'disk')"
                accept=".qcow2,.raw,.vmdk,.vhd,.vhdx,.img"
                :disabled="storageInfo.readonly"
              >
                <el-button type="primary" icon="Upload" :disabled="storageInfo.readonly">上传磁盘文件</el-button>
              </el-upload>
              <el-button icon="Refresh" @click="loadFiles('disk')">刷新</el-button>
            </div>
            <el-alert type="info" :closable="false" style="margin-bottom: 12px;">
              <template #title>此目录存放虚拟机导出的磁盘文件和从外部上传的磁盘文件（支持 .qcow2、.raw、.vmdk、.vhd、.vhdx、.img、.vfd）</template>
            </el-alert>
            <el-table :data="diskFiles" border v-loading="filesLoading" empty-text="暂无磁盘文件">
              <el-table-column prop="name" label="文件名" min-width="250" show-overflow-tooltip />
              <el-table-column prop="size_text" label="大小" width="120" />
              <el-table-column prop="mod_time" label="时间" width="180" />
              <el-table-column label="操作" width="180" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="primary" plain @click="handleDownload(row, 'disk')">下载</el-button>
                  <el-button size="small" type="danger" plain @click="handleDelete(row, 'disk')">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <!-- 挂载管理标签页 -->
          <el-tab-pane label="挂载管理" name="mounts">
            <div style="margin-bottom: 16px;">
              <el-button icon="Refresh" @click="loadMounts">刷新</el-button>
            </div>
            <el-table :data="mountList" border v-loading="mountsLoading" empty-text="暂无挂载">
              <el-table-column prop="vm_name" label="虚拟机" width="180" />
              <el-table-column prop="tag" label="挂载标签" width="200" />
              <el-table-column prop="source" label="源目录" min-width="250" show-overflow-tooltip />
              <el-table-column label="访问模式" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.access_mode === 'readonly' ? 'info' : 'warning'" size="small">
                    {{ row.access_mode === 'readonly' ? '只读' : '读写' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="180" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="primary" plain @click="showMountHelp(row.tag, row.access_mode === 'readonly')">挂载命令</el-button>
                  <el-button size="small" type="danger" plain @click="handleUnmount(row)">卸载</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </el-card>
    </template>

    <!-- 上传进度对话框 -->
    <el-dialog v-model="uploadDialogVisible" title="上传文件" width="480px" :close-on-click-modal="false" :before-close="onUploadDialogClose" append-to-body>
      <div v-if="uploadingFile" style="text-align: center;">
        <p style="margin-bottom: 12px; word-break: break-all;">{{ uploadingFile.name }}</p>
        <el-progress :percentage="uploadProgress" :status="uploadProgress >= 100 ? 'success' : ''" />
        <p v-if="uploadStatus === 'hashing'" style="margin-top: 12px; color: var(--el-text-color-secondary);">
          <el-icon style="vertical-align: middle;"><Loading /></el-icon>
          正在校验文件，准备分片上传...
        </p>
        <p v-else-if="uploadStatus === 'uploading'" style="margin-top: 12px; color: var(--el-text-color-secondary);">
          分片上传中（1MB / 片，并发 3）{{ uploadPaused ? ' · 已暂停' : '' }}
        </p>
        <p v-else-if="uploadStatus === 'done'" style="margin-top: 12px; color: var(--el-color-success);">
          上传完成，正在校验并落盘...
        </p>
        <div v-if="uploadStatus === 'uploading' || uploadStatus === 'hashing'" style="margin-top: 16px;">
          <el-button v-if="uploadStatus === 'uploading'" size="small" @click="togglePauseUpload">
            {{ uploadPaused ? '继续' : '暂停' }}
          </el-button>
          <el-button size="small" type="danger" plain @click="cancelUpload">取消</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- 挂载对话框 -->
    <el-dialog v-model="mountDialogVisible" title="挂载存储池到虚拟机" width="500px" append-to-body>
      <el-form :model="mountForm" label-width="100px">
        <el-form-item label="虚拟机">
          <el-select v-model="mountForm.vm_name" placeholder="选择虚拟机" style="width: 100%;" v-loading="vmListLoading">
            <el-option v-for="vm in vmList" :key="vm.name" :label="vm.name" :value="vm.name">
              <div style="display: flex; justify-content: space-between;">
                <span>{{ vm.name }}</span>
                <el-tag :type="vm.status === 'running' ? 'success' : 'info'" size="small">
                  {{ vm.status === 'running' ? '运行中' : '已关机' }}
                </el-tag>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="存储类别">
          <el-radio-group v-model="mountForm.category">
            <el-radio value="iso">ISO 镜像</el-radio>
            <el-radio value="share">文件共享</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="访问模式">
          <el-switch v-model="mountForm.readonly" active-text="只读" inactive-text="读写" />
        </el-form-item>
        <el-alert type="info" :closable="false">
          <template #title>通过 9p VirtFS 协议共享目录到 Linux 虚拟机（Windows 不支持）</template>
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="mountDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="mountLoading" @click="submitMount">确认挂载</el-button>
      </template>
    </el-dialog>

    <!-- 挂载命令说明对话框 -->
    <el-dialog v-model="mountHelpVisible" title="虚拟机内挂载说明" width="600px" append-to-body>
      <el-alert type="warning" :closable="false" style="margin-bottom: 16px;">
        <template #title>仅支持 Linux 虚拟机，Windows 不支持 9p VirtFS 协议</template>
      </el-alert>
      <div style="background: var(--el-fill-color-light); padding: 16px; border-radius: 8px; font-family: monospace; font-size: 13px; line-height: 2;">
        <p style="color: var(--el-text-color-regular); margin: 0 0 8px;"># 步骤 1: 创建挂载点</p>
        <code style="display: block; background: var(--el-bg-color); color: var(--el-color-warning); border: 1px solid var(--el-border-color-light); padding: 6px 12px; border-radius: 4px;">mkdir -p /mnt/{{ mountHelpTag }}</code>
        <p style="color: var(--el-text-color-regular); margin: 12px 0 8px;"># 步骤 2: 挂载共享目录</p>
        <code style="display: block; background: var(--el-bg-color); color: var(--el-color-warning); border: 1px solid var(--el-border-color-light); padding: 6px 12px; border-radius: 4px;">mount -t 9p -o trans=virtio,version=9p2000.L{{ mountHelpReadonly ? ',ro' : '' }} {{ mountHelpTag }} /mnt/{{ mountHelpTag }}</code>
        <p style="color: var(--el-text-color-regular); margin: 12px 0 8px;"># 步骤 3: 开机自动挂载（可选）</p>
        <code style="display: block; background: var(--el-bg-color); color: var(--el-color-warning); border: 1px solid var(--el-border-color-light); padding: 6px 12px; border-radius: 4px;">echo '{{ mountHelpTag }} /mnt/{{ mountHelpTag }} 9p trans=virtio,version=9p2000.L{{ mountHelpReadonly ? ',ro' : '' }},nofail 0 0' >> /etc/fstab</code>
      </div>
      <template #footer>
        <el-button @click="mountHelpVisible = false">关闭</el-button>
        <el-button type="primary" @click="copyMountCommands">复制命令</el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getStorageInfo, initStorage, getStorageFiles, storageUploadInit, storageUploadChunk, storageUploadComplete, storageUploadCancel, getPendingUploads, deleteStorageFile, getStorageDownloadUrl, mountStorage, unmountStorage, getUserMounts } from '@/api/storage'
import { ChunkUploader } from '@/utils/chunkUploader'
import { getSelfVMs } from '@/api/user'
import { getSettings } from '@/api/settings'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { copyTextWithFallback } from '@/utils/clipboard'

// 存储池信息
const storageInfo = ref({})
const initLoading = ref(false)

// 未完成上传（断点续传主动恢复）
const pendingUploads = ref([])
const resumeCategory = ref('')
const resumeFileInput = ref(null)

const loadPendingUploads = async () => {
  try {
    const res = await getPendingUploads()
    pendingUploads.value = res.data || []
  } catch (err) {
    console.error(err)
  }
}

const categoryLabel = (cat) => ({ iso: 'ISO', share: '共享', disk: '磁盘' }[cat] || cat)

// 继续上传：弹出文件选择器，用户选回同一文件后走 handleUpload（init 会校验 hash 并续传）
const resumeUpload = (item) => {
  resumeCategory.value = item.category
  const accept = item.category === 'iso' ? '.iso'
    : item.category === 'disk' ? '.qcow2,.raw,.vmdk,.vhd,.vhdx,.img' : ''
  if (resumeFileInput.value) {
    resumeFileInput.value.setAttribute('accept', accept)
    resumeFileInput.value.value = ''
    resumeFileInput.value.click()
  }
}

const onResumeFileChange = (e) => {
  const file = e.target.files && e.target.files[0]
  if (file) {
    handleUpload({ raw: file }, resumeCategory.value).finally(() => loadPendingUploads())
  }
}

const cancelPending = async (item) => {
  try {
    await ElMessageBox.confirm(`取消上传「${item.file_name}」并删除已传部分？`, '提示', { type: 'warning' })
    await storageUploadCancel(item.session_key)
    ElMessage.success('已取消')
    loadPendingUploads()
    loadStorageInfo()
  } catch {}
}

const route = useRoute()
const router = useRouter()
const activeTab = ref(route.query.tab || 'iso')

watch(() => route.query.tab, (val) => {
  if (val && val !== activeTab.value) {
    activeTab.value = val
    if (storageInfo.value && storageInfo.value.initialized) {
      loadFiles(val)
    }
  }
})

// 文件列表
const isoFiles = ref([])
const shareFiles = ref([])
const diskFiles = ref([])
const filesLoading = ref(false)

// 上传
const uploadDialogVisible = ref(false)
const uploadingFile = ref(null)
const uploadProgress = ref(0)
const uploadStatus = ref('idle') // hashing | uploading | done
const uploadPaused = ref(false)
const currentUploader = ref(null) // ChunkUploader 实例

// 挂载
const mountDialogVisible = ref(false)
const mountLoading = ref(false)
const vmList = ref([])
const vmListLoading = ref(false)
const mountForm = reactive({
  vm_name: '',
  category: 'share',
  readonly: false
})

// 挂载管理
const mountList = ref([])
const mountsLoading = ref(false)

// 挂载命令说明
const mountHelpVisible = ref(false)
const mountHelpTag = ref('')
const mountHelpReadonly = ref(false)

// 配额使用百分比
const usagePercentage = computed(() => {
  if (!storageInfo.value.max_bytes || storageInfo.value.max_bytes <= 0) return 0
  return Math.min(Math.round((storageInfo.value.used_bytes / storageInfo.value.max_bytes) * 100), 100)
})

// 加载存储池信息
const loadStorageInfo = async () => {
  try {
    const res = await getStorageInfo()
    storageInfo.value = res.data || {}
  } catch (err) {
    console.error(err)
  }
}

// 初始化存储池
const handleInit = async () => {
  initLoading.value = true
  try {
    await initStorage()
    ElMessage.success('存储池初始化成功')
    await loadStorageInfo()
    loadFiles(activeTab.value)
  } catch (err) {
    console.error(err)
  } finally {
    initLoading.value = false
  }
}

// 切换Tab时更新路由查询参数
const handleTabChange = (name) => {
  router.replace({ query: { ...route.query, tab: name } })
  loadFiles(name)
}

// 加载文件列表
const loadFiles = async (tab) => {
  const category = tab || activeTab.value
  if (category === 'mounts') {
    loadMounts()
    return
  }
  filesLoading.value = true
  try {
    const res = await getStorageFiles(category)
    if (category === 'iso') {
      isoFiles.value = res.data || []
    } else if (category === 'disk') {
      diskFiles.value = res.data || []
    } else {
      shareFiles.value = res.data || []
    }
  } catch (err) {
    console.error(err)
  } finally {
    filesLoading.value = false
  }
}

// 上传文件（分片 + 断点续传 + 秒传）
const handleUpload = async (uploadFile, category) => {
  const file = uploadFile.raw
  if (!file) return

  // ISO 文件类型检查
  if (category === 'iso' && !file.name.toLowerCase().endsWith('.iso')) {
    ElMessage.warning('ISO 类别仅支持 .iso 文件')
    return
  }

  // 上传前预检查配额（避免大文件传完后才被拒绝）
  try {
    const infoRes = await getStorageInfo()
    const info = infoRes.data || {}
    if (info.readonly) {
      ElMessage.error('存储空间已满，请先删除部分文件')
      return
    }
    if (info.max_bytes > 0 && (info.used_bytes + file.size) > info.max_bytes) {
      const remaining = info.max_bytes - info.used_bytes
      const remainMB = Math.max(0, remaining / 1024 / 1024).toFixed(1)
      const fileMB = (file.size / 1024 / 1024).toFixed(1)
      ElMessage.error(`存储空间不足，剩余 ${remainMB} MB，文件大小 ${fileMB} MB`)
      return
    }
    // 同步更新本地状态
    storageInfo.value = info
  } catch {
    // 预检查失败不阻止上传，由后端兜底
  }

  uploadingFile.value = file
  uploadProgress.value = 0
  uploadStatus.value = 'hashing'
  uploadPaused.value = false
  uploadDialogVisible.value = true

  // 读取分片上传并发数设置（读取失败或非法时退回默认值）
  let concurrency = 3
  try {
    const v = Number((await getSettings()).data?.chunk_upload_concurrency)
    if (Number.isInteger(v) && v >= 1 && v <= 10) concurrency = v
  } catch {
    // 忽略，使用默认并发数
  }

  const uploader = new ChunkUploader({
    init: storageUploadInit,
    chunk: storageUploadChunk,
    complete: storageUploadComplete,
  }, { concurrency })
  currentUploader.value = uploader

  try {
    await uploader.upload(file, { category }, {
      onHashProgress: () => { uploadStatus.value = 'hashing' },
      onUploadProgress: (ratio) => {
        uploadStatus.value = 'uploading'
        uploadProgress.value = Math.round(ratio * 100)
      },
    })
    uploadProgress.value = 100
    uploadStatus.value = 'done'
    ElMessage.success('文件上传成功')
    uploadDialogVisible.value = false
    loadFiles(category)
    loadStorageInfo() // 刷新配额
    loadPendingUploads() // 成功后会话已 completed，从未完成列表移除
  } catch (err) {
    if (uploader.state === 'canceled') {
      ElMessage.info('已取消上传')
      if (uploader.sessionKey) {
        storageUploadCancel(uploader.sessionKey).catch(() => {})
      }
    } else {
      ElMessage.error('上传失败：' + (err?.message || '请重试'))
    }
    uploadDialogVisible.value = false
  } finally {
    currentUploader.value = null
  }
}

// 暂停 / 继续
const togglePauseUpload = () => {
  const uploader = currentUploader.value
  if (!uploader) return
  if (uploadPaused.value) {
    uploader.resume()
    uploadPaused.value = false
  } else {
    uploader.pause()
    uploadPaused.value = true
  }
}

// 取消上传
const cancelUpload = () => {
  if (currentUploader.value) currentUploader.value.cancel()
}

// 对话框关闭拦截：上传中先取消，由 upload() 的 catch 负责关闭
const onUploadDialogClose = (done) => {
  const uploader = currentUploader.value
  if (uploader && uploader.state !== 'done') {
    uploader.cancel()
    return
  }
  done()
}

// 删除文件
const handleDelete = async (row, category) => {
  try {
    await ElMessageBox.confirm(`确定删除文件 ${row.name}？`, '提示', { type: 'warning' })
    await deleteStorageFile(category, row.name)
    ElMessage.success('文件已删除')
    loadFiles(category)
    loadStorageInfo()
  } catch {}
}

// 下载文件
const handleDownload = (row, category) => {
  const cat = category || 'share'
  const url = getStorageDownloadUrl(cat, row.name)
  window.open(url, '_blank')
}

// 显示挂载对话框
const showMountDialog = async (defaultCategory) => {
  mountForm.vm_name = ''
  mountForm.category = defaultCategory || 'share'
  mountForm.readonly = false
  mountDialogVisible.value = true

  // 加载VM列表
  vmListLoading.value = true
  try {
    const res = await getSelfVMs()
    vmList.value = res.data || []
  } catch (err) {
    console.error(err)
  } finally {
    vmListLoading.value = false
  }
}

// 提交挂载
const submitMount = async () => {
  if (!mountForm.vm_name) {
    ElMessage.warning('请选择虚拟机')
    return
  }
  mountLoading.value = true
  try {
    await mountStorage(mountForm)
    mountDialogVisible.value = false
    // 显示挂载命令说明
    const username = localStorage.getItem('username') || 'user'
    const tag = `user_${username}_${mountForm.category}`
    showMountHelp(tag, mountForm.readonly)
    ElMessage.success('存储池已挂载到虚拟机')
  } catch (err) {
    console.error(err)
  } finally {
    mountLoading.value = false
  }
}

// 显示挂载命令说明
const showMountHelp = (tag, readonly) => {
  mountHelpTag.value = tag
  mountHelpReadonly.value = !!readonly
  mountHelpVisible.value = true
}

// 复制挂载命令
const copyMountCommands = () => {
  const tag = mountHelpTag.value
  const roOpt = mountHelpReadonly.value ? ',ro' : ''
  const cmds = `mkdir -p /mnt/${tag}\nmount -t 9p -o trans=virtio,version=9p2000.L${roOpt} ${tag} /mnt/${tag}\necho '${tag} /mnt/${tag} 9p trans=virtio,version=9p2000.L${roOpt},nofail 0 0' >> /etc/fstab`
  copyTextWithFallback(cmds).then(() => {
    ElMessage.success('命令已复制到剪贴板')
  }).catch(() => {
    ElMessage.warning('复制失败，请手动复制')
  })
}

// 加载挂载列表
const loadMounts = async () => {
  mountsLoading.value = true
  try {
    const res = await getUserMounts()
    mountList.value = res.data || []
  } catch (err) {
    console.error(err)
  } finally {
    mountsLoading.value = false
  }
}

// 卸载
const handleUnmount = async (row) => {
  try {
    await ElMessageBox.confirm(`确定从虚拟机 ${row.vm_name} 卸载挂载 "${row.tag}"？`, '提示', { type: 'warning' })
    await unmountStorage(row.vm_name, row.tag)
    ElMessage.success('已卸载')
    loadMounts()
  } catch {}
}

onMounted(() => {
  loadStorageInfo()
  loadPendingUploads()
  // 延迟加载文件列表（等存储信息返回后再加载）
  setTimeout(() => {
    if (storageInfo.value.initialized) {
      loadFiles(activeTab.value)
    }
  }, 500)
})

</script>

<style scoped>
.init-card {
  margin-top: 20px;
}

.quota-card {
  margin-top: 16px;
}

.quota-info {
  display: flex;
  gap: 40px;
}

.quota-item {
  display: flex;
  flex-direction: column;
}

.quota-label {
  font-size: 13px;
  color: var(--el-text-color-regular);
  margin-bottom: 4px;
}

.quota-value {
  font-size: 20px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

@media (max-width: 768px) {
  .storage-container {
    padding: 4px;
  }

  .storage-container h2 {
    font-size: 16px;
    margin-bottom: 12px;
  }
}
</style>
