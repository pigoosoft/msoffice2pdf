<script setup lang="ts">
import { Operation } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'

const props = defineProps<{
  open: boolean
  label?: string
}>()

defineEmits<{
  toggle: []
}>()

const { t } = useI18n()
const resolvedLabel = computed(() => props.label || t('common.actions'))
</script>

<template>
  <button
    type="button"
    class="actions-col-toggle"
    :class="{ 'is-open': open }"
    :title="resolvedLabel"
    :aria-label="resolvedLabel"
    @click.stop="$emit('toggle')"
  >
    <el-icon class="actions-col-toggle__icon" :size="16">
      <Operation />
    </el-icon>
  </button>
</template>

<style scoped>
.actions-col-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
  user-select: none;
}

.actions-col-toggle:hover,
.actions-col-toggle.is-open {
  color: var(--el-color-primary);
}

.actions-col-toggle__icon {
  transition: color 0.15s ease;
}
</style>
