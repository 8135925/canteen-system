<template>
  <div class="app-content">
    <van-nav-bar :title="mustChange ? '首次登录，请修改密码' : '修改密码'" />
    <van-notice-bar v-if="mustChange" wrapable :scrollable="false" text="首次登录或密码被重置后，必须修改密码后才能继续使用系统" />
    <van-form @submit="onSubmit">
      <van-cell-group inset style="margin-top: 12px">
        <van-field v-model="oldPwd" type="password" label="旧密码" required />
        <van-field v-model="newPwd" type="password" label="新密码" placeholder="至少 6 位" required />
        <van-field v-model="confirmPwd" type="password" label="确认新密码" required />
      </van-cell-group>
      <div class="submit-row">
        <van-button round block type="primary" native-type="submit" :loading="loading">确认修改</van-button>
      </div>
    </van-form>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { put, setToken, getToken } from '@canteen/api-client'
import { showToast } from 'vant'

const router = useRouter()
const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const loading = ref(false)

// mcp 状态从 token 恢复无法直接读；用路由 query 或本地标记。简化：改密成功后跳回。
const mustChange = computed(() => sessionStorage.getItem('must_change') === '1' || !!(window as any).__mcp)

async function onSubmit() {
  if (newPwd.value !== confirmPwd.value) {
    showToast('两次输入的新密码不一致')
    return
  }
  loading.value = true
  try {
    const data = await put<{ token: string }>('/my/password', {
      old_password: oldPwd.value,
      new_password: newPwd.value,
    })
    // 重签的 mcp=false token
    if (getToken()) setToken(data.token, !!localStorage.getItem('canteen_token'))
    sessionStorage.removeItem('must_change')
    showToast('修改成功')
    router.push('/menus')
  } catch {
    /* 统一提示 */
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.submit-row {
  margin: 16px 16px;
}
</style>
