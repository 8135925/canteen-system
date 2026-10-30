import { createApp } from 'vue'
import Vant from 'vant'
import App from './App.vue'
import router from './router'
import { configureClient, clearToken } from '@canteen/api-client'
import { showToast } from 'vant'
import 'vant/lib/index.css'

configureClient({
  onUnauthorized: () => {
    showToast('请先登录')
    router.push('/login')
  },
  onMustChangePassword: () => {
    router.push('/my/password')
  },
  onErrorToast: (msg) => showToast(msg),
})

// 自动登录恢复：启动时校验 localStorage token（T1.15）
configureAutoLogin(router)

function configureAutoLogin(router: any) {
  const token = localStorage.getItem('canteen_token')
  if (token) {
    fetch('/api/v1/me', { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => {
        if (!r.ok) {
          // remember 30 天内不应失效；失效则清除
          localStorage.removeItem('canteen_token')
        }
      })
      .catch(() => {})
  }
}

const app = createApp(App)
app.use(router)
app.use(Vant)
app.mount('#app')

export { clearToken }
