import { suiteRunMutations } from '@api/queries/suites'
import type { Share, SuiteRun } from '@api/types'
import { toast } from '@app/Toaster'
import { CopyText } from '@components/CopyText'
import { Alert, Button, Field, Input, Modal, RadioButtonGroup, Stack } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

export function SuiteRunShareDialog({
  suiteRun,
  onClose,
}: {
  suiteRun: SuiteRun
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const [ttl, setTtl] = useState('7d')
  const [scope, setScope] = useState<Share['scope']>('metrics')
  const [title, setTitle] = useState(suiteRun.name)
  const [share, setShare] = useState<Share | undefined>()
  const m = useMutation({
    mutationFn: () =>
      suiteRunMutations.share(slug, suiteRun.id, {
        ttl: ttl.trim() || undefined,
        scope,
        title: title.trim() || undefined,
      }),
    onSuccess: (s) => {
      setShare(s)
      toast.success(t('suites.suiteRuns.toasts.shared'))
    },
    onError: (e) => toast.error(e),
  })
  const ttlBad = ttl.trim() !== '' && !/^\d+(d|h|m|s)$/.test(ttl.trim())
  const url = share ? `${window.location.origin}${share.url}` : ''
  return (
    <Modal title={t('suites.suiteRuns.share.title')} isOpen onDismiss={onClose}>
      <Stack direction="column" gap={1}>
        <Field label={t('suites.suiteRuns.share.linkTitle')} htmlFor="share-title">
          <Input id="share-title" value={title} onChange={(e) => setTitle(e.currentTarget.value)} />
        </Field>
        <Field
          label={t('suites.suiteRuns.share.ttl')}
          description={t('suites.suiteRuns.share.ttlHint')}
          htmlFor="share-ttl"
          invalid={ttlBad}
          error={ttlBad ? t('common.validation.duration') : undefined}
        >
          <Input
            id="share-ttl"
            width={12}
            value={ttl}
            onChange={(e) => setTtl(e.currentTarget.value)}
          />
        </Field>
        <Field label={t('suites.suiteRuns.share.scope')}>
          <RadioButtonGroup<Share['scope']>
            options={[
              { label: t('suites.suiteRuns.share.scopes.overview'), value: 'overview' },
              { label: t('suites.suiteRuns.share.scopes.metrics'), value: 'metrics' },
              { label: t('suites.suiteRuns.share.scopes.configs'), value: 'configs' },
            ]}
            value={scope}
            onChange={setScope}
          />
        </Field>
        {share && (
          <Alert severity="success" title={t('suites.suiteRuns.share.ready')}>
            <CopyText value={url} />
          </Alert>
        )}
      </Stack>
      <Modal.ButtonRow>
        <Button variant="secondary" fill="outline" onClick={onClose}>
          {t('common.actions.close')}
        </Button>
        {!share && (
          <Button icon="share-alt" disabled={m.isPending || ttlBad} onClick={() => m.mutate()}>
            {t('suites.suiteRuns.share.submit')}
          </Button>
        )}
      </Modal.ButtonRow>
    </Modal>
  )
}
