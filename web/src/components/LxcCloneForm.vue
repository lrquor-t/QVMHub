<template>
  <el-dialog v-model="visible" title="克隆容器" width="520px" :close-on-click-modal="false" append-to-body>
    <el-alert v-if="!isZfs" type="warning" :closable="false" show-icon style="margin-bottom: 12px">
      该容器非 zfs 存储，暂不支持快照克隆（仅 zfs 后端支持）
    </el-alert>
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
      <el-form-item label="源容器">
        <el-input :model-value="form.src_name" disabled />
      </el-form-item>
      <el-form-item label="快照" prop="snap">
        <el-select v-model="form.snap" placeholder="选择一个快照" :loading="loadingSnaps" :disabled="!isZfs" style="width: 100%">
          <el-option
            v-for="s in snapshots"
            :key="s.name"
            :label="`${s.name}${s.comment ? ' · ' + s.comment : ''}`"
            :value="s.name"
          />
          <template #empty>
            <div class="snap-empty" @click="gotoSnapshot">
              <el-icon><Plus /></el-icon>
              暂无快照，去创建快照
            </div>
          </template>
        </el-select>
        <div v-if="!loadingSnaps && isZfs && snapshots.length === 0" class="form-hint form-hint-link" @click="gotoSnapshot">
          <el-icon><WarningFilled /></el-icon>
          该容器暂无快照，<span class="link">去创建快照 →</span>
        </div>
      </el-form-item>
      <el-form-item label="新容器名" prop="dst_name">
        <el-input v-model="form.dst_name" placeholder="小写字母/数字/连字符，2-63 字符">
          <template #append><el-button @click="handleGenerateName">随机生成</el-button></template>
        </el-input>
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="form.remark" placeholder="选填" maxlength="200" show-word-limit />
      </el-form-item>
      <div class="form-hint">
        <el-icon><InfoFilled /></el-icon>
        其余规格（CPU/内存/自启/分组/网络）继承自源容器，仅刷新 MAC 与主机名
      </div>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" :disabled="!isZfs || snapshots.length === 0" @click="submit">克隆</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { InfoFilled, WarningFilled, Plus } from '@element-plus/icons-vue'
import { listLXCSnapshots, cloneLXCFromSnapshot, listLXCInterfaces } from '@/api/lxc'
import { getVPCSwitches } from '@/api/vpc'
import { generateRandomLXCName } from '@/utils/lxc'

const emit = defineEmits(['success', 'goto-snapshot'])
const visible = ref(false)
const submitting = ref(false)
const loadingSnaps = ref(false)
const snapshots = ref([])
const srcRow = ref(null)
const formRef = ref(null)
const form = reactive({
  src_name: '',
  backing: '',
  snap: '',
  dst_name: '',
  remark: '',
})
const isZfs = computed(() => (form.backing || '').toLowerCase() === 'zfs')
const vpcSwitches = ref([])
const srcPrimarySwitch = computed(() => vpcSwitches.value.find(s => s.id === srcPrimarySwitchId.value) || null)
const loadSrcPrimary = async (srcName) => {
  srcPrimarySwitchId.value = 0
  vpcSwitches.value = []
  try {
    const [ifaces, sw] = await Promise.all([listLXCInterfaces(srcName), getVPCSwitches()])
    vpcSwitches.value = sw.data || []
    const primary = (ifaces.data || []).find(i => i.is_primary)
    if (primary) srcPrimarySwitchId.value = primary.switch_id || 0
  } catch (e) {}
}

const rules = {
  snap: [{ required: true, message: '请选择一个快照', trigger: 'change' }],
  dst_name: [
    { required: true, message: '请输入新容器名', trigger: 'blur' },
    { pattern: /^[a-z0-9][a-z0-9-]{1,62}$/, message: '小写字母/数字/连字符，2-63 字符', trigger: 'blur' }
  ]
}

const open = async (row) => {
  srcRow.value = row
  Object.assign(form, {
    src_name: row?.name || '',
    backing: row?.backing || '',
    snap: '',
    dst_name: generateRandomLXCName(), // 打开时先随机生成一个新名，用户可改
    remark: '',
  })
  formRef.value?.clearValidate()
  snapshots.value = []
  srcPrimarySwitchId.value = 0
  vpcSwitches.value = []
  visible.value = true
  if (!isZfs.value) return
  loadingSnaps.value = true
  try {
    const res = await listLXCSnapshots(form.src_name)
    snapshots.value = res.data || []
  } catch (e) {} finally { loadingSnaps.value = false }
}
defineExpose({ open })

// 无快照时跳转到管理抽屉的「快照」tab 创建一个
const gotoSnapshot = () => {
  visible.value = false
  emit('goto-snapshot', srcRow.value)
}
// 重新随机一个新容器名
const handleGenerateName = async () => {
  form.dst_name = generateRandomLXCName()
  await formRef.value?.validateField('dst_name').catch(() => false)
}

const submit = async () => {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  submitting.value = true
  try {
    await cloneLXCFromSnapshot(form.src_name, {
      snap: form.snap,
      dst_name: form.dst_name,
      remark: form.remark,
    })
    ElMessage.success('克隆任务已提交，可在任务中心查看进度')
    visible.value = false
    emit('success')
  } catch (e) {} finally { submitting.value = false }
}
</script>

<style scoped>
.form-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
.form-hint-warn {
  color: var(--el-color-warning);
}
.form-hint-link {
  cursor: pointer;
}
.form-hint-link .link {
  color: var(--el-color-primary);
}
.snap-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 8px 0;
  cursor: pointer;
  color: var(--el-color-primary);
}
</style>
