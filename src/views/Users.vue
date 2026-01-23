<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import request from '@/api/request'
import { ElMessage, ElMessageBox } from 'element-plus'

const users = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const formRef = ref()

const form = reactive({
  username: '',
  password: '',
})

const fetchUsers = async () => {
  loading.value = true
  try {
    const res: any = await request.get('/users')
    users.value = res
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const handleAdd = () => {
  form.username = ''
  form.password = ''
  dialogVisible.value = true
}

const handleDelete = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定删除用户 "${row.username}" 吗？`, '提示', {
      type: 'warning',
    })
    await request.delete(`/users/${row.id}`)
    ElMessage.success('删除成功')
    fetchUsers()
  } catch (e) {
    // cancelled
  }
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid: boolean) => {
    if (valid) {
      try {
        await request.post('/users', form)
        ElMessage.success('用户创建成功')
        dialogVisible.value = false
        fetchUsers()
      } catch (e) {
        // handled
      }
    }
  })
}

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

onMounted(() => {
  fetchUsers()
})
</script>

<template>
  <div class="space-y-6">
    <div class="bg-white p-4 rounded-lg shadow flex justify-between items-center">
      <h2 class="text-lg font-bold text-gray-700">用户管理</h2>
      <el-button type="primary" color="#FF6B35" @click="handleAdd">
        <el-icon class="mr-1"><Plus /></el-icon> 新建用户
      </el-button>
    </div>

    <div class="bg-white rounded-lg shadow overflow-hidden">
      <el-table :data="users" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="username" label="用户名" />
        <el-table-column prop="role" label="角色">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : 'info'">{{ row.role }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" />
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button
              v-if="row.role !== 'admin'"
              link
              type="danger"
              @click="handleDelete(row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog
      v-model="dialogVisible"
      title="新建用户"
      width="400px"
    >
      <el-form :model="form" ref="formRef" :rules="rules" label-width="80px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit">确定</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>
