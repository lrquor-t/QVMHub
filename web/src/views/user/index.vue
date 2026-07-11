<template>
  <div class="user-list-container">
    <h2>用户管理</h2>
    <el-card>
      <el-button type="primary" icon="Plus" style="margin-bottom: 20px;" @click="handleCreate">新增用户</el-button>

      <div class="filter-bar">
        <el-input
          v-model="userSearchText"
          placeholder="搜索用户名"
          clearable
          :prefix-icon="Search"
          size="small"
          style="width: 160px;"
        />
        <el-input
          v-model="userEmailSearch"
          placeholder="搜索邮箱"
          clearable
          :prefix-icon="Search"
          size="small"
          style="width: 200px;"
        />
        <el-select
          v-model="userRoleFilter"
          placeholder="角色筛选"
          clearable
          size="small"
          style="width: 130px;"
        >
          <el-option label="管理员" value="admin" />
          <el-option label="普通用户" value="user" />
        </el-select>
        <el-select
          v-model="userStatusFilter"
          placeholder="状态筛选"
          clearable
          size="small"
          style="width: 130px;"
        >
          <el-option label="正常" value="active" />
          <el-option label="待激活" value="pending_invite" />
          <el-option label="已封禁" value="disabled" />
        </el-select>
        <el-select
          v-model="userCloudTypeFilter"
          placeholder="用户类型"
          clearable
          size="small"
          style="width: 130px;"
        >
          <el-option label="弹性云" value="elastic" />
          <el-option label="轻量云" value="lightweight" />
        </el-select>
      </div>

      <el-table :data="paginatedUserData" border style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="username" label="用户名" width="120" />
        <el-table-column prop="email" label="邮箱" width="220" />
        <el-table-column prop="role" label="角色" width="100">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : 'success'">
              {{ row.role === 'admin' ? '管理员' : '普通用户' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="用户类型" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.role !== 'admin'" :type="row.cloud_type === 'lightweight' ? 'warning' : 'success'">
              {{ row.cloud_type === 'lightweight' ? '轻量云' : '弹性云' }}
            </el-tag>
            <span v-else style="color: #999;">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'pending_invite' ? 'warning' : (row.status === 'active' ? 'success' : 'danger')">
              {{ row.status === 'pending_invite' ? '待激活' : (row.status === 'active' ? '正常' : '已封禁') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="CPU 配额" width="120">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_cpu }} / {{ row.max_cpu || '不限' }}</span>
              <el-progress v-if="row.max_cpu > 0" :percentage="Math.min(Math.round(row.quota.used_cpu / row.max_cpu * 100), 100)" :stroke-width="4" :show-text="false" style="margin-top:2px;" />
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'compute') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="内存配额" width="140">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_memory }}GB / {{ row.max_memory ? row.max_memory + 'GB' : '不限' }}</span>
              <el-progress v-if="row.max_memory > 0" :percentage="Math.min(Math.round(row.quota.used_memory / row.max_memory * 100), 100)" :stroke-width="4" :show-text="false" style="margin-top:2px;" />
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'compute') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="磁盘配额" width="140">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_disk }}GB / {{ row.max_disk ? row.max_disk + 'GB' : '不限' }}</span>
              <el-progress v-if="row.max_disk > 0" :percentage="Math.min(Math.round(row.quota.used_disk / row.max_disk * 100), 100)" :stroke-width="4" :show-text="false" style="margin-top:2px;" />
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'compute') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="VM 数量" width="120">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_vm }} / {{ row.max_vm || '不限' }}</span>
              <el-progress v-if="row.max_vm > 0" :percentage="Math.min(Math.round(row.quota.used_vm / row.max_vm * 100), 100)" :stroke-width="4" :show-text="false" style="margin-top:2px;" />
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'compute') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="快照数量" width="120">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_snapshots || 0 }} / {{ row.max_snapshots || '不限' }}</span>
              <el-progress v-if="row.max_snapshots > 0" :percentage="Math.min(Math.round((row.quota.used_snapshots || 0) / row.max_snapshots * 100), 100)" :stroke-width="4" :show-text="false" style="margin-top:2px;" />
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'snapshot') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="端口转发" width="130">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <el-tag size="small" :type="row.enable_port_forward ? 'success' : 'info'">
                {{ row.enable_port_forward ? '已开通' : '未开通' }}
              </el-tag>
              <template v-if="row.enable_port_forward">
                <div style="margin-top: 4px;">{{ row.quota.used_port_forwards || 0 }} / {{ row.max_port_forwards || '不限' }}</div>
                <el-progress v-if="row.max_port_forwards > 0" :percentage="Math.min(Math.round((row.quota.used_port_forwards || 0) / row.max_port_forwards * 100), 100)" :stroke-width="4" :show-text="false" style="margin-top:2px;" />
              </template>
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'port_forward') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="公网 IP" width="120">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_public_ips || 0 }} / {{ row.max_public_ips || '不限' }}</span>
              <el-progress v-if="row.max_public_ips > 0" :percentage="Math.min(Math.round((row.quota.used_public_ips || 0) / row.max_public_ips * 100), 100)" :stroke-width="4" :show-text="false" style="margin-top:2px;" />
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'disabled') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="存储配额" width="140">
          <template #default="{ row }">
            <template v-if="row.quota">
              <span>{{ row.quota.used_storage_gb || '0 B' }} / {{ row.max_storage ? row.max_storage + 'GB' : '不限' }}</span>
              <el-progress v-if="row.max_storage > 0" :percentage="Math.min(Math.round(row.quota.used_storage / (row.max_storage * 1073741824) * 100), 100)" :stroke-width="4" :show-text="false" style="margin-top:2px;" />
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'disabled') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="运行时长" width="180">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_runtime_display || '0秒' }} / {{ row.max_runtime_hours ? row.max_runtime_hours + '小时' : '不限' }}</span>
              <el-progress v-if="row.max_runtime_hours > 0" :percentage="Math.min(Math.round(row.quota.used_runtime_seconds / (row.max_runtime_hours * 3600) * 100), 100)" :stroke-width="4" :show-text="false" :status="row.quota.runtime_quota_reached ? 'exception' : ''" style="margin-top:2px;" />
              <el-tag v-if="row.quota.runtime_quota_reached" type="danger" size="small" style="margin-top:2px;">已耗尽</el-tag>
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'disabled') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="下行流量" width="160">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_traffic_down_gb || '0 B' }} / {{ row.max_traffic_down ? (row.max_traffic_down < 1 ? (row.max_traffic_down * 1024).toFixed(0) + ' MB' : row.max_traffic_down + ' GB') : '不限' }}</span>
              <el-progress v-if="row.max_traffic_down > 0" :percentage="Math.min(Math.round(row.quota.used_traffic_down / (row.max_traffic_down * 1073741824) * 100), 100)" :stroke-width="4" :show-text="false" :status="row.quota.is_limited_down ? 'exception' : ''" style="margin-top:2px;" />
              <el-tag v-if="row.quota.is_limited_down" type="danger" size="small" style="margin-top:2px;">已限速</el-tag>
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'traffic') }}</span>
          </template>
        </el-table-column>
        <el-table-column label="上行流量" width="160">
          <template #default="{ row }">
            <template v-if="row.role !== 'admin' && row.quota">
              <span>{{ row.quota.used_traffic_up_gb || '0 B' }} / {{ row.max_traffic_up ? (row.max_traffic_up < 1 ? (row.max_traffic_up * 1024).toFixed(0) + ' MB' : row.max_traffic_up + ' GB') : '不限' }}</span>
              <el-progress v-if="row.max_traffic_up > 0" :percentage="Math.min(Math.round(row.quota.used_traffic_up / (row.max_traffic_up * 1073741824) * 100), 100)" :stroke-width="4" :show-text="false" :status="row.quota.is_limited_up ? 'exception' : ''" style="margin-top:2px;" />
              <el-tag v-if="row.quota.is_limited_up" type="danger" size="small" style="margin-top:2px;">已限速</el-tag>
            </template>
            <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'traffic') }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="vms" label="虚拟机">
          <template #default="{ row }">
            <el-tag v-for="vm in (row.vms || []).slice(0, 5)" :key="vm" size="small" style="margin: 2px 4px 2px 0;">{{ vm }}</el-tag>
            <span v-if="(row.vms || []).length > 5" style="color: #999; font-size: 12px;">+{{ row.vms.length - 5 }}</span>
            <span v-if="!row.vms || row.vms.length === 0" style="color: #999;">未分配</span>
            <div v-if="isLightweightUser(row) && (row.lightweight_vm_registrations || []).length" class="registration-mini-list">
              <el-tag
                v-for="item in (row.lightweight_vm_registrations || []).slice(0, 3)"
                :key="item.id"
                size="small"
                :type="registrationStatusType(item.status)"
                effect="plain"
              >
                {{ registrationStatusText(item.status) }}：{{ item.vm_name }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="SSH" width="80" align="center">
          <template #default="{ row }">
            <el-switch
              v-if="row.role !== 'admin'"
              v-model="row.ssh_enabled"
              size="small"
              :disabled="row.status !== 'active'"
              @change="(val) => handleToggleSSH(row, val)"
            />
            <span v-else style="color: #999;">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="380" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="warning" :disabled="row.role === 'admin' && row.username === userStore.username" @click="handleEditQuota(row)">配置</el-button>
            <el-button size="small" type="primary" :disabled="row.role === 'admin'" @click="handleAssign(row)">
              {{ row.cloud_type === 'lightweight' ? '注册VM' : '分配VM' }}
            </el-button>
            <el-button size="small" type="success" v-if="row.status === 'pending_invite'" @click="handleResendInvite(row)">重发邀请</el-button>
            <el-button size="small" type="danger" v-if="row.username !== 'admin' && row.username !== userStore.username && row.status === 'active'" @click="handleToggleStatus(row, 'disabled')">封禁</el-button>
            <el-button size="small" type="success" v-if="row.username !== 'admin' && row.username !== userStore.username && row.status === 'disabled'" @click="handleToggleStatus(row, 'active')">解封</el-button>
            <el-button size="small" type="info" :disabled="row.role === 'admin' || row.cloud_type === 'lightweight' || !(row.quota && (row.quota.is_limited_down || row.quota.is_limited_up))" @click="handleResetTraffic(row)">重置流量</el-button>
            <el-button size="small" type="danger" :disabled="(row.role === 'admin' && row.username === userStore.username) || row.username === 'admin'" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrap">
        <el-pagination
          v-if="filteredTableData.length > userPageSize"
          background
          layout="total, prev, pager, next"
          :total="filteredTableData.length"
          :page-size="userPageSize"
          :current-page="userCurrentPage"
          @current-change="userCurrentPage = $event"
        />
      </div>

      <!-- ===== 移动端卡片视图 ===== -->
      <div class="mobile-card-list">
        <el-card
          v-for="row in paginatedUserData"
          :key="row.id"
          class="user-mobile-card"
          shadow="hover"
        >
          <div class="user-card-header">
            <div class="user-card-name-row">
              <span class="user-card-name">{{ row.username }}</span>
              <el-tag :type="row.role === 'admin' ? 'danger' : 'success'" size="small">
                {{ row.role === 'admin' ? '管理员' : '普通用户' }}
              </el-tag>
            </div>
            <div class="user-card-meta">
              <span>{{ row.email }}</span>
              <el-tag v-if="row.role !== 'admin'" :type="row.cloud_type === 'lightweight' ? 'warning' : 'success'" size="small">
                {{ row.cloud_type === 'lightweight' ? '轻量云' : '弹性云' }}
              </el-tag>
              <el-tag :type="row.status === 'pending_invite' ? 'warning' : (row.status === 'active' ? 'success' : 'danger')" size="small">
                {{ row.status === 'pending_invite' ? '待激活' : (row.status === 'active' ? '正常' : '已封禁') }}
              </el-tag>
            </div>
          </div>
          <div class="user-card-body">
            <template v-if="row.role !== 'admin'">
              <div class="user-card-info-row">
                <span class="user-card-label">CPU 配额</span>
                <span class="user-card-value">
                  <template v-if="row.quota">{{ row.quota.used_cpu }} / {{ row.max_cpu || '不限' }}</template>
                  <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'compute') }}</span>
                </span>
              </div>
              <div class="user-card-info-row">
                <span class="user-card-label">内存配额</span>
                <span class="user-card-value">
                  <template v-if="row.quota">{{ row.quota.used_memory }}GB / {{ row.max_memory ? row.max_memory + 'GB' : '不限' }}</template>
                  <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'compute') }}</span>
                </span>
              </div>
              <div class="user-card-info-row">
                <span class="user-card-label">磁盘配额</span>
                <span class="user-card-value">
                  <template v-if="row.quota">{{ row.quota.used_disk }}GB / {{ row.max_disk ? row.max_disk + 'GB' : '不限' }}</template>
                  <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'compute') }}</span>
                </span>
              </div>
              <div class="user-card-info-row">
                <span class="user-card-label">VM 数量</span>
                <span class="user-card-value">
                  <template v-if="row.quota">{{ row.quota.used_vm }} / {{ row.max_vm || '不限' }}</template>
                  <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'compute') }}</span>
                </span>
              </div>
              <div class="user-card-info-row">
                <span class="user-card-label">快照数量</span>
                <span class="user-card-value">
                  <template v-if="row.quota">{{ row.quota.used_snapshots || 0 }} / {{ row.max_snapshots || '不限' }}</template>
                  <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'snapshot') }}</span>
                </span>
              </div>
              <div class="user-card-info-row">
                <span class="user-card-label">端口转发</span>
                <span class="user-card-value">
                  <template v-if="row.quota">
                    <el-tag size="small" :type="row.enable_port_forward ? 'success' : 'info'">{{ row.enable_port_forward ? '已开通' : '未开通' }}</el-tag>
                    <span v-if="row.enable_port_forward" style="margin-left:8px">{{ row.quota.used_port_forwards || 0 }}/{{ row.max_port_forwards || '不限' }}</span>
                  </template>
                  <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'port_forward') }}</span>
                </span>
              </div>
              <div class="user-card-info-row">
                <span class="user-card-label">公网 IP</span>
                <span class="user-card-value">
                  <template v-if="row.quota">{{ row.quota.used_public_ips || 0 }} / {{ row.max_public_ips || '不限' }}</template>
                  <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'disabled') }}</span>
                </span>
              </div>
              <div class="user-card-info-row">
                <span class="user-card-label">存储配额</span>
                <span class="user-card-value">
                  <template v-if="row.quota">{{ row.quota.used_storage_gb || '0 B' }} / {{ row.max_storage ? row.max_storage + 'GB' : '不限' }}</template>
                  <span v-else style="color: #999;">{{ userQuotaPlaceholder(row, 'disabled') }}</span>
                </span>
              </div>
              <div class="user-card-info-row">
                <span class="user-card-label">SSH</span>
                <span class="user-card-value">
                  <el-switch v-model="row.ssh_enabled" size="small" :disabled="row.status !== 'active'" @change="(val) => handleToggleSSH(row, val)" />
                </span>
              </div>
            </template>
            <div v-else class="user-card-info-row">
              <span class="user-card-label">类型</span>
              <span class="user-card-value">管理员</span>
            </div>
            <div v-if="row.role === 'admin' && row.quota" class="user-card-info-row">
              <span class="user-card-label">存储配额</span>
              <span class="user-card-value">{{ row.quota.used_storage_gb || '0 B' }} / {{ row.max_storage ? row.max_storage + 'GB' : '不限' }}</span>
            </div>
            <div v-if="row.vms && row.vms.length" class="user-card-vms">
              <span class="user-card-label">虚拟机</span>
              <div class="user-card-vm-tags">
                <el-tag v-for="vm in (row.vms || []).slice(0, 8)" :key="vm" size="small">{{ vm }}</el-tag>
                <span v-if="(row.vms || []).length > 8" style="color: #999; font-size: 12px;">+{{ row.vms.length - 8 }}</span>
              </div>
            </div>
          </div>
          <div class="user-card-actions">
            <el-button size="small" type="warning" :disabled="row.role === 'admin' && row.username === userStore.username" @click="handleEditQuota(row)">配置</el-button>
            <el-button size="small" type="primary" :disabled="row.role === 'admin'" @click="handleAssign(row)">
              {{ row.cloud_type === 'lightweight' ? '注册VM' : '分配VM' }}
            </el-button>
            <el-button size="small" type="success" v-if="row.status === 'pending_invite'" @click="handleResendInvite(row)">重发邀请</el-button>
            <el-button size="small" type="danger" v-if="row.username !== 'admin' && row.username !== userStore.username && row.status === 'active'" @click="handleToggleStatus(row, 'disabled')">封禁</el-button>
            <el-button size="small" type="success" v-if="row.username !== 'admin' && row.username !== userStore.username && row.status === 'disabled'" @click="handleToggleStatus(row, 'active')">解封</el-button>
            <el-button size="small" type="info" :disabled="row.role === 'admin' || row.cloud_type === 'lightweight' || !(row.quota && (row.quota.is_limited_down || row.quota.is_limited_up))" @click="handleResetTraffic(row)">重置流量</el-button>
            <el-button size="small" type="danger" :disabled="(row.role === 'admin' && row.username === userStore.username) || row.username === 'admin'" @click="handleDelete(row)">删除</el-button>
          </div>
        </el-card>
      </div>
    </el-card>

    <!-- 新增用户对话框 -->
    <el-dialog title="新增用户" v-model="createVisible" width="800px" append-to-body>
      <el-form :model="form" :rules="createRules" ref="formRef" label-width="100px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" />
        </el-form-item>
        <!-- SMTP 未配置时，需要填写初始密码 -->
        <el-form-item v-if="!smtpConfigured" label="初始密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="为用户设置初始密码" />
          <div class="form-tip">
            <el-icon><Warning /></el-icon>
            SMTP 未配置，用户将直接使用此密码登录，无需邮件邀请。
          </div>
        </el-form-item>
        <el-form-item v-if="!smtpConfigured" label="确认密码" prop="confirmPassword">
          <el-input v-model="form.confirmPassword" type="password" show-password placeholder="请再次输入密码" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="form.role">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
        <template v-if="form.role === 'user'">
          <el-form-item label="用户类型">
            <el-select v-model="form.cloud_type" style="width: 220px;">
              <el-option label="弹性云" value="elastic" />
              <el-option label="轻量云" value="lightweight" />
            </el-select>
          </el-form-item>
          <template v-if="form.cloud_type === 'lightweight'">
            <el-form-item label="VM 来源">
              <el-radio-group v-model="form.lightweight_vm_source">
                <el-radio value="existing">选择已有 VM</el-radio>
                <el-radio value="register">注册新 VM</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item v-if="form.lightweight_vm_source === 'register'" label="专用 VPC">
              <el-select v-model="form.dedicated_vpc_switch_id" filterable placeholder="请选择管理员创建的 NAT VPC" style="width: 100%;">
                <el-option v-for="item in natVPCSwitches" :key="item.id" :label="vpcOptionLabel(item)" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="form.lightweight_vm_source === 'existing'" label="选择 VM">
              <el-select v-model="form.lightweight_existing_vms" multiple filterable placeholder="选择要分配给该用户的已有 VM" style="width: 100%;" v-loading="allVMsLoading">
                <el-option v-for="vm in allVMs" :key="vm" :label="vm" :value="vm" :disabled="isVMAlreadyAssigned(vm)" />
              </el-select>
            </el-form-item>
            <div v-if="form.lightweight_vm_source === 'register'" class="registration-panel">
              <div class="registration-panel-header">
                <div>
                  <strong>待注册 VM</strong>
                  <span class="registration-hint">配置会随邀请邮件发送，用户确认凭据后才开通。</span>
                </div>
                <el-button size="small" type="primary" :disabled="!form.dedicated_vpc_switch_id" @click="openCreateRegistrationForm">添加注册VM</el-button>
              </div>
              <el-table v-if="form.lightweight_vm_registrations.length" :data="form.lightweight_vm_registrations" border size="small">
                <el-table-column prop="vm_name" label="名称" min-width="120" />
                <el-table-column prop="template" label="模板" min-width="140" show-overflow-tooltip />
                <el-table-column label="规格" width="120">
                  <template #default="{ row }">{{ row.vcpu }}C / {{ row.ram }}GB / {{ row.disk_size }}GB</template>
                </el-table-column>
                <el-table-column label="网络配额" min-width="180">
                  <template #default="{ row }">{{ formatRegistrationQuota(row) }}</template>
                </el-table-column>
                <el-table-column label="操作" width="90">
                  <template #default="{ $index }">
                    <el-button size="small" type="danger" plain @click="removeCreateRegistration($index)">移除</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <el-empty v-else description="还没有待注册 VM" :image-size="64" />
            </div>
            <div v-if="form.lightweight_vm_source === 'existing' && form.lightweight_existing_vms.length" class="registration-panel">
              <div class="registration-panel-header">
                <div>
                  <strong>已有 VM 配额</strong>
                  <span class="registration-hint">为每个已选 VM 设置流量、带宽和端口转发等配额。</span>
                </div>
              </div>
              <el-table :data="existingVMQuotaRows" border size="small">
                <el-table-column prop="vm_name" label="虚拟机" min-width="140" />
                <el-table-column label="下行月流量(GB)" width="150">
                  <template #default="{ row }"><el-input-number v-model="row.traffic_down_gb" :min="0" :precision="2" size="small" /></template>
                </el-table-column>
                <el-table-column label="上行月流量(GB)" width="150">
                  <template #default="{ row }"><el-input-number v-model="row.traffic_up_gb" :min="0" :precision="2" size="small" /></template>
                </el-table-column>
                <el-table-column label="下行带宽(Mbps)" width="150">
                  <template #default="{ row }"><el-input-number v-model="row.bandwidth_down_mbps" :min="0" size="small" /></template>
                </el-table-column>
                <el-table-column label="上行带宽(Mbps)" width="150">
                  <template #default="{ row }"><el-input-number v-model="row.bandwidth_up_mbps" :min="0" size="small" /></template>
                </el-table-column>
                <el-table-column label="端口转发上限" width="140">
                  <template #default="{ row }"><el-input-number v-model="row.max_port_forwards" :min="0" size="small" /></template>
                </el-table-column>
                <el-table-column label="快照上限" width="120">
                  <template #default="{ row }"><el-input-number v-model="row.max_snapshots" :min="0" size="small" /></template>
                </el-table-column>
                <el-table-column label="运行时长(小时)" width="140">
                  <template #default="{ row }"><el-input-number v-model="row.max_runtime_hours" :min="0" size="small" /></template>
                </el-table-column>
              </el-table>
            </div>
          </template>
          <QuotaForm v-if="form.cloud_type !== 'lightweight'" :form="form" />
        </template>
        <template v-if="form.role === 'admin'">
          <el-divider content-position="left">存储配额</el-divider>
          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="存储配额 (GB)">
                <el-input-number v-model="form.max_storage" :min="0" :max="102400" controls-position="right" style="flex: 1;" />
                <div class="form-tip" style="margin-top:2px">设为 0 表示不限制</div>
              </el-form-item>
            </el-col>
          </el-row>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submitCreate">确定</el-button>
      </template>
    </el-dialog>

    <!-- 编辑配额对话框 -->
    <el-dialog title="编辑用户配置" v-model="quotaVisible" width="900px" append-to-body>
      <div style="margin-bottom: 16px;">
        <el-alert type="info" :closable="false">
          <template #title>
            用户：<strong>{{ quotaForm.username }}</strong>
            <el-tag v-if="quotaUserRole === 'admin'" type="danger" size="small" style="margin-left: 8px;">管理员</el-tag>
            <el-tag v-else-if="quotaUserRole === 'user'" type="success" size="small" style="margin-left: 8px;">普通用户</el-tag>
            <span style="margin-left: 12px;">设为 0 表示不限制</span>
          </template>
        </el-alert>
      </div>
      <el-form v-if="quotaUserRole !== 'admin'" label-width="120px">
        <el-form-item label="用户类型">
          <el-select v-model="quotaForm.cloud_type" style="width: 220px;">
            <el-option label="弹性云" value="elastic" />
            <el-option label="轻量云" value="lightweight" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="quotaForm.cloud_type === 'lightweight'" label="专用 VPC">
          <el-select v-model="quotaForm.dedicated_vpc_switch_id" filterable placeholder="请选择管理员创建的 NAT VPC" style="width: 100%;">
            <el-option v-for="item in natVPCSwitches" :key="item.id" :label="vpcOptionLabel(item)" :value="item.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <!-- 管理员仅显示存储配额 -->
      <el-form v-if="quotaUserRole === 'admin'" label-width="120px">
        <el-form-item label="存储配额 (GB)">
          <div style="display: flex; align-items: center; gap: 12px; width: 100%;">
            <el-input-number v-model="quotaForm.max_storage" :min="0" :max="102400" controls-position="right" style="width: 240px;" />
            <span v-if="quotaUsage" class="quota-used-tag">
              <template v-if="quotaForm.max_storage > 0">
                {{ quotaUsage.used_storage_gb || '0 B' }}/{{ quotaForm.max_storage }}GB
              </template>
              <template v-else>已用 {{ quotaUsage.used_storage_gb || '0 B' }}（不限）</template>
            </span>
          </div>
          <div class="form-tip" style="margin-top:4px">设为 0 表示不限制。管理员仅受存储配额约束，其他资源不受限制。</div>
        </el-form-item>
      </el-form>
      <QuotaForm v-if="quotaUserRole !== 'admin' && quotaForm.cloud_type !== 'lightweight'" :form="quotaForm" :show-usage="true" :usage="quotaUsage" />
      <template #footer>
        <el-button @click="quotaVisible = false">取消</el-button>
        <el-button type="primary" :loading="quotaLoading" @click="submitQuota">保存</el-button>
      </template>
    </el-dialog>

    <!-- 分配VM对话框 -->
    <el-dialog title="分配虚拟机" v-model="assignVisible" width="900px" append-to-body>
      <el-alert type="info" :closable="false" style="margin-bottom: 16px;">
        <template #title>用户：<strong>{{ assignForm.username }}</strong></template>
      </el-alert>
      <el-form label-width="80px">
        <el-form-item label="虚拟机">
          <el-select v-model="assignForm.vms" multiple filterable placeholder="选择虚拟机" style="width: 100%;" v-loading="allVMsLoading">
            <el-option v-for="vm in allVMs" :key="vm" :label="vm" :value="vm" />
          </el-select>
        </el-form-item>
      </el-form>
      <el-table v-if="assignForm.cloud_type === 'lightweight' && assignForm.vms.length" :data="assignLightweightRows" border size="small">
        <el-table-column prop="vm_name" label="虚拟机" min-width="140" />
        <el-table-column label="下行月流量(GB)" width="150">
          <template #default="{ row }"><el-input-number v-model="row.traffic_down_gb" :min="0" :precision="2" /></template>
        </el-table-column>
        <el-table-column label="上行月流量(GB)" width="150">
          <template #default="{ row }"><el-input-number v-model="row.traffic_up_gb" :min="0" :precision="2" /></template>
        </el-table-column>
        <el-table-column label="下行带宽(Mbps)" width="150">
          <template #default="{ row }"><el-input-number v-model="row.bandwidth_down_mbps" :min="0" /></template>
        </el-table-column>
        <el-table-column label="上行带宽(Mbps)" width="150">
          <template #default="{ row }"><el-input-number v-model="row.bandwidth_up_mbps" :min="0" /></template>
        </el-table-column>
        <el-table-column label="端口转发上限" width="140">
          <template #default="{ row }"><el-input-number v-model="row.max_port_forwards" :min="0" /></template>
        </el-table-column>
        <el-table-column label="快照上限" width="120">
          <template #default="{ row }"><el-input-number v-model="row.max_snapshots" :min="0" /></template>
        </el-table-column>
        <el-table-column label="运行时长(小时)" width="140">
          <template #default="{ row }"><el-input-number v-model="row.max_runtime_hours" :min="0" /></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="assignVisible = false">取消</el-button>
        <el-button type="primary" :loading="assignLoading" @click="submitAssign">保存</el-button>
      </template>
    </el-dialog>

    <!-- 注册VM对话框 -->
    <el-dialog title="注册轻量云 VM" v-model="registrationVisible" width="960px" append-to-body>
      <el-alert type="info" :closable="false" style="margin-bottom: 16px;">
        <template #title>
          用户：<strong>{{ registrationForm.username }}</strong>
          <span style="margin-left: 12px;">可注册新VM或分配已有VM。</span>
        </template>
      </el-alert>
      <el-form label-width="100px" style="margin-bottom: 16px;">
        <el-form-item label="VM 来源">
          <el-radio-group v-model="registrationForm.vm_source">
            <el-radio value="existing">选择已有 VM</el-radio>
            <el-radio value="register">注册新 VM</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template v-if="registrationForm.vm_source === 'existing'">
        <el-form label-width="100px" style="margin-bottom: 16px;">
          <el-form-item label="选择 VM">
            <el-select v-model="registrationForm.existing_vms" multiple filterable placeholder="选择要分配给该用户的已有 VM" style="width: 100%;" v-loading="allVMsLoading">
              <el-option v-for="vm in allVMs" :key="vm" :label="vm" :value="vm" :disabled="isVMAlreadyAssigned(vm)" />
            </el-select>
          </el-form-item>
        </el-form>
        <div v-if="registrationForm.existing_vms.length" class="registration-panel">
          <div class="registration-panel-header">
            <div>
              <strong>已有 VM 配额</strong>
              <span class="registration-hint">为每个已选 VM 设置流量、带宽和端口转发等配额。</span>
            </div>
          </div>
          <el-table :data="existingVMQuotaRowsInRegistration" border size="small">
            <el-table-column prop="vm_name" label="虚拟机" min-width="140" />
            <el-table-column label="下行月流量(GB)" width="150">
              <template #default="{ row }"><el-input-number v-model="row.traffic_down_gb" :min="0" :precision="2" size="small" /></template>
            </el-table-column>
            <el-table-column label="上行月流量(GB)" width="150">
              <template #default="{ row }"><el-input-number v-model="row.traffic_up_gb" :min="0" :precision="2" size="small" /></template>
            </el-table-column>
            <el-table-column label="下行带宽(Mbps)" width="150">
              <template #default="{ row }"><el-input-number v-model="row.bandwidth_down_mbps" :min="0" size="small" /></template>
            </el-table-column>
            <el-table-column label="上行带宽(Mbps)" width="150">
              <template #default="{ row }"><el-input-number v-model="row.bandwidth_up_mbps" :min="0" size="small" /></template>
            </el-table-column>
            <el-table-column label="端口转发上限" width="140">
              <template #default="{ row }"><el-input-number v-model="row.max_port_forwards" :min="0" size="small" /></template>
            </el-table-column>
            <el-table-column label="快照上限" width="120">
              <template #default="{ row }"><el-input-number v-model="row.max_snapshots" :min="0" size="small" /></template>
            </el-table-column>
            <el-table-column label="运行时长(小时)" width="140">
              <template #default="{ row }"><el-input-number v-model="row.max_runtime_hours" :min="0" size="small" /></template>
            </el-table-column>
          </el-table>
        </div>
      </template>
      <template v-if="registrationForm.vm_source === 'register'">
        <div class="registration-panel-header">
          <strong>当前注册 VM</strong>
          <el-button size="small" type="primary" :disabled="!registrationForm.dedicated_vpc_switch_id" @click="openExistingRegistrationForm">添加注册VM</el-button>
        </div>
      </template>
      <el-table :data="registrationRows" border size="small">
        <el-table-column prop="vm_name" label="名称" min-width="120" />
        <el-table-column prop="template" label="模板" min-width="140" show-overflow-tooltip />
        <el-table-column label="规格" width="120">
          <template #default="{ row }">{{ row.vcpu }}C / {{ row.ram }}GB / {{ row.disk_size }}GB</template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="registrationStatusType(row.status)">{{ registrationStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="网络配额" min-width="180">
          <template #default="{ row }">{{ formatRegistrationQuota(row) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row, $index }">
            <el-button size="small" type="primary" plain :disabled="row.status === 'provisioning'" @click="openRegistrationQuotaEdit(row)">编辑</el-button>
            <el-button
              v-if="row.status !== 'active' && row.status !== 'provisioning'"
              size="small"
              type="danger"
              plain
              :loading="row.deleting"
              @click="removeExistingRegistration(row, $index)"
            >
              删除
            </el-button>
            <el-button
              v-if="row.status === 'active'"
              size="small"
              type="warning"
              plain
              :loading="row.deleting"
              @click="removeActiveRegistration(row)"
            >
              移除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="registrationVisible = false">关闭</el-button>
        <template v-if="registrationForm.vm_source === 'existing'">
          <el-button type="primary" :loading="registrationLoading" :disabled="!registrationForm.existing_vms.length" @click="submitExistingVMs">分配已有 VM</el-button>
        </template>
        <template v-if="registrationForm.vm_source === 'register'">
          <el-button type="primary" :loading="registrationLoading" :disabled="!registrationDrafts.length" @click="submitExistingRegistrations">保存新增注册</el-button>
        </template>
      </template>
    </el-dialog>

    <el-dialog title="编辑轻量云 VM 配额" v-model="registrationQuotaVisible" width="620px" append-to-body>
      <el-alert type="info" :closable="false" style="margin-bottom: 16px;">
        <template #title>
          VM：<strong>{{ registrationQuotaForm.vm_name }}</strong>
          <span style="margin-left: 12px;">修改已开通 VM 会立即更新带宽限制。</span>
        </template>
      </el-alert>
      <el-form label-width="140px">
        <el-form-item label="下行月流量(GB)">
          <el-input-number v-model="registrationQuotaForm.traffic_down_gb" :min="0" :precision="2" style="width: 100%;" />
        </el-form-item>
        <el-form-item label="上行月流量(GB)">
          <el-input-number v-model="registrationQuotaForm.traffic_up_gb" :min="0" :precision="2" style="width: 100%;" />
        </el-form-item>
        <el-form-item label="下行带宽(Mbps)">
          <el-input-number v-model="registrationQuotaForm.bandwidth_down_mbps" :min="0" style="width: 100%;" />
        </el-form-item>
        <el-form-item label="上行带宽(Mbps)">
          <el-input-number v-model="registrationQuotaForm.bandwidth_up_mbps" :min="0" style="width: 100%;" />
        </el-form-item>
        <el-form-item label="端口转发上限">
          <el-input-number v-model="registrationQuotaForm.max_port_forwards" :min="0" style="width: 100%;" />
        </el-form-item>
        <el-form-item label="快照上限">
          <el-input-number v-model="registrationQuotaForm.max_snapshots" :min="0" style="width: 100%;" />
        </el-form-item>
        <el-form-item label="运行时长配额(小时)">
          <el-input-number v-model="registrationQuotaForm.max_runtime_hours" :min="0" style="width: 100%;" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="registrationQuotaVisible = false">取消</el-button>
        <el-button type="primary" :loading="registrationQuotaLoading" @click="submitRegistrationQuotaEdit">保存</el-button>
      </template>
    </el-dialog>

    <VmForm ref="registrationFormRef" @draft="handleRegistrationDraft" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import {
  getUserList,
  createUser,
  deleteUser,
  updateUserQuota,
  updateUserStatus,
  assignVms,
  toggleUserSSH,
  resetUserTraffic,
  resendInvite,
  createLightweightVmRegistrations,
  deleteLightweightVmRegistration,
  removeLightweightRegisteredVm,
  updateLightweightVmQuota
} from '@/api/user'
import { getVmList } from '@/api/vm'
import { getVPCSwitches } from '@/api/vpc'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Warning } from '@element-plus/icons-vue'
import QuotaForm from '@/components/QuotaForm.vue'
import VmForm from '@/components/VmForm.vue'
import { useUserStore } from '@/store/user'
import { passwordValidator, checkPasswordBreachAsync } from '@/utils/validate'

const userStore = useUserStore()

// SMTP 是否已配置
const smtpConfigured = computed(() => userStore.security?.smtp_configured === true)

const tableData = ref([])
const loading = ref(false)
const userSearchText = ref('')
const userEmailSearch = ref('')
const userRoleFilter = ref('')
const userStatusFilter = ref('')
const userCloudTypeFilter = ref('')
const createVisible = ref(false)
const submitLoading = ref(false)
const formRef = ref(null)

const form = reactive({
  username: '', email: '', password: '', confirmPassword: '', role: 'user',
  cloud_type: 'elastic',
  dedicated_vpc_switch_id: null,
  max_cpu: 4, max_memory: 8, max_disk: 100, max_vm: 5, max_storage: 10, max_runtime_hours: 0,
  enable_port_forward: true,
  max_port_forwards: 10,
  max_snapshots: 5,
  max_public_ips: 0,
  max_bandwidth_up: 0, max_bandwidth_down: 0,
  max_traffic_down: 0, max_traffic_up: 0,
  lightweight_vm_registrations: [],
  lightweight_vm_source: 'existing',
  lightweight_existing_vms: []
})
const createRules = reactive({
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }]
})

// SMTP 未配置时动态添加密码规则
const smtpNotConfiguredRules = {
  password: [
    { required: true, message: '请输入初始密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (!value) { callback(new Error('请输入初始密码')); return }
        passwordValidator(rule, value, callback)
      },
      trigger: 'blur'
    }
  ],
  confirmPassword: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== form.password) { callback(new Error('两次输入的密码不一致')); return }
        callback()
      },
      trigger: 'blur'
    }
  ]
}

// 配额编辑
const quotaVisible = ref(false)
const quotaLoading = ref(false)
const quotaUsage = ref(null)
const quotaForm = reactive({
  username: '', max_cpu: 0, max_memory: 0, max_disk: 0, max_vm: 0, max_storage: 0, max_runtime_hours: 0,
  cloud_type: 'elastic',
  dedicated_vpc_switch_id: null,
  enable_port_forward: true,
  max_port_forwards: 10,
  max_snapshots: 5,
  max_public_ips: 0,
  max_bandwidth_up: 0, max_bandwidth_down: 0,
  max_traffic_down: 0, max_traffic_up: 0
})
const quotaUserRole = ref('')  // 当前正在编辑配额的用户的角色

// 分配VM
const assignVisible = ref(false)
const assignLoading = ref(false)
const assignForm = reactive({ username: '', vms: [], cloud_type: 'elastic' })
const assignLightweightRows = ref([])
const allVMs = ref([])
const allVMsLoading = ref(false)
const vpcSwitches = ref([])
const natVPCSwitches = computed(() => vpcSwitches.value.filter(item => !item.bridge_mode || item.bridge_mode === 'nat'))
const registrationFormRef = ref(null)
const registrationVisible = ref(false)
const registrationLoading = ref(false)
const registrationForm = reactive({
  username: '',
  dedicated_vpc_switch_id: null,
  dedicated_vpc_label: '',
  vm_source: 'register',
  existing_vms: [],
  existing_vms_quota_data: []
})
const persistedRegistrations = ref([])
const registrationQuotas = ref([])
const registrationDrafts = ref([])
const registrationTarget = ref('create')

const existingVMQuotaRowsInRegistration = computed(() => {
  return registrationForm.existing_vms.map(vmName => {
    const existing = registrationForm.existing_vms_quota_data.find(item => item.vm_name === vmName)
    return existing || {
      vm_name: vmName,
      traffic_down_gb: 0,
      traffic_up_gb: 0,
      bandwidth_down_mbps: 0,
      bandwidth_up_mbps: 0,
      max_port_forwards: 10,
      max_snapshots: 2,
      max_runtime_hours: 0
    }
  })
})
const quotaOnlyRegistrations = computed(() => {
  const registeredNames = new Set(persistedRegistrations.value.map(item => item.vm_name))
  return registrationQuotas.value
    .filter(item => !registeredNames.has(item.vm_name))
    .map(item => ({
      ...item,
      id: null,
      template: '已开通 VM',
      template_type: '',
      vcpu: '-',
      ram: '-',
      disk_size: '-',
      status: 'active',
      quota_only: true
    }))
})
const registrationRows = computed(() => [...persistedRegistrations.value, ...quotaOnlyRegistrations.value, ...registrationDrafts.value])
const registrationQuotaVisible = ref(false)
const registrationQuotaLoading = ref(false)
const registrationQuotaEditingRow = ref(null)
const registrationQuotaForm = reactive({
  vm_name: '',
  traffic_down_gb: 0,
  traffic_up_gb: 0,
  bandwidth_down_mbps: 0,
  bandwidth_up_mbps: 0,
  max_port_forwards: 10,
  max_snapshots: 2,
  max_runtime_hours: 0
})

const vpcOptionLabel = (item) => `${item.username ? item.username + ' / ' : ''}${item.name} (${item.cidr || '-'})`
const selectedCreateVPCLabel = computed(() => {
  const item = natVPCSwitches.value.find(vpc => Number(vpc.id) === Number(form.dedicated_vpc_switch_id))
  return item ? vpcOptionLabel(item) : ''
})

const existingVMQuotaRows = computed(() => {
  return form.lightweight_existing_vms.map(vmName => {
    const existing = existingVMQuotaData.value.find(item => item.vm_name === vmName)
    return existing || {
      vm_name: vmName,
      traffic_down_gb: 0,
      traffic_up_gb: 0,
      bandwidth_down_mbps: 0,
      bandwidth_up_mbps: 0,
      max_port_forwards: 10,
      max_snapshots: 2,
      max_runtime_hours: 0
    }
  })
})

const existingVMQuotaData = ref([])

const isLightweightUser = (row) => row?.role !== 'admin' && row?.cloud_type === 'lightweight'

const isVMAlreadyAssigned = (vmName) => {
  const currentUser = form.username || registrationForm.username
  return tableData.value.some(user => {
    if (user.username === currentUser) return false
    return (user.vms || []).includes(vmName)
  })
}

const userQuotaPlaceholder = (row, type) => {
  if (!isLightweightUser(row)) return '-'
  const textMap = {
    compute: '管理员分配',
    port_forward: '单 VM 配额',
    snapshot: '单 VM 配额',
    traffic: '单 VM 配额',
    disabled: '不适用'
  }
  return textMap[type] || '不适用'
}

const registrationStatusText = (status) => {
  const map = {
    pending: '待确认',
    provisioning: '开通中',
    active: '已开通',
    failed: '失败',
    draft: '未保存'
  }
  return map[status] || status || '待确认'
}

const registrationStatusType = (status) => {
  const map = {
    pending: 'warning',
    provisioning: 'primary',
    active: 'success',
    failed: 'danger',
    draft: 'info'
  }
  return map[status] || 'warning'
}

const formatRegistrationQuota = (row) => {
  const traffic = `流量 ${row.traffic_down_gb || 0}/${row.traffic_up_gb || 0}GB`
  const bandwidth = `带宽 ${row.bandwidth_down_mbps || 0}/${row.bandwidth_up_mbps || 0}Mbps`
  const ports = `端口 ${row.max_port_forwards ?? 10}`
  const snapshots = `快照 ${row.max_snapshots ?? 2}`
  const runtime = `运行 ${row.max_runtime_hours ? `${row.max_runtime_hours}小时` : '不限'}`
  return `${traffic}，${bandwidth}，${ports}，${snapshots}，${runtime}`
}

const buildRegistrationPayload = (items) => items.map(({ client_id, status, deleting, ...item }) => item)
const buildRegistrationQuotaPayload = (row) => ({
  vm_name: row.vm_name,
  traffic_down_gb: Number(row.traffic_down_gb || 0),
  traffic_up_gb: Number(row.traffic_up_gb || 0),
  bandwidth_down_mbps: Number(row.bandwidth_down_mbps || 0),
  bandwidth_up_mbps: Number(row.bandwidth_up_mbps || 0),
  max_port_forwards: Number(row.max_port_forwards ?? 10),
  max_snapshots: Number(row.max_snapshots ?? 2),
  max_runtime_hours: Number(row.max_runtime_hours || 0)
})

const fetchVPCSwitchOptions = async () => {
  try {
    const res = await getVPCSwitches()
    vpcSwitches.value = res.data || []
  } catch (err) {
    console.error(err)
  }
}

const fetchAllVMs = async () => {
  allVMsLoading.value = true
  try {
    const res = await getVmList()
    allVMs.value = (res.data || []).map(v => v.name)
  } catch (err) {
    console.error(err)
  } finally {
    allVMsLoading.value = false
  }
}

const syncAssignLightweightRows = () => {
  const existing = new Map(assignLightweightRows.value.map(item => [item.vm_name, item]))
  assignLightweightRows.value = assignForm.vms.map(vmName => existing.get(vmName) || {
    vm_name: vmName,
    traffic_down_gb: 0,
    traffic_up_gb: 0,
    bandwidth_down_mbps: 0,
    bandwidth_up_mbps: 0,
    max_port_forwards: 10,
    max_snapshots: 2,
    max_runtime_hours: 0
  })
}

watch(() => [...assignForm.vms], () => {
  if (assignForm.cloud_type === 'lightweight') {
    syncAssignLightweightRows()
  }
})

watch(() => form.cloud_type, (cloudType) => {
  if (cloudType !== 'lightweight') {
    form.lightweight_vm_registrations = []
  }
})

// 创建表单角色切换时重置存储配额默认值
watch(() => form.role, (role) => {
  if (role === 'admin') {
    form.max_storage = 0
  } else {
    form.max_storage = 10
  }
})

const userCurrentPage = ref(1)
const userPageSize = ref(100)

const filteredTableData = computed(() => {
  let data = tableData.value
  if (userSearchText.value) {
    const q = userSearchText.value.toLowerCase()
    data = data.filter(u => u.username.toLowerCase().includes(q))
  }
  if (userEmailSearch.value) {
    const q = userEmailSearch.value.toLowerCase()
    data = data.filter(u => (u.email || '').toLowerCase().includes(q))
  }
  if (userRoleFilter.value) {
    data = data.filter(u => u.role === userRoleFilter.value)
  }
  if (userStatusFilter.value) {
    data = data.filter(u => u.status === userStatusFilter.value)
  }
  if (userCloudTypeFilter.value) {
    data = data.filter(u => u.cloud_type === userCloudTypeFilter.value)
  }
  return data
})

const paginatedUserData = computed(() => {
  const start = (userCurrentPage.value - 1) * userPageSize.value
  return filteredTableData.value.slice(start, start + userPageSize.value)
})

const fetchData = async () => {
  loading.value = true
  try {
    const res = await getUserList()
    tableData.value = res.data || []
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

watch([userSearchText, userEmailSearch, userRoleFilter, userStatusFilter, userCloudTypeFilter], () => {
  userCurrentPage.value = 1
})

onMounted(() => {
  fetchData()
  fetchVPCSwitchOptions()
})

const handleCreate = () => {
  form.username = ''
  form.email = ''
  form.password = ''
  form.confirmPassword = ''
  form.role = 'user'
  form.cloud_type = 'elastic'
  form.dedicated_vpc_switch_id = null
  form.max_cpu = 4
  form.max_memory = 8
  form.max_disk = 100
  form.max_vm = 5
  form.max_storage = 10
  form.max_runtime_hours = 0
  form.enable_port_forward = true
  form.max_port_forwards = 10
  form.max_snapshots = 5
  form.max_public_ips = 0
  form.max_bandwidth_up = 0
  form.max_bandwidth_down = 0
  form.max_traffic_down = 0
  form.max_traffic_up = 0
  form.lightweight_vm_registrations = []
  form.lightweight_vm_source = 'existing'
  form.lightweight_existing_vms = []
  existingVMQuotaData.value = []
  allVMs.value = []
  createVisible.value = true
  // 动态设置密码验证规则
  if (!smtpConfigured.value) {
    createRules.password = smtpNotConfiguredRules.password
    createRules.confirmPassword = smtpNotConfiguredRules.confirmPassword
  } else {
    delete createRules.password
    delete createRules.confirmPassword
  }
  fetchAllVMs()
}

const submitCreate = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (valid) {
      if (form.role === 'user' && form.cloud_type === 'lightweight') {
        if (form.lightweight_vm_source === 'register' && !form.dedicated_vpc_switch_id) {
          ElMessage.warning('请先为轻量云用户选择专用 VPC')
          return
        }
        if (form.lightweight_vm_source === 'existing' && !form.lightweight_existing_vms.length) {
          ElMessage.warning('请至少选择一台已有 VM')
          return
        }
      }
      // SMTP 未配置时需要密码，进行泄露检测
      if (!smtpConfigured && form.password) {
        const breach = await checkPasswordBreachAsync(form.password)
        if (breach.enabled && breach.breached) {
          ElMessage.error('该密码已在已知泄露数据库中发现，请更换为更安全的密码')
          return
        }
      }
      submitLoading.value = true
      try {
        const payload = { ...form }
        delete payload.confirmPassword // 确认密码不发送到后端
        if (payload.cloud_type !== 'lightweight') {
          delete payload.lightweight_vm_registrations
          delete payload.lightweight_existing_vms
          delete payload.lightweight_vm_source
        } else if (payload.lightweight_vm_source === 'existing') {
          delete payload.lightweight_vm_registrations
          delete payload.dedicated_vpc_switch_id
          payload.lightweight_existing_vm_quotas = existingVMQuotaRows.value
        } else {
          delete payload.lightweight_existing_vms
          delete payload.lightweight_vm_source
          payload.lightweight_vm_registrations = buildRegistrationPayload(form.lightweight_vm_registrations)
        }
        const res = await createUser(payload)
        ElMessage.success(res.message || '邀请邮件已发送')
        createVisible.value = false
        fetchData()
      } catch (err) {
        console.error(err)
      } finally {
        submitLoading.value = false
      }
    }
  })
}

const handleEditQuota = (row) => {
  quotaUserRole.value = row.role || ''
  quotaForm.username = row.username
  quotaForm.cloud_type = row.cloud_type || 'elastic'
  quotaForm.dedicated_vpc_switch_id = row.dedicated_vpc_switch_id || null
  quotaForm.max_cpu = row.max_cpu || 0
  quotaForm.max_memory = row.max_memory || 0
  quotaForm.max_disk = row.max_disk || 0
  quotaForm.max_vm = row.max_vm || 0
  quotaForm.max_storage = row.max_storage || 0
  quotaForm.max_runtime_hours = row.max_runtime_hours || 0
  quotaForm.enable_port_forward = row.enable_port_forward ?? true
  quotaForm.max_port_forwards = row.max_port_forwards ?? 10
  quotaForm.max_snapshots = row.max_snapshots ?? 5
  quotaForm.max_public_ips = row.max_public_ips || 0
  quotaForm.max_bandwidth_up = row.max_bandwidth_up || 0
  quotaForm.max_bandwidth_down = row.max_bandwidth_down || 0
  quotaForm.max_traffic_down = row.max_traffic_down || 0
  quotaForm.max_traffic_up = row.max_traffic_up || 0
  quotaUsage.value = row.quota || null
  quotaVisible.value = true
}

const submitQuota = async () => {
  quotaLoading.value = true
  try {
    const payload = { ...quotaForm }
    // 管理员不需要发送 cloud_type 和 dedicated_vpc_switch_id
    if (quotaUserRole.value === 'admin') {
      delete payload.cloud_type
      delete payload.dedicated_vpc_switch_id
    }
    await updateUserQuota(quotaForm.username, payload)
    ElMessage.success('配额更新成功')
    quotaVisible.value = false
    fetchData()
  } catch (err) {
    console.error(err)
  } finally {
    quotaLoading.value = false
  }
}

const openCreateRegistrationForm = () => {
  if (!form.dedicated_vpc_switch_id) {
    ElMessage.warning('请先选择专用 VPC')
    return
  }
  registrationTarget.value = 'create'
  registrationFormRef.value?.open(null, 'lightweight-register', {
    dedicated_vpc_switch_id: form.dedicated_vpc_switch_id,
    dedicated_vpc_label: selectedCreateVPCLabel.value
  })
}

const openExistingRegistrationForm = () => {
  if (!registrationForm.dedicated_vpc_switch_id) {
    ElMessage.warning('该用户尚未配置专用 VPC，请先在用户配置中设置')
    return
  }
  registrationTarget.value = 'existing'
  registrationFormRef.value?.open(null, 'lightweight-register', {
    dedicated_vpc_switch_id: registrationForm.dedicated_vpc_switch_id,
    dedicated_vpc_label: registrationForm.dedicated_vpc_label
  })
}

const normalizeRegistrationDraft = (draft) => ({
  ...draft,
  max_snapshots: draft.max_snapshots ?? 2,
  max_runtime_hours: draft.max_runtime_hours ?? 0,
  status: draft.status || 'draft',
  client_id: draft.client_id || `${Date.now()}-${Math.random().toString(16).slice(2)}`
})

const handleRegistrationDraft = (draft) => {
  const normalized = normalizeRegistrationDraft(draft)
  if (registrationTarget.value === 'create') {
    const exists = form.lightweight_vm_registrations.some(item => item.vm_name === normalized.vm_name)
    if (exists) {
      ElMessage.warning('注册列表中已存在同名 VM')
      return
    }
    form.lightweight_vm_registrations.push(normalized)
    return
  }
  const exists = registrationRows.value.some(item => item.vm_name === normalized.vm_name)
  if (exists) {
    ElMessage.warning('注册列表中已存在同名 VM')
    return
  }
  registrationDrafts.value.push(normalized)
}

const removeCreateRegistration = (index) => {
  form.lightweight_vm_registrations.splice(index, 1)
}

const openRegistrationQuotaEdit = (row) => {
  registrationQuotaEditingRow.value = row
  Object.assign(registrationQuotaForm, buildRegistrationQuotaPayload(row))
  registrationQuotaVisible.value = true
}

const patchRegistrationRow = (vmName, payload) => {
  const patch = { ...payload }
  const persisted = persistedRegistrations.value.find(item => item.vm_name === vmName)
  if (persisted) Object.assign(persisted, patch)
  const draft = registrationDrafts.value.find(item => item.vm_name === vmName)
  if (draft) Object.assign(draft, patch)
  const quota = registrationQuotas.value.find(item => item.vm_name === vmName)
  if (quota) Object.assign(quota, patch)
}

const replaceRegistrationFromResponse = (registration) => {
  if (!registration?.vm_name) return
  const index = persistedRegistrations.value.findIndex(item => item.vm_name === registration.vm_name)
  if (index >= 0) {
    persistedRegistrations.value[index] = { ...persistedRegistrations.value[index], ...registration }
  } else if (!registration.quota_only) {
    persistedRegistrations.value.push({ ...registration })
  }
}

const replaceQuotaFromResponse = (quota) => {
  if (!quota?.vm_name) return
  const index = registrationQuotas.value.findIndex(item => item.vm_name === quota.vm_name)
  if (index >= 0) {
    registrationQuotas.value[index] = { ...registrationQuotas.value[index], ...quota }
  } else {
    registrationQuotas.value.push({ ...quota })
  }
}

const submitRegistrationQuotaEdit = async () => {
  const row = registrationQuotaEditingRow.value
  if (!row?.vm_name) return
  const payload = buildRegistrationQuotaPayload(registrationQuotaForm)
  if (row.status === 'draft') {
    patchRegistrationRow(row.vm_name, payload)
    registrationQuotaVisible.value = false
    ElMessage.success('注册草稿配额已更新')
    return
  }
  registrationQuotaLoading.value = true
  try {
    const res = await updateLightweightVmQuota(registrationForm.username, payload)
    replaceRegistrationFromResponse(res.data?.registration)
    replaceQuotaFromResponse(res.data?.quota)
    patchRegistrationRow(row.vm_name, payload)
    ElMessage.success(res.message || '轻量云 VM 配额已更新')
    registrationQuotaVisible.value = false
    fetchData()
  } catch (err) {
    console.error(err)
  } finally {
    registrationQuotaLoading.value = false
  }
}

const removeExistingRegistration = async (row, index) => {
  if (!row.id) {
    const draftIndex = registrationDrafts.value.findIndex(item => item.client_id === row.client_id)
    if (draftIndex >= 0) registrationDrafts.value.splice(draftIndex, 1)
    return
  }
  try {
    await ElMessageBox.confirm(`确定删除待注册 VM ${row.vm_name}？`, '删除注册项', {
      type: 'warning',
      confirmButtonText: '确定删除',
      cancelButtonText: '取消'
    })
    row.deleting = true
    await deleteLightweightVmRegistration(registrationForm.username, row.id)
    ElMessage.success('注册项已删除')
    persistedRegistrations.value = persistedRegistrations.value.filter(item => item.id !== row.id)
    fetchData()
  } catch (err) {
    if (err) console.error(err)
  } finally {
    row.deleting = false
  }
}

const removeActiveRegistration = async (row) => {
  if (!row?.vm_name) return
  try {
    await ElMessageBox.confirm(
      `确定移除已开通 VM ${row.vm_name}？该操作会移除注册列表记录和单 VM 配额，不会删除虚拟机本体。`,
      '移除已开通 VM',
      {
        type: 'warning',
        confirmButtonText: '确定移除',
        cancelButtonText: '取消'
      }
    )
    row.deleting = true
    const res = await removeLightweightRegisteredVm(registrationForm.username, row.vm_name)
    ElMessage.success(res.message || '轻量云 VM 已移除')
    persistedRegistrations.value = persistedRegistrations.value.filter(item => item.vm_name !== row.vm_name)
    registrationQuotas.value = registrationQuotas.value.filter(item => item.vm_name !== row.vm_name)
    fetchData()
    fetchAllVMs()
  } catch (err) {
    if (err) console.error(err)
  } finally {
    row.deleting = false
  }
}

const submitExistingRegistrations = async () => {
  if (!registrationDrafts.value.length) return
  registrationLoading.value = true
  try {
    const payload = {
      registrations: buildRegistrationPayload(registrationDrafts.value)
    }
    const res = await createLightweightVmRegistrations(registrationForm.username, payload)
    ElMessage.success(res.message || '注册 VM 已保存')
    registrationVisible.value = false
    registrationDrafts.value = []
    fetchData()
  } catch (err) {
    console.error(err)
  } finally {
    registrationLoading.value = false
  }
}

const submitExistingVMs = async () => {
  if (!registrationForm.existing_vms.length) return
  registrationLoading.value = true
  try {
    const quotas = existingVMQuotaRowsInRegistration.value.map(item => ({
      vm_name: item.vm_name,
      traffic_down_gb: Number(item.traffic_down_gb || 0),
      traffic_up_gb: Number(item.traffic_up_gb || 0),
      bandwidth_down_mbps: Number(item.bandwidth_down_mbps || 0),
      bandwidth_up_mbps: Number(item.bandwidth_up_mbps || 0),
      max_port_forwards: Number(item.max_port_forwards ?? 10),
      max_snapshots: Number(item.max_snapshots ?? 2),
      max_runtime_hours: Number(item.max_runtime_hours || 0)
    }))
    const payload = {
      vms: registrationForm.existing_vms,
      lightweight_quotas: quotas
    }
    const res = await assignVms(registrationForm.username, payload)
    ElMessage.success(res.message || '已有 VM 分配成功')
    registrationVisible.value = false
    fetchData()
  } catch (err) {
    console.error(err)
  } finally {
    registrationLoading.value = false
  }
}

const handleAssign = async (row) => {
  if (row.cloud_type === 'lightweight') {
    registrationForm.username = row.username
    registrationForm.dedicated_vpc_switch_id = row.dedicated_vpc_switch_id || null
    const vpc = natVPCSwitches.value.find(item => Number(item.id) === Number(row.dedicated_vpc_switch_id))
    registrationForm.dedicated_vpc_label = vpc ? vpcOptionLabel(vpc) : `交换机 ID ${row.dedicated_vpc_switch_id || '-'}`
    registrationForm.vm_source = 'register'
    registrationForm.existing_vms = []
    registrationForm.existing_vms_quota_data = []
    persistedRegistrations.value = (row.lightweight_vm_registrations || []).map(item => ({ ...item }))
    registrationQuotas.value = (row.lightweight_quotas || []).map(item => ({ ...item }))
    registrationDrafts.value = []
    registrationVisible.value = true
    fetchAllVMs()
    return
  }
  assignForm.username = row.username
  assignForm.vms = row.vms ? [...row.vms] : []
  assignForm.cloud_type = row.cloud_type || 'elastic'
  assignLightweightRows.value = (row.lightweight_quotas || []).map(item => ({
    vm_name: item.vm_name,
    traffic_down_gb: item.traffic_down_gb || 0,
    traffic_up_gb: item.traffic_up_gb || 0,
    bandwidth_down_mbps: item.bandwidth_down_mbps || 0,
    bandwidth_up_mbps: item.bandwidth_up_mbps || 0,
    max_port_forwards: item.max_port_forwards ?? 10,
    max_snapshots: item.max_snapshots ?? 2,
    max_runtime_hours: item.max_runtime_hours || 0
  }))
  syncAssignLightweightRows()
  assignVisible.value = true

  // 获取所有VM列表
  allVMsLoading.value = true
  try {
    const res = await getVmList()
    allVMs.value = (res.data || []).map(v => v.name)
  } catch (err) {
    console.error(err)
  } finally {
    allVMsLoading.value = false
  }
}

const submitAssign = async () => {
  assignLoading.value = true
  try {
    const payload = { vms: assignForm.vms }
    if (assignForm.cloud_type === 'lightweight') {
      payload.lightweight_quotas = assignLightweightRows.value.filter(item => assignForm.vms.includes(item.vm_name))
    }
    await assignVms(assignForm.username, payload)
    ElMessage.success('分配成功')
    assignVisible.value = false
    fetchData()
  } catch (err) {
    console.error(err)
  } finally {
    assignLoading.value = false
  }
}

const handleDelete = async (row) => {
  const vmCount = (row.vms || []).length
  let confirmMsg = `确定删除用户 ${row.username}？`
  if (vmCount > 0) {
    confirmMsg += `\n\n⚠️ 将同时删除该用户的 ${vmCount} 台虚拟机（${row.vms.join('、')}）及其所有磁盘、快照、网络配置。`
  }
  confirmMsg += '\n\n⚠️ 用户的存储池（ISO 镜像、共享文件）也将被清除。'
  confirmMsg += '\n\n此操作不可恢复！'

  try {
    await ElMessageBox.confirm(confirmMsg, '删除用户及所有资产', {
      type: 'warning',
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
      dangerouslyUseHTMLString: false,
    })
    const res = await deleteUser(row.username)
    if (res.data?.task_id) {
      ElMessage.success(`删除用户任务已提交（任务ID: ${res.data.task_id}），请在任务中心查看进度`)
    } else {
      ElMessage.success('删除用户任务已提交')
    }
    // 延迟刷新列表，给任务一点时间
    setTimeout(fetchData, 2000)
  } catch {}
}

const handleToggleSSH = async (row, val) => {
  try {
    await toggleUserSSH(row.username, val)
    ElMessage.success(`用户 ${row.username} 的 SSH 访问已${val ? '开启' : '关闭'}`)
  } catch (err) {
    // 失败时回滚开关状态
    row.ssh_enabled = !val
    console.error(err)
  }
}

const handleToggleStatus = async (row, targetStatus) => {
  const isDisable = targetStatus === 'disabled'
  const title = isDisable ? '封禁账户' : '解封账户'
  let confirmMsg = isDisable
    ? `确定封禁用户 ${row.username}？`
    : `确定解封用户 ${row.username}？`

  if (isDisable) {
    confirmMsg += '\n\n封禁后该用户将立即退出登录，系统会尝试关闭该用户下所有运行中的虚拟机，并同步关闭 SSH 访问。'
    confirmMsg += '\n\n此操作不会删除用户资产，解封后可继续使用。'
  } else {
    confirmMsg += '\n\n解封后用户可重新登录面板，但之前被关闭的虚拟机不会自动恢复启动。'
  }

  try {
    await ElMessageBox.confirm(confirmMsg, title, {
      type: 'warning',
      confirmButtonText: isDisable ? '确定封禁' : '确定解封',
      cancelButtonText: '取消',
      dangerouslyUseHTMLString: false,
    })

    const res = await updateUserStatus(row.username, { status: targetStatus })
    if (res.data?.task_id) {
      ElMessage.success(`${isDisable ? '封禁' : '解封'}任务已提交（任务ID: ${res.data.task_id}），请在任务中心查看进度`)
    } else {
      ElMessage.success(res.message || (isDisable ? '用户已封禁' : '用户已解封'))
    }
    setTimeout(fetchData, isDisable ? 2000 : 500)
  } catch {}
}

const handleResendInvite = async (row) => {
  try {
    await resendInvite(row.username)
    ElMessage.success(`已向 ${row.email} 重发邀请邮件`)
  } catch (err) {
    console.error(err)
  }
}

const handleResetTraffic = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定重置用户 ${row.username} 的本月流量配额？\n\n重置后将恢复正常网络速率。`,
      '重置流量配额',
      { type: 'warning', confirmButtonText: '确定重置', cancelButtonText: '取消' }
    )
    await resetUserTraffic(row.username)
    ElMessage.success(`用户 ${row.username} 的流量配额已重置`)
    fetchData()
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
.user-list-container {
  padding: 10px;
}

.registration-mini-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}

.registration-panel {
  margin: 14px 0 4px;
  padding: 14px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  background: #fafcff;
}

.registration-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.registration-hint {
  margin-left: 10px;
  color: #909399;
  font-size: 12px;
}

.form-tip {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

/* ===== 默认隐藏移动卡片（必须在 @media 之前） ===== */
.mobile-card-list {
  display: none;
}

@media (max-width: 768px) {
  .user-list-container {
    padding: 4px;
  }

  .user-list-container h2 {
    font-size: 16px;
    margin-bottom: 12px;
  }

  .registration-panel-header {
    flex-direction: column;
    align-items: flex-start;
  }

  /* 隐藏表格，显示卡片 */
  .user-list-container .el-table {
    display: none !important;
  }

  .mobile-card-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
}

/* ===== 用户移动卡片样式 ===== */
.user-mobile-card {
  border-radius: 10px;
}

.user-mobile-card .el-card__body {
  padding: 0;
}

.user-card-header {
  padding: 14px 16px 10px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.user-card-name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.user-card-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.user-card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-wrap: wrap;
}

.user-card-body {
  padding: 8px 16px;
}

.user-card-info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.user-card-info-row:last-child {
  border-bottom: none;
}

.user-card-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
  margin-right: 12px;
}

.user-card-value {
  font-size: 13px;
  color: var(--el-text-color-primary);
  text-align: right;
  display: flex;
  align-items: center;
  gap: 4px;
}

.user-card-vms {
  padding: 8px 0;
  border-top: 1px solid var(--el-border-color-extra-light);
}

.user-card-vm-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}

.user-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 10px 16px;
  border-top: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  border-radius: 0 0 10px 10px;
}

.user-card-actions .el-button {
  margin-left: 0;
}

@media (max-width: 480px) {
  .user-card-header {
    padding: 10px 12px 8px;
  }

  .user-card-body {
    padding: 6px 12px;
  }

  .user-card-actions {
    padding: 8px 12px;
    gap: 4px;
  }
}
</style>
