<script lang="ts">
  import { onMount } from 'svelte'
  import type { app } from '../../wailsjs/go/models'
  import { echarts, escapeHTML, type EChartsType } from '../lib/echarts'
  import { formatBytes, formatCount, formatPercent } from '../lib/format'
  import { getLocale, t } from '../lib/i18n'
  import { getUnit } from '../lib/prefs.svelte'
  import { categoryColors, chartTheme } from '../lib/theme'

  interface Props {
    report: app.ExtReport | null
    dark: boolean
  }
  let { report, dark }: Props = $props()

  interface Row {
    label: string
    category: string
    size: number
    count: number
  }

  let el = $state<HTMLDivElement>()
  let chart = $state.raw<EChartsType>()

  const rows = $derived.by((): Row[] => {
    if (!report) return []
    const out: Row[] = (report.Items ?? []).map((it) => ({
      label: it.Ext ? (it.Ext.startsWith('.') ? it.Ext : '.' + it.Ext) : t('types.noExt'),
      category: it.Category,
      size: it.Size,
      count: it.Count,
    }))
    if (report.Other && report.Other.Size > 0) {
      out.push({ label: t('types.other'), category: 'other', size: report.Other.Size, count: report.Other.Count })
    }
    return out
  })

  const color = (category: string) => categoryColors[category] ?? categoryColors.other

  onMount(() => {
    const c = echarts.init(el!)
    const ro = new ResizeObserver(() => c.resize())
    ro.observe(el!)
    chart = c
    return () => {
      ro.disconnect()
      c.dispose()
    }
  })

  $effect(() => {
    const unit = getUnit()
    const th = chartTheme(dark)
    const total = report?.Total ?? 0
    chart?.setOption(
      {
        tooltip: {
          confine: true,
          backgroundColor: th.tooltipBg,
          borderColor: th.tooltipBorder,
          textStyle: { color: th.tooltipText, fontSize: 12 },
          formatter: (p: { name: string; value: number }) =>
            `<b>${escapeHTML(p.name)}</b><br>${formatBytes(p.value, unit)} · ${formatPercent(p.value, total)}`,
        },
        series: [
          {
            type: 'pie',
            radius: ['50%', '85%'],
            animation: false,
            label: { show: false },
            itemStyle: { borderColor: th.border, borderWidth: 1 },
            data: rows.map((r) => ({ name: r.label, value: r.size, itemStyle: { color: color(r.category) } })),
          },
        ],
      },
      { notMerge: true },
    )
  })
</script>

<div class="flex flex-col">
  <div bind:this={el} class="h-44 w-full shrink-0"></div>
  {#if report === null}
    <p class="p-4 text-sm text-zinc-500">{t('common.loading')}</p>
  {:else if rows.length === 0}
    <p class="p-4 text-sm text-zinc-500">{t('results.empty')}</p>
  {:else}
    <table class="w-full text-xs">
      <tbody>
        {#each rows as r (r.label)}
          <tr class="border-t border-zinc-100 dark:border-zinc-800">
            <td class="py-1.5 pl-3">
              <span class="flex items-center gap-2">
                <span class="size-2.5 shrink-0 rounded-sm" style:background-color={color(r.category)}></span>
                <span class="truncate font-mono">{r.label}</span>
              </span>
            </td>
            <td class="py-1.5 text-zinc-500 dark:text-zinc-400">{t('types.' + r.category)}</td>
            <td class="py-1.5 text-right font-medium tabular-nums">{formatBytes(r.size, getUnit())}</td>
            <td class="py-1.5 text-right text-zinc-500 tabular-nums dark:text-zinc-400">{formatCount(r.count, getLocale())}</td>
            <td class="py-1.5 pr-3 text-right text-zinc-500 tabular-nums dark:text-zinc-400">
              {formatPercent(r.size, report.Total)}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>
