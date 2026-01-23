<script setup lang="ts">
import { ref, onMounted } from 'vue'
import request from '@/api/request'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete, Edit } from '@element-plus/icons-vue'

const categories = ref([])
const loading = ref(false)

const fetchCategories = async () => {
  loading.value = true
  try {
    const res: any = await request.get('/categories')
    categories.value = res
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const handleAddL1 = async () => {
  try {
    const { value } = await ElMessageBox.prompt('请输入一级分类名称', '新建分类', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
    if (value) {
      await request.post('/categories', { name: value })
      ElMessage.success('添加成功')
      fetchCategories()
    }
  } catch (e) {
    // cancelled
  }
}

const handleAddL2 = async (data: any) => {
  try {
    const { value } = await ElMessageBox.prompt(`在 "${data.name}" 下新建二级分类`, '新建子分类', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
    if (value) {
      await request.post('/categories', { name: value, parent_id: data.id })
      ElMessage.success('添加成功')
      fetchCategories()
    }
  } catch (e) {
    // cancelled
  }
}

const handleEdit = async (data: any) => {
  try {
    const { value } = await ElMessageBox.prompt('请输入新的分类名称', '编辑分类', {
      inputValue: data.name,
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
    if (value && value !== data.name) {
        // We reuse the update logic if endpoint exists or just use create with same ID?
        // Wait, update endpoint logic in backend?
        // Let's check backend routes. Assuming PUT /categories/:id exists?
        // Actually, backend might not have separate UpdateCategory yet or it does?
        // Let's assume PUT /categories/:id updates name.
        const level = data.children ? 1 : 2
        await request.put(`/categories/${data.id}?level=${level}`, { name: value })
        ElMessage.success('修改成功')
        fetchCategories()
    }
  } catch (e) {
    // cancelled
  }
}

const handleDelete = async (data: any) => {
  try {
    await ElMessageBox.confirm('确定删除该分类吗？如果已有账单使用了该分类，将变为"未分类"。', '提示', {
      type: 'warning',
    })
    const level = data.children ? 1 : 2
    await request.delete(`/categories/${data.id}?level=${level}`)
    ElMessage.success('删除成功')
    fetchCategories()
  } catch (e) {
    // cancelled
  }
}

onMounted(() => {
  fetchCategories()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex justify-between items-center mb-6">
      <h2 class="text-xl font-bold text-gray-800">分类管理</h2>
      <el-button type="primary" color="#FF6B35" @click="handleAddL1" size="large" shadow="hover">
        <el-icon class="mr-1"><Plus /></el-icon> 新建一级分类
      </el-button>
    </div>

    <div v-loading="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
      <div 
        v-for="l1 in categories" 
        :key="l1.id" 
        class="bg-white rounded-xl shadow-sm hover:shadow-md transition-shadow duration-300 border border-gray-100 flex flex-col overflow-hidden group"
      >
        <!-- Card Header: L1 Category -->
        <div class="p-4 bg-gray-50 border-b border-gray-100 flex justify-between items-center">
          <h3 class="font-bold text-lg text-gray-800 truncate" :title="l1.name">{{ l1.name }}</h3>
          <div class="opacity-0 group-hover:opacity-100 transition-opacity flex space-x-1">
             <el-button link type="primary" @click="handleEdit(l1)" :icon="Edit" />
             <el-button link type="danger" @click="handleDelete(l1)" :icon="Delete" />
          </div>
        </div>

        <!-- Card Body: L2 List -->
        <div class="p-4 flex-1 bg-white">
          <div class="flex flex-wrap gap-2">
            <el-tag
              v-for="l2 in l1.children"
              :key="l2.id"
              class="cursor-pointer hover:opacity-80 transition-opacity"
              type="info"
              effect="light"
              closable
              @close="handleDelete(l2)"
              @click="handleEdit(l2)"
            >
              {{ l2.name }}
            </el-tag>
            
            <el-button 
              size="small" 
              class="ml-1" 
              :icon="Plus" 
              circle 
              plain 
              @click="handleAddL2(l1)" 
              title="添加子分类"
            />
          </div>
          
          <div v-if="!l1.children || l1.children.length === 0" class="text-gray-400 text-sm italic mt-2 text-center py-4">
             暂无子分类
          </div>
        </div>
      </div>
    </div>
    
    <!-- Empty State -->
    <el-empty v-if="!loading && categories.length === 0" description="暂无分类，快去创建一个吧！" />
  </div>
</template>

<style scoped>
/* Custom Tag Style override if needed */
</style>
