<template>
  <div class="quota-form">
    <!-- 计算资源 -->
    <el-divider content-position="left">
      <span class="section-title">计算资源</span>
      <el-text type="info" size="small" style="margin-left: 8px;">（设为 0 表示不限制）</el-text>
    </el-divider>
    <el-row :gutter="20">
      <el-col :md="8" :sm="12" :xs="24">
        <el-form-item label="CPU 核心数">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_cpu" :min="0" :max="256" controls-position="right" style="flex: 1;" />
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_cpu > 0">
                {{ usage.used_cpu || 0 }}/{{ form.max_cpu }}
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_cpu || 0) / form.max_cpu * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usagePercentage(form.max_cpu, usage.used_cpu) >= 80 ? 'warning' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_cpu || 0 }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
      <el-col :md="8" :sm="12" :xs="24">
        <el-form-item label="内存 (GB)">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_memory" :min="0" :max="4096" controls-position="right" style="flex: 1;" />
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_memory > 0">
                {{ usage.used_memory || 0 }}/{{ form.max_memory }}GB
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_memory || 0) / form.max_memory * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usagePercentage(form.max_memory, usage.used_memory) >= 80 ? 'warning' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_memory || 0 }}GB</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
      <el-col :md="8" :sm="12" :xs="24">
        <el-form-item label="VM 数量">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_vm" :min="0" :max="1000" controls-position="right" style="flex: 1;" />
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_vm > 0">
                {{ usage.used_vm || 0 }}/{{ form.max_vm }}
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_vm || 0) / form.max_vm * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usagePercentage(form.max_vm, usage.used_vm) >= 80 ? 'warning' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_vm || 0 }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <!-- 存储资源 -->
    <el-divider content-position="left">
      <span class="section-title">存储资源</span>
      <el-text type="info" size="small" style="margin-left: 8px;">（设为 0 表示不限制）</el-text>
    </el-divider>
    <el-row :gutter="20">
      <el-col :md="12" :sm="12" :xs="24">
        <el-form-item label="磁盘 (GB)">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_disk" :min="0" :max="102400" controls-position="right" style="flex: 1;" />
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_disk > 0">
                {{ usage.used_disk || 0 }}/{{ form.max_disk }}GB
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_disk || 0) / form.max_disk * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usagePercentage(form.max_disk, usage.used_disk) >= 80 ? 'warning' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_disk || 0 }}GB</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
      <el-col :md="12" :sm="12" :xs="24">
        <el-form-item label="存储配额 (GB)">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_storage" :min="0" :max="102400" controls-position="right" style="flex: 1;" />
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_storage > 0">
                {{ usage.used_storage_gb || '0 B' }}/{{ form.max_storage }}GB
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_storage || 0) / (form.max_storage * 1073741824) * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usagePercentage(form.max_storage, (usage.used_storage || 0) / 1073741824) >= 80 ? 'warning' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_storage_gb || '0 B' }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <!-- 运行时长配额 -->
    <el-divider content-position="left">
      <span class="section-title">运行时长配额</span>
      <el-text type="info" size="small" style="margin-left: 8px;">（设为 0 表示不限制，耗尽后无法开机）</el-text>
    </el-divider>
    <el-row :gutter="20">
      <el-col :span="12">
        <el-form-item label="总运行时长 (小时)">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_runtime_hours" :min="0" :max="1000000" controls-position="right" style="flex: 1;" />
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_runtime_hours > 0">
                {{ usage.used_runtime_display || '0秒' }}/{{ form.max_runtime_hours }}小时
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_runtime_seconds || 0) / (form.max_runtime_hours * 3600) * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usage.runtime_quota_reached ? 'exception' : (usagePercentage(form.max_runtime_hours, usage.used_runtime_seconds / 3600) >= 80 ? 'warning' : '')"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_runtime_display || '0秒' }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <!-- 网络资源 -->
    <el-divider content-position="left">
      <span class="section-title">网络资源</span>
      <el-text type="info" size="small" style="margin-left: 8px;">（设为 0 表示不限制）</el-text>
    </el-divider>

    <!-- 端口转发与公网 IP -->
    <el-row :gutter="20">
      <el-col :span="12">
        <el-form-item label="端口转发">
          <div class="quota-field-row">
            <el-switch v-model="form.enable_port_forward" active-text="开通" inactive-text="关闭" />
            <span v-if="showUsage && usage && form.enable_port_forward" class="quota-used-tag" style="margin-left: 8px;">
              <template v-if="form.max_port_forwards > 0">
                {{ usage.used_port_forwards || 0 }}/{{ form.max_port_forwards }}
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_port_forwards || 0) / form.max_port_forwards * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_port_forwards || 0 }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
      <el-col :span="12">
        <el-form-item v-if="form.enable_port_forward" label="转发上限">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_port_forwards" :min="0" :max="100000" controls-position="right" style="flex: 1;" />
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-form-item label="公网 IP">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_public_ips" :min="0" :max="10000" controls-position="right" style="flex: 1;" />
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_public_ips > 0">
                {{ usage.used_public_ips || 0 }}/{{ form.max_public_ips }}
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_public_ips || 0) / form.max_public_ips * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usagePercentage(form.max_public_ips, usage.used_public_ips) >= 80 ? 'warning' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_public_ips || 0 }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-form-item label="快照数量">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_snapshots" :min="0" :max="100000" controls-position="right" style="flex: 1;" />
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_snapshots > 0">
                {{ usage.used_snapshots || 0 }}/{{ form.max_snapshots }}
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_snapshots || 0) / form.max_snapshots * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usagePercentage(form.max_snapshots, usage.used_snapshots) >= 80 ? 'warning' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_snapshots || 0 }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <!-- 带宽配额 -->
    <el-divider content-position="left" style="margin-top: 8px;">
      <span class="section-subtitle">带宽配额</span>
      <el-text type="info" size="small" style="margin-left: 8px;">（Mbps，设为 0 表示不限制）</el-text>
    </el-divider>
    <el-row :gutter="20">
      <el-col :sm="12" :xs="24">
        <el-form-item label="下行带宽">
          <el-input-number v-model="form.max_bandwidth_down" :min="0" :max="100000" controls-position="right" style="width: 100%;">
            <template #suffix>Mbps</template>
          </el-input-number>
        </el-form-item>
      </el-col>
      <el-col :sm="12" :xs="24">
        <el-form-item label="上行带宽">
          <el-input-number v-model="form.max_bandwidth_up" :min="0" :max="100000" controls-position="right" style="width: 100%;">
            <template #suffix>Mbps</template>
          </el-input-number>
        </el-form-item>
      </el-col>
    </el-row>

    <!-- 流量配额 -->
    <el-divider content-position="left" style="margin-top: 8px;">
      <span class="section-subtitle">流量配额</span>
      <el-text type="info" size="small" style="margin-left: 8px;">（GB/月，超限后限速：下行10Mbps/上行1Mbps）</el-text>
    </el-divider>
    <el-row :gutter="20">
      <el-col :sm="12" :xs="24">
        <el-form-item label="下行流量">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_traffic_down" :min="0" :max="1000000" :precision="2" controls-position="right" style="flex: 1;">
              <template #suffix>GB</template>
            </el-input-number>
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_traffic_down > 0">
                {{ usage.used_traffic_down_gb || '0 B' }}/{{ form.max_traffic_down }}GB
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_traffic_down || 0) / (form.max_traffic_down * 1073741824) * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usage.is_limited_down ? 'exception' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_traffic_down_gb || '0 B' }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
      <el-col :sm="12" :xs="24">
        <el-form-item label="上行流量">
          <div class="quota-field-row">
            <el-input-number v-model="form.max_traffic_up" :min="0" :max="1000000" :precision="2" controls-position="right" style="flex: 1;">
              <template #suffix>GB</template>
            </el-input-number>
            <span v-if="showUsage && usage" class="quota-used-tag">
              <template v-if="form.max_traffic_up > 0">
                {{ usage.used_traffic_up_gb || '0 B' }}/{{ form.max_traffic_up }}GB
                <el-progress
                  :percentage="Math.min(Math.round((usage.used_traffic_up || 0) / (form.max_traffic_up * 1073741824) * 100), 100)"
                  :stroke-width="4" :show-text="false"
                  :status="usage.is_limited_up ? 'exception' : ''"
                  style="margin-top: 2px;"
                />
              </template>
              <template v-else>已用 {{ usage.used_traffic_up_gb || '0 B' }}</template>
            </span>
          </div>
        </el-form-item>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
const props = defineProps({
  form: { type: Object, required: true },
  showUsage: { type: Boolean, default: false },
  usage: { type: Object, default: null }
})

function usagePercentage(max, used) {
  if (!max || max <= 0) return 0
  return Math.min(Math.round((used || 0) / max * 100), 100)
}


</script>

<style scoped>
.quota-form {
}

.section-title {
  font-weight: 600;
  font-size: 14px;
}

.section-subtitle {
  font-weight: 500;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.quota-field-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.quota-used-tag {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
  min-width: 90px;
  text-align: right;
}
</style>
