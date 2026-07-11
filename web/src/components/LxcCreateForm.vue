<template>
  <el-dialog v-model="visible" title="创建 LXC 容器" width="1100px" align-center append-to-body :close-on-click-modal="false" @closed="onClosed">
    <div class="lxc-create-wrap">
      <!-- 步骤圆点（仿 VmForm） -->
      <div class="step-indicator-bar">
        <div v-for="(s, i) in steps" :key="s.name"
          :class="['step-dot-item', { active: step === i, done: step > i }]"
          @click="step = i">
          <div class="step-dot-badge"><span v-if="step > i">✓</span><span v-else>{{ i + 1 }}</span></div>
          <div class="step-dot-label">{{ s.title }}</div>
        </div>
      </div>

      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px" class="lxc-create-form">
        <!-- 步骤 0：来源 -->
        <div v-show="step === 0" class="step-pane">
          <div class="step-pane-header"><div class="step-pane-title">来源</div><div class="step-pane-desc">选择克隆模板或官方镜像下载</div></div>
          <el-form-item label="来源">
            <el-radio-group v-model="form.source">
              <el-radio value="clone">克隆模板</el-radio>
              <el-radio value="download">官方镜像下载</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item v-if="form.source === 'clone'" label="模板" prop="template">
            <el-select v-model="form.template" filterable placeholder="选择模板" style="width:100%">
              <el-option v-for="t in templates" :key="t.name" :label="t.display_name || t.name" :value="t.name" :disabled="t.disabled" />
            </el-select>
          </el-form-item>
          <template v-else>
            <el-form-item label="发行版" prop="distro">
              <el-select v-model="form.distro" filterable :loading="downloadLoading" placeholder="选择发行版" style="width:100%">
                <el-option v-for="d in dlDistros" :key="d" :label="d" :value="d" />
              </el-select>
            </el-form-item>
            <el-form-item label="版本" prop="release">
              <el-select v-model="form.release" filterable :disabled="!form.distro" placeholder="选择版本" style="width:100%">
                <el-option v-for="r in dlReleases" :key="r" :label="r" :value="r" />
              </el-select>
            </el-form-item>
            <el-form-item label="架构" prop="arch">
              <el-select v-model="form.arch" :disabled="!form.release" placeholder="选择架构" style="width:100%">
                <el-option v-for="a in dlArches" :key="a" :label="a" :value="a" />
              </el-select>
            </el-form-item>
          </template>
        </div>

        <!-- 步骤 1：基本配置 -->
        <div v-show="step === 1" class="step-pane">
          <div class="step-pane-header"><div class="step-pane-title">基本配置</div><div class="step-pane-desc">名称与资源</div></div>
          <el-form-item label="名称" prop="name">
            <el-input v-model="form.name">
              <template #append><el-button @click="handleGenerateName">随机生成</el-button></template>
            </el-input>
            <div class="form-tip">小写字母/数字/-，2-63 字符，开头字母/数字。</div>
          </el-form-item>
          <el-form-item label="创建数量">
            <el-input-number v-model="form.batch_count" :min="1" :max="100" style="width: 100%;" />
            <div class="form-tip" v-if="form.batch_count > 1">
              <el-icon><InfoFilled /></el-icon>
              批量模式下名称作为前缀，将创建 {{ form.batch_count }} 个容器：
              {{ batchPreviewNames.slice(0, 3).join('、') }}{{ batchPreviewNames.length > 3 ? ' …' : '' }}
              （共 {{ form.batch_count * form.memory_mb }}MB 内存<span v-if="form.disk_limit_gb > 0">，每容器上限 {{ form.disk_limit_gb }}GB 磁盘</span>）
            </div>
          </el-form-item>
          <el-form-item label="容器目录">
            <el-input :model-value="containerPathPreview" disabled />
            <div class="form-tip">容器将创建于此；目录随名称变化，由系统设置「LXC 容器目录」决定。</div>
          </el-form-item>
          <el-form-item label="CPU 权重">
            <el-input-number v-model="form.cpu_shares" :min="1" :max="10000" />
            <div class="form-tip"><el-icon><InfoFilled /></el-icon> 相对优先级（cgroup2 cpu.weight，范围 1–10000，系统默认 100），不限制核数；值越大，CPU 紧张时越优先分配。</div>
          </el-form-item>
          <el-form-item label="内存(MB)"><el-input-number v-model="form.memory_mb" :min="0" :step="512" /></el-form-item>
          <el-form-item v-if="diskLimitVisible" label="磁盘上限(GB)">
            <el-input-number v-model="form.disk_limit_gb" :min="0" :step="10" />
            <div class="form-tip"><el-icon><InfoFilled /></el-icon> 仅 zfs 后端生效，限制容器 rootfs 数据量；0/空=不限。</div>
          </el-form-item>
          <el-form-item label="自动启动"><el-switch v-model="form.autostart" /></el-form-item>
          <el-form-item label="分组"><el-input v-model="form.group_name" /></el-form-item>
          <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
        </div>

        <!-- 步骤 2：网络 -->
        <div v-show="step === 2" class="step-pane">
          <div class="step-pane-header"><div class="step-pane-title">网络</div><div class="step-pane-desc">配置网卡（首个有效网口为主网卡，其余为附加网卡）。不添加则容器为裸网。</div></div>
          <div class="form-section-card">
            <div class="form-section-card-header">
              <span>网口列表</span>
              <el-button size="small" type="primary" plain style="margin-left:auto" @click="addNic">添加网口</el-button>
            </div>
            <div v-if="extraNics.length" class="form-section-card-body">
              <div v-for="(nic, i) in extraNics" :key="i" class="extra-nic-row">
                <div class="extra-nic-header">
                  <el-tag size="small" type="info">网口 #{{ i + 1 }}{{ i === primaryNicIndex ? '（主）' : '' }}</el-tag>
                  <el-button size="small" type="danger" text @click="extraNics.splice(i, 1)">删除</el-button>
                </div>
                <el-row :gutter="16">
                  <el-col :span="12">
                    <el-form-item label="VPC 交换机" label-width="100px">
                      <el-select v-model="nic.switch_id" filterable placeholder="选择交换机" style="width:100%"
                        @change="onNicSwitchChange(nic)" @focus="loadVPCOptions">
                        <el-option v-for="s in vpcSwitches" :key="s.id" :label="switchLabel(s)" :value="s.id" />
                      </el-select>
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="安全组" label-width="100px">
                      <el-select v-model="nic.security_group_id" clearable filterable placeholder="可选" style="width:100%">
                        <el-option v-for="g in sgsForSwitch(nic.switch_id)" :key="g.id" :label="g.is_default ? `${g.name}（默认）` : g.name" :value="g.id" />
                      </el-select>
                    </el-form-item>
                  </el-col>
                </el-row>
              </div>
            </div>
            <div v-else class="form-section-card-body">
              <el-empty description="未添加网口，容器将无 VPC 网络接入（保持裸网）。" :image-size="48" />
            </div>
          </div>
        </div>

        <!-- 步骤 3：确认 -->
        <div v-show="step === 3" class="step-pane">
          <div class="step-pane-header"><div class="step-pane-title">确认</div><div class="step-pane-desc">请核对后创建</div></div>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="来源">{{ form.source === 'clone' ? '克隆模板' : '官方镜像下载' }}</el-descriptions-item>
            <el-descriptions-item label="模板/镜像">{{ form.source === 'clone' ? (form.template || '-') : `${form.distro}/${form.release}/${form.arch}` }}</el-descriptions-item>
            <el-descriptions-item label="名称">{{ form.name || '-' }}</el-descriptions-item>
            <el-descriptions-item label="CPU/内存">{{ form.cpu_shares }} / {{ form.memory_mb }}MB</el-descriptions-item>
            <el-descriptions-item label="自动启动">{{ form.autostart ? '是' : '否' }}</el-descriptions-item>
            <el-descriptions-item label="分组/备注">{{ form.group_name || '-' }} / {{ form.remark || '-' }}</el-descriptions-item>
            <el-descriptions-item label="网口" :span="2">{{ nicsSummary }}</el-descriptions-item>
            <el-descriptions-item v-if="form.batch_count > 1" label="将创建的容器" :span="2">
              <div class="form-tip">
                <el-tag v-for="nm in batchPreviewNames.slice(0, 12)" :key="nm" type="info" style="margin: 2px;">{{ nm }}</el-tag>
                <span v-if="batchPreviewNames.length > 12"> …共 {{ batchPreviewNames.length }} 个</span>
              </div>
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </el-form>
    </div>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button v-if="step > 0" @click="prevStep">上一步</el-button>
      <el-button v-if="step < steps.length - 1" type="primary" @click="nextStep">下一步</el-button>
      <el-button type="warning" :loading="loading" :disabled="!allRequiredFilled" @click="submit">立即创建</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { InfoFilled } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { createLXC, batchCreateLXC, getLXCTemplateList, getLXCDownloadList, getLXCBackingInfo } from '@/api/lxc'
import { getVPCSwitches, getVPCSecurityGroups } from '@/api/vpc'
import { getSettings } from '@/api/settings'
import { generateRandomLXCName } from '@/utils/lxc'

const emit = defineEmits(['success'])
const userStore = useUserStore()
const isAdmin = computed(() => userStore.role === 'admin')

const visible = ref(false)
const step = ref(0)
const steps = [
  { title: '来源', name: 'source' },
  { title: '基本配置', name: 'basic' },
  { title: '网络', name: 'network' },
  { title: '确认', name: 'confirm' }
]
const formRef = ref(null)
const loading = ref(false)

const defaultForm = () => ({ name: '', batch_count: 1, template: '', source: 'clone', distro: '', release: '', arch: '', cpu_shares: 256, memory_mb: 2048, disk_limit_gb: 0, autostart: false, group_name: '', remark: '' })
const form = reactive(defaultForm())
const rules = {
  name: [
    { required: true, message: '请填写名称', trigger: 'blur' },
    { pattern: /^[a-z0-9][a-z0-9-]{1,62}$/, message: '小写字母/数字/-，2-63 字符，开头字母/数字', trigger: 'blur' }
  ],
  template: [{ required: true, message: '请选择模板', trigger: 'change' }],
  distro: [{ required: true, message: '请选择发行版', trigger: 'change' }],
  release: [{ required: true, message: '请选择版本', trigger: 'change' }],
  arch: [{ required: true, message: '请选择架构', trigger: 'change' }]
}

// 来源数据
const templates = ref([])
const lxcLxcPath = ref('')
const downloadList = ref([])
const downloadLoading = ref(false)
const containerPathPreview = computed(() => {
  const base = (lxcLxcPath.value || '/var/lib/lxc').replace(/\/+$/, '')
  return `${base}/${form.name || '<名称>'}/`
})
const dlDistros = computed(() => [...new Set(downloadList.value.map(e => e.distro))].sort())
const dlReleases = computed(() => [...new Set(downloadList.value.filter(e => e.distro === form.distro).map(e => e.release))].sort())
const dlArches = computed(() => [...new Set(downloadList.value.filter(e => e.distro === form.distro && e.release === form.release).map(e => e.arch))].sort())
const fetchDownloadList = async () => {
  if (downloadList.value.length || downloadLoading.value) return
  downloadLoading.value = true
  try { const r = await getLXCDownloadList(); downloadList.value = r.data || [] }
  catch (e) { ElMessage.error('获取镜像清单失败（需宿主机外网）') }
  finally { downloadLoading.value = false }
}
// backing 判定：clone 看选中模板 backing；download 看 backing-info is_zfs
const downloadIsZfs = ref(false)
const fetchBackingInfo = async () => {
  try { const r = await getLXCBackingInfo(); downloadIsZfs.value = !!r.data?.is_zfs } catch {}
}
const diskLimitVisible = computed(() => {
  if (form.source === 'clone') {
    const t = templates.value.find(x => x.name === form.template)
    return !!t && t.backing === 'zfs'
  }
  return downloadIsZfs.value
})
watch(() => form.distro, () => { form.release = ''; form.arch = '' })
watch(() => form.release, () => { form.arch = dlArches.value.includes('amd64') ? 'amd64' : (dlArches.value[0] || '') })
watch(() => form.source, (s) => { if (s === 'download') { fetchDownloadList(); fetchBackingInfo() } })

// 网口
const extraNics = ref([]) // [{switch_id, security_group_id}]
// 主网卡 = 首个有效（含 switch_id）网口；与 buildAllNicsPayload 提升逻辑保持一致，避免标签与实际主网卡错位
const primaryNicIndex = computed(() => extraNics.value.findIndex(n => n.switch_id))
const vpcSwitches = ref([])
const vpcSecurityGroups = ref([])
const loadVPCOptions = async () => {
  if (vpcSwitches.value.length) return
  const [sw, sg] = await Promise.all([getVPCSwitches(), getVPCSecurityGroups()])
  vpcSwitches.value = sw.data || []
  vpcSecurityGroups.value = sg.data || []
}
const switchLabel = (s) => {
  const prefix = isAdmin.value && s.username ? `${s.username} / ` : ''
  if (s.bridge_mode === 'bridge') return `${prefix}${s.name}（桥接直通：${s.bridge_name || '-'}）`
  return `${prefix}${s.name} (${s.cidr})`
}
const switchOf = (id) => vpcSwitches.value.find(s => s.id === id) || null
// 系统交换机（username 空）→ 全部；私属交换机 → 按属主过滤（与 LxcNetworkPanel 一致）
const sgsForSwitch = (switchId) => {
  const sw = switchOf(switchId)
  if (!sw || !sw.username) return vpcSecurityGroups.value
  return vpcSecurityGroups.value.filter(g => !g.username || g.username === sw.username)
}
const mergeGroups = (incoming) => {
  const exists = new Set(vpcSecurityGroups.value.map(g => g.id))
  for (const g of incoming || []) {
    if (!exists.has(g.id)) { vpcSecurityGroups.value.push(g); exists.add(g.id) }
  }
}
// admin + 私属交换机 + 属主无预存 SG → 按 {username} 补拉（触发 EnsureDefaultSecurityGroup）
const ensureSGsForSwitch = async (switchId) => {
  const sw = switchOf(switchId)
  if (!sw || sw.bridge_mode === 'bridge' || !sw.username || !isAdmin.value) return
  if (vpcSecurityGroups.value.some(g => g.username === sw.username)) return
  const res = await getVPCSecurityGroups({ username: sw.username })
  mergeGroups(res.data || [])
}
const onNicSwitchChange = async (nic) => {
  await ensureSGsForSwitch(nic.switch_id)
  // 切交换机后，若已选 SG 不属于新交换机属主则清空
  const sw = switchOf(nic.switch_id)
  const cur = vpcSecurityGroups.value.find(g => g.id === nic.security_group_id)
  if (cur && sw && sw.username && cur.username !== sw.username) nic.security_group_id = null
}
const addNic = async () => { await loadVPCOptions(); extraNics.value.push({ switch_id: vpcSwitches.value[0]?.id ?? null, security_group_id: null }) }
const buildAllNicsPayload = () => {
  const valid = extraNics.value.filter(n => n.switch_id)
  if (!valid.length) return { primarySwitchId: 0, primarySecurityGroupId: 0, extraNics: [] }
  const first = valid[0]
  return {
    primarySwitchId: first.switch_id,
    primarySecurityGroupId: first.security_group_id || 0,
    extraNics: valid.slice(1).map(n => ({ switch_id: n.switch_id, security_group_id: n.security_group_id || 0 }))
  }
}
const nicsSummary = computed(() => {
  const valid = extraNics.value.filter(n => n.switch_id)
  if (!valid.length) return '无（裸网）'
  return valid.map((n, i) => `#${i + 1} ${switchOf(n.switch_id)?.name || '?'}${n.security_group_id ? '(SG)' : ''}`).join('，')
})
// 批量预览名（与后端 BatchName 格式一致：prefix-NN，2 位补零，>99 自动升 3 位）
const batchPreviewNames = computed(() => {
  const n = form.batch_count || 1
  if (n <= 1) return []
  const prefix = form.name || 'xxx'
  const names = []
  for (let i = 1; i <= n; i++) names.push(`${prefix}-${String(i).padStart(2, '0')}`)
  return names
})

// 导航
const handleGenerateName = async () => {
  form.name = generateRandomLXCName()
  await formRef.value?.validateField('name').catch(() => false)
}
const prevStep = () => { if (step.value > 0) step.value-- }
const nextStep = async () => {
  if (!formRef.value) return
  const validateFields = step.value === 0
    ? (form.source === 'clone' ? ['template'] : ['distro', 'release', 'arch'])
    : step.value === 1 ? ['name'] : []
  let valid = true
  if (validateFields.length) {
    try { await formRef.value.validateField(validateFields) } catch (e) { valid = false }
  }
  if (valid && step.value < steps.length - 1) step.value++
}
const allRequiredFilled = computed(() => {
  if (!/^[a-z0-9][a-z0-9-]{1,62}$/.test(form.name)) return false
  if (form.source === 'clone') return !!form.template
  return !!(form.distro && form.release && form.arch)
})

const open = async () => {
  Object.assign(form, defaultForm())
  form.name = generateRandomLXCName() // 打开时先随机生成一个名称，用户可改
  extraNics.value = []
  step.value = 0
  try { const r = await getLXCTemplateList(); templates.value = r.data || [] } catch (e) {}
  if (!lxcLxcPath.value) { try { const s = await getSettings(); lxcLxcPath.value = s.data?.lxc_lxc_path || '' } catch (e) {} }
  visible.value = true
}
defineExpose({ open })

const submit = async () => {
  if (!allRequiredFilled.value) { ElMessage.warning('请补全必填项'); return }
  await formRef.value.validate(async (valid) => {
    if (!valid) { ElMessage.warning('表单有误'); return }
    const nics = buildAllNicsPayload()
    loading.value = true
    try {
      if (form.batch_count > 1) {
        await batchCreateLXC({
          prefix: form.name,
          start_num: 1,
          count: form.batch_count,
          source: form.source,
          template: form.template, distro: form.distro, release: form.release, arch: form.arch,
          cpu_shares: form.cpu_shares, memory_mb: form.memory_mb, disk_limit_gb: form.disk_limit_gb,
          autostart: form.autostart, group_name: form.group_name, remark: form.remark,
          switch_id: nics.primarySwitchId, security_group_id: nics.primarySecurityGroupId,
          extra_nics: nics.extraNics
        })
        ElMessage.success(`批量创建任务已提交（${form.batch_count} 个），请在任务中查看进度`)
      } else {
        await createLXC({
          name: form.name, source: form.source,
          template: form.template, distro: form.distro, release: form.release, arch: form.arch,
          cpu_shares: form.cpu_shares, memory_mb: form.memory_mb, disk_limit_gb: form.disk_limit_gb,
          autostart: form.autostart, group_name: form.group_name, remark: form.remark,
          switch_id: nics.primarySwitchId, security_group_id: nics.primarySecurityGroupId,
          extra_nics: nics.extraNics
        })
        ElMessage.success('创建任务已提交')
      }
      visible.value = false
      emit('success')
    } catch (e) { /* request 拦截器已弹 toast */ } finally { loading.value = false }
  })
}
const onClosed = () => { step.value = 0 }
</script>

<style scoped>
.lxc-create-wrap { display: flex; flex-direction: column; gap: 14px; }
.step-indicator-bar { display: flex; gap: 6px; background: var(--el-fill-color-light, #f5f6f8); border-radius: 10px; padding: 6px; }
.step-dot-item { display: flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 8px; cursor: pointer; font-size: 13px; color: var(--el-text-color-secondary); flex: 1; }
.step-dot-item.active { background: var(--el-color-primary-light-9, #ecf5ff); color: var(--el-color-primary); }
.step-dot-item.done { color: var(--el-color-success); }
.step-dot-badge { width: 22px; height: 22px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: 12px; background: var(--el-fill-color, #e4e7ed); color: var(--el-text-color-secondary); }
.step-dot-item.active .step-dot-badge { background: var(--el-color-primary); color: #fff; }
.step-dot-item.done .step-dot-badge { background: var(--el-color-success); color: #fff; }
.step-dot-label { white-space: nowrap; }
.step-pane { padding: 4px 2px; }
.step-pane-header { margin-bottom: 12px; }
.step-pane-title { font-size: 16px; font-weight: 700; }
.step-pane-desc { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
.form-section-card { border: 1px solid var(--el-border-color-lighter); border-radius: 8px; padding: 12px; }
.form-section-card-header { display: flex; align-items: center; margin-bottom: 10px; font-weight: 600; }
.form-section-card-body { display: flex; flex-direction: column; gap: 12px; }
.extra-nic-row { border: 1px solid var(--el-border-color-lighter); border-radius: 6px; padding: 10px; }
.extra-nic-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.form-tip { font-size: 12px; color: var(--el-text-color-secondary); display: flex; align-items: center; gap: 4px; }
.nic-hint { font-size: 12px; color: var(--el-text-color-secondary); padding: 0 0 0 100px; margin-top: -4px; }
</style>
