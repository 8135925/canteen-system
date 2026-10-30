<template>
  <div>
    <el-card>
      <h3>账号管理</h3>
      <el-form inline>
        <el-form-item>
          <el-select v-model="role" style="width: 160px" @change="load">
            <el-option label="全部（员工+管理员）" value="" />
            <el-option label="普通员工" value="employee" />
            <el-option label="食堂管理员" value="canteen_admin" />
          </el-select>
        </el-form-item>
        <el-form-item><el-input v-model="keyword" placeholder="工号/姓名" @keyup.enter="load" /></el-form-item>
        <el-form-item><el-button type="primary" @click="load">查询</el-button></el-form-item>
        <el-form-item><el-button type="success" @click="createDialog = true">创建食堂管理员</el-button></el-form-item>
      </el-form>

      <el-table :data="users" border>
        <el-table-column prop="emp_no" label="工号" width="120" />
        <el-table-column prop="name" label="姓名" width="120" />
        <el-table-column prop="department" label="部门" />
        <el-table-column prop="role" label="角色" width="120">
          <template #default="{ row }">{{ roleLabel(row.role) }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'">{{ row.status === 'active' ? '正常' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180">
          <template #default="{ row }">
            <el-button v-if="row.status === 'active'" link type="danger" @click="setStatus(row, 'disabled')">禁用</el-button>
            <el-button v-else link type="success" @click="setStatus(row, 'active')">启用</el-button>
            <el-button link type="warning" @click="resetPwd(row)">重置密码</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination style="margin-top: 12px" layout="prev, pager, next, total" :total="total" :page-size="pageSize" v-model:current-page="page" @current-change="load" />
    </el-card>

    <el-dialog v-model="createDialog" title="创建食堂管理员" width="420">
      <el-form label-width="60px">
        <el-form-item label="工号"><el-input v-model="form.emp_no" /></el-form-item>
        <el-form-item label="姓名"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="部门"><el-input v-model="form.department" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialog = false">取消</el-button>
        <el-button type="primary" @click="submitCreate">创建（默认密码 123456）</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, post, patch } from '@canteen/api-client'
import { ElMessage, ElMessageBox } from 'element-plus'

const users = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const role = ref('')
const keyword = ref('')
const createDialog = ref(false)
const form = ref({ emp_no: '', name: '', department: '' })

const roleLabel = (r: string) => ({ employee: '普通员工', canteen_admin: '食堂管理员', super_admin: '超级管理员' }[r] || r)

async function load() {
  const data = await get('/super/users', { role: role.value, keyword: keyword.value, page: String(page.value) })
  users.value = data.users || []
  total.value = data.total || 0
}

async function submitCreate() {
  await post('/super/canteen-admins', form.value)
  ElMessage.success('创建成功')
  createDialog.value = false
  form.value = { emp_no: '', name: '', department: '' }
  await load()
}

async function setStatus(row: any, status: string) {
  await patch(`/super/users/${row.id}/status`, { status })
  ElMessage.success(status === 'disabled' ? '已禁用' : '已启用')
  await load()
}

async function resetPwd(row: any) {
  await ElMessageBox.confirm(`确认将 ${row.name}（${row.emp_no}）的密码重置为 123456？`, '重置密码')
  await post(`/super/users/${row.id}/reset-password`)
  ElMessage.success('已重置为 123456，该账号下次登录需修改密码')
}

onMounted(load)
</script>
