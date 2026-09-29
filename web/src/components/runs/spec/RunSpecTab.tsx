import { catalogQueries } from '@api/queries/catalog'
import type { Run } from '@api/types'
import { SectionTitle } from '@app/PageHeader'
import { toast } from '@app/Toaster'
import { CodeEditorLazy } from '@components/code/CodeEditorLazy'
import type { Schema } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Badge,
  Button,
  CollapsableSection,
  LoadingPlaceholder,
  RadioButtonGroup,
  Stack,
  Text,
  useStyles2,
} from '@grafana/ui'
import { toYaml } from '@helpers/yaml'
import { useRun } from '@hooks/useRun'
import { useTenant } from '@hooks/useTenant'
import { downloadText } from '@lib/download'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SpecDiffDrawer } from './SpecDiffDrawer'

const getStyles = (theme: GrafanaTheme2) => ({
  // Wide screens: both columns fill the tab body; the editor and the snapshot column scroll on
  // their own (web/AGENTS.md §18). Narrow: one column, the tab body scrolls.
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 3fr) minmax(0, 2fr)',
    gap: theme.spacing(3),
    flex: '1 1 auto',
    minHeight: 0,
    minWidth: 0,
    [theme.breakpoints.down('xl')]: { gridTemplateColumns: 'minmax(0, 1fr)', flex: 'none' },
  }),
  col: css({ minWidth: 0, minHeight: 0, overflow: 'auto' }),
  editorCol: css({
    minWidth: 0,
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1.5),
  }),
  panel: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(2),
    minWidth: 0,
  }),
  editorWrap: css({
    flex: '1 1 auto',
    minHeight: 360,
    display: 'flex',
    flexDirection: 'column',
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    overflow: 'hidden',
  }),
  bar: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1), flexWrap: 'wrap' }),
  right: css({ marginLeft: 'auto', display: 'flex', gap: theme.spacing(1) }),
  kv: css({
    display: 'grid',
    gridTemplateColumns: 'max-content minmax(0, 1fr)',
    gap: theme.spacing(0.5, 2),
    fontSize: theme.typography.bodySmall.fontSize,
    '& > span:nth-of-type(odd)': { color: theme.colors.text.secondary },
    '& > span:nth-of-type(even)': { minWidth: 0, overflowWrap: 'anywhere' },
  }),
})

function DatabaseParams({ run }: { run: Run }) {
  const { t } = useTranslation()
  const schemaId = run.snapshot.database.schema?.id ?? `db.${run.snapshot.database.kind}.params`
  const schema = useQuery(catalogQueries.schema(schemaId))
  if (schema.isPending) return <LoadingPlaceholder text={t('common.misc.loading')} />
  if (schema.isError)
    return (
      <CodeEditorLazy
        value={JSON.stringify(run.snapshot.database.params, null, 2)}
        language="json"
        height={260}
      />
    )
  return (
    <SchemaForm
      schema={schema.data as unknown as Schema}
      value={run.snapshot.database.params}
      onChange={() => undefined}
      readOnly
      dense
      diffOnly
    />
  )
}

export function RunSpecTab({ id }: { id: string }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const { slug } = useTenant()
  const run = useRun(slug, id).data
  const [format, setFormat] = useState<'yaml' | 'json'>('yaml')
  const [diff, setDiff] = useState(false)
  const values = run?.run_spec?.values
  const text = useMemo(
    () => (values ? (format === 'json' ? JSON.stringify(values, null, 2) : toYaml(values)) : ''),
    [values, format]
  )
  if (!run) return null
  const snap = run.snapshot
  const segments = snap.workload.segments as Record<string, unknown>[]
  const configs = Object.entries(snap.database.configs ?? {})
  const effective = Object.entries(snap.effective_configs ?? {})

  return (
    <div className={styles.grid}>
      <div className={styles.editorCol}>
        <div className={styles.bar}>
          <SectionTitle>{t('runs.spec.runSpec')}</SectionTitle>
          {run.run_spec?.schema && typeof run.run_spec.schema.id === 'string' && (
            <Badge
              text={`${run.run_spec.schema.id}@${run.run_spec.schema.version ?? ''}`}
              color="darkgrey"
            />
          )}
          <div className={styles.right}>
            <RadioButtonGroup
              size="sm"
              value={format}
              options={[
                { value: 'yaml', label: 'YAML' },
                { value: 'json', label: 'JSON' },
              ]}
              onChange={setFormat}
            />
            <Button
              size="sm"
              variant="secondary"
              icon="copy"
              disabled={!text}
              onClick={() => {
                void navigator.clipboard.writeText(text)
                toast.success(t('common.actions.copied'))
              }}
            >
              {t('common.actions.copy')}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              icon="download-alt"
              disabled={!text}
              onClick={() =>
                downloadText(
                  `run-spec-${run.id}.${format}`,
                  text,
                  format === 'json' ? 'application/json' : 'text/yaml'
                )
              }
            >
              {t('common.actions.download')}
            </Button>
            <Button size="sm" icon="columns" onClick={() => setDiff(true)}>
              {t('runs.spec.diff.button')}
            </Button>
          </div>
        </div>
        {values ? (
          <div className={styles.editorWrap}>
            <CodeEditorLazy value={text} language={format} height="fill" />
          </div>
        ) : (
          <Alert severity="info" title={t('runs.spec.noSpec')}>
            {t('runs.spec.noSpecHint')}
          </Alert>
        )}
        <Text color="secondary" variant="bodySmall">
          {t('runs.spec.secretsNote')}
        </Text>
      </div>

      <div className={styles.col}>
        <Stack direction="column" gap={2}>
          <section className={styles.panel}>
            <Stack direction="column" gap={1.5}>
              <SectionTitle
                right={
                  <Badge
                    text={`${snap.database.kind} ${snap.database.version}`}
                    color="blue"
                    icon="database"
                  />
                }
              >
                {t('runs.spec.database')}
              </SectionTitle>
              <div className={styles.kv}>
                <span>{t('common.fields.name')}</span>
                <span>{snap.database_name ?? '—'}</span>
                {snap.database.image && (
                  <>
                    <span>{t('runs.overview.components.image')}</span>
                    <span>{snap.database.image}</span>
                  </>
                )}
                {snap.database.external?.dsn && (
                  <>
                    <span>DSN</span>
                    <span>{snap.database.external.dsn}</span>
                  </>
                )}
              </div>
              <CollapsableSection label={t('runs.spec.params')} isOpen>
                <DatabaseParams run={run} />
              </CollapsableSection>
              {configs.length > 0 && (
                <CollapsableSection
                  label={t('runs.spec.configs', { count: configs.length })}
                  isOpen={false}
                >
                  <Stack direction="column" gap={1}>
                    {configs.map(([role, byId]) => (
                      <div key={role}>
                        <Text weight="medium">{role}</Text>
                        {Object.entries(byId).map(([sid, v]) => (
                          <CollapsableSection key={sid} label={sid} isOpen={false}>
                            <CodeEditorLazy value={toYaml(v)} language="yaml" height={200} />
                          </CollapsableSection>
                        ))}
                      </div>
                    ))}
                  </Stack>
                </CollapsableSection>
              )}
              {effective.length > 0 && (
                <CollapsableSection label={t('runs.spec.effectiveConfigs')} isOpen={false}>
                  <Stack direction="column" gap={1}>
                    {effective.map(([role, byId]) => (
                      <div key={role}>
                        <Text weight="medium">{role}</Text>
                        {Object.entries(byId).map(([sid, v]) => (
                          <CollapsableSection key={sid} label={sid} isOpen={false}>
                            <CodeEditorLazy value={toYaml(v.values)} language="yaml" height={200} />
                          </CollapsableSection>
                        ))}
                      </div>
                    ))}
                  </Stack>
                </CollapsableSection>
              )}
            </Stack>
          </section>

          <section className={styles.panel}>
            <Stack direction="column" gap={1.5}>
              <SectionTitle
                right={
                  <Badge
                    text={`stroppy ${snap.workload.stroppy_version} · ${snap.workload.protocol}`}
                    color="purple"
                    icon="rocket"
                  />
                }
              >
                {t('runs.spec.workload')}
              </SectionTitle>
              <div className={styles.kv}>
                <span>{t('common.fields.name')}</span>
                <span>{snap.workload_name ?? '—'}</span>
                <span>{t('runs.overview.snapshot.segments')}</span>
                <span>{segments.length}</span>
              </div>
              {segments.map((s, i) => (
                <CollapsableSection
                  key={String(s.name ?? i)}
                  label={`${i + 1}. ${String(s.name ?? t('runs.spec.segment'))}`}
                  isOpen={i === 0}
                >
                  <CodeEditorLazy value={toYaml(s)} language="yaml" height={180} />
                </CollapsableSection>
              ))}
              {snap.workload.options && (
                <CollapsableSection label={t('runs.spec.workloadOptions')} isOpen={false}>
                  <CodeEditorLazy
                    value={toYaml(snap.workload.options)}
                    language="yaml"
                    height={180}
                  />
                </CollapsableSection>
              )}
            </Stack>
          </section>

          <section className={styles.panel}>
            <Stack direction="column" gap={1.5}>
              <SectionTitle>{t('runs.spec.sizes')}</SectionTitle>
              <div className={styles.kv}>
                {Object.entries(snap.sizes).map(([role, v]) => (
                  <span key={role} style={{ display: 'contents' }}>
                    <span>{role}</span>
                    <span>
                      <Badge text={v.size} color="darkgrey" />{' '}
                      {v.disk?.gb ? `${v.disk.gb} GB ${v.disk.type ?? ''}` : ''}
                    </span>
                  </span>
                ))}
                <span>{t('common.fields.provider')}</span>
                <span>{snap.provider_profile.name ?? snap.provider_profile.id}</span>
                <span>{t('runs.overview.snapshot.keep')}</span>
                <span>{snap.keep ?? '0s'}</span>
              </div>
              {snap.machines && snap.machines.length > 0 && (
                <CollapsableSection
                  label={t('runs.spec.machines', { count: snap.machines.length })}
                  isOpen={false}
                >
                  <CodeEditorLazy value={toYaml(snap.machines)} language="yaml" height={220} />
                </CollapsableSection>
              )}
              {snap.execution && Object.keys(snap.execution).length > 0 && (
                <CollapsableSection label={t('runs.spec.execution')} isOpen={false}>
                  <CodeEditorLazy value={toYaml(snap.execution)} language="yaml" height={180} />
                </CollapsableSection>
              )}
            </Stack>
          </section>
        </Stack>
      </div>
      {diff && <SpecDiffDrawer run={run} onClose={() => setDiff(false)} />}
    </div>
  )
}
