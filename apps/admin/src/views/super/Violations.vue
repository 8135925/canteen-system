<template>
  <div>
    <el-card>
      <h3>违约查看</h3>
      <el-form inline>
        <el-form-item label="范围">
          <el-date-picker v-model="range" type="daterange" value-format="YYYY-MM-DD" start-placeholder="开始" end-placeholder="结束" />
        </el-form-item>
        <el-form-item><el-button type="primary" @click="load">查询</el-button></el-form-item>
      </el-form>
      <el-table :data="rows" border>
        <el-table-column prop="emp_no" label="工号" width="120" />
        <el-table-column prop="name" label="姓名" width="120" />
        <el-table-column prop="department" label="部门" />
        <el-table-column prop="no_show" label="报餐未核销（违约）" width="160" />
        <el-table-column prop="walk_in" label="未报名就餐（违约）" width="160" />
        <el-table-column prop="total" label="违约总次数">
          <template #default="{ row }"><b style="color: #f56c6c">{{ row.total }}</b></template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, type ViolationRow } from '@canteen/api-client'

const range = ref([new Date(Date.now() - 29 * 86400000).toISOString().slice(0, 10), new Date().toISOString().slice(0, 10)])
const rows = ref<ViolationRow[]>([])

async function load() {
  const data = await get('/super/violations', { from: range.value[0], to: range.value[1] })
  rows.value = data.violations || []
}

onMounted(load)
</script>
