<script setup lang="ts">
import dayjs from 'dayjs'
import { CircleClose, Plus } from '@element-plus/icons-vue'

defineProps<{
  categories: any[]
  categoryL2Options: any[]
  filters: {
    quickFilter: string
    dateRange: string[]
    categoryL1Id: string
    categoryL2Id: string
    source: string
    sort: string
  }
  trendPointDate: string
  summaryFiltered: number
  transactions: any[]
  listLoading: boolean
  total: number
  currentPage: number
  pageSize: number
}>()

const emit = defineEmits<{
  (e: 'add'): void
  (e: 'edit', row: any): void
  (e: 'delete', row: any): void
  (e: 'clear-trend-point'): void
  (e: 'quick-filter-change', v: string): void
  (e: 'date-range-change'): void
  (e: 'category-l1-change'): void
  (e: 'category-l2-change'): void
  (e: 'source-change'): void
  (e: 'sort-change'): void
  (e: 'page-change', page: number): void
  (e: 'update:pageSize', v: number): void
}>()
</script>

<template>
  <div class="bg-white p-6 rounded-2xl shadow-card">
    <div class="flex flex-wrap items-center justify-between gap-3 mb-4">
      <h3 class="text-lg font-bold text-gray-700 flex items-center gap-2">
        <div class="w-1 h-5 bg-gray-400 rounded-full"></div>
        流水明细
        <span v-if="trendPointDate" class="text-sm bg-secondary/10 text-secondary px-2 py-0.5 rounded ml-2">
          已筛选: {{ trendPointDate }}
          <el-icon class="cursor-pointer ml-1 translate-y-0.5" @click="emit('clear-trend-point')"><CircleClose /></el-icon>
        </span>
      </h3>
      <div class="flex items-center gap-3">
        <div class="text-sm text-gray-500 font-bold">当前筛选总支出：¥{{ Number(summaryFiltered).toFixed(2) }}</div>
        <el-button type="primary" color="#FF6B35" @click="emit('add')">
          <el-icon class="mr-1"><Plus /></el-icon> 记一笔
        </el-button>
      </div>
    </div>

    <div class="bg-white p-4 rounded-xl border border-gray-100 mb-4">
      <div class="flex flex-wrap items-center gap-3">
        <el-radio-group :model-value="filters.quickFilter" @update:model-value="(v: any) => emit('quick-filter-change', v)">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="week">最近一周</el-radio-button>
          <el-radio-button label="month">最近一月</el-radio-button>
          <el-radio-button label="year">最近一年</el-radio-button>
          <el-radio-button label="custom">自定义</el-radio-button>
        </el-radio-group>

        <el-date-picker
          v-if="filters.quickFilter === 'custom'"
          v-model="filters.dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          @change="emit('date-range-change')"
          style="width: 240px"
        />

        <el-select v-model="filters.categoryL1Id" placeholder="一级分类" clearable style="width: 160px" @change="emit('category-l1-change')">
          <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>

        <el-select
          v-model="filters.categoryL2Id"
          placeholder="二级分类"
          clearable
          style="width: 160px"
          :disabled="!filters.categoryL1Id"
          @change="emit('category-l2-change')"
        >
          <el-option v-for="c in categoryL2Options" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>

        <el-select v-model="filters.source" placeholder="来源" clearable style="width: 140px" @change="emit('source-change')">
          <el-option label="无" value="无" />
          <el-option label="微信" value="微信" />
          <el-option label="支付宝" value="支付宝" />
          <el-option label="银行卡" value="银行卡" />
          <el-option label="现金" value="现金" />
        </el-select>

        <el-select v-model="filters.sort" placeholder="排序" clearable style="width: 160px" @change="emit('sort-change')">
          <el-option label="时间降序" value="transaction_time_desc" />
          <el-option label="时间升序" value="transaction_time_asc" />
          <el-option label="金额降序" value="amount_desc" />
          <el-option label="金额升序" value="amount_asc" />
          <el-option label="来源升序" value="source_asc" />
          <el-option label="来源降序" value="source_desc" />
        </el-select>
      </div>
    </div>

    <el-table :data="transactions" v-loading="listLoading" style="width: 100%">
      <el-table-column prop="transaction_time" label="时间" width="160">
        <template #default="{ row }">
          {{ dayjs(row.transaction_time).format('YYYY-MM-DD HH:mm') }}
        </template>
      </el-table-column>
      <el-table-column prop="merchant" label="商家/商品" show-overflow-tooltip min-width="180" />
      <el-table-column prop="amount" label="金额" width="120">
        <template #default="{ row }">
          <span class="font-bold text-primary">¥{{ row.amount.toFixed(2) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="分类" width="160">
        <template #default="{ row }">
          <span class="text-gray-500">{{ row.category_l1?.name }}</span>
          <span class="mx-1">/</span>
          <span class="font-bold text-gray-700">{{ row.category_l2?.name }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="source" label="来源" width="90" />
      <el-table-column prop="remark" label="备注" show-overflow-tooltip min-width="180" />
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="emit('edit', row)">编辑</el-button>
          <el-button link type="danger" size="small" @click="emit('delete', row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="p-4 border-t flex justify-end">
      <el-pagination
        :current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="(v: number) => emit('update:pageSize', v)"
        @current-change="(p: number) => emit('page-change', p)"
      />
    </div>
  </div>
</template>
