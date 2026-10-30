<template>
  <div class="app-content">
    <van-nav-bar title="历史菜谱" left-arrow @click-left="$router.back()" />
    <van-cell title="日期范围" :value="`${start} ~ ${end}`" @click="showPicker = true" />
    <van-cell title="上周菜单">
      <van-button size="small" type="primary" plain @click="lastWeek">查看上周</van-button>
    </van-cell>
    <div v-for="d in days" :key="d">
      <van-divider>{{ d }}</van-divider>
      <div v-for="m in meals" :key="m.key">
        <div class="meal-title">{{ m.label }}</div>
        <div v-if="dishesOf(d, m.key).length === 0" class="empty-sm">无菜品</div>
        <div v-for="dish in dishesOf(d, m.key)" :key="dish.id" class="hist-dish">
          <span>{{ dish.dish_name }}</span>
          <span v-if="dish.status === 'off_shelf'" class="off-tag">已下架</span>
          <span class="likes">👍{{ dish.like_count }} 👎{{ dish.dislike_count }}</span>
        </div>
      </div>
    </div>
    <van-popup v-model:show="showPicker" position="bottom">
      <van-date-picker
        :min-date="minDate"
        :max-date="new Date()"
        title="选择起始日期"
        @confirm="onPick"
        @cancel="showPicker = false"
      />
    </van-popup>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { get, type Dish } from '@canteen/api-client'

const meals = [
  { key: 'breakfast', label: '早餐' },
  { key: 'lunch', label: '午餐' },
  { key: 'dinner', label: '晚餐' },
]

function fmt(d: Date) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const start = ref(fmt(new Date(Date.now() - 6 * 86400000)))
const end = ref(fmt(new Date()))
const showPicker = ref(false)
const minDate = new Date(Date.now() - 30 * 86400000)

const dishes = ref<Dish[]>([])

const days = computed(() => {
  const out: string[] = []
  const cur = new Date(start.value)
  while (fmt(cur) <= end.value) {
    out.push(fmt(cur))
    cur.setDate(cur.getDate() + 1)
  }
  return out
})

function dishesOf(day: string, meal: string) {
  return dishes.value.filter((x) => x.menu_date === day && x.meal === meal)
}

async function load() {
  const data = await get('/menus', { start: start.value, end: end.value })
  dishes.value = data.dishes || []
}

function onPick({ selectedValues }: any) {
  start.value = selectedValues.join('-')
  end.value = fmt(new Date())
  showPicker.value = false
  load()
}

function lastWeek() {
  end.value = fmt(new Date())
  start.value = fmt(new Date(Date.now() - 6 * 86400000))
  load()
}

onMounted(load)
</script>

<style scoped>
.meal-title {
  font-weight: 600;
  margin: 6px 16px 2px;
  color: #666;
  font-size: 13px;
}
.empty-sm {
  color: #bbb;
  font-size: 12px;
  margin: 2px 16px;
}
.hist-dish {
  display: flex;
  gap: 8px;
  align-items: center;
  background: #fff;
  margin: 4px 12px;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 14px;
}
.off-tag {
  background: #eee;
  color: #999;
  font-size: 11px;
  padding: 0 5px;
  border-radius: 3px;
}
.likes {
  margin-left: auto;
  color: #999;
  font-size: 12px;
}
</style>
