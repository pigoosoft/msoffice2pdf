import { http, dataOf } from './http'
import type { Role } from './types'
import type { PageResult, UploadItem } from './upload'
import type { PdfItem } from './pdf'

export interface AdminUser {
  uid: string
  role: Role | string
  status: number
  token?: string
  created_at: string
  updated_at: string
}

export interface CreateUserPayload {
  uid: string
  pwd: string
  role?: Role | string
}

export interface UpdateUserPayload {
  pwd?: string
  role?: Role | string
}

export function listUsers(page = 1, pageSize = 20) {
  return http
    .get('/api/admin/users', {
      params: { page, page_size: pageSize },
    })
    .then((r) => dataOf<PageResult<AdminUser>>(r))
}

export function getUser(uid: string) {
  return http.get(`/api/admin/users/${encodeURIComponent(uid)}`).then((r) => dataOf<AdminUser>(r))
}

export function createUser(payload: CreateUserPayload) {
  return http.post('/api/admin/users', payload).then((r) => dataOf<AdminUser>(r))
}

export function updateUser(uid: string, payload: UpdateUserPayload) {
  return http
    .put(`/api/admin/users/${encodeURIComponent(uid)}`, payload)
    .then((r) => dataOf<AdminUser>(r))
}

export function deleteUser(uid: string) {
  return http.delete(`/api/admin/users/${encodeURIComponent(uid)}`).then((r) => dataOf(r))
}

export function freezeUser(uid: string, frozen: boolean) {
  return http
    .post(`/api/admin/users/${encodeURIComponent(uid)}/freeze`, { frozen })
    .then((r) => dataOf<AdminUser>(r))
}

export function resetUserToken(uid: string) {
  return http
    .post(`/api/admin/users/${encodeURIComponent(uid)}/reset-token`)
    .then((r) => dataOf<AdminUser>(r))
}

export function listAdminUploads(page = 1, pageSize = 20, uid?: string) {
  return http
    .get('/api/admin/uploads', {
      params: {
        page,
        page_size: pageSize,
        ...(uid ? { uid } : {}),
      },
    })
    .then((r) => dataOf<PageResult<UploadItem>>(r))
}

export function listAdminPdfs(page = 1, pageSize = 20, uid?: string) {
  return http
    .get('/api/admin/pdfs', {
      params: {
        page,
        page_size: pageSize,
        ...(uid ? { uid } : {}),
      },
    })
    .then((r) => dataOf<PageResult<PdfItem>>(r))
}

export type MetricsRange = '1h' | '24h' | '7d'

export interface MetricsLimits {
  mem_limit_bytes: number
  disk_min_free_bytes: number
  log_backlog_max_bytes: number
}

export interface MetricsSample {
  sampled_at: string
  pending: number
  queued: number
  converting: number
  failed: number
  channel_len: number
  workers_cur: number
  workers_max: number
  workers_min: number
  log_backlog_bytes: number
  heap_alloc: number
  ram_avail: number
  disk_free_min: number
  degrade_reason: string
}

export interface MetricsCurrent extends MetricsSample {
  limits: MetricsLimits
}

export interface MetricsHistory {
  range: MetricsRange | string
  bucket: string
  points: MetricsSample[]
}

export function getAdminMetrics() {
  return http.get('/api/admin/metrics').then((r) => dataOf<MetricsCurrent>(r))
}

export function getAdminMetricsHistory(range: MetricsRange) {
  return http
    .get('/api/admin/metrics/history', { params: { range } })
    .then((r) => dataOf<MetricsHistory>(r))
}
