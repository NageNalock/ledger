<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  totalAll: number
  budgetEnabled: boolean
  budgetLimit: number
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'update:budgetEnabled', v: boolean): void
  (e: 'update:budgetLimit', v: number): void
  (e: 'save'): void
}>()

const noBudget = computed({
  get: () => !props.budgetEnabled,
  set: (v: boolean) => emit('update:budgetEnabled', !v),
})

const hasBudgetLimit = computed(() => props.budgetEnabled && Number(props.budgetLimit) > 0)

const percent = computed(() => {
  if (!hasBudgetLimit.value) return 0
  const p = (Number(props.totalAll) / Number(props.budgetLimit)) * 100
  if (!Number.isFinite(p)) return 0
  return Math.min(100, Math.max(0, p))
})

const status = computed<'success' | 'warning' | 'exception'>(() => {
  if (!hasBudgetLimit.value) return 'success'
  const raw = (Number(props.totalAll) / Number(props.budgetLimit)) * 100
  if (raw >= 100) return 'exception'
  if (raw >= 80) return 'warning'
  return 'success'
})

const text = computed(() => {
  if (!props.budgetEnabled) return '无预算上限'
  return `预算上限：¥${Number(props.budgetLimit).toFixed(2)}`
})
</script>

<template>
  <div class="bg-white p-6 rounded-2xl shadow-card">
    <h3 class="text-base font-bold text-gray-700 flex items-center gap-2 mb-4">
      <div class="w-1 h-5 bg-accent rounded-full"></div> 预算
    </h3>

    <div class="flex items-center justify-between">
      <div class="text-sm text-gray-600 font-bold">无预算上限</div>
      <el-switch v-model="noBudget" />
    </div>

    <div class="mt-3 text-xs text-gray-400 font-bold">{{ text }}</div>

    <div v-if="hasBudgetLimit" class="mt-4 flex items-center justify-between gap-3">
      <div class="flex-1">
        <div class="text-xs text-gray-400 font-bold">已用（全部时间）</div>
        <div class="text-lg font-bold text-gray-800">¥{{ Number(totalAll).toFixed(2) }}</div>
      </div>
      <el-progress type="dashboard" :percentage="percent" :status="status" :width="120" />
    </div>

    <div v-if="noBudget" class="mt-4 p-3 rounded-xl bg-gray-50 text-sm text-gray-600">
      当前项目默认无预算上限。
    </div>

    <div v-else class="mt-4 space-y-3">
      <div class="text-xs text-gray-400 font-bold">设置预算上限</div>
      <el-input-number
        :model-value="budgetLimit"
        :precision="2"
        :min="0"
        style="width: 100%"
        @update:model-value="(v: any) => emit('update:budgetLimit', Number(v || 0))"
      />
      <el-button type="primary" color="#FF6B35" :loading="saving" style="width: 100%" @click="emit('save')">
        保存
      </el-button>
    </div>
  </div>
</template>

