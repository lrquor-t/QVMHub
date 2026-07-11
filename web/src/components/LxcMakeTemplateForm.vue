<template>
  <el-dialog v-model="visible" title="制作模板" width="520px" :close-on-click-modal="false" append-to-body>
    <el-form ref="formRef" :model="form" :rules="rules" label-width="120px">
      <el-form-item label="源容器">
        <el-input :model-value="form.src_name" disabled />
      </el-form-item>
      <el-form-item label="模板名" prop="name">
        <el-input v-model="form.name" placeholder="小写字母/数字/连字符，2-63 字符" />
      </el-form-item>
      <el-form-item label="显示名">
        <el-input v-model="form.display_name" placeholder="选填，默认用模板名" />
      </el-form-item>
      <el-form-item label="描述">
        <el-input v-model="form.description" type="textarea" :rows="2" placeholder="选填" />
      </el-form-item>
      <el-form-item label="克隆后初始化命令">
        <el-input v-model="form.post_create_command" type="textarea" :rows="2" placeholder="选填" />
        <div class="form-hint">
          <el-icon><InfoFilled /></el-icon>
          每次从该模板克隆出新容器后执行（经 lxc-attach）
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">制作</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { InfoFilled } from '@element-plus/icons-vue'
import { makeLXCTemplateFromContainer } from '@/api/lxc'

const emit = defineEmits(['success'])
const visible = ref(false)
const submitting = ref(false)
const formRef = ref(null)
const form = reactive({
  src_name: '',
  name: '',
  display_name: '',
  description: '',
  post_create_command: ''
})

const rules = {
  name: [
    { required: true, message: '请输入模板名', trigger: 'blur' },
    { pattern: /^[a-z0-9][a-z0-9-]{1,62}$/, message: '小写字母/数字/连字符，2-63 字符', trigger: 'blur' }
  ]
}

const open = (row) => {
  Object.assign(form, { src_name: row?.name || '', name: '', display_name: '', description: '', post_create_command: '' })
  formRef.value?.clearValidate()
  visible.value = true
}
defineExpose({ open })

const submit = async () => {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  submitting.value = true
  try {
    await makeLXCTemplateFromContainer(form.src_name, {
      name: form.name,
      display_name: form.display_name,
      description: form.description,
      post_create_command: form.post_create_command
    })
    ElMessage.success('模板制作任务已提交，完成后将出现在「LXC 模板」管理页')
    visible.value = false
    emit('success')
  } catch (e) {
    // request 拦截器已弹错误提示
  } finally {
    submitting.value = false
  }
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
</style>
