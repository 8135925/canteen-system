<template>
  <div>
    <el-card>
      <h3>现场核销 / 未报名登记</h3>
      <el-form inline>
        <el-form-item label="工号">
          <el-input ref="empInput" v-model="empNo" placeholder="输入员工工号" style="width: 200px" @keyup.enter="lookup" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="lookup">查询</el-button>
        </el-form-item>
      </el-form>

      <el-descriptions v-if="result" :title="`查询结果：${result.user.name}（${result.user.emp_no}）`" border>
        <el-descriptions-item label="部门">{{ result.user.department || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用餐日期">{{ result.menu_date }}</el-descriptions-item>
        <el-descriptions-item label="餐次">{{ mealLabel(result.meal) }}</el-descriptions-item>
        <el-descriptions-item label="核销截止">{{ result.deadline }}</el-descriptions-item>
        <el-descriptions-item label="当前报名">
          <el-tag v-if="result.signup">{{ result.signup.status }}</el-tag>
          <el-tag v-else type="info">无报名</el-tag>
        </el-descriptions-item>
      </el-descriptions>

      <div v-if="result" style="margin-top: 16px">
        <el-alert v-if="result.reason" :title="result.reason" type="warning" :closable="false" style="margin-bottom: 12px" />
        <el-button v-if="result.action === 'verify'" type="success" @click="doVerify">确认核销</el-button>
        <el-button v-if="result.action === 'walk_in'" type="warning" @click="doWalkIn">登记未报名就餐</el-button>
        <el-tag v-if="result.action === 'already_verified'" type="success">已核销，勿重复操作</el-tag>
        <el-tag v-if="result.action === 'violated'" type="danger">已判违约，不能核销或登记</el-tag>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { get, post } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

const empNo = ref('')
const result = ref<any>(null)

const today = () => new Date().toISOString().slice(0, 10)
const mealLabel = (m: string) => ({ breakfast: '早餐', lunch: '午餐', dinner: '晚餐' }[m] || m)

async function lookup() {
  if (!empNo.value) {
    ElMessage.warning('请输入工号')
    return
  }
  try {
    result.value = await get('/admin/verify/lookup', { emp_no: empNo.value, menu_date: today(), meal: currentMeal.value })
  } catch {
    result.value = null
  }
}

const currentMeal = ref(mealOfNow())
function mealOfNow() {
  const h = new Date().getHours()
  if (h < 10) return 'breakfast'
  if (h < 14) return 'lunch'
  return 'dinner'
}

async function doVerify() {
  await post('/admin/verify', { emp_no: empNo.value, menu_date: today(), meal: currentMeal.value })
  ElMessage.success('核销成功')
  await lookup()
}

async function doWalkIn() {
  await post('/admin/walk-in', { emp_no: empNo.value, menu_date: today(), meal: currentMeal.value })
  ElMessage.success('登记成功')
  await lookup()
}
</script>
