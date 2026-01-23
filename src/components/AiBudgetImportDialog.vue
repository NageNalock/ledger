<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import request from '@/api/request'
import { ElMessage } from 'element-plus'
import { Plus, Sparkles, Trash2 } from 'lucide-vue-next'

type CategoryL2 = { id: number; parent_id: number; name: string }
type CategoryTree = { id: number; name: string; children: CategoryL2[] }

type L2SplitDraft = { category_l2_id: string; amount: number }
type L1SplitDraft = { category_l1_id: string; amount: number; l2: L2SplitDraft[]; open: boolean }

const props = defineProps<{
  modelValue: boolean
  analysisId: string
  defaultMonth: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'saved'): void
}>()

const open = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const loading = ref(false)
const saving = ref(false)

const month = ref('')
const categories = ref<CategoryTree[]>([])

const draftTotal = ref<number>(0)
const draftSplitL1 = ref<L1SplitDraft[]>([])

const existingBudgets = ref<{ total: number; active?: boolean } | null>(null)

const allocatedSum = computed(() => draftSplitL1.value.reduce((acc, l) => acc + (Number(l.amount) || 0), 0))
const unallocatedBudget = computed(() => Math.max(0, (Number(draftTotal.value) || 0) - allocatedSum.value))

const l2OptionsForL1 = (l1Id: string) => {
  const l1 = categories.value.find((c) => String(c.id) === String(l1Id))
  return l1?.children || []
}

const l2AllocatedSum = (row: L1SplitDraft) => row.l2.reduce((acc, x) => acc + (Number(x.amount) || 0), 0)
const l2Unallocated = (row: L1SplitDraft) => Math.max(0, (Number(row.amount) || 0) - l2AllocatedSum(row))

const fetchCategories = async () => {
  if (categories.value.length) return
  try {
    const res: any = await request.get('/categories', { timeout: 20000 })
    categories.value = Array.isArray(res)
      ? res.map((c: any) => ({
          id: Number(c.id),
          name: String(c.name),
          children: Array.isArray(c.children)
            ? c.children.map((x: any) => ({ id: Number(x.id), parent_id: Number(x.parent_id), name: String(x.name) }))
            : []
        }))
      : []
  } catch {
    categories.value = []
  }
}

const checkExisting = async () => {
  existingBudgets.value = null
  const m = String(month.value || '').trim()
  if (!m) return
  try {
    const res: any = await request.get('/budgets/history', { params: { month: m, limit: 1, offset: 0 }, timeout: 20000 })
    const total = Number(res?.total || 0)
    const items = Array.isArray(res?.items) ? res.items : []
    const active = items.some((x: any) => String(x?.status) === 'active')
    existingBudgets.value = { total, active }
  } catch {
    existingBudgets.value = null
  }
}

const applyDraft = (draft: any) => {
  draftTotal.value = Number(draft?.total_amount || 0)
  const lines = Array.isArray(draft?.lines) ? draft.lines : []
  const l1Map = new Map<string, L1SplitDraft>()

  for (const l of lines) {
    const l1 = String(l?.category_l1_id || '').trim()
    const l2 = String(l?.category_l2_id || '').trim()
    const amt = Number(l?.amount || 0)
    if (!l1 || amt <= 0) continue
    if (!l2) {
      l1Map.set(l1, { category_l1_id: l1, amount: amt, l2: [], open: false })
      continue
    }
    const parent = l1Map.get(l1) || { category_l1_id: l1, amount: 0, l2: [], open: false }
    parent.l2.push({ category_l2_id: l2, amount: amt })
    l1Map.set(l1, parent)
  }

  draftSplitL1.value = Array.from(l1Map.values())
}

const generateDraft = async () => {
  if (!props.analysisId) return
  loading.value = true
  try {
    const res: any = await request.post(
      '/ai/budget-draft',
      { analysis_id: props.analysisId, month: month.value },
      { timeout: 30000 }
    )
    applyDraft(res)
    if (res?.month) month.value = String(res.month)
  } catch (e: any) {
    ElMessage.error(String(e?.response?.data?.error || '生成预算草稿失败'))
  } finally {
    loading.value = false
  }
}

const addL1 = () => {
  draftSplitL1.value.push({ category_l1_id: '', amount: Math.min(100, unallocatedBudget.value || 0), l2: [], open: false })
}

const removeL1 = (idx: number) => {
  draftSplitL1.value.splice(idx, 1)
}

const toggleL2 = (idx: number) => {
  draftSplitL1.value[idx].open = !draftSplitL1.value[idx].open
}

const addL2 = (idx: number) => {
  draftSplitL1.value[idx].open = true
  draftSplitL1.value[idx].l2.push({ category_l2_id: '', amount: Math.min(50, l2Unallocated(draftSplitL1.value[idx]) || 0) })
}

const removeL2 = (idx: number, j: number) => {
  draftSplitL1.value[idx].l2.splice(j, 1)
}

const buildLinesPayload = () => {
  const lines: Array<{ category_l1_id: number; category_l2_id?: number; amount: number }> = []
  const seenL1 = new Set<string>()
  const seenL2 = new Set<string>()

  for (const row of draftSplitL1.value) {
    const l1id = String(row.category_l1_id || '').trim()
    const l1amt = Number(row.amount) || 0
    if (!l1id || l1amt <= 0) throw new Error('一级分类拆分需要选择分类并填写金额')
    if (seenL1.has(l1id)) throw new Error('一级分类不能重复')
    seenL1.add(l1id)
    lines.push({ category_l1_id: Number(l1id), amount: l1amt })

    const l2sum = l2AllocatedSum(row)
    if (l2sum - l1amt > 0.0001) throw new Error('二级分类拆分金额之和不能超过该一级分类预算')

    for (const c of row.l2) {
      const l2id = String(c.category_l2_id || '').trim()
      const l2amt = Number(c.amount) || 0
      if (!l2id || l2amt <= 0) throw new Error('二级分类拆分需要选择分类并填写金额')
      const key = `${l1id}:${l2id}`
      if (seenL2.has(key)) throw new Error('二级分类不能重复')
      seenL2.add(key)
      lines.push({ category_l1_id: Number(l1id), category_l2_id: Number(l2id), amount: l2amt })
    }
  }

  const amt = Number(draftTotal.value) || 0
  const l1Sum = draftSplitL1.value.reduce((acc, r) => acc + (Number(r.amount) || 0), 0)
  if (l1Sum - amt > 0.0001) throw new Error('一级分类拆分金额之和不能超过总预算')

  return lines
}

const save = async () => {
  const amt = Number(draftTotal.value) || 0
  if (amt <= 0) {
    ElMessage.error('请输入大于 0 的预算金额')
    return
  }
  const m = String(month.value || '').trim()
  if (!m) {
    ElMessage.error('请选择预算月份')
    return
  }

  let lines: any[] = []
  try {
    lines = draftSplitL1.value.length ? buildLinesPayload() : []
  } catch (e: any) {
    ElMessage.error(String(e?.message || '拆分校验失败'))
    return
  }

  saving.value = true
  try {
    await request.put('/budgets/active', { month: m, status: 'active', total_amount: amt, lines }, { timeout: 30000 })
    ElMessage.success('预算已设置')
    open.value = false
    emit('saved')
  } catch (e: any) {
    ElMessage.error(String(e?.response?.data?.error || '保存失败'))
  } finally {
    saving.value = false
  }
}

const reset = () => {
  month.value = String(props.defaultMonth || '').trim()
  draftTotal.value = 0
  draftSplitL1.value = []
  existingBudgets.value = null
}

watch(
  () => open.value,
  async (v) => {
    if (!v) return
    reset()
    await fetchCategories()
    await generateDraft()
    await checkExisting()
  }
)

watch(
  () => month.value,
  () => {
    checkExisting()
  }
)

onMounted(() => {
  if (props.modelValue) {
    fetchCategories()
  }
})
</script>

<template>
  <el-dialog
    v-model="open"
    title="带入到预算"
    width="720px"
    align-center
    :close-on-click-modal="!saving"
    :close-on-press-escape="!saving"
    :show-close="!saving"
  >
    <div v-loading="loading" class="space-y-5">
      <el-alert
        v-if="existingBudgets && existingBudgets.total > 0"
        :type="existingBudgets.active ? 'warning' : 'info'"
        show-icon
        :closable="false"
        class="!rounded-2xl"
      >
        <template #title>
          <span class="font-black">该月已存在预算（{{ existingBudgets.total }} 条）</span>
        </template>
        <template #default>
          <span class="font-bold">继续保存会覆盖该月的生效预算，请确认。</span>
        </template>
      </el-alert>

      <div class="rounded-2xl border border-[#F1D6D0] bg-[#FFF8F5] p-4">
        <div class="flex items-center justify-between gap-3">
          <div>
            <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">预算月份</div>
            <div class="mt-2 flex items-center gap-2">
              <el-date-picker v-model="month" type="month" value-format="YYYY-MM" class="!w-[160px]" :disabled="saving" />
              <el-button round type="primary" color="#7CD6C9" plain class="!px-4 !font-black" :disabled="saving" @click="generateDraft">
                <template #icon><Sparkles class="w-4 h-4" /></template>
                重新生成
              </el-button>
            </div>
          </div>
          <div class="text-right">
            <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">总未分配预算</div>
            <div class="mt-1 text-sm font-black" :class="unallocatedBudget < 0 ? 'text-rose-700' : 'text-[#5A3B32]'">{{ unallocatedBudget.toFixed(2) }}</div>
          </div>
        </div>
      </div>

      <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
        <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">预算金额</div>
        <div class="mt-2">
          <el-input-number v-model="draftTotal" :min="0" :step="50" class="!w-full" controls-position="right" :disabled="saving" />
        </div>
      </div>

      <div class="rounded-2xl border border-[#F1D6D0] bg-white p-4">
        <div class="flex items-center justify-between gap-2">
          <div>
            <div class="text-xs font-extrabold text-[#9A7A71] uppercase tracking-wider">预算拆分（可选）</div>
            <div class="mt-1 text-xs font-bold text-[#8E6C63]">AI 已按一级分类生成草稿，你可以继续细化到二级</div>
          </div>
          <el-button round type="primary" color="#FF8A65" plain class="!px-4 !font-black" :disabled="saving" @click="addL1">
            <template #icon><Plus class="w-4 h-4" /></template>
            一级
          </el-button>
        </div>

        <div v-if="draftSplitL1.length" class="mt-4 space-y-3">
          <div v-for="(row, idx) in draftSplitL1" :key="idx" class="rounded-2xl border border-[#F1D6D0] bg-[#FFF8F5] p-3">
            <div class="grid grid-cols-[1fr_130px_32px] gap-2 items-center">
              <el-select
                v-model="row.category_l1_id"
                filterable
                placeholder="选择一级分类"
                class="!w-full"
                :disabled="saving"
                @change="() => { row.l2 = []; row.open = false }"
              >
                <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="String(c.id)" />
              </el-select>
              <el-input-number v-model="row.amount" :min="0" :step="50" class="!w-full" controls-position="right" :disabled="saving" />
              <el-button circle plain type="danger" :disabled="saving" @click="removeL1(idx)">
                <Trash2 class="w-4 h-4" />
              </el-button>
            </div>

            <div class="mt-2 flex items-center justify-between">
              <div class="text-xs font-bold text-[#8E6C63]">二级已拆分 {{ l2AllocatedSum(row).toFixed(2) }} · 未分配预算 {{ l2Unallocated(row).toFixed(2) }}</div>
              <div class="flex items-center gap-2">
                <el-button round type="primary" color="#7CD6C9" plain class="!px-3 !font-black" :disabled="saving || !row.category_l1_id" @click="addL2(idx)">
                  <template #icon><Plus class="w-4 h-4" /></template>
                  二级
                </el-button>
                <el-button round class="!px-3 !font-black" :disabled="saving || !row.category_l1_id" @click="toggleL2(idx)">
                  {{ row.open ? '收起' : '展开' }}
                </el-button>
              </div>
            </div>

            <div v-if="row.open" class="mt-3 space-y-2">
              <div v-if="!row.category_l1_id" class="text-xs font-bold text-[#8E6C63]">先选择一级分类，再配置二级拆分</div>
              <div v-else>
                <div v-for="(c, j) in row.l2" :key="j" class="grid grid-cols-[1fr_130px_32px] gap-2 items-center">
                  <el-select v-model="c.category_l2_id" filterable placeholder="选择二级分类" class="!w-full" :disabled="saving">
                    <el-option v-for="o in l2OptionsForL1(row.category_l1_id)" :key="o.id" :label="o.name" :value="String(o.id)" />
                  </el-select>
                  <el-input-number v-model="c.amount" :min="0" :step="50" class="!w-full" controls-position="right" :disabled="saving" />
                  <el-button circle plain type="danger" :disabled="saving" @click="removeL2(idx, j)">
                    <Trash2 class="w-4 h-4" />
                  </el-button>
                </div>
                <div v-if="row.l2.length === 0" class="text-xs font-bold text-[#8E6C63]">未分配预算</div>
              </div>
            </div>
          </div>

          <div class="mt-2 text-xs font-bold text-[#8E6C63]">一级已拆分 {{ allocatedSum.toFixed(2) }} · 总未分配预算 {{ unallocatedBudget.toFixed(2) }}</div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <el-button round class="!px-5 !font-bold" :disabled="saving" @click="open = false">取消</el-button>
        <el-button round type="primary" color="#FF8A65" class="!px-6 !font-black" :loading="saving" @click="save">确定并设置预算</el-button>
      </div>
    </template>
  </el-dialog>
</template>
