<script lang="ts">
  import { onMount } from 'svelte'
  import type { EChartsCoreOption } from 'echarts/core'
  import type { app } from '../../wailsjs/go/models'
  import { adviceLabel } from '../lib/advice'
  import { toTreemapData, type TreemapDatum } from '../lib/chart'
  import { echarts, escapeHTML, type EChartsType } from '../lib/echarts'
  import { formatBytes, formatCount, formatPercent, type Unit } from '../lib/format'
  import { getLocale, t, type Locale } from '../lib/i18n'
  import { getUnit } from '../lib/prefs.svelte'
  import { chartTheme, mixColor, type ChartTheme } from '../lib/theme'
  import type { ChartKind } from './item'

  interface Props {
    view: app.View
    kind: ChartKind
    dark: boolean
    onOpenDir: (id: number) => void
    onSelectFile: (d: TreemapDatum) => void
    onOpenAggregate: (d: TreemapDatum) => void
  }
  let { view, kind, dark, onOpenDir, onSelectFile, onOpenAggregate }: Props = $props()

  interface Params {
    data?: TreemapDatum
  }

  let el = $state<HTMLDivElement>()
  let chart = $state.raw<EChartsType>()

  function adviceColor(d: TreemapDatum, th: ChartTheme): string | null {
    return d.advice === 'protected' ? th.protected : d.advice === 'recommended' ? th.recommended : null
  }

  function decorate(nodes: TreemapDatum[], th: ChartTheme, border: string | null, parent?: string): object[] {
    return nodes.map((d, i) => {
      const o: Record<string, unknown> = { ...d }
      // Children get darker shades of their parent so sibling tiles stay distinguishable.
      const c = d.aggregate
        ? th.aggregate
        : parent
          ? mixColor(parent, '#000000', 0.1 + (i % 4) * 0.07)
          : th.palette[i % th.palette.length]
      o.itemStyle = { color: c, borderColor: adviceColor(d, th) ?? border ?? c }
      if (d.children) o.children = decorate(d.children, th, border, c)
      return o
    })
  }

  function buildOption(v: app.View, k: ChartKind, th: ChartTheme, unit: Unit, locale: Locale): EChartsCoreOption {
    // Treemap parents are filled with their border color, so it must match the tile color there.
    const data = decorate(
      toTreemapData(v, (n) => t('results.aggregate', { count: formatCount(n, locale) })),
      th,
      k === 'treemap' ? null : th.border,
    )
    const size = (n: number) => formatBytes(n, unit)
    // Small tiles get a shorter label (or none) instead of a truncated two-line one.
    const tileLabel = (p: Params) => {
      const d = p.data
      if (!d) return ''
      const frac = v.Size > 0 ? d.value / v.Size : 0
      if (frac < 0.01) return ''
      return frac < 0.02 ? d.name : `${d.name}\n${size(d.value)}`
    }
    const shadow = { textShadowColor: 'rgba(0,0,0,0.45)', textShadowBlur: 2 }
    const tooltip = {
      confine: true,
      backgroundColor: th.tooltipBg,
      borderColor: th.tooltipBorder,
      textStyle: { color: th.tooltipText, fontSize: 12 },
      extraCssText: 'max-width: 360px; white-space: normal; word-break: break-all;',
      formatter: (p: Params) => {
        const d = p.data
        if (!d || typeof d.value !== 'number') return ''
        const lines = [
          `<div style="font-weight:600;margin-bottom:2px">${escapeHTML(d.name)}</div>`,
          `${t('detail.size')}: ${size(d.value)} · ${formatPercent(d.value, v.Size)}`,
        ]
        if (d.isDir || d.aggregate) lines.push(`${t('detail.files')}: ${formatCount(d.fileCount, locale)}`)
        const color = adviceColor(d, th)
        if (color) lines.push(`<span style="color:${color}">${escapeHTML(adviceLabel(d.advice, d.reason, t))}</span>`)
        return lines.join('<br>')
      },
    }
    const series =
      k === 'treemap'
        ? {
            type: 'treemap',
            data,
            roam: false,
            nodeClick: false,
            breadcrumb: { show: false },
            left: 0,
            top: 0,
            right: 0,
            bottom: 0,
            animation: false,
            label: {
              show: true,
              color: th.label,
              fontSize: 12,
              overflow: 'truncate',
              ...shadow,
              formatter: tileLabel,
            },
            upperLabel: {
              show: true,
              height: 20,
              color: th.upperLabel,
              fontSize: 11,
              fontWeight: 500,
              overflow: 'truncate',
              ...shadow,
              formatter: (p: Params) => (p.data ? `${p.data.name}  ${size(p.data.value)}` : ''),
            },
            itemStyle: { borderColor: th.border },
            levels: [
              { itemStyle: { borderColor: th.border, borderWidth: 0, gapWidth: 2 }, upperLabel: { show: false } },
              { itemStyle: { borderWidth: 2, gapWidth: 1 } },
              { itemStyle: { borderWidth: 0, gapWidth: 1 } },
            ],
          }
        : {
            type: 'sunburst',
            data,
            radius: ['15%', '95%'],
            nodeClick: false,
            animation: false,
            itemStyle: { borderColor: th.border, borderWidth: 1 },
            label: { color: th.label, fontSize: 11, minAngle: 8, rotate: 'radial', ...shadow },
            emphasis: { focus: 'ancestor' },
            levels: [{}, {}, { label: { fontSize: 10 } }],
          }
    return { tooltip, series: [series] }
  }

  onMount(() => {
    const c = echarts.init(el!)
    c.on('click', (p) => {
      const d = (p as Params).data
      if (!d) return
      if (d.aggregate) {
        onOpenAggregate(d)
        return
      }
      if (typeof d.id !== 'number') return
      if (d.isDir && d.id >= 0) onOpenDir(d.id)
      else if (!d.isDir) onSelectFile(d)
    })
    const ro = new ResizeObserver(() => c.resize())
    ro.observe(el!)
    chart = c
    return () => {
      ro.disconnect()
      c.dispose()
    }
  })

  $effect(() => {
    chart?.setOption(buildOption(view, kind, chartTheme(dark), getUnit(), getLocale()), { notMerge: true })
  })
</script>

<div bind:this={el} class="h-full w-full"></div>
