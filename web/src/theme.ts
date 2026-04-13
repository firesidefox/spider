export type Theme = 'dark' | 'light'

export interface ThemeTokens {
  bg: string
  nav: string
  navBorder: string
  surface: string
  cardBg: string
  panel: string
  border: string
  borderFocus: string
  primary: string
  primaryHover: string
  accent: string
  text: string
  textSub: string
  muted: string
  label: string
  green: string
  red: string
  yellow: string
  purple: string
  inputBg: string
  rowAlt: string
  rowHover: string
  cardShadow: string
}

const dark: ThemeTokens = {
  bg: '#0d0f1a',
  nav: '#080a12',
  navBorder: '#1e2338',
  surface: 'rgba(30,33,50,0.92)',
  cardBg: 'rgba(30,33,50,0.92)',
  panel: '#12141f',
  border: '#2c3150',
  borderFocus: '#6366f1',
  primary: '#6366f1',
  primaryHover: '#4f52d4',
  accent: '#e94560',
  text: '#eceef5',
  textSub: '#b0b8c8',
  muted: '#b0b8c8',
  label: '#8892a4',
  green: '#4ade80',
  red: '#f87171',
  yellow: '#fbbf24',
  purple: '#a78bfa',
  inputBg: 'rgba(15,17,28,0.6)',
  rowAlt: 'rgba(255,255,255,0.018)',
  rowHover: 'rgba(99,102,241,0.07)',
  cardShadow: '0 1px 3px rgba(0,0,0,0.5), 0 4px 20px rgba(0,0,0,0.3)',
}

const light: ThemeTokens = {
  bg: '#f0f2f8',
  nav: '#ffffff',
  navBorder: '#e2e4ed',
  surface: '#ffffff',
  cardBg: '#ffffff',
  panel: '#ffffff',
  border: '#d8dce8',
  borderFocus: '#6366f1',
  primary: '#6366f1',
  primaryHover: '#4f52d4',
  accent: '#e94560',
  text: '#111827',
  textSub: '#374151',
  muted: '#4b5563',
  label: '#4b5563',
  green: '#15803d',
  red: '#dc2626',
  yellow: '#b45309',
  purple: '#6d28d9',
  inputBg: '#ffffff',
  rowAlt: 'rgba(0,0,0,0.018)',
  rowHover: 'rgba(99,102,241,0.05)',
  cardShadow: '0 1px 3px rgba(0,0,0,0.06), 0 4px 16px rgba(0,0,0,0.07)',
}

export const themes = { dark, light }

const STORAGE_KEY = 'spider-theme'

export function getSavedTheme(): Theme {
  return (localStorage.getItem(STORAGE_KEY) as Theme) || 'dark'
}

export function saveTheme(t: Theme) {
  localStorage.setItem(STORAGE_KEY, t)
}
