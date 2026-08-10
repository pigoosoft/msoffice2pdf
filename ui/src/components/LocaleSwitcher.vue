<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { setAppLocale, type AppLocale } from '@/i18n'

const { t, locale } = useI18n()

const current = computed({
  get: () => locale.value as AppLocale,
  set: (v: AppLocale) => setAppLocale(v),
})
</script>

<template>
  <el-select
    v-model="current"
    size="small"
    class="locale-switcher"
    :aria-label="t('lang.label')"
  >
    <el-option label="EN" value="en" />
    <el-option label="中文" value="zh-CN" />
  </el-select>
</template>

<style scoped>
.locale-switcher {
  width: 88px;
}
</style>
