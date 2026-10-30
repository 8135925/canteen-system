import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '@canteen/api-client'

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes: [
    { path: '/login', component: () => import('../views/Login.vue') },
    {
      path: '/',
      component: () => import('../layouts/AdminLayout.vue'),
      children: [
        { path: '', redirect: '/verify' },
        { path: 'verify', component: () => import('../views/Verify.vue'), meta: { menu: '现场核销', roles: ['canteen_admin'] } },
        { path: 'menus/publish', component: () => import('../views/MenuPublish.vue'), meta: { menu: '菜谱发布', roles: ['canteen_admin'] } },
        { path: 'menus/items', component: () => import('../views/MenuItemShelf.vue'), meta: { menu: '菜品下架', roles: ['canteen_admin'] } },
        { path: 'employees', component: () => import('../views/Employees.vue'), meta: { menu: '员工管理', roles: ['canteen_admin'] } },
        { path: 'verify-rules', component: () => import('../views/VerifyRules.vue'), meta: { menu: '核销规则', roles: ['canteen_admin'] } },
        { path: 'statistics', component: () => import('../views/Statistics.vue'), meta: { menu: '数据统计', roles: ['canteen_admin'] } },
        { path: 'suggestions', component: () => import('../views/Suggestions.vue'), meta: { menu: '建议管理', roles: ['canteen_admin'] } },
        { path: 'password', component: () => import('../views/Password.vue'), meta: { menu: '修改密码' } },
        { path: 'super/users', component: () => import('../views/super/Users.vue'), meta: { menu: '账号管理', roles: ['super_admin'] } },
        { path: 'super/violations', component: () => import('../views/super/Violations.vue'), meta: { menu: '违约查看', roles: ['super_admin'] } },
        { path: 'super/dashboard', component: () => import('../views/super/Dashboard.vue'), meta: { menu: '数据看板', roles: ['super_admin'] } },
      ],
    },
  ],
})

// 路由守卫：校验 token 与角色
router.beforeEach((to) => {
  if (to.path === '/login') return true
  if (!getToken()) return '/login'
  const roles = to.meta.roles as string[] | undefined
  if (roles && roles.length > 0) {
    const role = sessionStorage.getItem('canteen_role') || localStorage.getItem('canteen_role') || ''
    if (!roles.includes(role)) return '/verify'
  }
  return true
})

export default router
