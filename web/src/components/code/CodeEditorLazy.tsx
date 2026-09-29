import { css } from '@emotion/css'
import { LoadingPlaceholder } from '@grafana/ui'
import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

const CodeEditor = lazy(async () => {
  const [{ setupMonaco }, ui] = await Promise.all([import('./monaco-setup'), import('@grafana/ui')])
  setupMonaco()
  return { default: ui.CodeEditor }
})

export type CodeLanguage = 'json' | 'yaml' | 'logsql' | 'promql' | 'markdown' | 'plaintext'

const fillBox = css({ flex: '1 1 auto', minHeight: 0, height: '100%' })

// Monaco is heavy; load it (and its workers) only when a code view is actually rendered.
// `height="fill"` sizes the editor to its parent (a flex/grid cell with a definite height):
// Grafana's CodeEditor wraps Monaco in an auto-height box, so `100%` would collapse to zero.
export function CodeEditorLazy(props: {
  value: string
  language: CodeLanguage
  height?: number | string | 'fill'
  readOnly?: boolean
  onChange?: (v: string) => void
  onBlur?: (v: string) => void
}) {
  if (props.height === 'fill') return <FilledEditor {...props} />
  return <Editor {...props} height={props.height ?? 420} />
}

function FilledEditor(props: Parameters<typeof Editor>[0]) {
  const ref = useRef<HTMLDivElement>(null)
  const [h, setH] = useState(0)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(([e]) => setH(Math.floor(e.contentRect.height)))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  return (
    <div ref={ref} className={fillBox}>
      {h > 0 && <Editor {...props} height={h} />}
    </div>
  )
}

function Editor({
  value,
  language,
  height = 420,
  readOnly = true,
  onChange,
  onBlur,
}: {
  value: string
  language: CodeLanguage
  height?: number | string
  readOnly?: boolean
  onChange?: (v: string) => void
  onBlur?: (v: string) => void
}) {
  const { t } = useTranslation()
  return (
    <Suspense fallback={<LoadingPlaceholder text={t('common.misc.loading')} />}>
      <CodeEditor
        value={value}
        language={language === 'logsql' || language === 'promql' ? 'plaintext' : language}
        height={height}
        readOnly={readOnly}
        showMiniMap={false}
        showLineNumbers
        wordWrap
        onChange={onChange}
        onBlur={onBlur}
        monacoOptions={{ scrollBeyondLastLine: false, fontSize: 12 }}
      />
    </Suspense>
  )
}
