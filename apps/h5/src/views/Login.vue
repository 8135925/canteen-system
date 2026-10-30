<template>
  <div class="app-content login-page">
    <van-nav-bar title="登录" />
    <van-form @submit="onSubmit">
      <van-cell-group inset>
        <van-field v-model="empNo" label="工号" placeholder="请输入工号" required />
        <van-field v-model="password" type="password" label="密码" placeholder="请输入密码" required />
      </van-cell-group>
      <div class="remember-row">
        <van-checkbox v-model="remember">记住登录状态</van-checkbox>
      </div>
      <div class="submit-row">
        <van-button round block type="primary" native-type="submit" :loading="loading">登录</van-button>
      </div>
      <div class="tip">首次登录使用默认密码 123456，登录后需修改密码</div>
    </van-form>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { request, setToken, type LoginUser } from '@canteen/api-client'
import { showToast } from 'vant'

const router = useRouter()
const empNo = ref('')
const password = ref('')
const remember = ref(true)
const loading = ref(false)

async function onSubmit() {
  loading.value = true
  try {
    const data = await request<{ token: string; user: LoginUser }>('POST', '/auth/login', {
      body: { emp_no: empNo.value, password: password.value, remember: remember.value },
      skipAuthRedirect: true,
    })
    setToken(data.token, remember.value)
    showToast('登录成功')
    if (data.user.role === 'super_admin' || data.user.role === 'canteen_admin') {
      router.push('/admin/')
    } else {
      router.push('/menus')
    }
  } catch {
    /* 统一提示 */
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  padding-top: 40px;
}
.remember-row {
  margin: 16px 24px;
}
.submit-row {
  margin: 16px 16px;
}
.tip {
  text-align: center;
  color: #999;
  font-size: 12px;
  margin-top: 12px;
}
</style>
