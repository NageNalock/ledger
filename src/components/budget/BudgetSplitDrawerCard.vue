<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronDown, Layers } from 'lucide-vue-next'
import { formatMoney } from '@/utils/format'

type CategoryL1 = { id: number; name: string }
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

const props = defineProps<{
  total: number
  used: number
  remaining: number
  ratio: number
  lines: BudgetLine[]
}>()

const open = ref(false)
const openL1 = ref<Record<string, boolean>>({})

const safeRatio = computed(() => Math.min(1, Math.max(0, Number(props.ratio || 0))))
const percent = computed(() => Math.round(safeRatio.value * 100))

const hasLines = computed(() => Number(props.total || 0) > 0)

const l1Lines = computed(() => (Array.isArray(props.lines) ? props.lines : []).filter((l) => l?.scope === 'l1' && Number(l?.amount || 0) > 0))
const l2Lines = computed(() => (Array.isArray(props.lines) ? props.lines : []).filter((l) => l?.scope === 'l2' && Number(l?.amount || 0) > 0))
const l1Unsplit = computed(() => (Array.isArray(props.lines) ? props.lines : []).filter((l) => l?.scope === 'l1_unsplit' && Number(l?.amount || 0) > 0))
const globalUnsplit = computed(() => (Array.isArray(props.lines) ? props.lines : []).find((l) => l?.scope === 'global_unsplit' && Number(l?.amount || 0) > 0) || null)

const l1Name = (l: BudgetLine) => String(l?.category_l1?.name || '一级分类')
const l2Name = (l: BudgetLine) => String(l?.category_l2?.name || l?.note || '未分配预算')

const isUnsplit = (l: BudgetLine) => {
  const note = String(l?.note || '')
  return note === '未分配预算' || note === '未拆分' || String(l?.scope || '').includes('unsplit')
}

const childrenForL1 = (l1id: number) => {
  const a = l2Lines.value.filter((x) => Number(x?.category_l1_id || 0) === l1id)
  const u = l1Unsplit.value.filter((x) => Number(x?.category_l1_id || 0) === l1id)
  const all = [...a, ...u]
  all.sort((x, y) => Number(isUnsplit(x)) - Number(isUnsplit(y)))
  return all
}

const toggleL1 = (l1id: number) => {
  const key = String(l1id)
  openL1.value = { ...openL1.value, [key]: !openL1.value[key] }
}

const isL1Open = (l1id: number) => Boolean(openL1.value[String(l1id)])
const linePct = (l: BudgetLine) => Math.min(100, Math.max(0, Math.round((Number(l?.usage_ratio || 0) || 0) * 100)))

const lineColor = (l: BudgetLine) => {
  const r = Number(l?.usage_ratio || 0) || 0
  if (r >= 1) return '#FB7185'
  if (r >= 0.8) return '#FF8A65'
  return '#7CD6C9'
}

const totalColor = computed(() => {
  if (safeRatio.value >= 1) return '#FB7185'
  if (safeRatio.value >= 0.8) return '#FF8A65'
  return '#7CD6C9'
})

const formatText = (p: number) => `${p}%`

const toggle = () => {
  open.value = !open.value
  if (!open.value) openL1.value = {}
}
</script>

<template>
  <div v-if="hasLines" class="rounded-3xl border border-[#F1D6D0] bg-white p-5 shadow-soft hover:shadow-md transition-shadow">
    <button type="button" class="w-full text-left" @click="toggle">
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <div class="h-9 w-9 rounded-2xl bg-secondary/15 ring-1 ring-secondary/20 flex items-center justify-center text-secondary">
            <Layers class="w-4 h-4" />
          </div>
          <div>
            <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">拆分明细</div>
            <div class="mt-1 text-sm font-black text-[#5A3B32]">总计 {{ formatMoney(used) }} / {{ formatMoney(total) }}</div>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <ChevronDown class="w-4 h-4 text-[#8E6C63] transition-transform" :class="open ? 'rotate-180' : ''" />
        </div>
      </div>

      <div class="mt-3 flex items-center gap-2">
        <el-progress :percentage="percent" :stroke-width="12" :show-text="false" :color="totalColor" class="flex-1 min-w-0" />
        <div class="w-[44px] text-right text-[12px] font-black text-[#5A3B32]">{{ formatText(percent) }}</div>
      </div>
      <div class="mt-2 flex items-center justify-between text-[11px] font-bold text-[#8E6C63]">
        <span>剩余 {{ formatMoney(remaining) }}</span>
        <span>{{ open ? '收起' : '展开明细' }}</span>
      </div>
    </button>

    <el-collapse-transition>
      <div v-show="open" class="mt-4 border-t border-[#F1D6D0] pt-4">
        <div class="space-y-3">
          <div v-for="l1 in l1Lines" :key="`${l1.id}-${l1.category_l1_id || 'na'}`" class="rounded-2xl border border-[#F1D6D0] bg-white p-3">
            <button type="button" class="w-full text-left" @click="toggleL1(Number(l1.category_l1_id || 0))">
              <div class="flex items-center justify-between gap-2">
                <div class="min-w-0 truncate text-sm font-black text-[#5A3B32]">{{ l1Name(l1) }}</div>
                <div class="shrink-0 text-[11px] font-bold text-[#8E6C63]">{{ formatMoney(l1.used_amount) }} / {{ formatMoney(l1.amount) }}</div>
              </div>
              <div class="mt-2 flex items-center gap-2">
                <el-progress :percentage="linePct(l1)" :stroke-width="10" :show-text="false" :color="lineColor(l1)" class="flex-1 min-w-0" />
                <div class="w-[44px] text-right text-[12px] font-black text-[#5A3B32]">{{ formatText(linePct(l1)) }}</div>
              </div>
              <div class="mt-1 flex items-center justify-between text-[11px] font-bold text-[#8E6C63]">
                <span :class="l1.remaining < 0 ? 'text-rose-700' : ''">剩余 {{ formatMoney(l1.remaining) }}</span>
                <span>{{ isL1Open(Number(l1.category_l1_id || 0)) ? '收起二级' : '展开二级' }}</span>
              </div>
            </button>

            <el-collapse-transition>
              <div v-show="isL1Open(Number(l1.category_l1_id || 0))" class="mt-3 space-y-2">
                <div v-for="c in childrenForL1(Number(l1.category_l1_id || 0))" :key="`${c.id}-${c.category_l2_id || c.note || 'na'}`" class="rounded-xl border border-[#F1D6D0] bg-[#FFF8F5] p-3">
                  <div class="flex items-center justify-between gap-2">
                    <div class="min-w-0 truncate text-[13px] font-black text-[#5A3B32]">{{ isUnsplit(c) ? `${l1Name(l1)} · 未分配预算` : l2Name(c) }}</div>
                    <div class="shrink-0 text-[11px] font-bold text-[#8E6C63]">{{ formatMoney(c.used_amount) }} / {{ formatMoney(c.amount) }}</div>
                  </div>
                  <div class="mt-1 text-[11px] font-bold" :class="c.remaining < 0 ? 'text-rose-700' : 'text-[#8E6C63]'">剩余 {{ formatMoney(c.remaining) }}</div>
                  <div class="mt-2 flex items-center gap-2">
                    <el-progress :percentage="linePct(c)" :stroke-width="8" :show-text="false" :color="lineColor(c)" class="flex-1 min-w-0" />
                    <div class="w-[44px] text-right text-[12px] font-black text-[#5A3B32]">{{ formatText(linePct(c)) }}</div>
                  </div>
                </div>
              </div>
            </el-collapse-transition>
          </div>

          <div v-if="globalUnsplit" class="rounded-2xl border border-[#F1D6D0] bg-[#FFF8F5] p-3">
            <div class="flex items-center justify-between gap-2">
              <div class="min-w-0 truncate text-sm font-black text-[#5A3B32]">总预算 · 未分配预算</div>
              <div class="shrink-0 text-[11px] font-bold text-[#8E6C63]">{{ formatMoney(globalUnsplit.used_amount) }} / {{ formatMoney(globalUnsplit.amount) }}</div>
            </div>
            <div class="mt-1 text-[11px] font-bold" :class="globalUnsplit.remaining < 0 ? 'text-rose-700' : 'text-[#8E6C63]'">剩余 {{ formatMoney(globalUnsplit.remaining) }}</div>
            <div class="mt-2 flex items-center gap-2">
              <el-progress :percentage="linePct(globalUnsplit)" :stroke-width="10" :show-text="false" :color="lineColor(globalUnsplit)" class="flex-1 min-w-0" />
              <div class="w-[44px] text-right text-[12px] font-black text-[#5A3B32]">{{ formatText(linePct(globalUnsplit)) }}</div>
            </div>
          </div>
        </div>
      </div>
    </el-collapse-transition>
  </div>
</template>
