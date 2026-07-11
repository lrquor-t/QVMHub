// 菜单目录：每个菜单项元信息的唯一事实来源（代码）。仅当真实功能页增删时改动。
// 数据库只存结构(menu_layout)，标题/图标/路由不进数据库，避免与真实路由漂移。
export const menuCatalog = [
  { key: 'home',         title: '首页',       icon: 'home',        route: '/dashboard',       adminOnly: false, lightweightHidden: true,  protected: false, defaultGroup: null },
  { key: 'vm-list',      title: '虚拟机列表', icon: 'vm',          route: '/vm/list',         adminOnly: false, lightweightHidden: false, protected: true,  defaultGroup: 'host' },
  { key: 'nodes',        title: '节点管理',   icon: 'node',        route: '/nodes',           adminOnly: true,  lightweightHidden: true,  protected: false, defaultGroup: 'host' },
  { key: 'template',     title: 'KVM模板',    icon: 'template',    route: '/template/list',   adminOnly: true,  lightweightHidden: true,  protected: false, defaultGroup: 'template' },
  { key: 'lxc-list',     title: 'LXC容器',    icon: 'lxc',         route: '/lxc/list',        adminOnly: false, lightweightHidden: true,  protected: false, defaultGroup: 'host' },
  { key: 'lxc-template', title: 'LXC模板',    icon: 'lxc',         route: '/lxc/template',    adminOnly: true,  lightweightHidden: true,  protected: false, defaultGroup: 'template' },
  { key: 'network',      title: '网络',       icon: 'network',     route: '/network',         adminOnly: false, lightweightHidden: true,  protected: false, defaultGroup: 'network', alt: { title: 'VPC网络', icon: 'vpc' } },
  { key: 'public-ip',    title: '公网 IP',    icon: 'globe',       route: '/public-ip',       adminOnly: true,  lightweightHidden: true,  protected: false, defaultGroup: 'network' },
  { key: 'firewall',     title: '防火墙',     icon: 'firewall',    route: '/firewall',        adminOnly: true,  lightweightHidden: true,  protected: false, defaultGroup: 'network' },
  { key: 'storage-pool', title: '存储池',     icon: 'storage-pool',route: '/storage-pool/list',adminOnly: true,  lightweightHidden: true,  protected: false, defaultGroup: 'storage' },
  { key: 'my-storage',   title: '我的存储',   icon: 'folder',      route: '/my-storage',      adminOnly: false, lightweightHidden: true,  protected: false, defaultGroup: 'storage' },
  { key: 'user-list',    title: '用户管理',   icon: 'user',        route: '/user/list',       adminOnly: true,  lightweightHidden: true,  protected: false, defaultGroup: 'system' },
  { key: 'scheduler',    title: '调度事件',   icon: 'scheduler',   route: '/scheduler/events',adminOnly: true,  lightweightHidden: true,  protected: false, defaultGroup: 'system' },
  { key: 'settings',     title: '系统设置',   icon: 'setting',     route: '/settings',        adminOnly: true,  lightweightHidden: true,  protected: true,  defaultGroup: 'system' },
  { key: 'about',        title: '关于项目',   icon: 'about',       route: '/about',           adminOnly: false, lightweightHidden: false, protected: true,  defaultGroup: null }
]

const ROLE_KEYS = ['admin', 'elastic', 'lightweight']

// 默认菜单（按角色各一份）—— 首次加载/回退时使用，镜像改造前各角色实际可见的菜单。
export const defaultMenuLayouts = {
  admin: { version: 3, nodes: [
    { kind: 'item', key: 'home', enabled: true },
    { kind: 'group', id: 'host', title: '主机管理', icon: 'host', enabled: true, children: [
      { kind: 'item', key: 'vm-list', enabled: true },
      { kind: 'item', key: 'nodes', enabled: true },
      { kind: 'item', key: 'lxc-list', enabled: true }
    ]},
    { kind: 'group', id: 'template', title: '模板管理', icon: 'template-group', enabled: true, children: [
      { kind: 'item', key: 'template', enabled: true },
      { kind: 'item', key: 'lxc-template', enabled: true }
    ]},
    { kind: 'group', id: 'network', title: '网络管理', icon: 'network', enabled: true, children: [
      { kind: 'item', key: 'network', enabled: true },
      { kind: 'item', key: 'public-ip', enabled: true },
      { kind: 'item', key: 'firewall', enabled: true }
    ]},
    { kind: 'group', id: 'storage', title: '存储管理', icon: 'storage', enabled: true, children: [
      { kind: 'item', key: 'storage-pool', enabled: true },
      { kind: 'item', key: 'my-storage', enabled: true }
    ]},
    { kind: 'group', id: 'system', title: '系统管理', icon: 'system', enabled: true, children: [
      { kind: 'item', key: 'user-list', enabled: true },
      { kind: 'item', key: 'scheduler', enabled: true },
      { kind: 'item', key: 'settings', enabled: true }
    ]},
    { kind: 'item', key: 'about', enabled: true }
  ]},
  elastic: { version: 1, nodes: [
    { kind: 'item', key: 'home', enabled: true },
    { kind: 'item', key: 'vm-list', enabled: true },
    { kind: 'item', key: 'network', enabled: true },
    { kind: 'item', key: 'my-storage', enabled: true },
    { kind: 'item', key: 'about', enabled: true }
  ]},
  lightweight: { version: 1, nodes: [
    { kind: 'item', key: 'vm-list', enabled: true },
    { kind: 'item', key: 'about', enabled: true }
  ]}
}

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

// 某角色是否应看到该项（与改造前的角色过滤一致）
export function validForRole(item, role) {
  if (role === 'admin') return true
  if (role === 'lightweight') return !item.lightweightHidden
  return !item.adminOnly // 弹性云非管理员
}

// 某角色可用的 catalog 项（编辑器的可添加项来源）
export function catalogForRole(role) {
  return menuCatalog.filter((i) => validForRole(i, role))
}

// 某角色的受保护项 key（不可隐藏/删除；settings 仅对管理员受保护——非管理员本就看不到）
export function protectedKeysForRole(role) {
  return menuCatalog.filter((i) => i.protected && validForRole(i, role)).map((i) => i.key)
}

// 解析存储的 map：{admin,elastic,lightweight}，每份各自回退默认；兼容旧的单树格式({version,nodes})。
function parseLayoutMap(raw) {
  const out = {
    admin: clone(defaultMenuLayouts.admin),
    elastic: clone(defaultMenuLayouts.elastic),
    lightweight: clone(defaultMenuLayouts.lightweight)
  }
  if (typeof raw !== 'string' || raw.trim() === '') return out
  let obj
  try { obj = JSON.parse(raw) } catch { return out }
  if (!obj || typeof obj !== 'object') return out
  // 兼容旧单树格式：{version,nodes} 视为 admin 一份
  if (Array.isArray(obj.nodes)) { out.admin = { version: obj.version || 1, nodes: obj.nodes }; return out }
  for (const r of ROLE_KEYS) {
    const v = obj[r]
    if (v && Array.isArray(v.nodes)) out[r] = { version: v.version || 1, nodes: v.nodes }
  }
  return out
}

function collectItemKeys(nodes, set) {
  for (const n of nodes) {
    if (n.kind === 'item' && n.key) set.add(n.key)
    else if (n.kind === 'group' && Array.isArray(n.children)) collectItemKeys(n.children, set)
  }
  return set
}

function findGroupById(nodes, id) {
  for (const n of nodes) {
    if (n.kind === 'group' && n.id === id) return n
  }
  return null
}

// 合并：按当前用户角色取对应那一份菜单树，做受保护强制 + 角色安全过滤 + 空分组隐藏。
// 与"全局单树"模型的区别：不再自动注入全部缺失项——每角色的菜单由管理员在编辑器里独立裁剪，
// 这里只保证受保护项可见、陈旧/越权项被裁掉。layout/index.vue 按真实访客角色自动取对应那一份。
export function composeMenu(layoutMapRaw, ctx = {}) {
  const isAdmin = !!ctx.isAdmin
  const isLightweight = !isAdmin && !!ctx.isLightweight
  const role = isAdmin ? 'admin' : isLightweight ? 'lightweight' : 'elastic'

  const byKey = Object.create(null)
  for (const item of menuCatalog) byKey[item.key] = item

  const map = parseLayoutMap(layoutMapRaw)
  const working = clone(map[role].nodes)

  // 版本合并：存储版本低于默认版本时，把默认有但存储缺失的项注入其 defaultGroup（仅渲染层；不回写）。
  const storedVersion = map[role].version || 1
  const defaultVersion = defaultMenuLayouts[role].version || 1
  if (storedVersion < defaultVersion) {
    const referencedNow = collectItemKeys(working, new Set())
    const defaults = clone(defaultMenuLayouts[role].nodes)
    for (const dnode of defaults) {
      if (dnode.kind === 'item' && dnode.key && !referencedNow.has(dnode.key)) {
        const item = menuCatalog.find((i) => i.key === dnode.key)
        if (!item || !validForRole(item, role)) continue
        const group = item.defaultGroup ? findGroupById(working, item.defaultGroup) : null
        if (group && Array.isArray(group.children)) group.children.push({ kind: 'item', key: dnode.key, enabled: true })
        else working.push({ kind: 'item', key: dnode.key, enabled: true })
      }
    }
    // 同步默认分组的外观（图标/空标题）到已存储的同 id 分组，让升级后的新组图标对老安装生效（仅渲染层；管理员保存后版本戳齐即停止）。
    for (const dnode of defaults) {
      if (dnode.kind === 'group' && dnode.id) {
        const g = findGroupById(working, dnode.id)
        if (g) {
          if (dnode.icon) g.icon = dnode.icon
          if (!g.title || String(g.title).trim() === '') g.title = dnode.title
        }
      }
    }
  }

  // 受保护项（对本角色有效者）若缺失则补回其 defaultGroup，防止管理员把自己锁死
  const referenced = collectItemKeys(working, new Set())
  for (const item of menuCatalog) {
    if (item.protected && validForRole(item, role) && !referenced.has(item.key)) {
      const node = { kind: 'item', key: item.key, enabled: true }
      const group = item.defaultGroup ? findGroupById(working, item.defaultGroup) : null
      if (group && Array.isArray(group.children)) group.children.push(node)
      else working.push(node)
    }
  }

  // 受保护项救援：被关闭的分组里的受保护子项提升到顶层（与写入来源无关的渲染层保障）
  const topSnapshot = working.slice()
  for (const n of topSnapshot) {
    if (n.kind === 'group' && n.enabled === false && Array.isArray(n.children)) {
      n.children = n.children.filter((c) => {
        const item = c.kind === 'item' ? byKey[c.key] : null
        if (item && item.protected && validForRole(item, role)) {
          working.push({ kind: 'item', key: c.key, enabled: true })
          return false
        }
        return true
      })
    }
  }

  const resolveMeta = (item) => {
    const useAlt = role === 'elastic' && item.alt
    return {
      key: item.key,
      title: useAlt ? item.alt.title : item.title,
      icon: useAlt ? item.alt.icon : item.icon,
      route: item.route
    }
  }

  const composeNodes = (nodes) => {
    const out = []
    for (const node of nodes) {
      if (node.kind === 'item') {
        const item = byKey[node.key]
        if (!item) continue // 裁剪陈旧项
        if (!validForRole(item, role)) continue // 角色安全过滤（防止越权项泄漏）
        const enabled = item.protected ? true : node.enabled !== false // 受保护强制可见
        if (!enabled) continue
        out.push({ type: 'item', ...resolveMeta(item) })
      } else if (node.kind === 'group') {
        if (node.enabled === false) continue // 整组关闭则隐藏
        const children = composeNodes(node.children || [])
        if (children.length === 0) continue // 空分组隐藏
        out.push({
          type: 'group',
          id: node.id,
          title: (node.title && String(node.title).trim()) || '未命名分组',
          icon: node.icon || 'folder',
          children
        })
      }
    }
    return out
  }

  return composeNodes(working)
}
