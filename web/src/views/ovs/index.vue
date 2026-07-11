<template>
  <div class="ovs-page">
    <div class="page-toolbar">
      <div>
        <h2>OVS 网络</h2>
        <p>查看 OVS 网桥、服务、NAT、FORWARD、端口和 DHCP 状态。</p>
      </div>
      <div class="toolbar-actions">
        <el-button icon="Refresh" @click="loadData" :loading="loading">刷新</el-button>
        <el-button icon="View" @click="handleCheck" :loading="checking">一键检测</el-button>
        <el-button type="warning" icon="Tools" @click="handleRepair" :loading="repairing">一键修复</el-button>
      </div>
    </div>

    <el-alert
      class="status-alert"
      :type="status?.healthy ? 'success' : 'warning'"
      :closable="false"
      :title="status?.healthy ? 'OVS 基础状态正常' : 'OVS 基础状态存在需要关注的项目'"
    />

    <el-row :gutter="16">
      <el-col :xs="24" :md="12" :lg="8">
        <el-card shadow="never" class="section-card">
          <template #header>基础状态</template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="网桥">{{ status?.bridge || '-' }}</el-descriptions-item>
            <el-descriptions-item label="网关 IP">{{ status?.gateway_ip || '-' }}</el-descriptions-item>
            <el-descriptions-item label="内网 CIDR">{{ status?.subnet_cidr || '-' }}</el-descriptions-item>
            <el-descriptions-item label="出口网卡">{{ status?.uplink || '未检测到' }}</el-descriptions-item>
            <el-descriptions-item label="网桥存在">{{ yesNo(status?.bridge_exists) }}</el-descriptions-item>
            <el-descriptions-item label="网关配置">{{ yesNo(status?.bridge_has_gateway) }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12" :lg="8">
        <el-card shadow="never" class="section-card">
          <template #header>服务与转发</template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="openvswitch-switch">
              <el-tag :type="status?.openvswitch_service?.active ? 'success' : 'danger'" size="small">
                {{ status?.openvswitch_service?.state || '-' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="OVS dnsmasq">
              <el-tag :type="status?.dnsmasq_service?.active ? 'success' : 'danger'" size="small">
                {{ status?.dnsmasq_service?.state || '-' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="ip_forward">{{ yesNo(status?.ip_forward_enabled) }}</el-descriptions-item>
            <el-descriptions-item label="NAT">{{ yesNo(status?.nat_rule?.exists) }}</el-descriptions-item>
            <el-descriptions-item label="出站 FORWARD">{{ yesNo(status?.forward_out_rule?.exists) }}</el-descriptions-item>
            <el-descriptions-item label="回程 FORWARD">{{ yesNo(status?.forward_return_rule?.exists) }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="8">
        <el-card shadow="never" class="section-card">
          <template #header>健康检查</template>
          <div v-if="checkResult?.repair_suggestions?.length" class="suggestions">
            <el-tag v-for="item in checkResult.repair_suggestions" :key="item" type="warning" effect="plain">{{ item }}</el-tag>
          </div>
          <el-empty v-else description="暂无修复建议" :image-size="80" />
          <el-alert
            v-if="lastRepairTask"
            class="task-alert"
            type="info"
            :closable="false"
            :title="`最近一次修复任务 #${lastRepairTask.id}：${lastRepairTask.message || lastRepairTask.status}`"
          />
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header">
          <span>OVS 端口</span>
          <el-tag :type="ports?.issues?.length ? 'warning' : 'success'" size="small">
            {{ ports?.issues?.length ? `${ports.issues.length} 个异常` : '正常' }}
          </el-tag>
        </div>
      </template>
      <el-table :data="ports?.ports || []" border size="small">
        <el-table-column prop="name" label="端口名" min-width="120" />
        <el-table-column prop="ofport" label="ofport" width="90" />
        <el-table-column prop="type" label="类型" width="100" />
        <el-table-column prop="vm_name" label="关联 VM" min-width="130">
          <template #default="{ row }">{{ row.vm_name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="mac" label="MAC" min-width="150">
          <template #default="{ row }">{{ row.mac || '-' }}</template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" min-width="130">
          <template #default="{ row }">{{ row.ip || '-' }}</template>
        </el-table-column>
        <el-table-column prop="ip_source" label="IP 来源" width="110">
          <template #default="{ row }">{{ ipSourceText(row.ip_source) }}</template>
        </el-table-column>
        <el-table-column label="异常" min-width="180">
          <template #default="{ row }">{{ row.issues?.length ? row.issues.join('；') : '-' }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-row :gutter="16">
      <el-col :xs="24" :lg="12">
        <el-card shadow="never" class="section-card">
          <template #header>DHCP 静态绑定</template>
          <el-table :data="leases?.static_hosts || []" border size="small" max-height="280">
            <el-table-column prop="vm_name" label="VM" min-width="130" />
            <el-table-column prop="mac" label="MAC" min-width="150" />
            <el-table-column prop="ip" label="IP" min-width="130" />
          </el-table>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="12">
        <el-card shadow="never" class="section-card">
          <template #header>DHCP 租约</template>
          <el-table :data="leases?.dhcp_leases || []" border size="small" max-height="280">
            <el-table-column prop="hostname" label="主机名" min-width="120" />
            <el-table-column prop="mac" label="MAC" min-width="150" />
            <el-table-column prop="ip" label="IP" min-width="120" />
            <el-table-column prop="expiry_time" label="过期时间" min-width="160" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header">
          <span>租约冲突</span>
          <el-tag :type="leases?.conflicts?.length ? 'danger' : 'success'" size="small">
            {{ leases?.conflicts?.length ? `${leases.conflicts.length} 个冲突` : '无冲突' }}
          </el-tag>
        </div>
      </template>
      <el-table :data="leases?.conflicts || []" border size="small">
        <el-table-column prop="type" label="类型" width="120" />
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column prop="mac" label="MAC" min-width="150" />
        <el-table-column prop="static_vm_name" label="静态绑定 VM" min-width="130" />
        <el-table-column prop="lease_host" label="租约主机名" min-width="130" />
        <el-table-column prop="message" label="说明" min-width="260" />
      </el-table>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header>NAT/转发规则</template>
      <el-table :data="ruleRows" border size="small">
        <el-table-column prop="name" label="规则" width="160" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.exists ? 'success' : 'danger'" size="small">{{ row.exists ? '存在' : '缺失' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="command" label="检查命令" min-width="420" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { checkOVSNetwork, getOVSLeases, getOVSPorts, getOVSStatus, repairOVSNetwork } from '@/api/ovs'

const loading = ref(false)
const checking = ref(false)
const repairing = ref(false)
const status = ref(null)
const ports = ref(null)
const leases = ref(null)
const checkResult = ref(null)
const lastRepairTask = ref(null)

const ruleRows = computed(() => {
  if (!status.value) return []
  return [
    status.value.nat_rule,
    status.value.forward_out_rule,
    status.value.forward_return_rule
  ].filter(Boolean)
})

const yesNo = (value) => value ? '是' : '否'

const ipSourceText = (source) => {
  const map = {
    static: '静态绑定',
    dhcp: 'DHCP'
  }
  return map[source] || '-'
}

async function loadData() {
  loading.value = true
  try {
    const [statusRes, portsRes, leasesRes] = await Promise.all([
      getOVSStatus(),
      getOVSPorts(),
      getOVSLeases()
    ])
    status.value = statusRes.data
    ports.value = portsRes.data
    leases.value = leasesRes.data
  } finally {
    loading.value = false
  }
}

async function handleCheck() {
  checking.value = true
  try {
    const res = await checkOVSNetwork()
    checkResult.value = res.data
    status.value = res.data?.status
    ports.value = res.data?.ports
    leases.value = res.data?.leases
    ElMessage.success(res.data?.healthy ? 'OVS 网络检查通过' : 'OVS 网络检查完成，请查看建议')
  } finally {
    checking.value = false
  }
}

async function handleRepair() {
  await ElMessageBox.confirm('修复会补齐 OVS 网桥、dnsmasq、ip_forward、NAT 和 FORWARD 规则，确认继续？', '高风险操作', { type: 'warning' })
  repairing.value = true
  try {
    const res = await repairOVSNetwork()
    lastRepairTask.value = res.data
    ElMessage.success(res.message || 'OVS 网络修复任务已提交')
  } finally {
    repairing.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.ovs-page {
  padding: 10px;
}

.page-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  margin-bottom: 12px;
}

.page-toolbar h2 {
  margin: 0 0 6px;
}

.page-toolbar p {
  margin: 0;
  color: #909399;
}

.toolbar-actions,
.card-header,
.suggestions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar-actions {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.card-header {
  justify-content: space-between;
}

.suggestions {
  flex-wrap: wrap;
}

.status-alert,
.section-card {
  margin-bottom: 16px;
}

.task-alert {
  margin-top: 12px;
}

@media (max-width: 900px) {
  .page-toolbar {
    flex-direction: column;
  }
}
</style>
