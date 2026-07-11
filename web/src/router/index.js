import { createRouter, createWebHistory } from 'vue-router'
import Layout from '@/layout/index.vue'
import NProgress from 'nprogress'
import { applyDocumentTitle } from '@/utils/site'

// 配置 NProgress（可选）：关闭进度环
NProgress.configure({ showSpinner: false })

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { title: '登录' }
  },
  {
    path: '/invite',
    name: 'InviteRegister',
    component: () => import('@/views/invite/index.vue'),
    meta: { title: '邀请注册' }
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: () => import('@/views/reset-password/index.vue'),
    meta: { title: '重置密码' }
  },
  {
    path: '/vm/:id/vnc-window',
    name: 'VncWindow',
    component: () => import('@/views/vm/vnc-window.vue'),
    meta: { title: 'VNC 控制台' }
  },
  {
    path: '/lxc/console/:name',
    name: 'LxcConsole',
    component: () => import('@/views/lxc/console.vue'),
    meta: { title: 'LXC 控制台' }
  },
  {
    path: '/',
    component: Layout,
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { title: '首页', icon: 'HomeFilled' }
      },
      {
        path: 'vm/list',
        name: 'VmList',
        component: () => import('@/views/vm/list.vue'),
        meta: { title: '虚拟机列表', icon: 'Monitor' }
      },
      {
        path: 'vm/detail/:id',
        name: 'VmDetail',
        component: () => import('@/views/vm/detail.vue'),
        meta: { title: '虚拟机详情', hidden: true }
      },
      {
        path: 'lxc/list',
        name: 'LxcList',
        component: () => import('@/views/lxc/list.vue'),
        meta: { title: 'LXC 容器', icon: 'Monitor' }
      },
      {
        path: 'lxc/template',
        name: 'LxcTemplate',
        component: () => import('@/views/lxc/template.vue'),
        meta: { title: 'LXC 模板', icon: 'Files', adminOnly: true }
      },
      {
        path: 'template/list',
        name: 'TemplateList',
        component: () => import('@/views/template/index.vue'),
        meta: { title: '模板管理', icon: 'Files' }
      },
      {
        path: 'network',
        name: 'NetworkCenter',
        component: () => import('@/views/network/index.vue'),
        meta: { title: '网络', icon: 'Connection' }
      },
      {
        path: 'public-ip',
        name: 'PublicIP',
        component: () => import('@/views/public-ip/index.vue'),
        meta: { title: '公网 IP', icon: 'Connection', adminOnly: true }
      },
      {
        path: 'ovs',
        redirect: '/network'
      },
      {
        path: 'firewall',
        name: 'NetworkFirewall',
        component: () => import('@/views/firewall/index.vue'),
        meta: { title: '防火墙', icon: 'Lock', adminOnly: true }
      },
      {
        path: 'storage-pool/list',
        name: 'StoragePoolList',
        component: () => import('@/views/storage-pool/index.vue'),
        meta: { title: '存储池', icon: 'Box' }
      },
      {
        path: 'nodes',
        name: 'NodeManage',
        component: () => import('@/views/node/index.vue'),
        meta: { title: '节点管理', icon: 'Connection', adminOnly: true }
      },
      {
        path: 'my-storage',
        name: 'MyStorage',
        component: () => import('@/views/storage/index.vue'),
        meta: { title: '我的存储', icon: 'FolderOpened' }
      },
      {
        path: 'user/list',
        name: 'UserList',
        component: () => import('@/views/user/index.vue'),
        meta: { title: '用户管理', icon: 'User', adminOnly: true }
      },
      {
        path: 'scheduler/events',
        name: 'SchedulerEvents',
        component: () => import('@/views/scheduler/index.vue'),
        meta: { title: '调度事件', icon: 'DataAnalysis', adminOnly: true }
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/settings/index.vue'),
        meta: { title: '系统设置', icon: 'Setting', adminOnly: true }
      },
      {
        path: 'api-docs',
        name: 'ApiDocs',
        component: () => import('@/views/api-docs/index.vue'),
        meta: { title: '接口文档', hidden: true }
      },
      {
        path: 'task/recent',
        name: 'TaskRecent',
        component: () => import('@/views/task/index.vue'),
        meta: { title: '任务中心' }
      },
      {
        path: 'about',
        name: 'About',
        component: () => import('@/views/about/index.vue'),
        meta: { title: '关于项目' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 简单前置路由守卫
router.beforeEach((to) => {
  NProgress.start()
  const token = localStorage.getItem('token')
  const publicPaths = ['/login', '/invite', '/reset-password']
  if (!publicPaths.includes(to.path) && !token) {
    return '/login'
  }
  const cloudType = localStorage.getItem('cloud_type') || 'elastic'
  const role = localStorage.getItem('role') || ''
  if (token && role !== 'admin' && cloudType === 'lightweight') {
    const allowed = to.path === '/vm/list' ||
      to.path.startsWith('/vm/detail/') ||
      to.path.endsWith('/vnc-window') ||
      to.path === '/api-docs' ||
      to.path === '/about' ||
      publicPaths.includes(to.path)
    if (to.path === '/' || to.path === '/dashboard') {
      return '/vm/list'
    }
    if (!allowed) {
      return '/vm/list'
    }
  }
})

router.afterEach((to) => {
  applyDocumentTitle(to.meta?.title || '')
  NProgress.done()
})

export default router
