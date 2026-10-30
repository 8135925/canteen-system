<template>
  <el-container class="layout">
    <!-- 侧栏：logo + 菜单（pure-admin 风格深色侧栏） -->
    <el-aside :width="collapsed ? '64px' : '210px'" class="aside">
      <div class="logo">
        <span class="logo-icon">食堂</span>
        <span v-if="!collapsed" class="logo-text">食堂报餐管理</span>
      </div>
      <el-scrollbar>
        <el-menu
          :default-active="$route.path"
          :collapse="collapsed"
          :collapse-transition="false"
          router
          background-color="#1f1f1f"
          text-color="rgba(254,254,254,0.65)"
          active-text-color="#fff"
          class="side-menu"
        >
          <template v-for="item in menuItems" :key="item.path">
            <el-menu-item :index="item.path">
              <el-icon><component :is="item.icon" /></el-icon>
              <template #title>{{ item.menu }}</template>
            </el-menu-item>
          </template>
        </el-menu>
      </el-scrollbar>
    </el-aside>

    <el-container class="main-col">
      <!-- 顶部条：折叠按钮 + 面包屑 + 用户下拉 -->
      <el-header class="header" height="60px">
        <div class="header-left">
          <el-icon class="fold-btn" @click="collapsed = !collapsed">
            <Expand v-if="collapsed" />
            <Fold v-else />
          </el-icon>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item>首页</el-breadcrumb-item>
            <el-breadcrumb-item v-if="$route.meta.menu">{{ $route.meta.menu }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="header-right">
          <el-dropdown trigger="click" @command="onUserCmd">
            <span class="user-chip">
              <el-avatar :size="30" class="user-avatar">{{ avatarChar }}</el-avatar>
              <span class="uname">{{ userName }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-if="canChangePwd" command="password">
                  <el-icon><Key /></el-icon>修改密码
                </el-dropdown-item>
                <el-dropdown-item command="logout" divided>
                  <el-icon><SwitchButton /></el-icon>退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- 多标签页栏 -->
      <div class="tags-bar">
        <div
          v-for="t in tags"
          :key="t.path"
          class="tag"
          :class="{ active: t.path === $route.path }"
          @click="goTag(t)"
        >
          <span class="tag-dot" />
          {{ t.menu }}
          <el-icon v-if="t.path !== '/welcome'" class="tag-close" @click.stop="closeTag(t)">
            <Close />
          </el-icon>
        </div>
      </div>

      <!-- 内容区 -->
      <el-main class="content">
        <router-view v-slot="{ Component }">
          <component :is="Component" />
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Fold, Expand, ArrowDown, Close, Key, SwitchButton, HomeFilled,
  Tickets, Food, Sell, Stopwatch, DataAnalysis, ChatDotRound,
  UserFilled, Coin, TrendCharts, EditPen,
} from '@element-plus/icons-vue'
import { clearToken, get } from '@canteen/api-client'
import { ElMessage, ElMessageBox } from 'element-plus'

const route = useRoute()
const router = useRouter()
const collapsed = ref(false)

const allMenus = [
  { path: '/welcome', menu: '首页', role: '', icon: HomeFilled },
  { path: '/verify', menu: '现场核销', role: 'canteen_admin', icon: Tickets },
  { path: '/menus/publish', menu: '菜谱发布', role: 'canteen_admin', icon: Food },
  { path: '/menus/items', menu: '菜品下架', role: 'canteen_admin', icon: Sell },
  { path: '/verify-rules', menu: '核销规则', role: 'canteen_admin', icon: Stopwatch },
  { path: '/statistics', menu: '数据统计', role: 'canteen_admin', icon: DataAnalysis },
  { path: '/suggestions', menu: '建议管理', role: 'canteen_admin', icon: ChatDotRound },
  { path: '/employees', menu: '员工管理', role: 'super_admin', icon: UserFilled },
  { path: '/super/users', menu: '账号管理', role: 'super_admin', icon: Coin },
  { path: '/super/violations', menu: '违约查看', role: 'super_admin', icon: TrendCharts },
  { path: '/super/dashboard', menu: '数据看板', role: 'super_admin', icon: DataAnalysis },
  { path: '/password', menu: '修改密码', role: '', icon: EditPen },
]

const role = sessionStorage.getItem('canteen_role') || localStorage.getItem('canteen_role') || ''
const menuItems = allMenus.filter((m) => !m.role || m.role === role)
const userName = computed(() => sessionStorage.getItem('canteen_name') || localStorage.getItem('canteen_name') || '')
const avatarChar = computed(() => (userName.value || '管').slice(0, 1))

// 演示模式下超管密码固定 123123 且不可修改（后端同时拦截），隐藏改密菜单
const demoMode = ref(false)
const canChangePwd = computed(() => !(demoMode.value && role === 'super_admin'))

onMounted(async () => {
  try {
    const me = await get('/me')
    demoMode.value = !!me.demo_mode
  } catch {
    /* 未登录由守卫处理 */
  }
})

/* 多标签页：记录访问过的菜单 */
const tags = ref<{ path: string; menu: string }[]>([])
const homeTag = { path: '/welcome', menu: '首页' }

if (!tags.value.some((t) => t.path === homeTag.path)) {
  tags.value.push(homeTag)
}

watch(
  () => route.fullPath,
  () => {
    const menu = route.meta.menu as string | undefined
    if (!menu) return
    if (!tags.value.some((t) => t.path === route.path)) {
      tags.value.push({ path: route.path, menu })
    }
  },
  { immediate: true },
)

function goTag(t: { path: string; menu: string }) {
  if (t.path !== route.path) router.push(t.path)
}

function closeTag(t: { path: string; menu: string }) {
  const i = tags.value.findIndex((x) => x.path === t.path)
  if (i < 0) return
  tags.value.splice(i, 1)
  if (t.path === route.path) {
    const next = tags.value[i - 1] || tags.value[i] || homeTag
    router.push(next.path)
  }
}

function onUserCmd(cmd: string) {
  if (cmd === 'password') {
    router.push('/password')
    return
  }
  ElMessageBox.confirm('确定退出登录吗？', '提示', { type: 'warning' }).then(() => {
    clearToken()
    sessionStorage.removeItem('canteen_role')
    sessionStorage.removeItem('canteen_name')
    ElMessage.success('已退出')
    router.push('/login')
  }).catch(() => {})
}
</script>

<style scoped>
.layout {
  height: 100vh;
}
.aside {
  background: #1f1f1f;
  transition: width 0.25s;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #fff;
  flex-shrink: 0;
}
.logo-icon {
  background: #409eff;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  border-radius: 6px;
  padding: 4px 5px;
  letter-spacing: 1px;
}
.logo-text {
  font-size: 15px;
  font-weight: 700;
  white-space: nowrap;
}
.side-menu {
  border-right: none;
}
.side-menu:not(.el-menu--collapse) {
  width: 210px;
}
.side-menu :deep(.el-menu-item) {
  margin: 4px 10px;
  border-radius: 6px;
  height: 44px;
}
.side-menu :deep(.el-menu-item.is-active) {
  background: #409eff;
}
.side-menu :deep(.el-menu-item:hover) {
  background: rgba(255, 255, 255, 0.08);
}
.main-col {
  min-width: 0;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #e8e8e8;
  padding: 0 16px;
  position: sticky;
  top: 0;
  z-index: 10;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}
.fold-btn {
  font-size: 18px;
  cursor: pointer;
  color: #555;
}
.fold-btn:hover {
  color: #409eff;
}
.header-right {
  display: flex;
  align-items: center;
}
.user-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  outline: none;
}
.user-avatar {
  background: #409eff;
  font-weight: 700;
}
.uname {
  color: #333;
  font-size: 14px;
}
.tags-bar {
  background: #fff;
  border-bottom: 1px solid #e8e8e8;
  padding: 5px 12px;
  display: flex;
  gap: 8px;
  overflow-x: auto;
  flex-shrink: 0;
}
.tag {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  font-size: 12px;
  color: #666;
  background: #f5f7fa;
  border: 1px solid #e4e7ed;
  border-radius: 3px;
  cursor: pointer;
  white-space: nowrap;
  user-select: none;
}
.tag.active {
  color: #409eff;
  background: #ecf5ff;
  border-color: #b3d8ff;
}
.tag-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #c0c4cc;
}
.tag.active .tag-dot {
  background: #409eff;
}
.tag-close {
  border-radius: 50%;
  font-size: 11px;
}
.tag-close:hover {
  background: #c0c4cc;
  color: #fff;
}
.content {
  background: #f0f2f5;
  padding: 14px;
  overflow-y: auto;
}
/* 卡片风格向 pure-admin 靠拢 */
.content :deep(.el-card) {
  border: none;
  border-radius: 6px;
  box-shadow: 0 1px 4px rgba(0, 21, 41, 0.08) !important;
}
.content :deep(.el-card__body) {
  padding: 18px 20px;
}
</style>
