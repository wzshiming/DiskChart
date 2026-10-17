import * as echarts from 'echarts/core'
import { PieChart, SunburstChart, TreemapChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([TreemapChart, SunburstChart, PieChart, TooltipComponent, CanvasRenderer])

export { echarts }
export type { EChartsType } from 'echarts/core'

const ESCAPES: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }

export function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ESCAPES[c])
}
