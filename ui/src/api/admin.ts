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
