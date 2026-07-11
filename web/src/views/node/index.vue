<template>
  <div class="node-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>节点管理</span>
          <el-button type="primary" icon="Plus" @click="openDialog()">添加节点</el-button>
        </div>
      </template>

      <div class="filter-bar">
        <el-input
          v-model="nodeSearchText"
          placeholder="搜索名称"
          clearable
          :prefix-icon="Search"
          size="small"
          style="width: 160px;"
        />
        <el-select
          v-model="nodeStatusFilter"
          placeholder="状态筛选"
          clearable
          size="small"
          style="width: 130px;"
        >
          <el-option label="在线" value="online" />
          <el-option label="异常" value="error" />
          <el-option label="未知" value="unknown" />
        </el-select>
        <el-select
          v-model="nodeEnabledFilter"
          placeholder="启用状态"
          clearable
          size="small"
          style="width: 130px;"
        >
          <el-option label="启用" :value="true" />
          <el-option label="禁用" :value="false" />
        </el-select>
      </div>

      <el-table v-loading="loading" :data="paginatedNodes" border>
        <el-table-column prop="name" label="节点名称" min-width="140" />
        <el-table-column prop="api_base_url" label="面板地址" min-width="220" show-overflow-tooltip />
        <el-table-column label="SSH" min-width="180">
          <template #default="{ row }">
            {{ row.ssh_user }}@{{ row.ssh_host }}:{{ row.ssh_port }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' ? 'success' : row.status === 'error' ? 'danger' : 'info'">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_probe_message" label="最近探测" min-width="220" show-overflow-tooltip />
        <el-table-column label="启用" width="90">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="probe(row)" :loading="probeLoading[row.id]">探测</el-button>
            <el-button size="small" type="primary" plain @click="openDialog(row)">编辑</el-button>
            <el-button size="small" type="danger" plain @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrap">
        <el-pagination
          v-if="filteredNodes.length > nodePageSize"
          background
          layout="total, prev, pager, next"
          :total="filteredNodes.length"
          :page-size="nodePageSize"
          :current-page="nodeCurrentPage"
          @current-change="nodeCurrentPage = $event"
        />
      </div>

      <!-- ===== 移动端卡片视图 ===== -->
      <div class="mobile-card-list">
        <el-card
          v-for="row in paginatedNodes"
          :key="row.id"
          class="node-mobile-card"
          shadow="hover"
        >
          <div class="node-card-header">
            <div class="node-card-name-row">
              <span class="node-card-name">{{ row.name }}</span>
              <el-tag :type="row.status === 'online' ? 'success' : row.status === 'error' ? 'danger' : 'info'" size="small">
                {{ statusText(row.status) }}
              </el-tag>
            </div>
            <div class="node-card-meta">
              <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
            </div>
          </div>
          <div class="node-card-body">
            <div class="node-card-info-row">
              <span class="node-card-label">面板地址</span>
              <span class="node-card-value" :title="row.api_base_url">{{ row.api_base_url }}</span>
            </div>
            <div class="node-card-info-row">
              <span class="node-card-label">SSH</span>
              <span class="node-card-value">{{ row.ssh_user }}@{{ row.ssh_host }}:{{ row.ssh_port }}</span>
            </div>
            <div v-if="row.last_probe_message" class="node-card-info-row">
              <span class="node-card-label">最近探测</span>
              <span class="node-card-value">{{ row.last_probe_message }}</span>
            </div>
          </div>
          <div class="node-card-actions">
            <el-button size="small" @click="probe(row)" :loading="probeLoading[row.id]">探测</el-button>
            <el-button size="small" type="primary" plain @click="openDialog(row)">编辑</el-button>
            <el-button size="small" type="danger" plain @click="remove(row)">删除</el-button>
          </div>
        </el-card>
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑节点' : '添加节点'" width="640px" destroy-on-close append-to-body>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="130px">
        <el-form-item label="节点名称" prop="name">
          <el-input v-model="form.name" placeholder="例如 kvm-test2" />
        </el-form-item>
        <el-form-item label="面板 API 地址" prop="api_base_url">
          <el-input v-model="form.api_base_url" placeholder="http://192.168.11.19:8080" />
        </el-form-item>
        <el-form-item label="API ID" prop="api_key_id">
          <el-input v-model="form.api_key_id" placeholder="目标面板管理员 API ID" />
        </el-form-item>
        <el-form-item label="API Key" :prop="form.id ? '' : 'api_key'">
          <el-input v-model="form.api_key" type="password" show-password placeholder="编辑时留空表示不修改" />
        </el-form-item>
        <el-form-item label="SSH 地址" prop="ssh_host">
          <el-input v-model="form.ssh_host" placeholder="192.168.11.19" />
        </el-form-item>
        <el-form-item label="SSH 端口" prop="ssh_port">
          <el-input-number v-model="form.ssh_port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item label="SSH 用户" prop="ssh_user">
          <el-input v-model="form.ssh_user" placeholder="root" />
        </el-form-item>
        <el-form-item label="root 密码" :prop="form.id ? '' : 'ssh_password'">
          <el-input v-model="form.ssh_password" type="password" show-password placeholder="编辑时留空表示不修改" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, watch, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { createNode, deleteNode, listNodes, probeNode, updateNode } from '@/api/node'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const nodes = ref([])
const nodeSearchText = ref('')
const nodeStatusFilter = ref('')
const nodeEnabledFilter = ref(null)
const probeLoading = ref({})
const formRef = ref(null)
const form = reactive({
  id: 0,
  name: '',
  api_base_url: '',
  api_key_id: '',
  api_key: '',
  ssh_host: '',
  ssh_port: 22,
  ssh_user: 'root',
  ssh_password: '',
  enabled: true
})

const rules = {
  name: [{ required: true, message: '请输入节点名称', trigger: 'blur' }],
  api_base_url: [{ required: true, message: '请输入面板 API 地址', trigger: 'blur' }],
  api_key_id: [{ required: true, message: '请输入目标面板 API ID', trigger: 'blur' }],
  api_key: [{ required: true, message: '请输入目标面板 API Key', trigger: 'blur' }],
  ssh_host: [{ required: true, message: '请输入 SSH 地址', trigger: 'blur' }],
  ssh_password: [{ required: true, message: '请输入 root 密码', trigger: 'blur' }]
}

const statusText = (status) => {
  if (status === 'online') return '在线'
  if (status === 'error') return '异常'
  return '未知'
}

const nodeCurrentPage = ref(1)
const nodePageSize = ref(100)

const filteredNodes = computed(() => {
  let data = nodes.value
  if (nodeSearchText.value) {
    const q = nodeSearchText.value.toLowerCase()
    data = data.filter(n => n.name.toLowerCase().includes(q))
  }
  if (nodeStatusFilter.value) {
    data = data.filter(n => n.status === nodeStatusFilter.value)
  }
  if (nodeEnabledFilter.value !== null) {
    data = data.filter(n => n.enabled === nodeEnabledFilter.value)
  }
  return data
})

const paginatedNodes = computed(() => {
  const start = (nodeCurrentPage.value - 1) * nodePageSize.value
  return filteredNodes.value.slice(start, start + nodePageSize.value)
})

const fetchNodes = async () => {
  loading.value = true
  try {
    const res = await listNodes()
    nodes.value = res.data || []
  } finally {
    loading.value = false
  }
}

const resetForm = () => {
  Object.assign(form, {
    id: 0,
    name: '',
    api_base_url: '',
    api_key_id: '',
    api_key: '',
    ssh_host: '',
    ssh_port: 22,
    ssh_user: 'root',
    ssh_password: '',
    enabled: true
  })
}

const openDialog = (row) => {
  resetForm()
  if (row) {
    Object.assign(form, {
      id: row.id,
      name: row.name,
      api_base_url: row.api_base_url,
      api_key_id: row.api_key_id,
      ssh_host: row.ssh_host,
      ssh_port: row.ssh_port || 22,
      ssh_user: row.ssh_user || 'root',
      enabled: row.enabled
    })
  }
  dialogVisible.value = true
}

const submit = async () => {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const payload = { ...form }
    if (form.id) {
      if (!payload.api_key) delete payload.api_key
      if (!payload.ssh_password) delete payload.ssh_password
      await updateNode(form.id, payload)
      ElMessage.success('节点已更新')
    } else {
      await createNode(payload)
      ElMessage.success('节点已创建')
    }
    dialogVisible.value = false
    fetchNodes()
  } finally {
    saving.value = false
  }
}

const probe = async (row) => {
  probeLoading.value[row.id] = true
  try {
    const res = await probeNode(row.id)
    ElMessage.success(res.message || '节点探测通过')
    fetchNodes()
  } finally {
    probeLoading.value[row.id] = false
  }
}

const remove = async (row) => {
  await ElMessageBox.confirm(`确定删除节点 ${row.name} 吗？`, '删除节点', { type: 'warning' })
  await deleteNode(row.id)
  ElMessage.success('节点已删除')
  fetchNodes()
}

watch([nodeSearchText, nodeStatusFilter, nodeEnabledFilter], () => {
  nodeCurrentPage.value = 1
})

onMounted(fetchNodes)
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
.filter-bar {
  padding: 0 4px;
}
.node-page {
  padding: 10px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

/* ===== 默认隐藏移动卡片（必须在 @media 之前） ===== */
.mobile-card-list {
  display: none;
}

@media (max-width: 768px) {
  .node-page {
    padding: 4px;
  }

  .node-page h2 {
    font-size: 16px;
    margin-bottom: 12px;
  }

  /* 隐藏表格，显示卡片 */
  .node-page .el-table {
    display: none !important;
  }

  .mobile-card-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
}

/* ===== 节点移动卡片样式 ===== */
.node-mobile-card {
  border-radius: 10px;
}

.node-mobile-card .el-card__body {
  padding: 0;
}

.node-card-header {
  padding: 14px 16px 10px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.node-card-name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.node-card-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.node-card-meta {
  display: flex;
  gap: 8px;
}

.node-card-body {
  padding: 8px 16px;
}

.node-card-info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.node-card-info-row:last-child {
  border-bottom: none;
}

.node-card-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
  margin-right: 12px;
}

.node-card-value {
  font-size: 13px;
  color: var(--el-text-color-primary);
  text-align: right;
  word-break: break-all;
  max-width: 65%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 10px 16px;
  border-top: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  border-radius: 0 0 10px 10px;
}

.node-card-actions .el-button {
  margin-left: 0;
}

@media (max-width: 480px) {
  .node-card-header {
    padding: 10px 12px 8px;
  }

  .node-card-body {
    padding: 6px 12px;
  }

  .node-card-actions {
    padding: 8px 12px;
  }
}
</style>
