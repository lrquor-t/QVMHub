<template>
  <div class="vm-list-container">
    <div class="maintenance-shell">
      <div class="maintenance-content" :class="{ 'content-blurred': showMaintenanceOverlay }">
        <el-card>
          <div class="vm-header">
            <div class="vm-header-left">
              <h2 class="vm-title">虚拟机列表</h2>
            </div>
            <div class="vm-header-center">
              <el-input
                v-model="vmSearchText"
                placeholder="搜索虚拟机名称、备注、模板..."
                clearable
                :prefix-icon="Search"
                class="vm-search-input"
              />
            </div>
            <div class="vm-header-right">
              <div class="vm-actions-primary">
                <el-switch 
                  v-model="autoRefresh" 
                  active-text="自动刷新" 
                  @change="onAutoRefreshChange"
                  class="auto-refresh-switch"
                />
                <el-button type="success" icon="Refresh" @click="fetchData" :loading="loading">刷新</el-button>
                <el-button v-if="!isLightweight" type="primary" icon="Plus" @click="handleCreate">新建虚拟机</el-button>
              </div>
              <div v-show="hasSelection" class="vm-actions-batch">
                <el-button type="warning" @click="handleBatchOperate('start')" :loading="batchOperating">开机</el-button>
                <el-button type="info" @click="handleBatchOperate('shutdown')" :loading="batchOperating">关机</el-button>
                <el-button v-if="!isLightweight" type="danger" @click="handleBatchDelete" :loading="batchOperating">删除</el-button>
              </div>
            </div>
          </div>

          <div class="vm-toolbar">
            <div class="vm-toolbar-left">
              <div class="group-toggle-bar">
                <el-radio-group v-model="vmGroupBy" size="small">
                  <el-radio-button value="status">按状态</el-radio-button>
                  <el-radio-button value="template">按模板</el-radio-button>
                  <el-radio-button value="custom">自定义</el-radio-button>
                  <el-radio-button value="">全部</el-radio-button>
                </el-radio-group>
              </div>
              <div class="view-mode-toggle">
                <el-tooltip content="卡片视图" placement="top">
                  <el-button
                    size="small"
                    :type="viewMode === 'card' ? 'primary' : 'default'"
                    @click="viewMode = 'card'"
                  >
                    <el-icon><Grid /></el-icon>
                  </el-button>
                </el-tooltip>
                <el-tooltip content="列表视图" placement="top">
                  <el-button
                    size="small"
                    :type="viewMode === 'list' ? 'primary' : 'default'"
                    @click="viewMode = 'list'"
                  >
                    <el-icon><ListIcon /></el-icon>
                  </el-button>
                </el-tooltip>
              </div>
            </div>
            <div class="vm-toolbar-right"></div>
          </div>

          <div v-if="isLightweight && pendingRegistrations.length" class="pending-registration-panel">
            <div class="pending-registration-header">
              <div>
                <h3>待开通服务器</h3>
                <p>这些服务器已由管理员登记配置。请逐台补全登录凭据，确认后系统才会开始开通。</p>
              </div>
              <el-button size="small" type="success" :loading="pendingRegistrationLoading" @click="fetchPendingRegistrations">刷新待开通</el-button>
            </div>
            <el-row :gutter="16">
              <el-col v-for="item in pendingRegistrations" :key="item.id" :xs="24" :sm="12" :lg="8">
                <el-card shadow="hover" class="pending-registration-card">
                  <div class="pending-registration-title">
                    <span>{{ item.vm_name }}</span>
                    <el-tag :type="registrationStatusType(item.status)" size="small">{{ registrationStatusText(item.status) }}</el-tag>
                  </div>
                  <div class="pending-registration-meta">模板：{{ item.template || '-' }}</div>
                  <div class="pending-registration-meta">规格：{{ item.vcpu }}C / {{ item.ram }}GB / {{ item.disk_size }}GB</div>
                  <div class="pending-registration-meta">网络：{{ formatRegistrationQuota(item) }}</div>
                  <div v-if="item.error_message" class="pending-registration-error">{{ item.error_message }}</div>
                  <el-button
                    type="primary"
                    size="small"
                    style="width: 100%; margin-top: 12px;"
                    :disabled="item.status === 'provisioning'"
                    @click="openRegistrationConfirm(item)"
                  >
                    {{ item.status === 'failed' ? '重新确认开通' : '确认开通' }}
                  </el-button>
                </el-card>
              </el-col>
            </el-row>
          </div>

          <!-- ===== 分组视图（列表模式） ===== -->
          <div v-if="vmGroupBy && viewMode === 'list'" class="grouped-view">
            <div
              v-for="group in groupList"
              :key="group.key"
              class="group-section"
              :class="{ 'group-collapsed': !groupExpandedMap[group.key] }"
            >
              <div class="group-header" @click="toggleGroupExpand(group.key)">
                <div class="group-header-left">
                  <el-icon class="group-expand-icon" :class="{ 'is-expanded': groupExpandedMap[group.key] }">
                    <ArrowRight />
                  </el-icon>
                  <span class="group-name">
                    <el-tag :type="groupTagType(group.key)" size="small" effect="plain">{{ group.label }}</el-tag>
                  </span>
                  <span class="group-count">{{ group.vms.length }} 台</span>
                </div>
                <div class="group-header-right">
                  <el-button
                    v-if="group.vms.length > 0"
                    link
                    type="primary"
                    size="small"
                    @click.stop="selectGroupVMs(group)"
                  >
                    {{ groupAllSelected(group) ? '取消全选' : '全选本组' }}
                  </el-button>
                </div>
              </div>
              <div v-show="groupExpandedMap[group.key]" class="group-body">
                <div class="group-table-wrap">
                  <el-table
                    :data="group.vms"
                    border
                    style="width: 100%"
                    @selection-change="(val) => onGroupSelectionChange(group.key, val)"
                    :ref="(el) => setGroupTableRef(group.key, el)"
                  >
                    <el-table-column type="selection" width="55" align="center" />
                    <el-table-column label="名称">
                      <template #default="{ row }">
                        <el-link type="primary" :underline="false" @click="$router.push(`/vm/detail/${row.name}`)">
                          {{ row.name }}
                        </el-link>
                      </template>
                    </el-table-column>
                    <el-table-column prop="status" label="状态" width="100" align="center" sortable :sort-method="sortStatus">
                      <template #default="{ row }">
                        <el-tag :type="statusType(row.status)">
                          {{ statusText(row.status) }}
                        </el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column prop="vcpu" label="CPU" width="70" align="center" />
                    <el-table-column label="内存" width="90" align="center">
                      <template #default="{ row }">
                        {{ row.memory >= 1024 ? (row.memory / 1024).toFixed(1) + ' GB' : row.memory + ' MB' }}
                      </template>
                    </el-table-column>
                    <el-table-column label="IP 地址" width="150">
                      <template #default="{ row }">
                        <div class="disk-cell">
                          <el-tooltip v-if="hasLoadedIP(row.name) && ipTooltipText(row.name)" :content="ipTooltipText(row.name)" placement="top" effect="dark">
                            <span class="ip-unreachable">{{ ipDisplayText(row.name) }}</span>
                          </el-tooltip>
                          <span v-else-if="hasLoadedIP(row.name)">{{ ipDisplayText(row.name) }}</span>
                          <el-button v-else link type="primary" :loading="ipLoadingMap[row.name]" @click="loadVmIP(row)">点击加载</el-button>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column label="磁盘" width="170" align="center">
                      <template #default="{ row }">
                        <div class="disk-cell">
                          <span>{{ row.disk_size || '-' }}</span>
                          <el-button v-if="row.disk_size && !diskUsageMap[row.name]" link type="primary" :loading="diskLoadingMap[row.name]" @click="loadVmDiskUsage(row)">查看占用</el-button>
                          <span v-else-if="diskUsageMap[row.name]" class="disk-used-text">已用 {{ diskUsageMap[row.name].used_gb }}</span>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column label="备注/分组" min-width="180">
                      <template #default="{ row }">
                        <div class="remark-cell">
                          <div style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap;">
                            <el-tag v-if="row.group" size="small" type="warning" effect="plain" style="max-width: 100px; overflow: hidden; text-overflow: ellipsis;">{{ row.group }}</el-tag>
                            <span class="remark-text" :title="row.remark || ''">{{ row.remark || '-' }}</span>
                          </div>
                          <div style="display: flex; gap: 4px;">
                            <el-button v-if="!isLightweight" link type="warning" size="small" :disabled="isMigrating(row)" @click.stop="handleEditGroup(row)">分组</el-button>
                            <el-button v-if="!isLightweight" link type="primary" size="small" :disabled="isMigrating(row)" @click.stop="handleEditRemark(row)">备注</el-button>
                          </div>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column label="资源使用" width="160" align="center">
                      <template #default="{ row }">
                        <template v-if="row.status === 'running' && row.cpu_percent >= 0 && row.mem_percent >= 0">
                          <div style="display: flex; flex-direction: column; gap: 4px;">
                            <el-tooltip content="CPU使用率" placement="left">
                              <el-progress :percentage="Math.min(row.cpu_percent, 100)" :stroke-width="14" :text-inside="true" :color="progressColor(row.cpu_percent)" :format="() => 'CPU ' + row.cpu_percent.toFixed(1) + '%'" />
                            </el-tooltip>
                            <el-tooltip content="内存使用率" placement="left">
                              <el-progress :percentage="Math.min(row.mem_percent, 100)" :stroke-width="14" :text-inside="true" :color="progressColor(row.mem_percent)" :format="() => 'MEM ' + row.mem_percent.toFixed(1) + '%'" />
                            </el-tooltip>
                          </div>
                        </template>
                        <span v-else>-</span>
                      </template>
                    </el-table-column>
                    <el-table-column label="连续运行" width="130" align="center">
                      <template #default="{ row }">
                        <el-tooltip v-if="row.continuous_running_since" :content="`开始时间：${row.continuous_running_since}`" placement="top">
                          <span>{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
                        </el-tooltip>
                        <span v-else>{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
                      </template>
                    </el-table-column>
                    <el-table-column prop="template" label="模板" width="120" show-overflow-tooltip :formatter="formatTemplateValue" />
                    <el-table-column prop="created_at" label="创建时间" min-width="170" align="center" sortable />
                    <el-table-column label="操作" width="250" fixed="right" align="center">
                      <template #default="{ row }">
                        <el-tooltip :content="row.locked && row.status === 'running' ? '虚拟机已锁定，关机需二次确认' : (row.status === 'running' ? '关机' : row.status === 'paused' ? '继续启动' : '开机')" placement="top">
                          <el-button size="small" circle class="operate-btn" :type="row.status === 'running' ? 'warning' : 'success'" :loading="operatingMap[row.name]" :icon="row.status === 'running' ? 'SwitchButton' : 'VideoPlay'" :disabled="isMigrating(row)" @click="handleOperate(row, row.status === 'running' ? 'shutdown' : 'start')" />
                        </el-tooltip>
                        <el-tooltip v-if="row.status === 'paused'" content="重置" placement="top">
                          <el-button size="small" circle class="operate-btn" type="danger" :loading="operatingMap[row.name]" icon="RefreshRight" :disabled="isMigrating(row)" @click="handleOperate(row, 'reset')" />
                        </el-tooltip>
                        <el-tooltip v-if="!isLightweight" content="编辑" placement="top">
                          <el-button size="small" type="primary" plain circle icon="Edit" :disabled="isMigrating(row)" @click="handleEdit(row)" />
                        </el-tooltip>
                        <el-tooltip content="详情" placement="top">
                          <el-button size="small" type="success" plain circle icon="View" @click="$router.push(`/vm/detail/${row.name}`)" />
                        </el-tooltip>
                        <el-dropdown style="margin-left: 12px;" @command="cmd => handleMore(cmd, row)">
                          <span class="el-dropdown-link">
                            <el-tooltip content="更多" placement="top">
                              <el-button size="small" circle icon="MoreFilled" />
                            </el-tooltip>
                          </span>
                          <template #dropdown>
                            <el-dropdown-menu>
                              <el-dropdown-item v-if="isAdmin" command="template" :disabled="isMigrating(row)">制作模板</el-dropdown-item>
                              <el-dropdown-item v-if="!isLightweight" command="export" :disabled="isMigrating(row)">导出虚拟机</el-dropdown-item>
                              <el-dropdown-item v-if="!isLightweight" command="network" :disabled="isMigrating(row)">网络管理</el-dropdown-item>
                              <el-dropdown-item v-if="!isLightweight" command="reinstall" :disabled="isMigrating(row)">重装系统</el-dropdown-item>
                              <el-dropdown-item v-if="isAdmin" command="migrate" :disabled="isMigrating(row)">迁移</el-dropdown-item>
                              <el-dropdown-item v-if="isAdmin && row.is_linked_clone" command="make_independent" divided :disabled="isMigrating(row) || row.status !== 'shut off'">转为独立虚拟机</el-dropdown-item>
                              <el-dropdown-item command="snapshot" :disabled="isMigrating(row)">快照管理</el-dropdown-item>
                              <el-dropdown-item divided :command="row.in_rescue ? 'rescue_stop' : 'rescue_start'" :disabled="isMigrating(row)">
                                <span :style="{ color: row.in_rescue ? '#E6A23C' : '#409EFF' }">{{ row.in_rescue ? '关闭救援系统' : '启动救援系统' }}</span>
                              </el-dropdown-item>
                              <el-dropdown-item v-if="!isLightweight" command="delete" divided style="color: red;" :disabled="isMigrating(row) || row.locked">删除</el-dropdown-item>
                              <el-dropdown-item v-if="!isLightweight" divided :command="row.locked ? 'unlock' : 'lock'">
                                <span :style="{ color: row.locked ? '#E6A23C' : '#409EFF' }">
                                  <el-icon style="margin-right: 4px;"><component :is="row.locked ? 'Lock' : 'Unlock'" /></el-icon>
                                  {{ row.locked ? '解除锁定' : '锁定虚拟机' }}
                                </span>
                              </el-dropdown-item>
                            </el-dropdown-menu>
                          </template>
                        </el-dropdown>
                      </template>
                    </el-table-column>
                  </el-table>
                </div>
              </div>
            </div>
          </div>

          <!-- ===== 分组卡片视图（PC端） ===== -->
          <div v-if="vmGroupBy && viewMode === 'card'" class="grouped-card-view">
            <div
              v-for="group in groupList"
              :key="group.key"
              class="group-section"
              :class="{ 'group-collapsed': !groupExpandedMap[group.key] }"
            >
              <div class="group-header" @click="toggleGroupExpand(group.key)">
                <div class="group-header-left">
                  <el-icon class="group-expand-icon" :class="{ 'is-expanded': groupExpandedMap[group.key] }">
                    <ArrowRight />
                  </el-icon>
                  <span class="group-name">
                    <el-tag :type="groupTagType(group.key)" size="small" effect="plain">{{ group.label }}</el-tag>
                  </span>
                  <span class="group-count">{{ group.vms.length }} 台</span>
                </div>
                <div class="group-header-right">
                  <el-checkbox
                    v-if="group.vms.length > 0"
                    :model-value="groupAllSelected(group)"
                    @change="selectGroupVMs(group)"
                    @click.stop
                  >全选</el-checkbox>
                </div>
              </div>
              <div v-show="groupExpandedMap[group.key]" class="group-body">
                <div class="card-grid">
                  <div v-for="row in group.vms" :key="row.name" class="vm-card" :class="cardStatusClass(row)">
                    <div class="card-header">
                      <div class="card-title">
                        <VmStatusIcons :status="row.status || 'shut off'" :size="14" />
                        <el-link type="primary" :underline="false" @click="$router.push(`/vm/detail/${row.name}`)">
                          {{ row.name }}
                        </el-link>
                        <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
                      </div>
                      <el-tooltip :content="row.locked && row.status === 'running' ? '虚拟机已锁定，关机需二次确认' : (row.status === 'running' ? '关机' : row.status === 'paused' ? '继续启动' : '开机')" placement="top">
                        <el-button
                          size="small"
                          circle
                          :type="row.status === 'running' ? 'warning' : 'success'"
                          :loading="operatingMap[row.name]"
                          :icon="row.status === 'running' ? 'SwitchButton' : 'VideoPlay'"
                          :disabled="isMigrating(row)"
                          @click="handleOperate(row, row.status === 'running' ? 'shutdown' : 'start')"
                        />
                      </el-tooltip>
                    </div>
                    <div class="card-specs">
                      <span class="spec-item">
                        <span class="spec-label">CPU</span>
                        <span class="spec-value">{{ row.vcpu }}核</span>
                      </span>
                      <span class="spec-item">
                        <span class="spec-label">内存</span>
                        <span class="spec-value">{{ formatMemory(row.memory) }}</span>
                      </span>
                      <span class="spec-item">
                        <span class="spec-label">磁盘</span>
                        <span class="spec-value">{{ row.disk_size || '-' }}</span>
                      </span>
                    </div>
                    <div class="card-info">
                      <div class="info-row">
                        <span class="info-label">IP 地址</span>
                        <span class="info-value">
                          <template v-if="hasLoadedIP(row.name)">
                            <el-tooltip v-if="ipTooltipText(row.name)" :content="ipTooltipText(row.name)" placement="top" effect="dark">
                              <span class="ip-unreachable">{{ ipDisplayText(row.name) }}</span>
                            </el-tooltip>
                            <span v-else>{{ ipDisplayText(row.name) }}</span>
                          </template>
                          <el-button v-else link type="primary" size="small" :loading="ipLoadingMap[row.name]" @click="loadVmIP(row)">点击加载</el-button>
                        </span>
                      </div>
                      <div class="info-row">
                        <span class="info-label">模板</span>
                        <span class="info-value">{{ formatTemplateValue(null, null, row.template) }}</span>
                      </div>
                      <div class="info-row">
                        <span class="info-label">备注</span>
                        <span class="info-value">
                          <el-tag v-if="row.group" size="small" type="warning" effect="plain" style="margin-right: 4px;">{{ row.group }}</el-tag>
                          {{ row.remark || '-' }}
                          <span v-if="!isLightweight" class="card-remark-edit">
                            <el-button link type="warning" size="small" :disabled="isMigrating(row)" @click.stop="handleEditGroup(row)">分组</el-button>
                            <el-button link type="primary" size="small" :disabled="isMigrating(row)" @click.stop="handleEditRemark(row)">备注</el-button>
                          </span>
                        </span>
                      </div>
                      <div class="info-row">
                        <span class="info-label">连续运行</span>
                        <span class="info-value">
                          <el-tooltip v-if="row.continuous_running_since" :content="`开始时间：${row.continuous_running_since}`" placement="top">
                            <span>{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
                          </el-tooltip>
                          <span v-else>{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
                        </span>
                      </div>
                    </div>
                    <div v-if="row.status === 'running' && row.cpu_percent >= 0 && row.mem_percent >= 0" class="card-resource">
                      <div class="resource-item">
                        <span class="resource-label">CPU</span>
                        <el-progress :percentage="Math.min(row.cpu_percent, 100)" :stroke-width="6" :color="progressColor(row.cpu_percent)" :show-text="true">
                          <template #default><span class="progress-text">{{ row.cpu_percent.toFixed(1) }}%</span></template>
                        </el-progress>
                      </div>
                      <div class="resource-item">
                        <span class="resource-label">内存</span>
                        <el-progress :percentage="Math.min(row.mem_percent, 100)" :stroke-width="6" :color="progressColor(row.mem_percent)" :show-text="true">
                          <template #default><span class="progress-text">{{ row.mem_percent.toFixed(1) }}%</span></template>
                        </el-progress>
                      </div>
                    </div>
                    <div class="card-footer">
                      <div class="card-footer-left">
                        <el-button size="small" type="primary" plain @click="$router.push(`/vm/detail/${row.name}`)">详情</el-button>
                        <el-button v-if="!isLightweight" size="small" plain :disabled="isMigrating(row)" @click="handleEdit(row)">编辑</el-button>
                      </div>
                      <el-dropdown @command="cmd => handleMore(cmd, row)">
                        <el-button size="small" circle icon="MoreFilled" :disabled="isMigrating(row)" />
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item v-if="isAdmin" command="template" :disabled="isMigrating(row)">制作模板</el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" command="export" :disabled="isMigrating(row)">导出虚拟机</el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" command="network" :disabled="isMigrating(row)">网络管理</el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" command="reinstall" :disabled="isMigrating(row)">重装系统</el-dropdown-item>
                            <el-dropdown-item v-if="isAdmin" command="migrate" :disabled="isMigrating(row)">迁移</el-dropdown-item>
                            <el-dropdown-item v-if="isAdmin && row.is_linked_clone" command="make_independent" divided :disabled="isMigrating(row) || row.status !== 'shut off'">转为独立虚拟机</el-dropdown-item>
                            <el-dropdown-item command="snapshot" :disabled="isMigrating(row)">快照管理</el-dropdown-item>
                            <el-dropdown-item divided :command="row.in_rescue ? 'rescue_stop' : 'rescue_start'" :disabled="isMigrating(row)">
                              <span :style="{ color: row.in_rescue ? '#E6A23C' : '#409EFF' }">{{ row.in_rescue ? '关闭救援系统' : '启动救援系统' }}</span>
                            </el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" command="delete" divided style="color: red;" :disabled="isMigrating(row) || row.locked">删除</el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" divided :command="row.locked ? 'unlock' : 'lock'">
                              <span :style="{ color: row.locked ? '#E6A23C' : '#409EFF' }">{{ row.locked ? '解除锁定' : '锁定虚拟机' }}</span>
                            </el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- ===== 全部视图卡片模式（PC端） ===== -->
          <div v-if="!vmGroupBy && viewMode === 'card'" class="all-card-view">
            <div class="card-grid">
              <div v-for="row in paginatedTableData" :key="row.name" class="vm-card" :class="cardStatusClass(row)">
                <div class="card-header">
                  <div class="card-title">
                    <VmStatusIcons :status="row.status || 'shut off'" :size="14" />
                    <el-link type="primary" :underline="false" @click="$router.push(`/vm/detail/${row.name}`)">
                      {{ row.name }}
                    </el-link>
                    <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
                  </div>
                  <el-tooltip :content="row.locked && row.status === 'running' ? '虚拟机已锁定，关机需二次确认' : (row.status === 'running' ? '关机' : row.status === 'paused' ? '继续启动' : '开机')" placement="top">
                    <el-button
                      size="small"
                      circle
                      :type="row.status === 'running' ? 'warning' : 'success'"
                      :loading="operatingMap[row.name]"
                      :icon="row.status === 'running' ? 'SwitchButton' : 'VideoPlay'"
                      :disabled="isMigrating(row)"
                      @click="handleOperate(row, row.status === 'running' ? 'shutdown' : 'start')"
                    />
                  </el-tooltip>
                </div>
                <div class="card-specs">
                  <span class="spec-item">
                    <span class="spec-label">CPU</span>
                    <span class="spec-value">{{ row.vcpu }}核</span>
                  </span>
                  <span class="spec-item">
                    <span class="spec-label">内存</span>
                    <span class="spec-value">{{ formatMemory(row.memory) }}</span>
                  </span>
                  <span class="spec-item">
                    <span class="spec-label">磁盘</span>
                    <span class="spec-value">{{ row.disk_size || '-' }}</span>
                  </span>
                </div>
                <div class="card-info">
                  <div class="info-row">
                    <span class="info-label">IP 地址</span>
                    <span class="info-value">
                      <template v-if="hasLoadedIP(row.name)">
                        <el-tooltip v-if="ipTooltipText(row.name)" :content="ipTooltipText(row.name)" placement="top" effect="dark">
                          <span class="ip-unreachable">{{ ipDisplayText(row.name) }}</span>
                        </el-tooltip>
                        <span v-else>{{ ipDisplayText(row.name) }}</span>
                      </template>
                      <el-button v-else link type="primary" size="small" :loading="ipLoadingMap[row.name]" @click="loadVmIP(row)">点击加载</el-button>
                    </span>
                  </div>
                  <div class="info-row">
                    <span class="info-label">模板</span>
                    <span class="info-value">{{ formatTemplateValue(null, null, row.template) }}</span>
                  </div>
                  <div class="info-row">
                    <span class="info-label">备注</span>
                    <span class="info-value">
                      <el-tag v-if="row.group" size="small" type="warning" effect="plain" style="margin-right: 4px;">{{ row.group }}</el-tag>
                      {{ row.remark || '-' }}
                      <span v-if="!isLightweight" class="card-remark-edit">
                        <el-button link type="warning" size="small" :disabled="isMigrating(row)" @click.stop="handleEditGroup(row)">分组</el-button>
                        <el-button link type="primary" size="small" :disabled="isMigrating(row)" @click.stop="handleEditRemark(row)">备注</el-button>
                      </span>
                    </span>
                  </div>
                  <div class="info-row">
                    <span class="info-label">连续运行</span>
                    <span class="info-value">
                      <el-tooltip v-if="row.continuous_running_since" :content="`开始时间：${row.continuous_running_since}`" placement="top">
                        <span>{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
                      </el-tooltip>
                      <span v-else>{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
                    </span>
                  </div>
                </div>
                <div v-if="row.status === 'running' && row.cpu_percent >= 0 && row.mem_percent >= 0" class="card-resource">
                  <div class="resource-item">
                    <span class="resource-label">CPU</span>
                    <el-progress :percentage="Math.min(row.cpu_percent, 100)" :stroke-width="6" :color="progressColor(row.cpu_percent)" :show-text="true">
                      <template #default><span class="progress-text">{{ row.cpu_percent.toFixed(1) }}%</span></template>
                    </el-progress>
                  </div>
                  <div class="resource-item">
                    <span class="resource-label">内存</span>
                    <el-progress :percentage="Math.min(row.mem_percent, 100)" :stroke-width="6" :color="progressColor(row.mem_percent)" :show-text="true">
                      <template #default><span class="progress-text">{{ row.mem_percent.toFixed(1) }}%</span></template>
                    </el-progress>
                  </div>
                </div>
                <div class="card-footer">
                  <div class="card-footer-left">
                    <el-button size="small" type="primary" plain @click="$router.push(`/vm/detail/${row.name}`)">详情</el-button>
                    <el-button v-if="!isLightweight" size="small" plain :disabled="isMigrating(row)" @click="handleEdit(row)">编辑</el-button>
                  </div>
                  <el-dropdown @command="cmd => handleMore(cmd, row)">
                    <el-button size="small" circle icon="MoreFilled" :disabled="isMigrating(row)" />
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item v-if="isAdmin" command="template" :disabled="isMigrating(row)">制作模板</el-dropdown-item>
                        <el-dropdown-item v-if="!isLightweight" command="export" :disabled="isMigrating(row)">导出虚拟机</el-dropdown-item>
                        <el-dropdown-item v-if="!isLightweight" command="network" :disabled="isMigrating(row)">网络管理</el-dropdown-item>
                        <el-dropdown-item v-if="!isLightweight" command="reinstall" :disabled="isMigrating(row)">重装系统</el-dropdown-item>
                        <el-dropdown-item v-if="isAdmin" command="migrate" :disabled="isMigrating(row)">迁移</el-dropdown-item>
                        <el-dropdown-item v-if="isAdmin && row.is_linked_clone" command="make_independent" divided :disabled="isMigrating(row) || row.status !== 'shut off'">转为独立虚拟机</el-dropdown-item>
                        <el-dropdown-item command="snapshot" :disabled="isMigrating(row)">快照管理</el-dropdown-item>
                        <el-dropdown-item divided :command="row.in_rescue ? 'rescue_stop' : 'rescue_start'" :disabled="isMigrating(row)">
                          <span :style="{ color: row.in_rescue ? '#E6A23C' : '#409EFF' }">{{ row.in_rescue ? '关闭救援系统' : '启动救援系统' }}</span>
                        </el-dropdown-item>
                        <el-dropdown-item v-if="!isLightweight" command="delete" divided style="color: red;" :disabled="isMigrating(row) || row.locked">删除</el-dropdown-item>
                        <el-dropdown-item v-if="!isLightweight" divided :command="row.locked ? 'unlock' : 'lock'">
                          <span :style="{ color: row.locked ? '#E6A23C' : '#409EFF' }">{{ row.locked ? '解除锁定' : '锁定虚拟机' }}</span>
                        </el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </div>
            </div>
            <div v-if="filteredTableData.length > pageSize" class="pagination-wrap">
              <el-pagination
                background
                layout="total, prev, pager, next"
                :total="filteredTableData.length"
                :page-size="pageSize"
                :current-page="currentPage"
                @current-change="handlePageChange"
              />
            </div>
          </div>

          <!-- ===== 分组移动端卡片视图 ===== -->
          <div v-if="vmGroupBy" class="mobile-grouped-cards">
            <div
              v-for="group in groupList"
              :key="group.key"
              class="group-section"
              :class="{ 'group-collapsed': !groupExpandedMap[group.key] }"
            >
              <div class="group-header" @click="toggleGroupExpand(group.key)">
                <div class="group-header-left">
                  <el-icon class="group-expand-icon" :class="{ 'is-expanded': groupExpandedMap[group.key] }">
                    <ArrowRight />
                  </el-icon>
                  <span class="group-name">
                    <el-tag :type="groupTagType(group.key)" size="small" effect="plain">{{ group.label }}</el-tag>
                  </span>
                  <span class="group-count">{{ group.vms.length }} 台</span>
                </div>
              </div>
              <div v-show="groupExpandedMap[group.key]" class="group-body">
                <div class="mobile-card-list">
                  <el-card v-for="row in group.vms" :key="row.name" class="vm-mobile-card" :class="{ 'vm-card-migrating': isMigrating(row) }" shadow="hover">
                    <div class="vm-card-header">
                      <div class="vm-card-name-row">
                        <el-link type="primary" :underline="false" @click="$router.push(`/vm/detail/${row.name}`)">{{ row.name }}</el-link>
                        <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
                      </div>
                      <div class="vm-card-specs">
                        <span class="vm-card-spec">{{ row.vcpu }} 核</span>
                        <span class="vm-card-spec">{{ row.memory >= 1024 ? (row.memory / 1024).toFixed(1) + ' GB' : row.memory + ' MB' }}</span>
                        <span class="vm-card-spec">磁盘 {{ row.disk_size || '-' }}</span>
                      </div>
                    </div>
                    <div class="vm-card-body">
                      <div class="vm-card-info-row">
                        <span class="vm-card-label">IP 地址</span>
                        <span class="vm-card-value">
                          <template v-if="hasLoadedIP(row.name)">
                            <el-tooltip v-if="ipTooltipText(row.name)" :content="ipTooltipText(row.name)" placement="top" effect="dark">
                              <span class="ip-unreachable">{{ ipDisplayText(row.name) }}</span>
                            </el-tooltip>
                            <span v-else>{{ ipDisplayText(row.name) }}</span>
                          </template>
                          <el-button v-else link type="primary" size="small" :loading="ipLoadingMap[row.name]" @click="loadVmIP(row)">点击加载</el-button>
                        </span>
                      </div>
                      <div class="vm-card-info-row">
                        <span class="vm-card-label">磁盘占用</span>
                        <span class="vm-card-value">
                          <template v-if="!row.disk_size">-</template>
                          <template v-else-if="!diskUsageMap[row.name]"><el-button link type="primary" size="small" :loading="diskLoadingMap[row.name]" @click="loadVmDiskUsage(row)">查看</el-button></template>
                          <template v-else>已用 {{ diskUsageMap[row.name].used_gb }}</template>
                        </span>
                      </div>
                      <div v-if="row.status === 'running' && row.cpu_percent >= 0 && row.mem_percent >= 0" class="vm-card-resource">
                        <div class="vm-card-resource-item">
                          <span class="vm-card-label">CPU</span>
                          <el-progress :percentage="Math.min(row.cpu_percent, 100)" :stroke-width="8" :color="progressColor(row.cpu_percent)" :show-text="true">
                            <template #default="{ percentage }"><span class="progress-text">{{ row.cpu_percent.toFixed(1) }}%</span></template>
                          </el-progress>
                        </div>
                        <div class="vm-card-resource-item">
                          <span class="vm-card-label">内存</span>
                          <el-progress :percentage="Math.min(row.mem_percent, 100)" :stroke-width="8" :color="progressColor(row.mem_percent)" :show-text="true">
                            <template #default="{ percentage }"><span class="progress-text">{{ row.mem_percent.toFixed(1) }}%</span></template>
                          </el-progress>
                        </div>
                      </div>
                      <div class="vm-card-info-row">
                        <span class="vm-card-label">连续运行</span>
                        <span class="vm-card-value">{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
                      </div>
                      <div v-if="row.remark || row.group" class="vm-card-info-row">
                      <span class="vm-card-label">备注/分组</span>
                      <div class="vm-card-value" style="display: flex; flex-wrap: wrap; gap: 4px; align-items: center; justify-content: flex-end;">
                        <el-tag v-if="row.group" size="small" type="warning" effect="plain">{{ row.group }}</el-tag>
                        <span class="vm-card-remark" v-if="row.remark">{{ row.remark }}</span>
                      </div>
                    </div>
                      <div class="vm-card-info-row">
                        <span class="vm-card-label">模板</span>
                        <span class="vm-card-value">{{ formatTemplateValue(null, null, row.template, null) }}</span>
                      </div>
                      <div class="vm-card-info-row">
                        <span class="vm-card-label">创建时间</span>
                        <span class="vm-card-value">{{ row.created_at }}</span>
                      </div>
                    </div>
                    <div class="vm-card-actions">
                      <el-button size="small" :type="row.status === 'running' ? 'warning' : 'success'" :loading="operatingMap[row.name]" :disabled="isMigrating(row)" @click="handleOperate(row, row.status === 'running' ? 'shutdown' : 'start')">{{ row.status === 'running' ? '关机' : row.status === 'paused' ? '继续' : '开机' }}</el-button>
                      <el-button v-if="row.status === 'paused'" size="small" type="danger" :loading="operatingMap[row.name]" :disabled="isMigrating(row)" @click="handleOperate(row, 'reset')">重置</el-button>
                      <el-button v-if="!isLightweight" size="small" type="primary" plain :disabled="isMigrating(row)" @click="handleEdit(row)">编辑</el-button>
                      <el-button size="small" type="success" plain @click="$router.push(`/vm/detail/${row.name}`)">详情</el-button>
                      <el-dropdown @command="cmd => handleMore(cmd, row)">
                        <el-button size="small" circle icon="MoreFilled" />
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item v-if="isAdmin" command="template" :disabled="isMigrating(row)">制作模板</el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" command="export" :disabled="isMigrating(row)">导出虚拟机</el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" command="network" :disabled="isMigrating(row)">网络管理</el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" command="reinstall" :disabled="isMigrating(row)">重装系统</el-dropdown-item>
                            <el-dropdown-item v-if="isAdmin" command="migrate" :disabled="isMigrating(row)">迁移</el-dropdown-item>
                            <el-dropdown-item v-if="isAdmin && row.is_linked_clone" command="make_independent" divided :disabled="isMigrating(row) || row.status !== 'shut off'">转为独立虚拟机</el-dropdown-item>
                            <el-dropdown-item command="snapshot" :disabled="isMigrating(row)">快照管理</el-dropdown-item>
                            <el-dropdown-item divided :command="row.in_rescue ? 'rescue_stop' : 'rescue_start'" :disabled="isMigrating(row)">
                              <span :style="{ color: row.in_rescue ? '#E6A23C' : '#409EFF' }">{{ row.in_rescue ? '关闭救援系统' : '启动救援系统' }}</span>
                            </el-dropdown-item>
                            <el-dropdown-item v-if="!isLightweight" command="delete" divided style="color: red;" :disabled="isMigrating(row)">删除</el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                    </div>
                  </el-card>
                </div>
              </div>
            </div>
          </div>

          <div v-show="!vmGroupBy && viewMode === 'list'" class="table-scroll-wrap">
          <el-table 
            :data="paginatedTableData" 
            border 
            style="width: 100%" 
            v-loading="loading" 
            @selection-change="handleSelectionChange"
            :default-sort="{ prop: 'created_at', order: 'descending' }"
          >
        <el-table-column type="selection" width="55" align="center" />
        <el-table-column label="名称">
          <template #default="{ row }">
            <el-link type="primary" :underline="false" @click="$router.push(`/vm/detail/${row.name}`)">
              {{ row.name }}
            </el-link>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100" align="center" sortable :sort-method="sortStatus">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="vcpu" label="CPU" width="70" align="center" />
        <el-table-column label="内存" width="90" align="center">
          <template #default="{ row }">
            {{ row.memory >= 1024 ? (row.memory / 1024).toFixed(1) + ' GB' : row.memory + ' MB' }}
          </template>
        </el-table-column>
        <el-table-column label="IP 地址" width="150">
          <template #default="{ row }">
            <div class="disk-cell">
              <el-tooltip v-if="hasLoadedIP(row.name) && ipTooltipText(row.name)" :content="ipTooltipText(row.name)" placement="top" effect="dark">
                <span class="ip-unreachable">{{ ipDisplayText(row.name) }}</span>
              </el-tooltip>
              <span v-else-if="hasLoadedIP(row.name)">{{ ipDisplayText(row.name) }}</span>
              <el-button
                v-else
                link
                type="primary"
                :loading="ipLoadingMap[row.name]"
                @click="loadVmIP(row)"
              >
                点击加载
              </el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="磁盘" width="170" align="center">
          <template #default="{ row }">
            <div class="disk-cell">
              <span>{{ row.disk_size || '-' }}</span>
              <el-button
                v-if="row.disk_size && !diskUsageMap[row.name]"
                link
                type="primary"
                :loading="diskLoadingMap[row.name]"
                @click="loadVmDiskUsage(row)"
              >
                查看占用
              </el-button>
              <span v-else-if="diskUsageMap[row.name]" class="disk-used-text">
                已用 {{ diskUsageMap[row.name].used_gb }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="备注/分组" min-width="220">
          <template #default="{ row }">
            <div class="remark-cell">
              <div style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap;">
                <el-tag v-if="row.group" size="small" type="warning" effect="plain" style="max-width: 100px; overflow: hidden; text-overflow: ellipsis;">{{ row.group }}</el-tag>
                <span class="remark-text" :title="row.remark || ''">{{ row.remark || '-' }}</span>
              </div>
              <div style="display: flex; gap: 4px;">
                <el-button
                  v-if="!isLightweight"
                  link
                  type="warning"
                  size="small"
                  :disabled="isMigrating(row)"
                  @click.stop="handleEditGroup(row)"
                >
                  分组
                </el-button>
                <el-button
                  v-if="!isLightweight"
                  link
                  type="primary"
                  size="small"
                  :disabled="isMigrating(row)"
                  @click.stop="handleEditRemark(row)"
                >
                  备注
                </el-button>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="资源使用" width="160" align="center">
          <template #default="{ row }">
            <template v-if="row.status === 'running' && row.cpu_percent >= 0 && row.mem_percent >= 0">
              <div style="display: flex; flex-direction: column; gap: 4px;">
                <el-tooltip content="CPU使用率" placement="left">
                  <el-progress :percentage="Math.min(row.cpu_percent, 100)" :stroke-width="14" :text-inside="true"
                    :color="progressColor(row.cpu_percent)" 
                    :format="() => 'CPU ' + row.cpu_percent.toFixed(1) + '%'" />
                </el-tooltip>
                <el-tooltip content="内存使用率" placement="left">
                  <el-progress :percentage="Math.min(row.mem_percent, 100)" :stroke-width="14" :text-inside="true"
                    :color="progressColor(row.mem_percent)"
                    :format="() => 'MEM ' + row.mem_percent.toFixed(1) + '%'" />
                </el-tooltip>
              </div>
            </template>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="连续运行" width="130" align="center">
          <template #default="{ row }">
            <el-tooltip
              v-if="row.continuous_running_since"
              :content="`开始时间：${row.continuous_running_since}`"
              placement="top"
            >
              <span>{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
            </el-tooltip>
            <span v-else>{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="template" label="模板" width="120" show-overflow-tooltip :formatter="formatTemplateValue" />
        <el-table-column prop="created_at" label="创建时间" min-width="170" align="center" sortable />
        <el-table-column label="操作" width="250" fixed="right" align="center">
          <template #default="{ row }">
            <el-tooltip :content="row.status === 'running' ? '关机' : row.status === 'paused' ? '继续启动' : '开机'" placement="top">
              <el-button 
                size="small" 
                circle
                class="operate-btn"
                :type="row.status === 'running' ? 'warning' : 'success'" 
                :loading="operatingMap[row.name]"
                :icon="row.status === 'running' ? 'SwitchButton' : 'VideoPlay'"
                :disabled="isMigrating(row)"
                @click="handleOperate(row, row.status === 'running' ? 'shutdown' : 'start')"
              />
            </el-tooltip>
            <el-tooltip v-if="row.status === 'paused'" content="重置" placement="top">
              <el-button
                size="small"
                circle
                class="operate-btn"
                type="danger"
                :loading="operatingMap[row.name]"
                icon="RefreshRight"
                :disabled="isMigrating(row)"
                @click="handleOperate(row, 'reset')"
              />
            </el-tooltip>
            <el-tooltip v-if="!isLightweight" content="编辑" placement="top">
              <el-button size="small" type="primary" plain circle icon="Edit" :disabled="isMigrating(row)" @click="handleEdit(row)" />
            </el-tooltip>
            <el-tooltip content="详情" placement="top">
              <el-button size="small" type="success" plain circle icon="View" @click="$router.push(`/vm/detail/${row.name}`)" />
            </el-tooltip>
            <el-dropdown style="margin-left: 12px;" @command="cmd => handleMore(cmd, row)">
              <span class="el-dropdown-link">
                <el-tooltip content="更多" placement="top">
                  <el-button size="small" circle icon="MoreFilled" />
                </el-tooltip>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-if="isAdmin" command="template" :disabled="isMigrating(row)">制作模板</el-dropdown-item>
                  <el-dropdown-item v-if="!isLightweight" command="export" :disabled="isMigrating(row)">导出虚拟机</el-dropdown-item>
                  <el-dropdown-item v-if="!isLightweight" command="network" :disabled="isMigrating(row)">网络管理</el-dropdown-item>
                  <el-dropdown-item v-if="!isLightweight" command="reinstall" :disabled="isMigrating(row)">重装系统</el-dropdown-item>
                  <el-dropdown-item v-if="isAdmin" command="migrate" :disabled="isMigrating(row)">迁移</el-dropdown-item>
                  <el-dropdown-item v-if="isAdmin && row.is_linked_clone" command="make_independent" divided :disabled="isMigrating(row) || row.status !== 'shut off'">转为独立虚拟机</el-dropdown-item>
                  <el-dropdown-item command="snapshot" :disabled="isMigrating(row)">快照管理</el-dropdown-item>
                  <el-dropdown-item divided :command="row.in_rescue ? 'rescue_stop' : 'rescue_start'" :disabled="isMigrating(row)">
                    <span :style="{ color: row.in_rescue ? '#E6A23C' : '#409EFF' }">
                      {{ row.in_rescue ? '关闭救援系统' : '启动救援系统' }}
                    </span>
                  </el-dropdown-item>
                  <el-dropdown-item v-if="!isLightweight" command="delete" divided style="color: red;" :disabled="isMigrating(row)">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
          </el-table>
          </div><!-- /table-scroll-wrap -->

          <div v-if="!vmGroupBy && viewMode === 'list'" class="pagination-wrap">
            <el-pagination
              v-if="filteredTableData.length > pageSize"
              background
              layout="total, prev, pager, next"
              :total="filteredTableData.length"
              :page-size="pageSize"
              :current-page="currentPage"
              @current-change="handlePageChange"
            />
          </div>

          <!-- ===== 移动端卡片视图 ===== -->
          <div v-if="!vmGroupBy" class="mobile-card-list">
            <el-card
              v-for="row in paginatedTableData"
              :key="row.name"
              class="vm-mobile-card"
              :class="{ 'vm-card-migrating': isMigrating(row) }"
              shadow="hover"
            >
              <div class="vm-card-header">
                <div class="vm-card-name-row">
                  <el-link type="primary" :underline="false" @click="$router.push(`/vm/detail/${row.name}`)">
                    {{ row.name }}
                  </el-link>
                  <el-tag :type="statusType(row.status)" size="small">
                    {{ statusText(row.status) }}
                  </el-tag>
                </div>
                <div class="vm-card-specs">
                  <span class="vm-card-spec">{{ row.vcpu }} 核</span>
                  <span class="vm-card-spec">{{ row.memory >= 1024 ? (row.memory / 1024).toFixed(1) + ' GB' : row.memory + ' MB' }}</span>
                  <span class="vm-card-spec">磁盘 {{ row.disk_size || '-' }}</span>
                </div>
              </div>
              <div class="vm-card-body">
                <div class="vm-card-info-row">
                  <span class="vm-card-label">IP 地址</span>
                  <span class="vm-card-value">
                    <template v-if="hasLoadedIP(row.name)">
                      <el-tooltip v-if="ipTooltipText(row.name)" :content="ipTooltipText(row.name)" placement="top" effect="dark">
                        <span class="ip-unreachable">{{ ipDisplayText(row.name) }}</span>
                      </el-tooltip>
                      <span v-else>{{ ipDisplayText(row.name) }}</span>
                    </template>
                    <el-button v-else link type="primary" size="small" :loading="ipLoadingMap[row.name]" @click="loadVmIP(row)">点击加载</el-button>
                  </span>
                </div>
                <div class="vm-card-info-row">
                  <span class="vm-card-label">磁盘占用</span>
                  <span class="vm-card-value">
                    <template v-if="!row.disk_size">-</template>
                    <template v-else-if="!diskUsageMap[row.name]">
                      <el-button link type="primary" size="small" :loading="diskLoadingMap[row.name]" @click="loadVmDiskUsage(row)">查看</el-button>
                    </template>
                    <template v-else>已用 {{ diskUsageMap[row.name].used_gb }}</template>
                  </span>
                </div>
                <div v-if="row.status === 'running' && row.cpu_percent >= 0 && row.mem_percent >= 0" class="vm-card-resource">
                  <div class="vm-card-resource-item">
                    <span class="vm-card-label">CPU</span>
                    <el-progress :percentage="Math.min(row.cpu_percent, 100)" :stroke-width="8" :color="progressColor(row.cpu_percent)" :show-text="true">
                      <template #default="{ percentage }"><span class="progress-text">{{ row.cpu_percent.toFixed(1) }}%</span></template>
                    </el-progress>
                  </div>
                  <div class="vm-card-resource-item">
                    <span class="vm-card-label">内存</span>
                    <el-progress :percentage="Math.min(row.mem_percent, 100)" :stroke-width="8" :color="progressColor(row.mem_percent)" :show-text="true">
                      <template #default="{ percentage }"><span class="progress-text">{{ row.mem_percent.toFixed(1) }}%</span></template>
                    </el-progress>
                  </div>
                </div>
                <div class="vm-card-info-row">
                  <span class="vm-card-label">连续运行</span>
                  <span class="vm-card-value">{{ formatContinuousRuntime(row.continuous_runtime_seconds, row.status) }}</span>
                </div>
                <div v-if="row.remark || row.group" class="vm-card-info-row">
                <span class="vm-card-label">备注/分组</span>
                <div class="vm-card-value" style="display: flex; flex-wrap: wrap; gap: 4px; align-items: center; justify-content: flex-end;">
                  <el-tag v-if="row.group" size="small" type="warning" effect="plain">{{ row.group }}</el-tag>
                  <span class="vm-card-remark" v-if="row.remark">{{ row.remark }}</span>
                </div>
              </div>
                <div class="vm-card-info-row">
                  <span class="vm-card-label">模板</span>
                  <span class="vm-card-value">{{ formatTemplateValue(null, null, row.template, null) }}</span>
                </div>
                <div class="vm-card-info-row">
                  <span class="vm-card-label">创建时间</span>
                  <span class="vm-card-value">{{ row.created_at }}</span>
                </div>
              </div>
              <div class="vm-card-actions">
                <el-button size="small" :type="row.status === 'running' ? 'warning' : 'success'" :loading="operatingMap[row.name]" :disabled="isMigrating(row)" @click="handleOperate(row, row.status === 'running' ? 'shutdown' : 'start')">
                  {{ row.status === 'running' ? '关机' : row.status === 'paused' ? '继续' : '开机' }}
                </el-button>
                <el-button v-if="row.status === 'paused'" size="small" type="danger" :loading="operatingMap[row.name]" :disabled="isMigrating(row)" @click="handleOperate(row, 'reset')">重置</el-button>
                <el-button v-if="!isLightweight" size="small" type="primary" plain :disabled="isMigrating(row)" @click="handleEdit(row)">编辑</el-button>
                <el-button size="small" type="success" plain @click="$router.push(`/vm/detail/${row.name}`)">详情</el-button>
                <el-dropdown @command="cmd => handleMore(cmd, row)">
                  <el-button size="small" circle icon="MoreFilled" />
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item v-if="isAdmin" command="template" :disabled="isMigrating(row)">制作模板</el-dropdown-item>
                      <el-dropdown-item v-if="!isLightweight" command="export" :disabled="isMigrating(row)">导出虚拟机</el-dropdown-item>
                      <el-dropdown-item v-if="!isLightweight" command="network" :disabled="isMigrating(row)">网络管理</el-dropdown-item>
                      <el-dropdown-item v-if="!isLightweight" command="reinstall" :disabled="isMigrating(row)">重装系统</el-dropdown-item>
                      <el-dropdown-item v-if="isAdmin" command="migrate" :disabled="isMigrating(row)">迁移</el-dropdown-item>
                      <el-dropdown-item v-if="isAdmin && row.is_linked_clone" command="make_independent" divided :disabled="isMigrating(row) || row.status !== 'shut off'">转为独立虚拟机</el-dropdown-item>
                      <el-dropdown-item command="snapshot" :disabled="isMigrating(row)">快照管理</el-dropdown-item>
                      <el-dropdown-item divided :command="row.in_rescue ? 'rescue_stop' : 'rescue_start'" :disabled="isMigrating(row)">
                        <span :style="{ color: row.in_rescue ? '#E6A23C' : '#409EFF' }">{{ row.in_rescue ? '关闭救援系统' : '启动救援系统' }}</span>
                      </el-dropdown-item>
                      <el-dropdown-item v-if="!isLightweight" command="delete" divided style="color: red;" :disabled="isMigrating(row)">删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </el-card>
          </div>
        </el-card>
      </div>

      <div v-if="showMaintenanceOverlay" class="maintenance-overlay">
        <el-card shadow="hover" class="maintenance-card">
          <h3>当前主机正在处于维护模式</h3>
          <p>虚拟机相关操作暂不可用，请稍后再试。</p>
        </el-card>
      </div>
    </div>

    <!-- 表单组件 -->
    <VmForm ref="vmFormRef" @success="fetchData" />
    <VmRemarkDialog ref="vmRemarkDialogRef" @success="handleRemarkSuccess" />
    <VmGroupDialog ref="vmGroupDialogRef" @success="handleGroupSuccess" />
    <SnapshotManage ref="snapshotRef" />
    <NetworkManage ref="networkRef" />
    <TemplateForm ref="templateFormRef" />
    <VmDeleteDialog ref="vmDeleteRef" @success="fetchData" />
    <VmMigrationDialog ref="migrationRef" @success="fetchData" />
    <VmReinstallDialog ref="reinstallRef" @success="fetchData" />

    <el-dialog title="确认开通服务器" v-model="registrationConfirmVisible" width="560px" append-to-body>
      <template v-if="selectedRegistration">
        <el-alert type="info" :closable="false" style="margin-bottom: 16px;">
          <template #title>
            {{ selectedRegistration.vm_name }}，{{ selectedRegistration.vcpu }}C / {{ selectedRegistration.ram }}GB / {{ selectedRegistration.disk_size }}GB
          </template>
        </el-alert>
        <el-descriptions :column="1" border size="small" style="margin-bottom: 16px;">
          <el-descriptions-item label="模板">{{ selectedRegistration.template || '-' }}</el-descriptions-item>
          <el-descriptions-item label="主机名">{{ selectedRegistration.hostname || '-' }}</el-descriptions-item>
          <el-descriptions-item label="网络配额">{{ formatRegistrationQuota(selectedRegistration) }}</el-descriptions-item>
        </el-descriptions>
        <el-form :model="registrationCredentialForm" label-width="110px">
          <el-form-item label="登录用户名">
            <el-input
              v-model="registrationCredentialForm.username"
              :disabled="isWindowsRegistration(selectedRegistration)"
              placeholder="请输入登录用户名"
            />
            <div v-if="isWindowsRegistration(selectedRegistration)" class="credential-tip">Windows 模板固定使用 administrator。</div>
          </el-form-item>
          <el-form-item label="登录密码">
            <el-input
              v-model="registrationCredentialForm.password"
              type="password"
              show-password
              placeholder="至少 12 位，支持字母、数字和符号"
            >
              <template #append>
                <el-button @click="generateRegistrationPassword">随机强密码</el-button>
              </template>
            </el-input>
          </el-form-item>
        </el-form>
      </template>
      <template #footer>
        <el-button @click="registrationConfirmVisible = false">取消</el-button>
        <el-button type="primary" :loading="registrationConfirmLoading" @click="submitRegistrationConfirm">确认并开通</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, reactive } from 'vue'
import { getDiskList, getVmIP, getVmList, operateVm, rescueVm, lockVm, unlockVm, makeVMIndependent } from '@/api/vm'
import { getSelfVMs, getSelfLightweightVmRegistrations, confirmSelfLightweightVmRegistration } from '@/api/user'
import { getUserInfo } from '@/api/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, ArrowRight, Grid, List as ListIcon } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { useVmStore } from '@/store/vm'
import VmForm from '@/components/VmForm.vue'
import VmRemarkDialog from '@/components/VmRemarkDialog.vue'
import VmGroupDialog from '@/components/VmGroupDialog.vue'
import SnapshotManage from '@/components/SnapshotManage.vue'
import NetworkManage from '@/components/NetworkManage.vue'
import TemplateForm from '@/components/TemplateForm.vue'
import VmDeleteDialog from '@/components/VmDeleteDialog.vue'
import { validatePassword, checkPasswordBreachAsync, generatePassword as genPwd } from '@/utils/validate'
import VmMigrationDialog from '@/components/VmMigrationDialog.vue'
import VmReinstallDialog from '@/components/VmReinstallDialog.vue'
import VmStatusIcons from '@/components/icons/VmStatusIcons.vue'
import { exportVM } from '@/api/storage'

const userStore = useUserStore()
const vmStore = useVmStore()
const isAdmin = computed(() => userStore.role === 'admin')
const isLightweight = computed(() => userStore.role !== 'admin' && userStore.cloudType === 'lightweight')

const tableData = ref([])
const loading = ref(false)
const autoRefresh = ref(false) // 管理员和用户默认都关闭自动刷新
const vmSearchText = ref('')
const vmStatusFilter = ref('')
const vmGroupBy = ref('status')
const viewMode = ref(localStorage.getItem('vmListViewMode') || 'card')
const groupExpandedMap = reactive({})
const groupTableRefs = reactive({})
const operatingMap = ref({}) // 跟踪每台虚拟机的操作状态（加载动画）
const ipAddressMap = ref({}) // 按需加载的 IP 数据
const ipStatusMap = ref({}) // IP 状态（vlan_bridge 等）
const ipLoadingMap = ref({}) // IP 单行加载状态
const diskUsageMap = ref({}) // 按需加载的磁盘占用数据
const diskLoadingMap = ref({}) // 磁盘占用单行加载状态
const vmFormRef = ref(null)
const vmRemarkDialogRef = ref(null)
const vmGroupDialogRef = ref(null)
const snapshotRef = ref(null)
const networkRef = ref(null)
const templateFormRef = ref(null)
const vmDeleteRef = ref(null)
const migrationRef = ref(null)
const reinstallRef = ref(null)
const pendingRegistrations = ref([])
const pendingRegistrationLoading = ref(false)
const registrationConfirmVisible = ref(false)
const registrationConfirmLoading = ref(false)
const selectedRegistration = ref(null)
const registrationCredentialForm = ref({
  username: '',
  password: ''
})

const selectedVms = ref([])
const hasSelection = computed(() => selectedVms.value.length > 0)
const batchOperating = ref(false)

const currentPage = ref(1)
const pageSize = ref(50)

const filteredTableData = computed(() => {
  let data = tableData.value
  if (vmSearchText.value) {
    const q = vmSearchText.value.toLowerCase()
    data = data.filter(vm =>
      vm.name.toLowerCase().includes(q) ||
      (vm.remark || '').toLowerCase().includes(q) ||
      (vm.template || '').toLowerCase().includes(q)
    )
  }
  return data
})

const paginatedTableData = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredTableData.value.slice(start, start + pageSize.value)
})

const handlePageChange = (page) => {
  currentPage.value = page
}

const resetPage = () => {
  currentPage.value = 1
}

// ===== 分组功能 ===== 

// 分组定义：key -> 排序权重
const groupDefsByType = {
  status: [
    { key: 'running', label: '运行中', order: 1, tagType: 'success' },
    { key: 'paused', label: '已暂停', order: 2, tagType: 'warning' },
    { key: 'shut off', label: '已关机', order: 3, tagType: 'info' },
    { key: 'migrating', label: '迁移中', order: 4, tagType: 'danger' },
  ],
  template: null // 动态生成
}

const groupList = computed(() => {
  if (!vmGroupBy.value) return []
  const data = filteredTableData.value
  
  if (vmGroupBy.value === 'status') {
    const defs = groupDefsByType.status
    const grouped = {}
    defs.forEach(d => { grouped[d.key] = { key: d.key, label: d.label, order: d.order, tagType: d.tagType, vms: [] } })
    data.forEach(vm => {
      const key = vm.status || 'shut off'
      if (!grouped[key]) {
        grouped[key] = { key, label: statusText(key), order: 99, tagType: statusType(key), vms: [] }
      }
      grouped[key].vms.push(vm)
    })
    return Object.values(grouped)
      .filter(g => g.vms.length > 0)
      .sort((a, b) => a.order - b.order)
  }
  
  if (vmGroupBy.value === 'template') {
    const grouped = {}
    data.forEach(vm => {
      const key = vm.template || '__none__'
      const label = vm.template || '无模板'
      if (!grouped[key]) {
        grouped[key] = { key, label, tagType: 'info', vms: [] }
      }
      grouped[key].vms.push(vm)
    })
    return Object.values(grouped)
      .filter(g => g.vms.length > 0)
      .sort((a, b) => a.label.localeCompare(b.label))
  }

  if (vmGroupBy.value === 'custom') {
    const grouped = {}
    data.forEach(vm => {
      const key = vm.group || '__ungrouped__'
      const label = vm.group || '未分组'
      if (!grouped[key]) {
        grouped[key] = { key, label, tagType: '', vms: [] }
      }
      grouped[key].vms.push(vm)
    })
    return Object.values(grouped)
      .filter(g => g.vms.length > 0)
      .sort((a, b) => {
        if (a.key === '__ungrouped__') return 1
        if (b.key === '__ungrouped__') return -1
        return a.label.localeCompare(b.label)
      })
  }
  
  return []
})

const toggleGroupExpand = (key) => {
  if (groupExpandedMap[key] === undefined) {
    groupExpandedMap[key] = true
  }
  groupExpandedMap[key] = !groupExpandedMap[key]
}

const groupTagType = (key) => {
  if (vmGroupBy.value === 'status') {
    const def = groupDefsByType.status.find(d => d.key === key)
    return def ? def.tagType : 'info'
  }
  return 'info'
}

// 初始化分组展开状态（默认全部展开）
watch(groupList, (groups) => {
  groups.forEach(g => {
    if (groupExpandedMap[g.key] === undefined) {
      groupExpandedMap[g.key] = true
    }
  })
}, { immediate: true })

// 分组表格引用管理
const setGroupTableRef = (key, el) => {
  if (el) {
    groupTableRefs[key] = el
  }
}

// 分组表格选择变化
const onGroupSelectionChange = (groupKey, val) => {
  // 清除其他组的选中项
  Object.keys(groupTableRefs).forEach(key => {
    if (key !== groupKey && groupTableRefs[key]) {
      groupTableRefs[key].clearSelection()
    }
  })
  selectedVms.value = val
}

const groupAllSelected = (group) => {
  if (group.vms.length === 0) return false
  return group.vms.every(vm => selectedVms.value.some(s => s.name === vm.name))
}

const selectGroupVMs = (group) => {
  if (groupAllSelected(group)) {
    // 取消全选
    if (groupTableRefs[group.key]) {
      groupTableRefs[group.key].clearSelection()
    }
  } else {
    // 全选
    group.vms.forEach(vm => {
      if (groupTableRefs[group.key]) {
        groupTableRefs[group.key].toggleRowSelection(vm, true)
      }
    })
  }
}

// 同步分组模式与批量操作
const handleSelectionChange = (val) => {
  selectedVms.value = val
}

watch(vmGroupBy, () => {
  selectedVms.value = []
  currentPage.value = 1
})

watch(viewMode, (val) => {
  localStorage.setItem('vmListViewMode', val)
})
const showMaintenanceOverlay = computed(() => !!userStore.security?.maintenance_mode)

let eventSource = null

const syncMaintenanceState = async () => {
  try {
    const res = await getUserInfo()
    userStore.setUserInfo(res.data.username, res.data.role, res.data.security || null, res.data.cloud_type || 'elastic')
  } catch (err) {
    console.error(err)
  }
}

const statusType = (status) => {
  const map = { running: 'success', 'shut off': 'info', paused: 'warning', migrating: 'danger' }
  return map[status] || 'info'
}

const statusText = (status) => {
  const map = { running: '运行中', 'shut off': '已关机', paused: '已暂停', migrating: '迁移中' }
  return map[status] || status
}

const sortStatus = (a, b) => {
  const weight = { migrating: 4, running: 3, paused: 2, 'shut off': 1 }
  const aW = weight[a.status] || 0
  const bW = weight[b.status] || 0
  return aW - bW
}

const isMigrating = (row) => row?.status === 'migrating'

const progressColor = (percent) => {
  if (percent >= 90) return '#F56C6C'
  if (percent >= 70) return '#E6A23C'
  return '#67C23A'
}

const formatContinuousRuntime = (seconds, status) => {
  const normalized = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0
  if (normalized <= 0) {
    return status === 'running' || status === 'paused' ? '不足 1 分钟' : '-'
  }

  const days = Math.floor(normalized / 86400)
  const hours = Math.floor((normalized % 86400) / 3600)
  const minutes = Math.floor((normalized % 3600) / 60)

  if (days > 0) {
    return `${days} 天 ${hours} 小时`
  }
  if (hours > 0) {
    return `${hours} 小时 ${minutes} 分钟`
  }
  return `${minutes || 1} 分钟`
}

const registrationStatusText = (status) => {
  const map = {
    pending: '待确认',
    provisioning: '开通中',
    failed: '失败'
  }
  return map[status] || status || '待确认'
}

const registrationStatusType = (status) => {
  const map = {
    pending: 'warning',
    provisioning: 'primary',
    failed: 'danger'
  }
  return map[status] || 'warning'
}

const formatRegistrationQuota = (row) => {
  const traffic = `${row.traffic_down_gb || 0}/${row.traffic_up_gb || 0}GB`
  const bandwidth = `${row.bandwidth_down_mbps || 0}/${row.bandwidth_up_mbps || 0}Mbps`
  const ports = row.max_port_forwards ?? 10
  return `流量 ${traffic}，带宽 ${bandwidth}，端口 ${ports}`
}

const isWindowsRegistration = (row) => row?.template_type === 'windows'

const formatTemplateValue = (_row, _column, value) => value || '-'

const cardStatusClass = (row) => ({
  'card-running': row.status === 'running',
  'card-paused': row.status === 'paused',
  'card-shutoff': row.status === 'shut off',
  'card-migrating': row.status === 'migrating'
})

const formatMemory = (memory) => {
  return memory >= 1024 ? (memory / 1024).toFixed(1) + ' GB' : memory + ' MB'
}

const validateRegistrationPassword = (password) => {
  return validatePassword(password).valid
}

const generateStrongPassword = () => genPwd()

const generateRegistrationPassword = () => {
  registrationCredentialForm.value.password = generateStrongPassword()
}

const fetchPendingRegistrations = async () => {
  if (!isLightweight.value) {
    pendingRegistrations.value = []
    return
  }
  pendingRegistrationLoading.value = true
  try {
    const res = await getSelfLightweightVmRegistrations()
    pendingRegistrations.value = res.data || []
  } catch (err) {
    console.error(err)
  } finally {
    pendingRegistrationLoading.value = false
  }
}

const openRegistrationConfirm = (row) => {
  selectedRegistration.value = row
  registrationCredentialForm.value = {
    username: isWindowsRegistration(row) ? 'administrator' : 'admin',
    password: generateStrongPassword()
  }
  registrationConfirmVisible.value = true
}

const submitRegistrationConfirm = async () => {
  const row = selectedRegistration.value
  if (!row) return
  const username = isWindowsRegistration(row) ? 'administrator' : registrationCredentialForm.value.username.trim()
  if (!username) {
    ElMessage.warning('请填写登录用户名')
    return
  }
  if (!validateRegistrationPassword(registrationCredentialForm.value.password)) {
    ElMessage.warning('密码至少 12 位，只支持字母、数字和 !@#$%^&*_-+=? 符号')
    return
  }
  // 异步泄露密码检测（HIBP API）
  const breach = await checkPasswordBreachAsync(registrationCredentialForm.value.password)
  if (breach.enabled && breach.breached) {
    ElMessage.error('该密码已在已知泄露数据库中发现，请更换为更安全的密码')
    return
  }
  registrationConfirmLoading.value = true
  try {
    const res = await confirmSelfLightweightVmRegistration(row.id, {
      username,
      password: registrationCredentialForm.value.password
    })
    ElMessage.success(res.message || '开通任务已提交，请在任务中心查看进度')
    registrationConfirmVisible.value = false
    fetchPendingRegistrations()
    fetchData()
  } catch (err) {
    console.error(err)
  } finally {
    registrationConfirmLoading.value = false
  }
}

const hasLoadedIP = (vmName) => Object.prototype.hasOwnProperty.call(ipAddressMap.value, vmName)

const ipDisplayText = (vmName) => {
  const ip = ipAddressMap.value[vmName]
  if (ip) return ip
  const status = ipStatusMap.value[vmName]
  if (status) return '无法获取'
  return '-'
}

const ipTooltipText = (vmName) => {
  const ip = ipAddressMap.value[vmName]
  if (ip) return ''
  const status = ipStatusMap.value[vmName]
  if (status === 'vlan_bridge') return '桥接 VLAN 模式下上游路由器分配的 IP 无法从宿主机获取'
  if (status === 'shut_off') return '虚拟机处于关机状态，无法获取 IP'
  return ''
}

const normalizeVmDiskUsage = (disks) => {
  if (!Array.isArray(disks)) {
    return null
  }

  const primaryDisk = disks.find(disk => disk?.device_type !== 'cdrom' && disk?.path)
  if (!primaryDisk || !primaryDisk.used_gb || primaryDisk.used_gb === '-') {
    return null
  }

  return {
    used_gb: `${primaryDisk.used_gb} GB`
  }
}

const syncIPAddressState = (data) => {
  const vmNameSet = new Set((Array.isArray(data) ? data : []).map(vm => vm.name))
  const nextAddressMap = { ...ipAddressMap.value }
  const nextLoadingMap = { ...ipLoadingMap.value }

  Object.keys(nextAddressMap).forEach((name) => {
    if (!vmNameSet.has(name)) {
      delete nextAddressMap[name]
    }
  })

  Object.keys(nextLoadingMap).forEach((name) => {
    if (!vmNameSet.has(name)) {
      delete nextLoadingMap[name]
    }
  })

  ipAddressMap.value = nextAddressMap
  ipLoadingMap.value = nextLoadingMap
}

const syncDiskUsageState = (data) => {
  const vmNameSet = new Set((Array.isArray(data) ? data : []).map(vm => vm.name))
  const nextUsageMap = { ...diskUsageMap.value }
  const nextLoadingMap = { ...diskLoadingMap.value }

  Object.keys(nextUsageMap).forEach((name) => {
    if (!vmNameSet.has(name)) {
      delete nextUsageMap[name]
    }
  })

  Object.keys(nextLoadingMap).forEach((name) => {
    if (!vmNameSet.has(name)) {
      delete nextLoadingMap[name]
    }
  })

  diskUsageMap.value = nextUsageMap
  diskLoadingMap.value = nextLoadingMap
}

// 更新表格数据并同步缓存
const updateTableData = (data) => {
  if (!Array.isArray(data)) return
  // 如果状态发生变化，清除对应 VM 的操作 loading 状态
  data.forEach(vm => {
    const oldVm = tableData.value.find(v => v.name === vm.name)
    if (oldVm && oldVm.status !== vm.status) {
      operatingMap.value[vm.name] = false
    }
    if (oldVm && oldVm.ip !== vm.ip) {
      delete ipAddressMap.value[vm.name]
      delete ipLoadingMap.value[vm.name]
    }
    if (oldVm && oldVm.disk_size !== vm.disk_size) {
      delete diskUsageMap.value[vm.name]
      delete diskLoadingMap.value[vm.name]
    }
  })
  syncIPAddressState(data)
  syncDiskUsageState(data)
  tableData.value = data
  vmStore.setVmList(data) // 同步更新缓存
}

// 统一 SSE 初始化（管理员和用户都使用 SSE）
const initSSE = () => {
  const token = userStore.token
  const baseUrl = import.meta.env.VITE_APP_BASE_API || '/api'
  const query = new URLSearchParams({
    token,
    include_resource_usage: '1',
    include_ip: '0'
  })
  // 管理员使用 /vm/sse，用户使用 /self/vms/sse
  const sseUrl = isAdmin.value
    ? `${baseUrl}/vm/sse?${query.toString()}`
    : `${baseUrl}/self/vms/sse?${query.toString()}`

  if (eventSource) {
    eventSource.close()
  }

  eventSource = new EventSource(sseUrl)

  eventSource.addEventListener('vm_list', (event) => {
    try {
      const data = JSON.parse(event.data)
      if (Array.isArray(data)) {
        updateTableData(data)
        loading.value = false
      }
    } catch (e) {
      console.error('解析 SSE 数据失败', e)
    }
  })

  eventSource.onerror = (error) => {
    console.error('SSE 连接错误:', error)
    if (eventSource) eventSource.close()
    // 5秒后重连
    if (autoRefresh.value) {
      setTimeout(initSSE, 5000)
    }
  }
}

const onAutoRefreshChange = (val) => {
  if (val) {
    initSSE()
  } else {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }
}

const fetchData = async () => {
  loading.value = true
  try {
    let res
    if (isAdmin.value) {
      res = await getVmList({ include_resource_usage: true, include_ip: false })
    } else {
      res = await getSelfVMs({ include_resource_usage: true, include_ip: false })
    }
    if (res && res.data && Array.isArray(res.data)) {
      updateTableData(res.data)
    }
    if (isLightweight.value) {
      fetchPendingRegistrations()
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await syncMaintenanceState()

  // 如果有缓存数据，直接使用缓存（避免从详情页返回时转圈 loading）
  if (vmStore.hasCachedData) {
    tableData.value = vmStore.vmList
    // 后台静默更新最新数据（不显示 loading）
    fetchDataSilently()
  } else {
    // 首次加载或缓存过期，带 loading 加载
    fetchData()
  }

  if (autoRefresh.value) {
    initSSE()
  }
})

// 静默获取数据（不显示 loading，用于有缓存时的后台更新）
const fetchDataSilently = async () => {
  try {
    let res
    if (isAdmin.value) {
      res = await getVmList({ include_resource_usage: true, include_ip: false })
    } else {
      res = await getSelfVMs({ include_resource_usage: true, include_ip: false })
    }
    if (res && res.data && Array.isArray(res.data)) {
      updateTableData(res.data)
    }
    if (isLightweight.value) {
      fetchPendingRegistrations()
    }
  } catch (err) {
    console.error(err)
  }
}

watch(vmSearchText, () => {
  currentPage.value = 1
})

onUnmounted(() => {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
})

const handleCreate = () => {
  if (isLightweight.value) return
  vmFormRef.value?.open()
}
const handleEdit = (row) => {
  if (isLightweight.value) return
  vmFormRef.value?.open(row)
}

const handleEditRemark = (row) => {
  if (isLightweight.value || isMigrating(row)) return
  vmRemarkDialogRef.value?.open(row.name, row.remark || '')
}

const handleReinstall = (row) => {
  if (isLightweight.value || isMigrating(row)) return
  reinstallRef.value?.open(row)
}

const handleRemarkSuccess = ({ name, remark }) => {
  const nextData = tableData.value.map(vm => (
    vm.name === name
      ? { ...vm, remark: remark || '' }
      : vm
  ))
  updateTableData(nextData)
}

const handleEditGroup = (row) => {
  if (isLightweight.value || isMigrating(row)) return
  const allGroups = [...new Set(tableData.value.map(vm => vm.group).filter(Boolean))]
  vmGroupDialogRef.value?.open(row.name, row.group || '', allGroups)
}

const handleGroupSuccess = ({ name, group }) => {
  const nextData = tableData.value.map(vm => (
    vm.name === name
      ? { ...vm, group: group || '' }
      : vm
  ))
  updateTableData(nextData)
}

const loadVmIP = async (row) => {
  if (!row?.name || ipLoadingMap.value[row.name]) {
    return
  }

  ipLoadingMap.value = {
    ...ipLoadingMap.value,
    [row.name]: true
  }

  try {
    const res = await getVmIP(row.name)
    ipAddressMap.value = {
      ...ipAddressMap.value,
      [row.name]: res?.data?.ip || ''
    }
    ipStatusMap.value = {
      ...ipStatusMap.value,
      [row.name]: res?.data?.ip_status || ''
    }
  } catch (err) {
    console.error('加载虚拟机 IP 失败', err)
  } finally {
    ipLoadingMap.value = {
      ...ipLoadingMap.value,
      [row.name]: false
    }
  }
}

const loadVmDiskUsage = async (row) => {
  if (!row?.name || !row.disk_size || diskLoadingMap.value[row.name]) {
    return
  }

  diskLoadingMap.value = {
    ...diskLoadingMap.value,
    [row.name]: true
  }

  try {
    const res = await getDiskList(row.name)
    const diskUsage = normalizeVmDiskUsage(res?.data)
    if (diskUsage) {
      diskUsageMap.value = {
        ...diskUsageMap.value,
        [row.name]: diskUsage
      }
    }
  } catch (err) {
    console.error('加载虚拟机磁盘占用失败', err)
  } finally {
    diskLoadingMap.value = {
      ...diskLoadingMap.value,
      [row.name]: false
    }
  }
}

const handleOperate = async (row, action) => {
  if (isMigrating(row)) {
    ElMessage.warning('虚拟机正在迁移中，暂不能执行操作')
    return
  }
  try {
    const actionText = action === 'start'
      ? (row.status === 'paused' ? '继续启动' : '开机')
      : { shutdown: '关机', destroy: '强制断电', reboot: '重启', reset: '重置' }[action]
    if (row.locked && (action === 'shutdown' || action === 'destroy')) {
      await ElMessageBox.confirm(
        `⚠️ 虚拟机「${row.name}」已锁定\n\n该操作可能影响正在运行的服务，确定要继续执行${actionText}操作吗？`,
        '虚拟机已锁定 - 二次确认',
        {
          type: 'error',
          confirmButtonText: '确认关机',
          cancelButtonText: '取消',
          confirmButtonClass: 'el-button--danger'
        }
      )
    } else {
      await ElMessageBox.confirm(`确定要对 ${row.name} 执行${actionText}操作吗?`, '提示', { type: 'warning' })
    }
    operatingMap.value[row.name] = true
    await operateVm(row.name, action)
    ElMessage.success(`${actionText}指令已下发`)
  } catch (e) {
    operatingMap.value[row.name] = false
  }
}

const handleBatchOperate = async (action) => {
  if (!hasSelection.value) return
  if (selectedVms.value.some(vm => vm.status === 'migrating')) {
    ElMessage.warning('选中的虚拟机包含迁移中状态，请先取消选择后再操作')
    return
  }
  // 批量关机/断电时检查是否有锁定的虚拟机，有则二次确认
  if ((action === 'shutdown' || action === 'destroy') && selectedVms.value.some(vm => vm.locked)) {
    const lockedNames = selectedVms.value.filter(vm => vm.locked).map(v => v.name).join(', ')
    const actionText = { shutdown: '关机', destroy: '强制断电' }[action]
    try {
      await ElMessageBox.confirm(
        `⚠️ 选中的虚拟机中包含已锁定的虚拟机（${lockedNames}）\n\n对已锁定虚拟机执行${actionText}操作可能影响正在运行的服务，确定要继续吗？`,
        '虚拟机已锁定 - 批量操作二次确认',
        {
          type: 'error',
          confirmButtonText: '确认执行',
          cancelButtonText: '取消',
          confirmButtonClass: 'el-button--danger'
        }
      )
    } catch {
      return
    }
  }
  const actionText = { start: '开机', shutdown: '关机', destroy: '强制断电', reboot: '重启', reset: '重置' }[action]
  try {
    await ElMessageBox.confirm(`确定要对选中的 ${selectedVms.value.length} 台虚拟机执行${actionText}操作吗?`, '批量操作提示', { type: 'warning' })
    batchOperating.value = true
    const promises = selectedVms.value.map(vm => operateVm(vm.name, action))
    const results = await Promise.allSettled(promises)
    const successCount = results.filter(r => r.status === 'fulfilled').length
    const failCount = results.filter(r => r.status === 'rejected').length
    
    ElMessage({
      type: failCount === 0 ? 'success' : 'warning',
      message: `批量${actionText}完成。成功: ${successCount}, 失败: ${failCount}`
    })
    
    fetchData()
  } catch (e) {
    // cancelled
  } finally {
    batchOperating.value = false
  }
}

const handleBatchDelete = () => {
  if (isLightweight.value) return
  if (!hasSelection.value) return
  if (selectedVms.value.some(vm => vm.status === 'migrating')) {
    ElMessage.warning('选中的虚拟机包含迁移中状态，请先取消选择后再删除')
    return
  }
  // 检查是否有锁定的虚拟机
  if (selectedVms.value.some(vm => vm.locked)) {
    const lockedNames = selectedVms.value.filter(vm => vm.locked).map(v => v.name).join(', ')
    ElMessage.warning(`选中的虚拟机中包含已锁定的虚拟机（${lockedNames}），请先解锁后再删除`)
    return
  }
  vmDeleteRef.value?.openBatch(selectedVms.value)
}

const handleMore = async (command, row) => {
  if (isMigrating(row)) {
    ElMessage.warning('虚拟机正在迁移中，暂不能执行操作')
    return
  }
  if (isLightweight.value && ['delete', 'export', 'network', 'template', 'reinstall', 'lock', 'unlock', 'make_independent'].includes(command)) {
    return
  }
  if (command === 'delete') {
    vmDeleteRef.value?.open(row.name)
  } else if (command === 'snapshot') {
    snapshotRef.value?.open(row.name, row.status)
  } else if (command === 'network') {
    networkRef.value?.open(row.name)
  } else if (command === 'reinstall') {
    handleReinstall(row)
  } else if (command === 'template') {
    templateFormRef.value?.open(row.name)
  } else if (command === 'migrate') {
    migrationRef.value?.open(row)
  } else if (command === 'export') {
    try {
      let confirmMsg = `确定要导出虚拟机 "${row.name}" 的磁盘到我的存储吗？\n导出过程可能需要较长时间，请在任务中心查看进度。`
      // 非管理员用户提醒配额限制
      if (!isAdmin.value) {
        confirmMsg += `\n\n⚠️ 注意：导出的磁盘文件将占用您的存储配额。如果导出过程中配额不足，系统将自动中止导出并清理不完整的文件。`
      }
      await ElMessageBox.confirm(
        confirmMsg,
        '导出虚拟机',
        { type: 'warning' }
      )
      const res = await exportVM({ vm_name: row.name })
      ElMessage.success(res.message || '导出任务已提交')
    } catch {
      // 取消确认或 API 错误（错误提示由 axios 拦截器统一处理）
    }
  } else if (command === 'rescue_start' || command === 'rescue_stop') {
    const isStart = command === 'rescue_start'
    const actionText = isStart ? '启动救援系统' : '关闭救援系统'
    const confirmMsg = isStart
      ? `确定要为虚拟机 "${row.name}" 启动救援系统吗？\n\nℹ️ 操作说明：\n1. 虚拟机将被强制关机\n2. 磁盘和网卡将切换为兼容模式\n3. 挂载救援ISO并重新开机\n4. 请在任务中心查看进度`
      : `确定要关闭虚拟机 "${row.name}" 的救援系统吗？\n\nℹ️ 操作说明：\n1. 虚拟机将被强制关机\n2. 卸载救援ISO并恢复原始配置\n3. 虚拟机将重新开机\n4. 请在任务中心查看进度`
    try {
      await ElMessageBox.confirm(confirmMsg, actionText, { type: 'warning' })
      const res = await rescueVm(row.name, isStart ? 'start' : 'stop')
      ElMessage.success(res.message || `${actionText}任务已提交`)
    } catch {
      // 取消确认或 API 错误
    }
  } else if (command === 'lock') {
    try {
      await ElMessageBox.confirm(
        `确定要锁定虚拟机 "${row.name}" 吗？\n锁定后虚拟机将无法删除，关机需二次确认。`,
        '锁定虚拟机',
        { type: 'warning' }
      )
      await lockVm(row.name)
      ElMessage.success('虚拟机已锁定')
      row.locked = true
    } catch {
      // 取消确认或 API 错误
    }
  } else if (command === 'unlock') {
    try {
      await ElMessageBox.confirm(
        `确定要解除虚拟机 "${row.name}" 的锁定吗？\n解除锁定需要进行二次验证。`,
        '解除锁定',
        { type: 'warning' }
      )
      await unlockVm(row.name)
      ElMessage.success('虚拟机已解锁')
      row.locked = false
    } catch {
      // 取消确认或 API 错误（428 二次验证由拦截器处理）
    }
  } else if (command === 'make_independent') {
    try {
      await ElMessageBox.confirm(
        `确定要将虚拟机 "${row.name}" 转为独立虚拟机吗？\n\n⚠️ 操作说明：\n1. 虚拟机必须处于关机状态\n2. 将通过 qemu-img convert 将模板 backing chain 合并为独立磁盘镜像\n3. 操作完成后虚拟机将脱离链式克隆关系，不再依赖原始模板\n4. 此操作需要较长时间，请在任务中心查看进度`,
        '转为独立虚拟机',
        {
          confirmButtonText: '确定转换',
          cancelButtonText: '取消',
          type: 'warning',
        }
      )
      const res = await makeVMIndependent(row.name)
      ElMessage.success(res.message || '转为独立虚拟机任务已提交')
      row.is_linked_clone = false
    } catch {
      // 取消确认或 API 错误
    }
  }
}
</script>

<style scoped>
.vm-list-container {
  padding: 10px;
}
.maintenance-shell {
  position: relative;
}
.maintenance-content {
  transition: filter 0.25s ease, opacity 0.25s ease;
}
.content-blurred {
  filter: blur(8px);
  opacity: 0.5;
  pointer-events: none;
  user-select: none;
}
.maintenance-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  z-index: 10;
}
.maintenance-card {
  width: min(460px, 100%);
  text-align: center;
  border: 1px solid var(--el-border-color);
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.16);
}
.maintenance-card h3 {
  margin: 0 0 12px;
  font-size: 22px;
  color: var(--el-text-color-primary);
}
.maintenance-card p {
  margin: 0;
  font-size: 14px;
  line-height: 1.8;
  color: var(--el-text-color-regular);
}
h2 {
  margin: 0;
  font-size: 18px;
  color: var(--el-text-color-primary);
}
.vm-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.operate-btn {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.disk-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: center;
}
.disk-used-text {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.remark-cell {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}
.remark-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.remark-cell > div:first-child {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}
.pending-registration-panel {
  margin-bottom: 18px;
  padding: 16px;
  border: 1px solid #f3d19e;
  border-radius: 10px;
  background: linear-gradient(135deg, #fff8ea 0%, #f7fbff 100%);
}
.pending-registration-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  margin-bottom: 14px;
}
.pending-registration-header h3 {
  margin: 0 0 6px;
  font-size: 16px;
  color: #7a4b00;
}
.pending-registration-header p {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.pending-registration-card {
  margin-bottom: 14px;
}
.pending-registration-title {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
  font-weight: 600;
  margin-bottom: 10px;
}
.pending-registration-meta {
  color: var(--el-text-color-regular);
  font-size: 13px;
  line-height: 1.8;
}
.pending-registration-error {
  margin-top: 8px;
  color: var(--el-color-danger);
  font-size: 12px;
  line-height: 1.5;
}
.credential-tip {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-top: 4px;
}

.vm-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.vm-header-left {
  flex-shrink: 0;
}
.vm-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  white-space: nowrap;
}
.vm-header-center {
  flex: 1;
  display: flex;
  justify-content: center;
  max-width: 500px;
  margin: 0 auto;
}
.vm-search-input {
  width: 100%;
}
.vm-header-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  flex-shrink: 0;
}
.vm-actions-primary {
  display: flex;
  align-items: center;
  gap: 10px;
}
.vm-actions-batch {
  display: flex;
  align-items: center;
  gap: 8px;
}
.auto-refresh-switch {
  margin-right: 5px;
}
.vm-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.vm-toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}
.vm-toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.view-mode-toggle {
  display: flex;
  align-items: center;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
}
.view-mode-toggle .el-button {
  border: none;
  border-radius: 0;
  margin: 0;
}
.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
.table-scroll-wrap {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  background: var(--el-bg-color);
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
  padding: 2px;
}

/* ===== 移动端适配 ===== */
.mobile-card-list {
  display: none;
}

@media (max-width: 768px) {
  .vm-list-container {
    padding: 4px;
  }

  h2 {
    font-size: 16px;
  }

  .vm-header {
    flex-wrap: wrap;
    gap: 12px;
  }

  .vm-header-center {
    order: 3;
    max-width: none;
    width: 100%;
  }

  .vm-header-right {
    order: 2;
  }

  .vm-title {
    font-size: 18px;
  }

  .vm-toolbar {
    flex-wrap: wrap;
    gap: 12px;
  }

  .vm-toolbar-left {
    flex-wrap: wrap;
    gap: 8px;
  }

  .vm-toolbar-right {
    width: 100%;
    justify-content: flex-end;
  }

  /* 隐藏表格，显示卡片 */
  .table-scroll-wrap {
    display: none !important;
  }

  /* 隐藏PC端卡片视图，使用移动端卡片 */
  .grouped-card-view,
  .all-card-view {
    display: none !important;
  }

  .mobile-card-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin: 0 -6px;
    padding: 0 6px;
  }

  .maintenance-card h3 {
    font-size: 17px;
  }

  .maintenance-card p {
    font-size: 12px;
  }

  .pending-registration-header {
    flex-direction: column;
  }

  .remark-cell {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .disk-cell {
    gap: 2px;
  }
}

/* ===== 虚拟机移动卡片样式 ===== */
.vm-mobile-card {
  border-radius: 10px;
  transition: box-shadow 0.2s;
}

.vm-mobile-card .el-card__body {
  padding: 0;
}

.vm-card-migrating {
  opacity: 0.55;
}

.vm-card-header {
  padding: 14px 16px 10px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.vm-card-name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.vm-card-specs {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.vm-card-spec {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  padding: 2px 8px;
  border-radius: 4px;
}

.vm-card-body {
  padding: 10px 16px;
}

.vm-card-info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.vm-card-info-row:last-child {
  border-bottom: none;
}

.vm-card-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
  margin-right: 12px;
}

.vm-card-value {
  font-size: 13px;
  color: var(--el-text-color-primary);
  text-align: right;
  word-break: break-all;
}

.vm-card-remark {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.vm-card-resource {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.vm-card-resource-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.vm-card-resource-item .vm-card-label {
  width: 32px;
  flex-shrink: 0;
}

.vm-card-resource-item .el-progress {
  flex: 1;
}

.progress-text {
  font-size: 11px;
  color: var(--el-text-color-primary);
}

.vm-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px 16px;
  border-top: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  border-radius: 0 0 10px 10px;
}

.vm-card-actions .el-button {
  margin-left: 0;
}

@media (max-width: 480px) {
  .vm-card-header {
    padding: 10px 12px 8px;
  }

  .vm-card-body {
    padding: 8px 12px;
  }

  .vm-card-actions {
    padding: 10px 12px;
  }

  .vm-card-spec {
    font-size: 12px;
    padding: 1px 6px;
  }
}

/* ===== 分组视图样式 ===== */
.grouped-view {
  display: block;
}

/* ===== 卡片视图样式 ===== */
.grouped-card-view,
.all-card-view {
  display: block;
}

.card-grid {
  display: grid;
  gap: 16px;
  padding: 16px;
}

@media (min-width: 1200px) {
  .card-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (min-width: 992px) and (max-width: 1199px) {
  .card-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (min-width: 769px) and (max-width: 991px) {
  .card-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.vm-card {
  background: var(--el-bg-color);
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  border-left: 4px solid transparent;
  display: flex;
  flex-direction: column;
  gap: 0;
}

.vm-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.vm-card.card-running {
  border-left-color: #67c23a;
}

.vm-card.card-paused {
  border-left-color: #e6a23c;
}

.vm-card.card-shutoff {
  border-left-color: #909399;
}

.vm-card.card-migrating {
  border-left-color: #f56c6c;
  opacity: 0.7;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
  gap: 8px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.card-title .el-link {
  font-size: 15px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-specs {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.spec-item {
  display: flex;
  align-items: center;
  gap: 4px;
  background: var(--el-fill-color-light);
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 13px;
}

.spec-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.spec-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.card-info {
  margin-bottom: 12px;
  flex: 1;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
  gap: 8px;
}

.info-row:last-child {
  border-bottom: none;
}

.info-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
}

.info-value {
  font-size: 13px;
  color: var(--el-text-color-primary);
  text-align: right;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.card-remark-edit {
  display: inline-flex;
  gap: 2px;
  margin-left: 4px;
  flex-shrink: 0;
}

.card-resource {
  margin-bottom: 14px;
  padding: 10px 12px;
  background: var(--el-fill-color-lighter);
  border-radius: 8px;
}

.resource-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.resource-item + .resource-item {
  margin-top: 6px;
}

.resource-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  width: 30px;
  flex-shrink: 0;
}

.resource-item .el-progress {
  flex: 1;
}

.progress-text {
  font-size: 11px;
  color: var(--el-text-color-primary);
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-extra-light);
  gap: 8px;
}

.card-footer-left {
  display: flex;
  gap: 6px;
}

.all-card-view .pagination-wrap {
  padding: 0 16px 16px;
}

.group-section {
  margin-bottom: 8px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
  background: var(--el-bg-color);
  transition: box-shadow 0.2s ease;
}

.group-section:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  cursor: pointer;
  user-select: none;
  background: var(--el-fill-color-lighter);
  border-bottom: 1px solid var(--el-border-color-lighter);
  transition: background 0.2s ease;
}

.group-header:hover {
  background: var(--el-fill-color-light);
}

.group-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-expand-icon {
  font-size: 14px;
  transition: transform 0.25s ease;
  color: var(--el-text-color-secondary);
}

.group-expand-icon.is-expanded {
  transform: rotate(90deg);
}

.group-name {
  font-weight: 600;
  font-size: 14px;
}

.group-count {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color);
  padding: 2px 8px;
  border-radius: 10px;
}

.group-header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-body {
  overflow: hidden;
}

.group-table-wrap {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

.group-table-wrap .el-table {
  border: none;
  border-radius: 0;
}

.group-table-wrap .el-table::before {
  height: 0;
}

.group-collapsed .group-header {
  border-bottom: none;
}

/* 分组移动端卡片视图 - 桌面端隐藏 */
.mobile-grouped-cards {
  display: none;
}

/* 分组视图响应式 */
@media (max-width: 768px) {
  .grouped-view {
    display: none !important;
  }

  .mobile-grouped-cards {
    display: block;
  }

  .group-header {
    padding: 8px 12px;
  }

  .group-header-left {
    gap: 6px;
  }

  .group-name {
    font-size: 13px;
  }
}

@media (max-width: 480px) {
  .group-header {
    padding: 6px 10px;
  }

  .group-count {
    font-size: 11px;
    padding: 1px 6px;
  }
}
.ip-unreachable {
  color: var(--el-color-warning);
  cursor: help;
  text-decoration: underline;
  text-decoration-style: dotted;
  text-underline-offset: 2px;
}
</style>
