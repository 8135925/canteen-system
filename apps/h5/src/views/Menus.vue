<template>
  <div class="app-content">
    <van-nav-bar title="菜谱" />
    <!-- 日期横滑 Tab（15 天） -->
    <div class="date-tabs">
      <div
        v-for="d in dates"
        :key="d.date"
        class="date-tab"
        :class="{ active: d.date === selectedDate, today: d.isToday }"
        @click="selectedDate = d.date"
      >
        <div class="date-label">{{ d.label }}</div>
        <div class="date-num">{{ d.short }}</div>
      </div>
    </div>
    <!-- 早/午/晚 Tab -->
    <van-tabs v-model:active="mealTab" sticky>
      <van-tab v-for="m in meals" :key="m.key" :title="m.label" :name="m.key">
        <div class="dish-list">
          <div v-if="dishesOf(m.key).length === 0" class="empty">该餐次暂无菜品</div>
          <div v-for="dish in dishesOf(m.key)" :key="dish.id" class="dish-card" :class="{ off: dish.status === 'off_shelf' }">
            <div class="dish-head">
              <span class="dish-name">{{ dish.dish_name }}</span>
              <span v-if="dish.status === 'off_shelf'" class="off-tag">已下架</span>
            </div>
            <div class="dish-tags">
              <van-tag v-for="t in dish.taste_tags" :key="t" plain type="primary" size="medium" class="tag">{{ t }}</van-tag>
            </div>
            <div class="dish-likes">
              <span :class="{ liked: myReviews[dish.dish_name] === 1 }">👍 {{ dish.like_count }}</span>
              <span :class="{ disliked: myReviews[dish.dish_name] === -1 }">👎 {{ dish.dislike_count }}</span>
              <!-- 点评入口仅当日 -->
              <span v-if="isToday && getToken()" class="review-actions">
                <van-button size="mini" type="success" plain @click="review(dish, 1)">赞</van-button>
                <van-button size="mini" type="danger" plain @click="review(dish, -1)">踩</van-button>
              </span>
            </div>
          </div>
        </div>
        <!-- 报名条（按餐次维度） -->
        <div v-if="getToken()" class="signup-bar">
          <van-button
            block
            :type="signupState(m.key) === 'signed' ? 'success' : 'primary'"
            :disabled="signupState(m.key) === 'locked'"
            @click="toggleSignup(m.key)"
          >
            {{ signupButtonText(m.key) }}
          </van-button>
        </div>
        <div v-else class="signup-bar">
          <van-button block plain type="primary" @click="$router.push('/login')">登录后可报餐</van-button>
        </div>
      </van-tab>
    </van-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { get, post, del, getToken, type Dish, type SignupView } from '@canteen/api-client'
import { showToast } from 'vant'

const meals = [
  { key: 'breakfast', label: '早餐' },
  { key: 'lunch', label: '午餐' },
  { key: 'dinner', label: '晚餐' },
]

const dates = computed(() => {
  const out: { date: string; label: string; short: string; isToday: boolean }[] = []
  const now = new Date()
  for (let i = 0; i < 15; i++) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + i)
    const iso = fmt(d)
    out.push({
      date: iso,
      label: i === 0 ? '今天' : weekdayName(d.getDay()),
      short: `${d.getMonth() + 1}/${d.getDate()}`,
      isToday: i === 0,
    })
  }
  return out
})

function fmt(d: Date) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
function weekdayName(w: number) {
  return ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][w]
}

const selectedDate = ref(fmt(new Date()))
const mealTab = ref('lunch')
const allDishes = ref<Dish[]>([])
const mySignups = ref<Record<string, string>>({}) // meal -> status
const myReviews = ref<Record<string, number>>({})

const isToday = computed(() => selectedDate.value === fmt(new Date()))

async function loadMenus() {
  const start = fmt(new Date())
  const end = fmt(new Date(Date.now() + 14 * 86400000))
  const data = await get('/menus', { start, end })
  allDishes.value = data.dishes || []
}

async function loadMySignups() {
  if (!getToken()) return
  try {
    const data = await get('/my/signups', { from: selectedDate.value, to: selectedDate.value })
    mySignups.value = {}
    for (const r of (data.records || []) as SignupView[]) {
      mySignups.value[r.meal] = r.status
    }
  } catch {
    /* 未登录等场景忽略 */
  }
}

function dishesOf(meal: string): Dish[] {
  return allDishes.value.filter((d) => d.menu_date === selectedDate.value && d.meal === meal)
}

// 报名状态机：已过当日 00:00 → locked（已锁定）
function signupState(meal: string): string {
  if (selectedDate.value <= fmt(new Date())) return 'locked'
  return mySignups.value[meal] === 'signed' ? 'signed' : 'none'
}

function signupButtonText(meal: string) {
  const s = signupState(meal)
  if (s === 'locked') return '已锁定'
  if (s === 'signed') return '已报名（点击取消）'
  return '报名'
}

async function toggleSignup(meal: string) {
  try {
    if (signupState(meal) === 'signed') {
      await del('/signups', { menu_date: selectedDate.value, meal })
      showToast('已取消报名')
    } else {
      await post('/signups', { menu_date: selectedDate.value, meal })
      showToast('报名成功')
    }
    await loadMySignups()
  } catch {
    /* 错误提示由统一封装处理 */
  }
}

async function review(dish: Dish, rating: number) {
  try {
    await post('/reviews', { menu_date: selectedDate.value, meal: dish.meal, dish_name: dish.dish_name, rating })
    myReviews.value[dish.dish_name] = rating
    dish.like_count += rating === 1 ? 1 : 0
    dish.dislike_count += rating === -1 ? 1 : 0
    showToast(rating === 1 ? '已点赞' : '已点踩')
  } catch {
    /* 已提示 */
  }
}

onMounted(async () => {
  await loadMenus()
  await loadMySignups()
})
</script>

<style scoped>
.date-tabs {
  display: flex;
  overflow-x: auto;
  background: #fff;
  padding: 8px 4px;
}
.date-tab {
  min-width: 56px;
  text-align: center;
  padding: 6px 8px;
  border-radius: 8px;
  margin: 0 4px;
  background: #f2f3f5;
}
.date-tab.active {
  background: #1989fa;
  color: #fff;
}
.date-label {
  font-size: 12px;
}
.date-num {
  font-size: 14px;
  font-weight: 600;
}
.dish-list {
  padding: 10px 12px;
}
.empty {
  text-align: center;
  color: #999;
  padding: 30px 0;
}
.dish-card {
  background: #fff;
  border-radius: 10px;
  padding: 12px;
  margin-bottom: 10px;
}
.dish-card.off {
  opacity: 0.5;
}
.dish-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.dish-name {
  font-size: 16px;
  font-weight: 600;
}
.off-tag {
  background: #eee;
  color: #999;
  font-size: 12px;
  padding: 1px 6px;
  border-radius: 4px;
}
.dish-tags {
  margin-top: 6px;
}
.tag {
  margin-right: 6px;
}
.dish-likes {
  margin-top: 8px;
  display: flex;
  gap: 14px;
  align-items: center;
  color: #666;
  font-size: 13px;
}
.liked {
  color: #07c160;
  font-weight: 700;
}
.disliked {
  color: #ee0a24;
  font-weight: 700;
}
.review-actions {
  margin-left: auto;
  display: flex;
  gap: 6px;
}
.signup-bar {
  padding: 10px 12px 20px;
}
</style>
