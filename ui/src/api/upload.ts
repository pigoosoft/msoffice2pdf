import { http, dataOf } from './http'
import type { AxiosProgressEvent } from 'axios'

export interface UploadItem {
  fileid: string
  request_id?: string
  filename: string
  status: string
  final_status?: string
  error_code?: string
  error_msg?: string
  retry_count?: number
  file_size: number
  file_path?: string
  archive_dir?: string
  upload_time: string
  finished_at?: string | null
}

export interface PageResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface UploadResult {
  fileid: string
  request_id?: string
  filename: string
  status: string
  upload_time: string
}

export function uploadFile(
  file: File,
  watermark?: string,
  password?: string,
  onProgress?: (percent: number) => void,
) {
  const form = new FormData()
  form.append('file', file)
  const wm = watermark?.trim()
  if (wm) {
    form.append('watermark', wm)
  }
  const pw = password?.trim()
  if (pw) {
    form.append('password', pw)
  }
  return http
    .post('/api/upload', form, {
      onUploadProgress: (e: AxiosProgressEvent) => {
        if (!onProgress || !e.total) return
        onProgress(Math.min(100, Math.round((e.loaded / e.total) * 100)))
      },
    })
    .then((r) => dataOf<UploadResult>(r))
}

export function listUploads(page = 1, pageSize = 20, status?: string) {
  return http
    .get('/api/uploads', {
      params: {
        page,
        page_size: pageSize,
        ...(status ? { status } : {}),
      },
    })
    .then((r) => dataOf<PageResult<UploadItem>>(r))
}

export function deleteUpload(fileid: string) {
  return http.delete(`/api/upload/${encodeURIComponent(fileid)}`).then((r) => dataOf<{ ok: boolean }>(r))
}

export interface UploadLimits {
  allowed_exts: string[]
  max_size: number
}

export function fetchUploadLimits() {
  return http.get('/api/upload/limits').then((r) => dataOf<UploadLimits>(r))
}
