import { http, dataOf } from './http'
import type { PageResult } from './upload'

export interface UploadHistoryItem {
  fileid: string
  request_id?: string
  filename: string
  final_status: string
  status: string
  error_code: string
  error_msg: string
  retry_count: number
  file_size: number
  archive_dir?: string
  upload_time: string
  finished_at: string
}

export interface PdfLogItem {
  id: number
  pdf_id: number
  fileid: string
  action: string
  detail: string
  ip_address: string
  user_agent: string
  created_at: string
  uid: string
}

export function listUploadHistory(page = 1, pageSize = 20, finalStatus?: string) {
  return http
    .get('/api/history/uploads', {
      params: {
        page,
        page_size: pageSize,
        ...(finalStatus ? { final_status: finalStatus } : {}),
      },
    })
    .then((r) => dataOf<PageResult<UploadHistoryItem>>(r))
}

export function listPdfLogs(page = 1, pageSize = 20, fileid?: string) {
  return http
    .get('/api/history/pdflogs', {
      params: {
        page,
        page_size: pageSize,
        ...(fileid ? { fileid } : {}),
      },
    })
    .then((r) => dataOf<PageResult<PdfLogItem>>(r))
}

export function listAdminPdfLogs(page = 1, pageSize = 20, fileid?: string, uid?: string) {
  return http
    .get('/api/admin/history/pdflogs', {
      params: {
        page,
        page_size: pageSize,
        ...(fileid ? { fileid } : {}),
        ...(uid ? { uid } : {}),
      },
    })
    .then((r) => dataOf<PageResult<PdfLogItem>>(r))
}
