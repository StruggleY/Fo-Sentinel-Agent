import { useState, useEffect } from 'react'
import ReactECharts from 'echarts-for-react'
import { Loader2 } from 'lucide-react'
import { eventService } from '@/services/event'

export default function EventTrendChart() {
  const [loading, setLoading] = useState(true)
  const [dates, setDates] = useState<string[]>([])
  const [series, setSeries] = useState<{ critical: number[]; high: number[]; medium: number[]; low: number[] }>({
    critical: [],
    high: [],
    medium: [],
    low: [],
  })

  useEffect(() => {
    const fetchData = async () => {
      try {
        const items = await eventService.getTrend(30)
        if (items.length === 0) { setLoading(false); return }
        const sorted = [...items].reverse()
        setDates(sorted.map(i => i.date))
        setSeries({
          critical: sorted.map(i => i.critical),
          high: sorted.map(i => i.high),
          medium: sorted.map(i => i.medium),
          low: sorted.map(i => i.low),
        })
      } catch (error) {
        console.error('获取事件趋势失败:', error)
      } finally {
        setLoading(false)
      }
    }
    fetchData()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center" style={{ height: '300px' }}>
        <Loader2 className="w-6 h-6 animate-spin text-primary-500" />
      </div>
    )
  }

  if (dates.length === 0) {
    return (
      <div className="flex items-center justify-center text-gray-400 text-sm" style={{ height: '300px' }}>
        暂无事件趋势数据
      </div>
    )
  }

  const mkArea = (color: string, opacity0 = 0.25, opacity1 = 0.02) => ({
    color: {
      type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
      colorStops: [
        { offset: 0, color: color.replace(')', `,${opacity0})`).replace('rgb', 'rgba') },
        { offset: 1, color: color.replace(')', `,${opacity1})`).replace('rgb', 'rgba') },
      ],
    },
  })

  const option = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross', crossStyle: { color: '#94a3b8' }, lineStyle: { color: '#e2e8f0', type: 'dashed' } },
      backgroundColor: '#1e293b',
      borderColor: '#334155',
      borderWidth: 1,
      textStyle: { color: '#f1f5f9', fontSize: 13 },
      extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.4); border-radius: 8px;',
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      formatter: (params: any[]) => {
        if (!params.length) return ''
        let html = `<div style="font-weight:600;margin-bottom:6px;color:#94a3b8">${String(params[0].name).substring(0, 10)}</div>`
        params.forEach((p: any) => {
          html += `<div style="display:flex;align-items:center;gap:8px;margin:3px 0">${p.marker}<span style="flex:1">${p.seriesName}</span><strong>${p.value}</strong></div>`
        })
        return html
      },
    },
    legend: {
      data: ['严重', '高危', '中危', '低危'],
      textStyle: { color: '#64748b', fontSize: 12 },
      top: 4,
      right: 8,
      icon: 'circle',
      itemWidth: 8,
      itemHeight: 8,
      itemGap: 16,
    },
    graphic: dates.length > 0 ? [{
      type: 'text',
      left: 12,
      bottom: 4,
      style: {
        text: dates[0]?.slice(0, 4) === dates[dates.length - 1]?.slice(0, 4)
          ? dates[0]?.slice(0, 4) + ' 年'
          : `${dates[0]?.slice(0, 4)} — ${dates[dates.length - 1]?.slice(0, 4)}`,
        fill: '#94a3b8',
        fontSize: 11,
      },
    }] : [],
    grid: { left: 12, right: 12, bottom: 28, top: 36, containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: dates,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: '#94a3b8',
        fontSize: 11,
        rotate: 0,
        margin: 8,
        interval: dates.length > 20 ? Math.floor(dates.length / 10) : dates.length > 10 ? 1 : 0,
        formatter: (val: string) => val.slice(5).replace('-', '/'), // show MM/DD
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: '#94a3b8', fontSize: 11 },
      splitLine: { lineStyle: { color: '#f1f5f9', type: 'dashed' } },
    },
    series: [
      {
        name: '严重',
        type: 'line',
        smooth: 0.4,
        symbol: 'circle',
        symbolSize: 5,
        showSymbol: false,
        data: series.critical,
        itemStyle: { color: '#ef4444' },
        lineStyle: { color: '#ef4444', width: 2.5 },
        areaStyle: mkArea('rgb(239,68,68)', 0.2, 0),
        emphasis: { focus: 'series', itemStyle: { borderWidth: 2 } },
      },
      {
        name: '高危',
        type: 'line',
        smooth: 0.4,
        symbol: 'circle',
        symbolSize: 5,
        showSymbol: false,
        data: series.high,
        itemStyle: { color: '#f97316' },
        lineStyle: { color: '#f97316', width: 2.5 },
        areaStyle: mkArea('rgb(249,115,22)', 0.18, 0),
        emphasis: { focus: 'series', itemStyle: { borderWidth: 2 } },
      },
      {
        name: '中危',
        type: 'line',
        smooth: 0.4,
        symbol: 'circle',
        symbolSize: 5,
        showSymbol: false,
        data: series.medium,
        itemStyle: { color: '#eab308' },
        lineStyle: { color: '#eab308', width: 2.5 },
        areaStyle: mkArea('rgb(234,179,8)', 0.15, 0),
        emphasis: { focus: 'series', itemStyle: { borderWidth: 2 } },
      },
      {
        name: '低危',
        type: 'line',
        smooth: 0.4,
        symbol: 'circle',
        symbolSize: 5,
        showSymbol: false,
        data: series.low,
        itemStyle: { color: '#3b82f6' },
        lineStyle: { color: '#3b82f6', width: 2.5 },
        areaStyle: mkArea('rgb(59,130,246)', 0.15, 0),
        emphasis: { focus: 'series', itemStyle: { borderWidth: 2 } },
      },
    ],
    animation: true,
    animationDuration: 800,
    animationEasing: 'cubicOut',
  }

  return <ReactECharts option={option} style={{ height: '300px' }} opts={{ renderer: 'svg' }} />
}

