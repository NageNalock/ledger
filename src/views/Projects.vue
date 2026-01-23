<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import request from '@/api/request'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { formatMoney } from '@/utils/format'

const router = useRouter()
const projects = ref<any[]>([])
const summaries = ref<Record<string, any>>({})
const loading = ref(false)

const skeletonItems = computed(() => Array.from({ length: 6 }).map((_, i) => ({ id: i })))

const formatCreatedAt = (value: any) => {
  if (!value) return ''
  const d = dayjs(value)
  if (!d.isValid()) return String(value)
  return d.format('YYYY-MM-DD HH:mm')
}

const getSummaryAmount = (projectId: number) => {
  return summaries.value[String(projectId)]?.total_amount ?? null
}

const getBudgetUsage = (project: any) => {
  const total = getSummaryAmount(project.id)
  const limit = Number(project.budget_limit || 0)
  if (!project.budget_enabled || limit <= 0 || total === null) {
    return null
  }
  const percent = Math.min(100, Math.round((total / limit) * 100))
  const status = percent >= 100 ? 'danger' : percent >= 80 ? 'warning' : 'normal'
  return { total, limit, percent, status }
}

const getBudgetBarClass = (status: string) => {
  if (status === 'danger') return 'bg-red-400'
  if (status === 'warning') return 'bg-amber-400'
  return 'bg-emerald-400'
}

const getBudgetTextClass = (status: string) => {
  if (status === 'danger') return 'text-red-500'
  if (status === 'warning') return 'text-amber-500'
  return 'text-emerald-500'
}

const fetchProjects = async () => {
  loading.value = true
  try {
    const res: any = await request.get('/projects')
    projects.value = res
    summaries.value = {}
    await Promise.allSettled(
      projects.value.map(async (p: any) => {
        const summary: any = await request.get(`/projects/${p.id}/summary`)
        summaries.value = {
          ...summaries.value,
          [String(p.id)]: summary
        }
      })
    )
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const handleAdd = async () => {
  try {
    const { value } = await ElMessageBox.prompt('请输入项目名称', '新建项目', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
    if (value) {
      await request.post('/projects', { name: value })
      ElMessage.success('添加成功')
      fetchProjects()
    }
  } catch (e) {
    return
  }
}

const handleEdit = async (row: any) => {
  try {
    const { value } = await ElMessageBox.prompt('请输入新的项目名称', '编辑项目', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputValue: row.name,
    })
    if (value && value !== row.name) {
      await request.put(`/projects/${row.id}`, { name: value, description: row.description })
      ElMessage.success('修改成功')
      fetchProjects()
    }
  } catch (e) {
    return
  }
}

const handleDelete = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定删除项目 "${row.name}" 吗？关联的账单将解除与该项目的关联。`, '提示', {
      type: 'warning',
    })
    await request.delete(`/projects/${row.id}`)
    ElMessage.success('删除成功')
    fetchProjects()
  } catch (e) {
    return
  }
}

const handleDetail = (row: any) => {
  router.push(`/projects/${row.id}`)
}

onMounted(() => {
  fetchProjects()
})
</script>

<template>
  <div class="space-y-6">
    <div class="bg-white p-6 rounded-2xl shadow-card border border-gray-100 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
      <div>
        <div class="text-xs text-gray-400 font-bold">Project Center</div>
        <h2 class="text-2xl font-extrabold text-gray-800 tracking-tight">项目管理</h2>
        <div class="text-sm text-gray-500 font-bold mt-1">按项目查看预算与支出概览，点击卡片进入详情。</div>
      </div>
      <div class="flex items-center gap-3">
        <el-button type="primary" color="#FF6B35" @click="handleAdd">
          <el-icon class="mr-1"><Plus /></el-icon> 新建项目
        </el-button>
      </div>
    </div>

    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="item in skeletonItems"
        :key="item.id"
        class="bg-white rounded-2xl border border-gray-100 p-5 shadow-sm animate-pulse"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-3">
            <div class="h-11 w-11 rounded-2xl bg-gray-100"></div>
            <div>
              <div class="h-4 w-40 bg-gray-100 rounded"></div>
              <div class="h-3 w-24 bg-gray-100 rounded mt-2"></div>
            </div>
          </div>
          <div class="h-8 w-8 bg-gray-100 rounded-xl"></div>
        </div>
        <div class="mt-5 grid grid-cols-2 gap-3">
          <div class="h-8 bg-gray-100 rounded-xl"></div>
          <div class="h-8 bg-gray-100 rounded-xl"></div>
        </div>
        <div class="mt-4 h-9 bg-gray-100 rounded-xl"></div>
      </div>
    </div>

    <div v-else-if="projects.length === 0" class="bg-white rounded-2xl shadow-card border border-gray-100 p-10">
      <div class="max-w-md mx-auto text-center">
        <div class="mx-auto h-16 w-16 rounded-3xl bg-gradient-to-tr from-primary/15 to-accent/15 flex items-center justify-center border border-primary/10">
          <el-icon size="28" class="text-primary"><FolderOpened /></el-icon>
        </div>
        <div class="mt-4 text-lg font-extrabold text-gray-800">暂无项目</div>
        <div class="mt-2 text-sm text-gray-500 font-bold">你还没有可查看的项目，可先点击右上角创建。</div>
      </div>
    </div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="p in projects"
        :key="p.id"
        class="group bg-white rounded-2xl border border-gray-100 p-5 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg hover:shadow-primary/10 hover:border-primary/20 focus-within:ring-2 focus-within:ring-primary/30"
        role="button"
        tabindex="0"
        @click="handleDetail(p)"
        @keydown.enter.prevent="handleDetail(p)"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-3 min-w-0">
            <div class="h-11 w-11 rounded-2xl bg-gradient-to-tr from-primary/25 to-accent/25 border border-white shadow-sm flex items-center justify-center">
              <el-icon size="22" class="text-primary"><Collection /></el-icon>
            </div>
            <div class="min-w-0">
              <div class="text-base font-extrabold text-gray-800 truncate">{{ p.name }}</div>
              <div class="text-xs text-gray-400 font-bold mt-1">创建于 {{ formatCreatedAt(p.created_at) }}</div>
            </div>
          </div>
          <el-button circle class="opacity-90 group-hover:opacity-100" @click.stop="handleDetail(p)">
            <el-icon><ArrowRight /></el-icon>
          </el-button>
        </div>

        <div class="mt-5 grid grid-cols-2 gap-3">
          <div class="rounded-xl bg-gray-50 border border-gray-100 px-3 py-2">
            <div class="text-[11px] text-gray-400 font-extrabold">预算状态</div>
            <div class="mt-1">
              <span
                v-if="p.budget_enabled && Number(p.budget_limit) > 0"
                class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-extrabold bg-primary/10 text-primary"
              >
                <el-icon size="14"><Coin /></el-icon>
                ¥{{ Number(p.budget_limit).toFixed(2) }}
              </span>
              <span
                v-else
                class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-extrabold bg-accent/10 text-accent"
              >
                <el-icon size="14"><CircleCheckFilled /></el-icon>
                无上限
              </span>
            </div>
            <div class="mt-2 text-[11px] text-gray-400 font-extrabold">已支出</div>
            <div class="text-sm font-extrabold text-gray-800">
              <span v-if="getSummaryAmount(p.id) === null">···</span>
              <span v-else>{{ formatMoney(getSummaryAmount(p.id)) }}</span>
            </div>
            <div v-if="getBudgetUsage(p)" class="mt-2">
              <div class="flex items-center justify-between text-[11px] font-extrabold">
                <span class="text-gray-400">预算使用</span>
                <span :class="getBudgetTextClass(getBudgetUsage(p).status)">
                  {{ getBudgetUsage(p).percent }}%
                </span>
              </div>
              <div class="mt-1 h-1.5 rounded-full bg-gray-200 overflow-hidden">
                <div
                  class="h-full transition-all duration-300"
                  :class="getBudgetBarClass(getBudgetUsage(p).status)"
                  :style="{ width: getBudgetUsage(p).percent + '%' }"
                ></div>
              </div>
              <div class="mt-1 text-[11px] text-gray-400 font-bold">
                {{ formatMoney(getBudgetUsage(p).total) }} / {{ formatMoney(getBudgetUsage(p).limit) }}
              </div>
            </div>
          </div>

          <div class="rounded-xl bg-gray-50 border border-gray-100 px-3 py-2">
            <div class="text-[11px] text-gray-400 font-extrabold">快捷操作</div>
            <div class="mt-2 flex items-center gap-2">
              <el-button size="small" class="!rounded-xl" @click.stop="handleEdit(p)">
                <el-icon class="mr-1"><Edit /></el-icon> 编辑
              </el-button>
              <el-button size="small" type="danger" plain class="!rounded-xl" @click.stop="handleDelete(p)">
                <el-icon class="mr-1"><Delete /></el-icon> 删除
              </el-button>
            </div>
          </div>
        </div>

        <div class="mt-4 rounded-xl bg-gradient-to-r from-primary/10 via-white to-accent/10 border border-primary/10 px-3 py-2 flex items-center justify-between">
          <div class="text-xs text-gray-600 font-extrabold">进入项目详情</div>
          <div class="text-xs text-primary font-extrabold">查看流水 / 预算 / 趋势</div>
        </div>
      </div>
    </div>
  </div>
</template>
