<template>
  <div class="menu-editor">
    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px;">
      <template #title>菜单编辑（按角色各一份，全局生效）</template>
      <div class="me-tip">
        先用左侧「编辑角色」选择要编辑的菜单；左侧只显示该角色<strong>能看到</strong>的菜单项并可改，
        右侧实时预览该角色效果。三套菜单互不影响。编辑仅管理员可操作。
        开关控制显示；「所属」选「顶层」即<strong>主菜单</strong>，选某个分组即<strong>子菜单</strong>。
        受保护项不可隐藏/删除；删除分组会将其子项移到顶层。
      </div>
    </el-alert>

    <div class="me-toolbar">
      <span class="me-role-label">编辑角色：</span>
      <el-select :model-value="editRole" class="me-role-sel" :teleported="false" @change="(v) => switchRole(v)">
        <el-option v-for="r in ROLE_OPTIONS" :key="r.value" :label="r.label" :value="r.value" />
      </el-select>
      <el-button :icon="Plus" @click="addGroup">新建分组</el-button>
      <el-button @click="resetDefault">恢复该角色默认</el-button>
      <el-button type="primary" :loading="saving" :icon="Check" @click="save">保存</el-button>
    </div>

    <el-row :gutter="16">
      <!-- 编辑区（当前角色） -->
      <el-col :xs="24" :md="14">
        <div class="me-rows">
          <template v-for="(row, idx) in flatRows" :key="(row.rowKind === 'group' ? 'g:' : 'i:') + (row.rowKind === 'group' ? row.node.id : row.node.key) + ':' + idx">
            <!-- 分组行 -->
            <div v-if="row.rowKind === 'group'" class="me-row me-row-group">
              <div class="me-row-main">
                <el-input v-model="row.node.title" class="me-group-title" placeholder="分组名称" />
                <el-select v-model="row.node.icon" class="me-icon-sel" :teleported="false">
                  <el-option v-for="ic in ICON_OPTIONS" :key="ic" :label="ic" :value="ic">
                    <SidebarIcons :icon="ic" :size="16" />
                    <span style="margin-left:6px;">{{ ic }}</span>
                  </el-option>
                </el-select>
                <el-tooltip :content="groupContainsProtected(row.node) ? '该分组含受保护项，不可关闭' : '显示/隐藏整组'" placement="top">
                  <span>
                    <el-switch v-model="row.node.enabled" :disabled="groupContainsProtected(row.node)" />
                  </span>
                </el-tooltip>
              </div>
              <div class="me-row-ops">
                <el-button text :icon="ArrowUp" :disabled="idx === 0" @click="reorderNode(row, -1)" />
                <el-button text :icon="ArrowDown" :disabled="isLastRow(idx)" @click="reorderNode(row, 1)" />
                <el-button text type="danger" :icon="Delete" @click="removeGroup(row.node)" />
              </div>
            </div>

            <!-- 菜单项行 -->
            <div v-else class="me-row me-row-item" :class="{ 'is-child': !!row.parent }">
              <div class="me-row-main">
                <SidebarIcons :icon="itemMeta(row.node.key)?.icon || 'folder'" :size="16" />
                <span class="me-item-title">{{ itemMeta(row.node.key)?.title || row.node.key }}</span>
                <el-tag v-if="isProtected(row.node.key)" size="small" type="warning" effect="plain">受保护</el-tag>
                <el-select
                  :model-value="row.parent ? row.parent.id : '__top__'"
                  class="me-parent-sel"
                  :teleported="false"
                  @change="(v) => moveItem(row.node.key, v)"
                >
                  <el-option label="顶层（主菜单）" value="__top__" />
                  <el-option v-for="g in topLevelGroups" :key="g.id" :label="g.title || g.id" :value="g.id" />
                </el-select>
                <el-tooltip :content="isProtected(row.node.key) ? '受保护，不可隐藏' : '显示/隐藏'" placement="top">
                  <span>
                    <el-switch v-model="row.node.enabled" :disabled="isProtected(row.node.key)" />
                  </span>
                </el-tooltip>
              </div>
              <div class="me-row-ops">
                <el-button text :icon="ArrowUp" :disabled="!canMoveItem(row, -1)" @click="moveItemUpDown(row, -1)" />
                <el-button text :icon="ArrowDown" :disabled="!canMoveItem(row, 1)" @click="moveItemUpDown(row, 1)" />
                <el-button v-if="!isProtected(row.node.key)" text type="danger" :icon="Delete" @click="removeItem(row.node.key)" />
              </div>
            </div>
          </template>
        </div>

        <!-- 该角色可添加的菜单项 -->
        <div v-if="addableItems.length" class="me-addable">
          <div class="me-addable-title">可添加到「{{ currentRoleLabel }}」菜单：</div>
          <el-button v-for="key in addableItems" :key="key" size="small" @click="addItem(key)">
            <SidebarIcons :icon="itemMeta(key)?.icon || 'folder'" :size="14" />
            <span style="margin-left:4px;">{{ itemMeta(key)?.title || key }}</span>
          </el-button>
        </div>
      </el-col>

      <!-- 实时预览（当前编辑角色） -->
      <el-col :xs="24" :md="10">
        <el-card class="me-preview" shadow="never">
          <template #header><span>实时预览（{{ currentRoleLabel }}）</span></template>
          <el-menu class="me-preview-menu">
            <template v-for="node in previewNodes" :key="node.type === 'group' ? ('g:' + node.id) : ('i:' + node.route)">
              <el-menu-item v-if="node.type === 'item'" :index="node.route" disabled>
                <SidebarIcons :icon="node.icon" />
                <span>{{ node.title }}</span>
              </el-menu-item>
              <el-sub-menu v-else :index="'g:' + node.id">
                <template #title>
                  <SidebarIcons :icon="node.icon" />
                  <span>{{ node.title }}</span>
                </template>
                <el-menu-item v-for="child in node.children" :key="child.route" :index="child.route" disabled>
                  <SidebarIcons :icon="child.icon" />
                  <span>{{ child.title }}</span>
                </el-menu-item>
              </el-sub-menu>
            </template>
          </el-menu>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Check, ArrowUp, ArrowDown, Delete } from '@element-plus/icons-vue'
import { updateSettings } from '@/api/settings'
import SidebarIcons from '@/components/icons/SidebarIcons.vue'
import { menuCatalog, defaultMenuLayouts, composeMenu, catalogForRole, protectedKeysForRole } from '@/utils/menu'
import { menuLayoutRaw, setMenuLayoutRaw } from '@/utils/site'

const ICON_OPTIONS = ['home', 'vm', 'template', 'network', 'globe', 'vpc', 'firewall', 'storage-pool', 'node', 'folder', 'user', 'scheduler', 'setting', 'storage', 'system', 'host', 'about']
const ROLE_OPTIONS = [
  { value: 'admin', label: '管理员' },
  { value: 'elastic', label: '弹性云用户' },
  { value: 'lightweight', label: '轻量云用户' }
]
const ROLE_LABEL = { admin: '管理员', elastic: '弹性云用户', lightweight: '轻量云用户' }

const byKey = Object.create(null)
menuCatalog.forEach((i) => { byKey[i.key] = i })
const itemMeta = (key) => byKey[key]

const clone = (x) => JSON.parse(JSON.stringify(x))

// 三套菜单树（{version,nodes}）；working 为当前编辑角色的 nodes（独立副本）
const layouts = ref({
  admin: clone(defaultMenuLayouts.admin),
  elastic: clone(defaultMenuLayouts.elastic),
  lightweight: clone(defaultMenuLayouts.lightweight)
})
const editRole = ref('admin')
const working = ref(clone(layouts.value.admin.nodes))
const saving = ref(false)

function loadFromRaw(raw) {
  let obj = null
  try { obj = typeof raw === 'string' && raw.trim() ? JSON.parse(raw) : null } catch { obj = null }
  if (obj && typeof obj === 'object') {
    if (Array.isArray(obj.nodes)) {
      // 兼容旧单树格式
      layouts.value = {
        admin: { version: 1, nodes: obj.nodes },
        elastic: clone(defaultMenuLayouts.elastic),
        lightweight: clone(defaultMenuLayouts.lightweight)
      }
    } else {
      layouts.value = {
        admin: obj.admin && Array.isArray(obj.admin.nodes) ? { version: 1, nodes: obj.admin.nodes } : clone(defaultMenuLayouts.admin),
        elastic: obj.elastic && Array.isArray(obj.elastic.nodes) ? { version: 1, nodes: obj.elastic.nodes } : clone(defaultMenuLayouts.elastic),
        lightweight: obj.lightweight && Array.isArray(obj.lightweight.nodes) ? { version: 1, nodes: obj.lightweight.nodes } : clone(defaultMenuLayouts.lightweight)
      }
    }
  } else {
    layouts.value = {
      admin: clone(defaultMenuLayouts.admin),
      elastic: clone(defaultMenuLayouts.elastic),
      lightweight: clone(defaultMenuLayouts.lightweight)
    }
  }
  editRole.value = 'admin'
  working.value = clone(layouts.value.admin.nodes)
}
onMounted(() => loadFromRaw(menuLayoutRaw.value))

const currentRoleLabel = computed(() => ROLE_LABEL[editRole.value] || editRole.value)
const protectedKeys = computed(() => protectedKeysForRole(editRole.value))
const isProtected = (key) => protectedKeys.value.includes(key)

function ctxForRole(r) {
  if (r === 'admin') return { isAdmin: true, isLightweight: false }
  if (r === 'lightweight') return { isAdmin: false, isLightweight: true }
  return { isAdmin: false, isLightweight: false }
}

// 切换编辑角色：先把当前 working 写回 layouts，再载入新角色的树
function switchRole(r) {
  if (r === editRole.value) return
  layouts.value[editRole.value].nodes = clone(working.value)
  editRole.value = r
  working.value = clone(layouts.value[r].nodes)
}

// ---- 只读派生（基于当前 working） ----
const referencedKeys = computed(() => {
  const set = new Set()
  const walk = (arr) => arr.forEach((n) => {
    if (n.kind === 'item' && n.key) set.add(n.key)
    else if (n.kind === 'group') walk(n.children || [])
  })
  walk(working.value)
  return set
})
const addableItems = computed(() => catalogForRole(editRole.value).map((i) => i.key).filter((k) => !referencedKeys.value.has(k)))
const topLevelGroups = computed(() => working.value.filter((n) => n.kind === 'group'))

const flatRows = computed(() => {
  const rows = []
  working.value.forEach((node, topIndex) => {
    if (node.kind === 'group') {
      rows.push({ rowKind: 'group', node, topIndex })
      ;(node.children || []).forEach((c) => {
        rows.push({ rowKind: 'item', node: c, parent: node, topIndex })
      })
    } else {
      rows.push({ rowKind: 'item', node, parent: null, topIndex })
    }
  })
  return rows
})
const isLastRow = (idx) => idx >= flatRows.value.length - 1

// 实时预览：用当前 working 覆盖对应角色，按该角色 ctx 渲染
const previewNodes = computed(() => {
  const liveMap = { ...layouts.value, [editRole.value]: { version: 1, nodes: working.value } }
  return composeMenu(JSON.stringify(liveMap), ctxForRole(editRole.value))
})

// ---- 分组操作 ----
function genGroupId() {
  return 'g_' + Math.random().toString(36).slice(2, 8)
}
function addGroup() {
  working.value.push({ kind: 'group', id: genGroupId(), title: '新分组', icon: 'folder', enabled: true, children: [] })
}
function groupContainsProtected(group) {
  return (group.children || []).some((c) => c.kind === 'item' && isProtected(c.key))
}
function removeGroup(group) {
  const idx = working.value.indexOf(group)
  if (idx < 0) return
  const replacement = []
  for (const c of group.children || []) replacement.push(c.kind === 'item' ? { ...c } : c)
  working.value.splice(idx, 1, ...replacement)
}

// ---- 通用树操作 ----
function removeItemByKey(nodes, key) {
  for (let i = 0; i < nodes.length; i++) {
    const n = nodes[i]
    if (n.kind === 'item' && n.key === key) return nodes.splice(i, 1)[0]
    if (n.kind === 'group') {
      const r = removeItemByKey(n.children || [], key)
      if (r) return r
    }
  }
  return null
}
function removeItem(key) {
  if (isProtected(key)) return // 受保护项不可删除
  removeItemByKey(working.value, key)
}
function findGroup(id) {
  return topLevelGroups.value.find((g) => g.id === id) || null
}
function moveItem(key, target) {
  const node = removeItemByKey(working.value, key)
  if (!node) return
  if (target === '__top__' || target == null) working.value.push(node)
  else {
    const g = findGroup(target)
    if (g) g.children.push(node)
    else working.value.push(node)
  }
}
function addItem(key) {
  if (referencedKeys.value.has(key)) return
  working.value.push({ kind: 'item', key, enabled: true })
}

// ---- 排序 ----
function reorderNode(row, delta) {
  const arr = working.value
  const i = row.topIndex
  const j = i + delta
  if (j < 0 || j >= arr.length) return
  const me = arr[i]
  arr.splice(i, 1)
  arr.splice(j, 0, me)
}
function itemSiblingAndIndex(row) {
  if (row.parent) {
    const arr = row.parent.children
    return { arr, index: arr.indexOf(row.node) }
  }
  return { arr: working.value, index: working.value.indexOf(row.node) }
}
function canMoveItem(row, delta) {
  const { arr, index } = itemSiblingAndIndex(row)
  return index + delta >= 0 && index + delta < arr.length
}
function moveItemUpDown(row, delta) {
  const { arr, index } = itemSiblingAndIndex(row)
  const j = index + delta
  if (j < 0 || j >= arr.length) return
  const [me] = arr.splice(index, 1)
  arr.splice(j, 0, me)
}

// ---- 校验与保存 ----
function validate() {
  const ids = new Set()
  for (const g of topLevelGroups.value) {
    if (!(g.title && String(g.title).trim())) return '存在未命名的分组'
    if (ids.has(g.id)) return '存在重复的分组标识'
    ids.add(g.id)
    if (g.enabled === false && groupContainsProtected(g)) return '受保护项所在的分组不能被关闭'
  }
  for (const key of protectedKeys.value) {
    if (!referencedKeys.value.has(key)) return `受保护项 ${itemMeta(key)?.title || key} 缺失`
  }
  return null
}
function resetDefault() {
  working.value = clone(defaultMenuLayouts[editRole.value].nodes)
  ElMessage.info(`已恢复「${currentRoleLabel.value}」默认菜单（需点击保存生效）`)
}
async function save() {
  // 先把当前 working 写回 layouts，再整体序列化
  layouts.value[editRole.value].nodes = clone(working.value)
  layouts.value[editRole.value].version = defaultMenuLayouts[editRole.value]?.version || layouts.value[editRole.value].version || 1
  const err = validate()
  if (err) { ElMessage.warning(err); return }
  const raw = JSON.stringify(layouts.value)
  saving.value = true
  try {
    await updateSettings({ menu_layout: raw })
    setMenuLayoutRaw(raw) // 即时刷新侧边栏
    ElMessage.success('菜单配置已保存')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.menu-editor { padding: 4px 2px; }
.me-tip { font-size: 12px; color: var(--el-text-color-secondary); line-height: 1.6; margin-top: 2px; }
.me-toolbar { margin-bottom: 12px; display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.me-role-label { font-size: 13px; color: var(--el-text-color-regular); }
.me-role-sel { width: 150px; }
.me-rows { display: flex; flex-direction: column; gap: 8px; }
.me-row {
  display: flex; align-items: center; justify-content: space-between;
  border: 1px solid var(--el-border-color-light); border-radius: 8px; padding: 8px 10px;
  background: var(--el-bg-color-overlay);
}
.me-row-group { background: var(--el-fill-color-light); }
.me-row-item.is-child { margin-left: 28px; }
.me-row-main { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.me-group-title { width: 160px; }
.me-icon-sel { width: 130px; }
.me-parent-sel { width: 180px; }
.me-item-title { font-weight: 500; }
.me-row-ops { display: flex; align-items: center; gap: 2px; }
.me-addable { margin-top: 14px; padding-top: 12px; border-top: 1px dashed var(--el-border-color); }
.me-addable-title { font-size: 12px; color: var(--el-text-color-secondary); margin-bottom: 8px; }
.me-preview { background: var(--el-bg-color-overlay); }
.me-preview-menu { border-right: none; }
.me-preview-menu :deep(.is-disabled) { opacity: 1 !important; cursor: default !important; color: var(--el-text-color-primary) !important; }
</style>
