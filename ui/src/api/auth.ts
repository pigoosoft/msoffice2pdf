import { http, dataOf } from './http'
import type { Role } from './types'

export function login(uid: string, pwd: string) {
  return http.post('/api/auth/login', { uid, pwd }).then((r) =>
    dataOf<{ uid: string; token: string; role: Role }>(r),
  )
}

export function logout() {
  return http.post('/api/auth/logout').then(() => undefined)
}

export function verify() {
  return http.get('/api/auth/verify').then((r) =>
    dataOf<{ uid: string; role: Role; status: number }>(r),
  )
}
