<template>
  <div>
    <el-card>
      <h3>数据统计</h3>
      <el-form inline>
        <el-form-item label="日期">
          <el-date-picker v-model="date" type="date" value-format="YYYY-MM-DD" @change="load" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load">查询</el-button>
        </el-form-item>
      </el-form>

      <h4>按餐次报名人数（{{ date }}）</h4>
      <el-table :data="signups" border style="max-width: 480px">
        <el-table-column prop="meal" label="餐次" width="120">
          <template #default="{ row }">{{ mealLabel(row.meal) }}</template>
        </el-table-column>
        <el-table-column prop="count" label="报名人数" />
      </el-table>

      <h4 style="margin-top: 20px">按部门报名分布</h4>
      <el-table :data="departments" border style="max-width: 480px">
        <el-table-column prop="department" label="部门" />
        <el-table-column prop="count" label="报名人数" />
      </el-table>
    </el-card>

    <el-card style="margin-top: 16px">
      <h3>实际就餐 vs 报名差异（近 30 天）</h3>
      <el-table :data="daily" border>
        <el-table-column prop="date" label="日期" width="120" />
        <el-table-column prop="signups" label="报名数" width="100" />
        <el-table-column prop="verified" label="核销数" width="100" />
        <el-table-column prop="walk_in" label="未报名就餐" width="110" />
        <el-table-column prop="diff" label="差值">
          <template #default="{ row }">
            <span :style="{ color: row.diff === 0 ? '#333' : row.diff > 0 ? '#67c23a' : '#f56c6c' }">{{ row.diff }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, type MealCount, type DeptCount, type DailyActualRow } from '@canteen/api-client'

const date = ref(new Date().toISOString().slice(0, 10))
const signups = ref<MealCount[]>([])
const departments = ref<DeptCount[]>([])
const daily = ref<DailyActualRow[]>([])
const mealLabel = (m: string) => ({ breakfast: '早餐', lunch: '午餐', dinner: '晚餐' }[m] || m)

async function load() {
  signups.value = (await get('/admin/stats/signups', { date })).signups || []
  departments.value = (await get('/admin/stats/departments', { date })).departments || []
  const to = date.value
  const from = new Date(Date.now() - 29 * 86400000).toISOString().slice(0, 10)
  daily.value = (await get('/admin/stats/daily', { from, to })).daily || []
}

onMounted(load)
</script>
