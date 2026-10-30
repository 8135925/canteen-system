import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import './style.css'
import App from './App.vue'
import router from './router'
import { configureClient, getToken } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

configureClient({
  onUnauthorized: () => {
    ElMessage.error('请先登录')
    router.push('/login')
  },
  onMustChangePassword: () => {
    ElMessage.warning('首次登录请先修改密码')
    router.push('/password')
  },
  onErrorToast: (msg) => ElMessage.error(msg),
})

const app = createApp(App)
app.use(router)
app.use(ElementPlus, { locale: zhCn })
app.mount('#app')

export { getToken }
