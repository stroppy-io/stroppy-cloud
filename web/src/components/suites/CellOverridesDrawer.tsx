import { providerQueries } from '@api/queries/settings'
import type { Schemas } from '@api/types'
import { RoleSizesEditor } from '@components/RoleSizesEditor'
import { TagsEditor } from '@components/TagsEditor'
import { Button, Checkbox, Combobox, Drawer, Field, Input, Stack, Switch, Text } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

type SuiteCell = Schemas['SuiteCell']

// Per-cell overrides: applied over axis values and the test's own settings.
export function CellOverridesDrawer({
  cell,
  roles,
  onApply,
  onClose,
}: {
  cell: SuiteCell
  roles: string[]
  onApply: (next: SuiteCell) => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const providers = useQuery(providerQueries.list(slug))
  const [name, setName] = useState(cell.name ?? '')
  const [o, setO] = useState<Schemas['LaunchOverrides']>(structuredClone(cell.overrides ?? {}))
  const [sizesOn, setSizesOn] = useState(!!cell.overrides?.sizes)
  const keepBad = !!o.keep && !/^\d+(h|m|s)$/.test(o.keep)

  const apply = () => {
    const next: Schemas['LaunchOverrides'] = {
      ...o,
      sizes: sizesOn ? (o.sizes ?? cell.axis?.sizes ?? {}) : undefined,
    }
    for (const k of Object.keys(next) as (keyof Schemas['LaunchOverrides'])[]) {
      const v = next[k]
      if (
        v === undefined ||
        v === '' ||
        (typeof v === 'object' && v !== null && Object.keys(v).length === 0)
      )
        delete next[k]
    }
    onApply({
      ...cell,
      name: name.trim() || cell.name,
      overrides: Object.keys(next).length ? next : undefined,
    })
    onClose()
  }

  return (
    <Drawer
      title={t('suites.cells.drawer.title')}
      subtitle={`${cell.name ?? cell.id} · ${t('suites.cells.drawer.subtitle')}`}
      size="sm"
      onClose={onClose}
    >
      <Stack direction="column" gap={1}>
        <Field label={t('suites.cells.drawer.name')} htmlFor="cell-name">
          <Input id="cell-name" value={name} onChange={(e) => setName(e.currentTarget.value)} />
        </Field>
        <Field label={t('suites.cells.drawer.provider')} htmlFor="cell-provider">
          <Combobox
            id="cell-provider"
            isClearable
            placeholder={t('suites.cells.drawer.providerPlaceholder')}
            loading={providers.isPending}
            options={(providers.data?.data ?? []).map((p) => ({
              label: p.name,
              value: p.id,
              description: `${p.kind} · ${t(`common.status.${p.status}`)}`,
            }))}
            value={o.provider_profile_id ?? null}
            onChange={(opt) => setO({ ...o, provider_profile_id: opt?.value })}
          />
        </Field>
        <Field label={t('suites.cells.drawer.sizes')}>
          <Stack direction="column" gap={1}>
            <Stack alignItems="center" gap={1}>
              <Switch
                id="cell-sizes-on"
                value={sizesOn}
                onChange={(e) => setSizesOn(e.currentTarget.checked)}
              />
              <label htmlFor="cell-sizes-on">
                <Text>{t('suites.cells.drawer.sizesToggle')}</Text>
              </label>
            </Stack>
            {sizesOn && (
              <RoleSizesEditor
                value={o.sizes ?? cell.axis?.sizes}
                roles={roles}
                onChange={(sizes) => setO({ ...o, sizes })}
              />
            )}
          </Stack>
        </Field>
        <Field
          label={t('suites.cells.drawer.keep')}
          htmlFor="cell-keep"
          invalid={keepBad}
          error={keepBad ? t('suites.form.errors.keep') : undefined}
          description={t('suites.form.keepHint')}
        >
          <Input
            id="cell-keep"
            width={16}
            placeholder="2h"
            value={o.keep ?? ''}
            onChange={(e) => setO({ ...o, keep: e.currentTarget.value || undefined })}
          />
        </Field>
        <Field label={t('suites.cells.drawer.rating')}>
          <Stack gap={2}>
            <Checkbox
              label={t('suites.form.ratingTenant')}
              value={o.rating?.tenant ?? false}
              onChange={(e) =>
                setO({ ...o, rating: { ...o.rating, tenant: e.currentTarget.checked } })
              }
            />
            <Checkbox
              label={t('suites.form.ratingGlobal')}
              value={o.rating?.global ?? false}
              onChange={(e) =>
                setO({ ...o, rating: { ...o.rating, global: e.currentTarget.checked } })
              }
            />
          </Stack>
        </Field>
        <Field label={t('suites.cells.drawer.labels')}>
          <TagsEditor value={o.labels} onChange={(labels) => setO({ ...o, labels })} />
        </Field>
        <Stack gap={1} justifyContent="space-between">
          <Button
            variant="destructive"
            fill="text"
            icon="trash-alt"
            onClick={() => {
              onApply({ ...cell, overrides: undefined })
              onClose()
            }}
          >
            {t('suites.actions.clearOverrides')}
          </Button>
          <Stack gap={1}>
            <Button variant="secondary" fill="outline" onClick={onClose}>
              {t('common.actions.cancel')}
            </Button>
            <Button disabled={keepBad} onClick={apply}>
              {t('suites.cells.drawer.apply')}
            </Button>
          </Stack>
        </Stack>
      </Stack>
    </Drawer>
  )
}
