<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import SettingsModal from '@/components/SettingsModal.vue'
import { Setting, SwitchButton } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const settingsVisible = ref(false)

const showNav = computed(() => route.name !== 'Login')

type NavLink = { name: string; path: string }
type NavEntry =
  | { kind: 'link'; name: string; path: string }
  | { kind: 'group'; name: string; items: NavLink[] }

const navEntries = computed<NavEntry[]>(() => {
  const transactions: NavLink[] = [{ name: '流水', path: '/transactions' }, { name: '导入账单', path: '/import' }]

  const management: NavLink[] = [{ name: '分类管理', path: '/categories' }]
  if (userStore.user.role === 'admin') {
    management.push({ name: '用户管理', path: '/users' })
  }

  return [
    { kind: 'link', name: '主页', path: '/' },
    { kind: 'group', name: '流水', items: transactions },
    { kind: 'group', name: '分析', items: [{ name: '报表', path: '/reports' }, { name: 'AI消费顾问', path: '/ai-advisor' }] },
    { kind: 'group', name: '规划', items: [{ name: '预算', path: '/budget' }, { name: '项目管理', path: '/projects' }] },
    { kind: 'group', name: '管理', items: management },
  ]
})

const isActiveNav = (path: string) => {
  if (path === '/') return route.path === '/'
  return route.path === path || route.path.startsWith(path + '/')
}

const isActiveGroup = (items: NavLink[]) => items.some((it) => isActiveNav(it.path))

const handleNavCommand = (path: string) => {
  router.push(path)
}

const handleLogout = () => {
  userStore.logout()
  router.push('/login')
}
</script>

<template>
  <div class="min-h-screen bg-background font-sans">
    <header v-if="showNav" class="sticky top-0 z-50 bg-background/85 backdrop-blur-md shadow-sm border-b border-[#FFE6DE]">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex justify-between h-16">
          <div class="flex">
            <div class="flex-shrink-0 flex items-center">
              <router-link
                to="/"
                class="site-logo text-2xl font-bold text-primary tracking-tight flex items-center gap-3"
              >
                <span class="site-logo__frame inline-flex h-12 w-12 items-center justify-center rounded-2xl ring-1 ring-primary/20 shadow-sm">
                  <img
                    src="/logo.jpeg"
                    alt="Ledger"
                    class="site-logo__image h-12 w-12 rounded-2xl"
                  />
                </span>
                Ledger
              </router-link>
            </div>
            <nav class="hidden sm:ml-10 sm:flex sm:space-x-2 items-center">
              <template v-for="entry in navEntries" :key="entry.kind === 'link' ? entry.path : entry.name">
                <router-link
                  v-if="entry.kind === 'link'"
                  :to="entry.path"
                  class="px-4 py-2 rounded-xl text-sm font-black transition-all duration-200"
                  :class="isActiveNav(entry.path) ? 'bg-primary text-white shadow-lg shadow-primary/25' : 'text-[#8E6C63] hover:bg-white/80 hover:text-[#5A3B32]'
                  "
                >
                  {{ entry.name }}
                </router-link>

                <el-dropdown v-else trigger="hover" placement="bottom-start" popper-class="nav-popover" @command="handleNavCommand">
                  <button
                    type="button"
                    class="px-4 py-2 rounded-xl text-sm font-black transition-all duration-200"
                    :class="isActiveGroup(entry.items) ? 'bg-primary text-white shadow-lg shadow-primary/25' : 'text-[#8E6C63] hover:bg-white/80 hover:text-[#5A3B32]'"
                  >
                    {{ entry.name }}
                  </button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item v-for="it in entry.items" :key="it.path" :command="it.path" :class="isActiveNav(it.path) ? 'nav-item nav-item--active' : 'nav-item'">
                        {{ it.name }}
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </nav>
          </div>
          <div class="flex items-center space-x-4">
            <el-popover
              placement="bottom-end"
              :width="200"
              trigger="hover"
              popper-class="cute-popover"
              :show-arrow="false"
              :offset="10"
            >
              <template #reference>
                <div class="bg-white px-2 py-1 rounded-full border border-gray-100 shadow-sm flex items-center gap-2 pr-4 cursor-pointer hover:shadow-md transition-shadow">
                    <el-avatar :size="28" :src="userStore.user.avatar || `https://robohash.org/${userStore.user.username}?set=set4&bgset=bg1`" class="mr-1" />
                    <span class="text-sm font-bold text-gray-600">{{ userStore.user.username }}</span>
                </div>
              </template>
              
              <!-- User Card Content -->
              <div class="flex flex-col items-center p-6 bg-white bg-opacity-90 backdrop-blur">
                 <div class="w-24 h-24 rounded-full p-1 bg-gradient-to-tr from-primary to-accent mb-3 shadow-lg transform hover:scale-105 transition-transform duration-300">
                    <div class="w-full h-full rounded-full bg-white p-1 overflow-hidden">
                       <img :src="userStore.user.avatar || `https://robohash.org/${userStore.user.username}?set=set4&bgset=bg1`" class="w-full h-full object-cover rounded-full" />
                    </div>
                 </div>
                 <h3 class="text-xl font-bold text-gray-800">{{ userStore.user.username }}</h3>
                 <p class="text-xs text-primary bg-primary/10 px-3 py-1 rounded-full mt-2 font-bold">{{ userStore.user.role === 'admin' ? '管理员' : '普通用户' }}</p>
              </div>
            </el-popover>

            <el-button circle @click="settingsVisible = true">
              <el-icon><Setting /></el-icon>
            </el-button>
            <el-button circle type="danger" plain @click="handleLogout">
                <el-icon><SwitchButton /></el-icon>
            </el-button>
          </div>
        </div>
      </div>
    </header>

    <main class="max-w-7xl mx-auto py-8 px-4 sm:px-6 lg:px-8">
      <router-view v-slot="{ Component }">
        <transition name="fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
    
    <SettingsModal v-model:visible="settingsVisible" />
  </div>
</template>

<style>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
