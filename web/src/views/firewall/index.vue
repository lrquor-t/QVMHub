<template>
  <div class="firewall-page">
    <div class="page-header">
      <div class="page-header-left">
        <div class="page-title-row">
          <el-icon class="page-icon"><Lock /></el-icon>
          <h2>防火墙</h2>
        </div>
        <p>管理宿主机入站防火墙、KVM 转发流量防火墙与当前连接清理</p>
      </div>
      <div class="page-header-right">
        <el-button size="large" :icon="Refresh" @click="loadAll" :loading="loading">刷新</el-button>
      </div>
    </div>

    <div class="tabs-wrapper">
      <div class="custom-tabs">
        <div
          :class="['custom-tab', { active: activeTab === 'host' }]"
          @click="switchTab('host')"
        >
          <el-icon><Monitor /></el-icon>
          <span>宿主机防火墙</span>
        </div>
        <div
          :class="['custom-tab', { active: activeTab === 'kvm' }]"
          @click="switchTab('kvm')"
        >
          <el-icon><Connection /></el-icon>
          <span>KVM 网络防火墙</span>
        </div>
        <div
          :class="['custom-tab', { active: activeTab === 'connections' }]"
          @click="switchTab('connections')"
        >
          <el-icon><Link /></el-icon>
          <span>连接管理</span>
        </div>
      </div>

      <div class="tab-content">
        <!-- 宿主机防火墙 -->
        <div v-show="activeTab === 'host'" class="tab-pane">
          <div class="status-banner" :class="hostStatus?.active ? 'enabled' : 'disabled'">
            <div class="status-banner-icon">
              <el-icon :size="24"><CircleCheckFilled v-if="hostStatus?.active" /><WarningFilled v-else /></el-icon>
            </div>
            <div class="status-banner-body">
              <div class="status-banner-title">{{ hostStatus?.active ? '宿主机防火墙已启用' : '宿主机防火墙已关闭' }}</div>
              <div class="status-banner-desc">{{ hostStatus?.active ? '防火墙规则正在保护宿主机入站流量' : '端口转发仍会写入 UFW 持久放通规则' }}</div>
            </div>
            <div class="status-banner-actions">
              <el-button type="success" :icon="SwitchButton" @click="openEnableHostFirewall" :loading="hostApplying">开启防火墙</el-button>
              <el-button type="warning" :icon="Close" @click="handleDisableHostFirewall">关闭防火墙</el-button>
            </div>
          </div>

          <el-row :gutter="20">
            <el-col :span="8">
              <div class="info-card">
                <div class="info-card-header">
                  <el-icon><Setting /></el-icon>
                  <span>运行状态</span>
                </div>
                <div class="info-list">
                  <div class="info-item">
                    <span class="info-label">UFW</span>
                    <el-tag :type="hostStatus?.ufw_available ? 'success' : 'danger'" size="small" effect="light">{{ hostStatus?.ufw_available ? '可用' : '不可用' }}</el-tag>
                  </div>
                  <div class="info-item">
                    <span class="info-label">入站默认</span>
                    <el-tag :type="hostStatus?.default_incoming === 'allow' ? 'danger' : 'success'" size="small" effect="light">{{ hostStatus?.default_incoming || '-' }}</el-tag>
                  </div>
                  <div class="info-item">
                    <span class="info-label">出站默认</span>
                    <el-tag :type="hostStatus?.default_outgoing === 'allow' ? 'success' : 'info'" size="small" effect="light">{{ hostStatus?.default_outgoing || '-' }}</el-tag>
                  </div>
                  <div class="info-item">
                    <span class="info-label">转发默认</span>
                    <el-tag :type="hostStatus?.default_routed === 'allow' ? 'warning' : 'success'" size="small" effect="light">{{ hostStatus?.default_routed || '-' }}</el-tag>
                  </div>
                  <div class="info-item">
                    <span class="info-label">SSH 端口</span>
                    <span class="code">{{ (hostStatus?.ssh_ports || []).join(', ') || '-' }}</span>
                  </div>
                  <div class="info-item">
                    <span class="info-label">面板端口</span>
                    <span class="code">{{ (hostStatus?.panel_ports || []).join(', ') || '-' }}</span>
                  </div>
                </div>
                <div v-if="hostStatus?.docker_compatibility" class="info-card-footer">
                  <el-icon><InfoFilled /></el-icon>
                  <span>{{ hostStatus.docker_compatibility }}</span>
                </div>
              </div>
            </el-col>

            <el-col :span="16">
              <div class="info-card table-card">
                <div class="info-card-header">
                  <el-icon><List /></el-icon>
                  <span>宿主机规则</span>
                  <el-tag size="small" effect="plain" round>{{ filteredHostRules.length }} 条</el-tag>
                  <div class="header-actions">
                    <el-button size="small" :icon="VideoCamera" @click="handleAddVNCDefault">添加 VNC 5900-5999</el-button>
                    <el-button size="small" type="primary" :icon="Plus" @click="openRuleDialog()">添加规则</el-button>
                  </div>
                </div>
                <div class="filter-bar">
                  <el-input
                    v-model="fwRuleSearchText"
                    placeholder="搜索端口"
                    clearable
                    :prefix-icon="Search"
                    size="small"
                    style="width: 130px;"
                  />
                  <el-select
                    v-model="fwProtocolFilter"
                    placeholder="协议筛选"
                    clearable
                    size="small"
                    style="width: 120px;"
                  >
                    <el-option label="TCP" value="tcp" />
                    <el-option label="UDP" value="udp" />
                  </el-select>
                  <el-select
                    v-model="fwActionFilter"
                    placeholder="动作筛选"
                    clearable
                    size="small"
                    style="width: 120px;"
                  >
                    <el-option label="允许" value="allow" />
                    <el-option label="拒绝" value="deny" />
                  </el-select>
                  <el-input
                    v-model="fwRemarkSearch"
                    placeholder="搜索备注"
                    clearable
                    :prefix-icon="Search"
                    size="small"
                    style="width: 150px;"
                  />
                </div>
                <el-table :data="filteredHostRules" size="small" border style="width: 100%;" max-height="500">
                  <el-table-column label="动作" width="90" align="center">
                    <template #default="{ row }">
                      <el-tag :type="row.action === 'allow' ? 'success' : 'danger'" size="small" effect="light" round>
                        {{ row.action === 'allow' ? '允许' : '拒绝' }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="协议" width="90" align="center">
                    <template #default="{ row }">
                      <span class="code">{{ row.protocol?.toUpperCase() }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="端口" width="130" align="center">
                    <template #default="{ row }">
                      <span class="code">{{ formatRulePort(row) }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="来源" min-width="160">
                    <template #default="{ row }">
                      <span class="code">{{ row.source_cidr || 'any' }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column prop="comment" label="备注" min-width="200" show-overflow-tooltip />
                  <el-table-column label="状态" width="150" align="center">
                    <template #default="{ row }">
                      <el-tag v-if="row.protected" type="danger" size="small" effect="light">{{ row.protected_reason || '保护规则' }}</el-tag>
                      <el-tag v-else-if="row.managed_by_panel" type="info" size="small" effect="light">面板管理</el-tag>
                      <span v-else class="text-muted">—</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="150" fixed="right" align="center">
                    <template #default="{ row }">
                      <el-button size="small" plain :disabled="row.protected" @click="openRuleDialog(row)">编辑</el-button>
                      <el-button size="small" type="danger" plain :disabled="row.protected" @click="handleDeleteHostRule(row)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <el-empty v-if="!loading && hostRules.length === 0" description="暂无防火墙规则" :image-size="80" />
              </div>
            </el-col>
          </el-row>
        </div>

        <!-- KVM 网络防火墙 -->
        <div v-show="activeTab === 'kvm'" class="tab-pane">
          <div class="kvm-actions-bar">
            <div>
              <h3>KVM 网络防火墙</h3>
              <p>仅管控 KVM 虚拟机 IPv4 转发流量，不影响宿主机和 Docker 容器网络。</p>
            </div>
            <div class="kvm-actions-buttons">
              <el-button :icon="View" @click="handlePreview">预览规则</el-button>
              <el-button type="primary" :icon="DocumentChecked" @click="handleSave" :loading="saving">保存策略</el-button>
              <el-button type="success" :icon="SwitchButton" @click="handleApply" :loading="applying">应用规则</el-button>
              <el-button type="warning" :icon="Close" @click="handleDisable">禁用</el-button>
              <el-button type="danger" :icon="RefreshLeft" @click="handleRollback">回滚</el-button>
            </div>
          </div>

          <div class="status-banner" :class="status?.active ? 'enabled' : 'info'">
            <div class="status-banner-icon">
              <el-icon :size="24"><CircleCheckFilled v-if="status?.active" /><InfoFilled v-else /></el-icon>
            </div>
            <div class="status-banner-body">
              <div class="status-banner-title">{{ status?.active ? '规则已生效' : '规则未应用' }}</div>
              <div class="status-banner-desc">{{ status?.active ? 'KVM 网络防火墙规则已在 nftables 中生效' : '当前仅保存或预览策略，尚未应用到 nftables' }}</div>
            </div>
          </div>

          <el-row :gutter="20">
            <el-col :span="14">
              <div class="info-card">
                <div class="info-card-header">
                  <el-icon><Setting /></el-icon>
                  <span>全局策略</span>
                </div>
                <div class="info-card-body-padded">
                  <el-form label-width="130px" label-position="left">
                    <el-form-item label="虚拟网桥">
                      <el-input v-model="policy.bridge" placeholder="br-ovs" />
                    </el-form-item>
                    <el-form-item label="虚拟机网段">
                      <el-input v-model="policy.vm_subnet" placeholder="192.168.122.0/24" />
                    </el-form-item>
                    <el-divider content-position="left">区域限制</el-divider>
                    <el-form-item label="出站区域限制">
                      <el-switch v-model="policy.outbound_enabled" active-text="启用" inactive-text="关闭" />
                    </el-form-item>
                    <el-form-item v-if="policy.outbound_enabled" label="允许出站区域">
                      <el-select v-model="policy.outbound_allowed_regions" multiple filterable style="width:100%;" placeholder="选择允许访问的目标区域">
                        <el-option v-for="item in regionOptions" :key="item.value" :label="item.label" :value="item.value" />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="入站区域限制">
                      <el-switch v-model="policy.inbound_enabled" active-text="启用" inactive-text="关闭" />
                    </el-form-item>
                    <el-form-item v-if="policy.inbound_enabled" label="允许入站区域">
                      <el-select v-model="policy.inbound_allowed_regions" multiple filterable style="width:100%;" placeholder="选择允许访问端口转发的来源区域">
                        <el-option v-for="item in regionOptions" :key="item.value" :label="item.label" :value="item.value" />
                      </el-select>
                    </el-form-item>
                    <el-divider content-position="left">高级设置</el-divider>
                    <el-form-item label="禁用 VM IPv6">
                      <el-switch v-model="policy.disable_vm_ipv6" active-text="启用" inactive-text="关闭" />
                    </el-form-item>
                    <el-form-item label="拦截动作">
                      <el-radio-group v-model="policy.block_action">
                        <el-radio-button label="reject">reject</el-radio-button>
                        <el-radio-button label="drop">drop</el-radio-button>
                      </el-radio-group>
                    </el-form-item>
                    <el-form-item label="白名单 CIDR">
                      <el-input v-model="whitelistText" type="textarea" :rows="5" placeholder="每行一个 IPv4 CIDR 或 IP，例如 203.0.113.10/32" />
                    </el-form-item>
                  </el-form>
                </div>
              </div>
            </el-col>

            <el-col :span="10">
              <div class="info-card">
                <div class="info-card-header">
                  <el-icon><DataAnalysis /></el-icon>
                  <span>区域数据</span>
                  <div class="header-actions">
                    <el-button size="small" :icon="Upload" @click="importVisible = true">本地导入</el-button>
                    <el-button size="small" type="primary" :icon="Download" @click="handleGeoUpdate">在线更新</el-button>
                  </div>
                </div>
                <div class="info-card-body-padded">
                  <el-form label-width="110px" label-position="left">
                    <el-form-item label="下载源">
                      <el-input v-model="policy.geoip_base_url" placeholder="https://www.ipdeny.com/ipblocks/data/aggregated" />
                    </el-form-item>
                    <el-form-item label="更新区域代码">
                      <el-input v-model="geoCodesText" placeholder="如 cn,us,jp" />
                    </el-form-item>
                  </el-form>
                </div>
                <el-table :data="policy.regions" size="small" border style="width: 100%;" max-height="280">
                  <el-table-column prop="code" label="代码" width="80" align="center">
                    <template #default="{ row }">
                      <el-tag effect="plain" size="small" round>{{ row.code }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="名称" min-width="120">
                    <template #default="{ row }">{{ row.name || row.code }}</template>
                  </el-table-column>
                  <el-table-column label="CIDR 数" width="90" align="center">
                    <template #default="{ row }">
                      <el-tag effect="plain" size="small" round>{{ row.cidrs?.length || 0 }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column prop="updated_at" label="更新时间" min-width="160" />
                </el-table>
                <div v-if="status?.geoip_copyright" class="info-card-footer">
                  <el-icon><InfoFilled /></el-icon>
                  <span>{{ status.geoip_copyright }}</span>
                </div>
              </div>
            </el-col>
          </el-row>

          <div class="info-card table-card" style="margin-top: 20px;">
            <div class="info-card-header">
              <el-icon><Monitor /></el-icon>
              <span>VM 覆盖策略</span>
              <el-tag size="small" effect="plain" round>{{ vmRows.length }} 台</el-tag>
            </div>
            <el-table :data="vmRows" border style="width: 100%;" max-height="400">
              <el-table-column prop="name" label="虚拟机" min-width="180">
                <template #default="{ row }">
                  <div class="vm-name-cell">
                    <el-icon :size="16"><Monitor /></el-icon>
                    <span>{{ row.name }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="管控模式" width="180" align="center">
                <template #default="{ row }">
                  <el-select v-model="policy.vm_overrides[row.name].mode" size="small" style="width: 140px;">
                    <el-option label="继承全局" value="inherit" />
                    <el-option label="关闭管控" value="disabled" />
                    <el-option label="仅允许入站" value="inbound_only" />
                    <el-option label="仅允许区域" value="allow" />
                    <el-option label="阻断区域" value="block" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="限制区域" min-width="280">
                <template #default="{ row }">
                  <el-select
                    v-model="policy.vm_overrides[row.name].regions"
                    multiple
                    filterable
                    size="small"
                    style="width:100%;"
                    :disabled="!['allow', 'block'].includes(policy.vm_overrides[row.name].mode)"
                    placeholder="选择区域"
                  >
                    <el-option v-for="item in regionOptions" :key="item.value" :label="item.label" :value="item.value" />
                  </el-select>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-if="vmRows.length === 0" description="暂无虚拟机" :image-size="80" />
          </div>
        </div>

        <!-- 连接管理 -->
        <div v-show="activeTab === 'connections'" class="tab-pane">
          <div class="status-banner warning">
            <div class="status-banner-icon">
              <el-icon :size="24"><WarningFilled /></el-icon>
            </div>
            <div class="status-banner-body">
              <div class="status-banner-title">高风险操作</div>
              <div class="status-banner-desc">关闭全部连接会断开 SSH 和面板连接，当前会话可能立即断开</div>
            </div>
          </div>

          <div class="info-card">
            <div class="info-card-header">
              <el-icon><Link /></el-icon>
              <span>连接控制</span>
            </div>
            <div class="actions-panel">
              <div class="action-group">
                <div class="action-group-label">非防火墙端口</div>
                <div class="action-group-desc">关闭本地端口不在 UFW 允许规则内的 TCP 已建立连接</div>
                <div class="action-group-buttons">
                  <el-button :icon="View" @click="previewConnections('non_firewall')">预览连接</el-button>
                  <el-button type="warning" :icon="Close" @click="confirmCloseConnections('non_firewall')">关闭连接</el-button>
                </div>
              </div>
              <el-divider direction="vertical" class="action-divider" />
              <div class="action-group">
                <div class="action-group-label">全部连接</div>
                <div class="action-group-desc">关闭所有 TCP 已建立连接，包括 SSH 和面板</div>
                <div class="action-group-buttons">
                  <el-button :icon="View" @click="previewConnections('all')">预览连接</el-button>
                  <el-button type="danger" :icon="CircleCloseFilled" @click="confirmCloseConnections('all')">关闭全部</el-button>
                </div>
              </div>
            </div>
          </div>

          <div v-if="connectionPreview" class="info-card table-card" style="margin-top: 20px;">
            <div class="info-card-header">
              <el-icon><List /></el-icon>
              <span>连接预览</span>
              <el-tag size="small" effect="plain" round>{{ connectionPreview?.connections?.length || 0 }} 个连接</el-tag>
            </div>
            <el-table :data="connectionPreview?.connections || []" border size="small" style="width: 100%;" max-height="400">
              <el-table-column label="协议" width="80" align="center">
                <template #default="{ row }">
                  <el-tag effect="plain" size="small" round>{{ row.protocol }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="本地地址" min-width="200">
                <template #default="{ row }">
                  <span class="code">{{ row.local_ip }}:{{ row.local_port }}</span>
                </template>
              </el-table-column>
              <el-table-column label="对端地址" min-width="200">
                <template #default="{ row }">
                  <span class="code">{{ row.peer_ip }}:{{ row.peer_port }}</span>
                </template>
              </el-table-column>
              <el-table-column label="防火墙端口" width="110" align="center">
                <template #default="{ row }">
                  <el-tag :type="row.allowed_port ? 'success' : 'warning'" size="small" effect="light">
                    {{ row.allowed_port ? '已放行' : '未放行' }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>
      </div>
    </div>

    <!-- 启用宿主机防火墙对话框 -->
    <el-dialog
      v-model="enableVisible"
      title="确认启用宿主机防火墙"
      width="960px"
      append-to-body
      destroy-on-close
      class="modern-dialog"
    >
      <div class="status-banner warning" style="margin-bottom: 16px;">
        <div class="status-banner-icon">
          <el-icon :size="20"><WarningFilled /></el-icon>
        </div>
        <div class="status-banner-body">
          <div class="status-banner-desc">请确认 SSH 和面板端口无误，启用后这两个端口对应规则不允许删除和编辑。</div>
        </div>
      </div>
      <el-table :data="enableRules" size="small" border>
        <el-table-column label="动作" width="100" align="center">
          <template #default="{ row }">
            <el-select v-model="row.action" size="small" :disabled="row.protected">
              <el-option label="允许" value="allow" />
              <el-option label="拒绝" value="deny" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="协议" width="100" align="center">
          <template #default="{ row }">
            <el-select v-model="row.protocol" size="small" :disabled="row.protected">
              <el-option label="TCP" value="tcp" />
              <el-option label="UDP" value="udp" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="起始端口" width="130" align="center">
          <template #default="{ row }">
            <el-input-number v-model="row.port_start" :min="1" :max="65535" size="small" :disabled="row.protected" />
          </template>
        </el-table-column>
        <el-table-column label="结束端口" width="130" align="center">
          <template #default="{ row }">
            <el-input-number v-model="row.port_end" :min="row.port_start || 1" :max="65535" size="small" :disabled="row.protected" />
          </template>
        </el-table-column>
        <el-table-column label="来源 CIDR" min-width="180">
          <template #default="{ row }">
            <el-input v-model="row.source_cidr" size="small" placeholder="any" :disabled="row.protected" />
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="180">
          <template #default="{ row }">
            <el-input v-model="row.comment" size="small" :disabled="row.protected" />
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="enableVisible = false" size="large">取消</el-button>
        <el-button type="primary" @click="handleEnableHostFirewall" :loading="hostApplying" size="large">确认启用</el-button>
      </template>
    </el-dialog>

    <!-- 宿主机规则对话框 -->
    <el-dialog
      v-model="ruleVisible"
      :title="editingRule?.id ? '编辑宿主机规则' : '添加宿主机规则'"
      width="560px"
      append-to-body
      destroy-on-close
      class="modern-dialog"
    >
      <el-form label-width="100px" label-position="left">
        <el-form-item label="动作">
          <el-radio-group v-model="ruleForm.action">
            <el-radio-button label="allow">允许</el-radio-button>
            <el-radio-button label="deny">拒绝</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="协议">
          <el-select v-model="ruleForm.protocol" style="width:100%;">
            <el-option label="TCP" value="tcp" />
            <el-option label="UDP" value="udp" />
            <el-option label="TCP + UDP" value="both" />
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="起始端口">
              <el-input-number v-model="ruleForm.port_start" :min="1" :max="65535" style="width:100%;" controls-position="right" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="结束端口">
              <el-input-number v-model="ruleForm.port_end" :min="ruleForm.port_start || 1" :max="65535" style="width:100%;" controls-position="right" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="来源 CIDR">
          <el-input v-model="ruleForm.source_cidr" placeholder="留空表示 any，例如 203.0.113.0/24" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="ruleForm.comment" placeholder="规则说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleVisible = false" size="large">取消</el-button>
        <el-button type="primary" @click="handleSaveHostRule" size="large">保存</el-button>
      </template>
    </el-dialog>

    <!-- 预览对话框 -->
    <el-dialog
      v-model="previewVisible"
      title="nftables 规则预览"
      width="820px"
      append-to-body
      destroy-on-close
      class="modern-dialog"
    >
      <div class="code-block-wrapper">
        <pre class="code-block"><code>{{ previewRules }}</code></pre>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false" size="large">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 导入区域对话框 -->
    <el-dialog
      v-model="importVisible"
      title="导入区域 CIDR"
      width="620px"
      append-to-body
      destroy-on-close
      class="modern-dialog"
    >
      <el-form label-width="100px" label-position="left">
        <el-form-item label="区域代码">
          <el-input v-model="importForm.code" placeholder="如 cn" />
        </el-form-item>
        <el-form-item label="区域名称">
          <el-input v-model="importForm.name" placeholder="如 中国大陆" />
        </el-form-item>
        <el-form-item label="来源说明">
          <el-input v-model="importForm.source" placeholder="local-import" />
        </el-form-item>
        <el-form-item label="CIDR 列表">
          <el-input v-model="importForm.cidrs" type="textarea" :rows="10" placeholder="每行一个 IPv4 CIDR" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importVisible = false" size="large">取消</el-button>
        <el-button type="primary" @click="handleImport" size="large">导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import {
  getFirewallStatus,
  saveFirewallPolicy,
  previewFirewallPolicy,
  applyFirewallPolicy,
  disableFirewall,
  rollbackFirewall,
  importFirewallRegion,
  updateFirewallGeoIP,
  getHostFirewallStatus,
  previewEnableHostFirewall,
  enableHostFirewall,
  disableHostFirewall,
  createHostFirewallRule,
  updateHostFirewallRule,
  deleteHostFirewallRule,
  addHostFirewallVNCDefaultRule,
  previewHostFirewallConnections,
  closeHostFirewallConnections
} from '@/api/firewall'
import {
  Lock, Monitor, Connection, Link, Setting, List, DataAnalysis,
  View, Refresh, Plus, Upload, Download, DocumentChecked,
  SwitchButton, Close, CircleCheckFilled, WarningFilled, InfoFilled,
  RefreshLeft, VideoCamera, CircleCloseFilled
} from '@element-plus/icons-vue'

const activeTab = ref('host')
const loading = ref(false)
const saving = ref(false)
const applying = ref(false)
const hostApplying = ref(false)
const status = ref(null)
const hostStatus = ref(null)
const fwRuleSearchText = ref('')
const fwProtocolFilter = ref('')
const fwActionFilter = ref('')
const fwRemarkSearch = ref('')
const whitelistText = ref('')
const geoCodesText = ref('cn')
const previewVisible = ref(false)
const previewRules = ref('')
const importVisible = ref(false)
const enableVisible = ref(false)
const ruleVisible = ref(false)
const editingRule = ref(null)
const enableRules = ref([])
const connectionPreview = ref(null)

function createDefaultPolicy() {
  return {
    bridge: 'br-ovs',
    vm_subnet: '192.168.122.0/24',
    outbound_enabled: false,
    outbound_allowed_regions: [],
    inbound_enabled: false,
    inbound_allowed_regions: [],
    disable_vm_ipv6: true,
    block_action: 'reject',
    whitelist_cidrs: [],
    geoip_base_url: 'https://www.ipdeny.com/ipblocks/data/aggregated',
    regions: [],
    vm_overrides: {}
  }
}

function createDefaultRule() {
  return { action: 'allow', protocol: 'tcp', port_start: 1, port_end: 65535, source_cidr: '', comment: '' }
}

const policy = reactive(createDefaultPolicy())
const importForm = reactive({ code: '', name: '', source: 'local-import', cidrs: '' })
const ruleForm = reactive(createDefaultRule())

const hostRules = computed(() => hostStatus.value?.rules || [])

const filteredHostRules = computed(() => {
  let data = hostRules.value
  if (fwRuleSearchText.value) {
    const q = fwRuleSearchText.value
    data = data.filter(r => {
      const port = r.port_start ? String(r.port_start) : ''
      const portEnd = r.port_end ? String(r.port_end) : ''
      return port.includes(q) || portEnd.includes(q)
    })
  }
  if (fwProtocolFilter.value) {
    data = data.filter(r => r.protocol === fwProtocolFilter.value)
  }
  if (fwActionFilter.value) {
    data = data.filter(r => r.action === fwActionFilter.value)
  }
  if (fwRemarkSearch.value) {
    const q = fwRemarkSearch.value.toLowerCase()
    data = data.filter(r => (r.comment || '').toLowerCase().includes(q))
  }
  return data
})
const regionOptions = computed(() => (policy.regions || []).map(item => ({
  label: `${item.name || item.code} (${item.code})`,
  value: item.code
})))

const vmRows = computed(() => {
  const rows = (status.value?.vms || [])
    .map(vm => (typeof vm === 'string' ? { name: vm } : vm))
    .filter(vm => vm?.name)
  rows.forEach(vm => {
    if (!policy.vm_overrides[vm.name]) {
      policy.vm_overrides[vm.name] = { mode: 'inherit', regions: [] }
    }
  })
  return rows
})

async function switchTab(name) {
  activeTab.value = name
  if (name === 'kvm' && !status.value) {
    await loadAll()
  }
}

function formatRulePort(row) {
  if (!row) return '-'
  if (row.port_start && row.port_end && row.port_start !== row.port_end) return `${row.port_start}-${row.port_end}`
  if (row.port_start) return String(row.port_start)
  if (row.port_end) return String(row.port_end)
  return String(row.port || 'any')
}

function normalizeRulePayload(rule) {
  return {
    action: rule.action || 'allow',
    protocol: rule.protocol || 'tcp',
    port_start: rule.port_start || null,
    port_end: rule.port_end || null,
    source_cidr: rule.source_cidr || '',
    comment: rule.comment || ''
  }
}

function buildPayload() {
  const data = JSON.parse(JSON.stringify(policy))
  data.whitelist_cidrs = whitelistText.value
    .split('\n')
    .map(line => line.trim())
    .filter(Boolean)
  return data
}

async function loadAll() {
  loading.value = true
  try {
    const jobs = [getHostFirewallStatus().then(res => { hostStatus.value = res.data })]
    if (activeTab.value === 'kvm') {
      jobs.push(getFirewallStatus().then(res => {
        status.value = res.data
        if (res.data?.policy) {
          Object.assign(policy, createDefaultPolicy(), res.data.policy)
          whitelistText.value = (res.data.policy.whitelist_cidrs || []).join('\n')
          geoCodesText.value = ''
        }
      }))
    }
    await Promise.all(jobs)
  } finally {
    loading.value = false
  }
}

async function openEnableHostFirewall() {
  const res = await previewEnableHostFirewall()
  enableRules.value = res.data?.rules || []
  enableVisible.value = true
}

async function handleEnableHostFirewall() {
  hostApplying.value = true
  try {
    const res = await enableHostFirewall({ rules: enableRules.value.map(normalizeRulePayload) })
    ElMessage.success(res.message || '宿主机防火墙启用任务已提交')
    enableVisible.value = false
  } finally {
    hostApplying.value = false
  }
}

async function handleDisableHostFirewall() {
  await ElMessageBox.confirm('确认关闭宿主机防火墙？关闭后端口转发不会再自动写入 UFW 放通。', '高风险操作', { type: 'warning' })
  const res = await disableHostFirewall()
  ElMessage.success(res.message || '宿主机防火墙关闭任务已提交')
}

function openRuleDialog(row) {
  editingRule.value = row || null
  Object.assign(ruleForm, createDefaultRule(), row || {})
  ruleVisible.value = true
}

async function handleSaveHostRule() {
  const payload = normalizeRulePayload(ruleForm)
  if (editingRule.value?.id) {
    await updateHostFirewallRule(editingRule.value.id, payload)
    ElMessage.success('宿主机防火墙规则已更新')
  } else {
    await createHostFirewallRule(payload)
    ElMessage.success('宿主机防火墙规则已添加')
  }
  ruleVisible.value = false
  await loadAll()
}

async function handleDeleteHostRule(row) {
  await ElMessageBox.confirm(`确认删除 ${formatRulePort(row)}/${row.protocol} 规则？`, '高风险操作', { type: 'warning' })
  await deleteHostFirewallRule(row.id)
  ElMessage.success('规则已删除')
  await loadAll()
}

async function handleAddVNCDefault() {
  await ElMessageBox.confirm('确认添加 VNC 默认端口范围 5900-5999/tcp？该规则不是保护规则，后续可编辑或删除。', '确认操作', { type: 'warning' })
  await addHostFirewallVNCDefaultRule()
  ElMessage.success('VNC 默认端口范围已添加')
  await loadAll()
}

async function previewConnections(mode) {
  const res = await previewHostFirewallConnections(mode)
  connectionPreview.value = res.data
}

async function confirmCloseConnections(mode) {
  const res = await previewHostFirewallConnections(mode)
  connectionPreview.value = res.data
  const message = mode === 'all'
    ? `将关闭全部 ${res.data?.count || 0} 个连接，包括 SSH 和面板连接，当前会话可能立即断开。确认继续？`
    : `将关闭 ${res.data?.count || 0} 个非防火墙端口连接，确认继续？`
  await ElMessageBox.confirm(message, '高风险操作', { type: 'warning' })
  await closeHostFirewallConnections({ mode })
  ElMessage.success('连接关闭命令已执行')
}

async function handleSave() {
  saving.value = true
  try {
    await saveFirewallPolicy(buildPayload())
    ElMessage.success('KVM 网络防火墙策略已保存')
    await loadAll()
  } finally {
    saving.value = false
  }
}

async function handlePreview() {
  const res = await previewFirewallPolicy(buildPayload())
  previewRules.value = res.data?.rules || ''
  previewVisible.value = true
}

async function handleApply() {
  await ElMessageBox.confirm('应用后会立即影响 KVM 虚拟机入站/出站转发流量，确认继续？', '高风险操作', { type: 'warning' })
  applying.value = true
  try {
    const res = await applyFirewallPolicy(buildPayload())
    ElMessage.success(res.message || 'KVM 网络防火墙应用任务已提交')
  } finally {
    applying.value = false
  }
}

async function handleDisable() {
  await ElMessageBox.confirm('确认禁用 KVM 网络防火墙并删除独立 nft 表？', '高风险操作', { type: 'warning' })
  const res = await disableFirewall()
  ElMessage.success(res.message || 'KVM 网络防火墙禁用任务已提交')
}

async function handleRollback() {
  await ElMessageBox.confirm('回滚会删除独立 nft 表，恢复到未管控状态，确认继续？', '高风险操作', { type: 'warning' })
  const res = await rollbackFirewall()
  ElMessage.success(res.message || 'KVM 网络防火墙回滚任务已提交')
}

async function handleImport() {
  await importFirewallRegion(importForm)
  ElMessage.success('区域 CIDR 已导入')
  importVisible.value = false
  Object.assign(importForm, { code: '', name: '', source: 'local-import', cidrs: '' })
  await loadAll()
}

async function handleGeoUpdate() {
  const codes = geoCodesText.value.split(/[,\s]+/).map(v => v.trim()).filter(Boolean)
  if (codes.length === 0) {
    ElMessage.warning('请输入需要更新的区域代码')
    return
  }
  const res = await updateFirewallGeoIP({ codes, base_url: policy.geoip_base_url })
  ElMessage.success(res.message || 'GeoIP 更新任务已提交')
}

onMounted(loadAll)
</script>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  flex-wrap: wrap;
}
.firewall-page {
  padding: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 24px 24px 20px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.page-header-left .page-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.page-header-left .page-icon {
  font-size: 24px;
  color: var(--el-color-primary);
}

.page-header-left h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.page-header-left p {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.page-header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

/* 自定义标签页 */
.tabs-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.custom-tabs {
  display: flex;
  gap: 0;
  padding: 0 24px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-lighter);
  overflow-x: auto;
}

.custom-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 14px 20px;
  font-size: 14px;
  color: var(--el-text-color-secondary);
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
  white-space: nowrap;
  user-select: none;
}

.custom-tab:hover {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.custom-tab.active {
  color: var(--el-color-primary);
  font-weight: 600;
  border-bottom-color: var(--el-color-primary);
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px 24px;
  background: var(--el-bg-color-page);
}

.tab-pane {
  animation: fadeIn 0.25s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

/* 状态横幅 */
.status-banner {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px 24px;
  border-radius: 8px;
  margin-bottom: 20px;
}

.status-banner.enabled {
  background: rgba(103, 194, 58, 0.06);
  border: 1px solid rgba(103, 194, 58, 0.2);
}

.status-banner.enabled .status-banner-icon {
  color: #67C23A;
}

.status-banner.disabled {
  background: rgba(144, 147, 153, 0.06);
  border: 1px solid rgba(144, 147, 153, 0.2);
}

.status-banner.disabled .status-banner-icon {
  color: #909399;
}

.status-banner.info {
  background: rgba(64, 158, 255, 0.06);
  border: 1px solid rgba(64, 158, 255, 0.2);
}

.status-banner.info .status-banner-icon {
  color: #409EFF;
}

.status-banner.warning {
  background: rgba(230, 162, 60, 0.06);
  border: 1px solid rgba(230, 162, 60, 0.2);
}

.status-banner.warning .status-banner-icon {
  color: #E6A23C;
}

.status-banner-body {
  flex: 1;
}

.status-banner-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-bottom: 4px;
}

.status-banner-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.status-banner-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

/* KVM 操作栏 */
.kvm-actions-bar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.kvm-actions-bar h3 {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.kvm-actions-bar p {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.kvm-actions-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

/* 信息卡片 */
.info-card {
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  margin-bottom: 0;
  overflow: hidden;
}

.info-card.table-card {
  margin-bottom: 0;
}

.info-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 20px;
  font-weight: 600;
  font-size: 14px;
  color: var(--el-text-color-primary);
  background: var(--el-bg-color-page);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.info-card-header .el-icon {
  color: var(--el-color-primary);
}

.info-card-body-padded {
  padding: 20px;
}

.info-card-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 20px;
  background: var(--el-fill-color-lighter);
  border-top: 1px solid var(--el-border-color-lighter);
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.info-card-footer .el-icon {
  color: #909399;
  font-size: 14px;
}

.header-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

.info-list {
  padding: 4px 0;
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 20px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.info-item:last-child {
  border-bottom: none;
}

.info-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
}

.info-value {
  font-size: 13px;
  color: var(--el-text-color-primary);
  text-align: right;
  word-break: break-all;
}

.code {
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', Consolas, monospace;
  font-size: 13px;
  background: var(--el-fill-color-light);
  padding: 2px 8px;
  border-radius: 4px;
}

.text-muted {
  color: var(--el-text-color-placeholder);
}

.vm-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--el-text-color-primary);
}

/* 连接操作面板 */
.actions-panel {
  display: flex;
  align-items: stretch;
  padding: 24px;
}

.action-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.action-group-label {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.action-group-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}

.action-group-buttons {
  display: flex;
  gap: 8px;
  margin-top: auto;
}

.action-divider {
  height: auto;
  margin: 0 32px;
}

/* 数据表格 */
.data-table-card .el-table {
  --el-table-border-color: var(--el-border-color-lighter);
}

:deep(.el-table th) {
  background: var(--el-fill-color-light);
  font-weight: 600;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

/* 代码块 */
.code-block-wrapper {
  border-radius: 8px;
  overflow: hidden;
}

.code-block {
  margin: 0;
  padding: 16px;
  background: #1e1e2e;
  color: #cdd6f4;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', Consolas, monospace;
  font-size: 13px;
  line-height: 1.7;
  overflow-x: auto;
  max-height: 500px;
  overflow-y: auto;
  border-radius: 8px;
}

.code-block code {
  white-space: pre;
}

/* 现代化对话框 */
:deep(.modern-dialog) .el-dialog {
  border-radius: 12px;
  overflow: hidden;
}

:deep(.modern-dialog) .el-dialog__header {
  padding: 20px 24px 16px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  margin: 0;
}

:deep(.modern-dialog) .el-dialog__title {
  font-size: 16px;
  font-weight: 600;
}

:deep(.modern-dialog) .el-dialog__body {
  padding: 24px;
}

:deep(.modern-dialog) .el-dialog__footer {
  padding: 16px 24px;
  border-top: 1px solid var(--el-border-color-lighter);
}

:deep(.modern-dialog) .el-divider__text {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  background: var(--el-bg-color);
}

/* 响应式 */
@media (max-width: 900px) {
  .page-header {
    flex-direction: column;
    gap: 12px;
    padding: 16px;
  }

  .custom-tabs {
    padding: 0 8px;
  }

  .custom-tab {
    padding: 12px 14px;
    font-size: 13px;
  }

  .tab-content {
    padding: 12px;
  }

  .kvm-actions-bar {
    flex-direction: column;
    gap: 12px;
  }

  .kvm-actions-buttons {
    width: 100%;
  }

  .status-banner {
    flex-direction: column;
    align-items: flex-start;
  }

  .status-banner-actions {
    width: 100%;
  }

  .actions-panel {
    flex-direction: column;
    gap: 20px;
  }

  .action-divider {
    width: 100%;
    height: 1px;
    margin: 0;
  }
}

/* ===== 手机小屏幕独立卡片模式 ===== */
@media (max-width: 768px) {
  .page-header {
    padding: 12px;
  }

  .page-header-left h2 {
    font-size: 16px;
  }

  .page-header-left .page-icon {
    font-size: 20px;
  }

  .page-header-left p {
    font-size: 12px;
    display: none;
  }

  .custom-tab {
    padding: 10px 12px;
    font-size: 12px;
    gap: 4px;
  }

  .tab-content {
    padding: 8px;
  }

  /* 两列布局改为单列 */
  .tab-pane .el-row {
    display: flex;
    flex-direction: column;
  }

  .tab-pane .el-row .el-col {
    max-width: 100% !important;
    flex: 0 0 100% !important;
    margin-bottom: 10px;
  }

  /* 状态横幅紧凑 */
  .status-banner {
    padding: 14px 16px;
    gap: 10px;
  }

  .status-banner-title {
    font-size: 14px;
  }

  .status-banner-desc {
    font-size: 12px;
  }

  .status-banner-actions {
    flex-wrap: wrap;
  }

  /* KVM 操作栏 */
  .kvm-actions-bar {
    margin-bottom: 12px;
  }

  .kvm-actions-bar h3 {
    font-size: 14px;
  }

  .kvm-actions-bar p {
    font-size: 12px;
  }

  /* 信息卡片边距 */
  .info-card {
    border-radius: 10px;
    margin-bottom: 10px;
  }

  .info-card-header {
    padding: 12px 14px;
    font-size: 13px;
  }

  .info-card-body-padded {
    padding: 12px;
  }

  .info-list .info-item {
    padding: 10px 14px;
  }

  /* 表格行 → 独立卡片 */
  .info-card.table-card .el-table {
    display: block !important;
    border: none !important;
  }

  .info-card.table-card .el-table__header-wrapper {
    display: none !important;
  }

  .info-card.table-card .el-table__body-wrapper {
    display: block !important;
    overflow: visible !important;
  }

  .info-card.table-card .el-table__body {
    display: block !important;
    width: 100% !important;
  }

  .info-card.table-card .el-table__body tbody {
    display: block !important;
  }

  .info-card.table-card .el-table__body tr {
    display: block !important;
    margin-bottom: 8px;
    border: 1px solid var(--el-border-color-lighter) !important;
    border-radius: 10px !important;
    background: var(--el-bg-color);
    overflow: hidden;
    transition: box-shadow 0.2s;
  }

  .info-card.table-card .el-table__body tr:last-child {
    margin-bottom: 0;
  }

  .info-card.table-card .el-table__body td {
    display: flex !important;
    align-items: center;
    justify-content: space-between;
    padding: 8px 14px !important;
    border-bottom: 1px solid var(--el-border-color-extra-light) !important;
    border-right: none !important;
    min-height: 36px;
  }

  .info-card.table-card .el-table__body td:last-child {
    border-bottom: none !important;
    background: var(--el-fill-color-lighter);
    padding: 10px 14px !important;
    gap: 6px;
    flex-wrap: wrap;
    justify-content: flex-start;
  }

  .info-card.table-card .el-table__body td::before {
    content: attr(data-label);
    font-size: 12px;
    color: var(--el-text-color-secondary);
    flex-shrink: 0;
    margin-right: 12px;
    white-space: nowrap;
  }

  .info-card.table-card .el-table__body td .cell {
    flex: 1;
    text-align: right;
    padding: 0;
    font-size: 13px;
  }

  .info-card.table-card .el-table__body td:last-child .cell {
    text-align: left;
  }

  .info-card.table-card .el-table__body td .el-button {
    margin-left: 0;
  }

  /* 操作按钮适配 */
  .header-actions {
    flex-wrap: wrap;
    gap: 4px;
  }

  .header-actions .el-button {
    font-size: 12px;
    padding: 5px 10px;
  }

  /* 连接管理动作面板 */
  .actions-panel {
    padding: 16px;
  }

  .action-group-label {
    font-size: 14px;
  }

  .action-group-desc {
    font-size: 12px;
  }

  /* 代码块 */
  .code-block {
    font-size: 11px;
    padding: 12px;
  }
}

@media (max-width: 480px) {
  .custom-tabs {
    padding: 0 4px;
    gap: 0;
  }

  .custom-tab {
    padding: 8px 10px;
    font-size: 11px;
    gap: 2px;
  }

  .status-banner {
    padding: 10px 12px;
  }

  .info-card-header {
    padding: 10px 12px;
  }

  .info-card.table-card .el-table__body td {
    padding: 6px 10px !important;
  }

  .info-card.table-card .el-table__body td:last-child {
    padding: 8px 10px !important;
  }
}
</style>
