<template>
  <div class="lxc-snapshot-panel">
    <div class="snap-toolbar">
      <div class="snap-toolbar-right">
        <el-button type="primary" :icon="Plus" @click="openCreate">创建快照</el-button>
        <el-button :icon="Refresh" @click="fetchData">刷新</el-button>
      </div>
    </div>

    <el-card shadow="hover" class="snap-card">
      <div class="section-title">快照列表（{{ tableData.length }}）</div>
      <el-table :data="tableData" border v-loading="loading">
        <el-table-column prop="name" label="名称" min-width="170" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.name }}</span>
            <el-tag v-if="row.has_clones" size="small" type="info" effect="plain" style="margin-left: 6px">已被克隆</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="200">
          <template #default="{ row }">{{ row.created_at || '-' }}</template>
        </el-table-column>
        <el-table-column label="备注" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ row.comment || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="180" align="center">
          <template #default="{ row, $index }">
            <el-button size="small" type="warning" @click="handleRestore(row, $index)">恢复</el-button>
            <el-tooltip
              v-if="row.has_clones"
              content="该快照已被克隆，需先删除克隆出的容器后才能删除"
              placement="top"
            >
              <el-button size="small" type="info" disabled>删除</el-button>
            </el-tooltip>
            <el-button v-else size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无快照" />
        </template>
      </el-table>
    </el-card>

    <!-- 创建快照弹窗（备注输入框） -->
    <el-dialog v-model="createVisible" title="创建快照" width="460px" append-to-body>
      <el-form label-width="60px" class="snap-create-form" @submit.prevent>
        <el-form-item label="备注">
          <el-input
            v-model="createComment"
            placeholder="可选备注"
            maxlength="200"
            show-word-limit
            clearable
            :disabled="creating"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreate">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import { listLXCSnapshots, createLXCSnapshot, restoreLXCSnapshot, deleteLXCSnapshot } from '@/api/lxc'

const props = defineProps({
  name: { type: String, required: true },
  status: { type: String, default: '' },
  backing: { type: String, default: '' }
})

const tableData = ref([])
const loading = ref(false)
const creating = ref(false)
const createVisible = ref(false)
const createComment = ref('')
let timer = null

const fetchData = async () => {
  if (!props.name) return
  loading.value = true
  try {
    const res = await listLXCSnapshots(props.name)
    tableData.value = res.data || []
  } catch (e) {} finally { loading.value = false }
}

// name 变化（抽屉切容器）→ 重新拉取
watch(() => props.name, (v) => { if (v) fetchData() })
onMounted(() => {
  fetchData()
  // 轻量自动刷新（与 list.vue 的 5s 自动刷新一致）：创建是异步任务，5s 内新快照会出现
  timer = setInterval(fetchData, 5000)
})
onBeforeUnmount(() => { if (timer) clearInterval(timer) })

const openCreate = () => {
  createComment.value = ''
  createVisible.value = true
}

const submitCreate = async () => {
  creating.value = true
  try {
    await createLXCSnapshot(props.name, createComment.value)
    ElMessage.success('快照任务已提交，可在任务中心查看进度')
    createVisible.value = false
    setTimeout(fetchData, 500)
  } catch (e) {} finally { creating.value = false }
}

const handleRestore = async (row, index) => {
  // 运行中拦截（zfs 回滚 + dir 恢复都应先关机）
  if (props.status === 'RUNNING') {
    ElMessage.warning('容器正在运行，请先关机再恢复快照')
    return
  }
  // zfs 回滚会销毁更新的快照 → 二次确认列出将被销毁的快照名
  // tableData 已是新→旧；index 之前（更新）的快照会被 zfs rollback -r 销毁
  let msg = `确定恢复到快照 [${row.name}] 吗？当前数据将被快照内容覆盖。`
  if (props.backing === 'zfs' && index > 0) {
    const newer = tableData.value.slice(0, index).map(s => s.name).join('、')
    msg = `恢复到快照 [${row.name}] 将一并销毁其后创建的快照（zfs 回滚）：\n${newer}\n\n此操作不可撤销，确定继续吗？`
  }
  try {
    await ElMessageBox.confirm(msg, '恢复快照', { type: 'warning', confirmButtonText: '确定恢复', cancelButtonText: '取消' })
    await restoreLXCSnapshot(props.name, row.name)
    ElMessage.success('已提交恢复')
    fetchData()
  } catch (e) {}
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确定删除快照 [${row.name}] 吗？`, '删除快照', { type: 'warning' })
    await deleteLXCSnapshot(props.name, row.name)
    ElMessage.success('已删除')
    fetchData()
  } catch (e) {}
}
</script>

<style scoped>
.lxc-snapshot-panel {
  padding: 4px 2px;
}
.snap-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 14px;
}
.snap-toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 快照列表卡片（与配置面板同节奏） */
.snap-card {
  border-radius: 12px;
  border: none;
  transition: box-shadow 0.2s var(--app-transition-fast, 0.15s);
}
.snap-card :deep(.el-card__body) {
  padding: 16px 18px;
}
.section-title {
  font-size: 16px;
  font-weight: 700;
  padding-left: 10px;
  border-left: 4px solid var(--el-color-primary);
  margin-bottom: 14px;
  color: var(--el-text-color-primary);
}

/* 创建弹窗表单与标题保持距离 */
.snap-create-form {
  padding-top: 12px;
}
</style>
