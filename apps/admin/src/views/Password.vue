<template>
  <div>
    <el-card style="max-width: 480px">
      <h3>修改密码</h3>
      <el-form label-width="90px">
        <el-form-item label="旧密码"><el-input v-model="oldPwd" type="password" show-password /></el-form-item>
        <el-form-item label="新密码"><el-input v-model="newPwd" type="password" show-password placeholder="至少 6 位" /></el-form-item>
        <el-form-item label="确认新密码"><el-input v-model="confirmPwd" type="password" show-password /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="submit">确认修改</el-button>
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false" title="修改成功后将重新签发登录凭证。" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { put, setToken } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')

async function submit() {
  if (newPwd.value !== confirmPwd.value) {
    ElMessage.error('两次输入的新密码不一致')
    return
  }
  const data = await put<{ token: string }>('/my/password', {
    old_password: oldPwd.value,
    new_password: newPwd.value,
  })
  setToken(data.token, !!localStorage.getItem('canteen_token'))
  ElMessage.success('修改成功')
  oldPwd.value = newPwd.value = confirmPwd.value = ''
}
</script>
