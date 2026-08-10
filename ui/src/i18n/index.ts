import { createI18n } from 'vue-i18n'
import en from './locales/en'
import zhCN from './locales/zh-CN'

export type AppLocale = 'en' | 'zh-CN'

export const LOCALE_STORAGE_KEY = 'msoffice2pdf.locale'

const messages = {
  en,
  'zh-CN': zhCN,
} as const

/** Map browser / stored value to a supported app locale; unknown → en. */
export function normalizeLocale(raw: string | null | undefined): AppLocale {
  if (!raw) return 'en'
  const v = raw.trim().toLowerCase().replace('_', '-')
  if (v === 'en' || v.startsWith('en-')) return 'en'
  if (v === 'zh' || v.startsWith('zh')) return 'zh-CN'
  return 'en'
}

/** Preference: localStorage → navigator.language → en. */
export function resolveInitialLocale(): AppLocale {
  try {
    const saved = localStorage.getItem(LOCALE_STORAGE_KEY)
    if (saved === 'en' || saved === 'zh-CN') return saved
    if (saved) return normalizeLocale(saved)
  } catch {
    // ignore storage errors (private mode, etc.)
  }
  if (typeof navigator !== 'undefined') {
    return normalizeLocale(navigator.language || (navigator as { userLanguage?: string }).userLanguage)
  }
  return 'en'
}

export function applyDocumentLang(locale: AppLocale) {
  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale === 'zh-CN' ? 'zh-CN' : 'en'
  }
}

const initial = resolveInitialLocale()
applyDocumentLang(initial)

export const i18n = createI18n({
  legacy: false,
  locale: initial,
  fallbackLocale: 'en',
  messages,
})

export function setAppLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  applyDocumentLang(locale)
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale)
  } catch {
    // ignore
  }
}
