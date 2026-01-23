<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import request from '@/api/request'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import { UploadFilled, Refresh, MagicStick } from '@element-plus/icons-vue'

const drafts = ref<any[]>([])
const categories = ref<any[]>([])
const projects = ref<any[]>([])
const loading = ref(false)
const selectedDrafts = ref<any[]>([])
const uploadRef = ref()
const importType = ref('alipay')
const batchAiLoading = ref(false)

const uploadHeaders = computed(() => {
    return { Authorization: `Bearer ${localStorage.getItem('token')}` }
})

const fetchDrafts = async () => {
  loading.value = true
  try {
    const res: any = await request.get('/drafts')
    drafts.value = res
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

const fetchProjects = async () => {
  try {
    const res: any = await request.get('/projects', { timeout: 20000 })
    projects.value = Array.isArray(res) ? res : []
  } catch (e) {
    projects.value = []
  }
}

onMounted(() => {
  fetchDrafts()
  fetchCategories()
  fetchProjects()
})

const handleUploadSuccess = (response: any) => {
  ElMessage.success(response.message || '上传成功')
  uploadRef.value.clearFiles()
  fetchDrafts()
}

const handleUploadError = (err: any) => {
  ElMessage.error('上传失败')
  console.error(err)
}

const handleDelete = async (id: number) => {
  try {
    await ElMessageBox.confirm('确定删除该草稿吗?', '提示', {
      type: 'warning',
    })
    await request.delete(`/drafts/${id}`)
    ElMessage.success('删除成功')
    fetchDrafts()
  } catch (e) {
    // cancelled or error
  }
}

const handleAnalyze = async (row: any) => {
  try {
    const errorMsg = escapeHtml(row.error_msg || '未知错误')
    const rawData = escapeHtml(row.raw_data || '无')
    const message = row.status === 'error'
      ? `<div class="ai-confirm">
          <div class="ai-confirm__header">
            <div class="ai-confirm__icon">✨</div>
            <div>
              <div class="ai-confirm__title">AI 分析确认</div>
              <div class="ai-confirm__subtitle">该草稿将重新进行分析. 该操作可能会导致记账不准确</div>
            </div>
          </div>
          <div class="ai-confirm__error">
            <div class="ai-confirm__label">上次失败</div>
            <div class="ai-confirm__text">${errorMsg}</div>
          </div>
          <div class="ai-confirm__raw">
            <div class="ai-confirm__label">原始数据</div>
            <div class="ai-confirm__text">${rawData}</div>
          </div>
        </div>`
      : `<div class="ai-confirm">
          <div class="ai-confirm__header">
            <div class="ai-confirm__icon">✨</div>
            <div>
              <div class="ai-confirm__title">AI 分析确认</div>
              <div class="ai-confirm__subtitle">确认对该草稿进行分析吗？</div>
            </div>
          </div>
        </div>`
    await ElMessageBox.confirm(
      message,
      'AI 分析确认',
      {
        type: row.status === 'error' ? 'warning' : 'info',
        confirmButtonText: '继续分析',
        cancelButtonText: '取消',
        customClass: 'ai-confirm-box',
        dangerouslyUseHTMLString: true
      }
    )
  } catch (e) {
    return
  }

  row.status = 'analyzing'
  try {
    const res: any = await request.post(`/drafts/${row.id}/analyze`)
    Object.assign(row, res)
    ElMessage.success('分析完成')
  } catch (e) {
    row.status = 'error'
    console.error(e)
  }
}

const escapeHtml = (value: string) => {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

const handleSelectionChange = (val: any[]) => {
  selectedDrafts.value = val
}

const handleBatchAnalyze = async () => {
  if (selectedDrafts.value.length === 0) return
  
  batchAiLoading.value = true
  const ids = selectedDrafts.value.map(d => d.id)
  
  try {
    await request.post('/drafts/analyze', { ids })
    ElMessage.success('已开始批量分析，请稍后刷新查看结果')
    
    // Update status optimistically
    for (const draft of drafts.value) {
      if (ids.includes(draft.id)) {
        draft.status = 'analyzing'
      }
    }
  } catch (e) {
    console.error(e)
  } finally {
    batchAiLoading.value = false
  }
}

const handleBatchSubmit = async () => {
  if (selectedDrafts.value.length === 0) return
  
  const ids = selectedDrafts.value.map(d => d.id)
  try {
    loading.value = true
    const res: any = await request.post('/drafts/submit', { ids })
    ElMessage.success(`成功提交 ${res.success_count} 条记录`)
    if (res.errors && res.errors.length > 0) {
      ElMessage.warning(`部分提交失败: ${res.errors.join('; ')}`)
    }
    fetchDrafts()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const handleBatchDelete = async () => {
  if (selectedDrafts.value.length === 0) return
  
  try {
    await ElMessageBox.confirm(`确定删除选中的 ${selectedDrafts.value.length} 条草稿吗?`, '提示', {
      type: 'warning',
    })
    
    // Ideally backend supports batch delete, but for now loop or specific endpoint
    // Let's assume we loop for simplicity or add batch endpoint later
    // Just loop frontend for now
    for (const d of selectedDrafts.value) {
        await request.delete(`/drafts/${d.id}`)
    }
    ElMessage.success('批量删除成功')
    fetchDrafts()
  } catch (e) {
      console.error(e)
  }
}

const getStatusType = (status: string) => {
    switch(status) {
        case 'success': return 'success'
        case 'error': return 'danger'
        case 'analyzing': return 'warning'
        default: return 'info'
    }
}

const getStatusText = (status: string) => {
    switch(status) {
        case 'success': return '可提交'
        case 'error': return '异常'
        case 'analyzing': return '分析中'
        default: return '待处理'
    }
}

// Edit Dialog
const editVisible = ref(false)
const editForm = ref<any>({})
const categoryL2Options = computed(() => {
    const l1 = categories.value.find((c: any) => c.name === editForm.value.category_l1_name)
    return l1?.children || []
})

const handleEdit = (row: any) => {
    editForm.value = { ...row }
    editVisible.value = true
}

const handleEditCategoryL1Change = () => {
    const l2Options = categoryL2Options.value
    const current = editForm.value.category_l2_name
    if (!l2Options.find((c: any) => c.name === current)) {
        editForm.value.category_l2_name = ''
    }
}

const handleEditSubmit = async () => {
    try {
        await request.put(`/drafts/${editForm.value.id}`, editForm.value)
        ElMessage.success('保存成功')
        editVisible.value = false
        fetchDrafts()
    } catch (e) {
        console.error(e)
    }
}

const projectUpdating = ref<Record<number, boolean>>({})

const handleProjectChange = async (row: any) => {
  if (!row?.id) return
  projectUpdating.value = { ...projectUpdating.value, [row.id]: true }
  try {
    await request.put(`/drafts/${row.id}`, row)
    ElMessage.success('已更新项目')
  } catch (e) {
    ElMessage.error('更新项目失败')
  } finally {
    projectUpdating.value = { ...projectUpdating.value, [row.id]: false }
  }
}

</script>

<template>
  <div class="space-y-6">
    <div class="bg-white p-6 rounded-lg shadow">
      <h2 class="text-lg font-medium mb-4">导入账单</h2>
      <div class="flex space-x-4 mb-4">
        <el-radio-group v-model="importType">
          <el-radio-button label="alipay">支付宝</el-radio-button>
          <el-radio-button label="wechat">微信支付</el-radio-button>
        </el-radio-group>
      </div>
      
      <el-upload
        v-if="importType === 'alipay'"
        ref="uploadRef"
        class="upload-demo w-full"
        drag
        action="/api/import/alipay"
        :headers="uploadHeaders"
        :on-success="handleUploadSuccess"
        :on-error="handleUploadError"
        accept=".csv"
      >
        <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
        <div class="el-upload__text">
          拖拽文件到此处或 <em>点击上传</em> 支付宝账单 CSV
        </div>
      </el-upload>

      <el-upload
        v-else
        ref="uploadRef"
        class="upload-demo w-full"
        drag
        action="/api/import/wechat"
        :headers="uploadHeaders"
        :on-success="handleUploadSuccess"
        :on-error="handleUploadError"
        accept=".csv,.xlsx"
      >
        <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
        <div class="el-upload__text">
          拖拽文件到此处或 <em>点击上传</em> 微信账单 (CSV/XLSX)
        </div>
      </el-upload>
    </div>

    <div class="bg-white p-6 rounded-lg shadow">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-medium">草稿箱</h2>
        <div class="space-x-2">
            <el-button @click="fetchDrafts" :icon="Refresh" circle />
            <el-button 
                type="warning" 
                plain
                :disabled="selectedDrafts.length === 0" 
                :loading="batchAiLoading"
                @click="handleBatchAnalyze"
            >
                <el-icon class="mr-1"><MagicStick /></el-icon> 批量 AI 分析
            </el-button>
            <el-button type="danger" :disabled="selectedDrafts.length === 0" @click="handleBatchDelete">
                批量删除
            </el-button>
            <el-button type="primary" :disabled="selectedDrafts.length === 0" @click="handleBatchSubmit">
                批量提交
            </el-button>
        </div>
      </div>

      <el-table
        v-loading="loading"
        :data="drafts"
        @selection-change="handleSelectionChange"
        style="width: 100%"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column label="状态" width="100">
            <template #default="{ row }">
                <el-tooltip
                    v-if="row.status === 'error'"
                    placement="top"
                    effect="light"
                >
                    <template #content>
                        <div style="max-width: 300px;">
                            <p class="font-bold text-red-500 mb-1">错误原因: {{ row.error_msg }}</p>
                            <div class="text-xs text-gray-500 border-t pt-1 mt-1">
                                <p class="mb-1 font-medium">原始数据:</p>
                                <p class="break-all">{{ row.raw_data }}</p>
                            </div>
                        </div>
                    </template>
                    <el-tag :type="getStatusType(row.status)" size="small" class="cursor-pointer">
                        {{ getStatusText(row.status) }}
                    </el-tag>
                </el-tooltip>
                <el-tag v-else :type="getStatusType(row.status)" size="small">
                    {{ getStatusText(row.status) }}
                </el-tag>
            </template>
        </el-table-column>
        <el-table-column label="时间" width="160">
            <template #default="{ row }">
                {{ dayjs(row.transaction_time).format('YYYY-MM-DD HH:mm') }}
            </template>
        </el-table-column>
        <el-table-column prop="merchant" label="商家/描述" min-width="150" show-overflow-tooltip />
        <el-table-column prop="source" label="来源" width="80" />
        <el-table-column prop="remark" label="备注" min-width="150" show-overflow-tooltip />
        <el-table-column label="项目" min-width="160">
            <template #default="{ row }">
                <el-select
                  v-model="row.project_id"
                  placeholder="未关联"
                  clearable
                  class="!w-full"
                  :disabled="row.status === 'analyzing' || projectUpdating[row.id]"
                  @change="() => handleProjectChange(row)"
                >
                  <el-option v-for="p in projects" :key="p.id" :label="p.name" :value="p.id" />
                </el-select>
            </template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="100">
            <template #default="{ row }">
                ¥{{ row.amount.toFixed(2) }}
            </template>
        </el-table-column>
        <el-table-column label="分类" width="180">
            <template #default="{ row }">
                <div v-if="row.category_l1_name">
                    <el-tag size="small">{{ row.category_l1_name }}</el-tag>
                    <span class="mx-1">/</span>
                    <el-tag size="small" type="info">{{ row.category_l2_name }}</el-tag>
                </div>
                <span v-else class="text-gray-400 text-xs">未分类</span>
            </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
            <template #default="{ row }">
                <div class="flex space-x-2">
                    <el-button 
                        size="small" 
                        @click="handleEdit(row)"
                        :disabled="row.status === 'analyzing'"
                    >
                        编辑
                    </el-button>
                    <el-button 
                        size="small" 
                        type="warning" 
                        plain
                        @click="handleAnalyze(row)"
                        :loading="row.status === 'analyzing'"
                    >
                        AI
                    </el-button>
                    <el-button 
                        size="small" 
                        type="danger" 
                        link 
                        @click="handleDelete(row.id)"
                        :disabled="row.status === 'analyzing'"
                    >
                        删除
                    </el-button>
                </div>
            </template>
        </el-table-column>
      </el-table>
    </div>
    
    <!-- Edit Dialog -->
    <el-dialog v-model="editVisible" title="编辑草稿" width="500px">
        <el-form :model="editForm" label-width="80px">
            <el-form-item label="商家">
                <el-input v-model="editForm.merchant" />
            </el-form-item>
            <el-form-item label="金额">
                <el-input-number v-model="editForm.amount" :precision="2" :step="1" style="width: 100%" />
            </el-form-item>
            <el-form-item label="备注">
                <el-input v-model="editForm.remark" type="textarea" />
            </el-form-item>
            <el-form-item label="项目">
                <el-select v-model="editForm.project_id" placeholder="未关联" clearable style="width: 100%">
                  <el-option v-for="p in projects" :key="p.id" :label="p.name" :value="p.id" />
                </el-select>
            </el-form-item>
            <el-form-item label="一级分类">
                <el-select
                  v-model="editForm.category_l1_name"
                  placeholder="请选择一级分类"
                  style="width: 100%"
                  @change="handleEditCategoryL1Change"
                >
                  <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.name" />
                </el-select>
            </el-form-item>
            <el-form-item label="二级分类">
                <el-select
                  v-model="editForm.category_l2_name"
                  placeholder="请选择二级分类"
                  style="width: 100%"
                >
                  <el-option v-for="c in categoryL2Options" :key="c.id" :label="c.name" :value="c.name" />
                </el-select>
            </el-form-item>
        </el-form>
        <template #footer>
            <span class="dialog-footer">
                <el-button @click="editVisible = false">取消</el-button>
                <el-button type="primary" @click="handleEditSubmit">保存</el-button>
            </span>
        </template>
    </el-dialog>
  </div>
</template>
