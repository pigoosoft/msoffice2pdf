import axios, { type AxiosError } from 'axios'
import type { Envelope } from './types'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'

export const http = axios.create({
  baseURL: '/',
  timeout: 120_000,
  withCredentials: true,
})

http.interceptors.response.use(
  (res) => {
    const body = res.data as Envelope
    if (body && typeof body.code === 'number' && body.code !== 0) {
      return Promise.reject(body)
    }
    return res
  },
  (err: AxiosError<Envelope>) => {
    const status = err.response?.status
    const msg = err.response?.data?.message || err.message
    if (status === 401) {
      const auth = useAuthStore()
      auth.clear()
      if (router.currentRoute.value.path !== '/login') {
        router.push({ path: '/login', query: { redirect: router.currentRoute.value.fullPath } })
      }
    } else if (status === 403) {
      ElMessage.error(msg || 'forbidden')
    }
    return Promise.reject(err)
  },
)

export function dataOf<T>(res: { data: Envelope<T> }): T {
  return res.data.data as T
}
