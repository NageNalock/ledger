<script setup lang="ts">
import TransactionModal from '@/components/TransactionModal.vue'
import ProjectBudgetCard from '@/components/ProjectBudgetCard.vue'
import ProjectTrendCard from '@/components/ProjectTrendCard.vue'
import ProjectTransactionsPanel from '@/components/ProjectTransactionsPanel.vue'
import { useProjectDetail } from '@/composables/useProjectDetail'
import { ArrowLeft } from '@element-plus/icons-vue'

const {
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
} = useProjectDetail()
</script>

<template>
  <div class="space-y-6">
    <div class="bg-white p-4 rounded-2xl shadow-card flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <el-button circle @click="router.push('/projects')">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <div>
          <div class="text-xs text-gray-400 font-bold">项目</div>
          <div class="text-lg font-bold text-gray-800">{{ pageTitle }}</div>
        </div>
      </div>
      <div class="text-sm text-gray-500 font-bold">全部时间总支出：¥{{ Number(summaryAll).toFixed(2) }}</div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <ProjectTrendCard
        :option="lineOption"
        :loading="trendLoading"
        :granularity="granularity"
        @update:granularity="handleGranularityUpdate"
        @point-click="handleTrendPointClick"
      />

      <ProjectBudgetCard
        :total-all="summaryAll"
        :budget-enabled="budgetEnabled"
        :budget-limit="budgetLimit"
        :saving="budgetSaving"
        @update:budget-enabled="(v) => (budgetEnabled = v)"
        @update:budget-limit="(v) => (budgetLimit = v)"
        @save="handleSaveBudget"
      />
    </div>

    <ProjectTransactionsPanel
      :categories="categories"
      :category-l2-options="categoryL2Options"
      :filters="filters"
      :trend-point-date="trendPointDate"
      :summary-filtered="summaryFiltered"
      :transactions="transactions"
      :list-loading="listLoading"
      :total="total"
      :current-page="currentPage"
      :page-size="pageSize"
      @add="handleAdd"
      @edit="handleEdit"
      @delete="handleDelete"
      @clear-trend-point="handleClearTrendPoint"
      @quick-filter-change="handleQuickFilterChange"
      @date-range-change="handleDateRangeChange"
      @category-l1-change="handleCategoryL1Change"
      @category-l2-change="handleCategoryL2Change"
      @source-change="handleSourceChange"
      @sort-change="handleSortChange"
      @page-change="handlePageChange"
      @update:pageSize="handlePageSizeUpdate"
    />

    <TransactionModal
      v-model:visible="modalVisible"
      :edit-data="editData"
      :default-project-id="Number(projectId)"
      @success="refreshAll"
    />
  </div>
</template>

