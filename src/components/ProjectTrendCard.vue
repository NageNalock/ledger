<script setup lang="ts">
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import VChart from 'vue-echarts'

use([CanvasRenderer, LineChart, TitleComponent, TooltipComponent, LegendComponent, GridComponent])

defineProps<{
  option: any
  loading: boolean
  granularity: 'day' | 'month'
}>()

const emit = defineEmits<{
  (e: 'update:granularity', v: 'day' | 'month'): void
  (e: 'point-click', params: any): void
}>()
</script>

<template>
  <div class="bg-white p-6 rounded-2xl shadow-card lg:col-span-2">
    <div class="flex flex-wrap items-center justify-between gap-3 mb-3">
      <h3 class="text-base font-bold text-gray-700 flex items-center gap-2">
        <div class="w-1 h-5 bg-primary rounded-full"></div> 趋势总览
      </h3>
      <el-radio-group
        :model-value="granularity"
        size="small"
        @update:model-value="(v: any) => emit('update:granularity', v)"
      >
        <el-radio-button label="day">按天</el-radio-button>
        <el-radio-button label="month">按月</el-radio-button>
      </el-radio-group>
    </div>
    <div class="h-[280px]" v-loading="loading">
      <v-chart class="chart" :option="option" autoresize @click="(p: any) => emit('point-click', p)" />
    </div>
  </div>
</template>

<style scoped>
.chart {
  height: 100%;
  width: 100%;
}
</style>
