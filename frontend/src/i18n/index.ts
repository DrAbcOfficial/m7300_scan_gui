import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN.json'
import enUS from './locales/en-US.json'

const STORAGE_KEY = 'pantum-scan-gui.lang'

export const SUPPORTED_LOCALES = ['zh-CN', 'en-US']

export function currentLocale(): string {
  const saved = localStorage.getItem(STORAGE_KEY)
  return saved && SUPPORTED_LOCALES.includes(saved) ? saved : 'zh-CN'
}

export function setLocale(locale: string) {
  i18n.global.locale.value = locale as 'zh-CN' | 'en-US'
  localStorage.setItem(STORAGE_KEY, locale)
}

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: currentLocale(),
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})
