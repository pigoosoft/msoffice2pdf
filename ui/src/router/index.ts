import { createRouter, createWebHistory } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { i18n } from '@/i18n'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    {
      path: '/',
      component: () => import('@/layouts/AppLayout.vue'),
      redirect: '/board',
      children: [
        { path: 'board', component: () => import('@/views/BoardView.vue') },
        { path: 'upload', component: () => import('@/views/UploadView.vue') },
        { path: 'pdfs', component: () => import('@/views/PdfsView.vue') },
        { path: 'history', component: () => import('@/views/HistoryView.vue') },
        { path: 'profile', component: () => import('@/views/ProfileView.vue') },
        {
          path: 'admin/users',
          component: () => import('@/views/admin/UsersView.vue'),
          meta: { admin: true },
        },
        {
          path: 'admin/overview',
          component: () => import('@/views/admin/OverviewView.vue'),
          meta: { admin: true },
        },
        {
          path: 'admin/perf',
          component: () => import('@/views/admin/PerfView.vue'),
          meta: { admin: true },
        },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) await auth.bootstrap()
  if (to.meta.public) {
    if (auth.isLoggedIn && to.path === '/login') return '/board'
    return true
  }
  if (!auth.isLoggedIn) return { path: '/login', query: { redirect: to.fullPath } }
  if (to.meta.admin && !auth.isAdmin) {
    ElMessage.warning(i18n.global.t('router.adminRequired'))
    return '/board'
  }
  return true
})

export default router
