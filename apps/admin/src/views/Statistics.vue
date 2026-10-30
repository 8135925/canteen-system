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

    <!-- 用餐明细：某天谁报名来没来 / 没报名来吃了 / 没报名也没来 -->
    <el-card style="margin-top: 16px">
      <div class="card-head">
        <h3>用餐明细（{{ detailDate }}）</h3>
      </div>
      <el-form inline>
        <el-form-item label="部门">
          <el-select v-model="detailDept" clearable placeholder="全部部门" style="width: 160px" @change="detailPage = 1; loadDetail()">
            <el-option v-for="d in departments" :key="d.department" :label="d.department" :value="d.department" />
          </el-select>
        </el-form-item>
        <el-form-item label="工号/姓名">
          <el-input v-model="detailKeyword" placeholder="搜索" style="width: 160px" clearable @keyup.enter="loadDetail" @clear="loadDetail" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadDetail">查询</el-button>
        </el-form-item>
        <el-form-item>
          <el-button @click="exportCsv">导出 CSV</el-button>
        </el-form-item>
      </el-form>

      <el-table :data="detailRows" border>
        <el-table-column prop="emp_no" label="工号" width="100" />
        <el-table-column prop="name" label="姓名" width="110" />
        <el-table-column prop="department" label="部门" width="130" />
        <el-table-column align="center" label="早餐">
          <template #default="{ row }">
            <el-tag v-if="row.breakfast" :type="stType(row.breakfast)" size="small">{{ stText(row.breakfast) }}</el-tag>
            <el-tag v-else type="info" size="small">未报名</el-tag>
          </template>
        </el-table-column>
        <el-table-column align="center" label="午餐">
          <template #default="{ row }">
            <el-tag v-if="row.lunch" :type="stType(row.lunch)" size="small">{{ stText(row.lunch) }}</el-tag>
            <el-tag v-else type="info" size="small">未报名</el-tag>
          </template>
        </el-table-column>
        <el-table-column align="center" label="晚餐">
          <template #default="{ row }">
            <el-tag v-if="row.dinner" :type="stType(row.dinner)" size="small">{{ stText(row.dinner) }}</el-tag>
            <el-tag v-else type="info" size="small">未报名</el-tag>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        style="margin-top: 12px"
        layout="total, prev, pager, next, sizes"
        :total="detailTotal"
        :page-size="detailPageSize"
        :current-page="detailPage"
        :page-sizes="[20, 50, 100]"
        @current-change="(p: number) => { detailPage = p; loadDetail() }"
        @size-change="(s: number) => { detailPageSize = s; detailPage = 1; loadDetail() }"
      />
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

// —— 用餐明细 ——
const detailDate = ref(date.value)
const detailDept = ref('')
const detailKeyword = ref('')
const detailPage = ref(1)
const detailPageSize = ref(20)
const detailTotal = ref(0)
const detailRows = ref<any[]>([])

const ST_MAP: Record<string, { text: string; type: 'success' | 'danger' | 'warning' | 'info' | 'primary' }> = {
  verified: { text: '报名已来', type: 'success' },
  no_show: { text: '报名未到', type: 'danger' },
  walk_in: { text: '未报名就餐', type: 'warning' },
  signed: { text: '已报名', type: 'primary' },
  cancelled: { text: '已取消', type: 'info' },
}
function stText(s: string) {
  return ST_MAP[s]?.text ?? '未报名'
}
function stType(s: string): any {
  return ST_MAP[s]?.type ?? 'info'
}

async function loadDetail() {
  detailDate.value = date.value
  const data = await get('/admin/stats/attendance-detail', {
    date: detailDate.value,
    dept: detailDept.value,
    keyword: detailKeyword.value,
    page: String(detailPage.value),
    page_size: String(detailPageSize.value),
  })
  detailRows.value = data.rows || []
  detailTotal.value = data.total || 0
}

function exportCsv() {
  const head = '工号,姓名,部门,早餐,午餐,晚餐\n'
  const body = detailRows.value
    .map((r) =>
      [r.emp_no, r.name, r.department, stText(r.breakfast), stText(r.lunch), stText(r.dinner)]
        .map((s) => (s.includes(',') ? `"${s}"` : s))
        .join(','),
    )
    .join('\n')
  // 带 BOM，Excel 打开中文不乱码
  const blob = new Blob(['\uFEFF' + head + body], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `用餐明细_${detailDate.value}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

async function load() {
  signups.value = (await get('/admin/stats/signups', { date: date.value })).signups || []
  departments.value = (await get('/admin/stats/departments', { date: date.value })).departments || []
  const to = date.value
  const from = new Date(Date.now() - 29 * 86400000).toISOString().slice(0, 10)
  daily.value = (await get('/admin/stats/daily', { from, to })).daily || []
  await loadDetail()
}

onMounted(load)
</script>

<style scoped>
.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.card-head h3 {
  margin: 0;
}
</style>
