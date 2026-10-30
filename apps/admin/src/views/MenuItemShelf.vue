<template>
  <div>
    <el-card>
      <h3>菜品下架 / 恢复</h3>
      <el-date-picker v-model="date" type="date" value-format="YYYY-MM-DD" @change="load" />
      <el-table :data="dishes" border style="margin-top: 12px">
        <el-table-column prop="meal" label="餐次" width="90" align="center" />
        <el-table-column prop="dish_name" label="菜品" min-width="180" show-overflow-tooltip />
        <el-table-column label="口味" min-width="140">
          <template #default="{ row }">{{ (row.taste_tags || []).join('、') }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'normal' ? 'success' : 'danger'">
              {{ row.status === 'normal' ? '正常' : '已下架' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.status === 'normal'"
              active-text="正常"
              inactive-text="下架"
              @change="toggle(row)"
            />
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, patch } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

const date = ref(new Date().toISOString().slice(0, 10))
const dishes = ref<any[]>([])
const mealLabel = (m: string) => ({ breakfast: '早餐', lunch: '午餐', dinner: '晚餐' }[m] || m)

async function load() {
  const data = await get('/menus', { start: date.value, end: date.value })
  dishes.value = data.dishes || []
}

async function toggle(row: any) {
  const next = row.status === 'normal' ? 'off_shelf' : 'normal'
  await patch(`/admin/menu-items/${row.id}/shelf`, { status: next })
  ElMessage.success(next === 'off_shelf' ? '已下架' : '已恢复')
  await load()
}

onMounted(load)
</script>
