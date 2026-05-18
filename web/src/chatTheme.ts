export type ChatThemeName = 'dark' | 'light' | 'one-dark-pro' | 'solarized-dark' | 'nord'
export type ChatDensityName = 'compact' | 'comfortable' | 'spacious'

export interface ChatDensity {
  fontSize: string
  fontSizeMono: string
  lineHeight: string
  blockPadding: string
  gutterWidth: string
  subLineGap: string
}

export interface ChatThemeTokens {
  name: ChatThemeName
  displayName: string
  msgBg: string
  codeBg: string
  codeBlockBorder: string
  text: string
  textSub: string
  muted: string
  labelColor: string
  primary: string
  accent: string
  green: string
  red: string
  yellow: string
  purple: string
}

export const densityPresets: Record<ChatDensityName, ChatDensity> = {
  compact: {
    fontSize: '13px',
    fontSizeMono: '12px',
    lineHeight: '1.55',
    blockPadding: '1px 0',
    gutterWidth: '20px',
    subLineGap: '3px',
  },
  comfortable: {
    fontSize: '14px',
    fontSizeMono: '13px',
    lineHeight: '1.65',
    blockPadding: '3px 0',
    gutterWidth: '22px',
    subLineGap: '5px',
  },
  spacious: {
    fontSize: '15px',
    fontSizeMono: '13.5px',
    lineHeight: '1.8',
    blockPadding: '6px 0',
    gutterWidth: '24px',
    subLineGap: '8px',
  },
}

export const chatThemes: Record<ChatThemeName, ChatThemeTokens> = {
  dark: {
    name: 'dark', displayName: 'Dark',
    msgBg: 'transparent', codeBg: '#12141f', codeBlockBorder: '#2c3150',
    text: '#eceef5', textSub: '#b0b8c8', muted: '#8892a4', labelColor: '#8892a4',
    primary: '#6366f1', accent: '#e94560',
    green: '#4ade80', red: '#f87171', yellow: '#fbbf24', purple: '#a78bfa',
  },
  light: {
    name: 'light', displayName: 'Light',
    msgBg: 'transparent', codeBg: '#f5f7ff', codeBlockBorder: '#d8dce8',
    text: '#111827', textSub: '#374151', muted: '#6b7280', labelColor: '#4b5563',
    primary: '#6366f1', accent: '#e94560',
    green: '#15803d', red: '#dc2626', yellow: '#b45309', purple: '#6d28d9',
  },
  'one-dark-pro': {
    name: 'one-dark-pro', displayName: 'One Dark Pro',
    msgBg: 'transparent', codeBg: '#282c34', codeBlockBorder: '#528bff',
    text: '#abb2bf', textSub: '#abb2bf', muted: '#5c6370', labelColor: '#5c6370',
    primary: '#61afef', accent: '#e06c75',
    green: '#98c379', red: '#e06c75', yellow: '#e5c07b', purple: '#c678dd',
  },
  'solarized-dark': {
    name: 'solarized-dark', displayName: 'Solarized Dark',
    msgBg: 'transparent', codeBg: '#073642', codeBlockBorder: '#268bd2',
    text: '#839496', textSub: '#93a1a1', muted: '#586e75', labelColor: '#657b83',
    primary: '#268bd2', accent: '#dc322f',
    green: '#859900', red: '#dc322f', yellow: '#b58900', purple: '#6c71c4',
  },
  nord: {
    name: 'nord', displayName: 'Nord',
    msgBg: 'transparent', codeBg: '#2e3440', codeBlockBorder: '#88c0d0',
    text: '#d8dee9', textSub: '#e5e9f0', muted: '#4c566a', labelColor: '#616e88',
    primary: '#88c0d0', accent: '#bf616a',
    green: '#a3be8c', red: '#bf616a', yellow: '#ebcb8b', purple: '#b48ead',
  },
}

const THEME_KEY = 'spider-chat-theme'
const DENSITY_KEY = 'spider-chat-density'

export function getSavedChatTheme(): ChatThemeName {
  return (localStorage.getItem(THEME_KEY) as ChatThemeName) || 'dark'
}

export function saveChatTheme(name: ChatThemeName) {
  localStorage.setItem(THEME_KEY, name)
}

export function getSavedChatDensity(): ChatDensityName {
  return (localStorage.getItem(DENSITY_KEY) as ChatDensityName) || 'compact'
}

export function saveChatDensity(name: ChatDensityName) {
  localStorage.setItem(DENSITY_KEY, name)
}
