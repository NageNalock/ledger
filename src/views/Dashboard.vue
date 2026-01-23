<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import request from '@/api/request'
import dayjs from 'dayjs'
import TransactionModal from '@/components/TransactionModal.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { formatMoney } from '@/utils/format'

const transactions = ref([])
const loading = ref(false)
const modalVisible = ref(false)
const editData = ref(undefined)
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const categories = ref<any[]>([])
const users = ref<Array<{ id: number; username: string; avatar?: string }>>([])

const userStore = useUserStore()
const currentUser = computed(() => userStore.user || {})

const filters = reactive({
  dateRange: [] as string[],
  quickFilter: 'all', // all, week, month, year
  categoryL1Id: '',
  categoryL2Id: '',
  sortBy: '',
  userId: ''
})

const categoryL2Options = computed(() => {
  const l1 = categories.value.find((c: any) => String(c.id) === String(filters.categoryL1Id))
  return l1?.children || []
})

const handleQuickFilter = (type: string) => {
  filters.quickFilter = type
  filters.dateRange = []
  
  const end = dayjs().format('YYYY-MM-DD')
  let start = ''
  
  switch(type) {
    case 'week':
      start = dayjs().subtract(7, 'day').format('YYYY-MM-DD')
      filters.dateRange = [start, end]
      break
    case 'month':
      start = dayjs().startOf('month').format('YYYY-MM-DD')
      filters.dateRange = [start, dayjs().endOf('month').format('YYYY-MM-DD')]
      break
    case 'year':
      start = dayjs().subtract(1, 'year').format('YYYY-MM-DD')
      filters.dateRange = [start, end]
      break
    case 'all':
      filters.dateRange = []
      break
  }
  
  currentPage.value = 1
  fetchTransactions()
}

const handleCategoryL1Change = () => {
  const l2Options = categoryL2Options.value
  const current = filters.categoryL2Id
  if (!l2Options.find((c: any) => String(c.id) === String(current))) {
    filters.categoryL2Id = ''
  }
  currentPage.value = 1
  fetchTransactions()
}

const handleCategoryL2Change = () => {
  currentPage.value = 1
  fetchTransactions()
}

const handleSortChange = () => {
  currentPage.value = 1
  fetchTransactions()
}

const handleUserChange = () => {
  currentPage.value = 1
  fetchTransactions()
}

const handleDateRangeChange = () => {
  filters.quickFilter = 'custom'
  currentPage.value = 1
  fetchTransactions()
}

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchTransactions()
}

const fetchTransactions = async () => {
  loading.value = true
  try {
    const params: any = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    
    if (filters.dateRange && filters.dateRange.length === 2) {
      params.start_date = filters.dateRange[0]
      params.end_date = filters.dateRange[1]
    }

    if (filters.categoryL1Id) {
      params.category_l1_id = filters.categoryL1Id
    }

    if (filters.categoryL2Id) {
      params.category_l2_id = filters.categoryL2Id
    }

    if (filters.sortBy) {
      params.sort_by = 'amount'
      params.sort_order = filters.sortBy
    }

    if (filters.userId) {
      params.user_id = filters.userId
    }
    
    const res: any = await request.get('/transactions', { params })
    transactions.value = res.data
    total.value = res.total
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const fetchCategories = async () => {
  try {
    const res: any = await request.get('/categories')
    categories.value = res || []
  } catch (e) {
    console.error(e)
  }
}

const fetchUsers = async () => {
  const fallbackUser = currentUser.value?.id ? [currentUser.value] : []
  try {
    const res: any = await request.get('/users', { timeout: 20000 })
    users.value = Array.isArray(res) ? res : fallbackUser
  } catch (e: any) {
    if (fallbackUser.length) {
      users.value = fallbackUser
      return
    }
    users.value = []
  }
}

const handleAdd = () => {
  editData.value = undefined
  modalVisible.value = true
}

const handleEdit = (row: any) => {
  editData.value = JSON.parse(JSON.stringify(row)) // Deep copy
  modalVisible.value = true
}

const handleDelete = async (row: any) => {
  try {
    await ElMessageBox.confirm('确定删除这条消费记录吗？', '提示', {
      type: 'warning',
    })
    await request.delete(`/transactions/${row.id}`)
    ElMessage.success('删除成功')
    fetchTransactions()
  } catch (e) {
    // cancelled
  }
}


onMounted(() => {
  handleQuickFilter(filters.quickFilter)
  fetchCategories()
  fetchUsers()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header / Filters -->
    <div class="bg-white p-5 rounded-2xl shadow-card border border-gray-100">
      <div class="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
        <div class="space-y-3">
          <div class="flex flex-wrap items-center gap-3">
            <el-radio-group v-model="filters.quickFilter" @change="handleQuickFilter">
              <el-radio-button label="all">全部</el-radio-button>
              <el-radio-button label="week">最近一周</el-radio-button>
              <el-radio-button label="month">本月</el-radio-button>
              <el-radio-button label="year">最近一年</el-radio-button>
            </el-radio-group>

            <el-date-picker
              v-model="filters.dateRange"
              type="daterange"
              range-separator="至"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              @change="handleDateRangeChange"
              class="!w-[300px]"
            />
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            <el-select v-model="filters.categoryL1Id" placeholder="一级分类" clearable class="!w-full" @change="handleCategoryL1Change">
              <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>

            <el-select
              v-model="filters.categoryL2Id"
              placeholder="二级分类"
              clearable
              class="!w-full"
              :disabled="!filters.categoryL1Id"
              @change="handleCategoryL2Change"
            >
              <el-option v-for="c in categoryL2Options" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>

            <el-select v-model="filters.userId" placeholder="记账人" clearable class="!w-full" @change="handleUserChange">
              <el-option v-for="u in users" :key="u.id" :label="u.username" :value="String(u.id)" />
            </el-select>

            <el-select v-model="filters.sortBy" placeholder="金额排序" clearable class="!w-full" @change="handleSortChange">
              <el-option label="金额升序" value="asc" />
              <el-option label="金额降序" value="desc" />
            </el-select>
          </div>
        </div>

        <div class="flex justify-end">
          <el-button type="primary" color="#FF8A65" class="!px-5 !font-bold" @click="handleAdd">
            <el-icon class="mr-1"><Plus /></el-icon> 记一笔
          </el-button>
        </div>
      </div>
    </div>

    <!-- Transaction List -->
    <div class="bg-white rounded-lg shadow overflow-hidden flex flex-col h-[calc(100vh-200px)]">
      <div class="flex-1 overflow-auto">
        <el-table :data="transactions" v-loading="loading" style="width: 100%" height="100%">
          <el-table-column prop="transaction_time" label="时间" width="160">
            <template #default="{ row }">
              {{ dayjs(row.transaction_time).format('YYYY-MM-DD HH:mm') }}
            </template>
          </el-table-column>
          <el-table-column prop="merchant" label="商家" min-width="120" show-overflow-tooltip />
          <el-table-column label="分类" width="160">
            <template #default="{ row }">
              <el-tag size="small">{{ row.category_l1?.name }}</el-tag>
              <span class="mx-1 text-gray-300">/</span>
              <span class="text-gray-600 text-sm">{{ row.category_l2?.name }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="amount" label="金额" width="120" align="right">
            <template #default="{ row }">
              <span class="font-bold text-gray-900">{{ formatMoney(row.amount) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="source" label="来源" width="80" />
          <el-table-column label="记账人" width="100">
             <template #default="{ row }">
               <el-popover
                 placement="top"
                 :width="200"
                 trigger="hover"
                 popper-class="cute-popover"
                 :show-arrow="false"
               >
                 <template #reference>
                    <div class="flex items-center cursor-help">
                      <el-avatar :size="24" :src="row.user?.avatar || `https://robohash.org/${row.user?.username || 'cat'}?set=set4&bgset=bg1`" />
                    </div>
                 </template>
                 
                 <!-- User Card Content -->
                 <div class="flex flex-col items-center p-6 bg-white bg-opacity-90 backdrop-blur">
                    <div class="w-20 h-20 rounded-full p-1 bg-gradient-to-tr from-primary to-accent mb-3 shadow-lg">
                       <div class="w-full h-full rounded-full bg-white p-1 overflow-hidden">
                          <img :src="row.user?.avatar || `https://robohash.org/${row.user?.username || 'cat'}?set=set4&bgset=bg1`" class="w-full h-full object-cover rounded-full" />
                       </div>
                    </div>
                    <h3 class="text-lg font-bold text-gray-800">{{ row.user?.username || 'Unknown' }}</h3>
                    <p class="text-xs text-gray-400 mt-1">记账小能手</p>
                 </div>
               </el-popover>
             </template>
          </el-table-column>
          <el-table-column prop="remark" label="备注" show-overflow-tooltip />
          <el-table-column label="项目" width="120">
            <template #default="{ row }">
              <el-tag v-if="row.project" type="success" size="small">{{ row.project.name }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
              <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
      
      <!-- Pagination -->
      <div class="p-4 border-t flex justify-end">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handlePageChange(1)" 
          @current-change="handlePageChange"
        />
      </div>
    </div>

    <TransactionModal
      v-model:visible="modalVisible"
      :edit-data="editData"
      @success="fetchTransactions"
    />
  </div>
</template>
