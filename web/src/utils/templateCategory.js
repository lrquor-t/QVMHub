export const DEFAULT_LINUX_TEMPLATE_CATEGORY = 'Ubuntu'
export const DEFAULT_WINDOWS_TEMPLATE_CATEGORY = 'WindowsServer2022'
export const DEFAULT_OPENWRT_TEMPLATE_CATEGORY = 'OpenWrt'

export const LINUX_TEMPLATE_CATEGORY_OPTIONS = [
  'Ubuntu',
  'Debian',
  'CentOS',
]

export const WINDOWS_TEMPLATE_CATEGORY_OPTIONS = [
  'WindowsServer2025',
  'WindowsServer2022',
  'Windows11',
  'Windows10',
  'WindowsServer2012R2',
  '其它',
]

export const OPENWRT_TEMPLATE_CATEGORY_OPTIONS = [
  'OpenWrt',
  'iStoreOS',
]

export const normalizeTemplateType = (type) => (type || '').toString().trim().toLowerCase()

export const templateTypeLabel = (type) => ({
  windows: 'Windows',
  fnos: 'FnOS',
  openwrt: 'OpenWrt',
}[normalizeTemplateType(type)] || 'Linux')

export const normalizeTemplateCategory = (type, category) => {
  const normalizedType = normalizeTemplateType(type)
  if (normalizedType !== 'linux' && normalizedType !== 'windows' && normalizedType !== 'openwrt') {
    return ''
  }
  const normalized = (category || '').toString().trim()
  let options, defaultCategory
  if (normalizedType === 'windows') {
    options = WINDOWS_TEMPLATE_CATEGORY_OPTIONS
    defaultCategory = DEFAULT_WINDOWS_TEMPLATE_CATEGORY
  } else if (normalizedType === 'openwrt') {
    options = OPENWRT_TEMPLATE_CATEGORY_OPTIONS
    defaultCategory = DEFAULT_OPENWRT_TEMPLATE_CATEGORY
  } else {
    options = LINUX_TEMPLATE_CATEGORY_OPTIONS
    defaultCategory = DEFAULT_LINUX_TEMPLATE_CATEGORY
  }
  if (!normalized) {
    return defaultCategory
  }
  const matched = options.find(item => item.toLowerCase() === normalized.toLowerCase())
  return matched || defaultCategory
}

export const templateCategoryLabel = (type, category) => {
  const normalizedType = normalizeTemplateType(type)
  if (normalizedType !== 'linux' && normalizedType !== 'windows' && normalizedType !== 'openwrt') {
    return ''
  }
  return normalizeTemplateCategory(normalizedType, category)
}

export const templateGroupLabel = (type, category) => {
  const typeLabel = templateTypeLabel(type)
  const categoryLabel = templateCategoryLabel(type, category)
  return categoryLabel ? `${typeLabel} / ${categoryLabel}` : typeLabel
}
