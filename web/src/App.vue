<template>
  <el-config-provider :locale="locale">
    <router-view />
  </el-config-provider>
</template>

<script setup>
import { computed, onMounted, watchEffect } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import { applyDocumentTitle, syncPublicSiteTitle } from '@/utils/site'

const locale = computed(() => zhCn)
const route = useRoute()

const VERSION_STORAGE_KEY = 'app_version'

async function checkVersion() {
  try {
    const res = await axios.get('/api/public/version', { timeout: 5000 })
    const serverVersion = res.data?.data?.version || ''
    if (!serverVersion) return

    const cachedVersion = localStorage.getItem(VERSION_STORAGE_KEY)

    if (cachedVersion && cachedVersion !== serverVersion) {
      // 版本号变化，更新缓存并强制刷新页面（跳过浏览器缓存）
      localStorage.setItem(VERSION_STORAGE_KEY, serverVersion)
      window.location.reload()
      return
    }

    if (!cachedVersion) {
      localStorage.setItem(VERSION_STORAGE_KEY, serverVersion)
    }
  } catch {
    // 版本检测失败静默忽略，不影响正常使用
  }
}

watchEffect(() => {
  applyDocumentTitle(route.meta?.title || '')
})

onMounted(async () => {
  await syncPublicSiteTitle()
  await checkVersion()
})
</script>

<style>
/* 全局基础样式 */
html, body, #app {
  margin: 0;
  padding: 0;
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', Helvetica, 'PingFang SC',
    'Hiragino Sans GB', 'Microsoft YaHei', '微软雅黑', Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  background-color: var(--app-bg-page, var(--el-bg-color-page));
  color: var(--el-text-color-primary);
  font-feature-settings: 'tnum' on, 'lnum' on;
}

* {
  box-sizing: border-box;
}

a {
  text-decoration: none;
  transition: color 0.15s ease;
}

/* 平滑滚动 */
html {
  scroll-behavior: smooth;
}

/* Focus 样式统一 */
:focus-visible {
  outline: 2px solid var(--el-color-primary-light-3);
  outline-offset: 2px;
}

:focus:not(:focus-visible) {
  outline: none;
}
</style>
