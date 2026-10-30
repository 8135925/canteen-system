import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '@canteen/api-client'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/menus' },
    { path: '/menus', component: () => import('../views/Menus.vue') },
    { path: '/history', component: () => import('../views/History.vue') },
    { path: '/login', component: () => import('../views/Login.vue') },
    { path: '/my/records', component: () => import('../views/MyRecords.vue') },
    { path: '/my/suggestions', component: () => import('../views/MySuggestions.vue') },
    { path: '/my', component: () => import('../views/My.vue') },
    { path: '/my/password', component: () => import('../views/ChangePwd.vue') },
  ],
})

// 路由守卫：需要登录的页面未登录跳登录页
router.beforeEach((to) => {
  const publicPages = ['/menus', '/history', '/login']
  if (publicPages.includes(to.path)) return true
  if (!getToken()) return '/login'
  return true
})

export default router
