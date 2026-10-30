<template>
  <div>
    <!-- 顶部统计卡 × 4 -->
    <el-row :gutter="14">
      <el-col v-for="c in cards" :key="c.label" :xs="12" :sm="12" :md="6">
        <el-card class="stat-card" shadow="hover">
          <div class="stat-body">
            <div class="stat-info">
              <div class="stat-label">{{ c.label }}</div>
              <div class="stat-num">{{ c.value }}</div>
              <div class="stat-sub" :style="{ color: c.subColor }">
                {{ c.sub }}
              </div>
            </div>
            <div class="stat-icon" :style="{ background: c.bg }">
              <el-icon :size="22" color="#fff"><component :is="c.icon" /></el-icon>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 中部：分析概览（柱状图） + 就餐率进度条 -->
    <el-row :gutter="14" style="margin-top: 14px">
      <el-col :md="16" :xs="24">
        <el-card shadow="hover">
          <div class="card-head">
            <h3>分析概览</h3>
            <el-radio-group v-model="weekMode" size="small" @change="load">
              <el-radio-button label="week">本周</el-radio-button>
              <el-radio-button label="last">上周</el-radio-button>
            </el-radio-group>
          </div>
          <div ref="barRef" style="height: 330px" />
        </el-card>
      </el-col>
      <el-col :md="8" :xs="24">
        <el-card shadow="hover">
          <div class="card-head">
            <h3>就餐率</h3>
          </div>
          <div class="rate-list">
            <div v-for="r in rates" :key="r.date" class="rate-row">
              <span class="rate-date">{{ weekday(r.date) }}</span>
              <el-progress
                class="rate-bar"
                :percentage="Math.round(r.rate * 100)"
                :stroke-width="12"
                :color="rateColor"
              />
            </div>
            <el-empty v-if="rates.length === 0" description="暂无数据" :image-size="60" />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 底部：数据统计表 + 最新动态 -->
    <el-row :gutter="14" style="margin-top: 14px">
      <el-col :md="16" :xs="24">
        <el-card shadow="hover">
          <div class="card-head">
            <h3>数据统计</h3>
          </div>
          <el-table :data="dailyRows" size="default">
            <el-table-column prop="date" label="日期" width="120" />
            <el-table-column prop="signups" label="报名人数" />
            <el-table-column prop="verified" label="核销数" />
            <el-table-column prop="walk_in" label="未报名就餐" />
            <el-table-column label="未到（违约）">
              <template #default="{ row }">
                <span :style="{ color: row.diff > 0 ? '#f56c6c' : '#67c23a' }">{{ row.diff }}</span>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
      <el-col :md="8" :xs="24">
        <el-card shadow="hover">
          <div class="card-head">
            <h3>最新动态</h3>
          </div>
          <el-timeline>
            <el-timeline-item
              v-for="s in suggestions"
              :key="s.id"
              :timestamp="new Date(s.created_at).toLocaleDateString()"
              :type="s.status === 'adopted' ? 'success' : s.status === 'rejected' ? 'danger' : 'primary'"
            >
              {{ s.content }}
            </el-timeline-item>
          </el-timeline>
          <el-empty v-if="suggestions.length === 0" description="暂无动态" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import * as echarts from 'echarts'
import {
  User, CircleCheck, WarningFilled, Star, Coin,
} from '@element-plus/icons-vue'
import { get, type Suggestion } from '@canteen/api-client'

const today = new Date().toISOString().slice(0, 10)
const weekMode = ref<'week' | 'last'>('week')

const cards = computed(() => [
  {
    label: '今日报名人数',
    value: String(todaySignup.value),
    sub: subText.value,
    subColor: '#67c23a',
    bg: 'linear-gradient(135deg,#409eff,#79bbff)',
    icon: User,
  },
  {
    label: '今日核销数',
    value: String(todayVerify.value),
    sub: '现场核销完成',
    subColor: '#409eff',
    bg: 'linear-gradient(135deg,#f56c6c,#fab6b6)',
    icon: CircleCheck,
  },
  {
    label: '本月未到（违约）',
    value: String(monthNoShow.value),
    sub: '报餐未到 + 未报名就餐',
    subColor: '#e6a23c',
    bg: 'linear-gradient(135deg,#e6a23c,#f3d19e)',
    icon: WarningFilled,
  },
  {
    label: '菜品满意度',
    value: Math.round(satisfaction.value * 1000) / 10 + '%',
    sub: '按全部赞踩统计',
    subColor: '#67c23a',
    bg: 'linear-gradient(135deg,#67c23a,#95d475)',
    icon: Star,
  },
])

const todaySignup = ref(0)
const todayVerify = ref(0)
const yesterdaySignup = ref(0)
const monthNoShow = ref(0)
const satisfaction = ref(0)
const dailyRows = ref<any[]>([])
const rates = ref<{ date: string; rate: number; signups: number }[]>([])
const suggestions = ref<Suggestion[]>([])

const subText = computed(() => {
  if (yesterdaySignup.value === 0) return '昨日无报名'
  const diff = todaySignup.value - yesterdaySignup.value
  const pct = Math.round(Math.abs(diff) / yesterdaySignup.value * 100)
  return (diff >= 0 ? '+%' : '-%').replace('%', pct + '%') + ' 较昨日'
})

const barRef = ref<HTMLElement>()
let barChart: echarts.ECharts | null = null

function rangeOf(mode: 'week' | 'last') {
  const to = mode === 'week' ? today : new Date(Date.now() - 7 * 86400000).toISOString().slice(0, 10)
  const from = new Date(new Date(to).getTime() - 6 * 86400000).toISOString().slice(0, 10)
  return { from, to }
}

async function load() {
  const data = await get('/admin/welcome')
  todaySignup.value = data.today_signup ?? 0
  todayVerify.value = data.today_verify ?? 0
  yesterdaySignup.value = data.yesterday_signup ?? 0
  monthNoShow.value = data.month_noshow ?? 0
  satisfaction.value = data.satisfaction ?? 0
  suggestions.value = data.latest_suggestions || []

  const { from, to } = rangeOf(weekMode.value)
  const daily = await get('/admin/stats/daily', { from, to })
  dailyRows.value = (daily.daily || []).slice().reverse()
  rates.value = (data.rates || []).filter(
    (r: any) => r.date >= from && r.date <= to,
  )

  await nextTick()
  if (!barRef.value) return
  if (!barChart) barChart = echarts.init(barRef.value)
  barChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['报名人数', '实际就餐'], bottom: 0 },
    grid: { left: 40, right: 20, top: 20, bottom: 40 },
    xAxis: { type: 'category', data: dailyRows.value.slice().reverse().map((r) => weekday(r.date)) },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      { name: '报名人数', type: 'bar', data: dailyRows.value.slice().reverse().map((r) => r.signups), itemStyle: { color: '#409eff', borderRadius: [3, 3, 0, 0] }, barMaxWidth: 26 },
      { name: '实际就餐', type: 'bar', data: dailyRows.value.slice().reverse().map((r) => r.verified + r.walk_in), itemStyle: { color: '#ff9e6b', borderRadius: [3, 3, 0, 0] }, barMaxWidth: 26 },
    ],
  })
}

function weekday(date: string) {
  const names = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
  return names[new Date(date).getDay()]
}

const rateColor = [
  { color: '#5bc0de', percent: 60 },
  { color: '#67c23a', percent: 100 },
]

onMounted(load)
</script>

<style scoped>
.stat-card :deep(.el-card__body) {
  padding: 18px;
}
.stat-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.stat-label {
  font-size: 13px;
  color: #909399;
}
.stat-num {
  font-size: 26px;
  font-weight: 700;
  color: #303133;
  margin: 6px 0 4px;
}
.stat-sub {
  font-size: 12px;
}
.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.card-head h3 {
  margin: 0;
}
.rate-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-top: 6px;
}
.rate-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.rate-date {
  width: 40px;
  font-size: 13px;
  color: #606266;
  flex-shrink: 0;
}
.rate-bar {
  flex: 1;
}
</style>
