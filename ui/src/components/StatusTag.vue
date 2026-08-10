<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ status: string }>()

type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'

const typeMap: Record<string, TagType> = {
  pending: 'info',
  queued: 'info',
  converting: 'warning',
  generating: 'warning',
  completed: 'success',
  failed: 'danger',
  deleted: 'info',
  expired: 'danger',
}

const tagType = computed<TagType>(() => typeMap[props.status] || 'info')
</script>

<template>
  <el-tag :type="tagType" size="small" disable-transitions>
    {{ status || '-' }}
  </el-tag>
</template>
