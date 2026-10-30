<template>
  <div class="login-page">
    <!-- 顶部：四个动画角色（对输入行为做反应） -->
    <div class="char-wrap">
      <AnimatedCharacters
        :is-typing="isTyping"
        :is-password-focused="isPasswordFocused"
        :show-password="showPassword"
        :password-length="password.length"
        :login-failed="loginFailed"
        :login-success="loginSuccess"
      />
    </div>

    <!-- 登录表单 -->
    <van-form @submit="onSubmit">
      <van-cell-group inset>
        <van-field
          v-model="empNo"
          label="工号"
          placeholder="请输入工号"
          required
          @focus="isTyping = true"
          @blur="isTyping = false"
        />
        <van-field
          v-model="password"
          :type="showPassword ? 'text' : 'password'"
          label="密码"
          placeholder="请输入密码"
          required
          @focus="isPasswordFocused = true"
          @blur="isPasswordFocused = false"
        >
          <template #right-icon>
            <span class="eye-toggle" @click="showPassword = !showPassword">{{ showPassword ? '🙈' : '👁' }}</span>
          </template>
        </van-field>
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
import AnimatedCharacters from '../components/animated/AnimatedCharacters.vue'

const router = useRouter()
const empNo = ref('')
const password = ref('')
const remember = ref(true)
const loading = ref(false)

// 动画角色联动状态
const isTyping = ref(false)
const isPasswordFocused = ref(false)
const showPassword = ref(false)
const loginFailed = ref(false)
const loginSuccess = ref(false)

async function onSubmit() {
  loading.value = true
  try {
    const data = await request<{ token: string; user: LoginUser }>('POST', '/auth/login', {
      body: { emp_no: empNo.value, password: password.value, remember: remember.value },
      skipAuthRedirect: true,
    })
    // 登录成功：角色一起咧嘴，800ms 后跳转让动画演完
    loginSuccess.value = true
    setToken(data.token, remember.value)
    localStorage.setItem('canteen_role', data.user.role)
    localStorage.setItem('canteen_name', data.user.name)
    setTimeout(() => {
      // 首登/被重置：立即进入改密页
      if (data.user.must_change_password) {
        router.push('/my/password')
        return
      }
      if (data.user.role === 'super_admin' || data.user.role === 'canteen_admin') {
        // /admin/ 是独立 SPA，必须整页导航（router.push 只会匹配 h5 自己的路由）
        window.location.href = '/admin/'
      } else {
        router.push('/menus')
      }
    }, 800)
  } catch {
    // 登录失败：摇头 + 垮脸（1.5s 后复位，便于下次失败重播）
    loginFailed.value = true
    setTimeout(() => (loginFailed.value = false), 1500)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  background: linear-gradient(160deg, #eef2ff 0%, #fce7f3 50%, #fef9c3 100%);
  display: flex;
  flex-direction: column;
}
.char-wrap {
  display: flex;
  justify-content: center;
  overflow: hidden;
  /* 场景 550px 宽在 480px 容器内等比缩放 */
  height: 330px;
}
.char-wrap :deep(.ac-scene) {
  transform: scale(0.72);
  transform-origin: top center;
}
.remember-row {
  margin: 14px 24px;
}
.submit-row {
  margin: 14px 16px;
}
.tip {
  text-align: center;
  color: #999;
  font-size: 12px;
  margin-top: 10px;
}
.eye-toggle {
  cursor: pointer;
  font-size: 16px;
  user-select: none;
}
</style>
