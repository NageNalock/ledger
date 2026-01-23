<script setup lang="ts">
import { computed, ref } from 'vue'
import dayjs from 'dayjs'
import HomeModulesPanel from '@/components/HomeModulesPanel.vue'
import TransactionModal from '@/components/TransactionModal.vue'

const mode = ref<'month' | '30d'>('month')

const startDate = computed(() => {
  if (mode.value === '30d') return dayjs().subtract(29, 'day').format('YYYY-MM-DD')
  return dayjs().startOf('month').format('YYYY-MM-DD')
})

const endDate = computed(() => {
  if (mode.value === '30d') return dayjs().format('YYYY-MM-DD')
  return dayjs().endOf('month').format('YYYY-MM-DD')
})

const modeText = computed(() => (mode.value === '30d' ? '最近 30 天' : '本月（自然月）'))

const modalVisible = ref(false)
const editData = ref(undefined)
const refreshKey = ref(0)

const handleModeChange = () => {
  refreshKey.value += 1
}

const handleAdd = () => {
  editData.value = undefined
  modalVisible.value = true
}

const handleSuccess = () => {
  refreshKey.value += 1
}
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-3xl border border-[#F1D6D0] bg-white p-5 shadow-soft">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="text-sm font-black text-[#5A3B32]">主页</div>
          <div class="mt-1 text-[11px] font-bold text-[#9A7A71]">口径：{{ modeText }}</div>
        </div>
        <el-radio-group v-model="mode" size="small" class="!rounded-2xl" @change="handleModeChange">
          <el-radio-button label="month">本月</el-radio-button>
          <el-radio-button label="30d">近 30 天</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <HomeModulesPanel :start-date="startDate" :end-date="endDate" :refresh-key="refreshKey" @add-transaction="handleAdd" />

    <TransactionModal v-model:visible="modalVisible" :edit-data="editData" @success="handleSuccess" />
  </div>
</template>
