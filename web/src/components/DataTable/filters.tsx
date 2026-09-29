import { css, cx } from '@emotion/css'
import { dateTime, type GrafanaTheme2 } from '@grafana/data'
import { Button, Checkbox, DateTimePicker, Input, Text, useStyles2 } from '@grafana/ui'
import { type KeyboardEvent, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type {
  ChecklistColumnFilter,
  DateColumnFilter,
  NumberColumnFilter,
  TextColumnFilter,
} from './types'

const getStyles = (theme: GrafanaTheme2) => ({
  body: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1), minWidth: 220 }),
  list: css({
    display: 'flex',
    flexDirection: 'column',
    maxHeight: 280,
    overflowY: 'auto',
    margin: theme.spacing(0, -1),
  }),
  row: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0.5, 1),
    cursor: 'pointer',
    '&:hover': { background: theme.colors.action.hover },
    label: { cursor: 'pointer' },
  }),
  rowAll: css({ borderBottom: `1px solid ${theme.colors.border.weak}` }),
  rowLabel: css({
    flex: 1,
    minWidth: 0,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  }),
  count: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontVariantNumeric: 'tabular-nums',
  }),
  range: css({ display: 'flex', gap: theme.spacing(1), alignItems: 'flex-end' }),
  field: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), flex: 1 }),
  footer: css({ display: 'flex', justifyContent: 'flex-end', gap: theme.spacing(0.5) }),
})

// Multi-select checklist with facet counts; the "All" row clears the filter.
export function ChecklistFilter({ filter }: { filter: ChecklistColumnFilter }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const selected = new Set(filter.value ?? [])
  const none = selected.size === 0
  const toggle = (v: string) => {
    if (filter.single) {
      filter.onChange(selected.has(v) ? undefined : [v])
      return
    }
    const next = new Set(selected)
    if (next.has(v)) next.delete(v)
    else next.add(v)
    filter.onChange(next.size ? [...next] : undefined)
  }
  return (
    <div className={styles.body}>
      <div className={styles.list}>
        <div className={cx(styles.row, styles.rowAll)}>
          <Checkbox
            value={none}
            label={t('common.misc.all')}
            onChange={() => filter.onChange(undefined)}
          />
        </div>
        {filter.options.length === 0 && (
          <div className={styles.row}>
            <Text color="secondary" variant="bodySmall">
              {t('common.table.noOptions')}
            </Text>
          </div>
        )}
        {filter.options.map((o) => (
          <div key={o.value} className={styles.row}>
            <Checkbox value={selected.has(o.value)} onChange={() => toggle(o.value)} />
            <button
              type="button"
              className={cx(styles.rowLabel, css({ all: 'unset', cursor: 'pointer' }))}
              onClick={() => toggle(o.value)}
              title={o.label}
            >
              {o.label}
            </button>
            {o.count !== undefined && <span className={styles.count}>{o.count}</span>}
          </div>
        ))}
      </div>
    </div>
  )
}

// Free text; commits on Enter / blur so a refetch never interrupts typing.
export function TextFilter({ filter }: { filter: TextColumnFilter }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [local, setLocal] = useState(filter.value ?? '')
  useEffect(() => setLocal(filter.value ?? ''), [filter.value])
  const commit = () => {
    const v = local.trim()
    if (v !== (filter.value ?? '')) filter.onChange(v || undefined)
  }
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') commit()
  }
  return (
    <div className={styles.body}>
      <Input
        autoFocus
        value={local}
        placeholder={filter.placeholder ?? t('common.table.textFilterPlaceholder')}
        onChange={(e) => setLocal(e.currentTarget.value)}
        onBlur={commit}
        onKeyDown={onKey}
      />
      <div className={styles.footer}>
        <Button
          size="sm"
          variant="secondary"
          fill="text"
          disabled={!local && !filter.value}
          onClick={() => {
            setLocal('')
            filter.onChange(undefined)
          }}
        >
          {t('common.actions.clear')}
        </Button>
        <Button size="sm" variant="primary" onClick={commit}>
          {t('common.actions.apply')}
        </Button>
      </div>
    </div>
  )
}

function toIso(d: ReturnType<typeof dateTime> | undefined): string | undefined {
  return d?.isValid() ? d.toISOString() : undefined
}

// From / to date-time bounds (ISO in the URL).
export function DateRangeFilter({ filter }: { filter: DateColumnFilter }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  return (
    <div className={styles.body}>
      <div className={styles.field}>
        <Text variant="bodySmall" color="secondary">
          {t('common.table.from')}
        </Text>
        <DateTimePicker
          date={filter.from ? dateTime(filter.from) : undefined}
          clearable
          onChange={(d) => filter.onChange(toIso(d), filter.to)}
        />
      </div>
      <div className={styles.field}>
        <Text variant="bodySmall" color="secondary">
          {t('common.table.to')}
        </Text>
        <DateTimePicker
          date={filter.to ? dateTime(filter.to) : undefined}
          clearable
          onChange={(d) => filter.onChange(filter.from, toIso(d))}
        />
      </div>
      <div className={styles.footer}>
        <Button
          size="sm"
          variant="secondary"
          fill="text"
          disabled={!filter.from && !filter.to}
          onClick={() => filter.onChange(undefined, undefined)}
        >
          {t('common.actions.clear')}
        </Button>
      </div>
    </div>
  )
}

function parseNum(v: string): number | undefined {
  if (v.trim() === '') return undefined
  const n = Number(v)
  return Number.isFinite(n) ? n : undefined
}

// Min / max numeric bounds; commits on Enter / blur.
export function NumberRangeFilter({ filter }: { filter: NumberColumnFilter }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const [lo, setLo] = useState(filter.min?.toString() ?? '')
  const [hi, setHi] = useState(filter.max?.toString() ?? '')
  useEffect(() => setLo(filter.min?.toString() ?? ''), [filter.min])
  useEffect(() => setHi(filter.max?.toString() ?? ''), [filter.max])
  const commit = () => {
    const min = parseNum(lo)
    const max = parseNum(hi)
    if (min !== filter.min || max !== filter.max) filter.onChange(min, max)
  }
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') commit()
  }
  const suffix = filter.unit ? <Text color="secondary">{filter.unit}</Text> : undefined
  return (
    <div className={styles.body}>
      <div className={styles.range}>
        <div className={styles.field}>
          <Text variant="bodySmall" color="secondary">
            {t('common.table.min')}
          </Text>
          <Input
            type="number"
            step={filter.step}
            value={lo}
            suffix={suffix}
            onChange={(e) => setLo(e.currentTarget.value)}
            onBlur={commit}
            onKeyDown={onKey}
          />
        </div>
        <div className={styles.field}>
          <Text variant="bodySmall" color="secondary">
            {t('common.table.max')}
          </Text>
          <Input
            type="number"
            step={filter.step}
            value={hi}
            suffix={suffix}
            onChange={(e) => setHi(e.currentTarget.value)}
            onBlur={commit}
            onKeyDown={onKey}
          />
        </div>
      </div>
      <div className={styles.footer}>
        <Button
          size="sm"
          variant="secondary"
          fill="text"
          disabled={filter.min === undefined && filter.max === undefined}
          onClick={() => {
            setLo('')
            setHi('')
            filter.onChange(undefined, undefined)
          }}
        >
          {t('common.actions.clear')}
        </Button>
        <Button size="sm" variant="primary" onClick={commit}>
          {t('common.actions.apply')}
        </Button>
      </div>
    </div>
  )
}
