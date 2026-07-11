<template>
  <el-card class="box-card chart-card">
    <template #header>
      <div class="chart-header">
        <div class="chart-header-left">
          <el-radio-group v-model="chartMode" size="small" @change="onChartModeChange">
            <el-radio-button value="realtime">实时监控</el-radio-button>
            <el-radio-button value="history">历史查询</el-radio-button>
          </el-radio-group>
        </div>
        <div class="chart-header-right" v-if="chartMode === 'history'">
          <el-date-picker
            v-model="historyDateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            size="small"
            style="width: 280px; margin-right: 10px;"
          />
          <el-button type="primary" size="small" @click="fetchHistory" :loading="historyLoading">查询</el-button>
          <el-button type="success" size="small" @click="fetchLast24Hours" :loading="historyLoading">近24小时</el-button>
        </div>
      </div>
    </template>
    <el-row :gutter="20">
      <el-col :span="12">
        <div ref="cpuChartRef" class="chart-container"></div>
      </el-col>
      <el-col :span="12">
        <div ref="memChartRef" class="chart-container"></div>
      </el-col>
      <el-col :span="12" style="margin-top: 20px;">
        <div ref="netChartRef" class="chart-container"></div>
      </el-col>
      <el-col :span="12" style="margin-top: 20px;">
        <div ref="diskChartRef" class="chart-container"></div>
      </el-col>
    </el-row>
  </el-card>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as echarts from 'echarts'
import { ElMessage } from 'element-plus'
import { getVmStats, getVmStatsHistory } from '@/api/vm'
import { getLXCStats, getLXCStatsHistory } from '@/api/lxc'

// 动态导入以支持host stats API （假定我们在别的组件创建了 api/host）
// 这里引入宿主机API如果放在统一的 api 文件会更好，我们可以内联请求或者添加一个通用的 utils/request
import request from '@/utils/request'

const props = defineProps({
  type: {
    type: String,
    required: true, // 'vm' | 'host' | 'lxc'
  },
  name: {
    type: String,
    default: '' // VM name (required if type === 'vm')
  },
  status: {
    type: String,
    default: 'running' // Only relevant for VM
  },
  defaultMode: {
    type: String,
    default: 'realtime' // 'realtime' or 'history'
  },
  externalStats: {
    type: Object,
    default: null // 外部传入的 stats 数据（SSE 推送）
  },
  disablePolling: {
    type: Boolean,
    default: false // 为 true 时禁用自行 XHR 轮询，由外部通过 externalStats 驱动
  },
  diskIoMode: {
    type: String,
    default: 'throughput' // 'throughput' | 'iops'
  }
})

// type 在组件生命周期内固定（vm/host/lxc），用普通常量即可。
const isLXC = props.type === 'lxc'

const cpuChartRef = ref(null)
const memChartRef = ref(null)
const netChartRef = ref(null)
const diskChartRef = ref(null)

const chartMode = ref(props.defaultMode)
const historyDateRange = ref([])
const historyLoading = ref(false)

let charts = []
let timer = null

const commonChartOptions = (yAxisMax = undefined, yAxisFormatter = undefined) => {
  const yAxis = { 
    type: 'value', 
    max: yAxisMax,
    splitLine: { lineStyle: { color: 'rgba(150, 150, 150, 0.15)' } }
  }
  if (yAxisFormatter) {
    yAxis.axisLabel = { formatter: yAxisFormatter }
  }
  return {
    tooltip: { trigger: 'axis' },
    grid: { left: '3%', right: '4%', bottom: '15%', containLabel: true },
    textStyle: { color: '#8f9399' },
    xAxis: { 
      type: 'category', 
      boundaryGap: false, 
      data: [],
      axisLine: { lineStyle: { color: 'rgba(150, 150, 150, 0.3)' } }
    },
    yAxis,
    dataZoom: [
      { type: 'inside', start: 0, end: 100 },
      { type: 'slider', start: 0, end: 100, bottom: 0, textStyle: { color: '#8f9399' } }
    ],
  }
}

const initCharts = () => {
  // Y 轴流量值自动单位转换（图表内部数据以 KB/s 为单位）
  const formatAxisTraffic = (valueKB) => {
    if (valueKB == null || valueKB === '' || Number(valueKB) < 0) return '0 KB/s'
    const units = ['KB/s', 'MB/s', 'GB/s', 'TB/s']
    let val = Number(valueKB)
    let idx = 0
    while (val >= 1024 && idx < units.length - 1) {
      val /= 1024
      idx += 1
    }
    return `${val.toFixed(1)} ${units[idx]}`
  }

  const cpuChart = echarts.init(cpuChartRef.value)
  cpuChart.setOption({
    ...commonChartOptions(100),
    title: { text: 'CPU 使用率 (%)', textStyle: { fontSize: 14, color: '#8f9399' } },
    series: [{ name: 'CPU', type: 'line', smooth: true, areaStyle: {}, data: [], itemStyle: { color: '#409EFF' } }]
  })

  const memChart = echarts.init(memChartRef.value)
  memChart.setOption({
    ...commonChartOptions(100),
    title: { text: '内存使用率 (%)', textStyle: { fontSize: 14, color: '#8f9399' } },
    series: [{ name: '内存', type: 'line', smooth: true, areaStyle: {}, data: [], itemStyle: { color: '#67C23A' } }]
  })

  const netChart = echarts.init(netChartRef.value)
  netChart.setOption({
    ...commonChartOptions(undefined, formatAxisTraffic),
    title: { text: '网络流量', textStyle: { fontSize: 14, color: '#8f9399' } },
    series: [
      { name: '接收', type: 'line', smooth: true, data: [] },
      { name: '发送', type: 'line', smooth: true, data: [] }
    ],
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value) => formatAxisTraffic(value)
    }
  })

  const diskChart = echarts.init(diskChartRef.value)
  diskChart.setOption({
    ...commonChartOptions(undefined, isLXC ? undefined : getDiskYAxisFormatter()),
    title: { text: isLXC ? '磁盘使用量 (GB)' : getDiskChartTitle(), textStyle: { fontSize: 14, color: '#8f9399' } },
    series: isLXC
      ? [{ name: '已用', type: 'line', smooth: true, areaStyle: {}, data: [] }]
      : [
          { name: '读取', type: 'line', smooth: true, data: [] },
          { name: '写入', type: 'line', smooth: true, data: [] }
        ],
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value) => props.diskIoMode === 'iops' ? value.toFixed(0) + ' IOPS' : formatAxisTraffic(value)
    }
  })

  charts = [cpuChart, memChart, netChart, diskChart]
}

let prevNet = { rx: 0, tx: 0 }
let prevDisk = { rd: 0, wr: 0 }

const fetchStatsData = async () => {
  if (props.type === 'lxc') {
    const res = await getLXCStats(props.name)
    return res.data
  } else if (props.type === 'vm') {
    const res = await getVmStats(props.name)
    return res.data
  } else {
    const res = await request({ url: '/host/stats', method: 'get' })
    return res.data
  }
}

// 使用 stats 数据更新图表（纯数据驱动，不关心数据来源）
const updateChartsWithData = (stats) => {
  if (chartMode.value !== 'realtime') return
  if (!stats || !charts.length) return

  const now = new Date()
  const timeStr = `${now.getHours()}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`

  // CPU
  const cpuOpt = charts[0].getOption()
  if (cpuOpt.xAxis[0].data.length > 30) cpuOpt.xAxis[0].data.shift()
  cpuOpt.xAxis[0].data.push(timeStr)
  if (cpuOpt.series[0].data.length > 30) cpuOpt.series[0].data.shift()
  cpuOpt.series[0].data.push(stats.cpu_percent || 0)
  charts[0].setOption(cpuOpt)

  // 内存
  const memOpt = charts[1].getOption()
  if (memOpt.xAxis[0].data.length > 30) memOpt.xAxis[0].data.shift()
  memOpt.xAxis[0].data.push(timeStr)
  if (memOpt.series[0].data.length > 30) memOpt.series[0].data.shift()
  const memPercent = stats.mem_total > 0 ? ((stats.mem_used / stats.mem_total) * 100).toFixed(1) : 0
  memOpt.series[0].data.push(Number(memPercent))
  charts[1].setOption(memOpt)

  // 网络（增量计算）
  const sseInterval = props.externalStats ? 3 : 5 // SSE 推送间隔3秒，自行轮询5秒
  const netOpt = charts[2].getOption()
  if (netOpt.xAxis[0].data.length > 30) netOpt.xAxis[0].data.shift()
  netOpt.xAxis[0].data.push(timeStr)
  const rxRate = prevNet.rx > 0 ? Math.max(0, (stats.net_rx_bytes - prevNet.rx) / 1024 / sseInterval) : 0
  const txRate = prevNet.tx > 0 ? Math.max(0, (stats.net_tx_bytes - prevNet.tx) / 1024 / sseInterval) : 0
  prevNet = { rx: stats.net_rx_bytes, tx: stats.net_tx_bytes }
  if (netOpt.series[0].data.length > 30) netOpt.series[0].data.shift()
  if (netOpt.series[1].data.length > 30) netOpt.series[1].data.shift()
  netOpt.series[0].data.push(rxRate.toFixed(1))
  netOpt.series[1].data.push(txRate.toFixed(1))
  charts[2].setOption(netOpt)

  // 磁盘：LXC 画使用量(GB 绝对值)；vm/host 画 I/O 速率（吞吐量/IOPS 双模式）
  const diskOpt = charts[3].getOption()
  if (diskOpt.xAxis[0].data.length > 30) diskOpt.xAxis[0].data.shift()
  diskOpt.xAxis[0].data.push(timeStr)
  if (isLXC) {
    const usedGB = stats.disk_total_bytes > 0 ? (stats.disk_used_bytes / 1e9) : 0
    if (diskOpt.series[0].data.length > 30) diskOpt.series[0].data.shift()
    diskOpt.series[0].data.push(Number(usedGB.toFixed(2)))
  } else {
    if (diskOpt.series[0].data.length > 30) diskOpt.series[0].data.shift()
    if (diskOpt.series[1].data.length > 30) diskOpt.series[1].data.shift()
    if (props.diskIoMode === 'iops') {
      // IOPS 模式：基于累计操作次数计算
      const rdIops = prevDisk.rd > 0 ? Math.max(0, (stats.disk_rd_ops - prevDisk.rd) / sseInterval) : 0
      const wrIops = prevDisk.wr > 0 ? Math.max(0, (stats.disk_wr_ops - prevDisk.wr) / sseInterval) : 0
      prevDisk = { rd: stats.disk_rd_ops, wr: stats.disk_wr_ops }
      diskOpt.series[0].data.push(rdIops)
      diskOpt.series[1].data.push(wrIops)
    } else {
      const rdRate = prevDisk.rd > 0 ? Math.max(0, (stats.disk_rd_bytes - prevDisk.rd) / 1024 / sseInterval) : 0
      const wrRate = prevDisk.wr > 0 ? Math.max(0, (stats.disk_wr_bytes - prevDisk.wr) / 1024 / sseInterval) : 0
      prevDisk = { rd: stats.disk_rd_bytes, wr: stats.disk_wr_bytes }
      diskOpt.series[0].data.push(rdRate.toFixed(1))
      diskOpt.series[1].data.push(wrRate.toFixed(1))
    }
  }
  charts[3].setOption(diskOpt)
}

// 自行拉取数据并更新图表（无外部 SSE 时使用）
const updateCharts = async () => {
  if (chartMode.value !== 'realtime') return
  const statusLower = (props.status || '').toLowerCase()
  if ((props.type === 'vm' || props.type === 'lxc') && statusLower !== 'running') return

  try {
    const stats = await fetchStatsData()
    updateChartsWithData(stats)
  } catch (err) {
    console.error('获取资源统计失败', err)
  }
}

// 监听外部 SSE 推送的 stats 数据变化
watch(() => props.externalStats, (newStats) => {
  if (newStats && chartMode.value === 'realtime') {
    updateChartsWithData(newStats)
  }
}, { deep: true })

// 父组件切换 :name 但未卸载本组件时（如抽屉 open(row,'monitor') 直达监控 tab）：
// 重置实时窗口与累计基线，避免两个容器数据混算，并按当前模式重拉。
watch(() => props.name, () => {
  if (!props.name) return
  prevNet = { rx: 0, tx: 0 }
  prevDisk = { rd: 0, wr: 0 }
  charts.forEach(chart => {
    if (!chart) return
    const o = chart.getOption()
    if (o?.xAxis?.[0]) o.xAxis[0].data = []
    if (o?.series) o.series.forEach(s => { s.data = [] })
    chart.setOption(o)
  })
  if (chartMode.value === 'realtime') updateCharts()
  else if (chartMode.value === 'history') fetchLast24Hours()
})

// 监听磁盘 IO 模式切换，更新图表标题和 Y 轴格式
const getDiskChartTitle = () => props.diskIoMode === 'iops' ? '磁盘 I/O (IOPS)' : '磁盘 I/O'
const getDiskYAxisFormatter = () => {
  if (props.diskIoMode === 'iops') {
    return (value) => {
      if (value >= 1000) return (value / 1000).toFixed(1) + 'K IOPS'
      return value.toFixed(0) + ' IOPS'
    }
  }
  return (valueKB) => {
    if (valueKB == null || valueKB === '' || Number(valueKB) < 0) return '0 KB/s'
    const units = ['KB/s', 'MB/s', 'GB/s', 'TB/s']
    let val = Number(valueKB)
    let idx = 0
    while (val >= 1024 && idx < units.length - 1) {
      val /= 1024
      idx += 1
    }
    return `${val.toFixed(1)} ${units[idx]}`
  }
}
watch(() => props.diskIoMode, () => {
  if (charts[3]) {
    charts[3].setOption({
      title: { text: getDiskChartTitle(), textStyle: { fontSize: 14, color: '#8f9399' } },
      yAxis: { axisLabel: { formatter: getDiskYAxisFormatter() } }
    })
    prevDisk = { rd: 0, wr: 0 }
  }
})

const fetchHistoryData = async (start, end) => {
  if (props.type === 'lxc') {
    const res = await getLXCStatsHistory(props.name, start, end)
    return res.data
  } else if (props.type === 'vm') {
    const res = await getVmStatsHistory(props.name, start, end)
    return res.data
  } else {
    const res = await request({ url: '/host/stats/history', method: 'get', params: { start, end } })
    return res.data
  }
}

const fetchHistory = async () => {
  if (!historyDateRange.value || historyDateRange.value.length !== 2) {
    ElMessage.warning('请选择查询日期范围')
    return
  }

  historyLoading.value = true
  try {
    const [start, end] = historyDateRange.value
    const records = await fetchHistoryData(start, end) || []

    if (records.length === 0) {
      ElMessage.info('所选日期范围内无数据')
      return
    }

    renderHistoryCharts(records)
  } catch (err) {
    console.error('查询历史记录失败', err)
  } finally {
    historyLoading.value = false
  }
}

const fetchLast24Hours = async () => {
  historyLoading.value = true
  try {
    const end = new Date()
    const start = new Date(end.getTime() - 24 * 3600 * 1000)
    
    // Format to YYYY-MM-DDTHH:mm:ss for backend compatibility
    const formatTime = (d) => {
      const pad = (n) => String(n).padStart(2, '0')
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
    }

    const records = await fetchHistoryData(formatTime(start), formatTime(end)) || []
    
    // Make sure we update the date range picker visually (requires exact match with component type though, so maybe just leave it or set text)
    if (records.length === 0) {
      ElMessage.info('过去24小时内无数据')
      return
    }

    renderHistoryCharts(records)
  } catch (err) {
    console.error('查询24小时历史记录失败', err)
  } finally {
    historyLoading.value = false
  }
}

const renderHistoryCharts = (records) => {
  const timeLabels = records.map(r => {
    const d = new Date(r.recorded_at)
    return `${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
  })

  // CPU
  charts[0].setOption({
    title: { text: 'CPU 使用率 (%) - 历史', textStyle: { fontSize: 14, color: '#8f9399' } },
    xAxis: { data: timeLabels },
    series: [{ data: records.map(r => r.cpu_percent.toFixed(1)) }]
  })

  // 内存
  charts[1].setOption({
    title: { text: '内存使用率 (%) - 历史', textStyle: { fontSize: 14, color: '#8f9399' } },
    xAxis: { data: timeLabels },
    series: [{
      data: records.map(r => r.mem_total > 0 ? ((r.mem_used / r.mem_total) * 100).toFixed(1) : 0)
    }]
  })

  // 网络
  const netRx = [], netTx = []
  for (let i = 0; i < records.length; i++) {
    if (i === 0) {
      netRx.push(0)
      netTx.push(0)
    } else {
      const dt = (new Date(records[i].recorded_at) - new Date(records[i - 1].recorded_at)) / 1000
      let rx = 0, tx = 0
      if (dt > 0) {
        rx = Math.max(0, (records[i].net_rx_bytes - records[i - 1].net_rx_bytes) / 1024 / dt)
        tx = Math.max(0, (records[i].net_tx_bytes - records[i - 1].net_tx_bytes) / 1024 / dt)
      }
      netRx.push(rx.toFixed(1))
      netTx.push(tx.toFixed(1))
    }
  }
  charts[2].setOption({
    title: { text: '网络流量 - 历史', textStyle: { fontSize: 14, color: '#8f9399' } },
    xAxis: { data: timeLabels },
    series: [{ data: netRx }, { data: netTx }]
  })

  // 磁盘：LXC 画使用量(GB 绝对值)；vm/host 画 I/O 速率（吞吐量/IOPS 双模式）
  if (isLXC) {
    charts[3].setOption({
      title: { text: '磁盘使用量 (GB) - 历史', textStyle: { fontSize: 14, color: '#8f9399' } },
      xAxis: { data: timeLabels },
      series: [{ data: records.map(r => r.disk_total_bytes > 0 ? Number((r.disk_used_bytes / 1e9).toFixed(2)) : 0) }]
    })
  } else {
    const diskRd = [], diskWr = []
    const isIopsMode = props.diskIoMode === 'iops'
    for (let i = 0; i < records.length; i++) {
      if (i === 0) {
        diskRd.push(0)
        diskWr.push(0)
      } else {
        const dt = (new Date(records[i].recorded_at) - new Date(records[i - 1].recorded_at)) / 1000
        let rd = 0, wr = 0
        if (dt > 0) {
          if (isIopsMode) {
            rd = Math.max(0, (records[i].disk_rd_ops - records[i - 1].disk_rd_ops) / dt)
            wr = Math.max(0, (records[i].disk_wr_ops - records[i - 1].disk_wr_ops) / dt)
          } else {
            rd = Math.max(0, (records[i].disk_rd_bytes - records[i - 1].disk_rd_bytes) / 1024 / dt)
            wr = Math.max(0, (records[i].disk_wr_bytes - records[i - 1].disk_wr_bytes) / 1024 / dt)
            // 防抖动限制
            if (rd > 10 * 1024 * 1024) rd = 0
            if (wr > 10 * 1024 * 1024) wr = 0
          }
        }
        diskRd.push(isIopsMode ? rd : rd.toFixed(1))
        diskWr.push(isIopsMode ? wr : wr.toFixed(1))
      }
    }
    charts[3].setOption({
      title: { text: getDiskChartTitle() + ' - 历史', textStyle: { fontSize: 14, color: '#8f9399' } },
      xAxis: { data: timeLabels },
      series: [{ data: diskRd }, { data: diskWr }]
    })
  }
}

const onChartModeChange = (mode) => {
  if (mode === 'realtime') {
    charts.forEach(chart => {
      if (!chart) return
      const opt = chart.getOption()
      if (opt) {
        if (opt.xAxis && opt.xAxis[0]) opt.xAxis[0].data = []
        if (opt.series) opt.series.forEach(s => { s.data = [] })
        chart.setOption(opt)
      }
    })
    if (charts[0]) charts[0].setOption({ title: { text: 'CPU 使用率 (%)', textStyle: { fontSize: 14, color: '#8f9399' } } })
    if (charts[1]) charts[1].setOption({ title: { text: '内存使用率 (%)', textStyle: { fontSize: 14, color: '#8f9399' } } })
    if (charts[2]) charts[2].setOption({ title: { text: '网络流量', textStyle: { fontSize: 14, color: '#8f9399' } } })
    if (charts[3]) charts[3].setOption({ title: { text: isLXC ? '磁盘使用量 (GB)' : getDiskChartTitle(), textStyle: { fontSize: 14, color: '#8f9399' } } })
    prevNet = { rx: 0, tx: 0 }
    prevDisk = { rd: 0, wr: 0 }
  } else if (mode === 'history') {
    // 自动加载一次近24小时的数据
    fetchLast24Hours()
  }
}

const handleResize = () => {
  charts.forEach(chart => {
    if (chart) chart.resize()
  })
}

let resizeObserver = null

onMounted(() => {
  initCharts()
  if (chartMode.value === 'history') {
    fetchLast24Hours()
  }

  // 应对 el-collapse 展开动画导致图表宽度为 100px 或 0 的问题
  // 动画通常 300ms，在动画期间及结束后强制重绘尺寸
  setTimeout(handleResize, 100)
  setTimeout(handleResize, 350)
  setTimeout(handleResize, 600)
    
  // 监听容器大小变化
  if (cpuChartRef.value) {
    resizeObserver = new ResizeObserver(() => {
      handleResize()
    })
    resizeObserver.observe(cpuChartRef.value)
  }

  // disablePolling 为 true 时由外部 SSE 驱动数据，不自行轮询
  if (!props.disablePolling) {
    timer = setInterval(updateCharts, 5000)
  }
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  window.removeEventListener('resize', handleResize)
  if (resizeObserver) resizeObserver.disconnect()
  charts.forEach(chart => {
    if (chart) chart.dispose()
  })
})
</script>

<style scoped>
.chart-container {
  width: 100%;
  height: 250px;
}
.chart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.chart-header-left {
  display: flex;
  align-items: center;
}
.chart-header-right {
  display: flex;
  align-items: center;
}

@media (max-width: 768px) {
  .chart-container {
    height: 200px;
  }

  .chart-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }

  .chart-header-right {
    width: 100%;
  }
}
</style>
