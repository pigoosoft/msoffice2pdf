<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import PagedTable, { type PagedColumn } from '@/components/PagedTable.vue'
import {
  listUsers,
  createUser,
  updateUser,
  deleteUser,
  freezeUser,
  resetUserToken,
  type AdminUser,
} from '@/api/admin'
import type { Envelope, Role } from '@/api/types'

const { t, locale } = useI18n()
const tableRef = ref<{ refresh: (resetPage?: boolean) => Promise<void> } | null>(null)

const createVisible = ref(false)
const createSubmitting = ref(false)
const createForm = reactive({
  uid: '',
  pwd: '',
  role: 'user' as Role,
})

const editVisible = ref(false)
const editSubmitting = ref(false)
const editUid = ref('')
const editForm = reactive({
  role: 'user' as Role,
})

const pwdVisible = ref(false)
const pwdSubmitting = ref(false)
const pwdUid = ref('')
const pwdForm = reactive({
  pwd: '',
  pwdConfirm: '',
})

function errMsg(e: unknown): string {
  return (e as Envelope)?.message || (e as Error)?.message || t('common.requestFailed')
}

function formatTime(iso: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const pad = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function isFrozen(row: AdminUser): boolean {
  return Number(row.status) === 1
}

function roleLabel(role: string): string {
  return role === 'admin' ? t('users.roleAdmin') : t('users.roleUser')
}

async function showOneTimeToken(title: string, token: string) {
  await ElMessageBox.alert(t('users.tokenOnceBody', { token }), title, {
    confirmButtonText: t('users.tokenCopied'),
    type: 'warning',
    dangerouslyUseHTMLString: false,
  })
}

const columns = computed<PagedColumn[]>(() => {
  void locale.value
  return [
    { prop: 'uid', label: t('common.uid'), minWidth: 120, showOverflowTooltip: true },
    {
      prop: 'role',
      label: t('common.role'),
      width: 100,
      formatter: (row) => roleLabel(String((row as AdminUser).role || '')),
    },
    { prop: 'status', label: t('common.status'), width: 100, slot: 'status' },
    { prop: 'token', label: t('common.token'), minWidth: 140, showOverflowTooltip: true },
    {
      prop: 'created_at',
      label: t('users.createdAt'),
      width: 170,
      formatter: (row) => formatTime(String((row as AdminUser).created_at || '')),
    },
    {
      label: t('common.actions'),
      // EN labels (Set password / Reset token / Delete) need more than zh.
      minWidth: locale.value === 'zh-CN' ? 320 : 390,
      fixed: 'right',
      slot: 'actions',
    },
  ]
})

function loader(page: number, pageSize: number) {
  return listUsers(page, pageSize)
}

function refresh(resetPage = false) {
  void tableRef.value?.refresh(resetPage)
}

function openCreate() {
  createForm.uid = ''
  createForm.pwd = ''
  createForm.role = 'user'
  createVisible.value = true
}

async function submitCreate() {
  const uid = createForm.uid.trim()
  const pwd = createForm.pwd
  if (!uid) {
    ElMessage.warning(t('users.needUid'))
    return
  }
  if (!pwd) {
    ElMessage.warning(t('users.needPassword'))
    return
  }
  createSubmitting.value = true
  try {
    const user = await createUser({ uid, pwd, role: createForm.role })
    createVisible.value = false
    ElMessage.success(t('users.created'))
    refresh(true)
    if (user.token) {
      await showOneTimeToken(t('users.tokenOnceTitle'), user.token)
    }
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    createSubmitting.value = false
  }
}

function openEdit(row: AdminUser) {
  editUid.value = row.uid
  editForm.role = (row.role === 'admin' ? 'admin' : 'user') as Role
  editVisible.value = true
}

async function submitEdit() {
  editSubmitting.value = true
  try {
    await updateUser(editUid.value, { role: editForm.role })
    editVisible.value = false
    ElMessage.success(t('users.updated'))
    refresh()
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    editSubmitting.value = false
  }
}

function openSetPwd(row: AdminUser) {
  pwdUid.value = row.uid
  pwdForm.pwd = ''
  pwdForm.pwdConfirm = ''
  pwdVisible.value = true
}

async function submitSetPwd() {
  const pwd = pwdForm.pwd
  if (!pwd) {
    ElMessage.warning(t('users.needNewPassword'))
    return
  }
  if (pwd !== pwdForm.pwdConfirm) {
    ElMessage.warning(t('users.passwordMismatch'))
    return
  }
  pwdSubmitting.value = true
  try {
    await updateUser(pwdUid.value, { pwd })
    pwdVisible.value = false
    ElMessage.success(t('users.passwordUpdated'))
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    pwdSubmitting.value = false
  }
}

async function onFreeze(row: AdminUser) {
  const frozen = !isFrozen(row)
  const action = frozen ? t('users.freeze') : t('users.unfreeze')
  try {
    await ElMessageBox.confirm(
      t('users.freezeConfirm', { action, uid: row.uid }),
      t('common.confirm'),
      {
        type: 'warning',
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
      },
    )
  } catch {
    return
  }
  try {
    await freezeUser(row.uid, frozen)
    ElMessage.success(t('users.freezeDone', { action }))
    refresh()
  } catch (e) {
    ElMessage.error(errMsg(e))
  }
}

async function onResetToken(row: AdminUser) {
  try {
    await ElMessageBox.confirm(
      t('users.resetTokenConfirm', { uid: row.uid }),
      t('users.resetTokenTitle'),
      {
        type: 'warning',
        confirmButtonText: t('users.reset'),
        cancelButtonText: t('common.cancel'),
      },
    )
  } catch {
    return
  }
  try {
    const user = await resetUserToken(row.uid)
    refresh()
    if (user.token) {
      await showOneTimeToken(t('users.tokenOnceNewTitle'), user.token)
    } else {
      ElMessage.success(t('users.tokenReset'))
    }
  } catch (e) {
    ElMessage.error(errMsg(e))
  }
}

async function onDelete(row: AdminUser) {
  try {
    await ElMessageBox.confirm(
      t('users.deleteConfirm', { uid: row.uid }),
      t('users.deleteTitle'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel'),
      },
    )
  } catch {
    return
  }
  try {
    await deleteUser(row.uid)
    ElMessage.success(t('users.deleted'))
    refresh()
  } catch (e) {
    ElMessage.error(errMsg(e))
  }
}
</script>

<template>
  <div class="users-page">
    <div class="page-head">
      <h2 class="page-title">{{ t('users.title') }}</h2>
      <div class="page-actions">
        <el-button type="primary" @click="openCreate">{{ t('users.createUser') }}</el-button>
        <el-button @click="refresh()">{{ t('common.refresh') }}</el-button>
      </div>
    </div>

    <PagedTable
      ref="tableRef"
      :columns="columns"
      :loader="loader"
      :empty-text="t('users.empty')"
    >
      <template #status="{ row }">
        <el-tag :type="isFrozen(row as AdminUser) ? 'danger' : 'success'" size="small">
          {{ isFrozen(row as AdminUser) ? t('users.statusFrozen') : t('users.statusNormal') }}
        </el-tag>
      </template>
      <template #actions="{ row }">
        <el-button link type="primary" @click="openEdit(row as AdminUser)">
          {{ t('common.edit') }}
        </el-button>
        <el-button link type="primary" @click="openSetPwd(row as AdminUser)">
          {{ t('users.setPassword') }}
        </el-button>
        <el-button link type="warning" @click="onFreeze(row as AdminUser)">
          {{ isFrozen(row as AdminUser) ? t('users.unfreeze') : t('users.freeze') }}
        </el-button>
        <el-button link type="primary" @click="onResetToken(row as AdminUser)">
          {{ t('users.resetToken') }}
        </el-button>
        <el-button link type="danger" @click="onDelete(row as AdminUser)">
          {{ t('common.delete') }}
        </el-button>
      </template>
    </PagedTable>

    <el-dialog
      v-model="createVisible"
      :title="t('users.createTitle')"
      width="420px"
      destroy-on-close
    >
      <el-form label-width="72px" @submit.prevent>
        <el-form-item :label="t('common.uid')" required>
          <el-input
            v-model="createForm.uid"
            autocomplete="off"
            :placeholder="t('users.uidPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('common.password')" required>
          <el-input
            v-model="createForm.pwd"
            type="password"
            show-password
            autocomplete="new-password"
            :placeholder="t('users.pwdPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('common.role')">
          <el-select v-model="createForm.role" style="width: 100%">
            <el-option :label="t('users.roleUser')" value="user" />
            <el-option :label="t('users.roleAdmin')" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="createSubmitting" @click="submitCreate">
          {{ t('common.create') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="editVisible"
      :title="t('users.editTitle', { uid: editUid })"
      width="420px"
      destroy-on-close
    >
      <el-form label-width="72px" @submit.prevent>
        <el-form-item :label="t('common.role')">
          <el-select v-model="editForm.role" style="width: 100%">
            <el-option :label="t('users.roleUser')" value="user" />
            <el-option :label="t('users.roleAdmin')" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="editSubmitting" @click="submitEdit">
          {{ t('common.save') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="pwdVisible"
      :title="t('users.setPwdTitle', { uid: pwdUid })"
      width="420px"
      destroy-on-close
    >
      <el-form label-width="88px" @submit.prevent>
        <el-form-item :label="t('users.newPwd')" required>
          <el-input
            v-model="pwdForm.pwd"
            type="password"
            show-password
            autocomplete="new-password"
            :placeholder="t('users.newPwdPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('users.confirmPwd')" required>
          <el-input
            v-model="pwdForm.pwdConfirm"
            type="password"
            show-password
            autocomplete="new-password"
            :placeholder="t('users.confirmPwdPlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="pwdSubmitting" @click="submitSetPwd">
          {{ t('common.save') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.page-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.page-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
