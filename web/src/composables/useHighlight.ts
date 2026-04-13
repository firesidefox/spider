import { shallowRef, ref } from 'vue'
import { createHighlighter, type Highlighter, type BundledLanguage } from 'shiki'

const LANGS: BundledLanguage[] = [
  'bash', 'python', 'javascript', 'typescript', 'go', 'rust',
  'json', 'yaml', 'sql', 'dockerfile', 'shellscript', 'ini',
  'toml', 'xml', 'html', 'css', 'java', 'c', 'cpp', 'ruby',
]

// 简单启发式语言检测
function guessLang(code: string): BundledLanguage {
  const s = code.slice(0, 2000)
  if (/^\s*\{[\s\S]*\}\s*$/.test(s) || /^\s*\[[\s\S]*\]\s*$/.test(s)) return 'json'
  if (/^---\n|^\w[\w-]*:\s+\S/m.test(s)) return 'yaml'
  if (/^FROM\s+\w|^RUN\s+|^CMD\s+|^EXPOSE\s+/m.test(s)) return 'dockerfile'
  if (/^(SELECT|INSERT|UPDATE|DELETE|CREATE|DROP|ALTER)\s/im.test(s)) return 'sql'
  if (/^(import|from)\s+\w.*\n|def\s+\w+\s*\(|:\s*$|^\s{4}/m.test(s)) return 'python'
  if (/^package\s+\w+|func\s+\w+\s*\(|:=\s*/m.test(s)) return 'go'
  if (/^(use|fn\s+\w+|let\s+mut|impl\s+)/m.test(s)) return 'rust'
  if (/^(import|export)\s+(default\s+)?|=>\s*\{|const\s+\w+\s*=/m.test(s)) return 'typescript'
  if (/^(#!\/.*sh|set\s+-[eux]|echo\s+|if\s+\[|for\s+\w+\s+in\s+)/m.test(s)) return 'bash'
  if (/^\$\s+\S|^>\s+\S/.test(s)) return 'shellscript'
  return 'bash'
}

let _highlighter: Highlighter | null = null
let _loading: Promise<Highlighter> | null = null

async function getHighlighter(): Promise<Highlighter> {
  if (_highlighter) return _highlighter
  if (_loading) return _loading
  _loading = createHighlighter({
    themes: ['github-dark', 'github-light'],
    langs: LANGS,
  }).then(h => { _highlighter = h; return h })
  return _loading
}

export function useHighlight() {
  const highlighter = shallowRef<Highlighter | null>(null)
  const ready = ref(false)

  getHighlighter().then(h => {
    highlighter.value = h
    ready.value = true
  })

  function highlight(code: string, isDark: boolean): string {
    if (!highlighter.value || !code.trim()) return ''
    const lang = guessLang(code)
    try {
      return highlighter.value.codeToHtml(code, {
        lang,
        theme: isDark ? 'github-dark' : 'github-light',
      })
    } catch {
      return ''
    }
  }

  return { ready, highlight }
}
