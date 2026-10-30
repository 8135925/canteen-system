<template>
  <div>
    <el-card>
      <el-form inline>
        <el-form-item label="范围">
          <el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始" end-placeholder="结束" />
        </el-form-item>
        <el-form-item><el-button type="primary" @click="load">刷新</el-button></el-form-item>
      </el-form>
    </el-card>

    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="12">
        <el-card><h3>每日就餐人数趋势</h3><div ref="trendRef" style="height: 300px" /></el-card>
      </el-col>
      <el-col :span="12">
        <el-card><h3>报餐未到率（浪费率）</h3><div ref="wasteRef" style="height: 300px" /></el-card>
      </el-col>
    </el-row>
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="12">
        <el-card><h3>各部门就餐情况对比</h3><div ref="deptRef" style="height: 300px" /></el-card>
      </el-col>
      <el-col :span="12">
        <el-card><h3>菜品受欢迎程度排行（赞/踩）</h3>
          <el-table :data="dishes" size="small" border max-height="300">
            <el-table-column prop="dish_name" label="菜品" />
            <el-table-column prop="like_count" label="赞" width="80" />
            <el-table-column prop="dislike_count" label="踩" width="80" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import * as echarts from 'echarts'
import { get, type DishRankRow } from '@canteen/api-client'

const range = ref([new Date(Date.now() - 29 * 86400000).toISOString().slice(0, 10), new Date().toISOString().slice(0, 10)])
const trendRef = ref<HTMLElement>()
const wasteRef = ref<HTMLElement>()
const deptRef = ref<HTMLElement>()
const dishes = ref<DishRankRow[]>([])

async function load() {
  const from = range.value[0]
  const to = range.value[1]
  const [trend, waste, dept] = await Promise.all([
    get('/super/dashboard/trend', { from, to }),
    get('/super/dashboard/waste-rate', { from, to }),
    get('/super/dashboard/departments', { from, to }),
  ])
  dishes.value = (await get('/super/dashboard/dishes', { from, to })).dishes || []

  // 趋势
  echarts.init(trendRef.value!).setOption({
    xAxis: { type: 'category', data: (trend.trend || []).map((r: any) => r.date) },
    yAxis: { type: 'value', minInterval: 1 },
    series: [{ data: (trend.trend || []).map((r: any) => r.dining), type: 'line', smooth: true, name: '就餐人数' }],
    tooltip: { trigger: 'axis' },
  })
  // 浪费率（%）
  echarts.init(wasteRef.value!).setOption({
    xAxis: { type: 'category', data: (waste.waste_rate || []).map((r: any) => r.date) },
    yAxis: { type: 'value', axisLabel: { formatter: '{value}%' } },
    series: [{ data: (waste.waste_rate || []).map((r: any) => Math.round(r.waste_rate * 10000) / 100), type: 'bar', name: '未到率%' }],
    tooltip: { trigger: 'axis' },
  })
  // 部门对比
  echarts.init(deptRef.value!).setOption({
    xAxis: { type: 'category', data: (dept.departments || []).map((r: any) => r.department) },
    yAxis: { type: 'value', minInterval: 1 },
    series: [{ data: (dept.departments || []).map((r: any) => r.dining), type: 'bar', name: '就餐人次' }],
    tooltip: { trigger: 'axis' },
  })
}

onMounted(load)
</script>
