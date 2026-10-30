<template>
  <div class="app-content">
    <van-nav-bar title="我的建议" left-arrow @click-left="$router.back()" />

    <!-- 提交新建议 -->
    <van-cell-group inset title="提交新建议" style="margin-top: 12px">
      <van-field
        v-model="content"
        type="textarea"
        rows="3"
        maxlength="200"
        show-word-limit
        placeholder="写下你想吃的菜品或对食堂的建议（最多 200 字）"
      />
      <div style="padding: 10px 16px">
        <van-button block type="primary" :disabled="!content.trim()" :loading="submitting" @click="submit">
          提交建议
        </van-button>
      </div>
    </van-cell-group>

    <!-- 我的建议列表 -->
    <van-cell-group inset title="我的建议记录" style="margin-top: 12px; margin-bottom: 20px">
      <van-cell v-for="s in list" :key="s.id" :title="s.content" :label="new Date(s.created_at).toLocaleString()">
        <van-tag :type="tagType(s.status)">{{ statusText(s.status) }}</van-tag>
      </van-cell>
      <van-cell v-if="list.length === 0" title="暂无建议，快去提一条吧" />
    </van-cell-group>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { get, post, type Suggestion } from '@canteen/api-client'
import { showToast } from 'vant'

const router = useRouter()
const list = ref<Suggestion[]>([])
const content = ref('')
const submitting = ref(false)

function statusText(s: string) {
  return { pending: '待处理', adopted: '已采纳', rejected: '已拒绝' }[s] || s
}
function tagType(s: string): any {
  return ({ pending: 'warning', adopted: 'success', rejected: 'danger' } as Record<string, string>)[s] || 'default'
}

async function load() {
  list.value = await get('/my/suggestions')
}

async function submit() {
  submitting.value = true
  try {
    await post('/suggestions', { content: content.value.trim() })
    showToast('提交成功，等待食堂管理员处理')
    content.value = ''
    await load()
  } catch {
    /* 统一提示 */
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  const role = localStorage.getItem('canteen_role') || sessionStorage.getItem('canteen_role') || 'employee'
  if (role !== 'employee') {
    showToast('管理员请在管理端处理建议')
    router.push('/menus')
    return
  }
  await load()
})
</script>
