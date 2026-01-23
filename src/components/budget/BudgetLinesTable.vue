<script setup lang="ts">
import { computed } from 'vue'
import { ChevronRight } from 'lucide-vue-next'
import { formatMoney, formatPct } from '@/utils/format'

type Category = { id: number; name: string }
type Line = {
  id: number
  category_l1?: Category
  category_l2?: Category
  amount: number
  used_amount: number
  remaining: number
  usage_ratio: number
  note: string
}

const props = defineProps<{
  granularity: 'l1' | 'l2'
  lines: Line[]
}>()

const rows = computed(() => (Array.isArray(props.lines) ? props.lines : []))

const rowName = (row: Line) => {
  if (props.granularity === 'l2') {
    const l1 = row.category_l1?.name || ''
    const l2 = row.category_l2?.name || ''
    return l1 && l2 ? `${l1} / ${l2}` : l2 || l1 || '未选择'
  }
  return row.category_l1?.name || '未选择'
}

const barStyle = (ratio: number) => ({ width: `${Math.round(Math.min(1, Math.max(0, ratio || 0)) * 1000) / 10}%` })
</script>

<template>
  <div class="rounded-3xl border border-[#F1D6D0] bg-white shadow-soft overflow-hidden">
    <div class="flex items-center justify-between p-5">
      <div>
        <div class="text-lg font-black text-[#5A3B32]">分类预算明细</div>
        <div class="mt-1 text-xs font-bold text-[#8E6C63]">按 {{ granularity === 'l2' ? '二级分类' : '一级分类' }} 统计使用情况</div>
      </div>
      <div class="text-xs font-bold text-[#8E6C63]">点击行可查看备注</div>
    </div>

    <el-table
      :data="rows"
      style="width: 100%"
      :header-cell-style="{ background: '#FFF8F5', color: '#6B6B6B', fontWeight: 900 }"
      row-class-name="cursor-pointer"
    >
      <el-table-column label="分类" min-width="220">
        <template #default="{ row }">
          <div class="flex items-center justify-between gap-3">
            <div class="text-sm font-black text-[#5A3B32]">{{ rowName(row) }}</div>
            <ChevronRight class="w-4 h-4 text-[#C9A79C]" />
          </div>
        </template>
      </el-table-column>
      <el-table-column label="预算" width="140">
        <template #default="{ row }">
          <div class="text-sm font-extrabold text-[#5A3B32]">{{ formatMoney(row.amount) }}</div>
        </template>
      </el-table-column>
      <el-table-column label="已使用" width="140">
        <template #default="{ row }">
          <div class="text-sm font-extrabold text-[#6B4E45]">{{ formatMoney(row.used_amount) }}</div>
        </template>
      </el-table-column>
      <el-table-column label="使用率" min-width="220">
        <template #default="{ row }">
          <div class="flex items-center gap-3">
            <div class="flex-1 h-2 rounded-full bg-[#FFF1EC] overflow-hidden">
              <div class="h-full bg-gradient-to-r from-primary to-[#FFB4A2] transition-all duration-500" :style="barStyle(row.usage_ratio)"></div>
            </div>
            <div class="w-12 text-right text-xs font-black" :class="row.usage_ratio >= 1 ? 'text-rose-600' : row.usage_ratio >= 0.8 ? 'text-amber-600' : 'text-emerald-600'">
              {{ formatPct(row.usage_ratio) }}
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="剩余" width="140">
        <template #default="{ row }">
          <div class="text-sm font-extrabold" :class="row.remaining < 0 ? 'text-rose-600' : 'text-emerald-700'">{{ formatMoney(row.remaining) }}</div>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

