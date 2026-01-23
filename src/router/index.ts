import { createRouter, createWebHistory } from 'vue-router'
import Login from '@/views/Login.vue'
import Home from '@/views/Home.vue'
import Dashboard from '@/views/Dashboard.vue'
import Category from '@/views/Category.vue'
import Projects from '@/views/Projects.vue'
import ProjectDetail from '@/views/ProjectDetail.vue'
import Users from '@/views/Users.vue'
import Reports from '@/views/Reports.vue'
import Import from '@/views/Import.vue'
import AiAdvisor from '@/views/AiAdvisor.vue'
import Budget from '@/views/Budget.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: Login,
    },
    {
      path: '/',
      name: 'Home',
      component: Home,
      meta: { requiresAuth: true },
    },
    {
      path: '/transactions',
      name: 'Transactions',
      component: Dashboard,
      meta: { requiresAuth: true },
    },
    {
      path: '/import',
      name: 'Import',
      component: Import,
      meta: { requiresAuth: true },
    },
    {
      path: '/categories',
      name: 'Category',
      component: Category,
      meta: { requiresAuth: true },
    },
    {
      path: '/projects',
      name: 'Projects',
      component: Projects,
      meta: { requiresAuth: true },
    },
    {
      path: '/projects/:id',
      name: 'ProjectDetail',
      component: ProjectDetail,
      meta: { requiresAuth: true },
    },
    {
      path: '/reports',
      name: 'Reports',
      component: Reports,
      meta: { requiresAuth: true },
    },
    {
      path: '/ai-advisor',
      name: 'AiAdvisor',
      component: AiAdvisor,
      meta: { requiresAuth: true },
    },
    {
      path: '/budget',
      name: 'Budget',
      component: Budget,
      meta: { requiresAuth: true },
    },
    {
      path: '/budget/edit',
      redirect: '/budget',
      meta: { requiresAuth: true },
    },
    {
      path: '/budget/history',
      redirect: '/budget',
      meta: { requiresAuth: true },
    },
    {
      path: '/users',
      name: 'Users',
      component: Users,
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    next('/login')
  } else {
    next()
  }
})

export default router
