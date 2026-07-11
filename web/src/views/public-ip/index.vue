<template>
  <div class="public-ip-page">
    <div class="page-header">
      <div>
        <h2>公网 IP</h2>
        <p>管理 1:1 NAT、经典网络和浮动 IP 迁移。</p>
      </div>
      <div class="header-actions">
        <el-button @click="fetchData" :loading="loading">刷新</el-button>
        <el-button type="warning" @click="handleApplyAll">重载规则</el-button>
        <el-button type="primary" @click="handleCreate">新增公网 IP</el-button>
      </div>
    </div>

    <el-alert
      type="warning"
      :closable="false"
      title="公网 IP 绑定、解绑、迁移会触发高风险验证，并通过任务队列应用规则。经典网络需要上游网络支持，VM 内 IP 由用户手动配置。"
      class="top-alert"
    />

    <div class="filter-bar">
      <el-input
        v-model="ipSearchText"
        placeholder="搜索公网 IP"
        clearable
        :prefix-icon="Search"
        size="small"
        style="width: 180px;"
      />
      <el-select
        v-model="ipStatusFilter"
        placeholder="状态筛选"
        clearable
        size="small"
        style="width: 130px;"
      >
        <el-option label="已绑定" value="bound" />
        <el-option label="空闲" value="free" />
        <el-option label="保留" value="reserved" />
      </el-select>
      <el-select
        v-model="ipModeFilter"
        placeholder="模式筛选"
        clearable
        size="small"
        style="width: 140px;"
      >
        <el-option label="1:1 NAT" value="nat" />
        <el-option label="经典网络-路由" value="classic_route" />
        <el-option label="经典网络-桥接" value="classic_bridge" />
      </el-select>
    </div>
    <el-table :data="paginatedIPData" border v-loading="loading" style="width: 100%;">
      <el-table-column prop="ip" label="公网 IP" width="150" />
      <el-table-column label="掩码/网关" min-width="190">
        <template #default="{ row }">
          <div>{{ row.cidr || '-' }}</div>
          <div class="muted">网关：{{ row.gateway || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="uplink_if" label="出口网卡" width="120">
        <template #default="{ row }">{{ row.uplink_if || '自动检测' }}</template>
      </el-table-column>
      <el-table-column label="支持模式" min-width="220">
        <template #default="{ row }">
          <el-tag v-for="label in row.mode_labels || []" :key="label" size="small" class="mode-tag">
            {{ label }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.binding ? 'success' : (row.status === 'reserved' ? 'warning' : 'info')">
            {{ row.binding ? '已绑定' : (row.status === 'reserved' ? '保留' : '空闲') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="绑定 VM" min-width="230">
        <template #default="{ row }">
          <template v-if="row.binding">
            <div>{{ row.binding.vm_name }} <el-tag size="small">{{ modeLabel(row.binding.mode) }}</el-tag></div>
            <div class="muted">用户：{{ row.binding.username }}</div>
            <div class="muted">私网：{{ row.binding.vm_private_ip || '-' }}</div>
            <div class="muted">运行态：{{ row.binding.runtime_status || '-' }}</div>
          </template>
          <span v-else class="muted">未绑定</span>
        </template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="140" />
      <el-table-column label="操作" width="360" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="handleEdit(row)">编辑</el-button>
          <el-button size="small" type="primary" v-if="!row.binding" @click="handleBind(row)">绑定</el-button>
          <el-button size="small" type="warning" v-if="row.binding" @click="handleMigrate(row)">迁移</el-button>
          <el-button size="small" type="info" @click="handlePreview(row)">{{ row.binding ? '预览' : '试算' }}</el-button>
          <el-button size="small" type="danger" v-if="row.binding" @click="handleUnbind(row)">解绑</el-button>
          <el-button size="small" type="danger" v-else @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-wrap">
      <el-pagination
        v-if="filteredTableData.length > ipPageSize"
        background
        layout="total, prev, pager, next"
        :total="filteredTableData.length"
        :page-size="ipPageSize"
        :current-page="ipCurrentPage"
        @current-change="ipCurrentPage = $event"
      />
    </div>

    <!-- ===== 移动端卡片视图 ===== -->
    <div class="mobile-card-list">
      <el-card
        v-for="row in paginatedIPData"
        :key="row.id"
        class="ip-mobile-card"
        shadow="hover"
      >
        <div class="ip-card-header">
          <div class="ip-card-name-row">
            <span class="ip-card-ip">{{ row.ip }}</span>
            <el-tag :type="row.binding ? 'success' : (row.status === 'reserved' ? 'warning' : 'info')" size="small">
              {{ row.binding ? '已绑定' : (row.status === 'reserved' ? '保留' : '空闲') }}
            </el-tag>
          </div>
          <div class="ip-card-meta">
            <span>{{ row.cidr || '-' }} / 网关: {{ row.gateway || '-' }}</span>
          </div>
        </div>
        <div class="ip-card-body">
          <div class="ip-card-info-row">
            <span class="ip-card-label">出口网卡</span>
            <span class="ip-card-value">{{ row.uplink_if || '自动检测' }}</span>
          </div>
          <div class="ip-card-info-row">
            <span class="ip-card-label">支持模式</span>
            <span class="ip-card-value">
              <el-tag v-for="label in row.mode_labels || []" :key="label" size="small" style="margin: 1px 2px;">{{ label }}</el-tag>
            </span>
          </div>
          <template v-if="row.binding">
            <div class="ip-card-binding">
              <div class="ip-card-info-row">
                <span class="ip-card-label">绑定 VM</span>
                <span class="ip-card-value">{{ row.binding.vm_name }}</span>
              </div>
              <div class="ip-card-info-row">
                <span class="ip-card-label">用户</span>
                <span class="ip-card-value">{{ row.binding.username }}</span>
              </div>
              <div class="ip-card-info-row">
                <span class="ip-card-label">私网 IP</span>
                <span class="ip-card-value">{{ row.binding.vm_private_ip || '-' }}</span>
              </div>
            </div>
          </template>
          <div v-if="row.remark" class="ip-card-info-row">
            <span class="ip-card-label">备注</span>
            <span class="ip-card-value">{{ row.remark }}</span>
          </div>
        </div>
        <div class="ip-card-actions">
          <el-button size="small" @click="handleEdit(row)">编辑</el-button>
          <el-button size="small" type="primary" v-if="!row.binding" @click="handleBind(row)">绑定</el-button>
          <el-button size="small" type="warning" v-if="row.binding" @click="handleMigrate(row)">迁移</el-button>
          <el-button size="small" type="info" @click="handlePreview(row)">{{ row.binding ? '预览' : '试算' }}</el-button>
          <el-button size="small" type="danger" v-if="row.binding" @click="handleUnbind(row)">解绑</el-button>
          <el-button size="small" type="danger" v-else @click="handleDelete(row)">删除</el-button>
        </div>
      </el-card>
    </div>

    <el-dialog v-model="ipDialogVisible" :title="ipForm.id ? '编辑公网 IP' : '新增公网 IP'" width="620px" append-to-body>
      <el-form :model="ipForm" label-width="110px">
        <el-form-item label="公网 IP">
          <el-input v-model="ipForm.ip" placeholder="例如 203.0.113.10" />
        </el-form-item>
        <el-form-item label="CIDR/掩码">
          <el-input v-model="ipForm.cidr" placeholder="例如 203.0.113.10/32 或 203.0.113.0/29" />
        </el-form-item>
        <el-form-item label="网关">
          <el-input v-model="ipForm.gateway" placeholder="经典网络给 VM 使用的网关" />
        </el-form-item>
        <el-form-item label="出口网卡">
          <el-input v-model="ipForm.uplink_if" placeholder="留空时自动检测默认出口" />
        </el-form-item>
        <el-form-item label="支持模式">
          <el-checkbox-group v-model="ipForm.modes">
            <el-checkbox label="nat">1:1 NAT</el-checkbox>
            <el-checkbox label="classic_route">经典网络-路由</el-checkbox>
            <el-checkbox label="classic_bridge">经典网络-桥接</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="ipForm.status" style="width: 100%;">
            <el-option label="空闲" value="free" />
            <el-option label="保留" value="reserved" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="ipForm.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ipDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="ipSaving" @click="submitIP">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="bindDialogVisible" :title="bindAction === 'migrate' ? '迁移公网 IP' : '绑定公网 IP'" width="720px" append-to-body>
      <el-form :model="bindForm" label-width="110px">
        <el-alert :title="selectedIP ? `公网 IP：${selectedIP.ip}` : ''" type="info" :closable="false" style="margin-bottom: 16px;" />
        <el-form-item label="用户">
          <el-select v-model="bindForm.username" filterable placeholder="选择用户" style="width: 100%;">
            <el-option v-for="user in users" :key="user.username" :label="user.username" :value="user.username" />
          </el-select>
        </el-form-item>
        <el-form-item label="虚拟机">
          <el-select v-model="bindForm.vm_name" filterable placeholder="选择虚拟机" style="width: 100%;">
            <el-option v-for="vm in filteredVMs" :key="vm.name" :label="vm.name" :value="vm.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="绑定模式">
          <el-select v-model="bindForm.mode" style="width: 100%;">
            <el-option v-for="mode in selectedModes" :key="mode" :label="modeLabel(mode)" :value="mode" />
          </el-select>
        </el-form-item>
        <el-form-item label="VM 私网 IP">
          <el-input v-model="bindForm.vm_private_ip" placeholder="NAT 必填，留空时后端自动解析或静态绑定" />
        </el-form-item>
        <el-form-item>
          <el-button @click="loadPreview" :loading="previewLoading">预览规则</el-button>
        </el-form-item>
      </el-form>

      <div v-if="preview" class="preview-panel">
        <h4>规则预览</h4>
        <pre>{{ preview.commands.join('\n') }}</pre>
        <h4>配置提示</h4>
        <p>{{ preview.config_hint }}</p>
        <el-alert v-for="item in preview.warnings || []" :key="item" :title="item" type="warning" :closable="false" style="margin-top: 8px;" />
      </div>

      <template #footer>
        <el-button @click="bindDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="bindLoading" @click="submitBind">
          {{ bindAction === 'migrate' ? '提交迁移任务' : '提交绑定任务' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="previewDialogVisible" title="规则预览" width="820px" append-to-body>
      <template v-if="preview">
        <pre class="rule-pre">{{ preview.commands.join('\n') }}</pre>
        <el-alert :title="preview.config_hint" type="info" :closable="false" />
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, watch, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import {
  applyPublicIPRules,
  bindPublicIP,
  createPublicIP,
  deletePublicIP,
  getPublicIPs,
  migratePublicIP,
  previewPublicIP,
  unbindPublicIP,
  updatePublicIP
} from '@/api/network'
import { getUserList } from '@/api/user'
import { getVmList } from '@/api/vm'

const tableData = ref([])
const loading = ref(false)
const ipSearchText = ref('')
const ipStatusFilter = ref('')
const ipModeFilter = ref('')
const users = ref([])
const vms = ref([])
const ipDialogVisible = ref(false)
const ipSaving = ref(false)
const bindDialogVisible = ref(false)
const bindLoading = ref(false)
const previewDialogVisible = ref(false)
const previewLoading = ref(false)
const selectedIP = ref(null)
const bindAction = ref('bind')
const preview = ref(null)

const ipForm = reactive({
  id: 0,
  ip: '',
  cidr: '',
  gateway: '',
  uplink_if: '',
  modes: ['nat', 'classic_route', 'classic_bridge'],
  status: 'free',
  remark: ''
})

const bindForm = reactive({
  username: '',
  vm_name: '',
  vm_private_ip: '',
  mode: 'nat'
})

const selectedModes = computed(() => selectedIP.value?.modes?.length ? selectedIP.value.modes : ['nat'])

const filteredVMs = computed(() => {
  if (!bindForm.username) return vms.value
  return vms.value.filter(vm => vm.username === bindForm.username || !vm.username)
})

const modeLabel = (mode) => {
  if (mode === 'nat') return '1:1 NAT'
  if (mode === 'classic_route') return '经典网络-路由'
  if (mode === 'classic_bridge') return '经典网络-桥接'
  return mode || '-'
}

const ipCurrentPage = ref(1)
const ipPageSize = ref(100)

const filteredTableData = computed(() => {
  let data = tableData.value
  if (ipSearchText.value) {
    const q = ipSearchText.value.toLowerCase()
    data = data.filter(row => row.ip.toLowerCase().includes(q))
  }
  if (ipStatusFilter.value) {
    if (ipStatusFilter.value === 'bound') {
      data = data.filter(row => !!row.binding)
    } else if (ipStatusFilter.value === 'free') {
      data = data.filter(row => !row.binding && row.status !== 'reserved')
    } else if (ipStatusFilter.value === 'reserved') {
      data = data.filter(row => row.status === 'reserved')
    }
  }
  if (ipModeFilter.value) {
    data = data.filter(row => (row.modes || []).includes(ipModeFilter.value))
  }
  return data
})

const paginatedIPData = computed(() => {
  const start = (ipCurrentPage.value - 1) * ipPageSize.value
  return filteredTableData.value.slice(start, start + ipPageSize.value)
})

const fetchData = async () => {
  loading.value = true
  try {
    const [ipRes, userRes, vmRes] = await Promise.all([getPublicIPs(), getUserList(), getVmList()])
    tableData.value = ipRes.data || []
    users.value = (userRes.data || [])
    const ownerMap = {}
    users.value.forEach(user => {
      ;(user.vms || []).forEach(vmName => { ownerMap[vmName] = user.username })
    })
    vms.value = (vmRes.data || []).map(vm => ({
      name: vm.name,
      username: ownerMap[vm.name] || ''
    }))
  } finally {
    loading.value = false
  }
}

watch([ipSearchText, ipStatusFilter, ipModeFilter], () => {
  ipCurrentPage.value = 1
})

onMounted(fetchData)

const resetIPForm = () => {
  ipForm.id = 0
  ipForm.ip = ''
  ipForm.cidr = ''
  ipForm.gateway = ''
  ipForm.uplink_if = ''
  ipForm.modes = ['nat', 'classic_route', 'classic_bridge']
  ipForm.status = 'free'
  ipForm.remark = ''
}

const handleCreate = () => {
  resetIPForm()
  ipDialogVisible.value = true
}

const handleEdit = (row) => {
  ipForm.id = row.id
  ipForm.ip = row.ip
  ipForm.cidr = row.cidr || ''
  ipForm.gateway = row.gateway || ''
  ipForm.uplink_if = row.uplink_if || ''
  ipForm.modes = row.modes?.length ? [...row.modes] : ['nat']
  ipForm.status = row.status === 'reserved' ? 'reserved' : 'free'
  ipForm.remark = row.remark || ''
  ipDialogVisible.value = true
}

const submitIP = async () => {
  ipSaving.value = true
  try {
    const payload = {
      ip: ipForm.ip,
      cidr: ipForm.cidr,
      gateway: ipForm.gateway,
      uplink_if: ipForm.uplink_if,
      supported_modes: ipForm.modes.join(','),
      status: ipForm.status,
      remark: ipForm.remark
    }
    if (ipForm.id) {
      await updatePublicIP(ipForm.id, payload)
    } else {
      await createPublicIP(payload)
    }
    ElMessage.success('公网 IP 已保存')
    ipDialogVisible.value = false
    fetchData()
  } finally {
    ipSaving.value = false
  }
}

const openBindDialog = (row, action) => {
  selectedIP.value = row
  bindAction.value = action
  preview.value = null
  const binding = row.binding || {}
  bindForm.username = binding.username || ''
  bindForm.vm_name = binding.vm_name || ''
  bindForm.vm_private_ip = binding.vm_private_ip || ''
  bindForm.mode = binding.mode || selectedModes.value[0] || 'nat'
  bindDialogVisible.value = true
}

const handleBind = (row) => openBindDialog(row, 'bind')
const handleMigrate = (row) => openBindDialog(row, 'migrate')

const loadPreview = async () => {
  if (!selectedIP.value) return
  previewLoading.value = true
  try {
    const res = await previewPublicIP(selectedIP.value.id, { ...bindForm })
    preview.value = res.data
  } finally {
    previewLoading.value = false
  }
}

const handlePreview = async (row) => {
  if (!row.binding) {
    openBindDialog(row, 'bind')
    return
  }
  selectedIP.value = row
  preview.value = null
  const binding = row.binding || {}
  try {
    const res = await previewPublicIP(row.id, {
      username: binding.username || bindForm.username,
      vm_name: binding.vm_name || bindForm.vm_name,
      vm_private_ip: binding.vm_private_ip || bindForm.vm_private_ip,
      mode: binding.mode || row.modes?.[0] || 'nat'
    })
    preview.value = res.data
    previewDialogVisible.value = true
  } catch {}
}

const submitBind = async () => {
  if (!selectedIP.value) return
  bindLoading.value = true
  try {
    const api = bindAction.value === 'migrate' ? migratePublicIP : bindPublicIP
    const res = await api(selectedIP.value.id, { ...bindForm })
    ElMessage.success(res.data?.task_id ? `任务已提交（任务ID: ${res.data.task_id}）` : '任务已提交')
    bindDialogVisible.value = false
    setTimeout(fetchData, 1200)
  } finally {
    bindLoading.value = false
  }
}

const handleUnbind = async (row) => {
  try {
    await ElMessageBox.confirm(`确定解绑公网 IP ${row.ip}？现有公网访问会中断。`, '解绑公网 IP', {
      type: 'warning',
      confirmButtonText: '确定解绑',
      cancelButtonText: '取消'
    })
    const res = await unbindPublicIP(row.id)
    ElMessage.success(res.data?.task_id ? `解绑任务已提交（任务ID: ${res.data.task_id}）` : '解绑任务已提交')
    setTimeout(fetchData, 1200)
  } catch {}
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确定删除公网 IP ${row.ip}？`, '删除公网 IP', {
      type: 'warning',
      confirmButtonText: '确定删除',
      cancelButtonText: '取消'
    })
    await deletePublicIP(row.id)
    ElMessage.success('公网 IP 已删除')
    fetchData()
  } catch {}
}

const handleApplyAll = async () => {
  try {
    await ElMessageBox.confirm('确定按当前绑定关系重新应用全部公网 IP 规则？', '重载公网 IP 规则', {
      type: 'warning',
      confirmButtonText: '确定重载',
      cancelButtonText: '取消'
    })
    const res = await applyPublicIPRules()
    ElMessage.success(res.data?.task_id ? `重载任务已提交（任务ID: ${res.data.task_id}）` : '重载任务已提交')
  } catch {}
}
</script>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
.public-ip-page {
  padding: 0;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.page-header h2 {
  margin: 0 0 6px;
  font-size: 24px;
}

.page-header p {
  margin: 0;
  color: #667085;
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.top-alert {
  margin-bottom: 16px;
}

.mode-tag {
  margin: 2px 4px 2px 0;
}

.muted {
  color: #667085;
  font-size: 12px;
  line-height: 1.7;
}

.preview-panel {
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  padding: 12px;
  background: #fafafa;
}

.preview-panel h4 {
  margin: 0 0 8px;
}

.preview-panel pre,
.rule-pre {
  max-height: 320px;
  overflow: auto;
  padding: 12px;
  background: #111827;
  color: #e5e7eb;
  border-radius: 6px;
  white-space: pre-wrap;
}

/* ===== 默认隐藏移动卡片 ===== */
.mobile-card-list {
  display: none;
}

/* ===== 公网IP移动卡片样式 ===== */
.ip-mobile-card {
  border-radius: 10px;
}

.ip-mobile-card .el-card__body {
  padding: 0;
}

.ip-card-header {
  padding: 14px 16px 10px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.ip-card-name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.ip-card-ip {
  font-size: 16px;
  font-weight: 700;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', Consolas, monospace;
  color: var(--el-color-primary);
}

.ip-card-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.ip-card-body {
  padding: 8px 16px;
}

.ip-card-info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.ip-card-info-row:last-child {
  border-bottom: none;
}

.ip-card-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
  margin-right: 12px;
}

.ip-card-value {
  font-size: 13px;
  color: var(--el-text-color-primary);
  text-align: right;
  word-break: break-all;
}

.ip-card-binding {
  border-top: 1px solid var(--el-border-color-extra-light);
  padding-top: 4px;
  margin-top: 2px;
}

.ip-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 10px 16px;
  border-top: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  border-radius: 0 0 10px 10px;
}

.ip-card-actions .el-button {
  margin-left: 0;
}

@media (max-width: 768px) {
  .public-ip-page .el-table {
    display: none !important;
  }

  .mobile-card-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .page-header {
    flex-direction: column;
    gap: 10px;
  }

  .header-actions {
    width: 100%;
  }
}

@media (max-width: 480px) {
  .ip-card-header {
    padding: 10px 12px 8px;
  }

  .ip-card-body {
    padding: 6px 12px;
  }

  .ip-card-actions {
    padding: 8px 12px;
  }
}
</style>
