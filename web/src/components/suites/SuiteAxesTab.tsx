import { catalogQueries } from '@api/queries/catalog'
import { testQueries } from '@api/queries/library'
import { providerQueries } from '@api/queries/settings'
import { suiteMutations, suiteQueries } from '@api/queries/suites'
import type { Schemas } from '@api/types'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { RoleSizesEditor } from '@components/RoleSizesEditor'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Badge,
  Button,
  Field,
  IconButton,
  Input,
  MultiCombobox,
  Stack,
  Text,
  useStyles2,
} from '@grafana/ui'
import { sortRoles } from '@helpers/sizes'
import { useTenant } from '@hooks/useTenant'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

type Suite = Schemas['Suite']
type Axes = Schemas['SuiteAxes']
type Variant = NonNullable<Axes['workload_variants']>[number]

interface Draft {
  testIds: string[]
  axes: Axes
}

const getStyles = (theme: GrafanaTheme2) => ({
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) 320px',
    gap: theme.spacing(3),
    marginTop: theme.spacing(2),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: '1fr' },
  }),
  section: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    padding: theme.spacing(2),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
  }),
  hint: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
  rows: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  variantRow: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(120px, 1.5fr) 90px 110px 100px 32px',
    gap: theme.spacing(1),
    alignItems: 'center',
  }),
  preview: css({
    position: 'sticky',
    top: theme.spacing(2),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
    padding: theme.spacing(2),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
  }),
  big: css({
    fontSize: theme.typography.h3.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
  }),
})

function draftOf(suite: Suite): Draft {
  return {
    testIds: (suite.tests ?? []).flatMap((x) => ('ref' in x ? [x.ref.id] : [])),
    axes: structuredClone(suite.axes ?? {}),
  }
}

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const id = window.setTimeout(() => setV(value), ms)
    return () => window.clearTimeout(id)
  }, [value, ms])
  return v
}

export function SuiteAxesTab({ suite, readOnly }: { suite: Suite; readOnly?: boolean }) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const { slug } = useTenant()
  const qc = useQueryClient()
  const [draft, setDraft] = useState<Draft>(() => draftOf(suite))
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset the draft when the server copy changes
  useEffect(() => setDraft(draftOf(suite)), [suite.id, suite.updated_at])
  const dirty = JSON.stringify(draft) !== JSON.stringify(draftOf(suite))

  const tests = useQuery(testQueries.options(slug))
  const providers = useQuery(providerQueries.list(slug))
  const catalog = useQuery(catalogQueries.databases())

  const selectedTests = useMemo(
    () =>
      draft.testIds
        .map((id) => tests.data?.data.find((x) => x.id === id))
        .filter((x): x is NonNullable<typeof x> => !!x),
    [draft.testIds, tests.data]
  )
  const roles = useMemo(() => {
    const set = new Set<string>(['db', 'runner'])
    for (const x of selectedTests) for (const r of Object.keys(x.sizes ?? {})) set.add(r)
    for (const s of draft.axes.sizes ?? []) for (const r of Object.keys(s)) set.add(r)
    return sortRoles([...set])
  }, [selectedTests, draft.axes.sizes])
  const kinds = useMemo(
    () => new Set(selectedTests.map((x) => x.summary?.db_kind).filter(Boolean)),
    [selectedTests]
  )
  const versionOptions = useMemo(() => {
    const out: { label: string; value: string; description?: string }[] = []
    const seen = new Set<string>()
    for (const db of catalog.data?.data ?? []) {
      if (kinds.size && !kinds.has(db.kind)) continue
      for (const v of db.versions) {
        const key = v.version
        if (seen.has(key)) continue
        seen.add(key)
        out.push({ label: v.version, value: v.version, description: db.title })
      }
    }
    // keep already selected versions visible even if the catalog does not list them
    for (const v of draft.axes.database_versions ?? [])
      if (!seen.has(v)) out.push({ label: v, value: v })
    return out
  }, [catalog.data, kinds, draft.axes.database_versions])

  const previewBody = useMemo<Schemas['SuiteWrite']>(
    () => ({
      name: suite.name,
      tests: draft.testIds.map((id) => ({ ref: { id } })),
      axes: draft.axes,
      cells: suite.cells,
      concurrency: suite.concurrency,
      defaults: suite.defaults,
    }),
    [suite, draft]
  )
  const debouncedBody = useDebounced(previewBody, 400)
  const preview = useQuery(suiteQueries.preview(slug, debouncedBody))
  const invalidCount =
    preview.data?.cells.filter((c) => c.enabled && c.validation && !c.validation.fits).length ?? 0

  const save = useMutation({
    mutationFn: () =>
      suiteMutations.patch(slug, suite.id, {
        tests: draft.testIds.map((id) => ({ ref: { id } })),
        axes: draft.axes,
        concurrency: suite.concurrency,
      }),
    onSuccess: () => {
      toast.success(t('suites.axes.saved'))
      void qc.invalidateQueries({ queryKey: ['t', slug, 'suites'] })
    },
    onError: (e) => toast.error(e),
  })

  const setAxes = (patch: Partial<Axes>) =>
    setDraft((d) => ({ ...d, axes: { ...d.axes, ...patch } }))
  const variants = draft.axes.workload_variants ?? []
  const setVariant = (i: number, patch: Partial<Variant>) =>
    setAxes({ workload_variants: variants.map((v, j) => (j === i ? { ...v, ...patch } : v)) })
  const variantErrors = variants.map((v) => ({
    name: !v.name?.trim() ? t('suites.axes.errors.variantName') : undefined,
    duration:
      v.duration && !/^\d+(h|m|s)$/.test(v.duration) ? t('suites.axes.errors.duration') : undefined,
  }))
  const hasLocalErrors = variantErrors.some((e) => e.name || e.duration)

  return (
    <div className={styles.grid}>
      <Stack direction="column" gap={2}>
        <Text color="secondary">{t('suites.axes.intro')}</Text>

        <section className={styles.section}>
          <SectionTitle>{t('suites.axes.tests')}</SectionTitle>
          <MultiCombobox
            options={(tests.data?.data ?? []).map((x) => ({
              label: x.name,
              value: x.id,
              description: [x.summary?.db_kind, x.summary?.db_version, x.summary?.topology_label]
                .filter(Boolean)
                .join(' · '),
            }))}
            value={draft.testIds}
            placeholder={t('suites.form.testsPlaceholder')}
            loading={tests.isPending}
            disabled={readOnly}
            onChange={(opts) => setDraft((d) => ({ ...d, testIds: opts.map((o) => o.value) }))}
          />
          <span className={styles.hint}>{t('suites.form.testsHint')}</span>
        </section>

        <section className={styles.section}>
          <SectionTitle>{t('suites.axes.providers')}</SectionTitle>
          <MultiCombobox
            options={(providers.data?.data ?? []).map((p) => ({
              label: p.name,
              value: p.id,
              description: `${p.kind} · ${t(`common.status.${p.status}`)}`,
            }))}
            value={draft.axes.provider_profiles ?? []}
            placeholder={t('suites.axes.providersPlaceholder')}
            loading={providers.isPending}
            disabled={readOnly}
            isClearable
            onChange={(opts) =>
              setAxes({ provider_profiles: opts.length ? opts.map((o) => o.value) : undefined })
            }
          />
          <span className={styles.hint}>{t('suites.axes.providersHint')}</span>
        </section>

        <section className={styles.section}>
          <SectionTitle
            right={
              !readOnly ? (
                <Button
                  size="sm"
                  variant="secondary"
                  icon="plus"
                  onClick={() =>
                    setAxes({
                      sizes: [
                        ...(draft.axes.sizes ?? []),
                        structuredClone(
                          draft.axes.sizes?.at(-1) ??
                            selectedTests[0]?.sizes ??
                            Object.fromEntries(
                              roles.map((r) => [r, { size: r === 'runner' ? 'S' : 'M' }])
                            )
                        ) as Schemas['RoleSizes'],
                      ],
                    })
                  }
                >
                  {t('suites.actions.addSizesSet')}
                </Button>
              ) : undefined
            }
          >
            {t('suites.axes.sizes')}
          </SectionTitle>
          <div className={styles.rows}>
            {(draft.axes.sizes ?? []).length === 0 && (
              <span className={styles.hint}>{t('suites.axes.noSizes')}</span>
            )}
            {(draft.axes.sizes ?? []).map((s, i) => (
              <RoleSizesEditor
                key={i}
                value={s}
                roles={roles}
                disabled={readOnly}
                onChange={(next) =>
                  setAxes({ sizes: (draft.axes.sizes ?? []).map((x, j) => (j === i ? next : x)) })
                }
                onRemove={
                  readOnly
                    ? undefined
                    : () => {
                        const next = (draft.axes.sizes ?? []).filter((_, j) => j !== i)
                        setAxes({ sizes: next.length ? next : undefined })
                      }
                }
              />
            ))}
          </div>
          <span className={styles.hint}>{t('suites.axes.sizesHint')}</span>
        </section>

        <section className={styles.section}>
          <SectionTitle>{t('suites.axes.versions')}</SectionTitle>
          <MultiCombobox
            options={versionOptions}
            value={draft.axes.database_versions ?? []}
            placeholder={t('suites.axes.versionsPlaceholder')}
            loading={catalog.isPending}
            disabled={readOnly}
            isClearable
            onChange={(opts) =>
              setAxes({ database_versions: opts.length ? opts.map((o) => o.value) : undefined })
            }
          />
          <span className={styles.hint}>{t('suites.axes.versionsHint')}</span>
        </section>

        <section className={styles.section}>
          <SectionTitle
            right={
              !readOnly ? (
                <Button
                  size="sm"
                  variant="secondary"
                  icon="plus"
                  onClick={() =>
                    setAxes({
                      workload_variants: [...variants, { name: `variant-${variants.length + 1}` }],
                    })
                  }
                >
                  {t('suites.actions.addVariant')}
                </Button>
              ) : undefined
            }
          >
            {t('suites.axes.variants')}
          </SectionTitle>
          <div className={styles.rows}>
            {variants.length === 0 && (
              <span className={styles.hint}>{t('suites.axes.noVariants')}</span>
            )}
            {variants.length > 0 && (
              <div className={styles.variantRow}>
                <span className={styles.hint}>{t('suites.axes.variant.name')}</span>
                <span className={styles.hint}>{t('suites.axes.variant.vus')}</span>
                <span className={styles.hint}>{t('suites.axes.variant.scale')}</span>
                <span className={styles.hint}>{t('suites.axes.variant.duration')}</span>
                <span />
              </div>
            )}
            {variants.map((v, i) => (
              <div key={i} className={styles.variantRow}>
                <Field invalid={!!variantErrors[i].name} error={variantErrors[i].name} noMargin>
                  <Input
                    value={v.name ?? ''}
                    placeholder={t('suites.axes.variant.namePlaceholder')}
                    disabled={readOnly}
                    aria-label={t('suites.axes.variant.name')}
                    onChange={(e) => setVariant(i, { name: e.currentTarget.value })}
                  />
                </Field>
                <Input
                  type="number"
                  min={1}
                  value={v.vus ?? ''}
                  disabled={readOnly}
                  aria-label={t('suites.axes.variant.vus')}
                  onChange={(e) =>
                    setVariant(i, {
                      vus: e.currentTarget.value ? Number(e.currentTarget.value) : undefined,
                    })
                  }
                />
                <Input
                  type="number"
                  min={1}
                  value={v.scale_factor ?? ''}
                  disabled={readOnly}
                  aria-label={t('suites.axes.variant.scale')}
                  onChange={(e) =>
                    setVariant(i, {
                      scale_factor: e.currentTarget.value
                        ? Number(e.currentTarget.value)
                        : undefined,
                    })
                  }
                />
                <Field
                  invalid={!!variantErrors[i].duration}
                  error={variantErrors[i].duration}
                  noMargin
                >
                  <Input
                    value={v.duration ?? ''}
                    placeholder={t('suites.axes.variant.durationPlaceholder')}
                    disabled={readOnly}
                    aria-label={t('suites.axes.variant.duration')}
                    onChange={(e) =>
                      setVariant(i, { duration: e.currentTarget.value || undefined })
                    }
                  />
                </Field>
                <IconButton
                  name="trash-alt"
                  tooltip={t('common.actions.remove')}
                  disabled={readOnly}
                  onClick={() => {
                    const next = variants.filter((_, j) => j !== i)
                    setAxes({ workload_variants: next.length ? next : undefined })
                  }}
                />
              </div>
            ))}
          </div>
          <span className={styles.hint}>{t('suites.axes.variantsHint')}</span>
        </section>
      </Stack>

      <aside className={styles.preview}>
        <SectionTitle
          right={
            dirty ? <Badge text={t('suites.axes.dirty')} color="orange" icon="edit" /> : undefined
          }
        >
          {t('suites.axes.preview')}
        </SectionTitle>
        <span className={styles.big}>
          {preview.data
            ? t('suites.axes.previewCells', { count: preview.data.cells.length })
            : t('common.misc.loading')}
        </span>
        {preview.data?.totals && (
          <span className={styles.hint}>
            {t('suites.axes.previewMachines', {
              machines: preview.data.totals.machines ?? 0,
              cpu: preview.data.totals.cpu ?? 0,
              ram: preview.data.totals.memory_gb ?? 0,
            })}
          </span>
        )}
        {invalidCount > 0 && (
          <Alert
            severity="warning"
            title={t('suites.axes.previewInvalid', { count: invalidCount })}
          />
        )}
        {preview.isError && <Alert severity="error" title={t('common.errors.generic')} />}
        {!readOnly && (
          <Stack gap={1}>
            <Button
              icon="save"
              disabled={!dirty || save.isPending || hasLocalErrors}
              onClick={() => save.mutate()}
            >
              {t('suites.actions.saveAxes')}
            </Button>
            <Button
              variant="secondary"
              fill="outline"
              disabled={!dirty}
              onClick={() => setDraft(draftOf(suite))}
            >
              {t('suites.actions.discard')}
            </Button>
          </Stack>
        )}
      </aside>
    </div>
  )
}
