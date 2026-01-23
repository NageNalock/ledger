import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import request from '@/api/request'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'

export function useProjectDetail() {
  const route = useRoute()
  const router = useRouter()

  const projectId = computed(() => String(route.params.id || ''))
  const project = ref<any>(null)

  const categories = ref<any[]>([])
  const trendData = ref<any[]>([])

  const trendLoading = ref(false)
  const listLoading = ref(false)
  const budgetSaving = ref(false)

  const granularity = ref<'day' | 'month'>('day')
  const trendPointDate = ref('')

  const budgetEnabled = ref(false)
  const budgetLimit = ref<number>(0)

  const summaryAll = ref(0)
  const summaryFiltered = ref(0)

  const transactions = ref<any[]>([])
  const total = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(20)

  const modalVisible = ref(false)
  const editData = ref<any | undefined>(undefined)

  const filters = reactive({
    quickFilter: 'all',
    dateRange: [] as string[],
    categoryL1Id: '',
    categoryL2Id: '',
    source: '',
    sort: 'transaction_time_desc',
  })

  const pageTitle = computed(() => project.value?.name || '项目详情')

  const categoryL2Options = computed(() => {
    const l1 = categories.value.find((c: any) => String(c.id) === String(filters.categoryL1Id))
    return l1?.children || []
  })

  const activeDateRange = computed<[string, string]>(() => {
    if (filters.dateRange && filters.dateRange.length === 2) {
      return [filters.dateRange[0], filters.dateRange[1]]
    }

    const now = dayjs()
    if (filters.quickFilter === 'week') return [now.subtract(7, 'day').format('YYYY-MM-DD'), now.format('YYYY-MM-DD')]
    if (filters.quickFilter === 'month') return [now.subtract(1, 'month').format('YYYY-MM-DD'), now.format('YYYY-MM-DD')]
    if (filters.quickFilter === 'year') return [now.subtract(1, 'year').add(1, 'day').format('YYYY-MM-DD'), now.format('YYYY-MM-DD')]
    return ['2020-01-01', now.endOf('year').format('YYYY-MM-DD')]
  })

  const listDateRange = computed<[string, string]>(() => {
    let [start, end] = activeDateRange.value
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
    return [start, end]
  })

  const lineOption = computed(() => ({
    color: ['#FF8A65'],
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(255, 255, 255, 0.9)',
      borderColor: '#eee',
      textStyle: { color: '#333' },
      formatter: (params: any) => {
        const p = params?.[0]
        if (!p) return ''
        const title = p.axisValue
        return `<div class="font-bold mb-1">${title}</div><div class="flex justify-between w-40 text-xs"><span>${p.marker}${p.seriesName}</span><span class="font-bold">¥${Number(p.value).toFixed(2)}</span></div>`
      },
    },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: trendData.value.map((i: any) => i.date),
      axisLine: { lineStyle: { color: '#ddd' } },
      axisLabel: { color: '#666' },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#f5f5f5' } },
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
          opacity: 0.18,
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: '#FF8A65' },
              { offset: 1, color: '#fff' },
            ],
          },
        },
      },
    ],
  }))

  const fetchProject = async () => {
    const res: any = await request.get(`/projects/${projectId.value}`)
    project.value = res
    budgetEnabled.value = !!res?.budget_enabled
    budgetLimit.value = Number(res?.budget_limit || 0)
  }

  const fetchCategories = async () => {
    const res: any = await request.get('/categories')
    categories.value = res || []
  }

  const fetchSummaryAll = async () => {
    const res: any = await request.get(`/projects/${projectId.value}/summary`)
    summaryAll.value = Number(res?.total_amount || 0)
  }

  const fetchSummaryFiltered = async () => {
    const [start, end] = listDateRange.value
    const params: any = { start_date: start, end_date: end }
    if (filters.categoryL1Id) params.category_l1_id = filters.categoryL1Id
    if (filters.categoryL2Id) params.category_l2_id = filters.categoryL2Id
    if (filters.source) params.source = filters.source
    const res: any = await request.get(`/projects/${projectId.value}/summary`, { params })
    summaryFiltered.value = Number(res?.total_amount || 0)
  }

  const fetchTrend = async () => {
    trendLoading.value = true
    try {
      const [start, end] = activeDateRange.value
      const res: any = await request.get('/reports/trend', {
        params: { start_date: start, end_date: end, type: granularity.value, project_id: projectId.value },
      })
      trendData.value = res || []
    } finally {
      trendLoading.value = false
    }
  }

  const fetchTransactions = async () => {
    listLoading.value = true
    try {
      const [start, end] = listDateRange.value
      const params: any = {
        start_date: start,
        end_date: end,
        page: currentPage.value,
        page_size: pageSize.value,
        project_id: projectId.value,
      }
      if (filters.categoryL1Id) params.category_l1_id = filters.categoryL1Id
      if (filters.categoryL2Id) params.category_l2_id = filters.categoryL2Id
      if (filters.source) params.source = filters.source
      const [sortBy, sortOrder] = String(filters.sort || '').split('_')
      if (sortBy && sortOrder) {
        params.sort_by = sortBy
        params.sort_order = sortOrder
      }
      const res: any = await request.get('/transactions', { params })
      transactions.value = res.data
      total.value = res.total
    } finally {
      listLoading.value = false
    }
  }

  const refreshAll = async () => {
    await fetchSummaryAll()
    await fetchTrend()
    await fetchTransactions()
    await fetchSummaryFiltered()
  }

  const handleQuickFilterChange = async (v: string) => {
    filters.quickFilter = v
    filters.dateRange = []
    trendPointDate.value = ''
    currentPage.value = 1
    await refreshAll()
  }

  const handleDateRangeChange = async () => {
    filters.quickFilter = 'custom'
    trendPointDate.value = ''
    currentPage.value = 1
    await refreshAll()
  }

  const handleCategoryL1Change = async () => {
    if (!categoryL2Options.value.find((c: any) => String(c.id) === String(filters.categoryL2Id))) {
      filters.categoryL2Id = ''
    }
    currentPage.value = 1
    await fetchTransactions()
    await fetchSummaryFiltered()
  }

  const handleCategoryL2Change = async () => {
    currentPage.value = 1
    await fetchTransactions()
    await fetchSummaryFiltered()
  }

  const handleSourceChange = async () => {
    currentPage.value = 1
    await fetchTransactions()
    await fetchSummaryFiltered()
  }

  const handleSortChange = async () => {
    currentPage.value = 1
    await fetchTransactions()
  }

  const handlePageChange = async (page: number) => {
    currentPage.value = page
    await fetchTransactions()
  }

  const handlePageSizeUpdate = async (v: number) => {
    pageSize.value = v
    await handlePageChange(1)
  }

  const handleTrendPointClick = async (params: any) => {
    if (params.seriesType !== 'line') return
    trendPointDate.value = params.name || params.axisValue
    currentPage.value = 1
    await fetchTransactions()
    await fetchSummaryFiltered()
  }

  const handleClearTrendPoint = async () => {
    trendPointDate.value = ''
    currentPage.value = 1
    await fetchTransactions()
    await fetchSummaryFiltered()
  }

  const handleGranularityUpdate = async (v: 'day' | 'month') => {
    granularity.value = v
    trendPointDate.value = ''
    await fetchTrend()
    await fetchTransactions()
    await fetchSummaryFiltered()
  }

  const handleSaveBudget = async () => {
    if (!project.value) return
    budgetSaving.value = true
    try {
      const payload: any = {
        name: project.value.name,
        description: project.value.description,
        budget_enabled: budgetEnabled.value,
        budget_limit: budgetEnabled.value ? Number(budgetLimit.value || 0) : 0,
      }
      const res: any = await request.put(`/projects/${projectId.value}`, payload)
      project.value = res
      ElMessage.success('预算设置已保存')
      await fetchSummaryAll()
    } finally {
      budgetSaving.value = false
    }
  }

  const handleAdd = () => {
    editData.value = undefined
    modalVisible.value = true
  }

  const handleEdit = (row: any) => {
    editData.value = JSON.parse(JSON.stringify(row))
    modalVisible.value = true
  }

  const handleDelete = async (row: any) => {
    try {
      await ElMessageBox.confirm('确定删除这条消费记录吗？', '提示', { type: 'warning' })
      await request.delete(`/transactions/${row.id}`)
      ElMessage.success('删除成功')
      await refreshAll()
    } catch (e) {
      return
    }
  }

  onMounted(async () => {
    try {
      await Promise.all([fetchProject(), fetchCategories()])
      await refreshAll()
    } catch (e) {
      router.push('/projects')
    }
  })

  return {
    router,
    projectId,
    pageTitle,
    categories,
    categoryL2Options,
    filters,
    trendPointDate,
    summaryAll,
    summaryFiltered,
    trendLoading,
    lineOption,
    granularity,
    budgetSaving,
    budgetEnabled,
    budgetLimit,
    transactions,
    listLoading,
    total,
    currentPage,
    pageSize,
    modalVisible,
    editData,
    refreshAll,
    handleQuickFilterChange,
    handleDateRangeChange,
    handleCategoryL1Change,
    handleCategoryL2Change,
    handleSourceChange,
    handleSortChange,
    handlePageChange,
    handlePageSizeUpdate,
    handleTrendPointClick,
    handleClearTrendPoint,
    handleGranularityUpdate,
    handleSaveBudget,
    handleAdd,
    handleEdit,
    handleDelete,
  }
}

