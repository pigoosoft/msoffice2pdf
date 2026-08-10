import { http, dataOf } from './http'
import type { Role } from './types'

export interface Profile {
  uid: string
  role: Role
  status: number
  token: string
  upload_count: number
  convert_success_count: number
  created_at: string
  updated_at: string
}

export function getProfile() {
  return http.get('/api/profile').then((r) => dataOf<Profile>(r))
}

export function changePassword(oldPwd: string, newPwd: string) {
  return http.put('/api/profile/password', { old_pwd: oldPwd, new_pwd: newPwd }).then(() => undefined)
}

export function resetMyToken() {
  return http.post('/api/profile/reset-token').then((r) =>
    dataOf<{ uid: string; role: Role; status: number; token: string }>(r),
  )
}
