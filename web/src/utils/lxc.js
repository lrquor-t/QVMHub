// LXC 容器名随机生成（创建向导 / 克隆对话框共用）
// 格式 lxc-<6 位小写字母/数字>，匹配名称正则 ^[a-z0-9][a-z0-9-]{1,62}$
const lcCharset = 'abcdefghijklmnopqrstuvwxyz0123456789'

const getRandomInt = (max) => {
  const c = globalThis.crypto
  if (c && typeof c.getRandomValues === 'function') {
    const a = new Uint32Array(1)
    c.getRandomValues(a)
    return a[0] % max
  }
  return Math.floor(Math.random() * max)
}

const randomStringFrom = (charset, n) =>
  Array.from({ length: n }, () => charset[getRandomInt(charset.length)]).join('')

// generateRandomLXCName 返回形如 lxc-a1b2c3 的随机容器名。
export const generateRandomLXCName = () => `lxc-${randomStringFrom(lcCharset, 6)}`
