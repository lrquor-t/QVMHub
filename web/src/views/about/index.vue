<template>
  <div class="about-container">
    <el-collapse v-model="activeSections" class="about-collapse">
      <!-- 技术栈 -->
      <el-collapse-item name="tech">
        <template #title>
          <div class="section-header">
            <el-icon class="section-icon"><Cpu /></el-icon>
            <span class="section-title">技术栈</span>
          </div>
        </template>
        <div class="tech-grid">
          <a
            v-for="tech in techStack"
            :key="tech.name"
            :href="tech.url"
            target="_blank"
            rel="noopener"
            class="tech-item"
          >
            <img :src="tech.logo" :alt="tech.name" class="tech-logo" />
            <div class="tech-info">
              <span class="tech-name">{{ tech.name }}</span>
              <span class="tech-desc">{{ tech.desc }}</span>
            </div>
          </a>
        </div>
      </el-collapse-item>

      <!-- 项目信息 -->
      <el-collapse-item name="project">
        <template #title>
          <div class="section-header">
            <el-icon class="section-icon"><Link /></el-icon>
            <span class="section-title">项目信息</span>
          </div>
        </template>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">开源地址</span>
            <a class="info-link" href="https://github.com/QVMConsole/QVMConsole" target="_blank" rel="noopener">
              https://github.com/QVMConsole/QVMConsole
            </a>
          </div>
          <div class="info-item">
            <span class="info-label">开发者</span>
            <span class="info-value">
              星辰项目组-又菜又爱玩的小朱
              <a class="info-link-inline" href="https://github.com/yxsj245" target="_blank" rel="noopener">
                (@yxsj245)
              </a>
            </span>
          </div>
          <div class="info-item">
            <span class="info-label">项目官网</span>
            <a class="info-link" href="https://www.qvmconsole.cn/" target="_blank" rel="noopener">
              https://www.qvmconsole.cn/
            </a>
          </div>
          <div class="info-item">
            <span class="info-label">项目文档</span>
            <a class="info-link" href="https://qvmcdocs.xiaozhuhouses.asia" target="_blank" rel="noopener">
              https://qvmcdocs.xiaozhuhouses.asia
            </a>
          </div>
        </div>
      </el-collapse-item>

      <!-- 面板信息 -->
      <el-collapse-item name="panel">
        <template #title>
          <div class="section-header">
            <el-icon class="section-icon"><Monitor /></el-icon>
            <span class="section-title">面板信息</span>
          </div>
        </template>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">版本</span>
            <span class="info-value">{{ versionInfo.version || '开发版' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">构建时间</span>
            <span class="info-value">{{ versionInfo.build_time || '未设置' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">站点名称</span>
            <span class="info-value">{{ versionInfo.site_title || '未设置' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">运行模式</span>
            <span class="info-value">
              <el-tag size="small" :type="isDev ? 'warning' : 'success'">{{ isDev ? '开发环境' : '生产环境' }}</el-tag>
            </span>
          </div>
        </div>
      </el-collapse-item>

      <!-- 系统信息 -->
      <el-collapse-item name="system">
        <template #title>
          <div class="section-header">
            <el-icon class="section-icon"><Setting /></el-icon>
            <span class="section-title">系统信息</span>
          </div>
        </template>
        <div class="info-grid" v-loading="sysLoading">
          <div class="info-item">
            <span class="info-label">操作系统</span>
            <span class="info-value">{{ sysInfo.distro || sysInfo.os || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">内核版本</span>
            <span class="info-value">{{ sysInfo.kernel || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">系统架构</span>
            <span class="info-value">{{ sysInfo.arch || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">主机名</span>
            <span class="info-value">{{ sysInfo.hostname || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">CPU 核数</span>
            <span class="info-value">{{ sysInfo.num_cpu || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">Go 版本</span>
            <span class="info-value">{{ sysInfo.go_version || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">QEMU 版本</span>
            <span class="info-value">{{ sysInfo.qemu || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">libvirt 版本</span>
            <span class="info-value">{{ sysInfo.libvirt || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">系统运行时间</span>
            <span class="info-value">{{ sysInfo.uptime || '-' }}</span>
          </div>
        </div>
      </el-collapse-item>
    </el-collapse>

    <!-- 页脚 -->
    <div class="about-footer">
      <p>© {{ currentYear }} QVMConsole. 基于 Vue 3 + Element Plus + Go 构建</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getPublicVersion, getPublicSystemInfo } from '@/api/settings'
import { applyDocumentTitle } from '@/utils/site'
import { Cpu, Link, Monitor, Setting } from '@element-plus/icons-vue'

const activeSections = ref(['tech', 'project', 'panel', 'system'])
const isDev = import.meta.env.DEV
const versionInfo = ref({ version: '', build_time: '', site_title: '' })
const sysInfo = ref({})
const sysLoading = ref(false)
const currentYear = new Date().getFullYear()

const techStack = [
  {
    name: 'Vue 3',
    desc: '渐进式 JavaScript 框架',
    url: 'https://vuejs.org',
    logo: 'https://cdn.jsdelivr.net/gh/devicons/devicon/icons/vuejs/vuejs-original.svg'
  },
  {
    name: 'Element Plus',
    desc: 'Vue 3 UI 组件库',
    url: 'https://element-plus.org',
    logo: 'https://element-plus.org/images/element-plus-logo-small.svg'
  },
  {
    name: 'Vite',
    desc: '下一代前端构建工具',
    url: 'https://vitejs.dev',
    logo: 'https://cdn.jsdelivr.net/gh/devicons/devicon/icons/vitejs/vitejs-original.svg'
  },
  {
    name: 'Pinia',
    desc: 'Vue 状态管理库',
    url: 'https://pinia.vuejs.org',
    logo: 'https://pinia.vuejs.org/logo.svg'
  },
  {
    name: 'Go',
    desc: '高性能后端语言',
    url: 'https://go.dev',
    logo: 'https://cdn.jsdelivr.net/gh/devicons/devicon/icons/go/go-original.svg'
  },
  {
    name: 'Gin',
    desc: 'Go HTTP Web 框架',
    url: 'https://gin-gonic.com',
    logo: 'https://cdn.jsdelivr.net/gh/devicons/devicon/icons/go/go-original.svg'
  },
  {
    name: 'SQLite',
    desc: '轻量级嵌入式数据库',
    url: 'https://www.sqlite.org',
    logo: 'https://cdn.jsdelivr.net/gh/devicons/devicon/icons/sqlite/sqlite-original.svg'
  },
  {
    name: 'libvirt',
    desc: '虚拟化管理 API',
    url: 'https://libvirt.org',
    logo: 'https://libvirt.org/logos/logo-circle.svg'
  },
  {
    name: 'QEMU/KVM',
    desc: '硬件虚拟化方案',
    url: 'https://www.qemu.org',
    logo: 'https://www.qemu.org/logos/qemu-logo.svg'
  },
  {
    name: 'noVNC',
    desc: 'Web 远程桌面客户端',
    url: 'https://novnc.com',
    logo: 'https://novnc.com/img/novnc-logo.svg'
  }
]

const fetchVersion = async () => {
  try {
    const res = await getPublicVersion()
    versionInfo.value = {
      version: res.data?.version || '',
      build_time: res.data?.build_time || '',
      site_title: res.data?.site_title || ''
    }
  } catch {
    versionInfo.value = { version: 'dev', build_time: '', site_title: '' }
  }
}

const fetchSystemInfo = async () => {
  sysLoading.value = true
  try {
    const res = await getPublicSystemInfo()
    sysInfo.value = res.data || {}
  } catch {
    sysInfo.value = {}
  } finally {
    sysLoading.value = false
  }
}

onMounted(() => {
  applyDocumentTitle('关于项目')
  fetchVersion()
  fetchSystemInfo()
})
</script>

<style scoped>
.about-container {
  max-width: 960px;
  margin: 0 auto;
  padding: 20px;
  min-height: calc(100vh - 120px);
  display: flex;
  flex-direction: column;
}

.about-collapse {
  border: none;
}

.about-collapse :deep(.el-collapse-item) {
  margin-bottom: 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  overflow: hidden;
  background: var(--el-bg-color);
}

.about-collapse :deep(.el-collapse-item__header) {
  background: var(--el-fill-color-lighter);
  border-bottom: 1px solid var(--el-border-color-extra-light);
  padding: 0 20px;
  height: 48px;
  line-height: 48px;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.about-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
}

.about-collapse :deep(.el-collapse-item__content) {
  padding: 20px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.section-icon {
  font-size: 18px;
  color: var(--el-color-primary);
}

.section-title {
  font-weight: 600;
}

/* 技术栈 */
.tech-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.tech-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  background: var(--el-fill-color-lighter);
  border-radius: 10px;
  text-decoration: none;
  color: inherit;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.tech-item:hover {
  background: var(--el-fill-color-light);
  border-color: var(--el-color-primary-light-7);
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.tech-logo {
  width: 36px;
  height: 36px;
  object-fit: contain;
  flex-shrink: 0;
  border-radius: 6px;
}

.tech-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.tech-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.tech-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 信息网格 */
.info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  background: var(--el-fill-color-lighter);
  border-radius: 8px;
  gap: 12px;
}

.info-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
}

.info-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary);
  text-align: right;
  word-break: break-all;
}

.info-link {
  font-size: 13px;
  color: var(--el-color-primary);
  text-decoration: none;
  word-break: break-all;
  text-align: right;
}

.info-link:hover {
  text-decoration: underline;
}

.info-link-inline {
  color: var(--el-color-primary);
  text-decoration: none;
  margin-left: 4px;
}

.info-link-inline:hover {
  text-decoration: underline;
}

.text-muted {
  color: var(--el-text-color-secondary);
}

/* 页脚 */
.about-footer {
  text-align: center;
  padding: 24px 0;
  border-top: 1px solid var(--el-border-color-lighter);
  margin-top: auto;
}

.about-footer p {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* 响应式 */
@media (max-width: 768px) {
  .tech-grid {
    grid-template-columns: 1fr;
  }

  .info-grid {
    grid-template-columns: 1fr;
  }

  .info-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }

  .info-value,
  .info-link {
    text-align: left;
  }
}
</style>
