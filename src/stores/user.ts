import { defineStore } from 'pinia'
import { ref } from 'vue'
import request from '@/api/request'

export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref(JSON.parse(localStorage.getItem('user') || '{}'))

  const login = async (form: any) => {
    const res: any = await request.post('/auth/login', form)
    token.value = res.token
    user.value = res.user
    localStorage.setItem('token', res.token)
    localStorage.setItem('user', JSON.stringify(res.user))
  }

  // const register = async (form: any) => {
  //   await request.post('/auth/register', form)
  // }

  const logout = () => {
    token.value = ''
    user.value = {}
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  const updateSystemSettings = async (data: any) => {
    await request.put('/settings', data)
  }

  const fetchSystemSettings = async () => {
    return await request.get('/settings')
  }

  return { token, user, login, logout, updateSystemSettings, fetchSystemSettings }
})
