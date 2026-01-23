<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import request from '@/api/request'
import AiConsumptionAnalysisPanel from '@/components/AiConsumptionAnalysisPanel.vue'
import { Sparkles, Wand2, CalendarDays, FolderKanban } from 'lucide-vue-next'

type Project = {
  id: number
  name: string
}

const router = useRouter()

const dateRange = ref<[string, string] | null>(null)
const projectId = ref<string>('')
const userRequirement = ref('')

const projectsLoading = ref(false)
const projects = ref<Project[]>([])

const startDate = computed(() => dateRange.value?.[0] || '')
const endDate = computed(() => dateRange.value?.[1] || '')

const panelRef = ref<any>(null)

const unwrap = <T,>(v: any): T => (v && typeof v === 'object' && 'value' in v ? v.value : v)
const canGenerate = computed(() => Boolean(unwrap(panelRef.value?.canAnalyze)))
const generating = computed(() => Boolean(unwrap(panelRef.value?.loading)) || Boolean(unwrap(panelRef.value?.analysisInProgress)))
const generateLabel = computed(() => String(unwrap(panelRef.value?.analyzeButtonLabel) || '生成'))

const handleGenerate = () => {
  panelRef.value?.runAnalysis?.()
}

const fetchProjects = async () => {
  projectsLoading.value = true
  try {
    const res: any = await request.get('/projects', { timeout: 20000 })
    projects.value = Array.isArray(res) ? res : []
  } finally {
    projectsLoading.value = false
  }
}

onMounted(() => {
  fetchProjects()
})

const toReports = () => {
  router.push('/reports')
}
</script>

<template>
  <div class="space-y-6">
    <div class="bg-white p-6 rounded-2xl shadow-card border border-gray-100">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="flex items-center gap-2">
            <div class="h-10 w-10 rounded-2xl bg-gradient-to-br from-primary/15 to-accent/15 ring-1 ring-primary/10 flex items-center justify-center">
              <Sparkles class="w-5 h-5 text-primary" />
            </div>
            <h1 class="text-2xl font-black text-gray-900 tracking-tight">AI消费顾问</h1>
          </div>
          <p class="text-sm text-gray-500 font-medium mt-2">基于所选范围流水生成洞察与建议</p>
        </div>
        <el-button round class="!px-5 !font-bold" @click="toReports">
          返回报表
        </el-button>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-[340px_1fr] gap-6">
      <div class="bg-white p-6 rounded-2xl shadow-card border border-gray-100 lg:sticky lg:top-24 h-fit">
        <h3 class="text-lg font-bold text-gray-700 flex items-center gap-2 mb-4">
          <Wand2 class="w-5 h-5 text-primary" />
          生成设置
        </h3>

        <div class="space-y-4">
          <div>
            <div class="text-xs font-bold text-gray-400 uppercase tracking-wider mb-2 flex items-center gap-2">
              <CalendarDays class="w-4 h-4" />
              时间范围
            </div>
            <el-date-picker
              v-model="dateRange"
              type="daterange"
              range-separator="到"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              class="!w-full"
            />
          </div>

          <div>
            <div class="text-xs font-bold text-gray-400 uppercase tracking-wider mb-2 flex items-center gap-2">
              <FolderKanban class="w-4 h-4" />
              项目（可选）
            </div>
            <el-select v-model="projectId" filterable clearable placeholder="全部项目" class="!w-full" :loading="projectsLoading">
              <el-option v-for="p in projects" :key="p.id" :label="p.name" :value="String(p.id)" />
            </el-select>
          </div>

          <div>
            <div class="text-xs font-bold text-gray-400 uppercase tracking-wider mb-2">额外要求（可选）</div>
            <el-input v-model="userRequirement" type="textarea" :rows="3" placeholder="例如：想省钱、准备旅行、控制外卖..." class="!rounded-2xl" />
          </div>

          <div class="pt-2">
            <el-button
              type="primary"
              color="#8B5CF6"
              class="!w-full !h-11 !rounded-2xl !font-black shadow-md hover:!shadow-lg transition-shadow"
              :loading="generating"
              :disabled="!canGenerate || generating"
              @click="handleGenerate"
            >
              {{ generateLabel }}
            </el-button>
            <div v-if="!canGenerate" class="mt-2 text-xs font-bold text-gray-400">先选择时间范围即可生成分析</div>
          </div>
        </div>
      </div>

      <div>
        <AiConsumptionAnalysisPanel
          ref="panelRef"
          :start-date="startDate"
          :end-date="endDate"
          trend-type="day"
          :project-id="projectId"
          :user-requirement="userRequirement"
        />
      </div>
    </div>
  </div>
</template>
