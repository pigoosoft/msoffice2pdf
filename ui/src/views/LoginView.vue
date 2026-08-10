<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import type { Envelope } from '@/api/types'
import LocaleSwitcher from '@/components/LocaleSwitcher.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const uid = ref('')
const pwd = ref('')
const loading = ref(false)

async function onSubmit() {
  if (!uid.value || !pwd.value) {
    ElMessage.warning(t('login.needCredentials'))
    return
  }
  loading.value = true
  try {
    await auth.login(uid.value, pwd.value)
    const raw =
      typeof route.query.redirect === 'string' ? route.query.redirect : '/board'
    // Same-origin relative only: must start with "/" but not "//" (protocol-relative / open redirect).
    const redirect = raw.startsWith('/') && !raw.startsWith('//') ? raw : '/board'
    await router.replace(redirect)
  } catch (e) {
    const msg = (e as Envelope)?.message || (e as Error)?.message || t('login.failed')
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-lang">
      <LocaleSwitcher />
    </div>
    <el-card class="login-card" shadow="hover">
      <h1 class="login-title">MSOffice2Pdf</h1>
      <el-form label-position="top" @submit.prevent="onSubmit">
        <el-form-item :label="t('login.uid')">
          <el-input v-model="uid" autocomplete="username" clearable />
        </el-form-item>
        <el-form-item :label="t('login.password')">
          <el-input
            v-model="pwd"
            type="password"
            autocomplete="current-password"
            show-password
            clearable
          />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" class="login-btn">
          {{ t('login.submit') }}
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.login-page {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  background: var(--el-bg-color-page);
}

.login-lang {
  position: absolute;
  top: 16px;
  right: 16px;
}

.login-card {
  width: 360px;
}

.login-title {
  margin: 0 0 24px;
  font-size: 1.5rem;
  text-align: center;
}

.login-btn {
  width: 100%;
}
</style>
