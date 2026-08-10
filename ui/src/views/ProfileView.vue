<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Refresh, View, Hide } from '@element-plus/icons-vue'
import {
  getProfile,
  changePassword,
  resetMyToken,
  type Profile,
} from '@/api/profile'
import type { Envelope } from '@/api/types'

const { t } = useI18n()

const loading = ref(false)
const profile = ref<Profile | null>(null)
const tokenVisible = ref(false)

const pwdSubmitting = ref(false)
const pwdForm = reactive({
  oldPwd: '',
  newPwd: '',
  confirmPwd: '',
})

const tokenResetting = ref(false)

function errMsg(e: unknown): string {
  return (e as Envelope)?.message || (e as Error)?.message || t('common.requestFailed')
}

function roleLabel(role: string): string {
  return role === 'admin' ? t('users.roleAdmin') : t('users.roleUser')
}

function statusLabel(status: number): string {
  return Number(status) === 1 ? t('users.statusFrozen') : t('users.statusNormal')
}

async function load() {
  loading.value = true
  try {
    profile.value = await getProfile()
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    loading.value = false
  }
}

async function copyToken() {
  const token = profile.value?.token
  if (!token) return
  try {
    await navigator.clipboard.writeText(token)
    ElMessage.success(t('profile.tokenCopied'))
  } catch {
    ElMessage.error(t('common.requestFailed'))
  }
}

async function submitPassword() {
  if (!pwdForm.oldPwd || !pwdForm.newPwd) {
    ElMessage.warning(t('profile.needPasswords'))
    return
  }
  if (pwdForm.newPwd !== pwdForm.confirmPwd) {
    ElMessage.warning(t('users.passwordMismatch'))
    return
  }
  pwdSubmitting.value = true
  try {
    await changePassword(pwdForm.oldPwd, pwdForm.newPwd)
    pwdForm.oldPwd = ''
    pwdForm.newPwd = ''
    pwdForm.confirmPwd = ''
    ElMessage.success(t('users.passwordUpdated'))
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    pwdSubmitting.value = false
  }
}

async function onResetToken() {
  try {
    await ElMessageBox.confirm(t('profile.resetTokenConfirm'), t('users.resetTokenTitle'), {
      type: 'warning',
      confirmButtonText: t('users.reset'),
      cancelButtonText: t('common.cancel'),
    })
  } catch {
    return
  }
  tokenResetting.value = true
  try {
    const res = await resetMyToken()
    if (profile.value) {
      profile.value = { ...profile.value, token: res.token }
    }
    tokenVisible.value = true
    ElMessage.success(t('users.tokenReset'))
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    tokenResetting.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div v-loading="loading" class="profile-page">
    <div class="page-head">
      <h2 class="page-title">{{ t('profile.title') }}</h2>
      <el-button @click="load()">{{ t('common.refresh') }}</el-button>
    </div>

    <template v-if="profile">
      <el-row :gutter="16">
        <el-col :xs="24" :md="12">
          <el-card shadow="never" class="profile-card">
            <template #header>
              <span>{{ t('profile.account') }}</span>
            </template>
            <el-descriptions :column="1" border>
              <el-descriptions-item :label="t('common.uid')">{{ profile.uid }}</el-descriptions-item>
              <el-descriptions-item :label="t('common.role')">
                {{ roleLabel(profile.role) }}
              </el-descriptions-item>
              <el-descriptions-item :label="t('common.status')">
                {{ statusLabel(profile.status) }}
              </el-descriptions-item>
            </el-descriptions>
          </el-card>
        </el-col>

        <el-col :xs="24" :md="12">
          <el-card shadow="never" class="profile-card">
            <template #header>
              <span>{{ t('profile.stats') }}</span>
            </template>
            <div class="stat-grid">
              <div class="stat-item">
                <div class="stat-value">{{ profile.upload_count }}</div>
                <div class="stat-label">{{ t('profile.uploadCount') }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-value">{{ profile.convert_success_count }}</div>
                <div class="stat-label">{{ t('profile.convertSuccessCount') }}</div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-card shadow="never" class="profile-card">
        <template #header>
          <div class="card-head">
            <span>{{ t('profile.apiToken') }}</span>
            <el-button
              type="warning"
              plain
              :icon="Refresh"
              :loading="tokenResetting"
              @click="onResetToken"
            >
              {{ t('users.resetToken') }}
            </el-button>
          </div>
        </template>
        <p class="hint">{{ t('profile.tokenHint') }}</p>
        <div class="token-row">
          <el-input
            :model-value="profile.token"
            :type="tokenVisible ? 'text' : 'password'"
            readonly
            class="token-input"
          />
          <el-button :icon="tokenVisible ? Hide : View" @click="tokenVisible = !tokenVisible">
            {{ tokenVisible ? t('profile.hideToken') : t('profile.showToken') }}
          </el-button>
          <el-button type="primary" :icon="CopyDocument" @click="copyToken">
            {{ t('profile.copyToken') }}
          </el-button>
        </div>
      </el-card>

      <el-card shadow="never" class="profile-card">
        <template #header>
          <span>{{ t('profile.changePassword') }}</span>
        </template>
        <el-form label-position="top" class="pwd-form" @submit.prevent="submitPassword">
          <el-form-item :label="t('profile.oldPassword')">
            <el-input
              v-model="pwdForm.oldPwd"
              type="password"
              show-password
              autocomplete="current-password"
              :placeholder="t('profile.oldPasswordPlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="t('users.newPwd')">
            <el-input
              v-model="pwdForm.newPwd"
              type="password"
              show-password
              autocomplete="new-password"
              :placeholder="t('users.newPwdPlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="t('users.confirmPwd')">
            <el-input
              v-model="pwdForm.confirmPwd"
              type="password"
              show-password
              autocomplete="new-password"
              :placeholder="t('users.confirmPwdPlaceholder')"
            />
          </el-form-item>
          <el-button type="primary" native-type="submit" :loading="pwdSubmitting">
            {{ t('profile.savePassword') }}
          </el-button>
        </el-form>
      </el-card>
    </template>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.page-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.profile-card {
  margin-bottom: 16px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.stat-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.stat-item {
  text-align: center;
  padding: 12px 8px;
}

.stat-value {
  font-size: 1.75rem;
  font-weight: 600;
  line-height: 1.2;
}

.stat-label {
  margin-top: 6px;
  color: var(--el-text-color-secondary);
  font-size: 0.875rem;
}

.hint {
  margin: 0 0 12px;
  color: var(--el-text-color-secondary);
  font-size: 0.875rem;
}

.token-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.token-input {
  flex: 1 1 240px;
  min-width: 0;
}

.pwd-form {
  max-width: 420px;
}
</style>
