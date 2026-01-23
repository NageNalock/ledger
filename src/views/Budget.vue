<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import request from '@/api/request'
import BudgetSummaryCards from '@/components/budget/BudgetSummaryCards.vue'
import BudgetSplitDrawerCard from '@/components/budget/BudgetSplitDrawerCard.vue'
import { CalendarDays, Plus, Sparkles, Trash2 } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'

type Threshold = { pct: number; label: string }
type CategoryL1 = { id: number; name: string }
type CategoryL2 = { id: number; parent_id: number; name: string }
type CategoryTree = { id: number; name: string; children: CategoryL2[] }
type BudgetLine = {
  id: number
  scope?: string
  category_l1_id?: number
  category_l1?: CategoryL1
  category_l2_id?: number
  category_l2?: { id: number; parent_id: number; name: string }
  amount: number
  used_amount: number
  remaining: number
  usage_ratio: number
  note?: string
}
type Budget = {
  budget_id: string
  month: string
  status: 'draft' | 'active' | 'archived' | 'discarded'
  total_amount: number
  used_amount: number
  remaining: number
  usage_ratio: number
  thresholds: Threshold[]
  waterline: string
  created_at: string
  lines: BudgetLine[]
}
const month = ref<string>(new Date().toISOString().slice(0, 7))

const loading = ref(false)
const errorText = ref('')
const budget = ref<Budget | null>(null)
const spentAmount = ref(0)

const editOpen = ref(false)
const saving = ref(false)
const currentTotal = ref<number>(0)
const draftTotal = ref<number>(0)
const categories = ref<CategoryTree[]>([])
type L2SplitDraft = { category_l2_id: string; amount: number }
type L1SplitDraft = { category_l1_id: string; amount: number; l2: L2SplitDraft[]; open: boolean }
const draftSplitL1 = ref<L1SplitDraft[]>([])

const allocatedSum = computed(() => draftSplitL1.value.reduce((acc, l) => acc + (Number(l.amount) || 0), 0))
const unallocatedBudget = computed(() => Math.max(0, (Number(draftTotal.value) || 0) - allocatedSum.value))


const monthSpentLabel = computed(() => `¥${Number(spentAmount.value || 0).toFixed(2)}`)

const headerMeta = computed(() => {
  const parts: string[] = []
  parts.push(`本月花费 ${monthSpentLabel.value}`)
  if (budget.value?.total_amount) parts.push(`预算 ${Number(budget.value.total_amount || 0).toFixed(2)}`)
  return parts.join(' · ')
})

const levelText = computed(() => {
  if (!budget.value) return '未设置'
  if (budget.value.waterline === 'ok') return '正常'
  return budget.value.waterline
})

const levelTagType = computed(() => {
  if (!budget.value) return 'info'
  if (budget.value.usage_ratio >= 1) return 'danger'
  if (budget.value.usage_ratio >= 0.8) return 'warning'
  return 'success'
})

const fetchActive = async () => {
  loading.value = true
  errorText.value = ''
  try {
    const res: any = await request.get('/budgets/active', { params: { month: month.value }, timeout: 20000 })
    budget.value = res?.budget || null
    spentAmount.value = Number(res?.spent_amount || 0)
    if (budget.value) {
      currentTotal.value = Number(budget.value.total_amount || 0)

      const formLines = Array.isArray(budget.value.lines) ? budget.value.lines : []
      const l1Lines = formLines.filter((l) => l?.scope === 'l1' && Number(l?.category_l1_id || 0) > 0)
      const l2Lines = formLines.filter((l) => l?.scope === 'l2' && Number(l?.category_l2_id || 0) > 0 && Number(l?.category_l1_id || 0) > 0)
      if (editOpen.value) {
        draftSplitL1.value = l1Lines.map((l) => {
          const l1id = String(l.category_l1_id)
          return {
            category_l1_id: l1id,
            amount: Number(l.amount || 0),
            open: false,
            l2: l2Lines
              .filter((x) => String(x.category_l1_id) === l1id)
              .map((x) => ({ category_l2_id: String(x.category_l2_id), amount: Number(x.amount || 0) }))
          }
        })
      }
    } else {
      currentTotal.value = 0
    }

    if (!editOpen.value) {
      resetDraftFromCurrent()
    }
  } catch (e: any) {
    errorText.value = String(e?.response?.data?.error || '预算加载失败')
  } finally {
    loading.value = false
  }
}

const resetDraftFromCurrent = () => {
  if (budget.value) {
    draftTotal.value = Number(budget.value.total_amount || 0)
    const formLines = Array.isArray(budget.value.lines) ? budget.value.lines : []
    const l1Lines = formLines.filter((l) => l?.scope === 'l1' && Number(l?.category_l1_id || 0) > 0)
    const l2Lines = formLines.filter((l) => l?.scope === 'l2' && Number(l?.category_l2_id || 0) > 0 && Number(l?.category_l1_id || 0) > 0)
    draftSplitL1.value = l1Lines.map((l) => {
      const l1id = String(l.category_l1_id)
      return {
        category_l1_id: l1id,
        amount: Number(l.amount || 0),
        open: false,
        l2: l2Lines
          .filter((x) => String(x.category_l1_id) === l1id)
          .map((x) => ({ category_l2_id: String(x.category_l2_id), amount: Number(x.amount || 0) }))
      }
    })
  } else {
    draftTotal.value = 0
    draftSplitL1.value = []
  }
}

const fetchCategories = async () => {
  try {
    const res: any = await request.get('/categories', { timeout: 20000 })
    categories.value = Array.isArray(res)
      ? res.map((c: any) => ({
          id: Number(c.id),
          name: String(c.name),
          children: Array.isArray(c.children)
            ? c.children.map((x: any) => ({ id: Number(x.id), parent_id: Number(x.parent_id), name: String(x.name) }))
            : []
        }))
      : []
  } catch {
    categories.value = []
  }
}

const l2OptionsForL1 = (l1Id: string) => {
  const l1 = categories.value.find((c) => String(c.id) === String(l1Id))
  return l1?.children || []
}

const l2AllocatedSum = (row: L1SplitDraft) => row.l2.reduce((acc, x) => acc + (Number(x.amount) || 0), 0)
const l2Unallocated = (row: L1SplitDraft) => Math.max(0, (Number(row.amount) || 0) - l2AllocatedSum(row))

const openEdit = () => {
  resetDraftFromCurrent()
  editOpen.value = true
}

const cancelEdit = () => {
  if (saving.value) return
  editOpen.value = false
  resetDraftFromCurrent()
}

const saveBudget = async () => {
  const amt = Number(draftTotal.value) || 0
  if (amt <= 0) {
    ElMessage.error('请输入大于 0 的预算金额')
    return
  }

  const lines: Array<{ category_l1_id: number; category_l2_id?: number; amount: number }> = []
  if (draftSplitL1.value.length) {
    const seenL1 = new Set<string>()
    const seenL2 = new Set<string>()

    for (const row of draftSplitL1.value) {
      const l1id = String(row.category_l1_id || '').trim()
      const l1amt = Number(row.amount) || 0
      if (!l1id || l1amt <= 0) {
        ElMessage.error('一级分类拆分需要选择分类并填写金额')
        return
      }
      if (seenL1.has(l1id)) {
        ElMessage.error('一级分类不能重复')
        return
      }
      seenL1.add(l1id)

      lines.push({ category_l1_id: Number(l1id), amount: l1amt })

      const l2sum = l2AllocatedSum(row)
      if (l2sum - l1amt > 0.0001) {
        ElMessage.error('二级分类拆分金额之和不能超过该一级分类预算')
        return
      }

      for (const c of row.l2) {
        const l2id = String(c.category_l2_id || '').trim()
        const l2amt = Number(c.amount) || 0
        if (!l2id || l2amt <= 0) {
          ElMessage.error('二级分类拆分需要选择分类并填写金额')
          return
        }
        const key = `${l1id}:${l2id}`
        if (seenL2.has(key)) {
          ElMessage.error('二级分类不能重复')
          return
        }
        seenL2.add(key)
        lines.push({ category_l1_id: Number(l1id), category_l2_id: Number(l2id), amount: l2amt })
      }
    }

    const l1Sum = draftSplitL1.value.reduce((acc, r) => acc + (Number(r.amount) || 0), 0)
    if (l1Sum - amt > 0.0001) {
      ElMessage.error('一级分类拆分金额之和不能超过总预算')
      return
    }
  }

  saving.value = true
  try {
    const payload: any = { month: month.value, status: 'active', total_amount: amt, lines }
    await request.put('/budgets/active', payload, { timeout: 20000 })
    editOpen.value = false
    await fetchActive()
    ElMessage.success('预算已更新')
  } catch (e: any) {
    ElMessage.error(String(e?.response?.data?.error || '保存失败'))
  } finally {
    saving.value = false
  }
}

const discardBudget = async () => {
  if (!budget.value) return
  try {
    await ElMessageBox.confirm('废弃后该月将不再应用预算。', '确认废弃？', {
      type: 'warning',
      confirmButtonText: '废弃',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }

  await request.post('/budgets/active/discard', { month: month.value }, { timeout: 20000 })
  await fetchActive()
}

const addSplitLine = () => {
  draftSplitL1.value.push({ category_l1_id: '', amount: Math.min(100, unallocatedBudget.value || 0), l2: [], open: false })
}

const removeSplitLine = (idx: number) => {
  draftSplitL1.value.splice(idx, 1)
}

const toggleL2Panel = (idx: number) => {
  draftSplitL1.value[idx].open = !draftSplitL1.value[idx].open
}

const addL2Split = (idx: number) => {
  draftSplitL1.value[idx].open = true
  draftSplitL1.value[idx].l2.push({ category_l2_id: '', amount: Math.min(50, l2Unallocated(draftSplitL1.value[idx]) || 0) })
}

const removeL2Split = (idx: number, j: number) => {
  draftSplitL1.value[idx].l2.splice(j, 1)
}

onMounted(() => {
  fetchActive()
  fetchCategories()
})
</script>

<template>
  <div class="space-y-6">
    <div class="bg-white p-6 rounded-2xl shadow-card border border-gray-100 relative overflow-hidden">
      <div class="absolute -top-14 -right-14 h-48 w-48 rounded-full bg-primary/10 blur-2xl pointer-events-none"></div>
      <div class="absolute -bottom-16 -left-16 h-56 w-56 rounded-full bg-secondary/10 blur-2xl pointer-events-none"></div>
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="flex items-center gap-2">
            <div class="h-10 w-10 rounded-2xl bg-gradient-to-br from-primary/20 to-secondary/15 ring-1 ring-primary/15 flex items-center justify-center">
              <CalendarDays class="w-5 h-5 text-primary" />
            </div>
            <h1 class="text-2xl font-black text-gray-900 tracking-tight">预算</h1>
          </div>
          <div class="mt-2 flex flex-wrap items-center gap-2">
            <span class="text-sm text-gray-600 font-bold">{{ headerMeta }}</span>
            <el-tag effect="light" round class="!font-black" :type="levelTagType">{{ levelText }}</el-tag>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <el-date-picker v-model="month" type="month" value-format="YYYY-MM" class="!w-[160px]" @change="fetchActive" />
          <el-button round type="primary" color="#FF8A65" class="!px-5 !font-black" @click="openEdit">
            <template #icon><Sparkles class="w-4 h-4" /></template>
            {{ budget ? '调整预算' : '设置预算' }}
          </el-button>
        </div>
      </div>
    </div>

    <div v-if="errorText" class="rounded-2xl bg-red-50 border border-red-100 p-4 text-red-700 font-bold">{{ errorText }}</div>

    <div v-if="loading" class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div v-for="i in 3" :key="i" class="h-32 bg-white rounded-3xl animate-pulse border border-[#F1D6D0]" />
    </div>

    <div v-else class="space-y-6">
      <BudgetSummaryCards
        :total="budget?.total_amount || 0"
        :used="budget?.used_amount || spentAmount"
        :remaining="budget?.remaining || 0"
        :ratio="budget?.usage_ratio || 0"
        :thresholds="budget?.thresholds || []"
        :lines="budget?.lines || []"
      />

      <BudgetSplitDrawerCard
        :total="budget?.total_amount || 0"
        :used="budget?.used_amount || spentAmount"
        :remaining="budget?.remaining || 0"
        :ratio="budget?.usage_ratio || 0"
        :lines="budget?.lines || []"
      />

    </div>

    <el-dialog
      v-model="editOpen"
      :title="budget ? '调整预算' : '设置预算'"
      width="640px"
      align-center
      :close-on-click-modal="!saving"
      :close-on-press-escape="!saving"
      :show-close="!saving"
    >
      <div class="space-y-5">
        <div class="rounded-2xl border border-[#F1D6D0] bg-[#FFF8F5] p-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">月份</div>
              <div class="mt-1 text-sm font-black text-[#5A3B32]">{{ month }}</div>
            </div>
            <div class="text-right">
              <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">总未分配预算</div>
              <div class="mt-1 text-sm font-black" :class="unallocatedBudget < 0 ? 'text-rose-700' : 'text-[#5A3B32]'">{{ unallocatedBudget.toFixed(2) }}</div>
            </div>
          </div>
        </div>

        <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">预算金额</div>
              <div class="mt-2">
                <el-input-number v-model="draftTotal" :min="0" :step="50" class="!w-full" controls-position="right" :disabled="saving" />
              </div>
            </div>
            <div class="text-right">
              <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">水位线</div>
              <div class="mt-1 text-xs font-bold text-[#8E6C63]">50% 提醒 / 80% 预警 / 100% 超支</div>
            </div>
          </div>
        </div>

        <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
          <div class="flex items-center justify-between gap-2">
            <div>
              <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">预算拆分（可选）</div>
              <div class="mt-1 text-xs font-bold text-[#8E6C63]">先添加一级，再在该一级下配置二级；未填写二级则视为未分配预算</div>
            </div>
            <el-button round type="primary" color="#FF8A65" plain class="!px-4 !font-black" :disabled="saving" @click="addSplitLine">
              <template #icon><Plus class="w-4 h-4" /></template>
              一级
            </el-button>
          </div>

          <div v-if="draftSplitL1.length" class="mt-4 space-y-3">
            <div v-for="(row, idx) in draftSplitL1" :key="idx" class="rounded-2xl border border-[#F1D6D0] bg-[#FFF8F5] p-3">
              <div class="grid grid-cols-[1fr_130px_32px] gap-2 items-center">
                <el-select
                  v-model="row.category_l1_id"
                  filterable
                  placeholder="选择一级分类"
                  class="!w-full"
                  :disabled="saving"
                  @change="() => { row.l2 = []; row.open = false }"
                >
                  <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="String(c.id)" />
                </el-select>
                <el-input-number v-model="row.amount" :min="0" :step="50" class="!w-full" controls-position="right" :disabled="saving" />
                <el-button circle plain type="danger" :disabled="saving" @click="removeSplitLine(idx)">
                  <Trash2 class="w-4 h-4" />
                </el-button>
              </div>

              <div class="mt-2 flex items-center justify-between">
                <div class="text-xs font-bold text-[#8E6C63]">二级已拆分 {{ l2AllocatedSum(row).toFixed(2) }} · 未拆分 {{ l2Unallocated(row).toFixed(2) }}</div>
                <div class="flex items-center gap-2">
                  <el-button round type="primary" color="#7CD6C9" plain class="!px-3 !font-black" :disabled="saving || !row.category_l1_id" @click="addL2Split(idx)">
                    <template #icon><Plus class="w-4 h-4" /></template>
                    二级
                  </el-button>
                  <el-button round class="!px-3 !font-black" :disabled="saving || !row.category_l1_id" @click="toggleL2Panel(idx)">
                    {{ row.open ? '收起' : '展开' }}
                  </el-button>
                </div>
              </div>

              <div v-if="row.open" class="mt-3 space-y-2">
                <div v-if="!row.category_l1_id" class="text-xs font-bold text-[#8E6C63]">先选择一级分类，再配置二级拆分</div>
                <div v-else>
                  <div v-for="(c, j) in row.l2" :key="j" class="grid grid-cols-[1fr_130px_32px] gap-2 items-center">
                    <el-select v-model="c.category_l2_id" filterable placeholder="选择二级分类" class="!w-full" :disabled="saving">
                      <el-option v-for="o in l2OptionsForL1(row.category_l1_id)" :key="o.id" :label="o.name" :value="String(o.id)" />
                    </el-select>
                    <el-input-number v-model="c.amount" :min="0" :step="50" class="!w-full" controls-position="right" :disabled="saving" />
                    <el-button circle plain type="danger" :disabled="saving" @click="removeL2Split(idx, j)">
                      <Trash2 class="w-4 h-4" />
                    </el-button>
                  </div>
                  <div v-if="row.l2.length === 0" class="text-xs font-bold text-[#8E6C63]">未分配预算</div>
                </div>
              </div>
            </div>
          </div>

          <div class="mt-3 flex items-center justify-between" v-if="draftSplitL1.length">
            <div class="text-xs font-bold text-[#8E6C63]">一级已拆分 {{ allocatedSum.toFixed(2) }} · 总未分配预算 {{ unallocatedBudget.toFixed(2) }}</div>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <el-button v-if="budget && budget.status !== 'discarded'" round type="danger" plain class="!px-5 !font-bold" :loading="saving" :disabled="saving" @click="discardBudget">
            <template #icon><Trash2 class="w-4 h-4" /></template>
            废弃
          </el-button>
          <el-button
            round
            class="!px-5 !font-bold"
            :disabled="saving"
            @click="cancelEdit"
          >
            取消
          </el-button>
          <el-button round type="primary" color="#FF8A65" class="!px-6 !font-black" :loading="saving" @click="saveBudget">保存并生效</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
