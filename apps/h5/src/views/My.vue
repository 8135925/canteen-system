<template>
  <div class="app-content">
    <van-nav-bar title="我的" />
    <van-cell-group inset style="margin-top: 12px">
      <van-cell title="工号" :value="user?.emp_no || '-'" />
      <van-cell title="姓名" :value="user?.name || '-'" />
      <van-cell title="部门" :value="user?.department || '-'" />
    </van-cell-group>
    <van-cell-group inset style="margin-top: 12px">
      <van-cell title="修改密码" icon="edit" is-link to="/my/password" />
      <van-cell title="我的建议" icon="chat-o" is-link to="/my/suggestions" />
      <van-cell title="历史菜谱" icon="calendar-o" is-link to="/history" />
    </van-cell-group>
    <div class="logout-row">
      <van-button block type="danger" plain @click="logout">退出登录</van-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { get, clearToken, getToken, type LoginUser } from '@canteen/api-client'

const router = useRouter()
const user = ref<LoginUser | null>(null)

onMounted(async () => {
  if (getToken()) {
    try {
      user.value = await get('/me')
    } catch {
      /* 未登录跳转由统一封装处理 */
    }
  }
})

function logout() {
  clearToken()
  router.push('/login')
}
</script>

<style scoped>
.logout-row {
  margin: 24px 16px;
}
</style>
