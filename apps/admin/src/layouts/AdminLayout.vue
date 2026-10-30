<template>
  <el-container class="layout">
    <el-aside width="200px" class="aside">
      <div class="logo">食堂报餐管理</div>
      <el-menu :default-active="$route.path" router :default-openeds="openeds">
        <template v-for="item in menuItems" :key="item.path">
          <el-menu-item :index="item.path">{{ item.menu }}</el-menu-item>
        </template>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <el-breadcrumb separator="/">
          <el-breadcrumb-item>首页</el-breadcrumb-item>
          <el-breadcrumb-item>{{ $route.meta.menu || '' }}</el-breadcrumb-item>
        </el-breadcrumb>
        <div class="header-right">
          <span class="uname">{{ userName }}</span>
          <el-button link type="danger" @click="logout">退出</el-button>
        </div>
      </el-header>
      <el-main><router-view /></el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { clearToken } from '@canteen/api-client'

const route = useRoute()
const router = useRouter()

const allMenus = [
  { path: '/verify', menu: '现场核销', role: 'canteen_admin' },
  { path: '/menus/publish', menu: '菜谱发布', role: 'canteen_admin' },
  { path: '/menus/items', menu: '菜品下架', role: 'canteen_admin' },
  { path: '/employees', menu: '员工管理', role: 'canteen_admin' },
  { path: '/verify-rules', menu: '核销规则', role: 'canteen_admin' },
  { path: '/statistics', menu: '数据统计', role: 'canteen_admin' },
  { path: '/suggestions', menu: '建议管理', role: 'canteen_admin' },
  { path: '/super/users', menu: '账号管理', role: 'super_admin' },
  { path: '/super/violations', menu: '违约查看', role: 'super_admin' },
  { path: '/super/dashboard', menu: '数据看板', role: 'super_admin' },
  { path: '/password', menu: '修改密码', role: '' },
]

const role = sessionStorage.getItem('canteen_role') || localStorage.getItem('canteen_role') || ''
const menuItems = allMenus.filter((m) => !m.role || m.role === role)
const openeds = ['0']

const userName = computed(() => sessionStorage.getItem('canteen_name') || localStorage.getItem('canteen_name') || '')

function logout() {
  clearToken()
  router.push('/login')
}
</script>

<style scoped>
.layout {
  height: 100vh;
}
.aside {
  background: #001529;
  color: #fff;
}
.logo {
  height: 56px;
  line-height: 56px;
  text-align: center;
  color: #fff;
  font-weight: 700;
}
.aside :deep(.el-menu) {
  border-right: none;
  background: transparent;
}
.aside :deep(.el-menu-item) {
  color: #bbb;
}
.aside :deep(.el-menu-item.is-active) {
  color: #fff;
  background: #1890ff;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #eee;
  background: #fff;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.uname {
  color: #666;
}
</style>
