<template>
  <div class="lxc-tpl-container">
    <!-- 页面头 -->
    <div class="page-header-bar">
      <div class="page-header-left">
        <el-icon class="page-icon"><Files /></el-icon>
        <div class="page-header-text">
          <h2>LXC 模板</h2>
          <p>共 {{ total }} 个 · 总大小 {{ totalSize }}</p>
        </div>
      </div>
      <div class="page-header-right">
        <el-button type="success" :icon="Refresh" :loading="loading" @click="fetchData">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openImport">导入模板</el-button>
      </div>
    </div>

    <!-- KPI 统计 -->
    <div class="kpi-row kpi-row-3">
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-accent" style="background:var(--el-color-primary)"></div>
        <div class="kpi-body">
          <div class="kpi-head"><el-icon><Files /></el-icon><span>模板总数</span></div>
          <div class="kpi-value">{{ total }}</div>
        </div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-accent" style="background:var(--el-color-success)"></div>
        <div class="kpi-body">
          <div class="kpi-head"><el-icon><Coin /></el-icon><span>总大小</span></div>
          <div class="kpi-value">{{ totalSize }}</div>
        </div>
      </el-card>
      <el-card shadow="hover" class="kpi-card">
        <div class="kpi-accent" style="background:var(--el-color-warning)"></div>
        <div class="kpi-body">
          <div class="kpi-head"><el-icon><Collection /></el-icon><span>发行版数</span></div>
          <div class="kpi-value">{{ distroCount }}</div>
        </div>
      </el-card>
    </div>

    <!-- 表格 -->
    <div class="lxc-tpl-wrap" v-loading="loading">
      <el-table :data="tableData" style="width: 100%">
        <el-table-column label="名称" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="tpl-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="系统" min-width="150">
          <template #default="{ row }">
            <span class="distro-tag" :style="distroStyle(row.distro)">
              {{ [row.distro, row.release].filter(Boolean).join(' ') || '-' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="arch" label="架构" width="90" align="center" />
        <el-table-column prop="backing" label="后端" width="100" align="center" />
        <el-table-column label="rootfs 大小" width="120" align="center">
          <template #default="{ row }">{{ formatSize(row.rootfs_size_bytes) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.disabled ? 'info' : 'success'" size="small" effect="light">{{ row.disabled ? '禁用' : '启用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180" align="center">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="130" fixed="right" align="center">
          <template #default="{ row }">
            <el-tooltip content="模板设置" placement="top">
              <el-button size="small" type="primary" plain circle :icon="Setting" @click="openEdit(row)" />
            </el-tooltip>
            <el-tooltip content="删除模板" placement="top">
              <el-button size="small" type="danger" plain circle :icon="Delete" @click="handleDelete(row)" />
            </el-tooltip>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无模板">
            <el-button type="primary" :icon="Plus" @click="openImport">导入第一个模板</el-button>
          </el-empty>
        </template>
      </el-table>
    </div>

    <el-dialog v-model="importVisible" title="导入 LXC 模板" width="560px" append-to-body :close-on-click-modal="false" @close="onImportDialogClose">
      <el-form :model="importForm" label-width="110px" class="import-form">
        <div class="section-title">基本信息</div>
        <el-form-item label="模板名称" required>
          <el-input v-model="importForm.name" placeholder="如 ubuntu22（小写字母/数字/连字符）" />
        </el-form-item>

        <div class="section-title">来源</div>
        <el-form-item label="导入来源" required>
          <el-radio-group v-model="importForm.mode">
            <el-radio value="upload">上传文件</el-radio>
            <el-radio value="host">主机绝对路径</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="importForm.mode === 'upload'" label="rootfs 包" required>
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            :on-change="onFileChange"
            :on-remove="onFileRemove"
            :on-exceed="onFileExceed"
            accept=".tar,.tar.gz,.tgz,.tar.xz"
          >
            <el-button type="primary" plain :icon="UploadFilled">选择文件</el-button>
            <template #tip>
              <div class="el-upload__tip">支持 .tar / .tar.gz / .tgz / .tar.xz，需含顶层 rootfs 目录与 rootfs/etc/os-release</div>
            </template>
          </el-upload>
          <div v-if="uploading" class="upload-progress-wrap">
            <div class="upload-status">{{ uploadStatus || '处理中…' }} <span class="upload-status-pct">{{ uploadProgress }}%</span></div>
            <el-progress :percentage="uploadProgress" :stroke-width="16" :show-text="false" />
          </div>
        </el-form-item>
        <el-form-item v-else label="主机路径" required>
          <el-input v-model="importForm.host_path" placeholder="宿主机上 rootfs tarball 的绝对路径">
            <template #append>
              <el-button :loading="probing" @click="handleProbe">校验</el-button>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item v-if="probeOk" label="发行版">
          <el-input v-model="importForm.distro" placeholder="ubuntu / debian / ...（校验后自动回填）" />
        </el-form-item>
        <el-form-item v-if="probeOk" label="版本">
          <el-input v-model="importForm.release" placeholder="22.04 / bookworm / ...（校验后自动回填）" />
        </el-form-item>
        <el-form-item v-if="probeOk" label="架构">
          <el-select v-model="importForm.arch" disabled placeholder="校验后自动带出" style="width:100%">
            <el-option label="amd64" value="amd64" />
            <el-option label="arm64" value="arm64" />
          </el-select>
          <div class="arch-hint">跟随宿主机架构，不可更改</div>
        </el-form-item>

        <div class="section-title">元数据</div>
        <el-form-item label="创建后命令">
          <el-input v-model="importForm.post_create_command" type="textarea" :rows="2" placeholder="可选：首次创建容器后 lxc-attach 执行" />
        </el-form-item>
        <el-alert v-if="probeMsg" :type="probeOk ? 'success' : 'error'" :title="probeMsg" :closable="false" show-icon />
      </el-form>
      <template #footer>
        <el-button @click="importVisible = false">取消</el-button>
        <el-button v-if="importForm.mode === 'upload'" type="warning" :loading="uploading" @click="handleUploadAndProbe">
          {{ uploadedPath ? '重新上传并校验' : '上传并校验' }}
        </el-button>
        <el-button type="primary" :loading="importing" :disabled="!canImport" @click="handleImport">导入</el-button>
      </template>
    </el-dialog>

    <!-- 模板设置弹窗 -->
    <el-dialog v-model="editVisible" title="模板设置" width="520px" append-to-body :close-on-click-modal="false">
      <el-form :model="editForm" label-width="100px">
        <el-form-item label="模板名称">
          <el-input v-model="editForm.name" disabled />
        </el-form-item>
        <el-form-item label="显示名称">
          <el-input v-model="editForm.display_name" placeholder="用户侧显示名（克隆下拉标题等）" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="editForm.description" type="textarea" :rows="2" placeholder="可选" />
        </el-form-item>
        <el-form-item label="启用克隆">
          <el-switch v-model="editForm.clone_visible" />
          <span class="form-hint">关闭后该模板不出现在用户克隆下拉中</span>
        </el-form-item>
        <el-form-item label="禁用模板">
          <el-switch v-model="editForm.disabled" />
          <span class="form-hint">禁用后该模板不可用于创建容器</span>
        </el-form-item>
        <el-form-item label="创建后命令">
          <el-input v-model="editForm.post_create_command" type="textarea" :rows="3" placeholder="可选：首次创建容器后 lxc-attach 执行" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="editLoading" @click="handleSaveEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Delete, UploadFilled, Files, Coin, Collection, Setting } from '@element-plus/icons-vue'
import { ChunkUploader } from '@/utils/chunkUploader'
import { getSettings } from '@/api/settings'
import {
  getLXCTemplateList, finalizeLXCTemplate, deleteLXCTemplate, updateLXCTemplate,
  lxcTemplateUploadInit, lxcTemplateUploadChunk, lxcTemplateUploadComplete, lxcTemplateUploadCancel,
  probeLXCTemplate
} from '@/api/lxc'

const tableData = ref([])
const loading = ref(false)
const importVisible = ref(false)
const uploadRef = ref(null) // el-upload 实例，重开弹窗时 clearFiles 清掉残留选择
const importing = ref(false)
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadStatus = ref('') // 计算校验值… / 上传中… / 校验中…
const probing = ref(false)
const uploadedPath = ref('')
const rawFile = ref(null)
const probeOk = ref(false)
const probeMsg = ref('')
const importForm = ref({ name: '', mode: 'upload', host_path: '', distro: '', release: '', arch: '', post_create_command: '' })

// 模板设置弹窗
const editVisible = ref(false)
const editLoading = ref(false)
const editForm = ref({ name: '', display_name: '', description: '', clone_visible: true, disabled: false, post_create_command: '' })

const canImport = computed(() => {
  if (!importForm.value.name) return false
  return importForm.value.mode === 'upload' ? !!uploadedPath.value : !!importForm.value.host_path.trim()
})

const resetImportState = () => {
  importForm.value = { name: '', mode: 'upload', host_path: '', distro: '', release: '', arch: '', post_create_command: '' }
  rawFile.value = null
  uploadedPath.value = ''
  uploading.value = false
  uploadProgress.value = 0
  uploadStatus.value = ''
  uploadRef.value?.clearFiles()
  probeOk.value = false
  probeMsg.value = ''
}

const resetProbe = () => {
  probeOk.value = false
  probeMsg.value = ''
}

const openImport = () => {
  resetImportState()
  importVisible.value = true
}

// 对话框关闭：清理已上传但未导入的临时包
const onImportDialogClose = () => {
  const p = uploadedPath.value
  resetImportState()
  if (p) lxcTemplateUploadCancel(p).catch(() => {})
}

// 从文件名推导模板名：去 .tar/.tar.gz/.tgz/.tar.xz 后缀，小写，非 [a-z0-9-] 换成 -，去首尾 -。
// 结果须满足容器名规则 ^[a-z0-9][a-z0-9-]{1,62}$，否则返回空串（不回填）。
const deriveNameFromFile = (filename) => {
  if (!filename) return ''
  let n = filename.replace(/\.(tar(?:\.gz|\.xz)?|tgz)$/i, '')
  n = n.toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+/, '').replace(/-+$/, '')
  if (!/^[a-z0-9][a-z0-9-]{1,62}$/.test(n)) return ''
  return n
}

const onFileChange = (file) => {
  rawFile.value = file.raw || null
  uploadedPath.value = ''
  resetProbe()
  // 默认用文件名（去后缀）作模板名；用户已手动填写则保留。
  if (!importForm.value.name) {
    const derived = deriveNameFromFile(file.name)
    if (derived) importForm.value.name = derived
  }
}
const onFileRemove = () => {
  rawFile.value = null
  uploadedPath.value = ''
  resetProbe()
}
const onFileExceed = (files) => {
  const [f] = files
  rawFile.value = f || null
  uploadedPath.value = ''
  resetProbe()
  ElMessage.warning('请先移除已选文件后再选择新文件')
}

const handleUploadAndProbe = async () => {
  if (!rawFile.value) {
    ElMessage.warning('请先选择 rootfs 包')
    return
  }
  // 重新上传前清理旧临时包
  if (uploadedPath.value) {
    await lxcTemplateUploadCancel(uploadedPath.value).catch(() => {})
    uploadedPath.value = ''
  }
  uploading.value = true
  uploadProgress.value = 0
  uploadStatus.value = '计算校验值…'
  resetProbe()
  try {
    let concurrency = 3
    try {
      const v = Number((await getSettings()).data?.chunk_upload_concurrency)
      if (Number.isInteger(v) && v >= 1 && v <= 10) concurrency = v
    } catch {
      // 读取并发设置失败，回退默认
    }
    const uploader = new ChunkUploader(
      { init: lxcTemplateUploadInit, chunk: lxcTemplateUploadChunk, complete: lxcTemplateUploadComplete },
      { concurrency }
    )
    const { sessionKey } = await uploader.upload(rawFile.value, {}, {
      // MD5 阶段占 0–10%，上传阶段占 10–100%；阶段标签让用户清楚当前在干嘛
      onHashProgress: (ratio) => { uploadStatus.value = '计算校验值…'; uploadProgress.value = Math.round(ratio * 10) },
      onUploadProgress: (ratio) => { uploadStatus.value = '上传中…'; uploadProgress.value = 10 + Math.round(ratio * 90) },
    })
    uploadedPath.value = sessionKey
    uploadStatus.value = '校验中…'
    uploadProgress.value = 100
    await runProbe(sessionKey)
  } catch (e) {
    // 错误由 request 拦截器提示
  } finally {
    uploading.value = false
  }
}

const handleProbe = async () => {
  const p = importForm.value.host_path.trim()
  if (!p || !p.startsWith('/')) {
    ElMessage.warning('请输入宿主机上的绝对路径')
    return
  }
  await runProbe(p)
}

// path：上传模式传 sessionKey（作 source_path）；主机模式传 host_path
const runProbe = async (path) => {
  probing.value = true
  resetProbe()
  try {
    const isUpload = importForm.value.mode === 'upload'
    const res = await probeLXCTemplate(isUpload ? { source_path: path } : { host_path: path })
    const d = res.data || {}
    probeOk.value = !!d.ok
    if (d.ok) {
      const tag = [d.distro, d.release].filter(Boolean).join(' ')
      probeMsg.value = '校验通过' + (tag ? '：' + tag : '')
      if (d.distro && !importForm.value.distro) importForm.value.distro = d.distro
      if (d.release && !importForm.value.release) importForm.value.release = d.release
      // 架构由宿主机决定，始终以 probe 返回为准（非用户可编辑）
      if (d.arch) importForm.value.arch = d.arch
    } else {
      probeMsg.value = d.error || '校验失败'
    }
  } catch (e) {
    // 错误由 request 拦截器提示
  } finally {
    probing.value = false
  }
}

const handleImport = async () => {
  if (!importForm.value.name) {
    ElMessage.warning('请填写模板名称')
    return
  }
  importing.value = true
  try {
    const payload = {
      name: importForm.value.name,
      display_name: importForm.value.name,
      distro: importForm.value.distro,
      release: importForm.value.release,
      arch: importForm.value.arch,
      post_create_command: importForm.value.post_create_command,
    }
    if (importForm.value.mode === 'upload') {
      payload.source_path = uploadedPath.value
    } else {
      payload.host_path = importForm.value.host_path.trim()
    }
    const res = await finalizeLXCTemplate(payload)
    ElMessage.success(res.message || '导入任务已提交，请在任务中心查看进度')
    uploadedPath.value = '' // 临时包交给异步导入任务，onImportDialogClose 不再取消清理
    importVisible.value = false
    // 模板行在异步任务完成后才会出现；用户可在任务中心跟进，或稍后点刷新。
    fetchData()
  } catch (e) {
    // 错误由 request 拦截器提示
  } finally {
    importing.value = false
  }
}

const formatSize = (b) => {
  if (!b) return '-'
  const mb = b / 1024 / 1024
  if (mb < 1024) return mb.toFixed(0) + ' MB'
  return (mb / 1024).toFixed(2) + ' GB'
}

// 格式化后端返回的时间（Go time.Time 的 RFC3339Nano，如 2026-07-03T10:30:49.253794241+08:00）
// → 2026-07-03 10:30:49。解析失败时回退原值。
const formatTime = (t) => {
  if (!t) return '-'
  const d = new Date(t)
  if (isNaN(d.getTime())) return t
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// 发行版品牌色（Material 500 量级，亮色，深浅背景均可读）；
// alpha-tint 背景随卡背自适应，免去浅/深双套 CSS。
const DISTRO_COLORS = {
  ubuntu: '#E95420', debian: '#D32F2F', alpine: '#29B6F6', centos: '#AB47BC',
  rocky: '#10B981', alma: '#EF5350', fedora: '#5C6BC0', arch: '#26C6DA',
  suse: '#42A5F5', other: '#909399',
}
const distroKey = (distro) => {
  const d = (distro || '').toLowerCase()
  if (d.includes('ubuntu')) return 'ubuntu'
  if (d.includes('debian')) return 'debian'
  if (d.includes('alpine')) return 'alpine'
  if (d.includes('centos')) return 'centos'
  if (d.includes('rocky')) return 'rocky'
  if (d.includes('alma')) return 'alma'
  if (d.includes('fedora')) return 'fedora'
  if (d.includes('arch')) return 'arch'
  if (d.includes('suse') || d.includes('opensuse')) return 'suse'
  return 'other'
}
const distroStyle = (distro) => {
  const c = DISTRO_COLORS[distroKey(distro)]
  return { color: c, backgroundColor: c + '1f', border: '1px solid ' + c + '3d' }
}

// KPI 统计（从列表数据派生；totalSize 引用 formatSize，computed 懒求值，渲染时 formatSize 已就绪）
const total = computed(() => tableData.value.length)
const totalSize = computed(() => formatSize(tableData.value.reduce((s, r) => s + (r.rootfs_size_bytes || 0), 0)))
const distroCount = computed(() => new Set(tableData.value.map(r => r.distro).filter(Boolean)).size)

const fetchData = async () => {
  loading.value = true
  try {
    const res = await getLXCTemplateList()
    tableData.value = res.data || []
  } catch (e) {
    ElMessage.error('获取模板列表失败')
  } finally {
    loading.value = false
  }
}

const handleDelete = async (row) => {
  await ElMessageBox.confirm(`确认删除模板 ${row.name}？其金基底容器将一并销毁。`, '删除模板', { type: 'warning' })
  try {
    await deleteLXCTemplate(row.name)
    ElMessage.success('已删除')
    fetchData()
  } catch (e) {}
}

const openEdit = (row) => {
  editForm.value = {
    name: row.name,
    display_name: row.display_name || '',
    description: row.description || '',
    // clone_visible 默认 true：后端缺省 true，row 无该字段时按 true 回填
    clone_visible: row.clone_visible !== false,
    disabled: !!row.disabled,
    post_create_command: row.post_create_command || '',
  }
  editVisible.value = true
}

const handleSaveEdit = async () => {
  editLoading.value = true
  try {
    const res = await updateLXCTemplate(editForm.value.name, {
      display_name: editForm.value.display_name,
      description: editForm.value.description,
      clone_visible: editForm.value.clone_visible,
      disabled: editForm.value.disabled,
      post_create_command: editForm.value.post_create_command,
    })
    ElMessage.success(res.message || '模板设置已更新')
    editVisible.value = false
    fetchData()
  } catch (e) {
    // 错误由 request 拦截器统一提示
  } finally {
    editLoading.value = false
  }
}

onMounted(fetchData)
</script>

<style scoped>
.lxc-tpl-container {
  padding: 10px;
}

/* 页面头 page-header-bar */
.page-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 4px 16px;
}
.page-header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}
.page-icon {
  font-size: 22px;
  color: var(--el-color-primary);
}
.page-header-text h2 {
  margin: 0;
  font-size: 19px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--el-text-color-primary);
}
.page-header-text p {
  margin: 2px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.page-header-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

/* KPI 统计卡 */
.kpi-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
  padding: 0 4px 16px;
}
.kpi-row-3 {
  grid-template-columns: repeat(3, 1fr);
}
.kpi-card {
  border-radius: 12px;
  border: none;
  transition: transform 0.2s var(--app-transition-fast, 0.15s), box-shadow 0.2s;
}
.kpi-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--app-shadow-lg);
}
.kpi-card :deep(.el-card__body) {
  padding: 0;
}
.kpi-accent {
  height: 3px;
  border-radius: 12px 12px 0 0;
}
.kpi-body {
  padding: 14px 16px;
}
.kpi-head {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.kpi-head .el-icon {
  font-size: 15px;
}
.kpi-value {
  font-size: 24px;
  font-weight: 800;
  line-height: 1.2;
  margin-top: 4px;
  color: var(--el-text-color-primary);
}

/* 表格容器（hover-lift） */
.lxc-tpl-wrap {
  background: var(--app-bg-card);
  border-radius: 12px;
  box-shadow: var(--app-shadow-sm);
  border: 1px solid var(--app-border-light);
  padding: 2px;
  overflow: hidden;
  transition: box-shadow 0.2s var(--app-transition-fast, 0.15s);
}
.lxc-tpl-wrap:hover {
  box-shadow: var(--app-shadow-lg);
}

.tpl-name {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

/* 发行版标签（颜色由 distroStyle 内联，alpha-tint 深浅模式自适应） */
.distro-tag {
  display: inline-flex;
  align-items: center;
  padding: 2px 10px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.2px;
  white-space: nowrap;
}

/* 导入弹窗 + section 标题 */
.import-form {
  padding-top: 6px;
}
.section-title {
  font-size: 16px;
  font-weight: 700;
  padding-left: 10px;
  border-left: 4px solid var(--el-color-primary);
  margin: 18px 0 14px;
  color: var(--el-text-color-primary);
}
.section-title:first-child {
  margin-top: 4px;
}

/* 上传进度条：el-progress 放在 el-form-item（flex 容器）里会塌缩成只剩百分比文字，
   故显式撑满宽度并给出可见的轨道/填充色，避免被主题变量吃掉。 */
.upload-progress-wrap {
  width: 100%;
  margin-top: 8px;
}
.upload-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}
.upload-status-pct {
  font-variant-numeric: tabular-nums;
  color: var(--el-text-color-regular);
}
.upload-progress-wrap :deep(.el-progress) {
  width: 100%;
}
.upload-progress-wrap :deep(.el-progress-bar) {
  flex: 1;
}
.upload-progress-wrap :deep(.el-progress-bar__outer) {
  background-color: rgba(125, 125, 125, 0.18);
}
.upload-progress-wrap :deep(.el-progress-bar__inner) {
  background-color: var(--el-color-primary, #409eff);
  transition: width 0.2s ease;
}
html.dark .upload-progress-wrap :deep(.el-progress-bar__outer) {
  background-color: rgba(255, 255, 255, 0.12);
}

/* 架构只读提示 */
.arch-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
  line-height: 1.4;
}

/* 设置弹窗开关旁的提示文字 */
.form-hint {
  margin-left: 10px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

/* ===== 移动端 ===== */
@media (max-width: 768px) {
  .page-header-bar {
    flex-wrap: wrap;
    gap: 10px;
  }
  .kpi-row,
  .kpi-row-3 {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
