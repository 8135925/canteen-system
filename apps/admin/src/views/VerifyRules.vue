<template>
  <div>
    <el-card>
      <h3>核销规则（超过截止时间未核销计违约）</h3>
      <el-form label-width="120px" style="max-width: 480px">
        <el-form-item v-for="r in rules" :key="r.meal" :label="mealLabel(r.meal)">
          <el-time-select
            v-model="r.deadline"
            start="00:00"
            step="00:30"
            end="23:30"
            placeholder="核销截止时间"
          />
          <el-tag v-if="r.is_default" type="info" style="margin-left: 8px">默认值</el-tag>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="save">保存覆盖</el-button>
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false"
        title="默认值 = 当餐结束时间（早 09:00 / 午 13:30 / 晚 19:00）。规则调整即时生效于违约推导，建议仅在餐次核销截止前调整。" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, put } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

const rules = ref<{ meal: string; deadline: string; is_default: boolean }[]>([])
const mealLabel = (m: string) => ({ breakfast: '早餐', lunch: '午餐', dinner: '晚餐' }[m] || m)

async function load() {
  rules.value = await get('/admin/verify-rules')
}

async function save() {
  await put('/admin/verify-rules', {
    rules: rules.value.map((r) => ({ meal: r.meal, deadline: r.deadline })),
  })
  ElMessage.success('已保存')
  await load()
}

onMounted(load)
</script>
