<template>
  <div>
    <el-card>
      <h3>建议管理</h3>
      <el-form inline>
        <el-form-item label="状态">
          <el-select v-model="status" style="width: 140px" @change="load">
            <el-option label="全部" value="" />
            <el-option label="待处理" value="pending" />
            <el-option label="已采纳" value="adopted" />
            <el-option label="已拒绝" value="rejected" />
          </el-select>
        </el-form-item>
        <el-form-item><el-button type="primary" @click="load">查询</el-button></el-form-item>
      </el-form>
      <el-table :data="list" border>
        <el-table-column prop="user_name" label="提交人" width="120" />
        <el-table-column prop="content" label="建议内容" />
        <el-table-column prop="created_at" label="提交时间" width="180">
          <template #default="{ row }">{{ new Date(row.created_at).toLocaleString() }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="140">
          <template #default="{ row }">
            <el-select :model-value="row.status" size="small" @change="(v: string) => mark(row, v)">
              <el-option label="待处理" value="pending" />
              <el-option label="已采纳" value="adopted" />
              <el-option label="已拒绝" value="rejected" />
            </el-select>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, patch, type Suggestion } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

const list = ref<Suggestion[]>([])
const status = ref('')

async function load() {
  const data = await get('/admin/suggestions', { status: status.value })
  list.value = data.suggestions || []
}

async function mark(row: Suggestion, v: string) {
  await patch(`/admin/suggestions/${row.id}`, { status: v })
  ElMessage.success('已更新')
  await load()
}

onMounted(load)
</script>
