<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { ElMessage } from 'element-plus'
import request from '@/api/request'

const router = useRouter()
const userStore = useUserStore()

const loginForm = reactive({
  username: '',
  password: '',
  captcha_id: '',
  captcha_code: ''
})

const captchaText = ref('')
const captchaRequired = ref(false)
const captchaLoading = ref(false)

const fetchCaptcha = async () => {
  if (!loginForm.username) {
    ElMessage.warning('请输入用户名')
    return
  }
  captchaLoading.value = true
  try {
    const res: any = await request.get('/auth/captcha', { params: { username: loginForm.username } })
    captchaText.value = res.captcha_text
    loginForm.captcha_id = res.captcha_id
  } catch (e) {
    return
  } finally {
    captchaLoading.value = false
  }
}

const handleLogin = async () => {
  try {
    if (captchaRequired.value && !loginForm.captcha_code) {
      ElMessage.warning('请输入验证码')
      return
    }
    await userStore.login(loginForm)
    ElMessage.success('登录成功')
    captchaRequired.value = false
    captchaText.value = ''
    loginForm.captcha_code = ''
    loginForm.captcha_id = ''
    router.push('/')
  } catch (e) {
    const data = (e as any)?.response?.data
    if (data?.captcha_required) {
      captchaRequired.value = true
      await fetchCaptcha()
    }
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-orange-50 py-12 px-4 sm:px-6 lg:px-8">
    <div class="max-w-md w-full space-y-8 bg-white p-8 rounded-xl shadow-lg">
      <div class="text-center">
        <div class="login-logo">
          <img src="/logo.jpeg" alt="Ledger" class="login-logo__image" />
        </div>
        <h2 class="mt-6 text-3xl font-extrabold text-gray-900">
          欢迎回来
        </h2>
        <p class="mt-2 text-sm text-gray-600">
          多人共享记账服务 (仅管理员可添加用户)
        </p>
      </div>
      
      <el-form :model="loginForm" label-position="top" @submit.prevent="handleLogin" class="mt-8">
        <el-form-item label="用户名">
          <el-input v-model="loginForm.username" prefix-icon="User" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="loginForm.password" prefix-icon="Lock" type="password" show-password />
        </el-form-item>
        <el-form-item v-if="captchaRequired" label="验证码">
          <div class="flex items-center gap-3 w-full">
            <el-input v-model="loginForm.captcha_code" placeholder="请输入验证码" />
            <el-button :loading="captchaLoading" @click="fetchCaptcha" color="#F3F4F6">{{ captchaText || '获取验证码' }}</el-button>
          </div>
        </el-form-item>
        <el-button type="primary" class="w-full mt-4" native-type="submit" color="#FF6B35">登录</el-button>
      </el-form>
    </div>
  </div>
</template>
