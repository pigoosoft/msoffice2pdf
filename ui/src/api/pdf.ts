import { http, dataOf } from './http'
import type { Envelope } from './types'
import type { PageResult } from './upload'
import type { AxiosError } from 'axios'

export interface PdfStatus {
  fileid: string
  request_id?: string
  status: string
  final_status?: string
  error_code?: string
  error_msg?: string
  retry_count?: number
  created_at?: string
  finished_at?: string
  completed_at?: string
  pdf_filename?: string
  warn_code?: string
}

export interface PdfItem {
  fileid: string
  upload_id: number
  filename: string
  file_path: string
  file_size: number
  status: string
  warn_code?: string
}

export function getStatus(fileid: string) {
  return http
    .get(`/api/pdf/${encodeURIComponent(fileid)}/status`)
    .then((r) => dataOf<PdfStatus>(r))
}

function filenameFromDisposition(header: string | undefined): string | null {
  if (!header) return null
  const utf8 = /filename\*=UTF-8''([^;]+)/i.exec(header)
  if (utf8?.[1]) {
    try {
      return decodeURIComponent(utf8[1].trim())
    } catch {
      return utf8[1].trim()
    }
  }
  const plain = /filename="?([^";]+)"?/i.exec(header)
  return plain?.[1]?.trim() || null
}

async function rejectIfJsonBlob(blob: Blob, contentType: string | undefined): Promise<void> {
  const ct = (contentType || blob.type || '').toLowerCase()
  if (!ct.includes('application/json') && !ct.includes('text/json')) {
    return
  }
  const text = await blob.text()
  let envelope: Envelope
  try {
    envelope = JSON.parse(text) as Envelope
  } catch {
    throw new Error(text || 'download failed')
  }
  throw envelope
}

function triggerSave(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export async function downloadPdf(fileid: string, fallbackName?: string) {
  try {
    const res = await http.get(`/api/pdf/${encodeURIComponent(fileid)}/download`, {
      responseType: 'blob',
    })
    const blob = res.data as Blob
    await rejectIfJsonBlob(blob, res.headers['content-type'] as string | undefined)
    const name =
      filenameFromDisposition(res.headers['content-disposition'] as string | undefined) ||
      fallbackName ||
      `${fileid}.pdf`
    triggerSave(blob, name)
  } catch (e) {
    const ax = e as AxiosError<Blob>
    if (ax.response?.data instanceof Blob) {
      await rejectIfJsonBlob(ax.response.data, ax.response.headers['content-type'] as string | undefined)
    }
    throw e
  }
}

export function listPdfs(page = 1, pageSize = 20, status?: string) {
  return http
    .get('/api/pdfs', {
      params: {
        page,
        page_size: pageSize,
        ...(status ? { status } : {}),
      },
    })
    .then((r) => dataOf<PageResult<PdfItem>>(r))
}
