<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <h2>食堂报餐管理端</h2>
      <el-form @submit.prevent>
        <el-form-item label="工号"><el-input v-model="empNo" placeholder="工号" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="password" type="password" placeholder="密码" show-password /></el-form-item>
        <el-button type="primary" style="width: 100%" :loading="loading" @click="onLogin">登录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { post, request, setToken, type LoginUser } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

const router = useRouter()
const empNo = ref('')
const password = ref('')
const loading = ref(false)

async function onLogin() {
  loading.value = true
  try {
    const data = await request<{ token: string; user: LoginUser }>('POST', '/auth/login', {
      body: { emp_no: empNo.value, password: password.value, remember: false },
      skipAuthRedirect: true,
    })
    setToken(data.token, false)
    sessionStorage.setItem('canteen_role', data.user.role)
    sessionStorage.setItem('canteen_name', data.user.name)
    ElMessage.success('登录成功')
    if (data.user.role === 'employee') {
      ElMessage.warning('请使用员工端（H5）登录')
      return
    }
    router.push('/')
  } catch {
    /* 统一提示 */
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f0f2f5;
}
.login-card {
  width: 380px;
}
</style>
