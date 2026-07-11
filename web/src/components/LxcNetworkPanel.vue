<template>
  <div class="lxc-network-panel">
    <!-- 网卡列表 -->
    <el-card shadow="hover" class="cfg-card">
      <div class="section-title">网卡</div>
      <div class="vpc-static-ip-header">
        <span class="cfg-hint">主网卡（序号 0）可绑定静态 IP；附加网卡由各自交换机 DHCP 分配。</span>
        <div class="header-actions">
          <el-button v-if="isAdmin" type="primary" size="small" plain icon="Plus" @click="openAdd">添加网口</el-button>
          <el-button size="small" icon="Refresh" :loading="loading" @click="load">刷新</el-button>
        </div>
      </div>
      <el-table :data="interfaces" border size="small" v-loading="loading" style="margin-top: 10px;">
        <el-table-column label="序号" width="80" align="center">
          <template #default="{ row }">
            {{ row.order }}
            <el-tag v-if="row.is_primary" type="info" size="small" style="margin-left:4px;">主</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="MAC" prop="mac" min-width="150" />
        <el-table-column label="IP" min-width="120">
          <template #default="{ row }">
            <code v-if="row.ip">{{ row.ip }}</code>
            <span v-else style="color:var(--el-text-color-placeholder);">-</span>
          </template>
        </el-table-column>
        <el-table-column label="VPC 交换机" min-width="170">
          <template #default="{ row }">
            <span v-if="row.switch_id">
              {{ row.switch_name }}
              <el-tag size="small" :type="row.bridge_mode === 'bridge' ? 'warning' : 'info'">
                {{ row.bridge_mode === 'bridge' ? '桥接直通' : (row.cidr || '-') }}
              </el-tag>
            </span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="安全组" min-width="130">
          <template #default="{ row }">
            <span v-if="row.security_group_id">{{ row.security_group_name }}</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="限速(Mbps)" width="120" align="center">
          <template #default="{ row }">
            <span>↓{{ row.bandwidth_inbound_avg || '不限' }} / ↑{{ row.bandwidth_outbound_avg || '不限' }}</span>
          </template>
        </el-table-column>
        <el-table-column v-if="isAdmin" label="操作" width="150" align="center">
          <template #default="{ row }">
            <el-button type="primary" size="small" link @click="openEdit(row)">编辑</el-button>
            <el-popconfirm title="确定删除该网卡？已绑静态 IP 需先解绑。" placement="top" @confirm="onRemove(row)">
              <template #reference>
                <el-button type="danger" size="small" link :loading="submitting">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 静态 IP（绑定到主网卡） -->
    <el-card shadow="hover" class="cfg-card">
      <div class="section-title">静态 IP（主网卡）</div>
      <div style="margin-bottom:10px;">
        <el-button type="primary" size="small" :disabled="!primaryNIC" @click="openBind">绑定 IP</el-button>
      </div>
      <h4>静态绑定</h4>
      <el-table :data="currentBindings" border size="small" v-loading="ipLoading">
        <el-table-column prop="ip" label="IP 地址" />
        <el-table-column prop="mac" label="MAC 地址" />
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-button type="danger" size="small" @click="onUnbind(row)">解绑</el-button>
          </template>
        </el-table-column>
      </el-table>
      <h4 style="margin-top:14px;">DHCP 租约</h4>
      <el-table :data="currentLeases" border size="small">
        <el-table-column prop="hostname" label="主机名" />
        <el-table-column prop="ip" label="IP" />
        <el-table-column prop="mac" label="MAC" />
        <el-table-column prop="expiry_time" label="过期时间" />
      </el-table>
    </el-card>

    <!-- 添加/编辑 网口 -->
    <el-dialog :title="editOrder >= 0 ? '编辑网口' : '添加网口'" v-model="nicVisible" width="460px" append-to-body>
      <el-form :model="nicForm" label-width="100px">
        <el-form-item label="VPC 交换机">
          <el-select v-model="nicForm.switch_id" filterable placeholder="选择交换机" style="width:100%;">
            <el-option v-for="s in switches" :key="s.id" :label="switchLabel(s)" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="安全组" v-if="selectedSwitch && selectedSwitch.bridge_mode !== 'bridge'">
          <el-select v-model="nicForm.security_group_id" clearable placeholder="可选" style="width:100%;">
            <el-option v-for="g in filteredSGs" :key="g.id" :label="g.name + (g.is_default ? '（默认）' : '')" :value="g.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="下行限速">
          <el-input-number v-model="nicForm.bandwidth_inbound_avg" :min="0" /> <span class="cfg-hint">Mbps，0=不限</span>
        </el-form-item>
        <el-form-item label="上行限速">
          <el-input-number v-model="nicForm.bandwidth_outbound_avg" :min="0" /> <span class="cfg-hint">Mbps，0=不限</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="nicVisible = false" :disabled="submitting">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitNic">确定</el-button>
      </template>
    </el-dialog>

    <!-- 绑定 IP -->
    <el-dialog title="绑定静态 IP" v-model="bindVisible" width="420px" append-to-body>
      <el-form :model="bindForm" label-width="100px">
        <el-form-item label="DHCP 租约">
          <el-select v-model="selectedLeaseIP" style="width:100%;" placeholder="选择租约或自动分配" @change="onLease">
            <el-option v-for="l in currentLeases" :key="l.ip" :label="`${l.ip}（${l.hostname || name}）`" :value="l.ip" />
            <el-option label="自动分配 IP" value="" />
          </el-select>
        </el-form-item>
        <el-form-item label="IP 地址" v-if="selectedLeaseIP === ''">
          <el-input v-model="bindForm.ip" placeholder="留空自动分配，或输入完整IP/最后一位" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="bindVisible = false" :disabled="bindSubmitting">取消</el-button>
        <el-button type="primary" :loading="bindSubmitting" @click="submitBind">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '@/store/user'
import { listLXCInterfaces, addLXCInterface, updateLXCInterface, removeLXCInterface } from '@/api/lxc'
import { bindStaticIP, unbindStaticIP, getStaticIPList } from '@/api/network'
import { getVPCSwitches, getVPCSecurityGroups } from '@/api/vpc'

const props = defineProps({ name: { type: String, required: true } })
const userStore = useUserStore()
const isAdmin = computed(() => userStore.role === 'admin')

const loading = ref(false)
const submitting = ref(false)
const interfaces = ref([])
const switches = ref([])
const groups = ref([])

const primaryNIC = computed(() => interfaces.value.find(i => i.is_primary))

const load = async () => {
  loading.value = true
  try {
    const res = await listLXCInterfaces(props.name)
    interfaces.value = res.data || []
  } catch (e) {} finally { loading.value = false }
}

// 网口增/改
const nicVisible = ref(false)
const editOrder = ref(-1)
const nicForm = reactive({ switch_id: null, security_group_id: null, bandwidth_inbound_avg: 0, bandwidth_outbound_avg: 0 })
const selectedSwitch = computed(() => switches.value.find(s => s.id === nicForm.switch_id) || null)
const filteredSGs = computed(() => {
  if (!selectedSwitch.value) return groups.value
  // 系统交换机（username 为空，如「基础网络」）是共享的，不按属主过滤；
  // 否则按交换机属主过滤。与 VM NetworkList 一致（其 switch 无 username 时返回全部）。
  if (!selectedSwitch.value.username) return groups.value
  return groups.value.filter(g => !g.username || g.username === selectedSwitch.value.username)
})
// 合并新拉取的安全组（按 id 去重）。
const mergeGroups = (incoming) => {
  const exists = new Set(groups.value.map(g => g.id))
  for (const g of incoming || []) {
    if (!exists.has(g.id)) { groups.value.push(g); exists.add(g.id) }
  }
}
// 选中交换机后，确保该交换机属主的安全组（含默认）已加载。
// 后端 ListVPCSecurityGroups 仅在带 username 参数时才 EnsureDefaultSecurityGroup，
// 否则 admin 视角下若属主无预存 SG，下拉会为空（与 VM NetworkList ensureSecurityGroupsForSelectedSwitch 同理）。
const ensureSGsForSwitch = async () => {
  const sw = selectedSwitch.value
  if (!sw || sw.bridge_mode === 'bridge') return
  if (!isAdmin.value || !sw.username) return
  if (groups.value.some(g => g.username === sw.username)) return
  const res = await getVPCSecurityGroups({ username: sw.username })
  mergeGroups(res.data || [])
}
const switchLabel = (s) => {
  const prefix = isAdmin.value && s.username ? `${s.username} / ` : ''
  if (s.bridge_mode === 'bridge') return `${prefix}${s.name}（桥接直通：${s.bridge_name || '-'}）`
  return `${prefix}${s.name} (${s.cidr})`
}
const ensureSwitches = async () => {
  if (switches.value.length) return
  const [sw, sg] = await Promise.all([getVPCSwitches(), getVPCSecurityGroups()])
  switches.value = sw.data || []
  groups.value = sg.data || []
}
const openAdd = async () => { await ensureSwitches(); editOrder.value = -1; Object.assign(nicForm, { switch_id: null, security_group_id: null, bandwidth_inbound_avg: 0, bandwidth_outbound_avg: 0 }); nicVisible.value = true }
const openEdit = async (row) => { await ensureSwitches(); editOrder.value = row.order; Object.assign(nicForm, { switch_id: row.switch_id, security_group_id: row.security_group_id, bandwidth_inbound_avg: row.bandwidth_inbound_avg, bandwidth_outbound_avg: row.bandwidth_outbound_avg }); nicVisible.value = true }
const submitNic = async () => {
  submitting.value = true
  try {
    const payload = { switch_id: nicForm.switch_id, security_group_id: nicForm.security_group_id || 0, bandwidth_inbound_avg: nicForm.bandwidth_inbound_avg || 0, bandwidth_outbound_avg: nicForm.bandwidth_outbound_avg || 0 }
    if (editOrder.value >= 0) await updateLXCInterface(props.name, editOrder.value, payload)
    else await addLXCInterface(props.name, payload)
    ElMessage.success('已保存')
    nicVisible.value = false
    await load()
  } catch (e) {} finally { submitting.value = false }
}
const onRemove = async (row) => {
  submitting.value = true
  try {
    const force = row.is_primary ? { force: true } : {}
    await ElMessageBox.confirm(row.is_primary ? '删除主网卡将断网，确定？' : '确定删除该网卡？', '提示', { type: 'warning' }).catch(() => { throw new Error('cancel') })
    await removeLXCInterface(props.name, row.order, force)
    ElMessage.success('已删除')
    await load()
  } catch (e) {} finally { submitting.value = false }
}

// 静态 IP
const ipLoading = ref(false)
const staticBindings = ref([])
const dhcpLeases = ref([])
const currentBindings = computed(() => staticBindings.value.filter(b => b.vm_name === props.name))
const currentLeases = computed(() => {
  const ips = new Set(currentBindings.value.map(b => b.ip))
  const macs = new Set(currentBindings.value.map(b => b.mac))
  return dhcpLeases.value.filter(l => l.vm_name === props.name && !ips.has(l.ip) && !macs.has(l.mac))
})
const fetchIPs = async () => {
  ipLoading.value = true
  try {
    const res = await getStaticIPList()
    staticBindings.value = res.data?.static_bindings || []
    dhcpLeases.value = res.data?.dhcp_leases || []
  } catch (e) {} finally { ipLoading.value = false }
}
const bindVisible = ref(false)
const bindSubmitting = ref(false)
const bindForm = reactive({ vm_name: '', ip: '' })
const selectedLeaseIP = ref('')
const openBind = () => { bindForm.vm_name = props.name; bindForm.ip = ''; selectedLeaseIP.value = ''; bindVisible.value = true }
const onLease = (v) => { bindForm.ip = v }
const submitBind = async () => {
  bindSubmitting.value = true
  try {
    const res = await bindStaticIP(bindForm)
    ElMessage.success(res.message || '静态 IP 绑定成功')
    bindVisible.value = false
    await Promise.all([fetchIPs(), load()])
  } catch (e) {} finally { bindSubmitting.value = false }
}
const onUnbind = async (row) => {
  try {
    await ElMessageBox.confirm(`确定解绑 ${row.ip}？`, '提示', { type: 'warning' })
    await unbindStaticIP({ vm_name: props.name })
    ElMessage.success('已解绑')
    await Promise.all([fetchIPs(), load()])
  } catch (e) {}
}

onMounted(() => { load(); fetchIPs() })
watch(() => props.name, () => { load(); fetchIPs() })
// 选交换机时补拉该属主的安全组（含默认），避免 admin 视角下拉为空
watch(() => nicForm.switch_id, () => { ensureSGsForSwitch() })
</script>

<style scoped>
.lxc-network-panel { display: flex; flex-direction: column; gap: 16px; }
.cfg-card { border-radius: 12px; border: none; }
.cfg-card :deep(.el-card__body) { padding: 16px 18px; }
.section-title { font-size: 16px; font-weight: 700; padding-left: 10px; border-left: 4px solid var(--el-color-primary); margin-bottom: 14px; }
.cfg-hint { font-size: 12px; color: var(--el-text-color-secondary); }
.vpc-static-ip-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.header-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
h4 { margin: 0 0 8px 0; font-size: 14px; }
</style>
