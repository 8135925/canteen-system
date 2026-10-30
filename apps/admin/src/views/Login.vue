<template>
  <div class="login-wrap">
    <div class="login-panel">
      <!-- 左侧：四个动画角色（对输入行为做反应） -->
      <div class="char-side">
        <AnimatedCharacters
          :is-typing="isTyping"
          :is-password-focused="isPasswordFocused"
          :show-password="showPassword"
          :password-length="password.length"
          :login-failed="loginFailed"
          :login-success="loginSuccess"
        />
      </div>

      <!-- 右侧：登录表单 -->
      <div class="form-side">
        <h2>食堂报餐管理端</h2>
        <el-form @submit.prevent>
          <el-form-item label="工号">
            <el-input v-model="empNo" placeholder="工号" @focus="isTyping = true" @blur="isTyping = false" />
          </el-form-item>
          <el-form-item label="密码">
            <el-input
              v-model="password"
              :type="showPassword ? 'text' : 'password'"
              placeholder="密码"
              @focus="isPasswordFocused = true"
              @blur="isPasswordFocused = false"
            >
              <template #suffix>
                <span class="eye-toggle" @click="showPassword = !showPassword">{{ showPassword ? '🙈' : '👁' }}</span>
              </template>
            </el-input>
          </el-form-item>
          <el-button type="primary" style="width: 100%" :loading="loading" @click="onLogin">登录</el-button>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { request, setToken, type LoginUser } from '@canteen/api-client'
import { ElMessage } from 'element-plus'
import AnimatedCharacters from '../components/animated/AnimatedCharacters.vue'

const router = useRouter()
const empNo = ref('')
const password = ref('')
const loading = ref(false)

// 动画角色联动状态
const isTyping = ref(false)
const isPasswordFocused = ref(false)
const showPassword = ref(false)
const loginFailed = ref(false)
const loginSuccess = ref(false)

async function onLogin() {
  loading.value = true
  try {
    const data = await request<{ token: string; user: LoginUser }>('POST', '/auth/login', {
      body: { emp_no: empNo.value, password: password.value, remember: false },
      skipAuthRedirect: true,
    })
    // 登录成功：角色一起咧嘴，800ms 后跳转让动画演完
    loginSuccess.value = true
    // 员工账号也允许在 PC 登录：token 存 localStorage，跳转员工端（PC 限宽居中布局）
    setToken(data.token, data.user.role === 'employee')
    sessionStorage.setItem('canteen_role', data.user.role)
    sessionStorage.setItem('canteen_name', data.user.name)
    localStorage.setItem('canteen_role', data.user.role)
    localStorage.setItem('canteen_name', data.user.name)
    ElMessage.success('登录成功')
    if (data.user.role === 'employee') {
      setTimeout(() => {
        window.location.href = '/'
      }, 800)
      return
    }
    setTimeout(() => router.push('/'), 800)
  } catch {
    // 登录失败：摇头 + 垮脸
    loginFailed.value = true
    setTimeout(() => (loginFailed.value = false), 1500)
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
  background: linear-gradient(160deg, #eef2ff 0%, #fce7f3 50%, #fef9c3 100%);
}
.login-panel {
  display: flex;
  align-items: center;
  gap: 24px;
  background: rgba(255, 255, 255, 0.75);
  backdrop-filter: blur(8px);
  border-radius: 20px;
  padding: 24px 32px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.12);
}
.char-side {
  overflow: hidden;
}
.form-side {
  width: 340px;
}
.form-side h2 {
  margin: 0 0 20px;
  color: #333;
}
.eye-toggle {
  cursor: pointer;
  user-select: none;
}
@media (max-width: 900px) {
  .char-side {
    display: none;
  }
}
</style>
