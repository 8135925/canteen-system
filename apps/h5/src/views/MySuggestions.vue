<template>
  <div class="app-content">
    <van-nav-bar title="我的建议" left-arrow @click-left="$router.back()" />
    <van-cell-group inset>
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
import { get, type Suggestion } from '@canteen/api-client'
import { showToast } from 'vant'

const router = useRouter()
const list = ref<Suggestion[]>([])

function statusText(s: string) {
  return { pending: '待处理', adopted: '已采纳', rejected: '已拒绝' }[s] || s
}
function tagType(s: string): any {
  return ({ pending: 'warning', adopted: 'success', rejected: 'danger' } as Record<string, string>)[s] || 'default'
}

onMounted(async () => {
  const role = localStorage.getItem('canteen_role') || sessionStorage.getItem('canteen_role') || 'employee'
  if (role !== 'employee') {
    showToast('管理员请在管理端处理建议')
    router.push('/menus')
    return
  }
  list.value = await get('/my/suggestions')
})
</script>
