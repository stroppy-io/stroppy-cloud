import { isApiError } from '@api/errors'
import { keys } from '@api/queries/keys'
import { type ExportDocument, type LibraryKind, libraryByKind } from '@api/queries/library'
import type { Entity } from '@api/types'
import { toast } from '@app/Toaster'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Button,
  Field,
  FileDropzone,
  FileDropzoneDefaultChildren,
  Modal,
  Stack,
  Text,
  TextArea,
  useStyles2,
} from '@grafana/ui'
import { parseDocument } from '@helpers/yaml'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

const KIND_DOC: Record<LibraryKind, ExportDocument['kind']> = {
  database: 'Database',
  workload: 'Workload',
  test: 'Test',
}

const getStyles = (theme: GrafanaTheme2) => ({
  area: css({ fontFamily: theme.typography.fontFamilyMonospace, minHeight: 220 }),
  drop: css({ marginBottom: theme.spacing(2) }),
})

const EXAMPLE: Record<LibraryKind, string> = {
  database: `api_version: stroppy.io/v1
kind: Database
metadata:
  name: pg-17-single
spec:
  kind: postgres
  version: "17"
  params:
    replicas: 0
    ha: none`,
  workload: `api_version: stroppy.io/v1
kind: Workload
metadata:
  name: tpcb-quick
spec:
  stroppy_version: 6.1.0
  protocol: pg
  segments:
    - name: tpcb
      workload: { script: tpcb/tx, scale_factor: 10 }
      run: { executor: constant-vus, vus: 16, duration: 2m }`,
  test: `api_version: stroppy.io/v1
kind: Test
metadata:
  name: pg smoke
spec:
  database: { ref: { id: db-pg-single } }
  workload: { ref: { id: wl-tpcb-quick } }
  sizes:
    db: { size: S }
    runner: { size: XS }`,
}

// Paste YAML/JSON or drop a file → `:import` → navigate to the new record. Shared by all kinds.
export function ImportModal({
  kind,
  isOpen,
  onClose,
}: {
  kind: LibraryKind
  isOpen: boolean
  onClose: () => void
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [text, setText] = useState('')
  const [parseError, setParseError] = useState<string | undefined>()
  const m = useMutation({
    mutationFn: (doc: ExportDocument) => libraryByKind[kind].import(slug, doc) as Promise<Entity>,
    onSuccess: (created) => {
      void qc.invalidateQueries({ queryKey: keys.t(slug) })
      toast.success(t('library.import.done', { name: created.name }))
      onClose()
      setText('')
      const to =
        kind === 'database'
          ? '/t/$slug/library/databases/$id'
          : kind === 'workload'
            ? '/t/$slug/library/workloads/$id'
            : '/t/$slug/library/tests/$id'
      void navigate({ to, params: { slug, id: created.id } })
    },
  })
  const submit = () => {
    setParseError(undefined)
    let doc: unknown
    try {
      doc = parseDocument(text)
    } catch (e) {
      setParseError(
        t('library.import.parseError', { detail: e instanceof Error ? e.message : String(e) })
      )
      return
    }
    if (!doc || typeof doc !== 'object') {
      setParseError(t('library.import.notDocument'))
      return
    }
    const d = doc as Partial<ExportDocument>
    m.mutate({
      api_version: d.api_version ?? 'stroppy.io/v1',
      kind: d.kind ?? KIND_DOC[kind],
      metadata: d.metadata,
      spec: (d.spec ?? {}) as Record<string, unknown>,
    })
  }
  const serverIssues = isApiError(m.error) ? (m.error.validation?.errors ?? []) : []
  return (
    <Modal
      isOpen={isOpen}
      title={t('library.import.title', { kind: t(`library.kind.${kind}`) })}
      onDismiss={onClose}
    >
      <Stack direction="column" gap={1}>
        <Text color="secondary">{t('library.import.hint', { kind: KIND_DOC[kind] })}</Text>
        <div className={styles.drop}>
          <FileDropzone
            options={{
              multiple: false,
              accept: { 'application/json': ['.json'], 'application/yaml': ['.yaml', '.yml'] },
            }}
            readAs="readAsText"
            onLoad={(result) => {
              if (typeof result === 'string') setText(result)
            }}
            fileListRenderer={() => null}
          >
            <FileDropzoneDefaultChildren
              primaryText={t('library.import.drop')}
              secondaryText={t('library.import.dropHint')}
            />
          </FileDropzone>
        </div>
        <Field
          label={t('library.import.paste')}
          invalid={!!parseError}
          error={parseError}
          htmlFor="import-text"
        >
          <TextArea
            id="import-text"
            className={styles.area}
            rows={10}
            value={text}
            placeholder={EXAMPLE[kind]}
            onChange={(e) => setText(e.currentTarget.value)}
          />
        </Field>
        {m.isError && (
          <Alert
            severity="error"
            title={isApiError(m.error) ? m.error.title : t('common.errors.generic')}
          >
            {isApiError(m.error) && m.error.detail}
            {serverIssues.length > 0 && (
              <ul>
                {serverIssues.map((i) => (
                  <li key={`${i.path}-${i.code}`}>
                    <code>{i.path || 'document'}</code>: {i.message ?? i.code}
                  </li>
                ))}
              </ul>
            )}
          </Alert>
        )}
      </Stack>
      <Modal.ButtonRow
        leftItems={
          <Button variant="secondary" fill="text" size="sm" onClick={() => setText(EXAMPLE[kind])}>
            {t('library.import.useExample')}
          </Button>
        }
      >
        <Button variant="secondary" onClick={onClose}>
          {t('common.actions.cancel')}
        </Button>
        <Button icon="import" onClick={submit} disabled={m.isPending || !text.trim()}>
          {t('common.actions.import')}
        </Button>
      </Modal.ButtonRow>
    </Modal>
  )
}
