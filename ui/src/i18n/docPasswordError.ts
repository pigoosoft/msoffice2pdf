import { i18n } from './index'

export const ERR_DOC_PASSWORD_REQUIRED = 'ERR_DOC_PASSWORD_REQUIRED'
export const ERR_DOC_PASSWORD_WRONG = 'ERR_DOC_PASSWORD_WRONG'

function mapSentinel(value?: string): string | undefined {
  const v = value?.trim()
  if (v === ERR_DOC_PASSWORD_REQUIRED) {
    return String(i18n.global.t('history.errDocPasswordRequired'))
  }
  if (v === ERR_DOC_PASSWORD_WRONG) {
    return String(i18n.global.t('history.errDocPasswordWrong'))
  }
  return undefined
}

/** Map password error sentinels to i18n. Prefer code, then fallback; else raw text or '-'. */
export function formatDocPasswordError(code?: string, fallback?: string): string {
  return mapSentinel(code) || mapSentinel(fallback) || fallback?.trim() || code?.trim() || '-'
}
