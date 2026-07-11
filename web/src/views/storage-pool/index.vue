<template>
  <div class="storage-pool-page">
    <div class="page-header-bar">
      <div class="page-header-left">
        <div class="page-title-row">
          <el-icon class="page-icon"><Box /></el-icon>
          <h2>存储池</h2>
        </div>
        <p>管理宿主机硬盘分区，配置虚拟机落盘位置与格式化挂载</p>
      </div>
      <div class="page-header-right">
        <div class="filter-toggle">
          <el-switch v-model="showAvailableOnly" size="small" />
          <span>仅显示可用磁盘</span>
        </div>
        <el-button type="success" :icon="Plus" @click="openCreateVolume">创建存储池</el-button>
        <el-button type="primary" :icon="Refresh" @click="fetchData" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-row :gutter="12" class="overview-row">
      <el-col :span="6" :xs="12" :sm="6">
        <el-card shadow="hover" class="overview-card">
          <div class="overview-accent" style="background: #409EFF;"></div>
          <div class="overview-body">
            <div class="overview-header">
              <el-icon :size="18" color="#409EFF"><Box /></el-icon>
              <span class="overview-label">总容量</span>
            </div>
            <div class="overview-value">{{ formatBytes(overviewStats.totalSize) }}</div>
            <div class="overview-sub">{{ overviewStats.diskCount }} 块物理硬盘</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12" :sm="6">
        <el-card shadow="hover" class="overview-card">
          <div class="overview-accent" style="background: #E6A23C;"></div>
          <div class="overview-body">
            <div class="overview-header">
              <el-icon :size="18" color="#E6A23C"><FolderOpened /></el-icon>
              <span class="overview-label">已用空间</span>
            </div>
            <div class="overview-value">{{ formatBytes(overviewStats.totalUsed) }}</div>
            <div class="overview-sub">已挂载 {{ overviewStats.mountedCount }} 个分区</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12" :sm="6">
        <el-card shadow="hover" class="overview-card">
          <div class="overview-accent" style="background: #67C23A;"></div>
          <div class="overview-body">
            <div class="overview-header">
              <el-icon :size="18" color="#67C23A"><Coin /></el-icon>
              <span class="overview-label">可用空间</span>
            </div>
            <div class="overview-value">{{ formatBytes(overviewStats.totalAvail) }}</div>
            <div class="overview-sub">剩余可用</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6" :xs="12" :sm="6">
        <el-card shadow="hover" class="overview-card">
          <div class="overview-accent" style="background: #9C6ADE;"></div>
          <div class="overview-body">
            <div class="overview-header">
              <el-icon :size="18" color="#9C6ADE"><Files /></el-icon>
              <span class="overview-label">存储池数量</span>
            </div>
            <div class="overview-value">{{ overviewStats.diskCount }}</div>
            <div class="overview-sub">物理磁盘</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="12" class="chart-row">
      <el-col :span="12" :xs="24">
        <el-card shadow="never" class="chart-card">
          <template #header>
            <span class="chart-title">存储池容量分布</span>
          </template>
          <div ref="pieChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :span="12" :xs="24">
        <el-card shadow="never" class="chart-card">
          <template #header>
            <span class="chart-title">存储池容量对比</span>
          </template>
          <div ref="barChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
    </el-row>

    <div class="disk-group-list" v-loading="loading">
      <el-card
        v-for="disk in filteredTableData"
        :key="disk.id"
        class="disk-group-card"
        shadow="never"
      >
        <template #header>
          <div class="disk-group-header">
            <div class="disk-group-left">
              <el-button
                class="collapse-toggle-btn"
                :icon="isDiskCollapsed(disk.id) ? ArrowRight : ArrowDown"
                text
                size="small"
                @click="toggleDiskCollapse(disk.id)"
              />
              <div class="disk-group-info">
                <div class="disk-group-name-row">
                  <el-icon v-if="disk.type === 'vg' || disk.is_lvm_vg" class="disk-icon vg-icon" :size="20"><Connection /></el-icon>
                  <el-icon v-else-if="disk.is_zfs_pool" class="disk-icon" :size="20" color="#38BDF8"><Coin /></el-icon>
                  <el-icon v-else class="disk-icon" :size="20"><Box /></el-icon>
                  <span class="disk-group-name">{{ disk.display_name }}</span>
                  <el-tag v-if="disk.type === 'vg' || disk.is_lvm_vg" size="small" type="warning" effect="plain">VG</el-tag>
                  <el-tag v-if="disk.is_zfs_pool" size="small" type="success" effect="plain">ZFS</el-tag>
                  <el-tag v-if="disk.is_zfs_pool && disk.zfs_vdev_type" size="small" type="info" effect="plain">{{ zfsVdevLabel(disk.zfs_vdev_type) }}</el-tag>
                  <el-tag v-if="disk.is_default" size="small" type="success" effect="plain">默认</el-tag>
                  <el-tag v-if="disk.enabled" size="small" type="primary" effect="plain">已启用</el-tag>
                </div>
                <div class="disk-group-meta">
                  <template v-if="disk.device_path">
                    <span class="mono-text">{{ disk.device_path }}</span>
                    <span class="meta-sep">·</span>
                  </template>
                  <span>{{ typeLabel(disk.type) }}</span>
                  <template v-if="disk.pv_count > 0">
                    <span class="meta-sep">·</span>
                    <span>{{ disk.pv_count }} 个物理卷</span>
                  </template>
                  <template v-if="disk.lv_count > 0">
                    <span class="meta-sep">·</span>
                    <span>{{ disk.lv_count }} 个逻辑卷</span>
                  </template>
                  <template v-if="disk.model && disk.type !== 'vg' && !disk.is_lvm_vg && !disk.is_zfs_pool">
                    <span class="meta-sep">·</span>
                    <span>{{ disk.model }}</span>
                  </template>
                  <template v-if="disk.size > 0">
                    <span class="meta-sep">·</span>
                    <span>{{ formatBytes(disk.size) }}</span>
                  </template>
                </div>
              </div>
            </div>
            <div class="disk-group-actions">
              <template v-if="disk.type !== 'vg' && !disk.is_lvm_vg && !disk.is_zfs_pool">
                <el-button size="small" plain @click="openConfig(disk)">配置</el-button>
                <el-button size="small" plain type="primary" :disabled="!disk.can_use_for_vm || disk.is_default" @click="handleSetDefault(disk)">设为默认</el-button>
                <el-button size="small" plain type="warning" :disabled="!disk.can_format" @click="openFormat(disk)">格式化挂载</el-button>
                <el-button size="small" plain type="success" :disabled="!disk.can_format && !disk.configured" @click="openCreatePartition(disk)">创建分区</el-button>
                <el-button size="small" plain type="danger" :disabled="(!disk.children || disk.children.length === 0) && (!disk.mountpoints || disk.mountpoints.length === 0) || disk.system_disk || disk.readonly" @click="openDeletePartitions(disk)">清除磁盘</el-button>
              </template>
              <template v-if="disk.type === 'vg' || disk.is_lvm_vg">
                <el-button size="small" plain type="danger" :disabled="disk.system_disk" @click="openDeleteVolume(disk)">删除存储卷</el-button>
              </template>
              <template v-if="disk.is_zfs_pool">
                <el-button size="small" plain type="primary" @click="openZfsScrub(disk.name)">Scrub / 健康</el-button>
                <el-button size="small" plain type="warning" @click="openZfsExpand(disk)">扩容</el-button>
                <el-button size="small" plain type="success" @click="openCreateDataset(disk.name)">创建数据集</el-button>
                <el-button size="small" plain type="danger" :disabled="disk.system_disk" @click="openDeleteVolume(disk)">销毁存储池</el-button>
              </template>
            </div>
          </div>
        </template>

        <el-alert
          v-if="disk.has_existing_data"
          :title="disk.existing_data_warning"
          type="warning"
          :closable="false"
          show-icon
          class="existing-data-alert"
        />

        <div v-show="!isDiskCollapsed(disk.id)">
          <!-- LVM VG 卡片：分区域展示 PV 和 LV -->
          <template v-if="disk.type === 'vg' || disk.is_lvm_vg">
            <!-- PV 列表 -->
            <div v-if="hasPVChildren(disk)" class="vg-section">
              <div class="vg-section-title">
                <el-icon :size="14"><Box /></el-icon>
                <span>物理卷 (PV)</span>
              </div>
              <div
                v-for="part in flattenChildren(disk.children)"
                :key="part.id"
                v-show="part.type === 'pv'"
                class="partition-item"
                :style="{ paddingLeft: (20 + part.depth * 24) + 'px' }"
              >
                <div class="partition-main">
                  <div class="partition-name-row">
                    <el-icon v-if="part.depth > 0" class="sub-device-icon" :size="14"><Connection /></el-icon>
                    <span class="partition-name">{{ part.display_name }}</span>
                    <el-tag v-if="part.type === 'pv'" size="small" type="warning" effect="plain">PV</el-tag>
                    <el-tag size="small" type="info" effect="plain">{{ part.status_reason || '已加入卷组' }}</el-tag>
                  </div>
                  <div class="partition-meta">
                    <span class="mono-text">{{ part.device_path }}</span>
                    <template v-if="part.size > 0">
                      <span class="meta-sep">·</span>
                      <span>{{ formatBytes(part.size) }}</span>
                    </template>
                  </div>
                </div>
              </div>
            </div>

            <!-- LV 列表 -->
            <div v-if="hasLVChildren(disk)" class="vg-section">
              <div class="vg-section-title">
                <el-icon :size="14"><Files /></el-icon>
                <span>逻辑卷 (LV)</span>
              </div>
              <div
                v-for="part in flattenChildren(disk.children)"
                :key="part.id"
                v-show="part.type === 'lv'"
                class="partition-item"
                :style="{ paddingLeft: (20 + part.depth * 24) + 'px' }"
              >
                <div class="partition-main">
                  <div class="partition-name-row">
                    <el-icon v-if="part.depth > 0" class="sub-device-icon" :size="14"><Connection /></el-icon>
                    <span class="partition-name">{{ part.display_name }}</span>
                    <el-tag v-if="part.type === 'lv'" size="small" type="success" effect="plain">LV</el-tag>
                    <el-tag v-if="part.lv_type" size="small" type="warning" effect="plain">{{ part.lv_type }}</el-tag>
                    <el-tag v-if="part.is_default" size="small" type="success" effect="plain">默认</el-tag>
                    <el-tag v-if="part.enabled" size="small" type="primary" effect="plain">已启用</el-tag>
                    <el-tag v-if="part.can_use_for_vm" size="small" type="success" effect="plain">可用于虚拟机</el-tag>
                    <el-tooltip v-else-if="part.status_reason" :content="part.status_reason" placement="top">
                      <el-tag size="small" type="danger" effect="plain">{{ part.status_reason }}</el-tag>
                    </el-tooltip>
                  </div>
                  <div class="partition-meta">
                    <span class="mono-text">{{ part.device_path }}</span>
                    <span class="meta-sep">·</span>
                    <span>{{ part.fstype || '未知文件系统' }}</span>
                    <template v-if="part.mountpoints?.length">
                      <span class="meta-sep">·</span>
                      <span class="mono-text">{{ part.mountpoints.join(', ') }}</span>
                    </template>
                  </div>
                </div>
                <div class="partition-capacity" v-if="part.size > 0">
                  <el-progress
                    :percentage="part.use_percent || 0"
                    :stroke-width="8"
                    :color="progressColor(part.use_percent)"
                    :show-text="false"
                  />
                  <div class="partition-capacity-text">
                    <span :class="{ 'text-success': part.available > 0 }">{{ formatBytes(part.available) }} 可用</span>
                    <span class="capacity-sep">/</span>
                    <span>{{ formatBytes(part.size) }} 总计</span>
                  </div>
                </div>
                <div class="partition-actions" v-if="part.type === 'lv'">
                  <el-button size="small" plain @click="openConfig(part)">配置</el-button>
                  <el-button size="small" plain type="primary" :disabled="!part.can_use_for_vm || part.is_default" @click="handleSetDefault(part)">设为默认</el-button>
                  <el-button size="small" plain type="warning" :disabled="!part.can_format" @click="openFormat(part)">格式化挂载</el-button>
                </div>
              </div>
            </div>

            <el-empty v-if="!hasPVChildren(disk) && !hasLVChildren(disk)" description="无卷信息" :image-size="60" />
          </template>

          <!-- 非 VG 卡片：原有分区列表 -->
          <template v-else>
          <div v-if="disk.children && disk.children.length > 0" class="partition-list">
          <div
            v-for="part in flattenChildren(disk.children)"
            :key="part.id"
            class="partition-item"
            :style="{ paddingLeft: (20 + part.depth * 24) + 'px' }"
          >
            <div class="partition-main">
              <div class="partition-name-row">
                <el-icon v-if="part.depth > 0" class="sub-device-icon" :size="14"><Connection /></el-icon>
                <span class="partition-name">{{ part.display_name }}</span>
                <el-tag v-if="part.type === 'lvm'" size="small" type="warning" effect="plain">LVM</el-tag>
                <el-tag v-if="part.is_default" size="small" type="success" effect="plain">默认</el-tag>
                <el-tag v-if="part.enabled" size="small" type="primary" effect="plain">已启用</el-tag>
                <el-tag v-if="part.can_use_for_vm" size="small" type="success" effect="plain">可用于虚拟机</el-tag>
                <el-tooltip v-else-if="part.status_reason" :content="part.status_reason" placement="top">
                  <el-tag size="small" type="danger" effect="plain">{{ part.status_reason }}</el-tag>
                </el-tooltip>
              </div>
              <div class="partition-meta">
                <span class="mono-text">{{ part.device_path }}</span>
                <span class="meta-sep">·</span>
                <span>{{ part.fstype || '未知文件系统' }}</span>
                <template v-if="part.mountpoints?.length">
                  <span class="meta-sep">·</span>
                  <span class="mono-text">{{ part.mountpoints.join(', ') }}</span>
                </template>
              </div>
            </div>
            <div class="partition-capacity" v-if="part.size > 0">
              <el-progress
                :percentage="part.use_percent || 0"
                :stroke-width="8"
                :color="progressColor(part.use_percent)"
                :show-text="false"
              />
              <div class="partition-capacity-text">
                <span :class="{ 'text-success': part.available > 0 }">{{ formatBytes(part.available) }} 可用</span>
                <span class="capacity-sep">/</span>
                <span>{{ formatBytes(part.size) }} 总计</span>
              </div>
            </div>
            <div class="partition-actions" v-if="part.type !== 'zmember'">
              <el-button size="small" plain @click="openConfig(part)">配置</el-button>
              <el-button size="small" plain type="primary" :disabled="!part.can_use_for_vm || part.is_default" @click="handleSetDefault(part)">设为默认</el-button>
              <el-button size="small" plain type="warning" :disabled="!part.can_format" @click="openFormat(part)">格式化挂载</el-button>
              <el-button v-if="part.type === 'zdataset'" size="small" plain type="primary" @click="openZfsProperty(part.display_name)">属性</el-button>
              <el-button v-if="part.type === 'zdataset'" size="small" plain type="danger" @click="openDeleteDataset(part)">删除数据集</el-button>
            </div>
          </div>
          </div>
          <el-empty v-if="!disk.children || disk.children.length === 0" description="无分区信息" :image-size="60" />
          </template>
        </div>
      </el-card>
      <el-empty v-if="!loading && filteredTableData.length === 0" :description="tableData.length > 0 ? '当前没有符合条件的可用磁盘，可关闭「仅显示可用磁盘」查看全部' : '未发现存储设备'" />
    </div>

    <el-dialog title="配置存储池" v-model="configVisible" width="520px" :close-on-click-modal="false" append-to-body>
      <el-form :model="configForm" label-width="100px">
        <el-form-item label="设备">
          <el-input :model-value="currentRow?.device_path" disabled />
        </el-form-item>
        <el-form-item label="显示名称">
          <el-input v-model="configForm.display_name" placeholder="请输入用户侧显示名称" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="configForm.enabled" :disabled="!currentRow?.can_use_for_vm" active-text="允许用户创建到此硬盘" />
          <div v-if="!currentRow?.can_use_for_vm" class="form-tip">
            <el-icon><InfoFilled /></el-icon>
            {{ currentRow?.status_reason || '该硬盘当前不可用于虚拟机存储' }}
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="configVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingConfig" @click="saveConfig">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog title="格式化并挂载硬盘" v-model="formatVisible" width="560px" :close-on-click-modal="false" append-to-body>
      <el-alert type="error" :closable="false" show-icon class="danger-alert">
        <template #title>
          此操作会清空目标硬盘或分区上的全部数据，并写入开机自动挂载配置。
        </template>
      </el-alert>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="设备">{{ currentRow?.device_path }}</el-descriptions-item>
        <el-descriptions-item label="容量">{{ formatBytes(currentRow?.size) }}</el-descriptions-item>
        <el-descriptions-item label="当前文件系统">{{ currentRow?.fstype || '无' }}</el-descriptions-item>
        <el-descriptions-item label="挂载目录">/var/lib/kvm-storage/{{ currentRow?.id }}</el-descriptions-item>
      </el-descriptions>
      <el-form :model="{}" label-width="100px" style="margin-top: 16px;">
        <el-form-item label="文件系统">
          <el-select v-model="formatFSType" style="width: 100%;">
            <el-option label="ext4（推荐，稳定兼容）" value="ext4" />
            <el-option label="xfs（高性能，大文件优化）" value="xfs" />
            <el-option label="btrfs（快照/压缩等高级特性）" value="btrfs" />
          </el-select>
        </el-form-item>
      </el-form>
      <div class="confirm-line">
        <el-checkbox v-model="formatConfirmed">我确认要格式化该设备并挂载为虚拟机存储池</el-checkbox>
      </div>
      <template #footer>
        <el-button @click="formatVisible = false">取消</el-button>
        <el-button type="danger" :disabled="!formatConfirmed" :loading="formatting" @click="submitFormat">提交任务</el-button>
      </template>
    </el-dialog>

    <el-dialog title="创建分区" v-model="partitionVisible" width="520px" :close-on-click-modal="false" append-to-body>
      <el-alert type="warning" :closable="false" show-icon class="danger-alert">
        <template #title>
          此操作会在磁盘上创建新分区。若磁盘无分区表将自动创建 GPT 分区表。
        </template>
      </el-alert>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="设备">{{ currentRow?.device_path }}</el-descriptions-item>
        <el-descriptions-item label="容量">{{ formatBytes(currentRow?.size) }}</el-descriptions-item>
        <el-descriptions-item label="已有分区">{{ currentRow?.children?.length || 0 }} 个</el-descriptions-item>
      </el-descriptions>
      <el-form :model="partitionForm" label-width="100px" style="margin-top: 16px;">
        <el-form-item label="分区大小">
          <el-input-number
            v-model="partitionForm.size_gb"
            :min="0"
            :max="10000"
            :step="1"
            placeholder="留空则使用全部剩余空间"
            style="width: 100%;"
          />
          <div class="form-tip">
            <el-icon><InfoFilled /></el-icon>
            单位为 GB，输入 0 或留空表示使用磁盘全部剩余空间
          </div>
        </el-form-item>
      </el-form>
      <div class="confirm-line">
        <el-checkbox v-model="partitionConfirmed">我确认要在该磁盘上创建新分区</el-checkbox>
      </div>
      <template #footer>
        <el-button @click="partitionVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!partitionConfirmed" :loading="creatingPartition" @click="submitCreatePartition">提交任务</el-button>
      </template>
    </el-dialog>

    <el-dialog :title="deleteDialogTitle" v-model="deletePartitionsVisible" width="520px" :close-on-click-modal="false" append-to-body>
      <el-alert type="error" :closable="false" show-icon class="danger-alert">
        <template #title>
          {{ deleteDialogWarning }}
        </template>
      </el-alert>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="设备">{{ currentRow?.device_path }}</el-descriptions-item>
        <el-descriptions-item label="容量">{{ formatBytes(currentRow?.size) }}</el-descriptions-item>
        <el-descriptions-item v-if="(currentRow?.children?.length || 0) > 0" label="分区数">{{ currentRow?.children?.length }} 个</el-descriptions-item>
        <el-descriptions-item v-else-if="currentRow?.mountpoints?.length" label="挂载点">{{ currentRow?.mountpoints.join(', ') }}</el-descriptions-item>
        <el-descriptions-item v-if="currentRow?.fstype" label="文件系统">{{ currentRow.fstype }}</el-descriptions-item>
      </el-descriptions>
      <div class="confirm-line">
        <el-checkbox v-model="deletePartitionsConfirmed">{{ deleteConfirmText }}</el-checkbox>
      </div>
      <template #footer>
        <el-button @click="deletePartitionsVisible = false">取消</el-button>
        <el-button type="danger" :disabled="!deletePartitionsConfirmed" :loading="deletingPartitions" @click="submitDeletePartitions">提交任务</el-button>
      </template>
    </el-dialog>

    <!-- 创建存储池对话框 -->
    <el-dialog title="创建存储池" v-model="createVolumeVisible" width="680px" :close-on-click-modal="false" append-to-body>
      <!-- 第一步：选择存储卷类型 -->
      <template v-if="volumeStep === 'type'">
        <el-alert :type="volumeTypeHint.type" :closable="false" show-icon style="margin-bottom: 20px;">
          <template #title>{{ volumeTypeHint.text }}</template>
        </el-alert>
        <el-radio-group v-model="volumeType" size="large" style="width: 100%;">
          <el-row :gutter="12">
            <el-col :span="8">
              <el-card shadow="hover" class="volume-type-card" :class="{ selected: volumeType === 'lvm' }" @click="volumeType = 'lvm'">
                <div style="text-align: center; padding: 10px;">
                  <el-icon :size="36" color="#E6A23C"><Connection /></el-icon>
                  <h3 style="margin: 10px 0 4px;">LVM 存储卷</h3>
                  <p style="color: #999; font-size: 12px; margin: 0;">支持条带、镜像、多磁盘合并</p>
                </div>
              </el-card>
            </el-col>
            <el-col :span="8">
              <el-tooltip v-if="!zfsAvailable" :content="zfsUnavailableReason || '未安装 ZFS'" placement="top">
                <el-card shadow="hover" class="volume-type-card disabled-card" style="opacity: 0.4; cursor: not-allowed;">
                  <div style="text-align: center; padding: 10px;">
                    <el-icon :size="36" color="#909399"><Coin /></el-icon>
                    <h3 style="margin: 10px 0 4px;">ZFS 存储池</h3>
                    <p style="color: #999; font-size: 12px; margin: 0;">{{ zfsUnavailableReason || '未安装 ZFS' }}</p>
                  </div>
                </el-card>
              </el-tooltip>
              <el-card v-else shadow="hover" class="volume-type-card" :class="{ selected: volumeType === 'zfs' }" @click="volumeType = 'zfs'">
                <div style="text-align: center; padding: 10px;">
                  <el-icon :size="36" color="#38BDF8"><Coin /></el-icon>
                  <h3 style="margin: 10px 0 4px;">ZFS 存储池</h3>
                  <p style="color: #999; font-size: 12px; margin: 0;">支持镜像、RAIDZ、压缩、校验</p>
                </div>
              </el-card>
            </el-col>
            <el-col :span="8">
              <el-card shadow="hover" class="volume-type-card disabled-card" style="opacity: 0.4; cursor: not-allowed;">
                <div style="text-align: center; padding: 10px;">
                  <el-icon :size="36" color="#909399"><Box /></el-icon>
                  <h3 style="margin: 10px 0 4px;">Btrfs 存储池</h3>
                  <p style="color: #999; font-size: 12px; margin: 0;">即将推出</p>
                </div>
              </el-card>
            </el-col>
          </el-row>
        </el-radio-group>
        <div style="text-align: right; margin-top: 20px;">
          <el-button @click="createVolumeVisible = false">取消</el-button>
          <el-button type="primary" @click="volumeStep = 'config'" :disabled="!volumeType">下一步：配置卷</el-button>
        </div>
      </template>

      <!-- 第二步：LVM 配置表单 -->
      <template v-if="volumeStep === 'config' && volumeType === 'lvm'">
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px;">
          <template #title>此操作会将选中的磁盘初始化为 LVM 物理卷，并创建卷组和逻辑卷。磁盘上的所有数据将被清除。</template>
        </el-alert>

        <el-form :model="lvmForm" label-width="110px" label-position="top">
          <!-- 选择物理卷 -->
          <el-form-item label="物理卷 (PV) 选择">
            <div style="width: 100%;">
              <el-alert v-if="pvTargets.length === 0" type="info" :closable="false">
                未找到可用的磁盘设备。请确保有未挂载、非系统盘的磁盘。
              </el-alert>
              <el-checkbox-group v-model="lvmForm.device_ids" v-else>
                <el-card v-for="disk in pvTargets" :key="disk.id" shadow="never" class="pv-disk-item" style="margin-bottom: 8px;">
                  <el-checkbox :value="disk.id" style="width: 100%;">
                    <div style="display: flex; justify-content: space-between; align-items: center; width: 100%;">
                      <span style="font-weight: 500;">{{ disk.display_name }}</span>
                      <span style="color: #999; font-size: 12px;">{{ disk.device_path }} · {{ formatBytes(disk.size) }}</span>
                    </div>
                  </el-checkbox>
                </el-card>
              </el-checkbox-group>
            </div>
          </el-form-item>

          <!-- 卷组配置 -->
          <el-divider content-position="left">卷组 (VG) 配置</el-divider>
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="卷组名称">
                <el-input v-model="lvmForm.vg_name" placeholder="例如: vg-storage" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="PE 大小">
                <el-select v-model="lvmForm.pe_size" style="width: 100%;">
                  <el-option label="4M（默认，推荐）" value="4M" />
                  <el-option label="8M" value="8M" />
                  <el-option label="16M" value="16M" />
                  <el-option label="32M" value="32M" />
                  <el-option label="64M" value="64M" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <!-- 逻辑卷配置 -->
          <el-divider content-position="left">逻辑卷 (LV) 配置</el-divider>
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="逻辑卷名称">
                <el-input v-model="lvmForm.lv_name" placeholder="例如: lv-data" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="逻辑卷大小">
                <el-input v-model="lvmForm.lv_size" placeholder="10G / 50%VG / 100%FREE" />
                <div class="form-tip">
                  <el-icon><InfoFilled /></el-icon>
                  支持绝对值 (10G/500M) 或百分比 (50%VG/100%FREE)
                </div>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="LV 类型">
                <el-select v-model="lvmForm.lv_type" style="width: 100%;" @change="onLVTypeChange">
                  <el-option label="线性 (linear) — 顺序写入" value="linear" />
                  <el-option label="条带 (striped) — 并行写入，高性能" value="striped" />
                  <el-option label="镜像 (mirrored) — 数据冗余" value="mirrored" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item v-if="lvmForm.lv_type === 'striped'" label="条带数">
                <el-input-number v-model="lvmForm.stripes" :min="2" :max="16" style="width: 100%;" />
              </el-form-item>
              <el-form-item v-if="lvmForm.lv_type === 'mirrored'" label="镜像数">
                <el-input-number v-model="lvmForm.mirrors" :min="1" :max="3" style="width: 100%;" />
              </el-form-item>
            </el-col>
          </el-row>

          <!-- 文件系统与挂载 -->
          <el-divider content-position="left">文件系统与挂载</el-divider>
          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label="文件系统">
                <el-select v-model="lvmForm.fs_type" style="width: 100%;">
                  <el-option label="ext4（推荐）" value="ext4" />
                  <el-option label="xfs" value="xfs" />
                  <el-option label="btrfs" value="btrfs" />
                  <el-option label="不格式化" value="none" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="10">
              <el-form-item label="挂载路径">
                <el-input v-model="lvmForm.mount_path" placeholder="留空则自动生成 /var/lib/kvm-storage/..." />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="开机自动挂载">
                <el-switch v-model="lvmForm.add_fstab" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>

        <div class="confirm-line" style="margin-top: 16px;">
          <el-checkbox v-model="volumeConfirmed">我确认要创建 LVM 存储卷，选中的磁盘数据将被清除</el-checkbox>
        </div>

        <div style="text-align: right; margin-top: 20px;">
          <el-button @click="volumeStep = 'type'">上一步</el-button>
          <el-button @click="createVolumeVisible = false">取消</el-button>
          <el-button type="primary" :disabled="!volumeConfirmed || !lvmForm.vg_name || !lvmForm.lv_name || !lvmForm.lv_size || lvmForm.device_ids.length === 0" :loading="creatingVolume" @click="submitCreateVolume">提交任务</el-button>
        </div>
      </template>

      <!-- 第二步：ZFS 配置表单 -->
      <template v-if="volumeStep === 'config' && volumeType === 'zfs'">
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px;">
          <template #title>此操作会将选中的磁盘创建为 ZFS 存储池，并创建专用数据集作为虚拟机磁盘目录。磁盘上的所有数据将被清除。</template>
        </el-alert>

        <el-form :model="zfsForm" label-width="110px" label-position="top">
          <!-- 选择成员盘 -->
          <el-form-item label="成员磁盘选择">
            <div style="width: 100%;">
              <el-alert v-if="pvTargets.length === 0" type="info" :closable="false">
                未找到可用的磁盘设备。请确保有未挂载、非系统盘的磁盘。
              </el-alert>
              <el-checkbox-group v-model="zfsForm.device_ids" v-else>
                <el-card v-for="disk in pvTargets" :key="disk.id" shadow="never" class="pv-disk-item" style="margin-bottom: 8px;">
                  <el-checkbox :value="disk.id" style="width: 100%;">
                    <div style="display: flex; justify-content: space-between; align-items: center; width: 100%;">
                      <span style="font-weight: 500;">{{ disk.display_name }}</span>
                      <span style="color: #999; font-size: 12px;">{{ disk.device_path }} · {{ formatBytes(disk.size) }}</span>
                    </div>
                  </el-checkbox>
                </el-card>
              </el-checkbox-group>
              <div class="form-tip" v-if="pvTargets.length > 0">
                <el-icon><InfoFilled /></el-icon>
                当前选择 {{ zfsForm.device_ids.length }} 块，{{ zfsVdevLabel(zfsForm.vdev_type) }} 至少需要 {{ zfsVdevMinDisks(zfsForm.vdev_type) }} 块
                <span v-if="!zfsDisksEnough" style="color: #f56c6c;">（不足）</span>
              </div>
            </div>
          </el-form-item>

          <!-- 存储池配置 -->
          <el-divider content-position="left">存储池配置</el-divider>
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="存储池名称">
                <el-input v-model="zfsForm.pool_name" placeholder="例如: tank" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="拓扑 (vdev)">
                <el-select v-model="zfsForm.vdev_type" style="width: 100%;">
                  <el-option label="条带/单盘 (stripe)" value="stripe" />
                  <el-option label="镜像 (mirror) — 2 块起" value="mirror" />
                  <el-option label="RAIDZ1 — 3 块起" value="raidz1" />
                  <el-option label="RAIDZ2 — 4 块起" value="raidz2" />
                  <el-option label="RAIDZ3 — 5 块起" value="raidz3" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="扇区对齐 (ashift)">
                <el-select v-model="zfsForm.ashift" style="width: 100%;">
                  <el-option label="12（4K，默认推荐）" value="12" />
                  <el-option label="13（8K）" value="13" />
                  <el-option label="9（512B）" value="9" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="压缩">
                <el-select v-model="zfsForm.compression" style="width: 100%;">
                  <el-option label="lz4（推荐）" value="lz4" />
                  <el-option label="zstd" value="zstd" />
                  <el-option label="gzip" value="gzip" />
                  <el-option label="关闭" value="off" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <!-- 数据集与挂载 -->
          <el-divider content-position="left">数据集与挂载</el-divider>
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="数据集名称">
                <el-input v-model="zfsForm.dataset_name" placeholder="默认 vm-disks" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="挂载路径">
                <el-input v-model="zfsForm.mount_path" placeholder="留空则自动生成 /var/lib/kvm-storage/..." />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="关闭 atime">
            <el-switch v-model="zfsForm.atime_off" />
            <span style="color: #999; font-size: 12px; margin-left: 8px;">关闭可提升性能（推荐）</span>
          </el-form-item>
          <el-alert type="info" :closable="false" show-icon style="margin-top: 4px;">
            <template #title>ZFS 会在开机时自动导入并挂载存储池，无需写入 fstab。请确保宿主机已安装 zfsutils-linux 并启用 zfs.target。</template>
          </el-alert>
        </el-form>

        <div class="confirm-line" style="margin-top: 16px;">
          <el-checkbox v-model="volumeConfirmed">我确认要创建 ZFS 存储池，选中的磁盘数据将被清除</el-checkbox>
        </div>

        <div style="text-align: right; margin-top: 20px;">
          <el-button @click="volumeStep = 'type'">上一步</el-button>
          <el-button @click="createVolumeVisible = false">取消</el-button>
          <el-button type="primary" :disabled="!volumeConfirmed || !zfsForm.pool_name || zfsForm.device_ids.length === 0 || !zfsDisksEnough" :loading="creatingVolume" @click="submitCreateVolume">提交任务</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 删除存储卷确认对话框 -->
    <el-dialog :title="deleteVolumeDialogTitle" v-model="deleteVolumeVisible" width="520px" :close-on-click-modal="false" append-to-body>
      <el-alert type="error" :closable="false" show-icon style="margin-bottom: 16px;">
        <template #title>
          {{ deleteVolumeWarning }}
        </template>
      </el-alert>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item :label="deletingVolumeDisk?.is_zfs_pool ? '存储池名称' : '卷组名称'">{{ deletingVolumeDisk?.name }}</el-descriptions-item>
        <el-descriptions-item label="总容量">{{ formatBytes(deletingVolumeDisk?.size) }}</el-descriptions-item>
        <template v-if="!deletingVolumeDisk?.is_zfs_pool">
          <el-descriptions-item label="逻辑卷数">{{ deletingVolumeDisk?.lv_count || 0 }} 个</el-descriptions-item>
          <el-descriptions-item label="物理卷数">{{ deletingVolumeDisk?.pv_count || 0 }} 个</el-descriptions-item>
        </template>
        <template v-else>
          <el-descriptions-item label="拓扑">{{ zfsVdevLabel(deletingVolumeDisk?.zfs_vdev_type) }}</el-descriptions-item>
        </template>
      </el-descriptions>
      <div class="confirm-line" style="margin-top: 16px;">
        <el-checkbox v-model="deleteVolumeConfirmed">{{ deleteVolumeConfirmText }}</el-checkbox>
      </div>
      <template #footer>
        <el-button @click="deleteVolumeVisible = false">取消</el-button>
        <el-button type="danger" :disabled="!deleteVolumeConfirmed" :loading="deletingVolume" @click="submitDeleteVolume">提交任务</el-button>
      </template>
    </el-dialog>

    <!-- 创建 ZFS 数据集对话框 -->
    <el-dialog title="创建 ZFS 数据集" v-model="createDatasetVisible" width="480px" append-to-body :close-on-click-modal="false">
      <el-form label-width="100px">
        <el-form-item label="存储池" required>
          <el-select v-model="datasetForm.pool" placeholder="选择 ZFS 存储池" style="width:100%">
            <el-option v-for="p in zpoolList" :key="p" :label="p" :value="p" />
          </el-select>
        </el-form-item>
        <el-form-item label="数据集名称" required>
          <el-input v-model="datasetForm.name" placeholder="如 vm-storage 或 a/b/c（支持嵌套）" />
        </el-form-item>
        <div style="font-size:12px;color:var(--el-text-color-secondary);margin-left:100px;">将在所选存储池下创建数据集（如 zp01/vm-storage），可用于 VM 落盘。</div>
      </el-form>
      <template #footer>
        <el-button @click="createDatasetVisible = false">取消</el-button>
        <el-button type="primary" :loading="creatingDataset" @click="handleCreateDataset">创建</el-button>
      </template>
    </el-dialog>

    <!-- ZFS Scrub / 健康 对话框 -->
    <ZfsScrubDialog ref="zfsScrubDialogRef" />

    <!-- ZFS Dataset 属性 对话框 -->
    <ZfsPropertyDialog ref="zfsPropertyDialogRef" />

    <!-- ZFS 存储池扩容 对话框 -->
    <ZfsExpandDialog ref="zfsExpandDialogRef" @success="fetchData" />
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, Box, Refresh, FolderOpened, Coin, Files, Connection, ArrowRight, ArrowDown, Plus } from '@element-plus/icons-vue'
import { getStoragePoolList, updateStoragePoolConfig, setDefaultStoragePool, formatMountStoragePool, createStoragePartition, deleteStoragePartitions, getAvailablePVTargets, createLVMVolume, deleteLVMVolume, getZFSStatus, createZFSPool, createZFSDataset, deleteZFSDataset, deleteZFSPool } from '@/api/infra'
import ZfsScrubDialog from '@/components/ZfsScrubDialog.vue'
import ZfsPropertyDialog from '@/components/ZfsPropertyDialog.vue'
import ZfsExpandDialog from '@/components/ZfsExpandDialog.vue'
import * as echarts from 'echarts'

const tableData = ref([])
const loading = ref(false)
const showAvailableOnly = ref(true)
const configVisible = ref(false)
const formatVisible = ref(false)
const savingConfig = ref(false)
const formatting = ref(false)
const formatConfirmed = ref(false)
const formatFSType = ref('ext4')
const currentRow = ref(null)

// 创建 ZFS 数据集
const createDatasetVisible = ref(false)
const zfsScrubDialogRef = ref(null)
const openZfsScrub = (poolName) => {
  zfsScrubDialogRef.value?.open(poolName)
}
const zfsPropertyDialogRef = ref(null)
const openZfsProperty = (dataset) => { zfsPropertyDialogRef.value?.open(dataset) }
const zfsExpandDialogRef = ref(null)
const openZfsExpand = (disk) => {
  zfsExpandDialogRef.value?.open(disk.name, disk.expand_vdev_type)
}
const creatingDataset = ref(false)
const datasetForm = ref({ pool: '', name: '' })
const zpoolList = computed(() => tableData.value.filter(n => n.type === 'zpool').map(n => n.name))
const openCreateDataset = (poolName) => {
  datasetForm.value = { pool: poolName || zpoolList.value[0] || '', name: '' }
  createDatasetVisible.value = true
}
const handleCreateDataset = async () => {
  if (!datasetForm.value.pool || !datasetForm.value.name.trim()) {
    ElMessage.warning('请选择存储池并填写数据集名称')
    return
  }
  creatingDataset.value = true
  try {
    const res = await createZFSDataset({ pool: datasetForm.value.pool, name: datasetForm.value.name.trim() })
    ElMessage.success(res.message || '数据集已创建')
    createDatasetVisible.value = false
    fetchData()
  } catch (e) {} finally { creatingDataset.value = false }
}

const openDeleteDataset = async (part) => {
  try {
    await ElMessageBox.confirm(`确认删除数据集 ${part.display_name}？其数据将不可恢复！`, '删除数据集', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch { return }
  try {
    const res = await deleteZFSDataset({ name: part.display_name })
    ElMessage.success(res.message || '数据集已删除')
    fetchData()
  } catch (e) {}
}

// 折叠状态：记录哪些磁盘卡片被折叠
const collapsedDisks = reactive(new Set())

// 创建分区相关状态
const partitionVisible = ref(false)
const creatingPartition = ref(false)
const partitionConfirmed = ref(false)
const partitionForm = reactive({
  size_gb: 0,
})

// 删除分区相关状态
const deletePartitionsVisible = ref(false)
const deletingPartitions = ref(false)
const deletePartitionsConfirmed = ref(false)

// 创建存储池相关状态
const createVolumeVisible = ref(false)
const creatingVolume = ref(false)
const volumeStep = ref('type') // 'type' | 'config'
const volumeType = ref('lvm')
const volumeConfirmed = ref(false)
const pvTargets = ref([])
const pvTargetsLoading = ref(false)
const zfsAvailable = ref(false)
const zfsUnavailableReason = ref('')
const lvmForm = reactive({
  device_ids: [],
  vg_name: '',
  pe_size: '4M',
  lv_name: '',
  lv_size: '',
  lv_type: 'linear',
  stripes: 2,
  mirrors: 1,
  fs_type: 'ext4',
  mount_path: '',
  add_fstab: true,
})
const zfsForm = reactive({
  device_ids: [],
  pool_name: '',
  vdev_type: 'stripe',
  ashift: '12',
  compression: 'lz4',
  dataset_name: 'vm-disks',
  mount_path: '',
  atime_off: true,
})

// ZFS vdev 类型所需的最少磁盘数
const zfsVdevMinDisks = (vdev) => {
  const map = { stripe: 1, mirror: 2, raidz1: 3, raidz2: 4, raidz3: 5 }
  return map[vdev] || 1
}
const zfsVdevLabel = (vdev) => {
  const map = {
    stripe: '条带/单盘',
    mirror: '镜像 (mirror)',
    raidz1: 'RAIDZ1',
    raidz2: 'RAIDZ2',
    raidz3: 'RAIDZ3',
  }
  return map[vdev] || vdev
}
const zfsDisksEnough = computed(() => zfsForm.device_ids.length >= zfsVdevMinDisks(zfsForm.vdev_type))

// 根据当前选中的存储卷类型，在卡片上方给出不同的说明提示
const volumeTypeHint = computed(() => {
  if (volumeType.value === 'zfs') {
    if (!zfsAvailable.value) {
      return { type: 'warning', text: 'ZFS 存储池：' + (zfsUnavailableReason.value || '宿主机未安装 zfsutils-linux，无法创建。') }
    }
    return {
      type: 'success',
      text: 'ZFS 存储池：基于多块磁盘构建镜像或 RAIDZ 阵列，自带压缩、数据校验与自愈能力，数据安全性高，适合存放重要数据。',
    }
  }
  return {
    type: 'info',
    text: 'LVM 存储卷：将多块磁盘组合为卷组并划分逻辑卷，支持条带、镜像与容量灵活扩展，适合通用存储场景。',
  }
})

// 清除磁盘弹窗的动态文案
const hasDeleteChildren = computed(() => currentRow.value?.children?.length > 0)
const deleteDialogTitle = computed(() => hasDeleteChildren.value ? '删除所有分区' : '清除磁盘挂载')
const deleteDialogWarning = computed(() =>
  hasDeleteChildren.value
    ? '此操作将卸载并删除该磁盘上的所有分区，清除分区表，相关数据将不可恢复！'
    : '此操作将卸载该磁盘并清除文件系统签名，相关数据将不可恢复！'
)
const deleteConfirmText = computed(() =>
  hasDeleteChildren.value
    ? '我确认要删除该磁盘上的所有分区'
    : '我确认要清除该磁盘上的挂载并擦除数据'
)

const pieChartRef = ref(null)
const barChartRef = ref(null)
let pieChart = null
let barChart = null

const configForm = reactive({
  display_name: '',
  enabled: false,
})

const fetchData = async () => {
  loading.value = true
  try {
    const res = await getStoragePoolList()
    tableData.value = res.data || []
    // 自动折叠没有分区的磁盘
    collapsedDisks.clear()
    for (const disk of tableData.value) {
      if (!disk.children || disk.children.length === 0) {
        collapsedDisks.add(disk.id)
      }
    }
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)

const filteredTableData = computed(() => {
  if (!showAvailableOnly.value) return tableData.value
  return tableData.value.filter(disk => {
    // 已建 ZFS 存储池/数据集：开关打开时隐藏（仅显示可用磁盘，便于找空盘建新池）
    if (disk.is_zfs_pool) return false
    // 已配置的存储池始终显示
    if (disk.configured) return true
    // 可格式化的盘显示（可用作新存储池）
    if (disk.can_format) return true
    return false
  })
})

const openConfig = (row) => {
  currentRow.value = row
  configForm.display_name = row.display_name || ''
  configForm.enabled = !!row.enabled
  configVisible.value = true
}

const saveConfig = async () => {
  if (!currentRow.value) return
  savingConfig.value = true
  try {
    await updateStoragePoolConfig(currentRow.value.id, {
      display_name: configForm.display_name,
      enabled: configForm.enabled,
    })
    ElMessage.success('存储池配置已保存')
    configVisible.value = false
    fetchData()
  } finally {
    savingConfig.value = false
  }
}

const handleSetDefault = async (row) => {
  try {
    await ElMessageBox.confirm(`确定将 ${row.display_name} 设为默认虚拟机存储位置吗？`, '提示', { type: 'warning' })
    await setDefaultStoragePool(row.id)
    ElMessage.success('已设为默认存储位置')
    fetchData()
  } catch {}
}

const openFormat = (row) => {
  currentRow.value = row
  formatConfirmed.value = false
  formatFSType.value = 'ext4'
  formatVisible.value = true
}

const submitFormat = async () => {
  if (!currentRow.value) return
  formatting.value = true
  try {
    await formatMountStoragePool(currentRow.value.id, formatFSType.value)
    ElMessage.success('格式化并挂载任务已提交，请在任务中心查看进度')
    formatVisible.value = false
  } finally {
    formatting.value = false
  }
}

// ===== 折叠/展开 =====
const toggleDiskCollapse = (diskId) => {
  if (collapsedDisks.has(diskId)) {
    collapsedDisks.delete(diskId)
  } else {
    collapsedDisks.add(diskId)
  }
}

const isDiskCollapsed = (diskId) => {
  return collapsedDisks.has(diskId)
}

// ===== 创建分区 =====
const openCreatePartition = (disk) => {
  currentRow.value = disk
  partitionForm.size_gb = 0
  partitionConfirmed.value = false
  partitionVisible.value = true
}

const submitCreatePartition = async () => {
  if (!currentRow.value) return
  creatingPartition.value = true
  try {
    await createStoragePartition(currentRow.value.id, {
      size_gb: partitionForm.size_gb || 0,
    })
    ElMessage.success('创建分区任务已提交，请在任务中心查看进度')
    partitionVisible.value = false
  } finally {
    creatingPartition.value = false
  }
}

// ===== 删除所有分区 =====
const openDeletePartitions = (disk) => {
  currentRow.value = disk
  deletePartitionsConfirmed.value = false
  deletePartitionsVisible.value = true
}

const submitDeletePartitions = async () => {
  if (!currentRow.value) return
  deletingPartitions.value = true
  try {
    await deleteStoragePartitions(currentRow.value.id)
    ElMessage.success('删除分区任务已提交，请在任务中心查看进度')
    deletePartitionsVisible.value = false
    // 删除后刷新列表
    fetchData()
  } finally {
    deletingPartitions.value = false
  }
}

// ===== 创建 LVM 存储卷 =====
const openCreateVolume = async () => {
  volumeStep.value = 'type'
  volumeType.value = 'lvm'
  volumeConfirmed.value = false
  createVolumeVisible.value = true

  // 预加载可用 PV 列表
  pvTargetsLoading.value = true
  try {
    const res = await getAvailablePVTargets()
    pvTargets.value = res.data || []
  } catch (err) {
    console.error(err)
    pvTargets.value = []
  } finally {
    pvTargetsLoading.value = false
  }

  // 检测 ZFS 可用性
  try {
    const res = await getZFSStatus()
    zfsAvailable.value = !!res.data?.available
    zfsUnavailableReason.value = res.data?.reason || ''
  } catch (err) {
    console.error(err)
    zfsAvailable.value = false
    zfsUnavailableReason.value = '检测 ZFS 可用性失败'
  }
}

const onLVTypeChange = (type) => {
  if (type === 'striped') {
    lvmForm.stripes = lvmForm.stripes || 2
  }
  if (type === 'mirrored') {
    lvmForm.mirrors = lvmForm.mirrors || 1
  }
}

const submitCreateVolume = async () => {
  creatingVolume.value = true
  try {
    if (volumeType.value === 'zfs') {
      await createZFSPool({
        device_ids: zfsForm.device_ids,
        pool_name: zfsForm.pool_name,
        vdev_type: zfsForm.vdev_type,
        ashift: zfsForm.ashift,
        compression: zfsForm.compression,
        dataset_name: zfsForm.dataset_name,
        mount_path: zfsForm.mount_path,
        atime_off: zfsForm.atime_off,
      })
      ElMessage.success('创建 ZFS 存储池任务已提交，请在任务中心查看进度')
      createVolumeVisible.value = false
      zfsForm.device_ids = []
      zfsForm.pool_name = ''
      zfsForm.vdev_type = 'stripe'
      zfsForm.ashift = '12'
      zfsForm.compression = 'lz4'
      zfsForm.dataset_name = 'vm-disks'
      zfsForm.mount_path = ''
      zfsForm.atime_off = true
      return
    }
    await createLVMVolume({
      device_ids: lvmForm.device_ids,
      vg_name: lvmForm.vg_name,
      pe_size: lvmForm.pe_size,
      lv_name: lvmForm.lv_name,
      lv_size: lvmForm.lv_size,
      lv_type: lvmForm.lv_type,
      stripes: lvmForm.stripes,
      mirrors: lvmForm.mirrors,
      fs_type: lvmForm.fs_type,
      mount_path: lvmForm.mount_path,
      add_fstab: lvmForm.add_fstab,
    })
    ElMessage.success('创建 LVM 存储卷任务已提交，请在任务中心查看进度')
    createVolumeVisible.value = false
    // 重置表单
    lvmForm.device_ids = []
    lvmForm.vg_name = ''
    lvmForm.lv_name = ''
    lvmForm.lv_size = ''
    lvmForm.lv_type = 'linear'
    lvmForm.stripes = 2
    lvmForm.mirrors = 1
    lvmForm.fs_type = 'ext4'
    lvmForm.mount_path = ''
    lvmForm.add_fstab = true
  } finally {
    creatingVolume.value = false
  }
}

// ===== 删除 LVM/ZFS 存储卷 =====
const deleteVolumeVisible = ref(false)
const deletingVolume = ref(false)
const deleteVolumeConfirmed = ref(false)
const deletingVolumeDisk = ref(null)

const deleteVolumeDialogTitle = computed(() =>
  deletingVolumeDisk.value?.is_zfs_pool ? '销毁 ZFS 存储池' : '删除存储卷'
)
const deleteVolumeWarning = computed(() =>
  deletingVolumeDisk.value?.is_zfs_pool
    ? `此操作将销毁 ZFS 存储池「${deletingVolumeDisk.value?.name}」及其所有数据集，数据将不可恢复！`
    : `此操作将删除卷组「${deletingVolumeDisk.value?.name}」及其所有逻辑卷和物理卷，数据将不可恢复！`
)
const deleteVolumeConfirmText = computed(() =>
  deletingVolumeDisk.value?.is_zfs_pool
    ? '我确认要销毁该 ZFS 存储池及其所有数据集'
    : '我确认要删除该卷组及其所有逻辑卷和物理卷'
)

const openDeleteVolume = (disk) => {
  deletingVolumeDisk.value = disk
  deleteVolumeConfirmed.value = false
  deleteVolumeVisible.value = true
}

const submitDeleteVolume = async () => {
  const disk = deletingVolumeDisk.value
  if (!disk?.name) return
  deletingVolume.value = true
  try {
    if (disk.is_zfs_pool) {
      await deleteZFSPool(disk.name)
      ElMessage.success('销毁 ZFS 存储池任务已提交，请在任务中心查看进度')
    } else {
      await deleteLVMVolume(disk.name)
      ElMessage.success('删除 LVM 存储卷任务已提交，请在任务中心查看进度')
    }
    deleteVolumeVisible.value = false
    fetchData()
  } finally {
    deletingVolume.value = false
  }
}

const formatBytes = (bytes) => {
  if (!bytes || bytes <= 0) return '0'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let val = Number(bytes)
  let idx = 0
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024
    idx += 1
  }
  return `${val.toFixed(2)} ${units[idx]}`
}

const progressColor = (pct = 0) => {
  if (pct >= 90) return '#f56c6c'
  if (pct >= 70) return '#e6a23c'
  return '#409eff'
}

const typeLabel = (type) => {
  const map = { disk: '硬盘', part: '分区', lvm: 'LVM', loop: 'Loop', rom: '光驱', vg: '卷组', lv: '逻辑卷', pv: '物理卷', zpool: 'ZFS 池', zdataset: 'ZFS 数据集', zmember: '成员盘' }
  return map[type] || type || '-'
}

const collectLeafNodes = (nodes) => {
  const leaves = []
  for (const node of nodes) {
    if (!node.children || node.children.length === 0) {
      leaves.push(node)
    } else {
      leaves.push(...collectLeafNodes(node.children))
    }
  }
  return leaves
}

const isDarkMode = () => {
  return document.documentElement.classList.contains('dark')
}

const getThemeColor = (varName, fallback) => {
  const val = getComputedStyle(document.documentElement).getPropertyValue(varName).trim()
  return val || fallback
}

const leafPools = computed(() => collectLeafNodes(tableData.value))

const mountedLeafPools = computed(() => leafPools.value.filter(p => p.mountpoints && p.mountpoints.length > 0))

// 存储单元（总览/图表用）：zpool 用 pool 级总量（zpool list），不递归子数据集；
// 非 zfs（物理盘/分区）递归取叶子。避免 zfs 数据集共享池空间导致逐个加总重复计算。
const storageUnits = computed(() => {
  const units = []
  for (const node of tableData.value) {
    if (node.type === 'zpool') {
      units.push(node)
    } else {
      // 非 zpool 节点递归取叶子，跳过 zfs 挂载点（它们由 zpool 节点代表，避免重复）
      units.push(...collectLeafNodes([node]).filter(p => !p.fstype?.startsWith('zfs')))
    }
  }
  return units
})
const mountedStorageUnits = computed(() =>
  storageUnits.value.filter(p => p.type === 'zpool' || (p.mountpoints && p.mountpoints.length > 0))
)

const overviewStats = computed(() => {
  const all = storageUnits.value
  const mounted = mountedStorageUnits.value
  const totalSize = all.reduce((sum, p) => sum + (p.size || 0), 0)
  const totalUsed = mounted.reduce((sum, p) => sum + (p.used || 0), 0)
  const totalAvail = mounted.reduce((sum, p) => sum + (p.available || 0), 0)
  return {
    totalSize,
    totalUsed,
    totalAvail,
    diskCount: tableData.value.filter(d => d.type === 'disk').length,
    mountedCount: mounted.length,
  }
})

const pieChartData = computed(() => {
  return storageUnits.value
    .filter(p => p.size > 0)
    .map(p => ({
      name: p.display_name,
      value: p.size,
      used: p.used || 0,
      available: p.available || 0,
      usePercent: p.use_percent || 0,
    }))
})

const barChartData = computed(() => {
  return mountedStorageUnits.value
    .filter(p => p.size > 0)
    .map(p => ({
      name: p.display_name,
      used: p.used || 0,
      available: p.available || 0,
      total: p.size,
      usePercent: p.use_percent || 0,
    }))
})

const flattenChildren = (nodes, depth = 0) => {
  const result = []
  for (const node of nodes) {
    result.push({ ...node, depth })
    if (node.children && node.children.length > 0) {
      result.push(...flattenChildren(node.children, depth + 1))
    }
  }
  return result
}

const hasPVChildren = (disk) => {
  if (!disk.children || disk.children.length === 0) return false
  return disk.children.some(c => c.type === 'pv')
}

const hasLVChildren = (disk) => {
  if (!disk.children || disk.children.length === 0) return false
  return disk.children.some(c => c.type === 'lv')
}

const PALETTE = ['#409EFF', '#67C23A', '#E6A23C', '#9C6ADE', '#F56C6C', '#00B8D4']

const initCharts = () => {
  if (pieChartRef.value) {
    pieChart = echarts.init(pieChartRef.value)
  }
  if (barChartRef.value) {
    barChart = echarts.init(barChartRef.value)
  }
  updateCharts()
}

const updateCharts = () => {
  if (!pieChart && !barChart) return

  const dark = isDarkMode()
  const textColor = dark ? '#A3A6AD' : '#606266'
  const primaryTextColor = dark ? '#E5EAF3' : '#303133'
  const gridLineColor = dark ? 'rgba(255,255,255,0.08)' : 'rgba(150,150,150,0.15)'
  const availableColor = dark ? '#3A3A3C' : '#E5E7EB'
  const isMobile = window.innerWidth < 768

  if (pieChart) {
    const pieData = pieChartData.value
    const totalSize = overviewStats.value.totalSize
    pieChart.setOption({
      tooltip: {
        trigger: 'item',
        formatter: (params) => {
          const d = params.data
          const usedGB = (d.used / 1024 / 1024 / 1024).toFixed(2)
          const availGB = (d.available / 1024 / 1024 / 1024).toFixed(2)
          return `<b>${d.name}</b><br/>容量占比: ${params.percent}%<br/>已用: ${usedGB} GB<br/>可用: ${availGB} GB<br/>使用率: ${d.usePercent}%`
        },
        backgroundColor: dark ? '#2A2A2C' : '#fff',
        borderColor: dark ? '#4A4A4C' : '#E5E7EB',
        textStyle: { color: primaryTextColor },
      },
      legend: {
        show: !isMobile,
        orient: 'vertical',
        right: '5%',
        top: 'center',
        textStyle: { color: textColor, fontSize: 12 },
        itemWidth: 12,
        itemHeight: 12,
        itemGap: 8,
      },
      series: [{
        type: 'pie',
        radius: isMobile ? ['40%', '65%'] : ['45%', '70%'],
        center: isMobile ? ['50%', '50%'] : ['40%', '50%'],
        avoidLabelOverlap: true,
        padAngle: 2,
        itemStyle: { borderRadius: 6 },
        label: {
          show: !isMobile,
          formatter: '{b}\n{d}%',
          fontSize: 11,
          color: textColor,
          lineHeight: 16,
        },
        labelLine: { show: !isMobile },
        emphasis: {
          itemStyle: { shadowBlur: 10, shadowOffsetX: 0, shadowColor: 'rgba(0, 0, 0, 0.2)' },
        },
        data: pieData.map((d, i) => ({
          ...d,
          itemStyle: { color: PALETTE[i % PALETTE.length] },
        })),
        graphic: [{
          type: 'text',
          left: isMobile ? 'center' : '35%',
          top: 'center',
          style: {
            text: formatBytes(totalSize),
            textAlign: 'center',
            fill: primaryTextColor,
            fontSize: 18,
            fontWeight: 'bold',
          },
        }, {
          type: 'text',
          left: isMobile ? 'center' : '35%',
          top: isMobile ? '58%' : '58%',
          style: {
            text: '总容量',
            textAlign: 'center',
            fill: textColor,
            fontSize: 12,
          },
        }],
      }],
    })
  }

  if (barChart) {
    const barData = barChartData.value
    if (isMobile) {
      barChart.setOption({
        tooltip: {
          trigger: 'axis',
          axisPointer: { type: 'shadow' },
          formatter: (params) => {
            const d = barData[params[0]?.dataIndex]
            if (!d) return ''
            return `<b>${d.name}</b><br/>已用: ${formatBytes(d.used)} (${d.usePercent}%)<br/>可用: ${formatBytes(d.available)}<br/>总计: ${formatBytes(d.total)}`
          },
          backgroundColor: dark ? '#2A2A2C' : '#fff',
          borderColor: dark ? '#4A4A4C' : '#E5E7EB',
          textStyle: { color: primaryTextColor },
        },
        grid: { left: '4%', right: '12%', top: '10%', bottom: '10%', containLabel: true },
        xAxis: {
          type: 'value',
          axisLabel: { show: false },
          splitLine: { show: false },
          axisLine: { show: false },
        },
        yAxis: {
          type: 'category',
          data: barData.map(d => ''),
          axisLine: { show: false },
          axisTick: { show: false },
        },
        series: [
          {
            name: '已用',
            type: 'bar',
            stack: 'total',
            barWidth: 20,
            data: barData.map(d => ({
              value: d.used,
              itemStyle: { color: progressColor(d.usePercent), borderRadius: [4, 0, 0, 4] },
            })),
            label: {
              show: true,
              position: 'inside',
              formatter: (p) => {
                const d = barData[p.dataIndex]
                return d.usePercent >= 15 ? `${d.usePercent}%` : ''
              },
              fontSize: 10,
              color: '#fff',
            },
          },
          {
            name: '可用',
            type: 'bar',
            stack: 'total',
            data: barData.map(d => ({
              value: d.available,
              itemStyle: { color: availableColor, borderRadius: [0, 4, 4, 0] },
            })),
            label: {
              show: true,
              position: 'right',
              formatter: (p) => {
                const d = barData[p.dataIndex]
                return d.name
              },
              fontSize: 10,
              color: textColor,
              distance: 8,
            },
          },
        ],
      })
    } else {
      barChart.setOption({
        tooltip: {
          trigger: 'axis',
          axisPointer: { type: 'shadow' },
          formatter: (params) => {
            const d = barData[params[0]?.dataIndex]
            if (!d) return ''
            return `<b>${d.name}</b><br/>已用: ${formatBytes(d.used)} (${d.usePercent}%)<br/>可用: ${formatBytes(d.available)}<br/>总计: ${formatBytes(d.total)}`
          },
          backgroundColor: dark ? '#2A2A2C' : '#fff',
          borderColor: dark ? '#4A4A4C' : '#E5E7EB',
          textStyle: { color: primaryTextColor },
        },
        legend: {
          data: ['已用', '可用'],
          top: 0,
          textStyle: { color: textColor },
          itemWidth: 12,
          itemHeight: 12,
        },
        grid: { left: '3%', right: '6%', top: '15%', bottom: '10%', containLabel: true },
        xAxis: {
          type: 'category',
          data: barData.map(d => d.name),
          axisLabel: { color: textColor, fontSize: 11, rotate: barData.length > 5 ? 15 : 0 },
          axisLine: { lineStyle: { color: gridLineColor } },
        },
        yAxis: {
          type: 'value',
          axisLabel: {
            color: textColor,
            fontSize: 11,
            formatter: (v) => {
              if (v >= 1024 * 1024 * 1024 * 1024) return (v / 1024 / 1024 / 1024 / 1024).toFixed(1) + ' TB'
              if (v >= 1024 * 1024 * 1024) return (v / 1024 / 1024 / 1024).toFixed(1) + ' GB'
              if (v >= 1024 * 1024) return (v / 1024 / 1024).toFixed(0) + ' MB'
              return v
            },
          },
          splitLine: { lineStyle: { color: gridLineColor } },
        },
        series: [
          {
            name: '已用',
            type: 'bar',
            stack: 'total',
            barWidth: 32,
            data: barData.map(d => ({
              value: d.used,
              itemStyle: { color: progressColor(d.usePercent), borderRadius: [4, 0, 0, 4] },
            })),
            label: {
              show: true,
              position: 'inside',
              formatter: (p) => {
                const d = barData[p.dataIndex]
                return d.usePercent >= 10 ? `${d.usePercent}%` : ''
              },
              fontSize: 11,
              color: '#fff',
            },
          },
          {
            name: '可用',
            type: 'bar',
            stack: 'total',
            data: barData.map(d => ({
              value: d.available,
              itemStyle: { color: availableColor, borderRadius: [0, 4, 4, 0] },
            })),
          },
        ],
      })
    }
  }
}

const handleChartResize = () => {
  pieChart?.resize()
  barChart?.resize()
}

let themeObserver = null

watch(tableData, () => {
  nextTick(() => {
    updateCharts()
  })
})

onMounted(() => {
  nextTick(() => {
    initCharts()
  })
  window.addEventListener('resize', handleChartResize)
  themeObserver = new MutationObserver(() => {
    updateCharts()
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})

onUnmounted(() => {
  window.removeEventListener('resize', handleChartResize)
  if (themeObserver) {
    themeObserver.disconnect()
  }
  pieChart?.dispose()
  barChart?.dispose()
})
</script>

<style scoped>
.storage-pool-page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.storage-pool-page :deep(.el-table__expand-icon) {
  vertical-align: middle;
}

.page-header-bar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 20px 10px 16px;
}

.page-header-left .page-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.page-header-left .page-icon {
  font-size: 22px;
  color: var(--el-color-primary);
}

.page-header-left h2 {
  margin: 0;
  font-size: 19px;
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
  gap: 10px;
  flex-shrink: 0;
}

.overview-row {
  margin: 0 10px 12px;
}

.overview-card {
  border-radius: 12px;
  border: none;
  overflow: hidden;
  transition: transform .2s, box-shadow .2s;
}

.overview-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

.overview-card :deep(.el-card__body) {
  padding: 0;
}

.overview-accent {
  height: 3px;
}

.overview-body {
  padding: 14px 16px;
}

.overview-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.overview-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  font-weight: 500;
}

.overview-value {
  font-size: 20px;
  font-weight: 800;
  color: var(--el-text-color-primary);
  line-height: 1.2;
}

.overview-sub {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  margin-top: 2px;
}

.chart-row {
  margin: 0 10px 12px;
}

.chart-card {
  border-radius: 12px;
  border: none;
}

.chart-card :deep(.el-card__header) {
  padding: 12px 16px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.chart-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.chart-container {
  width: 100%;
  height: 300px;
}

/* ===== 磁盘分组卡片 ===== */
.disk-group-list {
  margin: 0 10px 10px;
}

.disk-group-card {
  border-radius: 12px;
  border: 1px solid var(--el-border-color-lighter);
  margin-bottom: 16px;
  overflow: hidden;
}

.disk-group-card :deep(.el-card__header) {
  padding: 16px 20px;
  background: var(--el-fill-color-lighter);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.disk-group-card :deep(.el-card__body) {
  padding: 0;
}

.disk-group-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.disk-group-left {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  flex: 1;
  min-width: 0;
}

.collapse-toggle-btn {
  flex-shrink: 0;
  margin-top: 2px;
  padding: 2px;
  color: var(--el-text-color-secondary);
}

.collapse-toggle-btn:hover {
  color: var(--el-color-primary);
}

.disk-group-info {
  flex: 1;
  min-width: 0;
}

.disk-group-name-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.disk-icon {
  color: var(--el-color-primary);
  flex-shrink: 0;
}

.disk-group-name {
  font-size: 16px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.disk-group-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.meta-sep {
  color: var(--el-text-color-placeholder);
  margin: 0 4px;
}

.disk-group-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

/* ===== 分区列表 ===== */
.partition-list {
  display: flex;
  flex-direction: column;
}

.partition-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
  transition: background-color .15s;
}

.partition-item:last-child {
  border-bottom: none;
}

.partition-item:hover {
  background: var(--el-fill-color-lighter);
}

.partition-main {
  flex: 1;
  min-width: 0;
}

.partition-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.partition-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.sub-device-icon {
  color: var(--el-text-color-placeholder);
  flex-shrink: 0;
}

.partition-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.partition-capacity {
  flex-shrink: 0;
  width: 180px;
}

.partition-capacity-text {
  display: flex;
  align-items: baseline;
  gap: 2px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.capacity-sep {
  color: var(--el-text-color-placeholder);
  margin: 0 2px;
}

.text-success {
  color: #67C23A;
}

.partition-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.mono-text {
  font-family: 'SF Mono', 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
}

.form-tip {
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  display: flex;
  align-items: center;
  gap: 4px;
}

.confirm-line {
  margin-top: 16px;
}

.danger-alert {
  margin-bottom: 16px;
}

.filter-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-right: 8px;
  white-space: nowrap;
}

.existing-data-alert {
  margin: 12px 20px 0;
}

@media (max-width: 900px) {
  .disk-group-header {
    flex-direction: column;
    gap: 12px;
  }

  .disk-group-actions {
    width: 100%;
  }

  .partition-item {
    flex-wrap: wrap;
    gap: 12px;
  }

  .partition-capacity {
    width: 100%;
    order: 3;
  }

  .partition-actions {
    width: 100%;
    order: 4;
  }
}

@media (max-width: 768px) {
  .overview-row {
    margin: 0 4px 8px;
  }

  .chart-row {
    margin: 0 4px 8px;
  }

  .chart-container {
    height: 220px;
  }

  .overview-value {
    font-size: 18px;
  }

  .disk-group-list {
    margin: 0 4px 4px;
  }

  .disk-group-card :deep(.el-card__header) {
    padding: 12px 16px;
  }

  .partition-item {
    padding: 12px 16px;
  }

  .partition-actions {
    flex-wrap: wrap;
  }
}

@media (max-width: 480px) {
  .overview-body {
    padding: 10px 12px;
  }

  .overview-value {
    font-size: 16px;
  }

  .overview-label {
    font-size: 12px;
  }

  .chart-container {
    height: 200px;
  }

  .disk-group-name {
    font-size: 14px;
  }

  .disk-group-meta {
    font-size: 12px;
  }
}

/* ===== LVM VG 卡片样式 ===== */
.vg-icon {
  color: #E6A23C;
}

.vg-section {
  border-top: 1px solid var(--el-border-color-lighter);
  padding: 8px 0;
}

.vg-section:first-child {
  border-top: none;
}

.vg-section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 16px 6px 12px;
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* ===== 创建存储池对话框样式 ===== */
.volume-type-card {
  cursor: pointer;
  border: 2px solid transparent;
  transition: all 0.2s;
}

.volume-type-card:hover {
  border-color: var(--el-color-primary-light-5);
}

.volume-type-card.selected {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.pv-disk-item {
  border: 1px solid var(--el-border-color-light);
  margin-bottom: 4px;
}

.pv-disk-item :deep(.el-card__body) {
  padding: 8px 12px;
}
</style>
