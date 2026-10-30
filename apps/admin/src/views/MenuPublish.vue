<template>
  <div>
    <el-card>
      <h3>菜谱发布（今日起 15 天内）</h3>
      <el-form inline>
        <el-form-item label="日期">
          <el-date-picker v-model="menuDate" type="date" value-format="YYYY-MM-DD" :disabled-date="disabledDate" />
        </el-form-item>
        <el-form-item label="餐次">
          <el-select v-model="meal" style="width: 120px">
            <el-option label="早餐" value="breakfast" />
            <el-option label="午餐" value="lunch" />
            <el-option label="晚餐" value="dinner" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button @click="copyLastWeek">复制上周</el-button>
        </el-form-item>
      </el-form>

      <el-table :data="rows" border>
        <el-table-column label="菜品名称" width="220">
          <template #default="{ $index }"><el-input v-model="rows[$index].dish_name" placeholder="菜名" /></template>
        </el-table-column>
        <el-table-column label="口味标签（逗号分隔）">
          <template #default="{ $index }"><el-input v-model="rows[$index].tags" placeholder="如：微辣,清淡" /></template>
        </el-table-column>
        <el-table-column width="80">
          <template #default="{ $index }">
            <el-button type="danger" link @click="rows.splice($index, 1)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top: 12px">
        <el-button @click="rows.push({ dish_name: '', tags: '' })">添加菜品</el-button>
        <el-button type="primary" :loading="publishing" @click="publish">发布该餐菜谱</el-button>
      </div>
    </el-card>

    <el-card style="margin-top: 16px">
      <h3>已发布菜谱（近 15 天）</h3>
      <el-collapse>
        <el-collapse-item v-for="day in publishedDays" :key="day.date" :title="`${day.date}`">
          <div v-for="g in day.groups" :key="g.meal">
            <b>{{ mealLabel(g.meal) }}：</b>{{ g.dishes.map((d: any) => d.dish_name).join('、') }}
          </div>
        </el-collapse-item>
      </el-collapse>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { get, post } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

const menuDate = ref(new Date().toISOString().slice(0, 10))
const meal = ref('lunch')
const rows = ref<{ dish_name: string; tags: string }[]>([{ dish_name: '', tags: '' }])
const publishing = ref(false)
const allDishes = ref<any[]>([])

const mealLabel = (m: string) => ({ breakfast: '早餐', lunch: '午餐', dinner: '晚餐' }[m] || m)

function disabledDate(d: Date) {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const max = new Date(today.getTime() + 14 * 86400000)
  return d < today || d > max
}

const publishedDays = computed(() => {
  const byDate: Record<string, any[]> = {}
  for (const d of allDishes.value) {
    byDate[d.menu_date] = byDate[d.menu_date] || []
    byDate[d.menu_date].push(d)
  }
  return Object.keys(byDate)
    .sort()
    .map((date) => {
      const groups: any[] = []
      for (const m of ['breakfast', 'lunch', 'dinner']) {
        const ds = byDate[date].filter((x) => x.meal === m)
        if (ds.length) groups.push({ meal: m, dishes: ds })
      }
      return { date, groups }
    })
})

async function publish() {
  const dishes = rows.value
    .filter((r) => r.dish_name.trim())
    .map((r) => ({ dish_name: r.dish_name.trim(), taste_tags: r.tags.split(/[,，]/).map((s) => s.trim()).filter(Boolean) }))
  if (dishes.length === 0) {
    ElMessage.warning('请至少添加一个菜品')
    return
  }
  publishing.value = true
  try {
    const data = await post('/admin/menus', {
      items: [{ menu_date: menuDate.value, meal: meal.value, dishes }],
    })
    ElMessage.success(`发布成功 ${data.success_count} 道${data.failures?.length ? `，跳过 ${data.failures.length} 道重复` : ''}`)
    await loadPublished()
  } catch {
    /* 统一提示 */
  } finally {
    publishing.value = false
  }
}

async function copyLastWeek() {
  const data = await get('/admin/menus/copy-last-week')
  const wd = String(((new Date(menuDate.value).getDay() + 6) % 7) + 1) // 1=周一
  const dishes = (data.week?.[wd]?.[meal.value] || []).map((d: any) => ({
    dish_name: d.dish_name,
    tags: (d.taste_tags || []).join(','),
  }))
  if (dishes.length === 0) {
    ElMessage.info('上周该日该餐无菜品')
    return
  }
  rows.value = dishes
  ElMessage.success('已载入上周菜单，请修改后发布')
}

async function loadPublished() {
  const today = new Date().toISOString().slice(0, 10)
  const end = new Date(Date.now() + 14 * 86400000).toISOString().slice(0, 10)
  const data = await get('/menus', { start: today, end })
  allDishes.value = data.dishes || []
}

onMounted(loadPublished)
</script>
