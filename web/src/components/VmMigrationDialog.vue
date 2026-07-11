<template>
  <el-dialog v-model="visible" :title="dialogTitle" width="900px" destroy-on-close append-to-body>
    <template v-if="vm">
      <el-form label-width="130px">
        <el-form-item label="虚拟机">
          <el-input :model-value="vm.name" disabled />
        </el-form-item>
        <el-form-item label="迁移硬件">
          <el-radio-group v-model="migrationKind" @change="handleMigrationKindChange">
            <el-radio-button label="vm">迁移虚拟机</el-radio-button>
            <el-radio-button label="disk">迁移硬盘</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <template v-if="migrationKind === 'vm'">
        <el-form-item label="迁移方式">
          <el-tag :type="migrationMode === 'live' ? 'warning' : 'info'">
            {{ migrationMode === 'live' ? '热迁移' : '冷迁移' }}
          </el-tag>
          <span class="mode-hint">当前状态：{{ sourceStateLabel }}</span>
        </el-form-item>
        <el-form-item label="目标节点">
          <el-select v-model="form.node_id" placeholder="请选择目标节点" filterable @change="loadOptions">
            <el-option
              v-for="node in nodes"
              :key="node.id"
              :label="`${node.name}（${node.ssh_host}）`"
              :value="node.id"
              :disabled="!node.enabled"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="目标存储">
          <el-select
            v-model="form.target_storage_pool_id"
            placeholder="请选择目标存储"
            filterable
            :loading="optionsLoading"
            :disabled="!optionsData"
            @change="handleDefaultStorageChange"
          >
            <el-option
              v-for="item in targetStorageTargets"
              :key="item.id"
              :label="storageLabel(item)"
              :value="item.id"
              :disabled="!item.enabled"
            />
          </el-select>
          <div class="form-tip">作为默认目标存储；多块硬盘可在下方为每块硬盘单独选择对端存储位置。</div>
        </el-form-item>
        <el-form-item v-if="sourceMigrationDisks.length > 1" label="硬盘目标存储">
          <el-table :data="sourceMigrationDisks" border size="small" v-loading="sourceDisksLoading" style="width: 100%;">
            <el-table-column prop="device" label="设备" width="80" />
            <el-table-column label="容量" width="100">
              <template #default="{ row }">{{ row.capacity_gb ? `${row.capacity_gb} GB` : '-' }}</template>
            </el-table-column>
            <el-table-column prop="path" label="源路径" min-width="240" show-overflow-tooltip />
            <el-table-column label="对端存储" min-width="220">
              <template #default="{ row }">
                <el-select
                  v-model="diskStorageForm[row.device]"
                  placeholder="请选择目标存储"
                  filterable
                  style="width: 100%;"
                  @change="markDirty"
                >
                  <el-option
                    v-for="item in targetStorageTargets"
                    :key="item.id"
                    :label="storageLabel(item)"
                    :value="item.id"
                    :disabled="!item.enabled"
                  />
                </el-select>
              </template>
            </el-table-column>
          </el-table>
          <div class="form-tip">迁移执行前会按每块硬盘选择的存储生成目标路径，并分别检查空间和冲突。</div>
        </el-form-item>
        <el-form-item v-if="showTargetNetwork && optionsData?.is_lightweight" label="轻量云 VPC">
          <el-select v-model="form.target_switch_id" placeholder="请选择目标轻量云 VPC" filterable @change="markDirty">
            <el-option
              v-for="item in lightweightSwitches"
              :key="item.id"
              :label="`${item.username} / ${item.name} / ${item.cidr}`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <template v-else-if="showTargetNetwork">
          <el-form-item label="目标 VPC">
            <el-select v-model="form.target_switch_id" placeholder="请选择目标 VPC" filterable @change="markDirty">
              <el-option
                v-for="item in optionsData.target_switches || []"
                :key="item.id"
                :label="`${item.username} / ${item.name} / ${item.cidr}`"
                :value="item.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="目标安全组">
            <el-select v-model="form.target_security_group_id" placeholder="默认安全组" filterable clearable @change="markDirty">
              <el-option
                v-for="item in optionsData.target_security_groups || []"
                :key="item.id"
                :label="`${item.username} / ${item.name}`"
                :value="item.id"
              />
            </el-select>
          </el-form-item>
        </template>
        <el-form-item label="预检策略">
          <el-checkbox v-model="form.skip_precheck" @change="markDirty">
            跳过完整预检
          </el-checkbox>
          <div class="form-tip">不勾选时可直接提交，任务开始后自动执行预检；勾选后跳过耗时 backing hash 对比。</div>
        </el-form-item>
        <el-form-item v-if="migrationMode === 'live'" label="迁移 CPU 限制">
          <el-checkbox v-model="form.enable_cpu_throttle" @change="markDirty">
            迁移时限制 CPU 使用率
          </el-checkbox>
          <el-input-number
            v-model="form.cpu_throttle_percent"
            :min="10"
            :max="100"
            :step="5"
            controls-position="right"
            style="width: 140px; margin-left: 12px;"
            @change="markDirty"
          />
          <span class="mode-hint">%</span>
          <div class="form-tip">脏页速率达到平均带宽 20% - 50% 时，后端会强制启用该限制；达到 50% 会阻止热迁移。</div>
        </el-form-item>
        </template>

        <template v-else>
          <el-form-item label="迁移方式">
            <el-tag :type="diskMigrationMode === 'live' ? 'warning' : 'info'">
              {{ diskMigrationMode === 'live' ? '热迁移' : '冷迁移' }}
            </el-tag>
            <span class="mode-hint">当前状态：{{ diskSourceStateLabel }}</span>
          </el-form-item>
          <el-form-item label="目标存储">
            <el-select
              v-model="diskForm.target_storage_pool_id"
              placeholder="请选择目标存储"
              filterable
              :loading="diskOptionsLoading"
              :disabled="!diskOptionsData"
              style="width: 100%;"
            >
              <el-option
                v-for="item in diskTargetStorageTargets"
                :key="item.id"
                :label="storageLabel(item)"
                :value="item.id"
                :disabled="!item.enabled"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="目标硬盘">
            <el-table
              :data="diskMigrationDisks"
              border
              size="small"
              v-loading="diskOptionsLoading"
              empty-text="暂无可迁移硬盘"
              style="width: 100%;"
              @row-click="selectDiskRow"
            >
              <el-table-column label="" width="48" align="center">
                <template #default="{ row }">
                  <el-radio v-model="diskForm.device" :label="row.device" :disabled="!row.can_migrate">
                    <span />
                  </el-radio>
                </template>
              </el-table-column>
              <el-table-column prop="device" label="设备" width="80" />
              <el-table-column label="容量" width="100">
                <template #default="{ row }">{{ row.capacity_gb ? `${row.capacity_gb} GB` : '-' }}</template>
              </el-table-column>
              <el-table-column prop="format" label="格式" width="90" />
              <el-table-column prop="bus" label="驱动" width="90" />
              <el-table-column prop="path" label="当前路径" min-width="240" show-overflow-tooltip />
              <el-table-column prop="backing_path" label="backing" min-width="200" show-overflow-tooltip />
              <el-table-column label="状态" width="120">
                <template #default="{ row }">
                  <el-tag :type="row.can_migrate ? 'success' : 'danger'" size="small">
                    {{ row.can_migrate ? '可迁移' : '不可迁移' }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>
            <div class="form-tip">运行中虚拟机会执行热迁移并在完成后切换硬盘路径；关机虚拟机会执行冷迁移并更新持久化 XML。</div>
          </el-form-item>
        </template>
      </el-form>

      <template v-if="migrationKind === 'vm'">
      <el-alert
        v-if="optionsData"
        type="info"
        :title="optionsSummary"
        :closable="false"
        style="margin: 12px 0;"
      />

      <el-button :disabled="!canPreview" :loading="previewLoading" @click="preview">
        执行预检
      </el-button>

      <el-alert
        v-if="previewData"
        :type="previewData.allowed ? 'success' : 'error'"
        :title="previewData.allowed ? '预检通过，可以提交迁移任务' : '预检未通过'"
        :closable="false"
        style="margin: 16px 0;"
      />
      <el-alert
        v-else-if="canSubmit"
        type="info"
        title="未执行预检也可以直接提交，迁移任务会在开始后生成执行计划"
        :closable="false"
        style="margin: 16px 0;"
      />
      <el-alert
        v-if="form.skip_precheck"
        type="warning"
        title="已选择跳过完整预检：任务不会提前计算 backing hash，失败原因会在任务详情中展示"
        :closable="false"
        style="margin-top: 10px;"
      />

      <el-descriptions v-if="previewData" :column="2" border size="small" class="summary">
        <el-descriptions-item label="源状态">{{ previewData.source_state || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用户">{{ previewData.owner }} / {{ previewData.cloud_type }}</el-descriptions-item>
        <el-descriptions-item label="目标用户">
          {{ previewData.will_create_target_user ? '自动注册' : '绑定已有用户' }}
        </el-descriptions-item>
        <el-descriptions-item label="目标存储">{{ previewData.target_storage_dir || '-' }}</el-descriptions-item>
        <el-descriptions-item label="所需容量">{{ formatBytes(previewData.required_storage_bytes) }}</el-descriptions-item>
        <el-descriptions-item label="VM 凭据">{{ previewData.credential ? '同步' : '无凭据记录' }}</el-descriptions-item>
      </el-descriptions>

      <el-descriptions v-if="previewData?.live_assessment" :column="2" border size="small" class="summary">
        <el-descriptions-item label="平均带宽">{{ formatMiB(previewData.live_assessment.average_bandwidth_mib) }}/s</el-descriptions-item>
        <el-descriptions-item label="脏页速率">{{ formatMiB(previewData.live_assessment.dirty_rate_mib) }}/s</el-descriptions-item>
        <el-descriptions-item label="脏页占比">{{ formatPercent(previewData.live_assessment.dirty_rate_ratio_percent) }}</el-descriptions-item>
        <el-descriptions-item label="CPU 限制">
          {{ previewData.live_assessment.cpu_throttle_enabled ? `${previewData.live_assessment.cpu_throttle_percent}%` : '不限制' }}
        </el-descriptions-item>
        <el-descriptions-item label="kvm_page_fault">
          {{ previewData.live_assessment.kvm_stat_available ? `${previewData.live_assessment.kvm_page_fault_rate || 0}` : '不可用' }}
        </el-descriptions-item>
        <el-descriptions-item label="评估结论">
          <el-tag :type="previewData.live_assessment.allowed ? 'success' : 'danger'">
            {{ previewData.live_assessment.allowed ? '允许热迁移' : '阻止热迁移' }}
          </el-tag>
        </el-descriptions-item>
      </el-descriptions>

        <el-table v-if="previewData?.disks?.length" :data="previewData.disks" border size="small" style="margin-top: 12px;">
        <el-table-column prop="target" label="磁盘" width="90" />
        <el-table-column label="目标存储" width="160">
          <template #default="{ row }">{{ storageName(row.target_storage_pool_id) }}</template>
        </el-table-column>
        <el-table-column prop="source_path" label="源 overlay" min-width="240" show-overflow-tooltip />
        <el-table-column prop="target_path" label="目标 overlay" min-width="240" show-overflow-tooltip />
        <el-table-column prop="backing_path" label="backing" min-width="220" show-overflow-tooltip />
      </el-table>

      <el-table v-if="previewData?.backing_checks?.length" :data="previewData.backing_checks" border size="small" style="margin-top: 12px;">
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.ok ? 'success' : 'danger'">{{ row.ok ? '通过' : '失败' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="backing 路径" min-width="260" show-overflow-tooltip />
        <el-table-column prop="message" label="说明" min-width="180" show-overflow-tooltip />
      </el-table>

      <el-table v-if="previewData?.port_forwards?.length" :data="previewData.port_forwards" border size="small" style="margin-top: 12px;">
        <el-table-column prop="protocol" label="协议" width="90" />
        <el-table-column prop="source_host_port" label="源端口" width="100" />
        <el-table-column label="目标端口" width="120">
          <template #default="{ row }">
            {{ row.target_host_port || '自动分配' }}
          </template>
        </el-table-column>
        <el-table-column prop="vm_port" label="VM 端口" width="100" />
        <el-table-column prop="dest_ip" label="源目标 IP" min-width="140" />
      </el-table>

      <el-alert
        v-for="item in previewData?.warnings || []"
        :key="item"
        type="warning"
        :title="item"
        :closable="false"
        style="margin-top: 10px;"
      />
      <el-alert
        v-for="item in previewData?.blockers || []"
        :key="item"
        type="error"
        :title="item"
        :closable="false"
        style="margin-top: 10px;"
      />
      </template>

      <template v-else>
        <el-alert
          v-if="diskOptionsSummary"
          type="info"
          :title="diskOptionsSummary"
          :closable="false"
          style="margin: 12px 0;"
        />
        <el-alert
          v-for="item in diskOptionsData?.warnings || []"
          :key="item"
          type="warning"
          :title="item"
          :closable="false"
          style="margin-top: 10px;"
        />
        <el-alert
          v-if="selectedDisk?.block_reason"
          type="error"
          :title="selectedDisk.block_reason"
          :closable="false"
          style="margin-top: 10px;"
        />
      </template>
    </template>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :disabled="!activeCanSubmit" :loading="activeSubmitting" @click="submit">
        {{ migrationKind === 'disk' ? '提交硬盘迁移' : '提交迁移' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getNodeMigrationOptions, listNodes } from '@/api/node'
import { getDiskList, getDiskMigrationOptions, migrateDisk, migrateVm, previewVmMigration } from '@/api/vm'

const emit = defineEmits(['success'])
const visible = ref(false)
const nodes = ref([])
const vm = ref(null)
const migrationKind = ref('vm')
const optionsData = ref(null)
const previewData = ref(null)
const diskOptionsData = ref(null)
const sourceMigrationDisks = ref([])
const optionsLoading = ref(false)
const previewLoading = ref(false)
const diskOptionsLoading = ref(false)
const sourceDisksLoading = ref(false)
const submitting = ref(false)
const diskSubmitting = ref(false)
const form = reactive({
  node_id: null,
  target_storage_pool_id: '',
  target_switch_id: null,
  target_security_group_id: null,
  skip_precheck: false,
  enable_cpu_throttle: false,
  cpu_throttle_percent: 50
})
const diskForm = reactive({
  device: '',
  target_storage_pool_id: ''
})
const diskStorageForm = reactive({})

const dialogTitle = computed(() => {
  return migrationKind.value === 'disk' ? '迁移硬盘' : '迁移虚拟机'
})

const migrationMode = computed(() => {
  return optionsData.value?.mode || (vm.value?.status === 'running' ? 'live' : 'cold')
})

const sourceStateLabel = computed(() => {
  return optionsData.value?.source_state || vm.value?.status || '-'
})

const targetStorageTargets = computed(() => optionsData.value?.target_storage_targets || [])
const diskTargetStorageTargets = computed(() => diskOptionsData.value?.target_storage_targets || [])
const diskMigrationDisks = computed(() => diskOptionsData.value?.disks || [])
const selectedDisk = computed(() => diskMigrationDisks.value.find(item => item.device === diskForm.device))

const diskMigrationMode = computed(() => {
  return diskOptionsData.value?.mode || (vm.value?.status === 'running' ? 'live' : 'cold')
})

const diskSourceStateLabel = computed(() => {
  return diskOptionsData.value?.source_state || vm.value?.status || '-'
})

const lightweightSwitches = computed(() => {
  return (optionsData.value?.target_switches || []).filter(item => {
    return !item.bridge_mode || item.bridge_mode === 'nat'
  })
})

const showTargetNetwork = computed(() => {
  return !!optionsData.value?.target_user_exists
})

const canPreview = computed(() => {
  if (!vm.value || !form.node_id || !form.target_storage_pool_id || optionsLoading.value || sourceDisksLoading.value) {
    return false
  }
  if (sourceMigrationDisks.value.length > 1 && sourceMigrationDisks.value.some(disk => !diskStorageForm[disk.device])) {
    return false
  }
  if (showTargetNetwork.value) {
    return !!form.target_switch_id
  }
  return true
})

const canSubmit = computed(() => {
  if (!canPreview.value) return false
  if (previewData.value && !previewData.value.allowed && !form.skip_precheck) {
    return false
  }
  return true
})

const canDiskSubmit = computed(() => {
  if (!vm.value || diskOptionsLoading.value || !diskOptionsData.value) {
    return false
  }
  if (!diskForm.device || !diskForm.target_storage_pool_id) {
    return false
  }
  return !!selectedDisk.value?.can_migrate
})

const activeCanSubmit = computed(() => {
  return migrationKind.value === 'disk' ? canDiskSubmit.value : canSubmit.value
})

const activeSubmitting = computed(() => {
  return migrationKind.value === 'disk' ? diskSubmitting.value : submitting.value
})

const optionsSummary = computed(() => {
  if (!optionsData.value) return ''
  const userAction = optionsData.value.will_create_target_user
    ? '目标将自动注册同名用户，并使用该用户默认网络'
    : '目标将绑定已有同名用户，请选择该用户下的网络'
  const mode = migrationMode.value === 'live' ? '开机状态按热迁移处理' : '关机状态按冷迁移处理'
  return `${mode}，${userAction}`
})

const diskOptionsSummary = computed(() => {
  if (!diskOptionsData.value) return ''
  const mode = diskMigrationMode.value === 'live' ? '运行中硬盘将按热迁移处理' : '关机硬盘将按冷迁移处理'
  const disk = selectedDisk.value
  if (!disk) return `${mode}，请选择要迁移的硬盘和目标存储`
  const chainText = disk.backing_path ? '，链式硬盘仅迁移活动 overlay' : ''
  return `${mode}，已选择 ${disk.device}${chainText}`
})

const open = async (row) => {
  vm.value = row
  migrationKind.value = 'vm'
  optionsData.value = null
  previewData.value = null
  diskOptionsData.value = null
  sourceMigrationDisks.value = []
  Object.keys(diskStorageForm).forEach(key => delete diskStorageForm[key])
  form.node_id = null
  form.target_storage_pool_id = ''
  form.target_switch_id = null
  form.target_security_group_id = null
  form.skip_precheck = false
  form.enable_cpu_throttle = false
  form.cpu_throttle_percent = 50
  diskForm.device = ''
  diskForm.target_storage_pool_id = ''
  visible.value = true
  const res = await listNodes()
  nodes.value = res.data || []
  const first = nodes.value.find(item => item.enabled)
  if (first) {
    form.node_id = first.id
    await loadOptions()
  }
}

const handleMigrationKindChange = async () => {
  if (migrationKind.value === 'disk' && !diskOptionsData.value) {
    await loadDiskOptions()
  }
}

const loadOptions = async () => {
  optionsData.value = null
  previewData.value = null
  sourceMigrationDisks.value = []
  Object.keys(diskStorageForm).forEach(key => delete diskStorageForm[key])
  form.target_storage_pool_id = ''
  form.target_switch_id = null
  form.target_security_group_id = null
  if (!visible.value || !vm.value || !form.node_id) return
  optionsLoading.value = true
  try {
    const res = await getNodeMigrationOptions(form.node_id, { vm_name: vm.value.name })
    optionsData.value = res.data
    const enabledTargets = targetStorageTargets.value.filter(item => item.enabled)
    const defaultStorage = enabledTargets.find(item => item.is_default) || enabledTargets[0]
    if (defaultStorage) {
      form.target_storage_pool_id = defaultStorage.id
    }
    await loadSourceMigrationDisks()
    applyDefaultStorageToDisks()
    if (res.data?.target_switch_id) {
      form.target_switch_id = res.data.target_switch_id
    }
    if (res.data?.target_security_group_id) {
      form.target_security_group_id = res.data.target_security_group_id
    }
  } finally {
    optionsLoading.value = false
  }
}

const loadSourceMigrationDisks = async () => {
  sourceMigrationDisks.value = []
  if (!vm.value) return
  sourceDisksLoading.value = true
  try {
    const res = await getDiskList(vm.value.name)
    sourceMigrationDisks.value = (res.data || []).filter(item => item.device_type === 'disk' && item.path)
  } finally {
    sourceDisksLoading.value = false
  }
}

const applyDefaultStorageToDisks = () => {
  sourceMigrationDisks.value.forEach((disk) => {
    if (!diskStorageForm[disk.device]) {
      diskStorageForm[disk.device] = form.target_storage_pool_id
    }
  })
}

const loadDiskOptions = async () => {
  diskOptionsData.value = null
  diskForm.device = ''
  diskForm.target_storage_pool_id = ''
  if (!visible.value || !vm.value) return
  diskOptionsLoading.value = true
  try {
    const res = await getDiskMigrationOptions(vm.value.name)
    diskOptionsData.value = res.data
    const firstDisk = diskMigrationDisks.value.find(item => item.can_migrate)
    if (firstDisk) {
      diskForm.device = firstDisk.device
    }
    const enabledTargets = diskTargetStorageTargets.value.filter(item => item.enabled)
    const defaultStorage = enabledTargets.find(item => item.is_default) || enabledTargets[0]
    if (defaultStorage) {
      diskForm.target_storage_pool_id = defaultStorage.id
    }
  } finally {
    diskOptionsLoading.value = false
  }
}

const markDirty = () => {
  previewData.value = null
  applyDefaultStorageToDisks()
}

const handleDefaultStorageChange = () => {
  previewData.value = null
  sourceMigrationDisks.value.forEach((disk) => {
    diskStorageForm[disk.device] = form.target_storage_pool_id
  })
}

const buildDiskStorageTargets = () => {
  return sourceMigrationDisks.value
    .filter(disk => disk.device && diskStorageForm[disk.device])
    .map(disk => ({
      target: disk.device,
      device: disk.device,
      target_storage_pool_id: diskStorageForm[disk.device]
    }))
}

const buildPreviewPayload = () => ({
  node_id: form.node_id,
  mode: migrationMode.value,
  skip_precheck: form.skip_precheck,
  target_storage_pool_id: form.target_storage_pool_id,
  disk_storage_targets: buildDiskStorageTargets(),
  target_switch_id: form.target_switch_id || 0,
  target_security_group_id: form.target_security_group_id || 0,
  enable_cpu_throttle: form.enable_cpu_throttle,
  cpu_throttle_percent: form.cpu_throttle_percent || 50
})

const buildSubmitPayload = () => ({
  node_id: previewData.value?.node?.id || form.node_id,
  mode: previewData.value?.mode || migrationMode.value,
  preview_id: form.skip_precheck ? '' : (previewData.value?.preview_id || ''),
  skip_precheck: form.skip_precheck,
  target_storage_pool_id: previewData.value?.target_storage_pool_id || form.target_storage_pool_id,
  disk_storage_targets: buildDiskStorageTargets(),
  target_switch_id: previewData.value?.target_switch_id || form.target_switch_id || 0,
  target_security_group_id: previewData.value?.target_security_group_id || form.target_security_group_id || 0,
  enable_cpu_throttle: form.enable_cpu_throttle,
  cpu_throttle_percent: form.cpu_throttle_percent || 50
})

const preview = async () => {
  if (!canPreview.value) {
    ElMessage.warning('请先补全目标节点、目标存储和网络选择')
    return
  }
  previewLoading.value = true
  try {
    const res = await previewVmMigration(vm.value.name, buildPreviewPayload())
    previewData.value = res.data
    if (res.data?.target_switch_id) {
      form.target_switch_id = res.data.target_switch_id
    }
    if (res.data?.target_security_group_id) {
      form.target_security_group_id = res.data.target_security_group_id
    }
    if (res.data?.target_storage_pool_id) {
      form.target_storage_pool_id = res.data.target_storage_pool_id
    }
  } finally {
    previewLoading.value = false
  }
}

const submit = async () => {
  if (migrationKind.value === 'disk') {
    await submitDiskMigration()
    return
  }
  if (!canSubmit.value) {
    ElMessage.warning('请先补全目标节点、目标存储和网络选择')
    return
  }
  submitting.value = true
  try {
    const res = await migrateVm(vm.value.name, buildSubmitPayload())
    ElMessage.success(res.message || '迁移任务已提交')
    visible.value = false
    emit('success')
  } finally {
    submitting.value = false
  }
}

const submitDiskMigration = async () => {
  if (!canDiskSubmit.value) {
    ElMessage.warning('请先选择可迁移硬盘和目标存储')
    return
  }
  diskSubmitting.value = true
  try {
    const res = await migrateDisk(vm.value.name, diskForm.device, {
      target_storage_pool_id: diskForm.target_storage_pool_id
    })
    ElMessage.success(res.message || '硬盘迁移任务已提交')
    visible.value = false
    emit('success')
  } finally {
    diskSubmitting.value = false
  }
}

const selectDiskRow = (row) => {
  if (row?.can_migrate) {
    diskForm.device = row.device
  }
}

const storageLabel = (item) => {
  const name = item.display_name || item.id
  return `${name}（可用 ${formatBytes(item.available)}）`
}

const storageName = (id) => {
  const item = targetStorageTargets.value.find(target => target.id === id)
  return item?.display_name || id || '-'
}

const formatBytes = (value) => {
  const size = Number(value || 0)
  if (size <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = size
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

const formatMiB = (value) => {
  const number = Number(value || 0)
  return `${number.toFixed(2)} MiB`
}

const formatPercent = (value) => {
  const number = Number(value || 0)
  return `${number.toFixed(1)}%`
}

defineExpose({ open })
</script>

<style scoped>
.summary {
  margin-top: 12px;
}

.mode-hint {
  color: var(--el-text-color-secondary);
  margin-left: 10px;
}

.form-tip {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
  margin-top: 4px;
}
</style>
