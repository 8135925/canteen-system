<template>
  <div class="app-content">
    <van-nav-bar title="用餐记录" />
    <div class="violation-card">
      <div class="v-item">
        <div class="v-num">{{ counts.no_show }}</div>
        <div class="v-label">未核销违约</div>
      </div>
      <div class="v-item">
        <div class="v-num">{{ counts.walk_in }}</div>
        <div class="v-label">未报名就餐</div>
      </div>
      <div class="v-item">
        <div class="v-num total">{{ counts.total }}</div>
        <div class="v-label">违约总次数</div>
      </div>
    </div>
    <van-cell-group inset title="近 30 天记录">
      <van-cell v-for="(r, i) in records" :key="i" :title="r.menu_date + ' ' + mealLabel(r.meal)">
        <van-tag :type="tagType(r.status)">{{ statusText(r.status) }}</van-tag>
      </van-cell>
      <van-cell v-if="records.length === 0" title="暂无记录" />
    </van-cell-group>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, type SignupView } from '@canteen/api-client'

const records = ref<SignupView[]>([])
const counts = ref({ no_show: 0, walk_in: 0, total: 0 })

function mealLabel(m: string) {
  return { breakfast: '早餐', lunch: '午餐', dinner: '晚餐' }[m] || m
}
function statusText(s: string) {
  return { signed: '已报名', no_show: '违约', verified: '已核销', cancelled: '已取消', walk_in: '未报名就餐' }[s] || s
}
function tagType(s: string) {
  if (s === 'no_show' || s === 'walk_in') return 'danger'
  if (s === 'verified') return 'success'
  if (s === 'cancelled') return 'default'
  return 'primary'
}

onMounted(async () => {
  const data = await get('/my/signups')
  records.value = data.records || []
  const vc = await get('/my/violation-count')
  counts.value = vc
})
</script>

<style scoped>
.violation-card {
  display: flex;
  background: #fff;
  margin: 12px;
  border-radius: 10px;
  padding: 16px 0;
}
.v-item {
  flex: 1;
  text-align: center;
}
.v-num {
  font-size: 22px;
  font-weight: 700;
  color: #333;
}
.v-num.total {
  color: #ee0a24;
}
.v-label {
  font-size: 12px;
  color: #999;
  margin-top: 2px;
}
</style>
