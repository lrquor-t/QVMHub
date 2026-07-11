<template>
  <div class="dashboard-container">
    <!-- SMTP 未配置安全警告横幅 -->
    <el-alert
      v-if="isAdmin && !smtpConfigured"
      type="error"
      :closable="false"
      class="security-warning-banner"
    >
      <template #title>
        <div class="banner-content">
          <el-icon size="18"><WarningFilled /></el-icon>
          <span>安全警告：SMTP 邮件服务尚未配置，邮箱绑定、邀请注册、找回密码和敏感操作二次验证等功能将无法使用。请尽快前往<el-link type="warning" :underline="true" @click="$router.push('/settings')">系统设置</el-link>完成 SMTP 配置，或确保当前处于安全可信的网络环境中。</span>
        </div>
      </template>
    </el-alert>

    <!-- ========== 管理员首页 ========== -->
    <template v-if="isAdmin">
      <!-- 实时监控环形图 -->
      <div class="section">
        <h2 class="section-title">实时资源监控</h2>
        <el-row :gutter="16" class="monitor-panel">
          <el-col :span="6" :xs="12">
            <el-card shadow="hover" class="ring-card">
              <div class="ring-label-top">CPU 使用率</div>
              <div class="ring-chart" ref="cpuRingRef"></div>
            </el-card>
          </el-col>
          <el-col :span="6" :xs="12">
            <el-card shadow="hover" class="ring-card">
              <div class="ring-label-top">内存使用率</div>
              <div class="ring-chart" ref="memRingRef"></div>
            </el-card>
          </el-col>
          <el-col :span="6" :xs="12">
            <el-card shadow="hover" class="ring-card">
              <div class="ring-label-top">Swap 使用率</div>
              <div class="ring-chart" ref="swapRingRef"></div>
            </el-card>
          </el-col>
          <el-col :span="6" :xs="12">
            <el-card shadow="hover" class="ring-card">
              <div class="ring-label-top">磁盘 IO 延迟</div>
              <div class="io-delay-wrap" v-loading="hostLoading">
                <div class="io-delay-value" :style="{ color: ioDelayColor }">{{ hostData.disk_io_latency_ms || '0' }}</div>
                <div class="io-delay-unit">ms (avg)</div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <!-- 物理机概览卡片 -->
        <el-row :gutter="16" style="margin-top: 16px;">
          <el-col :span="6" :xs="12" v-for="(stat, idx) in adminOverviewCards" :key="idx">
            <el-card shadow="hover" class="overview-card">
              <div class="overview-accent" :style="{ background: stat.color }"></div>
              <div class="overview-body">
                <div class="overview-header">
                  <el-icon :size="20" :color="stat.color"><component :is="stat.icon" /></el-icon>
                  <span class="overview-label">{{ stat.label }}</span>
                </div>
                <div class="overview-value">{{ stat.value }}</div>
                <div class="overview-sub" v-if="stat.sub">{{ stat.sub }}</div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>

      <!-- 挂载磁盘空间 -->
      <div class="section">
        <h2 class="section-title">挂载磁盘空间</h2>
        <el-card shadow="hover">
          <el-table :data="hostDisks" stripe size="default" v-loading="diskLoading" empty-text="暂无磁盘数据">
            <el-table-column prop="mount_point" label="挂载点" min-width="180">
              <template #default="{ row }">
                <strong>{{ row.mount_point }}</strong>
              </template>
            </el-table-column>
            <el-table-column prop="device" label="设备" min-width="120" />
            <el-table-column prop="fs_type" label="文件系统" width="100" />
            <el-table-column label="使用率" min-width="220">
              <template #default="{ row }">
                <div class="disk-bar-cell">
                  <el-progress
                    :percentage="getDiskPercent(row)"
                    :color="getDiskProgressColor(row)"
                    :stroke-width="10"
                    style="flex: 1;"
                  />
                  <span class="disk-pct-text" :style="{ color: getDiskProgressColor(row) }">{{ getDiskPercent(row) }}%</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="已用 / 总容量" min-width="180">
              <template #default="{ row }">
                <span>{{ formatKB(row.used_kb) }}</span>
                <span class="disk-details"> / {{ formatKB(row.total_kb) }}</span>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </div>

      <!-- 物理机24小时资源图表 -->
      <div class="section">
        <h2 class="section-title">物理机 24小时资源图表</h2>
        <ResourceCharts type="host" default-mode="history" />
      </div>
    </template>

    <!-- ========== 用户首页 ========== -->
    <template v-else>
      <!-- 资源总览 紧凑卡片 -->
      <div class="section">
        <h2 class="section-title">资源总览</h2>
        <el-row :gutter="16" class="stat-grid-cols5">
          <el-col :span="24" :sm="12" :md="4.8" v-for="(card, idx) in userOverviewCards" :key="idx">
            <el-card shadow="hover" class="compact-card">
              <div class="compact-accent" :style="{ background: card.color }"></div>
              <div class="compact-header">
                <el-icon :size="18" :color="card.color"><component :is="card.icon" /></el-icon>
                <span class="compact-label">{{ card.label }}</span>
              </div>
              <div class="compact-value">{{ card.used }}<span class="compact-unit"> / {{ card.maxText }}</span></div>
              <el-progress
                v-if="card.max > 0"
                :percentage="Math.min(card.percent, 100)"
                :color="card.color"
                :stroke-width="6"
                style="margin-top: 8px;"
              />
              <div class="compact-sub" v-else>{{ card.subText }}</div>
            </el-card>
          </el-col>
        </el-row>
      </div>

      <!-- 配额详情 折叠分类 -->
      <div class="section">
        <h2 class="section-title">配额详情</h2>

        <!-- 计算与实例 -->
        <el-card shadow="hover" class="quota-category-card" v-for="(cat, catIdx) in quotaCategories" :key="catIdx">
          <div class="quota-cat-header" @click="quotaCollapsed[catIdx] = !quotaCollapsed[catIdx]">
            <div class="cat-title-row">
              <el-icon :size="20" :color="cat.iconColor"><component :is="cat.icon" /></el-icon>
              <span class="cat-title-text">{{ cat.title }}</span>
            </div>
            <div class="cat-summary">{{ cat.summary }}</div>
            <el-icon class="cat-chevron" :class="{ rotated: !quotaCollapsed[catIdx] }"><ArrowDown /></el-icon>
          </div>
          <div class="quota-cat-body" v-show="!quotaCollapsed[catIdx]">
            <div v-for="(item, itemIdx) in cat.items" :key="itemIdx" class="quota-item-row">
              <div class="quota-item-icon">
                <el-icon :size="18" :color="item.color"><component :is="item.icon" /></el-icon>
              </div>
              <div class="quota-item-info">
                <div class="quota-item-label">{{ item.label }}</div>
                <div class="quota-item-desc" v-if="item.desc">{{ item.desc }}</div>
              </div>
              <div class="quota-item-progress" v-if="item.max > 0">
                <el-progress
                  :percentage="Math.min(item.percent, 100)"
                  :color="getProgressColor(item.percent / 100)"
                  :stroke-width="8"
                />
              </div>
              <div class="quota-item-value">{{ item.display }}</div>
            </div>

            <!-- 网络配额：云类型切换 -->
            <template v-if="cat.cloudTypeTabs">
              <div class="cloud-type-tabs" style="margin: 12px 0;">
                <el-radio-group v-model="activeCloudType" size="small" @change="onCloudTypeChange">
                  <el-radio-button value="elastic">弹性云</el-radio-button>
                  <el-radio-button value="lightweight">轻量云</el-radio-button>
                </el-radio-group>
              </div>
              <div v-for="(subItem, subIdx) in cat.cloudItems[activeCloudType]" :key="'cloud-'+subIdx" class="quota-item-row" :class="{ highlight: subItem.highlight }">
                <div class="quota-item-icon">
                  <el-icon :size="18" :color="subItem.color"><component :is="subItem.icon" /></el-icon>
                </div>
                <div class="quota-item-info">
                  <div class="quota-item-label">{{ subItem.label }}</div>
                  <div class="quota-item-desc" v-if="subItem.desc">{{ subItem.desc }}</div>
                </div>
                <div class="quota-item-progress" v-if="subItem.max > 0">
                  <el-progress
                    :percentage="Math.min(subItem.percent, 100)"
                    :color="getProgressColor(subItem.percent / 100)"
                    :stroke-width="8"
                  />
                </div>
                <div class="quota-item-value">{{ subItem.display }}</div>
              </div>
              <div class="cloud-source-tag">
                <el-tag :type="activeCloudType === 'elastic' ? 'primary' : 'success'" size="small" effect="plain">
                  {{ activeCloudType === 'elastic' ? '流量来源：VPC 虚拟交换机' : '流量来源：宿主机网桥' }}
                </el-tag>
              </div>
            </template>
          </div>
        </el-card>
      </div>
    </template>

    <!-- ========== 虚拟机资源追踪（公共区域） ========== -->
    <div class="section">
      <div class="vm-section-header">
        <h2 class="section-title" style="margin-bottom: 0;">{{ isAdmin ? '所有虚拟机' : '我的虚拟机' }} 资源追踪</h2>
        <el-space v-if="!isAdmin">
          <el-button type="primary" icon="Plus" @click="$router.push('/vm/list?action=clone')">从模板创建</el-button>
          <el-button icon="Monitor" @click="$router.push('/vm/list')">管理我的虚拟机</el-button>
        </el-space>
      </div>

      <el-collapse v-model="activeVmNames" class="custom-collapse" v-loading="vmLoading">
        <el-collapse-item
          v-for="vm in myVMs"
          :key="vm.name"
          :name="vm.name"
          class="vm-collapse-item"
        >
          <template #title>
            <div class="vm-collapse-title">
              <div class="vm-name">
                <el-icon><Monitor /></el-icon>
                <span>{{ vm.name }}</span>
              </div>
              <div class="vm-status">
                <el-tag :type="vm.status === 'running' ? 'success' : 'info'" size="small" effect="dark">
                  {{ vm.status === 'running' ? '运行中' : vm.status === 'shut off' ? '已关机' : vm.status }}
                </el-tag>
              </div>
              <div class="vm-meta">
                <span class="meta-item"><el-icon><Cpu /></el-icon> {{ vm.vcpu }} 核</span>
                <span class="meta-item"><el-icon><Coin /></el-icon> {{ Math.round(vm.max_memory / 1024) || '-' }} GB</span>
                <span class="meta-item"><el-icon><Connection /></el-icon> {{ vm.ip || '未分配 IP' }}</span>
              </div>
            </div>
          </template>
          
          <div v-if="activeVmNames.includes(vm.name)" style="padding-top: 15px;">
            <ResourceCharts type="vm" :name="vm.name" :status="vm.status" default-mode="history" />
            <div style="text-align: right; margin-top: 10px;">
              <el-button size="small" type="primary" plain @click="$router.push(`/vm/detail/${vm.name}`)">前往虚拟机详情</el-button>
            </div>
          </div>
        </el-collapse-item>
        
        <el-empty v-if="!vmLoading && myVMs.length === 0" description="暂无虚拟机数据" />
      </el-collapse>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { getSelfQuota, getSelfVMs } from '@/api/user'
import { getVmList } from '@/api/vm'
import { getHostStats, getHostDisks, createHostStatsSSE } from '@/api/host'
import { useUserStore } from '@/store/user'
import { 
  Cpu, Coin, Monitor, Download, Upload, Connection, Timer,
  FolderOpened, Camera, Link, ArrowDown, DataAnalysis, Files,
  WarningFilled
} from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import ResourceCharts from '@/components/ResourceCharts.vue'
import request from '@/utils/request'

const userStore = useUserStore()
const isAdmin = computed(() => userStore.role === 'admin')
const isLightweight = computed(() => userStore.cloudType === 'lightweight')

// SMTP 是否已配置，用于安全警告横幅
const smtpConfigured = computed(() => {
  return userStore.security?.smtp_configured === true
})

// ========== 管理员数据 ==========
const hostData = ref({})
const hostDisks = ref([])
const hostLoading = ref(false)
const diskLoading = ref(false)
const cpuRingRef = ref(null)
const memRingRef = ref(null)
const swapRingRef = ref(null)
let ringCharts = []
let hostStatsSSE = null

const ioDelayColor = computed(() => {
  const v = parseFloat(hostData.value.disk_io_latency_ms) || 0
  if (v >= 10) return '#F56C6C'
  if (v >= 5) return '#E6A23C'
  return '#409EFF'
})

const adminOverviewCards = computed(() => {
  const d = hostData.value
  const cards = [
    { label: '物理核数', value: `${d.cpu_count || 0} 核`, sub: d.hostname || '', icon: Cpu, color: '#409EFF' },
    { label: '运行中实例', value: `${d.vm_running || 0} / ${d.vm_total || 0}`, sub: '运行 / 总数', icon: Monitor, color: '#67C23A' },
    { label: '运行时间', value: d.uptime || '-', sub: (d.arch || '') + (d.arch && d.hostname ? ' · ' : '') + (d.hostname || ''), icon: Timer, color: '#9C6ADE' },
  ]
  if ((d.ksm_pages_sharing || 0) > 0) {
    const ksmGB = ((d.ksm_pages_sharing || 0) * 4096 / 1024 / 1024 / 1024).toFixed(1)
    cards.push({ label: 'KSM 内存合并', value: `${ksmGB} GB`, sub: '已节省内存', icon: DataAnalysis, color: '#E6A23C' })
  }
  return cards
})

// ========== 用户配额数据 ==========
const quota = ref({})
const myVMs = ref([])
const vmLoading = ref(false)
const activeVmNames = ref([])
const activeCloudType = ref('elastic')
const quotaCollapsed = reactive({ 0: false, 1: true, 2: true })

const userOverviewCards = computed(() => {
  const q = quota.value
  return [
    {
      label: 'CPU 已用 / 配额', used: `${q.used_cpu || 0}`, max: q.max_cpu || 0,
      maxText: q.max_cpu ? `${q.max_cpu} 核` : '不限', percent: q.max_cpu ? Math.round((q.used_cpu || 0) / q.max_cpu * 100) : 0,
      icon: Cpu, color: '#409EFF', subText: q.max_cpu ? `使用率 ${Math.round((q.used_cpu || 0) / (q.max_cpu || 1) * 100)}%` : '不限'
    },
    {
      label: '内存已用 / 配额', used: `${q.used_memory || 0}`, max: q.max_memory || 0,
      maxText: q.max_memory ? `${q.max_memory} GB` : '不限', percent: q.max_memory ? Math.round((q.used_memory || 0) / q.max_memory * 100) : 0,
      icon: Coin, color: '#67C23A', subText: q.max_cpu ? `使用率 ${Math.round((q.used_memory || 0) / (q.max_memory || 1) * 100)}%` : '不限'
    },
    {
      label: '虚拟机数量', used: `${q.used_vm || 0}`, max: q.max_vm || 0,
      maxText: q.max_vm ? `${q.max_vm} 台` : '不限', percent: q.max_vm ? Math.round((q.used_vm || 0) / q.max_vm * 100) : 0,
      icon: Monitor, color: '#F56C6C', subText: q.max_cpu ? `使用率 ${Math.round((q.used_vm || 0) / (q.max_vm || 1) * 100)}%` : '不限'
    },
    {
      label: '磁盘已用 / 配额', used: `${q.used_disk || 0}`, max: q.max_disk || 0,
      maxText: q.max_disk ? `${q.max_disk} GB` : '不限', percent: q.max_disk ? Math.round((q.used_disk || 0) / q.max_disk * 100) : 0,
      icon: FolderOpened, color: '#E6A23C', subText: q.max_cpu ? `使用率 ${Math.round((q.used_disk || 0) / (q.max_disk || 1) * 100)}%` : '不限'
    },
    {
      label: '本月运行时长', used: `${q.used_runtime_display || '0'}`, max: q.max_runtime_hours || 0,
      maxText: q.max_runtime_hours ? `${q.max_runtime_hours} 小时` : '不限', percent: q.max_runtime_hours ? Math.round((q.used_runtime_seconds || 0) / (q.max_runtime_hours * 3600) * 100) : 0,
      icon: Timer, color: '#9C6ADE', subText: q.runtime_quota_reached ? '已耗尽' : ''
    },
  ]
})

const quotaCategories = computed(() => {
  const q = quota.value
  return [
    {
      title: '计算与实例配额',
      icon: Cpu,
      iconColor: '#409EFF',
      summary: `核心 ${q.used_cpu || 0}/${q.max_cpu || '不限'} · 内存 ${q.used_memory || 0}/${q.max_memory || '不限'} · 实例 ${q.used_vm || 0}/${q.max_vm || '不限'}`,
      items: [
        { label: 'CPU 核心', desc: '已分配 / 总额度', max: q.max_cpu || 0, percent: q.max_cpu ? Math.round((q.used_cpu || 0) / q.max_cpu * 100) : 0, display: `${q.used_cpu || 0} / ${q.max_cpu || '不限'} 核`, icon: Cpu, color: '#409EFF' },
        { label: '内存', desc: '已分配 / 总额度', max: q.max_memory || 0, percent: q.max_memory ? Math.round((q.used_memory || 0) / q.max_memory * 100) : 0, display: `${q.used_memory || 0} GB / ${q.max_memory || '不限'} GB`, icon: Coin, color: '#67C23A' },
        { label: '虚拟机数量', max: q.max_vm || 0, percent: q.max_vm ? Math.round((q.used_vm || 0) / q.max_vm * 100) : 0, display: `${q.used_vm || 0} / ${q.max_vm || '不限'} 台`, icon: Monitor, color: '#F56C6C' },
        { label: '运行时长', desc: '本月累计', max: q.max_runtime_hours || 0, percent: q.max_runtime_hours ? Math.round((q.used_runtime_seconds || 0) / (q.max_runtime_hours * 3600) * 100) : 0, display: `${q.used_runtime_display || '0秒'} / ${q.max_runtime_hours ? q.max_runtime_hours + ' 小时' : '不限'}`, icon: Timer, color: '#909399' },
      ]
    },
    {
      title: '存储配额',
      icon: FolderOpened,
      iconColor: '#E6A23C',
      summary: `磁盘 ${q.used_disk || 0}/${q.max_disk || '不限'} GB · 快照 ${q.used_snapshots || 0}/${q.max_snapshots || '不限'}`,
      items: [
        { label: '磁盘容量', max: q.max_disk || 0, percent: q.max_disk ? Math.round((q.used_disk || 0) / q.max_disk * 100) : 0, display: `${q.used_disk || 0} GB / ${q.max_disk || '不限'} GB`, icon: FolderOpened, color: '#E6A23C' },
        { label: '快照数量', max: q.max_snapshots || 0, percent: q.max_snapshots ? Math.round((q.used_snapshots || 0) / q.max_snapshots * 100) : 0, display: `${q.used_snapshots || 0} / ${q.max_snapshots || '不限'}`, icon: Camera, color: '#9C6ADE' },
        { label: '存储空间', desc: '项目磁盘配额', max: q.max_storage || 0, percent: q.max_storage ? Math.round((parseFloat(q.used_storage_gb) || 0) / q.max_storage * 100) : 0, display: `${q.used_storage_gb || '0'} / ${q.max_storage || '不限'} GB`, icon: Files, color: '#E6A23C' },
      ]
    },
    {
      title: '网络资源配额',
      icon: Connection,
      iconColor: '#409EFF',
      summary: isLightweight.value 
        ? `带宽 ${q.max_bandwidth_down ? q.max_bandwidth_down + ' Mbps' : '不限'} · 流量 ${q.used_traffic_down_gb || '0'}/${q.max_traffic_down || '不限'} GB`
        : `带宽 ${q.max_bandwidth_down ? q.max_bandwidth_down + ' Mbps' : '不限'} · IP ${q.used_public_ips || 0}/${q.max_public_ips || '不限'}`,
      cloudTypeTabs: true,
      items: [],
      cloudItems: {
        elastic: [
          { label: '下行带宽', desc: '经由 VPC 交换机限速', max: q.max_bandwidth_down || 0, percent: 0, display: q.max_bandwidth_down ? `${q.max_bandwidth_down} Mbps` : '不限', icon: Download, color: '#409EFF' },
          { label: '上行带宽', desc: '经由 VPC 交换机限速', max: q.max_bandwidth_up || 0, percent: 0, display: q.max_bandwidth_up ? `${q.max_bandwidth_up} Mbps` : '不限', icon: Upload, color: '#67C23A' },
          { label: '下行流量 (日)', desc: 'VPC 交换机出口统计', max: q.max_traffic_down || 0, percent: q.max_traffic_down ? Math.round((q.used_traffic_down || 0) / (q.max_traffic_down * 1073741824) * 100) : 0, display: `${q.used_traffic_down_gb || '0'} / ${q.max_traffic_down || '不限'} GB`, icon: Download, color: '#E6A23C' },
          { label: '上行流量 (日)', desc: 'VPC 交换机入口统计', max: q.max_traffic_up || 0, percent: q.max_traffic_up ? Math.round((q.used_traffic_up || 0) / (q.max_traffic_up * 1073741824) * 100) : 0, display: `${q.used_traffic_up_gb || '0'} / ${q.max_traffic_up || '不限'} GB`, icon: Upload, color: '#F56C6C' },
          { label: '公网 IPv4', desc: '绑定至 VPC 弹性网卡', max: q.max_public_ips || 0, percent: q.max_public_ips ? Math.round((q.used_public_ips || 0) / q.max_public_ips * 100) : 0, display: `${q.used_public_ips || 0} / ${q.max_public_ips || '不限'}`, icon: Link, color: '#9C6ADE', highlight: true },
          { label: '端口转发规则', desc: q.enable_port_forward ? 'VPC NAT 网关规则' : '未启用', max: q.enable_port_forward ? (q.max_port_forwards || 0) : 0, percent: q.max_port_forwards ? Math.round((q.used_port_forwards || 0) / q.max_port_forwards * 100) : 0, display: q.enable_port_forward ? `${q.used_port_forwards || 0} / ${q.max_port_forwards || '不限'}` : '未启用', icon: Connection, color: '#9C6ADE' },
        ],
        lightweight: [
          { label: '下行带宽', desc: '宿主机网桥直接限速', max: q.max_bandwidth_down || 0, percent: 0, display: q.max_bandwidth_down ? `${q.max_bandwidth_down} Mbps` : '不限', icon: Download, color: '#409EFF' },
          { label: '上行带宽', desc: '宿主机网桥直接限速', max: q.max_bandwidth_up || 0, percent: 0, display: q.max_bandwidth_up ? `${q.max_bandwidth_up} Mbps` : '不限', icon: Upload, color: '#67C23A' },
          { label: '下行流量 (日)', desc: '网桥接口统计', max: q.max_traffic_down || 0, percent: q.max_traffic_down ? Math.round((q.used_traffic_down || 0) / (q.max_traffic_down * 1073741824) * 100) : 0, display: `${q.used_traffic_down_gb || '0'} / ${q.max_traffic_down || '不限'} GB`, icon: Download, color: '#409EFF' },
          { label: '上行流量 (日)', desc: '网桥接口统计', max: q.max_traffic_up || 0, percent: q.max_traffic_up ? Math.round((q.used_traffic_up || 0) / (q.max_traffic_up * 1073741824) * 100) : 0, display: `${q.used_traffic_up_gb || '0'} / ${q.max_traffic_up || '不限'} GB`, icon: Upload, color: '#67C23A' },
          { label: '端口转发规则', desc: q.enable_port_forward ? 'iptables DNAT 规则' : '未启用', max: q.enable_port_forward ? (q.max_port_forwards || 0) : 0, percent: q.max_port_forwards ? Math.round((q.used_port_forwards || 0) / q.max_port_forwards * 100) : 0, display: q.enable_port_forward ? `${q.used_port_forwards || 0} / ${q.max_port_forwards || '不限'}` : '未启用', icon: Connection, color: '#9C6ADE' },
        ]
      }
    },
  ]
})

// ========== 辅助函数 ==========
const getProgressColor = (ratio) => {
  if (ratio >= 0.9) return '#F56C6C'
  if (ratio >= 0.7) return '#E6A23C'
  return '#409EFF'
}

const getDiskPercent = (row) => {
  if (!row.total_kb || row.total_kb === 0) return 0
  return Math.round((row.used_kb || 0) / row.total_kb * 100)
}

const getDiskProgressColor = (row) => {
  const pct = getDiskPercent(row)
  if (pct >= 90) return '#F56C6C'
  if (pct >= 70) return '#E6A23C'
  return '#409EFF'
}

const formatKB = (kb) => {
  const n = parseInt(kb) || 0
  if (n >= 1024 * 1024 * 1024) return (n / 1024 / 1024 / 1024).toFixed(2) + ' TB'
  if (n >= 1024 * 1024) return (n / 1024 / 1024).toFixed(2) + ' GB'
  if (n >= 1024) return (n / 1024).toFixed(1) + ' MB'
  return n + ' KB'
}

const onCloudTypeChange = () => {
  // 切换云类型时自动刷新 summary
}

// ========== 数据获取 ==========
const fetchQuota = async () => {
  try {
    const res = await getSelfQuota()
    quota.value = res.data || {}
  } catch (err) {
    console.error('获取配额失败:', err)
  }
}

const fetchVMs = async () => {
  vmLoading.value = true
  try {
    if (isAdmin.value) {
      const res = await getVmList()
      myVMs.value = res.data || []
    } else {
      const res = await getSelfVMs()
      myVMs.value = res.data || []
    }
  } catch (err) {
    console.error('获取VM列表失败:', err)
  } finally {
    vmLoading.value = false
  }
}

const fetchHostStatsData = async () => {
  hostLoading.value = true
  try {
    const res = await getHostStats()
    hostData.value = res.data || {}
  } catch (err) {
    console.error('获取宿主机信息失败:', err)
  } finally {
    hostLoading.value = false
  }
}

const fetchHostDisksData = async () => {
  diskLoading.value = true
  try {
    const res = await getHostDisks()
    hostDisks.value = res.data || []
  } catch (err) {
    console.error('获取磁盘列表失败:', err)
  } finally {
    diskLoading.value = false
  }
}

// ========== 环形图 ==========
let ringInited = false
let ringInitPending = false

const createRingOption = (name, value, color, subText) => ({
  series: [{
    type: 'gauge',
    startAngle: 210,
    endAngle: -30,
    center: ['50%', '55%'],
    radius: '85%',
    min: 0,
    max: 100,
    animation: true,
    animationDuration: 600,
    animationEasing: 'cubicInOut',
    pointer: { show: false },
    progress: {
      show: true,
      overlap: false,
      roundCap: true,
      clip: false,
      itemStyle: { color }
    },
    axisLine: { lineStyle: { width: 14, color: [[1, '#e5e7eb']] } },
    axisTick: { show: false },
    splitLine: { show: false },
    axisLabel: { show: false },
    detail: {
      width: 60,
      height: 18,
      fontSize: 22,
      fontWeight: 'bold',
      color: color,
      formatter: '{value}%',
      offsetCenter: [0, '-6%']
    },
    title: {
      color: '#999',
      fontSize: 12,
      offsetCenter: [0, '22%']
    },
    data: [{ value, name: subText }]
  }]
})

const renderRingCharts = () => {
  if (!isAdmin.value) return

  const cpuVal = Math.round(parseFloat(hostData.value.cpu_percent) || 0)
  const memVal = hostData.value.mem_total ? Math.round((hostData.value.mem_used / hostData.value.mem_total) * 100) : 0
  const swapVal = hostData.value.swap_total ? Math.round((hostData.value.swap_used / hostData.value.swap_total) * 100) : 0

  if (!ringInited) {
    if (ringInitPending || !cpuRingRef.value) return
    ringInitPending = true
    nextTick(() => {
      if (!cpuRingRef.value) { ringInitPending = false; return }
      ringInited = true
      ringInitPending = false

      const c = echarts.init(cpuRingRef.value)
      c.setOption(createRingOption('CPU', cpuVal, '#409EFF', `${hostData.value.cpu_count || 0} 核`))
      ringCharts.push(c)

      const memGB = ((hostData.value.mem_used || 0) / 1024 / 1024).toFixed(1)
      const m = echarts.init(memRingRef.value)
      m.setOption(createRingOption('内存', memVal, '#67C23A', `${memGB} / ${((hostData.value.mem_total || 0) / 1024 / 1024).toFixed(0)} GB`))
      ringCharts.push(m)

      const swapGB = ((hostData.value.swap_used || 0) / 1024 / 1024).toFixed(1)
      const s = echarts.init(swapRingRef.value)
      s.setOption(createRingOption('Swap', swapVal, '#E6A23C', `${swapGB} / ${((hostData.value.swap_total || 0) / 1024 / 1024).toFixed(0)} GB`))
      ringCharts.push(s)
    })
    return
  }

  if (ringCharts[0]) {
    ringCharts[0].setOption(createRingOption('CPU', cpuVal, '#409EFF', `${hostData.value.cpu_count || 0} 核`))
  }
  if (ringCharts[1]) {
    const memGB = ((hostData.value.mem_used || 0) / 1024 / 1024).toFixed(1)
    ringCharts[1].setOption(createRingOption('内存', memVal, '#67C23A', `${memGB} / ${((hostData.value.mem_total || 0) / 1024 / 1024).toFixed(0)} GB`))
  }
  if (ringCharts[2]) {
    const swapGB = ((hostData.value.swap_used || 0) / 1024 / 1024).toFixed(1)
    ringCharts[2].setOption(createRingOption('Swap', swapVal, '#E6A23C', `${swapGB} / ${((hostData.value.swap_total || 0) / 1024 / 1024).toFixed(0)} GB`))
  }
}

watch(hostData, () => {
  renderRingCharts()
}, { deep: true })

// ========== SSE 实时推送（管理员） ==========
const startHostStatsSSE = () => {
  if (!isAdmin.value) return
  stopHostStatsSSE()
  const token = userStore.token
  if (!token) return
  hostStatsSSE = createHostStatsSSE(token)
  hostStatsSSE.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      if (data && typeof data.cpu_percent !== 'undefined') {
        hostData.value = data
      }
    } catch (e) {
      // ignore parse errors
    }
  }
  hostStatsSSE.onerror = () => {
    stopHostStatsSSE()
    // SSE 断开后 10s 重连
    setTimeout(startHostStatsSSE, 10000)
  }
}

const stopHostStatsSSE = () => {
  if (hostStatsSSE) {
    hostStatsSSE.close()
    hostStatsSSE = null
  }
}

// ========== 初始化 ==========
onMounted(() => {
  if (isAdmin.value) {
    fetchHostStatsData()
    fetchHostDisksData()
    startHostStatsSSE()
  } else {
    fetchQuota()
  }
  fetchVMs()
})

onUnmounted(() => {
  stopHostStatsSSE()
  ringCharts.forEach(c => c.dispose())
  ringCharts = []
  ringInited = false
})
</script>

<style scoped>
.dashboard-container {
  padding: 10px;
  background-color: var(--el-bg-color-page);
  min-height: calc(100vh - 84px);
}

/* ===== 安全警告横幅 ===== */
.security-warning-banner {
  margin-bottom: 20px;
  border-radius: 8px;
}
.security-warning-banner .banner-content {
  display: flex;
  align-items: center;
  gap: 8px;
}
.security-warning-banner .el-link {
  display: inline;
  font-weight: 600;
}

.section {
  margin-bottom: 28px;
}
.section-title {
  margin-top: 0;
  margin-bottom: 16px;
  font-size: 18px;
  font-weight: 700;
  color: var(--el-text-color-primary);
  padding-left: 12px;
  border-left: 4px solid var(--el-color-primary);
}

/* ===== 管理员环形图 ===== */
.monitor-panel {
  margin-bottom: 0;
}
.ring-card {
  border-radius: 12px;
  border: none;
  text-align: center;
  transition: transform .2s, box-shadow .2s;
}
.ring-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}
.ring-label-top {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-regular);
  margin-bottom: 4px;
}
.ring-chart {
  width: 100%;
  height: 150px;
}
.io-delay-wrap {
  padding: 16px 0;
}
.io-delay-value {
  font-size: 36px;
  font-weight: 800;
}
.io-delay-unit {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

/* ===== 管理员概览卡片 ===== */
.overview-card {
  border-radius: 12px;
  border: none;
  overflow: hidden;
  padding: 0;
  transition: transform .2s, box-shadow .2s;
}
.overview-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}
.overview-card :deep(.el-card__body) { padding: 0; }
.overview-accent {
  height: 3px;
}
.overview-body {
  padding: 16px 20px;
}
.overview-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.overview-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  font-weight: 500;
}
.overview-value {
  font-size: 22px;
  font-weight: 800;
  color: var(--el-text-color-primary);
}
.overview-sub {
  font-size: 12px;
  color: #9ca3af;
  margin-top: 2px;
}

/* ===== 磁盘表格 ===== */
.disk-bar-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}
.disk-pct-text {
  font-size: 13px;
  font-weight: 700;
  min-width: 36px;
  text-align: right;
}
.disk-details {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

/* ===== 用户紧凑卡片 5列 ===== */
.compact-card {
  border-radius: 12px;
  border: none;
  overflow: hidden;
  padding: 0;
  transition: transform .2s, box-shadow .2s;
  margin-bottom: 16px;
}
.compact-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}
.compact-card :deep(.el-card__body) { padding: 0; }
.compact-accent {
  height: 3px;
}
.compact-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 16px 0;
}
.compact-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  font-weight: 500;
}
.compact-value {
  font-size: 20px;
  font-weight: 800;
  padding: 6px 16px 0;
  color: var(--el-text-color-primary);
}
.compact-unit {
  font-size: 13px;
  color: #9ca3af;
  font-weight: normal;
}
.compact-sub {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  padding: 6px 16px 14px;
}
.compact-card :deep(.el-progress) {
  padding: 0 16px 14px;
}
.compact-card :deep(.el-progress-bar__outer) {
  background: #e5e7eb;
}

/* ===== 配额分类折叠 ===== */
.quota-category-card {
  border-radius: 12px;
  border: none;
  margin-bottom: 16px;
  overflow: hidden;
}
.quota-category-card :deep(.el-card__body) { padding: 0; }
.quota-cat-header {
  display: flex;
  align-items: center;
  padding: 14px 20px;
  cursor: pointer;
  user-select: none;
  transition: background .2s;
}
.quota-cat-header:hover {
  background: var(--el-fill-color-light);
}
.cat-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.cat-title-text {
  font-size: 16px;
  font-weight: 700;
}
.cat-summary {
  flex: 1;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-left: 24px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cat-chevron {
  transition: transform .25s;
  color: #9ca3af;
  flex-shrink: 0;
}
.cat-chevron.rotated {
  transform: rotate(180deg);
}
.quota-cat-body {
  padding: 0 20px 18px;
}

.quota-item-row {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  gap: 12px;
  border-radius: 8px;
  transition: background .15s;
}
.quota-item-row:hover {
  background: var(--el-fill-color-light);
}
.quota-item-row.highlight {
  background: var(--el-color-primary-light-9);
}
.quota-item-icon {
  flex-shrink: 0;
  width: 28px;
  text-align: center;
}
.quota-item-info {
  flex: 1;
  min-width: 80px;
}
.quota-item-label {
  font-size: 14px;
  font-weight: 500;
}
.quota-item-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.quota-item-progress {
  flex: 2;
  min-width: 100px;
  max-width: 200px;
}
.quota-item-value {
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
  text-align: right;
  min-width: 100px;
}

.cloud-type-tabs {
  padding: 4px 12px;
}
.cloud-source-tag {
  padding: 4px 12px 8px;
}

/* ===== VM 折叠面板 ===== */
.vm-section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}
.custom-collapse {
  border-top: none;
  border-bottom: none;
  min-height: 100px;
}
.vm-collapse-item {
  margin-bottom: 15px;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.05);
}

:deep(.el-collapse-item__header) {
  border-bottom: none;
  padding: 10px 20px;
  height: auto;
  min-height: 60px;
  line-height: inherit;
  background-color: var(--el-bg-color-overlay);
  transition: background-color 0.3s;
}
:deep(.el-collapse-item__header:hover) {
  background-color: var(--el-fill-color-light);
}
:deep(.el-collapse-item__wrap) {
  border-bottom: none;
  background-color: var(--el-bg-color-overlay);
}
:deep(.el-collapse-item__content) {
  padding: 0 20px 20px 20px;
}

.vm-collapse-title {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 20px;
}
.vm-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  min-width: 150px;
}
.vm-status { min-width: 80px; }
.vm-meta {
  display: flex;
  align-items: center;
  gap: 20px;
  color: var(--el-text-color-regular);
  font-size: 14px;
  margin-left: auto;
  margin-right: 20px;
}
.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

/* ===== 响应式 ===== */
@media (max-width: 900px) {
  .stat-grid-cols5 .el-col { flex: 0 0 50%; max-width: 50%; }
}
@media (max-width: 768px) {
  .dashboard-container {
    padding: 4px;
  }

  .section {
    margin-bottom: 20px;
  }

  .section-title {
    font-size: 15px;
    margin-bottom: 10px;
    padding-left: 8px;
  }

  .vm-section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .vm-collapse-title {
    flex-wrap: wrap;
    gap: 8px;
  }

  .vm-name {
    font-size: 14px;
    min-width: auto;
  }

  .vm-meta {
    margin-left: 0;
    margin-right: 0;
    width: 100%;
    gap: 12px;
    font-size: 12px;
    flex-wrap: wrap;
  }

  .quota-item-row {
    flex-wrap: wrap;
    gap: 6px;
    padding: 8px 8px;
  }

  .quota-item-progress {
    flex: 1 1 100%;
    max-width: none;
    order: 3;
  }

  .quota-item-value {
    min-width: auto;
    order: 2;
  }

  .quota-cat-header {
    padding: 10px 14px;
  }

  .cat-title-text {
    font-size: 14px;
  }

  .cat-summary {
    display: none;
  }

  .quota-cat-body {
    padding: 0 12px 14px;
  }

  .disk-bar-cell {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .disk-pct-text {
    text-align: left;
  }
}
@media (max-width: 500px) {
  .stat-grid-cols5 .el-col { flex: 0 0 100%; max-width: 100%; }
}
</style>
