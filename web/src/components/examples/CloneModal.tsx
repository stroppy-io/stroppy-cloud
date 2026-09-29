import { keys } from '@api/queries/keys'
import { exampleMutations } from '@api/queries/results'
import type { Example, TenantMembership } from '@api/types'
import { toast } from '@app/Toaster'
import { Alert, Button, Field, Input, Modal, Select, Stack, Text } from '@grafana/ui'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

const WRITABLE = new Set(['owner', 'admin', 'member'])

const LIBRARY_PATH: Record<Example['kind'], string> = {
  database: 'library/databases',
  workload: 'library/workloads',
  test: 'library/tests',
  suite: 'suites',
}

export function CloneModal({
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
  const writable = tenants.filter((m) => WRITABLE.has(m.role))
  const [slug, setSlug] = useState<string | undefined>(
    writable.find((m) => m.tenant.slug === defaultSlug)?.tenant.slug ?? writable[0]?.tenant.slug
  )
  const [name, setName] = useState(example.title)
  const clone = useMutation({
    mutationFn: () => exampleMutations.clone(slug ?? '', example.id, name.trim() || undefined),
    onSuccess: (res) => {
      void qc.invalidateQueries({ queryKey: keys.t(slug ?? '') })
      const created = res.created[0]
      const tenantName = tenants.find((m) => m.tenant.slug === slug)?.tenant.name ?? slug
      toast.success(t('examples.clone.done', { tenant: tenantName }), {
        description: created?.name,
        action: created
          ? {
              label: t('examples.clone.open'),
              onClick: () =>
                void navigate({
                  to: `/t/${slug}/${LIBRARY_PATH[example.kind]}/${created.id}` as never,
                }),
            }
          : undefined,
      })
      onClose()
    },
    onError: (e: Error) => toast.error(e),
  })
  return (
    <Modal title={t('examples.clone.title', { name: example.title })} isOpen onDismiss={onClose}>
      {writable.length === 0 ? (
        <Alert severity="warning" title={t('examples.clone.noTenant')} />
      ) : (
        <Stack direction="column" gap={2}>
          <Field label={t('examples.clone.tenant')} htmlFor="clone-tenant">
            <Select
              inputId="clone-tenant"
              options={writable.map((m) => ({
                label: m.tenant.name,
                value: m.tenant.slug,
                description: `/t/${m.tenant.slug} · ${t(`common.role.${m.role}`)}`,
              }))}
              value={slug}
              onChange={(o) => setSlug(o.value)}
            />
          </Field>
          <Field
            label={t('examples.clone.name')}
            description={t('examples.clone.nameHint')}
            htmlFor="clone-name"
          >
            <Input id="clone-name" value={name} onChange={(e) => setName(e.currentTarget.value)} />
          </Field>
          <Text color="secondary" variant="bodySmall">
            {t(`examples.kindOne.${example.kind}`)}
            {example.db_kind ? ` · ${example.db_kind}` : ''}
          </Text>
        </Stack>
      )}
      <Modal.ButtonRow>
        <Button variant="secondary" onClick={onClose} disabled={clone.isPending}>
          {t('common.actions.cancel')}
        </Button>
        <Button
          icon="copy"
          onClick={() => clone.mutate()}
          disabled={clone.isPending || !slug || writable.length === 0}
        >
          {t('examples.clone.submit')}
        </Button>
      </Modal.ButtonRow>
    </Modal>
  )
}
