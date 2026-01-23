<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import request from '@/api/request'
import { ElMessage } from 'element-plus'
import dayjs from 'dayjs'

const props = defineProps<{
  visible: boolean
  editData?: any
  defaultProjectId?: number
}>()

const emit = defineEmits(['update:visible', 'success'])

const formRef = ref()
const loading = ref(false)
const aiLoading = ref(false)

const form = reactive({
  id: undefined,
  transaction_time: dayjs().format('YYYY-MM-DDTHH:mm:ssZ'),
  merchant: '',
  amount: undefined,
  category_l1_id: '',
  category_l2_id: '',
  source: '无',
  remark: '',
  project_id: undefined as number | undefined,
})

const categories = ref<any[]>([])
const projects = ref<any[]>([])

// Fetch categories on mount
const fetchCategories = async () => {
  try {
    const res: any = await request.get('/categories')
    categories.value = res
  } catch (e) {
    console.error(e)
  }
}

const fetchProjects = async () => {
  try {
    const res: any = await request.get('/projects')
    projects.value = res
  } catch (e) {
    console.error(e)
  }
}

fetchCategories()
fetchProjects()

watch(() => props.visible, (val) => {
  if (val) {
    if (props.editData) {
      // Edit Mode
      const data = props.editData
      form.id = data.id
      form.transaction_time = dayjs(data.transaction_time).format('YYYY-MM-DDTHH:mm:ssZ')
      form.merchant = data.merchant
      form.amount = data.amount
      form.category_l1_id = data.category_l1_id
      form.category_l2_id = data.category_l2_id
      form.source = data.source || '无'
      form.remark = data.remark
      form.project_id = data.project_id || undefined
    } else {
      // Create Mode - Reset
      form.id = undefined
      form.transaction_time = dayjs().format('YYYY-MM-DDTHH:mm:ssZ')
      form.merchant = ''
      form.amount = undefined
      form.category_l1_id = ''
      form.category_l2_id = ''
      form.source = '无'
      form.remark = ''
      form.project_id = props.defaultProjectId
    }
  }
})

// Computed for L2 options based on L1 selection
const l2Options = computed(() => {
  const l1 = categories.value.find((c: any) => c.id === form.category_l1_id)
  return l1 ? l1.children : []
})

const handleAIClassify = async () => {
  if (!form.amount) {
    ElMessage.warning('请先填写金额')
    return
  }
  
  aiLoading.value = true
  try {
    const res: any = await request.post('/ai/classify', {
      merchant: form.merchant,
      amount: Number(form.amount),
      transaction_time: form.transaction_time,
      remark: form.remark,
    })
    
    // Find IDs based on names returned by AI
    const l1 = categories.value.find((c: any) => c.name === res.category_l1)
    if (l1) {
      form.category_l1_id = l1.id
      const l2 = l1.children.find((c: any) => c.name === res.category_l2)
      if (l2) {
        form.category_l2_id = l2.id
      }
      ElMessage.success(`AI分类成功: ${res.category_l1} - ${res.category_l2}`)
    } else {
      ElMessage.warning(`AI建议分类 "${res.category_l1}" 未在系统中找到`)
    }
  } catch (e) {
    ElMessage.error('AI分类失败')
  } finally {
    aiLoading.value = false
  }
}

const handleSubmit = async (continuous = false) => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid: boolean) => {
    if (valid) {
      loading.value = true
      try {
        if (form.id) {
          await request.put(`/transactions/${form.id}`, form)
          ElMessage.success('更新成功')
        } else {
          await request.post('/transactions', form)
          ElMessage.success('记账成功')
        }
        emit('success')
        if (!continuous) {
          emit('update:visible', false)
        } else {
          // Reset form but keep time/source maybe?
          form.merchant = ''
          form.amount = undefined
          form.remark = ''
          form.category_l1_id = ''
          form.category_l2_id = ''
          form.project_id = props.defaultProjectId
        }
      } catch (e) {
        // handled
      } finally {
        loading.value = false
      }
    }
  })
}

const handleClose = () => {
  emit('update:visible', false)
}

const rules = {
  transaction_time: [{ required: true, message: '请选择时间', trigger: 'change' }],
  amount: [{ required: true, message: '请输入金额', trigger: 'blur' }],
  category_l1_id: [{ required: true, message: '请选择一级分类', trigger: 'change' }],
  category_l2_id: [{ required: true, message: '请选择二级分类', trigger: 'change' }],
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="记一笔"
    width="500px"
    @close="handleClose"
    destroy-on-close
  >
    <el-form :model="form" ref="formRef" :rules="rules" label-width="80px">
      <el-form-item label="时间" prop="transaction_time">
        <el-date-picker
          v-model="form.transaction_time"
          type="datetime"
          placeholder="选择日期时间"
          format="YYYY-MM-DD HH:mm:ss"
          value-format="YYYY-MM-DDTHH:mm:ssZ"
          style="width: 100%"
        />
      </el-form-item>
      
      <el-form-item label="商家" prop="merchant">
        <el-input v-model="form.merchant" placeholder="例如: 7-11便利店" />
      </el-form-item>
      
      <el-form-item label="金额" prop="amount">
        <el-input-number v-model="form.amount" :precision="2" :step="1" :min="0" style="width: 100%" />
      </el-form-item>
      
      <el-form-item label="备注" prop="remark">
        <el-input v-model="form.remark" type="textarea" :rows="2" />
      </el-form-item>

      <div class="flex justify-end mb-4">
        <el-button type="primary" plain size="small" :loading="aiLoading" @click="handleAIClassify">
          <el-icon class="mr-1"><MagicStick /></el-icon> AI 智能分类
        </el-button>
      </div>
      
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item label="一级分类" prop="category_l1_id" label-width="100px">
            <el-select v-model="form.category_l1_id" placeholder="请选择" style="width: 100%">
              <el-option
                v-for="item in categories"
                :key="item.id"
                :label="item.name"
                :value="item.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="二级分类" prop="category_l2_id" label-width="100px">
            <el-select v-model="form.category_l2_id" placeholder="二级分类" style="width: 100%" :disabled="!form.category_l1_id">
              <el-option
                v-for="item in l2Options"
                :key="item.id"
                :label="item.name"
                :value="item.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>
      
      <el-form-item label="来源">
        <el-radio-group v-model="form.source">
          <el-radio label="无">无</el-radio>
          <el-radio label="微信">微信</el-radio>
          <el-radio label="支付宝">支付宝</el-radio>
          <el-radio label="银行卡">银行卡</el-radio>
          <el-radio label="现金">现金</el-radio>
        </el-radio-group>
      </el-form-item>

      <el-form-item label="关联项目">
        <el-select v-model="form.project_id" placeholder="选择项目 (可选)" style="width: 100%" clearable>
          <el-option
            v-for="item in projects"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </el-select>
      </el-form-item>
    </el-form>
    
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="handleClose">取消</el-button>
        <el-button type="primary" @click="handleSubmit(false)" :loading="loading">提交</el-button>
        <el-button v-if="!form.id" type="success" @click="handleSubmit(true)" :loading="loading">提交并继续</el-button>
      </span>
    </template>
  </el-dialog>
</template>
