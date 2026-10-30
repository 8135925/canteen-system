<template>
  <div>
    <el-card>
      <h3>员工管理</h3>
      <el-form inline>
        <el-form-item><el-input v-model="keyword" placeholder="工号/姓名搜索" @keyup.enter="load" /></el-form-item>
        <el-form-item><el-button type="primary" @click="load">查询</el-button></el-form-item>
        <el-form-item><el-button @click="addDialog = true">手动添加</el-button></el-form-item>
        <el-form-item>
          <el-upload :show-file-list="false" :before-upload="doImport" accept=".csv">
            <el-button type="success">CSV 批量导入</el-button>
          </el-upload>
        </el-form-item>
      </el-form>

      <el-table :data="users" border>
        <el-table-column prop="emp_no" label="工号" width="120" />
        <el-table-column prop="name" label="姓名" width="120" />
        <el-table-column prop="department" label="部门" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'">{{ row.status === 'active' ? '正常' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        style="margin-top: 12px"
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        v-model:current-page="page"
        @current-change="load"
      />
    </el-card>

    <el-dialog v-model="addDialog" title="手动添加员工" width="420">
      <el-form label-width="60px">
        <el-form-item label="工号"><el-input v-model="form.emp_no" /></el-form-item>
        <el-form-item label="姓名"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="部门"><el-input v-model="form.department" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialog = false">取消</el-button>
        <el-button type="primary" @click="submitAdd">添加（默认密码 123456）</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="editDialog" title="编辑员工" width="420">
      <el-form label-width="60px">
        <el-form-item label="工号"><el-input v-model="editForm.emp_no" disabled /></el-form-item>
        <el-form-item label="姓名"><el-input v-model="editForm.name" /></el-form-item>
        <el-form-item label="部门"><el-input v-model="editForm.department" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialog = false">取消</el-button>
        <el-button type="primary" @click="submitEdit">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialog" title="导入结果" width="520">
      <p>成功导入：<b>{{ importResult.success_count }}</b> 条</p>
      <el-table v-if="importResult.failures?.length" :data="importResult.failures" size="small" border max-height="300">
        <el-table-column prop="row" label="行号" width="80" />
        <el-table-column prop="reason" label="失败原因" />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, post, patch, upload } from '@canteen/api-client'
import { ElMessage } from 'element-plus'

const users = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const keyword = ref('')
const addDialog = ref(false)
const editDialog = ref(false)
const importDialog = ref(false)
const importResult = ref<any>({})
const form = ref({ emp_no: '', name: '', department: '' })
const editForm = ref({ id: 0, emp_no: '', name: '', department: '' })

async function load() {
  const data = await get('/admin/employees', { keyword: keyword.value, page: String(page.value), page_size: String(pageSize) })
  users.value = data.users || []
  total.value = data.total || 0
}

async function submitAdd() {
  await post('/admin/employees', form.value)
  ElMessage.success('添加成功')
  addDialog.value = false
  form.value = { emp_no: '', name: '', department: '' }
  await load()
}

function openEdit(row: any) {
  editForm.value = { id: row.id, emp_no: row.emp_no, name: row.name, department: row.department }
  editDialog.value = true
}

async function submitEdit() {
  await patch(`/admin/employees/${editForm.value.id}`, {
    name: editForm.value.name,
    department: editForm.value.department,
  })
  ElMessage.success('保存成功')
  editDialog.value = false
  await load()
}

async function doImport(file: File) {
  const data = await upload('/admin/employees/import', file)
  importResult.value = data
  importDialog.value = true
  await load()
  return false // 阻止 el-upload 默认上传
}

onMounted(load)
</script>
