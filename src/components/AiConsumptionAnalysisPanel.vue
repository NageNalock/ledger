<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import request from '@/api/request'
import { Sparkles, Clock, Coins, CircleAlert, CircleCheck, CircleHelp, CalendarPlus } from 'lucide-vue-next'
import { formatMoney } from '@/utils/format'
import AiBudgetImportDialog from '@/components/AiBudgetImportDialog.vue'


type NameAmount = { name: string; amount: number }
type NameAmountRatio = { name: string; amount: number; ratio: number }

type ConsumptionAnalysisResponse = {
  analysis_id: string
  created_at: string
  language: 'zh-CN'
  status?: 'pending' | 'running' | 'completed' | 'failed'
  started_at?: string | null
  finished_at?: string | null
  duration_ms?: number | null
  transaction_count?: number
  error_msg?: string
  range: {
    start_date: string
    end_date: string
    compare_start_date?: string
    compare_end_date?: string
  }
  kpis: {
    total_amount: number
    avg_per_day: number
    delta_amount?: number
    delta_ratio?: number
  }
  trend: Array<{ date: string; amount: number }>
  transactions: Array<{
    transaction_id: string
    time: string
    merchant: string
    amount: number
    category_l1: string
    category_l2: string
    remark: string
  }>
  evidence: {
    top_categories_l1: NameAmountRatio[]
    top_merchants: NameAmount[]
  }
  rigid: {
    amount: number
    ratio: number
    breakdown_l1: NameAmount[]
  }
  non_rigid: {
    amount: number
    ratio: number
    breakdown_l1: NameAmount[]
  }
  change_insights: {
    top_increase?: { name: string; amount: number }
    top_decrease?: { name: string; amount: number }
  }
  transaction_labels: Array<{
    transaction_id: string
    label: 'rigid' | 'non_rigid'
    assessment?: 'reasonable' | 'suspicious' | 'wasteful'
    reason: string
    confidence?: number
  }>
  removable_non_essential_items: Array<{
    title: string
    category_l1?: string
    merchant?: string
    amount?: number
    frequency_hint?: string
    reason: string
    action_suggestion: string
    estimated_saving?: { min?: number; max?: number }
    confidence?: 'low' | 'medium' | 'high'
  }>
  budget_suggestion: {
    next_period_total?: { min: number; max: number }
    next_period_rigid?: { min: number; max: number }
    next_period_non_rigid?: { min: number; max: number }
    project_budget?: {
      enabled: boolean
      limit: number
      status?: 'ok' | 'exceed'
      recommended_limit?: number
    }
  }
  ai_summary: {
    bullets: string[]
    caveats?: string[]
  }
}

type AnalysisStatus = 'idle' | 'pending' | 'running' | 'completed' | 'failed'

const props = defineProps<{
  startDate: string
  endDate: string
  trendType: 'day'
  projectId: string
  userRequirement: string
}>()

const loading = ref(false)
const bootstrapping = ref(false)
const data = ref<ConsumptionAnalysisResponse | null>(null)
const errorText = ref('')
const stale = ref(false)
const analysisStatus = ref<AnalysisStatus>('idle')
const analysisId = ref('')
const polling = ref(false)
let pollTimer: number | undefined

const historyOpen = ref(false)
const historyLoading = ref(false)
const historyError = ref('')
const historyItems = ref<
  Array<{
    analysis_id: string
    created_at: string
    status: AnalysisStatus
    error_msg?: string
    duration_ms?: number | null
    transaction_count: number
    range: { start_date: string; end_date: string }
    kpis: { total_amount: number; avg_per_day: number }
  }>
>([])

const budgetImportOpen = ref(false)
const budgetImportMonth = computed(() => {
  const endDate = String(data.value?.range?.end_date || '')
  if (endDate.length >= 7) return endDate.slice(0, 7)
  return new Date().toISOString().slice(0, 7)
})

const openBudgetImport = () => {
  if (!data.value?.analysis_id) return
  budgetImportOpen.value = true
}

const labelsOpen = ref(false)
const labelsFilter = ref<'all' | 'rigid' | 'non_rigid' | 'wasteful' | 'suspicious'>('all')

const canAnalyze = computed(() => !!props.startDate && !!props.endDate)
const analysisInProgress = computed(() => analysisStatus.value === 'pending' || analysisStatus.value === 'running')
const analyzeButtonLabel = computed(() => (analysisInProgress.value ? '分析中' : data.value ? '重新分析' : '开始分析'))
const shouldForceRegenerate = computed(() => !!data.value || analysisStatus.value === 'failed')
const statusText = computed(() => {
  if (analysisStatus.value === 'pending') return '任务已提交，正在排队'
  if (analysisStatus.value === 'running') return 'AI 正在分析，请稍候'
  if (analysisStatus.value === 'failed') return '分析失败，请查看原因后重试'
  return ''
})

const formatRatio = (v: number) => `${Math.round(Number(v || 0) * 100)}%`
const formatMs = (v?: number | null) => {
  if (!v && v !== 0) return '-'
  if (v < 1000) return `${v}ms`
  return `${(v / 1000).toFixed(1)}s`
}

const stopPolling = () => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
  polling.value = false
}

const fetchStatus = async (id: string) => {
  try {
    const res: any = await request.get(`/ai/consumption-analysis/${id}`, { timeout: 20000 })
    if (res?.status && res.status !== 'completed') {
      analysisStatus.value = res.status
      analysisId.value = res.analysis_id || id
      if (res.status === 'failed') {
        errorText.value = String(res.error_msg || 'AI 分析失败，请稍后重试')
        stopPolling()
      }
      return
    }
    data.value = res
    analysisStatus.value = 'completed'
    analysisId.value = res?.analysis_id || id
    stale.value = false
    errorText.value = ''
    stopPolling()
  } catch (e: any) {
    errorText.value = String(e?.response?.data?.error || 'AI 状态获取失败')
  }
}

const startPolling = (id: string) => {
  stopPolling()
  polling.value = true
  pollTimer = window.setInterval(() => {
    fetchStatus(id)
  }, 5000)
  fetchStatus(id)
}

const fetchAnalysis = async () => {
  if (!canAnalyze.value) return
  loading.value = true
  errorText.value = ''
  try {
    const res: any = await request.post(
      '/ai/consumption-analysis',
      {
        start_date: props.startDate,
        end_date: props.endDate,
        trend_type: props.trendType,
        project_id: props.projectId,
        compare: 'none',
        force_regenerate: shouldForceRegenerate.value,
        user_requirement: props.userRequirement,
      },
      { timeout: 10000 }
    )
    if (res?.status && res.status !== 'completed') {
      analysisStatus.value = res.status
      analysisId.value = res.analysis_id
      data.value = null
      stale.value = false
      startPolling(res.analysis_id)
    } else {
      data.value = res
      analysisStatus.value = 'completed'
      analysisId.value = res?.analysis_id || ''
      stale.value = false
      stopPolling()
    }
  } catch (e: any) {
    const msg = e?.response?.data?.error || (e?.code === 'ECONNABORTED' ? 'AI 分析请求超时，请稍后重试' : e?.code === 'ERR_CANCELED' ? 'AI 分析请求被取消' : 'AI 分析失败，请稍后重试')
    errorText.value = String(msg)
  } finally {
    loading.value = false
  }
}

const fetchHistory = async () => {
  historyLoading.value = true
  historyError.value = ''
  try {
    const res: any = await request.get('/ai/consumption-analysis/history', {
      params: { limit: 20, offset: 0 },
      timeout: 30000,
    })
    historyItems.value = Array.isArray(res?.items) ? res.items : []
  } catch (e: any) {
    historyError.value = String(e?.response?.data?.error || '历史记录加载失败')
  } finally {
    historyLoading.value = false
  }
}

const loadLatestAnalysis = async () => {

  if (loading.value || analysisInProgress.value) return
  const tryParamsList: Array<Record<string, any>> = []
  if (props.projectId) {
    tryParamsList.push({ limit: 1, offset: 0, project_id: props.projectId })
    if (props.startDate && props.endDate) {
      tryParamsList.unshift({ limit: 1, offset: 0, project_id: props.projectId, start_date: props.startDate, end_date: props.endDate })
    }
  } else if (props.startDate && props.endDate) {
    tryParamsList.push({ limit: 1, offset: 0, start_date: props.startDate, end_date: props.endDate })
  }
  tryParamsList.push({ limit: 1, offset: 0 })

  for (const params of tryParamsList) {
    try {
      const res: any = await request.get('/ai/consumption-analysis/history', { params, timeout: 20000 })
      const items = Array.isArray(res?.items) ? res.items : []
      const first = items[0]
      if (first?.analysis_id) {
        await loadHistoryDetail(String(first.analysis_id))
        return
      }
    } catch {
      continue
    }
  }
}

const openHistory = async () => {
  historyOpen.value = true
  await fetchHistory()
}

const loadHistoryDetail = async (id: string) => {
  loading.value = true
  errorText.value = ''
  try {
    const res: any = await request.get(`/ai/consumption-analysis/${id}`, { timeout: 20000 })
    if (res?.status && res.status !== 'completed') {
      analysisStatus.value = res.status
      analysisId.value = res.analysis_id || id
      data.value = null
      if (res.status === 'failed') {
        errorText.value = String(res.error_msg || 'AI 分析失败，请稍后重试')
        stopPolling()
      } else {
        startPolling(id)
      }
    } else {
      data.value = res
      analysisStatus.value = 'completed'
      analysisId.value = res?.analysis_id || id
      stale.value = false
      stopPolling()
    }
  } catch (e: any) {
    errorText.value = String(e?.response?.data?.error || '历史记录加载失败')
  } finally {
    historyOpen.value = false
    loading.value = false
  }
}

const statusLabel = (status: AnalysisStatus) => {
  if (status === 'pending') return '排队中'
  if (status === 'running') return '分析中'
  if (status === 'completed') return '已完成'
  if (status === 'failed') return '失败'
  return '未开始'
}

const statusTagType = (status: AnalysisStatus) => {
  if (status === 'completed') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running' || status === 'pending') return 'warning'
  return 'info'
}

const labelRows = computed(() => {
  const d = data.value
  if (!d) return [] as Array<{ time: string; merchant: string; category: string; amount: number; label: string; assessment: string; reason: string; confidence?: number }>
  const labels = new Map(d.transaction_labels?.map((l) => [l.transaction_id, l]) || [])
  const rows = (d.transactions || []).map((t) => {
    const l = labels.get(t.transaction_id)
    return {
      time: t.time,
      merchant: t.merchant,
      category: `${t.category_l1}/${t.category_l2}`,
      amount: t.amount,
      label: l?.label || 'non_rigid',
      assessment: l?.assessment || 'reasonable',
      reason: l?.reason || '',
      confidence: l?.confidence,
    }
  })
  if (labelsFilter.value === 'all') return rows
  if (labelsFilter.value === 'rigid') return rows.filter((r) => r.label === 'rigid')
  if (labelsFilter.value === 'non_rigid') return rows.filter((r) => r.label === 'non_rigid')
  if (labelsFilter.value === 'wasteful') return rows.filter((r) => r.assessment === 'wasteful')
  if (labelsFilter.value === 'suspicious') return rows.filter((r) => r.assessment === 'suspicious')
  return rows
})

watch(
  () => [props.startDate, props.endDate, props.projectId, props.userRequirement],
  () => {
    if (data.value) stale.value = true
  }
)

onMounted(() => {

  bootstrapping.value = true
  loadLatestAnalysis().finally(() => {
    bootstrapping.value = false
  })
})

const rigidPercent = computed(() => Math.min(100, Math.max(0, Math.round((data.value?.rigid?.ratio || 0) * 100))))
const nonRigidPercent = computed(() => Math.min(100, Math.max(0, Math.round((data.value?.non_rigid?.ratio || 0) * 100))))

defineExpose({
  runAnalysis: fetchAnalysis,
  canAnalyze,
  analyzeButtonLabel,
  analysisInProgress,
  loading,
  statusText,
})

</script>

<template>
  <div class="bg-white p-6 rounded-3xl shadow-sm border border-gray-100 transition-all hover:shadow-md">
    <div class="flex flex-wrap items-center justify-between gap-4 mb-6">
      <div class="flex items-center gap-3">
        <div class="p-2 bg-primary/10 rounded-2xl text-primary">
          <Sparkles class="w-6 h-6" />
        </div>
        <div>
          <h3 class="text-xl font-bold text-gray-800 flex items-center gap-2">
            AI 消费顾问
            <span v-if="stale" class="text-xs font-bold bg-amber-100 text-amber-700 px-2 py-0.5 rounded-full">数据已更新</span>
          </h3>
          <div class="text-sm text-gray-400 font-medium">智能分析你的消费习惯，提供贴心预算建议</div>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <el-button round class="!rounded-2xl !border-gray-200 hover:!bg-gray-50" :disabled="loading" @click="openHistory">
          <template #icon><Clock class="w-4 h-4" /></template>
          历史记录
        </el-button>
      </div>
    </div>

    <div
      v-if="!canAnalyze && !loading && !data && !analysisInProgress && analysisStatus !== 'failed' && !bootstrapping"
      class="rounded-2xl border border-dashed border-gray-300 bg-gray-50 p-8 text-center"
    >
      <div class="inline-block p-4 bg-white rounded-full shadow-sm mb-3">
        <Clock class="w-8 h-8 text-gray-300" />
      </div>
      <div class="text-gray-500 font-bold">请先选择时间范围，AI 才能为你分析哦 ~</div>
    </div>

    <div v-else-if="errorText" class="rounded-2xl bg-red-50 border border-red-100 p-4 flex gap-3 text-red-700">
      <CircleAlert class="w-5 h-5 shrink-0" />
      <div class="text-sm font-medium">
        {{ errorText }}
        <div class="mt-1 text-xs opacity-80">如提示配置缺失，请在右上角设置中填写 Ark Key/Endpoint。</div>
      </div>
    </div>

    <div v-else class="space-y-6">
      <div v-if="loading" class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div v-for="i in 3" :key="i" class="h-32 bg-gray-50 rounded-3xl animate-pulse border border-gray-100"></div>
      </div>

      <div v-else-if="bootstrapping" class="rounded-3xl border border-gray-100 bg-gray-50 p-6">
        <div class="text-sm font-bold text-gray-600">正在加载最近一次分析结果…</div>
        <div class="mt-4 grid grid-cols-1 md:grid-cols-3 gap-6">
          <div v-for="i in 3" :key="i" class="h-32 bg-white rounded-3xl animate-pulse border border-gray-100"></div>
        </div>
      </div>

      <div v-else-if="analysisInProgress" class="rounded-3xl border border-purple-100 bg-gradient-to-r from-purple-50 to-pink-50 p-6">
        <div class="flex items-center justify-between gap-4">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-white rounded-2xl shadow-sm text-purple-500">
              <CircleHelp class="w-5 h-5" />
            </div>
            <div>
              <div class="text-sm font-bold text-gray-700">{{ statusText }}</div>
              <div v-if="analysisId" class="text-xs text-gray-400 font-medium mt-1">任务编号 {{ analysisId }}</div>
            </div>
          </div>
          <el-tag :type="statusTagType(analysisStatus)" effect="light" round size="small" class="!font-bold">
            {{ statusLabel(analysisStatus) }}
          </el-tag>
        </div>
        <div class="mt-3 text-xs text-gray-500 font-medium">你可以继续浏览，稍后在历史记录查看结果。</div>
      </div>

      <template v-else-if="data">
        <!-- 1. KPI Cards -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="bg-gradient-to-br from-blue-50 to-blue-100/50 p-5 rounded-3xl border border-blue-100">
            <div class="text-xs text-blue-400 font-bold uppercase tracking-wider mb-1">总支出</div>
            <div class="text-2xl font-black text-blue-900">{{ formatMoney(data.kpis.total_amount) }}</div>
            <div class="mt-2 text-xs font-medium text-blue-600 bg-white/60 inline-block px-2 py-1 rounded-lg">
              日均 {{ formatMoney(data.kpis.avg_per_day) }}
            </div>
          </div>

          <div class="bg-gradient-to-br from-orange-50 to-orange-100/50 p-5 rounded-3xl border border-orange-100">
            <div class="text-xs text-orange-400 font-bold uppercase tracking-wider mb-1">下期预算建议</div>
            <div class="text-lg font-black text-orange-900 mt-1" v-if="data.budget_suggestion.next_period_total">
              {{ formatMoney(data.budget_suggestion.next_period_total.min) }} ~ {{ formatMoney(data.budget_suggestion.next_period_total.max) }}
            </div>
            <div v-else class="text-sm font-bold text-gray-400 mt-1">暂无建议</div>
            <div class="mt-2 flex gap-1 text-[10px] font-bold text-orange-700/70" v-if="data.budget_suggestion.next_period_rigid">
              <span>刚性:{{ formatMoney(data.budget_suggestion.next_period_rigid.min) }}</span>
              <span>·</span>
              <span>非刚性:{{ formatMoney(data.budget_suggestion.next_period_non_rigid?.min || 0) }}</span>
            </div>
          </div>
        </div>

        <div class="bg-white rounded-3xl border border-gray-100 p-4 shadow-sm">
          <div class="flex flex-wrap items-center gap-3 text-xs font-bold text-gray-500">
            <span class="inline-flex items-center gap-2 bg-gray-50 border border-gray-100 px-3 py-2 rounded-2xl">
              <Clock class="w-4 h-4 text-gray-400" />
              生成 {{ String(data.created_at).substring(0, 19).replace('T', ' ') }}
            </span>
            <span class="inline-flex items-center gap-2 bg-gray-50 border border-gray-100 px-3 py-2 rounded-2xl">
              <Clock class="w-4 h-4 text-gray-400" />
              耗时 {{ formatMs(data.duration_ms ?? null) }}
            </span>
            <span class="inline-flex items-center gap-2 bg-gray-50 border border-gray-100 px-3 py-2 rounded-2xl">
              <Coins class="w-4 h-4 text-amber-400" />
              流水 {{ data.transaction_count ?? 0 }} 笔
            </span>
          </div>
        </div>

        <!-- 2. AI Summary & Insights -->
        <div class="bg-white rounded-3xl border border-gray-100 p-6 shadow-sm relative overflow-hidden">
          <div class="absolute top-0 right-0 w-32 h-32 bg-purple-50 rounded-full blur-3xl -z-10 translate-x-10 -translate-y-10"></div>
          <h4 class="text-lg font-bold text-gray-800 mb-4 flex items-center gap-2">
            <Sparkles class="w-5 h-5 text-purple-500" />
            消费洞察
          </h4>
          
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
             <!-- Bullets -->
            <div class="space-y-3">
              <div v-for="(b, idx) in data.ai_summary.bullets" :key="idx" class="flex gap-3 text-sm text-gray-700 bg-gray-50/50 p-3 rounded-2xl">
                <span class="mt-1 h-2 w-2 rounded-full bg-purple-400 shrink-0 shadow-sm"></span>
                <span class="leading-relaxed font-medium">{{ b }}</span>
              </div>
              <div v-if="data.ai_summary.caveats && data.ai_summary.caveats.length" class="mt-4 p-4 bg-amber-50 rounded-2xl border border-amber-100">
                <div class="text-xs font-bold text-amber-500 mb-2 flex items-center gap-1">
                  <CircleAlert class="w-3 h-3" /> 注意事项
                </div>
                <div v-for="(c, idx) in data.ai_summary.caveats" :key="idx" class="text-xs text-amber-800 font-medium mb-1 last:mb-0">
                  • {{ c }}
                </div>
              </div>
            </div>

            <!-- Highlights -->
            <div class="space-y-3">
              <div v-if="data.change_insights.top_increase" class="p-4 bg-red-50/50 rounded-2xl border border-red-100 flex items-center justify-between">
                <div>
                  <div class="text-xs font-bold text-red-400 mb-1">支出增长最快</div>
                  <div class="font-bold text-gray-800">{{ data.change_insights.top_increase.name }}</div>
                </div>
                <div class="text-lg font-black text-red-500">{{ formatMoney(data.change_insights.top_increase.amount) }}</div>
              </div>
              
              <div v-if="data.change_insights.top_decrease" class="p-4 bg-green-50/50 rounded-2xl border border-green-100 flex items-center justify-between">
                <div>
                  <div class="text-xs font-bold text-green-500 mb-1">支出下降最多</div>
                  <div class="font-bold text-gray-800">{{ data.change_insights.top_decrease.name }}</div>
                </div>
                <div class="text-lg font-black text-green-600">{{ formatMoney(data.change_insights.top_decrease.amount) }}</div>
              </div>

              <!-- Rigid/Non-Rigid Progress -->
              <div class="p-4 bg-gray-50 rounded-2xl border border-gray-100">
                <div class="flex justify-between text-xs font-bold text-gray-500 mb-2">
                  <span>刚性支出 ({{ formatRatio(data.rigid.ratio) }})</span>
                  <span>非刚性支出 ({{ formatRatio(data.non_rigid.ratio) }})</span>
                </div>
                <div class="h-3 bg-gray-200 rounded-full overflow-hidden flex">
                  <div class="h-full bg-emerald-400" :style="{ width: rigidPercent + '%' }"></div>
                  <div class="h-full bg-amber-400" :style="{ width: nonRigidPercent + '%' }"></div>
                </div>
                <div class="flex justify-between text-xs font-bold mt-2">
                  <span class="text-emerald-600">{{ formatMoney(data.rigid.amount) }}</span>
                  <span class="text-amber-600">{{ formatMoney(data.non_rigid.amount) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 3. Removable Items (Optimization) -->
        <div class="bg-white rounded-3xl border border-gray-100 p-6 shadow-sm" v-if="data.removable_non_essential_items?.length">
          <h4 class="text-lg font-bold text-gray-800 mb-4 flex items-center gap-2">
            <Coins class="w-5 h-5 text-amber-500" />
            优化建议
          </h4>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div v-for="(it, idx) in data.removable_non_essential_items.slice(0, 6)" :key="idx" class="p-4 bg-gray-50 rounded-2xl hover:bg-white hover:shadow-md hover:border-amber-100 border border-transparent transition-all">
              <div class="flex justify-between items-start mb-2">
                <div class="font-bold text-gray-800 text-sm">{{ it.title }}</div>
                <el-tag size="small" type="info" effect="plain" class="!rounded-lg" v-if="it.amount">{{ formatMoney(it.amount || 0) }}</el-tag>
              </div>
              <div class="text-xs text-gray-500 font-medium mb-2 leading-relaxed">{{ it.reason }}</div>
              <div class="text-xs text-purple-600 font-bold bg-purple-50 px-2 py-1 rounded-lg inline-block">💡 {{ it.action_suggestion }}</div>
            </div>
          </div>
        </div>

        <!-- 4. Actions -->
        <div class="flex flex-wrap justify-center gap-3 pt-4">
          <el-button
            size="large"
            round
            class="!px-8 !h-12 !text-base !font-bold !border-gray-200 shadow-sm hover:!text-primary hover:!border-primary"
            @click="labelsOpen = true"
          >
            查看逐笔分析详情
          </el-button>

          <el-button size="large" round type="primary" color="#FF8A65" class="!px-8 !h-12 !text-base !font-black" @click="openBudgetImport">
            <template #icon><CalendarPlus class="w-5 h-5" /></template>
            带入到预算
          </el-button>
        </div>

        <!-- Evidence Collapse -->
        <div class="mt-4">
          <el-collapse class="!border-0">
            <el-collapse-item title="查看数据来源" name="1">
               <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 p-4 bg-gray-50 rounded-2xl">
                  <div>
                    <div class="text-xs font-bold text-gray-400 uppercase mb-2">Top 类目</div>
                    <div v-for="c in data.evidence.top_categories_l1" :key="c.name" class="flex justify-between text-xs py-1 text-gray-600 font-medium border-b border-gray-100 last:border-0">
                      <span>{{ c.name }}</span>
                      <span>{{ formatMoney(c.amount) }}</span>
                    </div>
                  </div>
                  <div>
                    <div class="text-xs font-bold text-gray-400 uppercase mb-2">Top 商户</div>
                    <div v-for="m in data.evidence.top_merchants" :key="m.name" class="flex justify-between text-xs py-1 text-gray-600 font-medium border-b border-gray-100 last:border-0">
                      <span>{{ m.name }}</span>
                      <span>{{ formatMoney(m.amount) }}</span>
                    </div>
                  </div>
               </div>
            </el-collapse-item>
          </el-collapse>
        </div>
      </template>
    </div>
  </div>

  <AiBudgetImportDialog v-if="data" v-model="budgetImportOpen" :analysis-id="data.analysis_id" :default-month="budgetImportMonth" />

  <!-- History Drawer -->
  <el-drawer v-model="historyOpen" size="860px" title="分析历史" class="!rounded-l-3xl">
    <div v-loading="historyLoading" class="h-full overflow-auto p-4">
      <div v-if="historyError" class="text-red-500 text-sm font-bold p-4">{{ historyError }}</div>
      <el-table v-else :data="historyItems" style="width: 100%" :header-cell-style="{ background: '#f9fafb', color: '#6b7280' }" class="rounded-2xl overflow-hidden border border-gray-100">
        <el-table-column label="生成时间" width="170">
          <template #default="{ row }">
            <div class="text-xs font-bold text-gray-600">{{ String(row.created_at).substring(0, 19).replace('T', ' ') }}</div>
          </template>
        </el-table-column>
        <el-table-column label="范围" min-width="190">
          <template #default="{ row }">
            <div class="text-xs font-bold text-gray-700">{{ row.range.start_date }} ~ {{ row.range.end_date }}</div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" effect="light" round size="small" class="!font-bold">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="110">
          <template #default="{ row }">
            <span class="text-xs font-bold text-gray-600">{{ formatMs(row.duration_ms ?? null) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="流水数" width="110">
          <template #default="{ row }">
            <span class="text-xs font-bold text-gray-600">{{ row.transaction_count }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110">
          <template #default="{ row }">
            <el-button size="small" round class="!font-bold" @click="loadHistoryDetail(row.analysis_id)">回看</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </el-drawer>

  <!-- Labels Detail Drawer -->
  <el-drawer v-model="labelsOpen" size="800px" title="逐笔合理性分析" class="!rounded-l-3xl">
    <div class="flex flex-col h-full">
      <div class="flex items-center gap-2 mb-4 px-1">
        <span class="text-sm font-bold text-gray-500">筛选:</span>
        <div class="flex gap-2">
           <el-radio-group v-model="labelsFilter" size="small" class="!flex-nowrap">
             <el-radio-button label="all">全部</el-radio-button>
             <el-radio-button label="rigid">刚性</el-radio-button>
             <el-radio-button label="non_rigid">非刚性</el-radio-button>
             <el-radio-button label="wasteful"><span class="text-red-500 font-bold">浪费</span></el-radio-button>
             <el-radio-button label="suspicious"><span class="text-amber-500 font-bold">存疑</span></el-radio-button>
           </el-radio-group>
        </div>
      </div>
      
      <div class="flex-1 overflow-hidden rounded-2xl border border-gray-100">
        <el-table :data="labelRows" style="width: 100%" height="100%" :header-cell-style="{ background: '#f9fafb', color: '#6b7280' }">
          <el-table-column prop="time" label="时间" width="160" />
          <el-table-column prop="merchant" label="商户" min-width="120" show-overflow-tooltip />
          <el-table-column prop="category" label="类目" min-width="120" show-overflow-tooltip />
          <el-table-column prop="amount" label="金额" width="100">
            <template #default="{ row }">
              <span class="font-bold text-gray-800">{{ formatMoney(row.amount) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="label" label="性质" width="90">
            <template #default="{ row }">
              <span v-if="row.label==='rigid'" class="text-xs font-bold text-emerald-600 bg-emerald-50 px-2 py-1 rounded-lg">刚性</span>
              <span v-else class="text-xs font-bold text-amber-600 bg-amber-50 px-2 py-1 rounded-lg">非刚性</span>
            </template>
          </el-table-column>
          <el-table-column prop="assessment" label="评价" width="90">
             <template #default="{ row }">
                <span v-if="row.assessment==='reasonable'" class="text-xs font-bold text-gray-400"><CircleCheck class="w-3 h-3 inline mr-1"/>合理</span>
                <span v-else-if="row.assessment==='wasteful'" class="text-xs font-black text-red-500 bg-red-50 px-2 py-1 rounded-lg">浪费</span>
                <span v-else-if="row.assessment==='suspicious'" class="text-xs font-black text-orange-500 bg-orange-50 px-2 py-1 rounded-lg">存疑</span>
                <span v-else class="text-xs text-gray-400">-</span>
             </template>
          </el-table-column>
          <el-table-column prop="reason" label="分析理由" min-width="200" show-overflow-tooltip />
        </el-table>
      </div>
    </div>
  </el-drawer>
</template>

<style scoped>
/* Custom Scrollbar for drawers */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: #e5e7eb;
  border-radius: 3px;
}
::-webkit-scrollbar-thumb:hover {
  background: #d1d5db;
}
</style>
