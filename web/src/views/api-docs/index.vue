<template>
  <div class="api-docs-container">
    <div class="api-docs-header">
      <div>
        <h2>接口文档</h2>
        <el-text type="info">外部程序可使用账户安全设置中生成的 API ID 和 API Key 调用兼容接口。</el-text>
      </div>
      <div class="header-actions">
        <el-tag effect="plain">{{ filteredEndpoints.length }} 个接口</el-tag>
        <el-button type="primary" @click="copyText(curlExample)">复制认证示例</el-button>
      </div>
    </div>

    <el-card class="doc-section">
      <template #header>认证与响应</template>
      <el-row :gutter="16">
        <el-col :xs="24" :md="12">
          <p>推荐使用独立请求头传递凭证，避免将 Key 放入 URL。登录、安全初始化、邮箱和 2FA 流程不接受 API Key。</p>
          <pre class="code-block">{{ curlExample }}</pre>
        </el-col>
        <el-col :xs="24" :md="12">
          <p>接口返回统一 JSON 结构。`code` 为 `200` 或 `0` 表示成功，其他值表示失败。</p>
          <pre class="code-block">{{ responseExample }}</pre>
        </el-col>
      </el-row>
    </el-card>

    <el-card class="doc-section">
      <template #header>接口检索</template>
      <div class="filters">
        <el-input v-model="keyword" placeholder="搜索路径、说明、请求字段" clearable />
        <el-select v-model="activeGroup" placeholder="选择模块" clearable>
          <el-option label="全部模块" value="" />
          <el-option v-for="group in endpointGroups" :key="group.name" :label="group.name" :value="group.name" />
        </el-select>
        <el-checkbox v-model="onlyHighRisk">只看二次验证</el-checkbox>
      </div>
    </el-card>

    <el-card
      v-for="group in visibleGroups"
      :key="group.name"
      class="doc-section"
    >
      <template #header>
        <div class="group-title">
          <span>{{ group.name }}</span>
          <el-tag size="small" effect="plain">{{ group.endpoints.length }} 个</el-tag>
        </div>
      </template>
      <p>{{ group.description }}</p>
      <el-collapse>
        <el-collapse-item
          v-for="endpoint in group.endpoints"
          :key="`${endpoint.method}-${endpoint.path}`"
          :name="`${endpoint.method}-${endpoint.path}`"
        >
          <template #title>
            <div class="endpoint-title">
              <el-tag :type="methodType(endpoint.method)" size="small">{{ endpoint.method }}</el-tag>
              <code>{{ endpoint.path }}</code>
              <span>{{ endpoint.summary }}</span>
              <el-tag v-if="endpoint.highRisk" type="warning" size="small">二次验证</el-tag>
            </div>
          </template>

          <div class="endpoint-detail">
            <el-descriptions :column="1" border>
              <el-descriptions-item label="认证">{{ endpoint.auth }}</el-descriptions-item>
              <el-descriptions-item label="请求头">
                <div class="line-list">
                  <code v-for="header in endpoint.headers" :key="header">{{ header }}</code>
                </div>
              </el-descriptions-item>
              <el-descriptions-item label="路径参数">{{ formatList(endpoint.pathParams) }}</el-descriptions-item>
              <el-descriptions-item label="查询参数">{{ formatList(endpoint.query) }}</el-descriptions-item>
              <el-descriptions-item label="请求体">{{ endpoint.body }}</el-descriptions-item>
              <el-descriptions-item label="返回">{{ endpoint.response }}</el-descriptions-item>
              <el-descriptions-item v-if="endpoint.highRisk" label="二次验证">{{ endpoint.highRisk }}</el-descriptions-item>
              <el-descriptions-item v-if="endpoint.notes?.length" label="备注">
                <div class="note-list">
                  <el-tag v-for="note in endpoint.notes" :key="note" size="small" effect="plain">{{ note }}</el-tag>
                </div>
              </el-descriptions-item>
            </el-descriptions>

            <div class="field-section">
              <h4>请求参数解释</h4>
              <el-table :data="requestFields(endpoint)" border size="small">
                <el-table-column prop="name" label="参数" min-width="170" />
                <el-table-column prop="location" label="位置" width="100" />
                <el-table-column prop="required" label="必填" width="80" />
                <el-table-column prop="description" label="说明" min-width="260" />
              </el-table>
            </div>

            <div class="field-section">
              <h4>返回参数解释</h4>
              <el-table :data="responseFields(endpoint)" border size="small">
                <el-table-column prop="name" label="字段" min-width="170" />
                <el-table-column prop="location" label="位置" width="120" />
                <el-table-column prop="description" label="说明" min-width="300" />
              </el-table>
            </div>

            <div class="example-row">
              <pre class="code-block">{{ buildCurl(endpoint) }}</pre>
              <el-button @click="copyText(buildCurl(endpoint))">复制 curl</el-button>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <el-card class="doc-section">
      <template #header>高风险操作</template>
      <p>删除虚拟机、重置密码、修改防火墙等接口仍会要求二次验证。API 调用收到 `428` 后，先完成 `/api/auth/high-risk/verify`，再在原请求携带 `X-High-Risk-Token`。</p>
      <pre class="code-block">{{ highRiskExample }}</pre>
    </el-card>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { copyTextWithFallback } from '@/utils/clipboard'
import { endpointGroups } from './endpointDocs'

const apiBase = `${window.location.origin}/api`
const keyword = ref('')
const activeGroup = ref('')
const onlyHighRisk = ref(false)

const curlExample = `curl -H "X-API-Key-ID: kvm_id_xxx" \\
  -H "X-API-Key: kvm_sk_xxx" \\
  "${apiBase}/vm/list"`

const responseExample = `{
  "code": 200,
  "message": "ok",
  "data": {}
}`

const highRiskExample = `curl -X POST "${apiBase}/auth/high-risk/verify" \\
  -H "X-API-Key-ID: kvm_id_xxx" \\
  -H "X-API-Key: kvm_sk_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{"method":"totp","code":"123456","operation":"delete_vm"}'`

const normalizedKeyword = computed(() => keyword.value.trim().toLowerCase())

const visibleGroups = computed(() => {
  return endpointGroups
    .filter(group => !activeGroup.value || group.name === activeGroup.value)
    .map(group => ({
      ...group,
      endpoints: group.endpoints.filter(endpoint => matchEndpoint(endpoint))
    }))
    .filter(group => group.endpoints.length)
})

const filteredEndpoints = computed(() => visibleGroups.value.flatMap(group => group.endpoints))

const matchEndpoint = (endpoint) => {
  if (onlyHighRisk.value && !endpoint.highRisk) return false
  const key = normalizedKeyword.value
  if (!key) return true
  return [
    endpoint.method,
    endpoint.path,
    endpoint.summary,
    endpoint.auth,
    endpoint.body,
    endpoint.response,
    endpoint.highRisk,
    ...(endpoint.pathParams || []),
    ...(endpoint.query || []),
    ...(endpoint.notes || [])
  ].join(' ').toLowerCase().includes(key)
}

const methodType = (method) => {
  if (method === 'GET') return 'success'
  if (method === 'POST') return 'primary'
  if (method === 'PUT' || method === 'PATCH') return 'warning'
  if (method === 'DELETE') return 'danger'
  return 'info'
}

const formatList = (items) => {
  return items?.length ? items.join(', ') : '无'
}

const fieldDescriptions = {
  action: '要执行的动作，例如开机、关机、重启、删除、启用或禁用。',
  address: '公网 IP 地址或资源地址。',
  accounts: '可选择的账号列表。',
  admin_name: '管理员侧展示名称。',
  allow_forged_transmits: '是否允许伪造发送，桥接安全相关选项。',
  allow_mac_change: '是否允许修改 MAC 地址，桥接安全相关选项。',
  allow_panel: '是否自动放行面板端口。',
  allow_promiscuous: '是否允许混杂模式，桥接安全相关选项。',
  allowed_methods: '当前登录阶段允许使用的验证方式。',
  apic: '是否启用 APIC，高级可编程中断控制器。',
  arch: '目标架构；virt_type 为 qemu 时可指定，例如 x86_64、aarch64、riscv64。',
  autostart: '是否随宿主机或服务启动自动启动虚拟机。',
  base64: 'SMBIOS 字段是否按 base64 方式写入。',
  bandwidth_down_mbps: '下行带宽限制，单位 Mbps。',
  bandwidth_inbound_avg: '入站平均带宽限制，单位 Mbps。',
  bandwidth_mbps: '总带宽限制，单位 Mbps。',
  bandwidth_outbound_avg: '出站平均带宽限制，单位 Mbps。',
  bandwidth_up_mbps: '上行带宽限制，单位 Mbps。',
  block_action: '防火墙阻断动作，例如 drop 或 reject。',
  boot_order: '虚拟机启动顺序。',
  boot_type: '启动类型，例如 BIOS 或 UEFI。',
  bridge_mode: 'VPC 交换机桥接模式，例如 nat 或 bridge。',
  bridge_name: '网桥名称。',
  bridge_vlan_id: '桥接 VLAN ID。',
  bus: '磁盘总线类型；在 extra_disks 中表示额外磁盘总线。',
  category: '分类字段；模板场景中表示 Linux/Windows 二级分类，用户存储场景中通常为 iso、share 或 disk。',
  challenge_id: '验证码挑战 ID，发送验证码接口返回后用于后续校验。',
  cidr: 'CIDR 网段，例如 10.0.0.0/24。',
  clone_visible: '模板是否在克隆入口中可见。',
  cloud_type: '用户云类型，例如 elastic 或 lightweight。',
  code: '业务状态码；响应中 200 或 0 表示成功。请求中表示验证码。',
  command: '要执行的 QEMU Monitor 命令。',
  confirm_options: '确认开通轻量云服务器时的附加确认选项。',
  confirm_password: '确认密码，必须与 password 一致。',
  cron: '定时任务 cron 表达式。',
  default_action: '防火墙默认动作。',
  delete_disks: '删除 VM 时是否同时删除磁盘文件。',
  delete_files: '删除模板时是否同时删除模板文件。',
  description: '描述信息。',
  dev: '虚拟磁盘设备名，例如 vda。',
  dhcp_end: 'DHCP 地址池结束地址。',
  dhcp_start: 'DHCP 地址池起始地址。',
  direction: '规则方向，通常为 inbound 或 outbound。',
  disk_size: '磁盘大小，通常单位为 GB。',
  disk_format: '系统盘格式，例如 qcow2 或 raw；留空时使用后端默认值。',
  disk_bus: '系统盘总线类型，例如 virtio、scsi、sata 或 ide。',
  display_name: '前端展示名称。',
  dns: 'DNS 服务器地址。',
  dynamic_enabled: '是否启用动态内存。',
  email: '邮箱地址。',
  enabled: '是否启用。',
  event_type: '调度事件类型。',
  execute_at: '一次性执行时间。',
  expires_in: '有效期秒数。',
  expose: '是否将 VNC 对外暴露。',
  extra_disks: '额外数据盘数组，每项包含 size、format、bus、storage_pool_id。',
  external_nic: '宿主机外网网卡名称。',
  file: '上传文件字段。',
  filename: '文件名。',
  filter: '抓包过滤表达式。',
  force: '是否强制执行。',
  force_new: '是否强制新建设备或配置。',
  freeze: '是否冻结虚拟机配置或启动状态。',
  family: 'SMBIOS family 字段。',
  format: '磁盘或文件格式；在 extra_disks 中表示额外磁盘格式。',
  gateway: '网关地址。',
  geoip_base_url: 'GeoIP 或地域库下载基础地址。',
  guest_agent: '是否启用 QEMU Guest Agent 相关能力。',
  guest_ip: '虚拟机内网 IP。',
  guest_port: '虚拟机内部端口。',
  guest_type: '来宾系统类型，例如 linux、windows 或 fnos。',
  host_interface: '宿主机物理网卡。',
  host_port: '宿主机暴露端口。',
  id: '资源 ID。',
  iface: '网卡名称。',
  image_url: '展示图片地址。',
  include_ip: '是否返回 IP 信息。',
  include_memory: '快照是否包含内存状态。内存快照耗时取决于虚拟机内存大小。',
  pause_for_memory_snapshot: '创建包含内存的运行中快照时，是否由面板主动暂停虚拟机再创建。默认 true（推荐）。false 仅不主动暂停，QEMU 保存内存时 VM 仍会进入 paused (saving) 状态。',
  auto_fix_nvram: '确认自动关机修复 UEFI NVRAM 后继续创建内存快照。',
  include_resource_usage: '是否附带资源使用统计。',
  include_snapshots: '导出时是否包含快照。',
  inbound_allowed_regions: '入站允许区域列表。',
  inbound_enabled: '是否启用入站区域控制。',
  interface_name: '网卡或抓包接口名称。',
  ip: 'IP 地址。',
  iso_path: 'ISO 镜像路径。',
  iso_paths: 'ISO 镜像路径列表，首个路径会作为主安装盘。',
  key: '规则或配置键。',
  key_prefix: 'API Key 的脱敏显示前后缀。',
  keyword: '搜索关键字。',
  mac: 'MAC 地址。',
  machine_type: '虚拟机机器类型，例如 q35 或 i440fx。',
  maintenance_mode: '是否启用维护模式。',
  manufacturer: 'SMBIOS manufacturer 厂商字段。',
  max_bandwidth_down: '最大下行带宽配额。',
  max_bandwidth_up: '最大上行带宽配额。',
  max_cpu: 'CPU 配额。',
  max_disk: '磁盘配额。',
  max_memory: '内存配额。',
  max_mb: '最大抓包文件大小，单位 MB。',
  max_packets: '最大抓包包数量。',
  max_port_forwards: '端口转发数量配额。',
  max_public_ips: '公网 IP 配额。',
  max_runtime_hours: '总运行时长配额，单位小时。',
  max_snapshots: '快照数量配额。',
  max_storage: '用户存储配额。',
  max_traffic_down: '下行流量配额。',
  max_traffic_up: '上行流量配额。',
  max_vm: '最大虚拟机数量。',
  memory: '内存大小或内存配置。',
  memory_auto_balloon: '动态内存是否允许自动 balloon 调整。',
  memory_backend: '动态内存后端，例如 balloon 或 virtio_mem。',
  memory_current: '当前内存值，通常由运行态或表单传入。',
  memory_dynamic: '动态内存配置。',
  memory_initial: '虚拟机初始内存。',
  memory_max: '动态内存最大值。',
  memory_min: '动态内存最小值。',
  message: '响应消息，失败时为中文错误说明。',
  method: '验证方式或请求处理方式。',
  mode: '操作模式，具体取值取决于接口。',
  name: '资源名称。',
  new_password: '新密码。',
  new_username: '新用户名。',
  nic_model: '虚拟网卡型号，例如 virtio。',
  old_password: '当前旧密码。',
  operation: '高风险验证对应的操作标识。',
  os_type: '操作系统类型。',
  os_variant: 'libosinfo 系统变体。',
  outbound_allowed_regions: '出站允许区域列表。',
  outbound_enabled: '是否启用出站区域控制。',
  pae: '是否启用 PAE。',
  page: '页码。',
  page_size: '每页数量。',
  params: '扩展参数对象。',
  password: '密码。用于登录、VNC、系统账户或二次验证确认时按接口语义处理。',
  path: '文件路径。',
  policy: '防火墙策略对象。',
  port: '端口号。',
  port_end: '端口范围结束。',
  port_start: '端口范围起始。',
  profile: '宿主机功能挡位，例如 off、balanced。',
  protocol: '协议，通常为 tcp、udp、icmp 或 all。',
  product: 'SMBIOS product 产品字段。',
  public_ip_id: '公网 IP 资源 ID。',
  publish: '是否发布模板。',
  published: '是否已发布。',
  ram: '内存大小，通常单位为 MB 或 GB，按接口表单语义处理。',
  reason: '原因或备注。',
  regions: '地域规则列表。',
  remark: '备注。',
  reset_token: '重置密码令牌。',
  role: '用户角色，例如 admin 或 user。',
  rules: '规则数组。',
  rtc_offset: '虚拟机 RTC 时钟偏移。',
  rtc_startdate: '虚拟机 RTC 起始时间。',
  run_at: '定时任务执行时间。',
  schedule_type: '定时任务类型。',
  scope: '作用范围，例如 node 或 all。',
  security: '账户安全状态对象。',
  security_group_id: 'VPC 安全组 ID。',
  selection_token: '忘记密码选择账号阶段令牌。',
  serial: 'SMBIOS serial 序列号字段。',
  site_title: '站点标题。',
  size: '大小；在 extra_disks 中表示额外磁盘大小，通常单位为 GB。',
  smbios1: 'libvirt SMBIOS type=1 配置对象，用于模拟厂商、产品、序列号等信息。',
  sku: 'SMBIOS SKU 字段。',
  smtp_host: 'SMTP 主机。',
  smtp_password: 'SMTP 密码。',
  smtp_port: 'SMTP 端口。',
  smtp_security: 'SMTP 加密方式。',
  source: '来源地址或数据源。',
  source_cidr: '来源 CIDR。',
  source_path: '源文件路径。',
  stage: '登录阶段，可能为 success、login_verify 或 bootstrap_security。',
  start: '查询开始时间。',
  status: '资源状态。',
  storage_pool_id: '存储池 ID。',
  switch_id: 'VPC 交换机 ID。',
  tag: '共享目录或挂载标签。',
  tags: '标签列表。',
  target_storage: '目标存储位置。',
  target_type: '目标类型，例如 cidr、switch、security_group。',
  target_value: '目标值。',
  task_id: '任务 ID。',
  template: '模板名称或模板对象。',
  template_name: '模板名称。',
  timezone: '时区。',
  token: '流程令牌或下载令牌。',
  traffic_down_gb: '下行流量配额，单位 GB。',
  traffic_up_gb: '上行流量配额，单位 GB。',
  transfer: '是否转移磁盘文件。',
  transfer_disks: '删除 VM 时要转移的磁盘列表。',
  transfer_user: '磁盘转移目标用户。',
  type: '类型。',
  uplink_if: '上联网卡。',
  username: '用户名。',
  uuid: 'SMBIOS UUID 字段或资源唯一标识。',
  vcpu: '虚拟 CPU 核心数。',
  version: 'SMBIOS version 版本字段。',
  video_model: '虚拟显卡型号。',
  cpu_limit_percent: 'CPU 限制百分比，0 表示无限制；管理员字段。',
  vlan_id: 'VLAN ID。',
  virt_type: '虚拟化方案，例如 kvm 或 qemu。',
  vm_ip: '虚拟机 IP。',
  vm_name: '虚拟机名称。',
  vms: '虚拟机名称列表。',
  weekdays: '每周执行日期。',
  watchdog: '虚拟机 watchdog 设备策略。',
  xml: 'libvirt domain XML 内容'
}

const ignoredFieldTokens = new Set([
  'JSON', 'FormData', 'Content', 'Type', 'application', 'json', 'multipart', 'form', 'data',
  'true', 'false', 'null', 'tcp', 'udp', 'icmp', 'all', 'GET', 'POST', 'PUT', 'DELETE',
  'success', 'login_verify', 'bootstrap_security', 'node', 'off', 'balanced',
  'kvm', 'qemu', 'x86_64', 'aarch64', 'riscv64', 'q35', 'i440fx',
  'virtio', 'scsi', 'sata', 'ide', 'raw', 'qcow2'
])

const normalizeParamName = (raw) => String(raw || '')
  .replace(/^:/, '')
  .replace(/\(.+?\)/g, '')
  .replace(/（.+?）/g, '')
  .replace(/<|>/g, '')
  .trim()

const describeField = (name) => {
  const normalized = normalizeParamName(name)
  return fieldDescriptions[normalized] || '业务字段，按该接口请求体说明填写；后端会按当前资源状态、权限和配额进行校验。'
}

const makeField = (name, location, required = '否', description = '') => ({
  name: normalizeParamName(name),
  location,
  required,
  description: description || describeField(name)
})

const extractFieldsFromText = (text) => {
  if (!text || text === '无') return []
  const matches = String(text).match(/[A-Za-z][A-Za-z0-9_]*|[a-z]+_[a-z0-9_]+/g) || []
  const seen = new Set()
  return matches
    .map(normalizeParamName)
    .filter(name => {
      if (!name || ignoredFieldTokens.has(name) || ignoredFieldTokens.has(name.toUpperCase())) return false
      if (name.length < 2 && name !== 'id') return false
      if (seen.has(name)) return false
      seen.add(name)
      return /[a-z_]/.test(name)
    })
}

const requestFields = (endpoint) => {
  const fields = []
  ;(endpoint.pathParams || []).forEach(name => fields.push(makeField(name, 'Path', '是')))
  ;(endpoint.query || []).forEach(name => fields.push(makeField(name, 'Query', '否')))
  extractFieldsFromText(endpoint.body).forEach(name => {
    const required = endpoint.requiredFields?.includes(name) ? '是' : '否'
    fields.push(makeField(name, endpoint.body?.startsWith('FormData') ? 'FormData' : 'Body', required))
  })
  if (!fields.length) {
    fields.push({ name: '无', location: '-', required: '-', description: '该接口不需要额外请求参数。' })
  }
  return fields
}

const responseFields = (endpoint) => {
  const fields = [
    { name: 'code', location: 'Root', description: fieldDescriptions.code },
    { name: 'message', location: 'Root', description: fieldDescriptions.message },
    { name: 'data', location: 'Root', description: endpoint.response || '接口业务数据。' }
  ]
  extractFieldsFromText(endpoint.response).forEach(name => {
    if (!['code', 'message', 'data'].includes(name)) {
      fields.push({ name, location: 'data', description: describeField(name) })
    }
  })
  if (endpoint.response?.includes('文件流')) {
    fields.push({ name: 'binary', location: 'Response Body', description: '文件下载接口直接返回二进制文件流，不使用统一 JSON data。' })
  }
  if (endpoint.response?.includes('text/event-stream')) {
    fields.push({ name: 'event', location: 'SSE', description: '服务端事件流数据，客户端需要按 EventSource/SSE 协议读取。' })
  }
  return fields
}

const buildCurl = (endpoint) => {
  const url = endpoint.path
    .replace(/:name/g, 'vm-name')
    .replace(/:username/g, 'username')
    .replace(/:filename/g, 'file-name')
    .replace(/:rule_key/g, 'rule-key')
    .replace(/:vm_name/g, 'vm-name')
    .replace(/:vmName/g, 'vm-name')
    .replace(/:task_id/g, '1')
    .replace(/:snap/g, 'snapshot-name')
    .replace(/:tag/g, 'tag')
    .replace(/:dev/g, 'vda')
    .replace(/:category/g, 'iso')
    .replace(/:id/g, '1')
  const lines = [`curl -X ${endpoint.method} "${apiBase}${url}"`]
  endpoint.headers
    .filter(header => header !== '无')
    .forEach(header => {
      lines.push(`  -H "${header}"`)
    })
  const body = sampleBody(endpoint)
  if (body) {
    lines.push(`  -d '${body}'`)
  }
  return lines.join(' \\\n')
}

const sampleBody = (endpoint) => {
  if (endpoint.body === '无' || endpoint.body?.startsWith('FormData')) return ''
  if (!['POST', 'PUT', 'PATCH', 'DELETE'].includes(endpoint.method)) return ''
  if (endpoint.path === '/vm/create') {
    return '{"name":"demo","vcpu":2,"ram":4096,"disk_size":40,"disk_format":"qcow2","disk_bus":"virtio","os_variant":"generic","iso_path":"/var/lib/libvirt/images/ISO/example.iso","iso_paths":["/var/lib/libvirt/images/ISO/example.iso","/var/lib/libvirt/images/ISO/virtio-win.iso"],"nic_model":"virtio","autostart":false,"freeze":false,"apic":true,"pae":true,"rtc_offset":"utc","rtc_startdate":"now","guest_agent":{"enabled":true},"smbios1":{"base64":false},"os_type":"linux","machine_type":"q35","boot_type":"bios","watchdog":"none","boot_order":["cdrom","disk"],"video_model":"virtio","cpu_limit_percent":80,"virt_type":"kvm","memory_dynamic":{"dynamic_enabled":false,"memory_backend":"balloon","memory_initial":4096},"switch_id":1,"security_group_id":1,"storage_pool_id":"default","extra_disks":[{"size":20,"format":"qcow2","bus":"virtio","storage_pool_id":"default"}]}'
  }
  if (endpoint.path === '/self/vm/create') {
    return '{"name":"demo","vcpu":2,"ram":4096,"disk_size":40,"disk_format":"qcow2","disk_bus":"virtio","os_variant":"generic","iso_path":"/mnt/user/iso/example.iso","iso_paths":["/mnt/user/iso/example.iso","/mnt/user/iso/virtio-win.iso"],"nic_model":"virtio","autostart":false,"freeze":false,"apic":true,"pae":true,"rtc_offset":"utc","rtc_startdate":"now","guest_agent":{"enabled":true},"smbios1":{"base64":false},"os_type":"linux","machine_type":"q35","boot_type":"bios","boot_order":["cdrom","disk"],"video_model":"virtio","memory_dynamic":{"dynamic_enabled":false,"memory_backend":"balloon","memory_initial":4096},"switch_id":1,"security_group_id":1,"storage_pool_id":"default","extra_disks":[{"size":20,"format":"qcow2","bus":"virtio","storage_pool_id":"default"}]}'
  }
  if (endpoint.body?.includes('action(start')) return '{"action":"start"}'
  if (endpoint.body?.includes('enabled')) return '{"enabled":true}'
  if (endpoint.body?.includes('password')) return '{"password":"StrongPassword123!"}'
  if (endpoint.body?.includes('security_group_id')) return '{"security_group_id":1}'
  if (endpoint.body?.includes('size_gb')) return '{"size_gb":20}'
  if (endpoint.body?.includes('profile')) return '{"profile":"balanced"}'
  if (endpoint.body?.includes('email')) return '{"email":"user@example.com"}'
  if (endpoint.body?.includes('username')) return '{"username":"test"}'
  if (endpoint.body?.includes('xml')) return '{"xml":"<domain>...</domain>"}'
  return '{"example":"请按请求体说明填写"}'
}

const copyText = async (text) => {
  try {
    await copyTextWithFallback(text)
    ElMessage.success('已复制')
  } catch (err) {
    ElMessage.error(err.message || '复制失败')
  }
}
</script>

<style scoped>
.api-docs-container {
  padding: 10px;
}
.api-docs-header,
.header-actions,
.filters,
.group-title,
.endpoint-title,
.example-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.api-docs-header {
  justify-content: space-between;
  margin-bottom: 16px;
}
.filters {
  align-items: center;
}
.filters .el-input {
  max-width: 420px;
}
.filters .el-select {
  width: 180px;
}
h2 {
  margin: 0 0 8px;
  font-size: 18px;
}
.doc-section {
  margin-bottom: 16px;
}
.doc-section p {
  margin: 0 0 12px;
  color: #606266;
  line-height: 1.7;
}
.group-title {
  justify-content: space-between;
}
.endpoint-title {
  min-width: 0;
  flex-wrap: wrap;
}
.endpoint-title code {
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.endpoint-title span {
  color: #606266;
}
.endpoint-detail {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.field-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.field-section h4 {
  margin: 0;
  font-size: 14px;
  color: var(--el-text-color-primary);
}
.line-list,
.note-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.line-list code {
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--el-fill-color-light);
}
.code-block {
  margin: 0;
  padding: 12px;
  flex: 1;
  overflow-x: auto;
  border-radius: 6px;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
  font-size: 13px;
  line-height: 1.6;
}
.example-row {
  align-items: flex-start;
}
@media (max-width: 768px) {
  .api-docs-header,
  .header-actions,
  .filters,
  .example-row {
    align-items: stretch;
    flex-direction: column;
  }
  .filters .el-input,
  .filters .el-select {
    max-width: none;
    width: 100%;
  }
}
</style>
