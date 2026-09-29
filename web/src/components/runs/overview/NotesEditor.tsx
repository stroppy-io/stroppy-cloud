import { css } from '@emotion/css'
import { type GrafanaTheme2, renderMarkdown } from '@grafana/data'
import { Button, IconButton, Stack, Text, TextArea, useStyles2 } from '@grafana/ui'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  md: css({
    fontSize: theme.typography.bodySmall.fontSize,
    'h1,h2,h3,h4': { fontSize: theme.typography.body.fontSize, margin: theme.spacing(1, 0, 0.5) },
    'p, ul, ol': { margin: theme.spacing(0, 0, 1) },
    code: {
      fontFamily: theme.typography.fontFamilyMonospace,
      background: theme.colors.background.secondary,
      padding: theme.spacing(0, 0.5),
      borderRadius: theme.shape.radius.default,
    },
    a: { color: theme.colors.text.link },
  }),
  head: css({ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }),
})

// Markdown notes: rendered view, inline textarea editor, saved with PATCH.
export function NotesEditor({
  value,
  canEdit,
  onSave,
}: {
  value: string | undefined
  canEdit: boolean
  onSave: (notes: string) => Promise<unknown>
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(value ?? '')
  const [busy, setBusy] = useState(false)
  const start = () => {
    setDraft(value ?? '')
    setEditing(true)
  }
  const save = async () => {
    setBusy(true)
    try {
      await onSave(draft)
      setEditing(false)
    } finally {
      setBusy(false)
    }
  }
  if (editing)
    return (
      <Stack direction="column" gap={1}>
        <TextArea
          rows={6}
          value={draft}
          autoFocus
          placeholder={t('runs.overview.notes.placeholder')}
          onChange={(e) => setDraft(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setEditing(false)
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') void save()
          }}
        />
        <Stack gap={1} justifyContent="flex-end">
          <Button size="sm" variant="secondary" fill="outline" onClick={() => setEditing(false)}>
            {t('common.actions.cancel')}
          </Button>
          <Button size="sm" onClick={() => void save()} disabled={busy}>
            {t('common.actions.save')}
          </Button>
        </Stack>
      </Stack>
    )
  return (
    <Stack direction="column" gap={0.5}>
      <div className={styles.head}>
        {canEdit && (
          <IconButton name="edit" size="sm" tooltip={t('common.actions.edit')} onClick={start} />
        )}
      </div>
      {value ? (
        // biome-ignore lint/security/noDangerouslySetInnerHtml: renderMarkdown sanitizes output
        <div className={styles.md} dangerouslySetInnerHTML={{ __html: renderMarkdown(value) }} />
      ) : (
        <Text color="secondary" variant="bodySmall">
          {canEdit ? t('runs.overview.notes.emptyEditable') : t('runs.overview.notes.empty')}
        </Text>
      )}
    </Stack>
  )
}
