<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import request from '@/api/request'
import dayjs from 'dayjs'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, BarChart, PieChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DataZoomComponent
} from 'echarts/components'
import VChart from 'vue-echarts'
import { CircleClose } from '@element-plus/icons-vue'

use([
  CanvasRenderer,
  LineChart,
  BarChart,
  PieChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DataZoomComponent
])


const dateRange = ref<[string, string]>(['', ''])
const granularity = ref<'day' | 'month'>('day')
const trendData = ref([])
const categoryData = ref([])
const loading = ref(false)
const quickSelect = ref('month') // month, year, all, custom

// Transaction List State
const transactions = ref([])
const totalTransactions = ref(0)
const currentPage = ref(1)
const pageSize = ref(10)
const listLoading = ref(false)
const filterCategory = ref('') // Clicked category name
const trendPointDate = ref('')
const categories = ref<any[]>([])
const listCategoryL1Id = ref('')
const listCategoryL2Id = ref('')
const listSort = ref('')

const categoryL2Options = computed(() => {
  const l1 = categories.value.find((c: any) => String(c.id) === String(listCategoryL1Id.value))
  return l1?.children || []
})

const fetchData = async () => {
  if (!dateRange.value || dateRange.value.length !== 2) return
  
  loading.value = true
  try {
    const [start, end] = dateRange.value
    
    // Fetch Trend
    const trendRes: any = await request.get('/reports/trend', {
      params: {
        start_date: start,
        end_date: end,
        type: granularity.value
      }
    })
    trendData.value = trendRes
    
    // Fetch Category
    const catRes: any = await request.get('/reports/category', {
      params: {
        start_date: start,
        end_date: end
      }
    })
    categoryData.value = catRes
    
    // Reset List Filter
    filterCategory.value = ''
    trendPointDate.value = ''
    listCategoryL1Id.value = ''
    listCategoryL2Id.value = ''
    listSort.value = ''
    currentPage.value = 1
    fetchTransactions()
    
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const fetchTransactions = async () => {
  if (!dateRange.value || dateRange.value.length !== 2) return
  
  listLoading.value = true
  try {
    let [start, end] = dateRange.value
    if (trendPointDate.value) {
      if (granularity.value === 'month') {
        const monthStart = dayjs(trendPointDate.value + '-01')
        start = monthStart.format('YYYY-MM-DD')
        end = monthStart.endOf('month').format('YYYY-MM-DD')
      } else {
        start = trendPointDate.value
        end = trendPointDate.value
      }
    }
    const params: any = {
      start_date: start,
      end_date: end,
      page: currentPage.value,
      page_size: pageSize.value
    }
    
    if (filterCategory.value) {
        params.category_l1 = filterCategory.value
    }

    if (!filterCategory.value && listCategoryL1Id.value) {
        params.category_l1_id = listCategoryL1Id.value
    }

    if (!filterCategory.value && listCategoryL2Id.value) {
        params.category_l2_id = listCategoryL2Id.value
    }

    if (listSort.value) {
        const [sortBy, sortOrder] = listSort.value.split('_')
        params.sort_by = sortBy
        params.sort_order = sortOrder
    }
    
    const res: any = await request.get('/transactions', { params })
    transactions.value = res.data
    totalTransactions.value = res.total
  } catch (e) {
    console.error(e)
  } finally {
    listLoading.value = false
  }
}

const handlePageChange = (page: number) => {
    currentPage.value = page
    fetchTransactions()
}

const handleChartClick = (params: any) => {
    // console.log('Chart Click:', params)
    if (params.seriesType === 'pie') {
        filterCategory.value = params.name
    } else if (params.seriesType === 'bar') {
        filterCategory.value = params.seriesName
    } else {
        return // Line chart click not mapped to category yet
    }
    listCategoryL1Id.value = ''
    listCategoryL2Id.value = ''
    currentPage.value = 1
    fetchTransactions()
}

const handleTrendClick = (params: any) => {
  if (params.seriesType !== 'line') return
  trendPointDate.value = params.name || params.axisValue
  currentPage.value = 1
  fetchTransactions()
}

const handleListCategoryL1Change = () => {
  if (!categoryL2Options.value.find((c: any) => String(c.id) === String(listCategoryL2Id.value))) {
    listCategoryL2Id.value = ''
  }
  filterCategory.value = ''
  currentPage.value = 1
  fetchTransactions()
}

const handleListCategoryL2Change = () => {
  filterCategory.value = ''
  currentPage.value = 1
  fetchTransactions()
}

const handleListSortChange = () => {
  currentPage.value = 1
  fetchTransactions()
}

const clearTrendPoint = () => {
  trendPointDate.value = ''
  currentPage.value = 1
  fetchTransactions()
}

// Quick Select Handlers
const handleQuickSelect = (val: string) => {
    quickSelect.value = val
    const now = dayjs()
    if (val === 'month') {
        dateRange.value = [now.startOf('month').format('YYYY-MM-DD'), now.endOf('month').format('YYYY-MM-DD')]
        granularity.value = 'day'
    } else if (val === 'year') {
        // Last 365 days
        dateRange.value = [now.subtract(1, 'year').add(1, 'day').format('YYYY-MM-DD'), now.format('YYYY-MM-DD')]
        granularity.value = 'month'
    } else if (val === 'all') {
        // Assume a reasonable range or dynamic
        dateRange.value = ['2020-01-01', now.endOf('year').format('YYYY-MM-DD')]
        granularity.value = 'month'
    }
}

// Watchers
watch([dateRange, granularity], () => {
  fetchData()
})

onMounted(() => {
  handleQuickSelect('month')
  fetchCategories()
})

const fetchCategories = async () => {
  try {
    const res: any = await request.get('/categories')
    categories.value = res || []
  } catch (e) {
    console.error(e)
  }
}

// Charts Options

// 1. Line Chart (Trend)
const lineOption = computed(() => ({
  color: ['#FF8A65', '#4DB6AC'],
  tooltip: {
    trigger: 'axis',
    backgroundColor: 'rgba(255, 255, 255, 0.9)',
    borderColor: '#eee',
    textStyle: { color: '#333' },
    formatter: (params: any) => {
        let res = `<div class="font-bold mb-1">${params[0].axisValue}</div>`
        params.forEach((p: any) => {
            res += `<div class="flex justify-between w-40 text-xs"><span>${p.marker}${p.seriesName}</span><span class="font-bold">¥${Number(p.value).toFixed(2)}</span></div>`
        })
        return res
    }
  },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: trendData.value.map((i: any) => i.date),
    axisLine: { lineStyle: { color: '#ddd' } },
    axisLabel: { color: '#666' }
  },
  yAxis: {
    type: 'value',
    axisLine: { show: false },
    axisTick: { show: false },
    splitLine: { lineStyle: { color: '#f5f5f5' } }
  },
  series: [
    {
      name: '支出',
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: 8,
      data: trendData.value.map((i: any) => i.amount),
      areaStyle: {
        opacity: 0.2,
        color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [{ offset: 0, color: '#FF8A65' }, { offset: 1, color: '#fff' }]
        }
      }
    }
  ]
}))

// 2. Stacked Bar Chart (Trend + Breakdown)
const barOption = computed(() => {
    // Extract all unique categories
    const categories = new Set<string>()
    trendData.value.forEach((d: any) => {
        if (d.breakdown) {
            Object.keys(d.breakdown).forEach(k => categories.add(k))
        }
    })
    const categoryList = Array.from(categories)

    const series = categoryList.map(cat => ({
        name: cat,
        type: 'bar',
        stack: 'total',
        emphasis: { focus: 'series' },
        data: trendData.value.map((d: any) => d.breakdown ? (d.breakdown[cat] || 0) : 0)
    }))

    return {
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        formatter: (params: any) => {
            let total = 0
            let res = `<div class="font-bold mb-1">${params[0].axisValue}</div>`
            params.forEach((p: any) => {
                if (p.value > 0) {
                    res += `<div class="flex justify-between w-40 text-xs"><span>${p.marker}${p.seriesName}</span><span class="font-bold">¥${Number(p.value).toFixed(2)}</span></div>`
                    total += p.value
                }
            })
            res += `<div class="border-t mt-1 pt-1 flex justify-between font-bold text-sm"><span>总计</span><span>¥${total.toFixed(2)}</span></div>`
            return res
        }
      },
      legend: { bottom: 0, type: 'scroll' },
      grid: { left: '3%', right: '4%', bottom: '10%', containLabel: true },
      xAxis: {
        type: 'category',
        data: trendData.value.map((i: any) => i.date),
        axisLine: { lineStyle: { color: '#ddd' } }
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: '#f5f5f5' } }
      },
      series: series
    }
})

// 3. Pie Chart (L1 + L2)
const pieOption = computed(() => {
    return {
      color: ['#FF8A65', '#4DB6AC', '#FFD54F', '#9575CD', '#4FC3F7', '#AED581', '#FFB74D', '#F06292'],
      tooltip: {
        trigger: 'item',
        formatter: (params: any) => {
            const data = params.data
            let res = `<div class="font-bold mb-1">${data.name}</div>`
            res += `<div class="text-sm">¥${Number(data.value).toFixed(2)} (${params.percent}%)</div>`
            
            if (data.children && data.children.length > 0) {
                res += `<div class="mt-2 border-t pt-2 text-xs text-gray-500">`
                data.children.forEach((c: any) => {
                    const percent = (c.value / data.value * 100).toFixed(1)
                    res += `<div class="flex justify-between"><span>${c.name}</span><span>¥${Number(c.value).toFixed(2)} (${percent}%)</span></div>`
                })
                res += `</div>`
            }
            return res
        }
      },
      legend: {
        top: '5%',
        left: 'center',
        itemWidth: 10,
        itemHeight: 10
      },
      series: [
        {
          name: '分类占比',
          type: 'pie',
          radius: ['40%', '70%'],
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 10,
            borderColor: '#fff',
            borderWidth: 2
          },
          label: { show: false },
          labelLine: { show: false },
          data: categoryData.value
        }
      ]
    }
})

const totalAmount = computed(() => {
  return trendData.value.reduce((sum, item: any) => sum + item.amount, 0).toFixed(2)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header Filter -->
    <div class="bg-white p-4 rounded-2xl shadow-card flex flex-wrap justify-between items-center gap-4">
       <div class="flex items-center gap-4 flex-wrap">
          <el-radio-group v-model="quickSelect" @change="handleQuickSelect">
             <el-radio-button label="month">本月</el-radio-button>
             <el-radio-button label="year">最近一年</el-radio-button>
             <el-radio-button label="all">全部</el-radio-button>
             <el-radio-button label="custom">自定义</el-radio-button>
          </el-radio-group>
          
          <el-date-picker
            v-if="quickSelect === 'custom'"
            v-model="dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            :clearable="false"
            style="width: 240px"
          />

          <div class="h-6 w-px bg-gray-200 mx-2"></div>

          <span class="text-sm text-gray-500 font-bold">聚合粒度:</span>
          <el-radio-group v-model="granularity" size="small">
             <el-radio-button label="day">按天</el-radio-button>
             <el-radio-button label="month">按月</el-radio-button>
          </el-radio-group>
       </div>
       
       <div class="flex flex-col items-end">
          <span class="text-xs text-gray-400 font-bold">总支出</span>
          <span class="text-2xl font-bold text-primary">¥{{ totalAmount }}</span>
       </div>
    </div>
    
    <!-- Charts Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
       <!-- Trend Line -->
       <div class="bg-white p-6 rounded-2xl shadow-card">
          <h3 class="text-lg font-bold text-gray-700 mb-4 flex items-center gap-2">
             <div class="w-1 h-5 bg-primary rounded-full"></div> 消费趋势 (总览)
          </h3>
          <div class="h-[350px]">
             <v-chart class="chart" :option="lineOption" autoresize @click="handleTrendClick" />
          </div>
       </div>
       
       <!-- Trend Bar (Stacked) -->
       <div class="bg-white p-6 rounded-2xl shadow-card">
          <h3 class="text-lg font-bold text-gray-700 mb-4 flex items-center gap-2">
             <div class="w-1 h-5 bg-secondary rounded-full"></div> 消费统计 (堆叠)
          </h3>
          <div class="h-[350px]">
             <v-chart class="chart" :option="barOption" autoresize />
          </div>
       </div>
       
       <!-- Category Pie -->
       <div class="bg-white p-6 rounded-2xl shadow-card lg:col-span-2">
          <h3 class="text-lg font-bold text-gray-700 mb-4 flex items-center gap-2">
             <div class="w-1 h-5 bg-accent rounded-full"></div> 分类占比 (含子分类)
          </h3>
          <div class="h-[500px]">
             <v-chart class="chart" :option="pieOption" autoresize @click="handleChartClick" />
          </div>
       </div>
    </div>

    <!-- Transaction List -->
    <div class="bg-white p-6 rounded-2xl shadow-card">
       <div class="flex flex-wrap items-center justify-between gap-3 mb-4">
           <h3 class="text-lg font-bold text-gray-700 flex items-center gap-2">
              <div class="w-1 h-5 bg-gray-400 rounded-full"></div> 
              明细列表 
              <span v-if="filterCategory" class="text-sm bg-primary/10 text-primary px-2 py-0.5 rounded ml-2">
                 已筛选: {{ filterCategory }} 
                 <el-icon class="cursor-pointer ml-1 translate-y-0.5" @click="filterCategory = ''; fetchTransactions()"><CircleClose /></el-icon>
              </span>
              <span v-if="trendPointDate" class="text-sm bg-secondary/10 text-secondary px-2 py-0.5 rounded ml-2">
                 已筛选: {{ trendPointDate }}
                 <el-icon class="cursor-pointer ml-1 translate-y-0.5" @click="clearTrendPoint"><CircleClose /></el-icon>
              </span>
           </h3>
           <div class="flex flex-wrap items-center gap-2">
             <el-select
               v-model="listCategoryL1Id"
               placeholder="一级分类"
               clearable
               style="width: 140px"
               @change="handleListCategoryL1Change"
             >
               <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
             </el-select>
             <el-select
               v-model="listCategoryL2Id"
               placeholder="二级分类"
               clearable
               :disabled="!listCategoryL1Id"
               style="width: 140px"
               @change="handleListCategoryL2Change"
             >
               <el-option v-for="c in categoryL2Options" :key="c.id" :label="c.name" :value="c.id" />
             </el-select>
             <el-select
               v-model="listSort"
               placeholder="排序"
               clearable
               style="width: 140px"
               @change="handleListSortChange"
             >
               <el-option label="时间降序" value="transaction_time_desc" />
               <el-option label="时间升序" value="transaction_time_asc" />
               <el-option label="金额降序" value="amount_desc" />
               <el-option label="金额升序" value="amount_asc" />
             </el-select>
           </div>
       </div>
       
       <el-table :data="transactions" v-loading="listLoading" style="width: 100%">
          <el-table-column prop="transaction_time" label="时间" width="160">
             <template #default="{ row }">
               {{ dayjs(row.transaction_time).format('YYYY-MM-DD HH:mm') }}
             </template>
          </el-table-column>
          <el-table-column label="分类" width="140">
             <template #default="{ row }">
               <span class="text-gray-500">{{ row.category_l1?.name }}</span>
               <span class="mx-1">/</span>
               <span class="font-bold text-gray-700">{{ row.category_l2?.name }}</span>
             </template>
          </el-table-column>
          <el-table-column prop="amount" label="金额" width="120">
             <template #default="{ row }">
               <span class="font-bold text-primary">¥{{ row.amount.toFixed(2) }}</span>
             </template>
          </el-table-column>
          <el-table-column prop="merchant" label="商家/商品" show-overflow-tooltip />
          <el-table-column prop="remark" label="备注" show-overflow-tooltip />
       </el-table>
       
       <div class="flex justify-end mt-4">
          <el-pagination
            background
            layout="prev, pager, next"
            :total="totalTransactions"
            :page-size="pageSize"
            v-model:current-page="currentPage"
            @current-change="handlePageChange"
          />
       </div>
    </div>
  </div>
</template>

<style scoped>
.chart {
  height: 100%;
  width: 100%;
}
</style>
