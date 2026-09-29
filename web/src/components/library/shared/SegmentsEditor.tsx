import { catalogQueries } from '@api/queries/catalog'
import { applyDefaults, type Schema, type Values } from '@components/schema/engine'
import { SchemaForm } from '@components/schema/SchemaForm'
import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, IconButton, LoadingPlaceholder, Stack, Text, useStyles2 } from '@grafana/ui'
import { useSchemaValidate } from '@hooks/useSchemaValidate'
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

export const SEGMENT_SCHEMA_ID = 'workload.segment'

export function newSegment(index: number, script = 'tpcc/tx'): Values {
  return {
    name: index === 0 ? 'main' : `segment-${index + 1}`,
    workload: { script },
    run: { executor: 'constant-vus', vus: 16, duration: '5m' },
    thresholds: { p99_ms: 200, error_rate: 0.01 },
  }
}

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    display: 'grid',
    gridTemplateColumns: '260px minmax(0, 1fr)',
    gap: theme.spacing(2),
    alignItems: 'start',
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  list: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  item: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(1, 1.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    cursor: 'pointer',
    textAlign: 'left',
    width: '100%',
    color: theme.colors.text.primary,
    '&:hover': { background: theme.colors.action.hover },
  }),
  active: css({
    borderColor: theme.colors.primary.border,
    background: theme.colors.action.selected,
  }),
  num: css({
    width: 22,
    height: 22,
    borderRadius: 999,
    display: 'inline-grid',
    placeItems: 'center',
    background: theme.colors.background.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    flexShrink: 0,
  }),
  meta: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontFamily: theme.typography.fontFamilyMonospace,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  grow: css({ minWidth: 0, flex: 1 }),
  editor: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(2),
    background: theme.colors.background.primary,
  }),
})

// Ordered segments with reorder/add/remove; the selected one is edited via the `workload.segment` schema.
export function SegmentsEditor({
  segments,
  onChange,
  selected,
  onSelect,
  readOnly,
  single,
}: {
  segments: Values[]
  onChange: (segments: Values[]) => void
  selected: number
  onSelect: (index: number) => void
  readOnly?: boolean
  // one segment only, no list (new-workload form)
  single?: boolean
}) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const schemaQ = useQuery(catalogQueries.schema(SEGMENT_SCHEMA_ID))
  const schema = schemaQ.data as Schema | undefined
  const idx = Math.min(Math.max(0, selected), Math.max(0, segments.length - 1))
  const current = segments[idx]
  const full = useMemo(
    () => (schema && current ? applyDefaults(schema.fields, current) : current),
    [schema, current]
  )
  const { errors, checking } = useSchemaValidate(
    SEGMENT_SCHEMA_ID,
    schema,
    full,
    !!current && !readOnly
  )

  const update = (i: number, v: Values) => onChange(segments.map((s, j) => (j === i ? v : s)))
  const move = (i: number, dir: -1 | 1) => {
    const j = i + dir
    if (j < 0 || j >= segments.length) return
    const next = [...segments]
    ;[next[i], next[j]] = [next[j], next[i]]
    onChange(next)
    onSelect(j)
  }
  const remove = (i: number) => {
    onChange(segments.filter((_, j) => j !== i))
    onSelect(Math.max(0, i - 1))
  }
  const add = () => {
    const script = (segments[segments.length - 1]?.workload as { script?: string } | undefined)
      ?.script
    onChange([...segments, newSegment(segments.length, script)])
    onSelect(segments.length)
  }

  const editor = (
    <div className={styles.editor}>
      {!current ? (
        <Stack direction="column" alignItems="center" gap={1}>
          <Text color="secondary">{t('library.segments.empty')}</Text>
          {!readOnly && (
            <Button size="sm" icon="plus" onClick={add}>
              {t('library.segments.add')}
            </Button>
          )}
        </Stack>
      ) : schema ? (
        <Stack direction="column" gap={1}>
          {!single && (
            <Stack justifyContent="space-between" alignItems="center">
              <Text weight="medium">
                {t('library.segments.editing', { n: idx + 1, name: String(current.name ?? '') })}
              </Text>
              <Text color="secondary" variant="bodySmall">
                {checking
                  ? t('schema.validating')
                  : Object.keys(errors).length
                    ? t('common.errors.validation')
                    : t('schema.valid')}
              </Text>
            </Stack>
          )}
          <SchemaForm
            schema={schema}
            value={full}
            onChange={(v) => update(idx, v)}
            errors={errors}
            readOnly={readOnly}
          />
        </Stack>
      ) : (
        <LoadingPlaceholder text={t('common.misc.loading')} />
      )}
    </div>
  )
  if (single) return editor
  return (
    <div className={styles.root}>
      <div className={styles.list}>
        {segments.map((s, i) => {
          const w = s.workload as { script?: string } | undefined
          const r = s.run as { vus?: number; duration?: string; iterations?: number } | undefined
          return (
            <div
              key={`${i}-${String(s.name)}`}
              className={cx(styles.item, i === idx && styles.active)}
            >
              <button
                type="button"
                className={cx(styles.grow)}
                style={{
                  all: 'unset',
                  cursor: 'pointer',
                  minWidth: 0,
                  flex: 1,
                  display: 'flex',
                  gap: 8,
                  alignItems: 'center',
                }}
                onClick={() => onSelect(i)}
              >
                <span className={styles.num}>{i + 1}</span>
                <span className={styles.grow}>
                  <div
                    style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                  >
                    {String(s.name ?? '—')}
                  </div>
                  <div className={styles.meta}>
                    {w?.script ?? '—'}
                    {r?.vus ? ` · ${r.vus}vu` : ''}
                    {r?.duration ? ` · ${r.duration}` : r?.iterations ? ` · ${r.iterations}it` : ''}
                  </div>
                </span>
              </button>
              {!readOnly && (
                <Stack gap={0} direction="column">
                  <IconButton
                    name="arrow-up"
                    size="sm"
                    tooltip={t('library.segments.moveUp')}
                    disabled={i === 0}
                    onClick={() => move(i, -1)}
                  />
                  <IconButton
                    name="arrow-down"
                    size="sm"
                    tooltip={t('library.segments.moveDown')}
                    disabled={i === segments.length - 1}
                    onClick={() => move(i, 1)}
                  />
                </Stack>
              )}
              {!readOnly && (
                <IconButton
                  name="trash-alt"
                  size="sm"
                  tooltip={t('common.actions.remove')}
                  onClick={() => remove(i)}
                />
              )}
            </div>
          )
        })}
        {!readOnly && (
          <Button size="sm" variant="secondary" icon="plus" onClick={add}>
            {t('library.segments.add')}
          </Button>
        )}
      </div>
      {editor}
    </div>
  )
}
