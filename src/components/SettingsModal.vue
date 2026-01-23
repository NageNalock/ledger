<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { useUserStore } from '@/stores/user'
import request from '@/api/request'
import { ElMessage } from 'element-plus'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits(['update:visible'])

const userStore = useUserStore()
const activeTab = ref('profile')
const loading = ref(false)

// System Settings Form
const systemForm = reactive({
  openai_base_url: '',
  openai_api_key: '',
  openai_model: '',
})

const testingAI = ref(false)
const loadingModels = ref(false)
const modelOptions = ref<string[]>([])

const previewChatUrl = () => {
  const base = (systemForm.openai_base_url || '').trim().replace(/\/+$/, '')
  if (!base) return ''
  return `${base}/chat/completions`
}

// Profile Form
const profileForm = reactive({
  username: '',
  new_password: '',
  confirm_password: '',
  avatar: '',
})

const defaultAvatar = 'https://robohash.org/cat?set=set4&bgset=bg1'

watch(() => props.visible, async (val) => {
  if (val) {
    // Init Profile
    profileForm.username = userStore.user.username
    profileForm.avatar = userStore.user.avatar || `https://robohash.org/${userStore.user.username}?set=set4&bgset=bg1`
    profileForm.new_password = ''
    profileForm.confirm_password = ''
    
    // Init System Settings (Admin Only)
    if (userStore.user.role === 'admin') {
        try {
          loading.value = true
          const res: any = await userStore.fetchSystemSettings()
          systemForm.openai_base_url = res.openai_base_url || ''
          systemForm.openai_model = res.openai_model || res.ark_endpoint_id || ''

          const apiKey = res.openai_api_key || res.ark_api_key || ''
          systemForm.openai_api_key = typeof apiKey === 'string' && apiKey.startsWith('******') ? '' : apiKey
        } catch (e) {
          console.error(e)
        } finally {
          loading.value = false
        }
    }
  }
})

const handleClose = () => {
  emit('update:visible', false)
}

const handleAvatarSuccess = (res: any) => {
  profileForm.avatar = res.url
}

const beforeAvatarUpload = (rawFile: any) => {
  if (rawFile.type !== 'image/jpeg' && rawFile.type !== 'image/png' && rawFile.type !== 'image/webp') {
    ElMessage.error('Avatar picture must be JPG/PNG/WEBP format!')
    return false
  } else if (rawFile.size / 1024 / 1024 > 2) {
    ElMessage.error('Avatar picture size can not exceed 2MB!')
    return false
  }
  return true
}

const customUpload = async (options: any) => {
  const formData = new FormData()
  formData.append('file', options.file)
  try {
    const res: any = await request.post('/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
    options.onSuccess(res)
  } catch (e) {
    options.onError(e)
  }
}

const handleProfileSubmit = async () => {
    if (profileForm.new_password && profileForm.new_password !== profileForm.confirm_password) {
        ElMessage.error('两次输入的密码不一致')
        return
    }
    
    loading.value = true
    try {
        const payload: any = { 
          username: profileForm.username,
          avatar: profileForm.avatar
        }
        if (profileForm.new_password) {
            payload.new_password = profileForm.new_password
        }
        
        await request.put('/users/profile', payload)
        
        // Update local store
        userStore.user.username = profileForm.username
        userStore.user.avatar = profileForm.avatar
        
        ElMessage.success('个人信息更新成功')
        if (profileForm.new_password) {
             ElMessage.info('密码已修改，下次请使用新密码登录')
        }
        handleClose()
    } catch (e) {
        console.error(e)
    } finally {
        loading.value = false
    }
}

const handleSystemSubmit = async () => {
  loading.value = true
  try {
    const payload: any = {}
    if (systemForm.openai_base_url.trim()) payload.openai_base_url = systemForm.openai_base_url.trim()
    if (systemForm.openai_model.trim()) payload.openai_model = systemForm.openai_model.trim()
    if (systemForm.openai_api_key.trim()) payload.openai_api_key = systemForm.openai_api_key.trim()

    await userStore.updateSystemSettings(payload)
    ElMessage.success('系统设置保存成功')
    handleClose()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const testAI = async () => {
  testingAI.value = true
  try {
    const payload: any = {
      base_url: systemForm.openai_base_url.trim(),
      api_key: systemForm.openai_api_key.trim(),
      model: systemForm.openai_model.trim(),
    }
    const res: any = await request.post('/ai/test', payload)
    ElMessage.success(`检测成功（${res.latency_ms}ms）`)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || e?.message || '检测失败')
  } finally {
    testingAI.value = false
  }
}

const fetchModels = async () => {
  loadingModels.value = true
  try {
    const payload: any = {
      base_url: systemForm.openai_base_url.trim(),
      api_key: systemForm.openai_api_key.trim(),
    }
    const res: any = await request.post('/ai/models', payload)
    modelOptions.value = Array.isArray(res?.models) ? res.models : []
    if (!modelOptions.value.length) {
      ElMessage.warning('未获取到模型列表')
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || e?.message || '获取模型列表失败')
  } finally {
    loadingModels.value = false
  }
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="设置"
    width="550px"
    @close="handleClose"
  >
    <el-tabs v-model="activeTab">
      <!-- Profile Tab -->
      <el-tab-pane label="个人设置" name="profile">
        <el-form label-position="top" class="mt-2">
          <el-form-item label="头像">
            <div class="flex items-center gap-4">
               <el-avatar :size="64" :src="profileForm.avatar || defaultAvatar" />
               <el-upload
                 class="avatar-uploader"
                 action="#"
                 :show-file-list="false"
                 :http-request="customUpload"
                 :before-upload="beforeAvatarUpload"
                 :on-success="handleAvatarSuccess"
               >
                 <el-button type="primary" plain size="small">更换头像</el-button>
               </el-upload>
            </div>
          </el-form-item>
          <el-form-item label="用户名">
            <el-input v-model="profileForm.username" placeholder="请输入用户名" />
          </el-form-item>
          <el-form-item label="新密码 (留空则不修改)">
            <el-input v-model="profileForm.new_password" type="password" placeholder="请输入新密码" show-password />
          </el-form-item>
          <el-form-item label="确认新密码" v-if="profileForm.new_password">
            <el-input v-model="profileForm.confirm_password" type="password" placeholder="请再次输入新密码" show-password />
          </el-form-item>
        </el-form>
        <div class="flex justify-end mt-6">
            <el-button @click="handleClose">取消</el-button>
            <el-button type="primary" color="#FF6B35" :loading="loading" @click="handleProfileSubmit">
              保存个人设置
            </el-button>
        </div>
      </el-tab-pane>


      <!-- System Tab (Admin Only) -->
      <el-tab-pane label="系统设置" name="system" v-if="userStore.user.role === 'admin'">
        <el-form label-position="top" class="mt-2">
          <el-form-item label="OpenAI Compatible Base URL">
            <el-input v-model="systemForm.openai_base_url" placeholder="例如：https://api.openai.com/v1 或 https://ark.cn-beijing.volces.com/api/v3" />
            <div class="text-xs text-gray-400 mt-1" v-if="previewChatUrl()">预览：{{ previewChatUrl() }}</div>
          </el-form-item>
          <el-form-item label="OpenAI Compatible API Key">
            <el-input v-model="systemForm.openai_api_key" placeholder="请输入 API Key（留空则不更新）" show-password>
              <template #append>
                <el-button :loading="testingAI" @click="testAI">检测</el-button>
              </template>
            </el-input>
            <div class="text-xs text-gray-400 mt-1">多个密钥可用逗号分隔；用于 AI 智能分类与消费分析功能。</div>
          </el-form-item>
          <el-form-item label="Model / Deployment">
            <div class="flex items-center gap-2">
              <el-select
                v-model="systemForm.openai_model"
                class="flex-1"
                placeholder="例如：gpt-4.1-mini 或 ep-xxxxxx"
                filterable
                allow-create
                default-first-option
                :loading="loadingModels"
              >
                <el-option v-for="m in modelOptions" :key="m" :label="m" :value="m" />
              </el-select>
              <el-button :loading="loadingModels" @click="fetchModels">获取模型列表</el-button>
            </div>
            <div class="text-xs text-gray-400 mt-1">OpenAI 填模型名；火山方舟填推理接入点 ID（ep-xxxxxx）。</div>
          </el-form-item>
        </el-form>
        <div class="flex justify-end mt-6">
            <el-button @click="handleClose">取消</el-button>
            <el-button type="primary" color="#FF6B35" :loading="loading" @click="handleSystemSubmit">
              保存系统设置
            </el-button>
        </div>
      </el-tab-pane>
    </el-tabs>
  </el-dialog>
</template>
