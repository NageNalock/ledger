<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import request from '@/api/request'
import { clamp01, formatMoney, formatPct } from '@/utils/format'
import { BarChart3, Bot, FileUp, PiggyBank, Sparkles, Store, Tags, TrendingUp } from 'lucide-vue-next'

const props = defineProps<{
  startDate: string
  endDate: string
  refreshKey?: number
}>()

const emit = defineEmits<{
  (e: 'add-transaction'): void
}>()

const router = useRouter()

type TrendItem = { date: string; amount: number; breakdown?: Record<string, number> }
type CategoryItem = { name: string; value: number; children?: CategoryItem[] }
type MerchantItem = { name: string; value: number }
type ActiveBudgetResp = {
  budget?: {
    month: string
    total_amount: number
    usage_ratio: number
    waterline?: string
  } | null
  spent_amount?: number
}

const overviewLoading = ref(false)
const overviewError = ref('')
const budgetLoading = ref(false)
const budgetError = ref('')

const totalAmount = ref(0)
const avgPerDay = ref(0)
const topCategory = ref<{ name: string; amount: number } | null>(null)
const topMerchant = ref<{ name: string; amount: number } | null>(null)

const budget = ref<ActiveBudgetResp['budget']>(null)
const spentAmount = ref(0)

const rangeText = computed(() => {
  const s = String(props.startDate || '').trim()
  const e = String(props.endDate || '').trim()
  if (!s || !e) return '—'
  return `${s} 至 ${e}`
})

const rangeDays = computed(() => {
  const s = dayjs(props.startDate)
  const e = dayjs(props.endDate)
  if (!s.isValid() || !e.isValid()) return 1
  const d = e.diff(s, 'day') + 1
  return d > 0 ? d : 1
})

const budgetMonth = computed(() => dayjs().format('YYYY-MM'))
const safeBudgetTotal = computed(() => Number(budget.value?.total_amount || 0) || 0)
const safeSpent = computed(() => Number(spentAmount.value || 0) || 0)
const rawBudgetRatio = computed(() => {
  const total = safeBudgetTotal.value
  if (total <= 0) return 0
  return safeSpent.value / total
})
const budgetRatio = computed(() => clamp01(rawBudgetRatio.value))

const budgetLevel = computed(() => {
  if (!budget.value || safeBudgetTotal.value <= 0) return '未设置'
  if (rawBudgetRatio.value > 1) return '超支'
  if (rawBudgetRatio.value >= 0.8) return '预警'
  return '正常'
})

const budgetTagType = computed(() => {
  if (!budget.value || safeBudgetTotal.value <= 0) return 'info'
  if (rawBudgetRatio.value > 1) return 'danger'
  if (rawBudgetRatio.value >= 0.8) return 'warning'
  return 'success'
})

const budgetBarClass = computed(() => {
  if (!budget.value || safeBudgetTotal.value <= 0) return 'bg-gray-200'
  if (rawBudgetRatio.value > 1) return 'bg-rose-500'
  if (rawBudgetRatio.value >= 0.8) return 'bg-primary'
  return 'bg-secondary'
})

const loadOverview = async () => {
  overviewLoading.value = true
  overviewError.value = ''
  try {
    const [trendRes, catRes, merchantRes] = await Promise.all([
      request.get('/reports/trend', {
        params: { start_date: props.startDate, end_date: props.endDate, type: 'day' },
        timeout: 20000,
      }),
      request.get('/reports/category', {
        params: { start_date: props.startDate, end_date: props.endDate },
        timeout: 20000,
      }),
      request.get('/reports/merchant', {
        params: { start_date: props.startDate, end_date: props.endDate, limit: 10 },
        timeout: 20000,
      }),
    ])

    const trend = Array.isArray(trendRes) ? (trendRes as TrendItem[]) : []
    const sum = trend.reduce((acc, x) => acc + (Number(x?.amount || 0) || 0), 0)
    totalAmount.value = Number(sum) || 0
    avgPerDay.value = rangeDays.value ? totalAmount.value / rangeDays.value : 0

    const categories = Array.isArray(catRes) ? (catRes as CategoryItem[]) : []
    const topCat = categories.length ? categories[0] : null
    topCategory.value = topCat
      ? { name: String(topCat.name || '').trim() || '—', amount: Number(topCat.value || 0) || 0 }
      : null

    const merchants = Array.isArray(merchantRes) ? (merchantRes as MerchantItem[]) : []
    const topM = merchants.length ? merchants[0] : null
    topMerchant.value = topM
      ? { name: String(topM.name || '').trim() || '—', amount: Number(topM.value || 0) || 0 }
      : null
  } catch (e: any) {
    overviewError.value = String(e?.response?.data?.error || '总览加载失败')
  } finally {
    overviewLoading.value = false
  }
}

const loadBudget = async () => {
  budgetLoading.value = true
  budgetError.value = ''
  try {
    const res: any = await request.get('/budgets/active', { params: { month: budgetMonth.value }, timeout: 20000 })
    budget.value = res?.budget || null
    spentAmount.value = Number(res?.spent_amount || 0)
  } catch (e: any) {
    budgetError.value = String(e?.response?.data?.error || '预算加载失败')
    budget.value = null
    spentAmount.value = 0
  } finally {
    budgetLoading.value = false
  }
}

watch(
  () => `${props.startDate}|${props.endDate}`,
  () => loadOverview()
)

watch(
  () => props.refreshKey,
  () => {
    loadOverview()
    loadBudget()
  }
)

onMounted(() => {
  loadOverview()
  loadBudget()
})

const goBudget = () => router.push('/budget')
const goImport = () => router.push('/import')
const goReports = () => router.push('/reports')
const goAI = () => router.push('/ai-advisor')
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-3xl border border-[#F1D6D0] bg-white p-5 shadow-soft" v-loading="overviewLoading">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="flex items-center gap-2">
          <div class="h-9 w-9 rounded-2xl bg-primary/10 ring-1 ring-primary/15 flex items-center justify-center text-primary">
            <Sparkles class="h-4 w-4" />
          </div>
          <div>
            <div class="text-sm font-black text-[#5A3B32]">数据总览</div>
            <div class="text-[11px] font-bold text-[#9A7A71]">口径：{{ rangeText }}</div>
          </div>
        </div>
        <div v-if="overviewError" class="text-xs font-bold text-rose-600">{{ overviewError }}</div>
      </div>

      <div class="mt-4 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <div class="rounded-2xl border border-[#F1D6D0] bg-[#FFF8F5] p-4">
          <div class="flex items-center justify-between">
            <div class="text-[11px] font-extrabold text-[#9A7A71]">总支出</div>
            <div class="h-9 w-9 rounded-2xl bg-primary/10 ring-1 ring-primary/15 flex items-center justify-center text-primary">
              <TrendingUp class="h-4 w-4" />
            </div>
          </div>
          <div class="mt-2 text-lg font-black text-[#5A3B32]">{{ formatMoney(totalAmount) }}</div>
          <div class="mt-1 text-[11px] font-bold text-[#8E6C63]">{{ rangeDays }} 天合计</div>
        </div>

        <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
          <div class="flex items-center justify-between">
            <div class="text-[11px] font-extrabold text-[#9A7A71]">日均</div>
            <div class="h-9 w-9 rounded-2xl bg-secondary/15 ring-1 ring-secondary/20 flex items-center justify-center text-secondary">
              <BarChart3 class="h-4 w-4" />
            </div>
          </div>
          <div class="mt-2 text-lg font-black text-[#5A3B32]">{{ formatMoney(avgPerDay) }}</div>
          <div class="mt-1 text-[11px] font-bold text-[#8E6C63]">平均每天花费</div>
        </div>

        <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
          <div class="flex items-center justify-between">
            <div class="text-[11px] font-extrabold text-[#9A7A71]">Top 一级分类</div>
            <div class="h-9 w-9 rounded-2xl bg-[#7CD6C9]/15 ring-1 ring-[#7CD6C9]/20 flex items-center justify-center text-secondary">
              <Tags class="h-4 w-4" />
            </div>
          </div>
          <div class="mt-2 text-sm font-black text-[#5A3B32]">{{ topCategory?.name || '—' }}</div>
          <div class="mt-1 text-[11px] font-bold text-[#8E6C63]">{{ topCategory ? formatMoney(topCategory.amount) : '—' }}</div>
        </div>

        <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
          <div class="flex items-center justify-between">
            <div class="text-[11px] font-extrabold text-[#9A7A71]">Top 商户</div>
            <div class="h-9 w-9 rounded-2xl bg-primary/10 ring-1 ring-primary/15 flex items-center justify-center text-primary">
              <Store class="h-4 w-4" />
            </div>
          </div>
          <div class="mt-2 text-sm font-black text-[#5A3B32]">{{ topMerchant?.name || '—' }}</div>
          <div class="mt-1 text-[11px] font-bold text-[#8E6C63]">{{ topMerchant ? formatMoney(topMerchant.amount) : '—' }}</div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="rounded-3xl border border-[#F1D6D0] bg-white p-5 shadow-soft lg:col-span-2" v-loading="budgetLoading">
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-2">
            <div class="h-9 w-9 rounded-2xl bg-primary/10 ring-1 ring-primary/15 flex items-center justify-center text-primary">
              <PiggyBank class="h-4 w-4" />
            </div>
            <div>
              <div class="text-sm font-black text-[#5A3B32]">预算状态</div>
              <div class="text-[11px] font-bold text-[#9A7A71]">周期：{{ budgetMonth }}</div>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <el-tag v-if="budget && safeBudgetTotal > 0" effect="light" round class="!font-black" :type="budgetTagType">
              {{ budgetLevel }} · {{ formatPct(safeBudgetTotal > 0 ? safeSpent / safeBudgetTotal : 0) }}
            </el-tag>
            <el-button size="small" class="!font-black" @click="goBudget">去预算页</el-button>
          </div>
        </div>

        <div v-if="budgetError" class="mt-3 text-xs font-bold text-rose-600">{{ budgetError }}</div>

        <div class="mt-4 grid grid-cols-1 md:grid-cols-3 gap-3">
          <div class="rounded-2xl border border-[#F1D6D0] bg-[#FFF8F5] p-4">
            <div class="text-[11px] font-extrabold text-[#9A7A71]">预算金额</div>
            <div class="mt-2 text-lg font-black" :class="safeBudgetTotal > 0 ? 'text-[#5A3B32]' : 'text-gray-400'">
              {{ safeBudgetTotal > 0 ? formatMoney(safeBudgetTotal) : '—' }}
            </div>
          </div>
          <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
            <div class="text-[11px] font-extrabold text-[#9A7A71]">已用金额</div>
            <div class="mt-2 text-lg font-black text-[#5A3B32]">{{ formatMoney(safeSpent) }}</div>
          </div>
          <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
            <div class="text-[11px] font-extrabold text-[#9A7A71]">使用进度</div>
            <div class="mt-3 h-3 rounded-full bg-[#FFF0EB] overflow-hidden ring-1 ring-[#F1D6D0]">
              <div class="h-full rounded-full transition-all" :class="budgetBarClass" :style="{ width: `${Math.min(100, Math.round(budgetRatio * 100))}%` }" />
            </div>
            <div class="mt-2 text-[11px] font-bold text-[#8E6C63]">
              {{
                safeBudgetTotal > 0
                  ? `${formatPct(safeSpent / safeBudgetTotal)}（${formatMoney(safeBudgetTotal - safeSpent)} 剩余）`
                  : '未设置激活预算，建议去预算页配置'
              }}
            </div>
          </div>
        </div>
      </div>

      <div class="rounded-3xl border border-[#F1D6D0] bg-white p-5 shadow-soft">
        <div class="flex items-center gap-2">
          <div class="h-9 w-9 rounded-2xl bg-secondary/15 ring-1 ring-secondary/20 flex items-center justify-center text-secondary">
            <BarChart3 class="h-4 w-4" />
          </div>
          <div>
            <div class="text-sm font-black text-[#5A3B32]">快捷操作</div>
            <div class="text-[11px] font-bold text-[#9A7A71]">常用入口一键到达</div>
          </div>
        </div>

        <div class="mt-4 grid grid-cols-2 gap-3">
          <el-button type="primary" color="#FF8A65" class="!h-11 !rounded-2xl !font-black" @click="emit('add-transaction')">记一笔</el-button>
          <el-button color="#7CD6C9" class="!h-11 !rounded-2xl !font-black !text-white" @click="goImport">
            <div class="flex items-center gap-2">
              <FileUp class="h-4 w-4" /> 导入账单
            </div>
          </el-button>
          <el-button class="!h-11 !rounded-2xl !font-black" @click="goReports">查看报表</el-button>
          <el-button class="!h-11 !rounded-2xl !font-black" @click="goAI">
            <div class="flex items-center gap-2">
              <Bot class="h-4 w-4" /> AI消费顾问
            </div>
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

