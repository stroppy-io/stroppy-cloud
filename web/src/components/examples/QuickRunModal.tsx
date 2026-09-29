import { keys } from '@api/queries/keys'
import { exampleMutations, providerQueries } from '@api/queries/results'
import type { Example, TenantMembership } from '@api/types'
import { toast } from '@app/Toaster'
import { Alert, Button, Field, Input, Modal, Select, Stack, Text } from '@grafana/ui'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

const CAN_RUN = new Set(['owner', 'admin', 'member'])

export function QuickRunModal({
  example,
  tenants,
  defaultSlug,
  onClose,
}: {
  example: Example
  tenants: TenantMembership[]
  defaultSlug?: string
  onClose: () => void
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const runnable = tenants.filter((m) => CAN_RUN.has(m.role))
  const [slug, setSlug] = useState<string | undefined>(
    runnable.find((m) => m.tenant.slug === defaultSlug)?.tenant.slug ?? runnable[0]?.tenant.slug
  )
  const [provider, setProvider] = useState<string | undefined>()
  const [name, setName] = useState(example.title)
  const providers = useQuery({ ...providerQueries.list(slug ?? ''), enabled: !!slug })
  const ready = (providers.data?.data ?? []).filter((p) => p.status === 'ready')
  // Preselect the first ready profile whenever the tenant changes.
  useEffect(() => {
    if (ready.length && !ready.some((p) => p.id === provider)) setProvider(ready[0].id)
  }, [ready, provider])

  const launch = useMutation({
    mutationFn: () =>
      exampleMutations.quickRun(slug ?? '', example.id, {
        name: name.trim() || undefined,
        provider_profile_id: provider,
      }),
    onSuccess: (run) => {
      void qc.invalidateQueries({ queryKey: [...keys.t(slug ?? ''), 'runs'] })
      void qc.invalidateQueries({ queryKey: [...keys.t(slug ?? ''), 'dashboard'] })
      toast.success(t('examples.quickRun.launched'), { description: run.name })
      onClose()
      void navigate({ to: '/t/$slug/runs/$id', params: { slug: slug ?? '', id: run.id } })
    },
    onError: (e: Error) => toast.error(e),
  })

  return (
    <Modal title={t('examples.quickRun.title', { name: example.title })} isOpen onDismiss={onClose}>
      <Stack direction="column" gap={2}>
        <Text color="secondary">{t('examples.quickRun.hint')}</Text>
        {runnable.length === 0 ? (
          <Alert severity="warning" title={t('examples.clone.noTenant')} />
        ) : (
          <>
            <Field label={t('examples.quickRun.tenant')} htmlFor="qr-tenant">
              <Select
                inputId="qr-tenant"
                options={runnable.map((m) => ({
                  label: m.tenant.name,
                  value: m.tenant.slug,
                  description: `/t/${m.tenant.slug}`,
                }))}
                value={slug}
                onChange={(o) => {
                  setSlug(o.value)
                  setProvider(undefined)
                }}
              />
            </Field>
            <Field
              label={t('examples.quickRun.provider')}
              description={t('examples.quickRun.providerHint')}
              htmlFor="qr-provider"
              invalid={providers.isSuccess && ready.length === 0}
              error={
                providers.isSuccess && ready.length === 0
                  ? t('examples.quickRun.noProviders')
                  : undefined
              }
            >
              <Select
                inputId="qr-provider"
                isLoading={providers.isPending}
                options={ready.map((p) => ({ label: p.name, value: p.id, description: p.kind }))}
                value={provider ?? null}
                onChange={(o) => setProvider(o?.value)}
              />
            </Field>
            <Field label={t('examples.quickRun.runName')} htmlFor="qr-name">
              <Input id="qr-name" value={name} onChange={(e) => setName(e.currentTarget.value)} />
            </Field>
          </>
        )}
      </Stack>
      <Modal.ButtonRow>
        <Button variant="secondary" onClick={onClose} disabled={launch.isPending}>
          {t('common.actions.cancel')}
        </Button>
        <Button
          icon="play"
          onClick={() => launch.mutate()}
          disabled={launch.isPending || !slug || !provider}
        >
          {t('examples.quickRun.submit')}
        </Button>
      </Modal.ButtonRow>
    </Modal>
  )
}
