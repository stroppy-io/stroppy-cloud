import { isApiError } from '@api/errors'
import { suiteMutations } from '@api/queries/suites'
import type { Schemas } from '@api/types'
import { toast } from '@app/Toaster'
import { Alert, Button, Field, FileDropzone, Modal, Stack, TextArea } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

// Paste-or-drop an ExportDocument (kind: Suite) and create a suite from it.
export function SuiteImportDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [text, setText] = useState('')
  const [error, setError] = useState<string | undefined>()
  const m = useMutation({
    mutationFn: (doc: Schemas['ExportDocument']) => suiteMutations.import(slug, doc),
    onSuccess: (suite) => {
      toast.success(t('suites.toasts.imported'), { description: suite.name })
      void qc.invalidateQueries({ queryKey: ['t', slug, 'suites'] })
      onClose()
      void navigate({ to: '/t/$slug/suites/$id', params: { slug, id: suite.id } })
    },
    onError: (e) =>
      setError(
        isApiError(e)
          ? (e.validation?.errors?.map((x) => x.message).join('; ') ?? e.detail ?? e.title)
          : String(e)
      ),
  })
  const submit = () => {
    setError(undefined)
    let doc: Schemas['ExportDocument']
    try {
      doc = JSON.parse(text)
    } catch {
      setError(t('suites.import.invalidJson'))
      return
    }
    m.mutate(doc)
  }
  return (
    <Modal title={t('suites.import.title')} isOpen onDismiss={onClose}>
      <Stack direction="column" gap={1}>
        <FileDropzone
          options={{ accept: { 'application/json': ['.json'] }, multiple: false }}
          readAs="readAsText"
          onLoad={(result) => setText(typeof result === 'string' ? result : '')}
        />
        <Field label={t('suites.import.hint')} htmlFor="import-text">
          <TextArea
            id="import-text"
            rows={10}
            value={text}
            placeholder={t('suites.import.placeholder')}
            onChange={(e) => setText(e.currentTarget.value)}
            style={{ fontFamily: 'monospace' }}
          />
        </Field>
        {error && <Alert severity="error" title={error} />}
      </Stack>
      <Modal.ButtonRow>
        <Button variant="secondary" fill="outline" onClick={onClose}>
          {t('common.actions.cancel')}
        </Button>
        <Button icon="import" disabled={!text.trim() || m.isPending} onClick={submit}>
          {t('suites.import.submit')}
        </Button>
      </Modal.ButtonRow>
    </Modal>
  )
}
