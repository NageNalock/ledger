<script setup lang="ts">
import { computed } from 'vue'
import { Coins, PiggyBank, Waves } from 'lucide-vue-next'
import { formatMoney, formatPct } from '@/utils/format'

type Threshold = { pct: number; label: string }
type BudgetLine = {
  scope?: string
  category_l1?: { id: number; name: string }
  category_l2?: { id: number; parent_id: number; name: string }
  usage_ratio?: number
  remaining: number
  note?: string
}

const props = withDefaults(
  defineProps<{
    total: number
    used: number
    remaining: number
    ratio: number
    thresholds?: Threshold[]
    lines?: BudgetLine[]
  }>(),
  {
    thresholds: () => [],
    lines: () => [],
  }
)

const safeRatio = computed(() => Math.min(1, Math.max(0, Number(props.ratio || 0))))
const hasBudget = computed(() => Number(props.total || 0) > 0)


const sortedThresholds = computed(() => {
  const t = Array.isArray(props.thresholds) ? [...props.thresholds] : []
  return t
    .filter((x) => typeof x?.pct === 'number' && x.pct >= 0 && x.pct <= 1)
    .sort((a, b) => a.pct - b.pct)
})

const waterlineText = computed(() => {
  if (!hasBudget.value) return '未设置'
  const r = safeRatio.value
  let label = '正常'
  for (const t of sortedThresholds.value) {
    if (r >= t.pct) {
      label = String(t.label || '').trim() || formatPct(t.pct)
    }
  }
  return label
})

const warningPct = computed(() => {
  const t = Array.isArray(props.thresholds) ? props.thresholds : []
  const warn = t.find((x) => String(x?.label || '').includes('预警'))
  if (warn && typeof warn.pct === 'number') return warn.pct
  return 0.8
})

const displayNameForLine = (l: BudgetLine) => {
  const scope = String(l?.scope || '')
  const isUnallocated = String(l?.note || '') === '未分配预算' || scope.includes('unsplit')
  if (scope === 'global_unsplit' || (isUnallocated && !l?.category_l1 && !l?.category_l2)) return '总预算·未分配预算'
  if (scope === 'l1_unsplit' || (isUnallocated && l?.category_l1 && !l?.category_l2)) return `${l.category_l1.name}·未分配预算`
  if (l?.category_l2?.name) return `${l?.category_l1?.name || '一级'} / ${l.category_l2.name}`
  if (l?.category_l1?.name) return l.category_l1.name
  return '未分配预算'
}

const overrunItems = computed(() => {
  const items: string[] = []
  if (Number(props.remaining) < 0) items.push('总预算')
  const lines = Array.isArray(props.lines) ? props.lines : []
  for (const l of lines) {
    if (Number(l?.remaining) >= 0) continue
    items.push(displayNameForLine(l))
  }
  return Array.from(new Set(items))
})

const warningItems = computed(() => {
  const items: string[] = []
  if (Number(props.remaining) >= 0 && safeRatio.value >= warningPct.value) items.push('总预算')
  const lines = Array.isArray(props.lines) ? props.lines : []
  for (const l of lines) {
    if (Number(l?.remaining) < 0) continue
    const r = Number(l?.usage_ratio || 0) || 0
    if (r < warningPct.value) continue
    items.push(displayNameForLine(l))
  }
  return Array.from(new Set(items))
})

const overrunText = computed(() => {
  if (!overrunItems.value.length) return ''
  const head = overrunItems.value.slice(0, 2).join('、')
  const more = overrunItems.value.length > 2 ? ` 等${overrunItems.value.length}项` : ''
  return `超支：${head}${more}`
})

const warningText = computed(() => {
  if (!warningItems.value.length) return ''
  const head = warningItems.value.slice(0, 2).join('、')
  const more = warningItems.value.length > 2 ? ` 等${warningItems.value.length}项` : ''
  return `预警：${head}${more}`
})

const tagType = computed(() => {
  if (!hasBudget.value) return 'info'
  if (safeRatio.value >= 1) return 'danger'
  if (safeRatio.value >= 0.8) return 'warning'
  return 'success'
})

</script>

<template>
  <div class="rounded-3xl border border-[#F1D6D0] bg-white p-5 shadow-soft hover:shadow-md transition-shadow">
    <div class="flex items-start justify-between gap-3">
      <div>
        <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">本月概览</div>
        <div class="mt-2 text-2xl font-black text-[#5A3B32]">{{ hasBudget ? formatMoney(used) : '—' }}</div>
        <div class="mt-1 text-xs font-bold text-[#8E6C63]">本月花费</div>
      </div>

      <div class="flex flex-col items-end gap-2">
        <el-tag effect="light" round class="!font-black" :type="tagType">水位线：{{ waterlineText }}</el-tag>
        <el-tag v-if="overrunText" effect="light" round class="!font-black" type="danger">{{ overrunText }}</el-tag>
        <el-tag v-else-if="warningText" effect="light" round class="!font-black" type="warning">{{ warningText }}</el-tag>
      </div>
    </div>

    <div class="mt-4 grid grid-cols-3 gap-3">
      <div class="rounded-2xl border border-[#F1D6D0] bg-[#FFF8F5] p-3">
        <div class="flex items-center justify-between">
          <div class="text-[11px] font-extrabold text-[#9A7A71]">预算</div>
          <div class="h-8 w-8 rounded-2xl bg-primary/10 ring-1 ring-primary/15 flex items-center justify-center text-primary">
            <PiggyBank class="w-4 h-4" />
          </div>
        </div>
        <div class="mt-2 text-sm font-black text-[#5A3B32]">{{ formatMoney(total) }}</div>
      </div>

      <div class="rounded-2xl border border-[#F1D6D0] bg-white p-3">
        <div class="flex items-center justify-between">
          <div class="text-[11px] font-extrabold text-[#9A7A71]">花费</div>
          <div class="h-8 w-8 rounded-2xl bg-[#FFB4A2]/20 ring-1 ring-[#FFB4A2]/30 flex items-center justify-center text-[#E85D4A]">
            <Coins class="w-4 h-4" />
          </div>
        </div>
        <div class="mt-2 text-sm font-black text-[#5A3B32]">{{ formatMoney(used) }}</div>
      </div>

      <div class="rounded-2xl border border-[#F1D6D0] bg-white p-3">
        <div class="flex items-center justify-between">
          <div class="text-[11px] font-extrabold text-[#9A7A71]">剩余</div>
          <div class="h-8 w-8 rounded-2xl bg-secondary/15 ring-1 ring-secondary/20 flex items-center justify-center text-secondary">
            <Waves class="w-4 h-4" />
          </div>
        </div>
        <div
          class="mt-2 text-sm font-black"
          :class="hasBudget ? (remaining < 0 ? 'text-rose-700' : 'text-[#5A3B32]') : 'text-gray-400'"
        >
          {{ hasBudget ? formatMoney(remaining) : '—' }}
        </div>
        <div v-if="hasBudget" class="mt-1 text-[11px] font-bold text-[#8E6C63]">预算 - 花费</div>
      </div>
    </div>

  </div>
</template>
