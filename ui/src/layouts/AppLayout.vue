<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Expand, Fold, Menu } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import LocaleSwitcher from '@/components/LocaleSwitcher.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const app = useAppStore()

const windowWidth = ref(window.innerWidth)

const isPhone = computed(() => windowWidth.value < 768)
const isPad = computed(() => windowWidth.value >= 768 && windowWidth.value < 992)
const isPC = computed(() => windowWidth.value >= 992)

const menuCollapsed = computed(() => isPad.value && app.sidebarCollapsed)
const asideWidth = computed(() => (menuCollapsed.value ? '64px' : '220px'))

function onResize() {
  windowWidth.value = window.innerWidth
  if (!isPhone.value) {
    app.drawerOpen = false
  }
  if (isPC.value) {
    app.sidebarCollapsed = false
  }
}

onMounted(() => window.addEventListener('resize', onResize))
onUnmounted(() => window.removeEventListener('resize', onResize))

function onMenuSelect() {
  if (isPhone.value) {
    app.drawerOpen = false
  }
}

function toggleSidebar() {
  app.sidebarCollapsed = !app.sidebarCollapsed
}

async function onLogout() {
  await auth.logout()
  await router.push('/login')
}
</script>

<template>
  <el-container class="app-layout">
    <el-header class="app-header">
      <div class="header-left">
        <el-button
          v-if="isPhone"
          :icon="Menu"
          text
          :aria-label="t('nav.openMenu')"
          @click="app.drawerOpen = true"
        />
        <el-button
          v-else-if="isPad"
          :icon="menuCollapsed ? Expand : Fold"
          text
          :aria-label="t('nav.collapseSidebar')"
          @click="toggleSidebar"
        />
        <span class="logo">MSOffice2Pdf</span>
      </div>
      <div class="header-right">
        <LocaleSwitcher />
        <span class="uid">{{ auth.uid }}</span>
        <el-button text @click="onLogout">{{ t('nav.logout') }}</el-button>
      </div>
    </el-header>

    <el-container class="app-body">
      <el-aside v-if="!isPhone" :width="asideWidth" class="app-aside">
        <el-menu
          :default-active="route.path"
          router
          :collapse="menuCollapsed"
          @select="onMenuSelect"
        >
          <el-menu-item index="/board">{{ t('nav.board') }}</el-menu-item>
          <el-menu-item index="/upload">{{ t('nav.upload') }}</el-menu-item>
          <el-menu-item index="/pdfs">{{ t('nav.pdfs') }}</el-menu-item>
          <el-menu-item index="/history">{{ t('nav.history') }}</el-menu-item>
          <el-sub-menu v-if="auth.isAdmin" index="admin">
            <template #title>{{ t('nav.admin') }}</template>
            <el-menu-item index="/admin/users">{{ t('nav.users') }}</el-menu-item>
            <el-menu-item index="/admin/overview">{{ t('nav.overview') }}</el-menu-item>
          </el-sub-menu>
        </el-menu>
      </el-aside>

      <el-main class="app-main">
        <router-view />
      </el-main>
    </el-container>

    <el-drawer
      v-if="isPhone"
      v-model="app.drawerOpen"
      direction="ltr"
      size="220px"
      :with-header="false"
      class="app-drawer"
    >
      <el-menu
        :default-active="route.path"
        router
        @select="onMenuSelect"
      >
        <el-menu-item index="/board">{{ t('nav.board') }}</el-menu-item>
        <el-menu-item index="/upload">{{ t('nav.upload') }}</el-menu-item>
        <el-menu-item index="/pdfs">{{ t('nav.pdfs') }}</el-menu-item>
        <el-menu-item index="/history">{{ t('nav.history') }}</el-menu-item>
        <el-sub-menu v-if="auth.isAdmin" index="admin">
          <template #title>{{ t('nav.admin') }}</template>
          <el-menu-item index="/admin/users">{{ t('nav.users') }}</el-menu-item>
          <el-menu-item index="/admin/overview">{{ t('nav.overview') }}</el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-drawer>
  </el-container>
</template>

<style scoped>
.app-layout {
  height: 100%;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--el-border-color-light);
  padding: 0 16px;
}

.header-left,
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.logo {
  font-weight: 600;
  font-size: 1.1rem;
}

.uid {
  color: var(--el-text-color-regular);
}

.app-body {
  min-height: 0;
  flex: 1;
}

.app-aside {
  border-right: 1px solid var(--el-border-color-light);
  overflow: hidden;
  transition: width 0.2s;
}

.app-aside :deep(.el-menu) {
  border-right: none;
  height: 100%;
}

.app-main {
  padding: 16px;
  background: var(--el-bg-color-page);
  overflow: auto;
}

.app-drawer :deep(.el-menu) {
  border-right: none;
}
</style>
