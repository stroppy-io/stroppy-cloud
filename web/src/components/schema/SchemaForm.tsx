import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import {
  Alert,
  Button,
  Checkbox,
  CollapsableSection,
  Combobox,
  Field,
  FieldSet,
  IconButton,
  Input,
  RadioButtonGroup,
  SecretInput,
  Stack,
  Switch,
  Text,
  TextArea,
  useStyles2,
} from '@grafana/ui'
import { type ReactNode, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  applyDefaults,
  choiceRaw,
  evalWhen,
  fieldDefault,
  fieldKind,
  groupFields,
  listItem,
  objectFields,
  type Schema,
  type SchemaField,
  type Values,
} from './engine'

export type Widget = (props: {
  field: SchemaField
  value: unknown
  onChange: (v: unknown) => void
  path: string
  root: Values
  readOnly?: boolean
  error?: string
}) => ReactNode

export interface SchemaFormProps {
  schema: Schema
  value: Values | undefined
  onChange: (v: Values) => void
  // `{ 'a.b[0].c': message }` from the server or local validation.
  errors?: Record<string, string>
  readOnly?: boolean
  // Override rendering by dotted path (or `*` suffix for prefix match) — topology previews, segment editors, …
  widgets?: Record<string, Widget>
  // Collapse groups after the first N; hide computed fields entirely when false.
  showComputed?: boolean
  dense?: boolean
  // Show only the fields whose values differ from defaults + all required (diff editing for configs).
  diffOnly?: boolean
}

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
    gap: theme.spacing(0, 2),
  }),
  full: css({ gridColumn: '1 / -1' }),
  nested: css({
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(1.5, 2, 0),
    marginBottom: theme.spacing(2),
    background: theme.colors.background.secondary,
  }),
  row: css({
    display: 'flex',
    gap: theme.spacing(1),
    alignItems: 'flex-start',
    marginBottom: theme.spacing(1),
  }),
  unit: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    padding: theme.spacing(0, 1),
  }),
  computed: css({ opacity: 0.8 }),
  desc: css({ color: theme.colors.text.secondary, fontSize: theme.typography.bodySmall.fontSize }),
  ro: css({
    padding: theme.spacing(0.5, 0),
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: theme.typography.bodySmall.fontSize,
    wordBreak: 'break-word',
  }),
  pre: css({ margin: 0, whiteSpace: 'pre-wrap', font: 'inherit' }),
})

function resolveWidget(widgets: SchemaFormProps['widgets'], path: string): Widget | undefined {
  if (!widgets) return undefined
  if (widgets[path]) return widgets[path]
  for (const [k, w] of Object.entries(widgets))
    if (k.endsWith('*') && path.startsWith(k.slice(0, -1))) return w
  return undefined
}

function humanError(
  t: (k: string, o?: Record<string, unknown>) => string,
  code: string | undefined
): string | undefined {
  if (!code) return undefined
  const [k, arg] = code.split(':')
  switch (k) {
    case 'required':
      return t('common.validation.required')
    case 'integer':
      return t('schema.err.integer')
    case 'number':
      return t('schema.err.number')
    case 'gte':
      return t('schema.err.gte', { n: arg })
    case 'lte':
      return t('schema.err.lte', { n: arg })
    case 'gt':
      return t('schema.err.gt', { n: arg })
    case 'lt':
      return t('schema.err.lt', { n: arg })
    case 'minLen':
      return t('common.validation.min', { min: arg })
    case 'maxLen':
      return t('common.validation.max', { max: arg })
    case 'pattern':
      return t('schema.err.pattern', { p: arg })
    default:
      return code
  }
}

export function SchemaForm({
  schema,
  value,
  onChange,
  errors,
  readOnly,
  widgets,
  showComputed = true,
  dense,
  diffOnly,
}: SchemaFormProps) {
  const styles = useStyles2(getStyles)
  const values = useMemo(() => applyDefaults(schema.fields, value), [schema, value])
  const groups = useMemo(() => groupFields(schema.fields), [schema])
  const set = (name: string, v: unknown) => onChange({ ...values, [name]: v })
  return (
    <div className={styles.root}>
      {groups.map((g, gi) => {
        const visible = g.fields.filter(
          (f) =>
            evalWhen(f.when, values, values) &&
            (showComputed || !f.computed) &&
            (!diffOnly ||
              f.required ||
              JSON.stringify(values[f.name]) !== JSON.stringify(fieldDefault(f)))
        )
        if (!visible.length) return null
        const body = (
          <div className={styles.grid}>
            {visible.map((f) => (
              <FieldRenderer
                key={f.name}
                field={f}
                path={f.name}
                value={values[f.name]}
                onChange={(v) => set(f.name, v)}
                root={values}
                errors={errors}
                readOnly={readOnly || f.computed || f.immutable}
                widgets={widgets}
                dense={dense}
                styles={styles}
              />
            ))}
          </div>
        )
        if (!g.group || groups.length === 1) return <div key={g.group || gi}>{body}</div>
        return (
          <CollapsableSection key={g.group} label={g.group} isOpen={gi < 3}>
            {body}
          </CollapsableSection>
        )
      })}
    </div>
  )
}

function FieldRenderer({
  field: f,
  path,
  value,
  onChange,
  root,
  errors,
  readOnly,
  widgets,
  dense,
  styles,
}: {
  field: SchemaField
  path: string
  value: unknown
  onChange: (v: unknown) => void
  root: Values
  errors?: Record<string, string>
  readOnly?: boolean
  widgets?: SchemaFormProps['widgets']
  dense?: boolean
  styles: ReturnType<typeof getStyles>
}) {
  const { t } = useTranslation()
  const kind = fieldKind(f)
  const error = humanError(t, errors?.[path])
  const custom = resolveWidget(widgets, path)
  const label = f.title ?? f.name
  const desc = f.description
  const wide =
    kind === 'object' ||
    kind === 'oneOf' ||
    kind === 'list' ||
    kind === 'map' ||
    kind === 'json' ||
    (kind === 'string' &&
      (f.string?.multiline || (f.string?.maxLen !== undefined && Number(f.string.maxLen) > 256)))
  const wrap = (control: ReactNode, opts: { horizontal?: boolean } = {}) => (
    <div className={cx(wide && styles.full, f.computed && styles.computed)}>
      <Field
        label={f.computed ? `${label} · ${t('common.misc.readOnly')}` : label}
        description={desc}
        required={f.required && !f.computed}
        invalid={!!error}
        error={error}
        htmlFor={path}
        horizontal={opts.horizontal}
        noMargin={dense}
      >
        {control as never}
      </Field>
    </div>
  )
  if (custom) return wrap(custom({ field: f, value, onChange, path, root, readOnly, error }))
  const id = path
  const disabled = readOnly

  // Read-only (reports, spec view): render values as text, never disabled controls.
  if (readOnly && !['object', 'oneOf', 'list', 'map', 'ref'].includes(kind)) {
    let text: ReactNode
    if (value === undefined || value === null || value === '')
      text = <Text color="secondary">{f.nullable ? t('schema.auto') : '—'}</Text>
    else if (kind === 'bool') text = value ? t('common.misc.yes') : t('common.misc.no')
    else if (kind === 'choice')
      text =
        f.choice?.options.find((o) => String(choiceRaw(o.value)) === String(value))?.label ??
        String(value)
    else if (kind === 'json')
      text = (
        <pre className={styles.pre}>
          {typeof value === 'string' ? value : JSON.stringify(value, null, 2)}
        </pre>
      )
    else text = `${String(value)}${f.unit ? ` ${f.unit}` : ''}`
    return wrap(<div className={styles.ro}>{text}</div>)
  }
  if (
    readOnly &&
    kind === 'list' &&
    Array.isArray(value) &&
    (!listItem(f) || !['object', 'oneOf'].includes(fieldKind(listItem(f) as SchemaField)))
  ) {
    return wrap(
      <div className={styles.ro}>
        {(value as unknown[]).length ? (
          (value as unknown[]).map(String).join(', ')
        ) : (
          <Text color="secondary">—</Text>
        )}
      </div>
    )
  }

  switch (kind) {
    case 'bool':
      return wrap(
        <Switch
          id={id}
          value={!!value}
          disabled={disabled}
          onChange={(e) => onChange(e.currentTarget.checked)}
        />
      )
    case 'choice': {
      const options = (f.choice?.options ?? []).map((o) => ({
        label: o.label ?? String(choiceRaw(o.value)),
        value: String(choiceRaw(o.value)),
        description: o.description,
      }))
      const raw = (v: string) => {
        const opt = f.choice?.options.find((o) => String(choiceRaw(o.value)) === v)
        return opt ? choiceRaw(opt.value) : v
      }
      if (options.length <= 4 && options.every((o) => o.label.length <= 24))
        return wrap(
          <RadioButtonGroup
            id={id}
            options={options}
            value={value === undefined ? undefined : String(value)}
            disabled={disabled}
            onChange={(v) => onChange(raw(v))}
          />
        )
      return wrap(
        <Combobox
          id={id}
          options={options}
          value={value === undefined || value === null ? null : String(value)}
          disabled={disabled}
          isClearable={!!f.nullable}
          onChange={(o: { value: string } | null) => onChange(o ? raw(o.value) : null)}
          width={30}
        />
      )
    }
    case 'int64':
    case 'uint64':
    case 'double': {
      const spec = f.int64 ?? f.uint64 ?? f.double
      return wrap(
        <Input
          id={id}
          type="number"
          value={value === undefined || value === null ? '' : String(value)}
          disabled={disabled}
          min={spec?.gte !== undefined ? Number(spec.gte) : undefined}
          max={spec?.lte !== undefined ? Number(spec.lte) : undefined}
          step={kind === 'double' ? 'any' : 1}
          suffix={f.unit ? <span className={styles.unit}>{f.unit}</span> : undefined}
          placeholder={f.nullable ? t('schema.auto') : undefined}
          width={20}
          onChange={(e) =>
            onChange(
              e.currentTarget.value === ''
                ? f.nullable
                  ? null
                  : undefined
                : kind === 'double'
                  ? Number(e.currentTarget.value)
                  : Number.parseInt(e.currentTarget.value, 10)
            )
          }
        />
      )
    }
    case 'duration':
      return wrap(
        <Input
          id={id}
          value={(value as string) ?? ''}
          disabled={disabled}
          placeholder={f.duration?.default ?? '30s'}
          width={16}
          onChange={(e) => onChange(e.currentTarget.value)}
        />
      )
    case 'timestamp':
      return wrap(
        <Input
          id={id}
          type="datetime-local"
          value={value ? new Date(value as string).toISOString().slice(0, 16) : ''}
          disabled={disabled}
          width={24}
          onChange={(e) =>
            onChange(
              e.currentTarget.value ? new Date(e.currentTarget.value).toISOString() : undefined
            )
          }
        />
      )
    case 'json':
      return wrap(
        <TextArea
          id={id}
          rows={6}
          value={
            typeof value === 'string'
              ? value
              : value === undefined
                ? ''
                : JSON.stringify(value, null, 2)
          }
          disabled={disabled}
          onChange={(e) => {
            try {
              onChange(JSON.parse(e.currentTarget.value))
            } catch {
              onChange(e.currentTarget.value)
            }
          }}
        />
      )
    case 'string': {
      if (f.secret)
        return wrap(
          <SecretInput
            id={id}
            value={(value as string) ?? ''}
            isConfigured={false}
            disabled={disabled}
            onReset={() => onChange('')}
            onChange={(e) => onChange(e.currentTarget.value)}
            width={30}
          />
        )
      const example = f.examples?.[0] ? String(choiceRaw(f.examples[0])) : undefined
      if (wide)
        return wrap(
          <TextArea
            id={id}
            rows={4}
            value={(value as string) ?? ''}
            disabled={disabled}
            placeholder={example}
            onChange={(e) => onChange(e.currentTarget.value)}
          />
        )
      return wrap(
        <Input
          id={id}
          value={(value as string) ?? ''}
          disabled={disabled}
          placeholder={example ?? (f.nullable ? t('schema.auto') : undefined)}
          onChange={(e) => onChange(e.currentTarget.value)}
        />
      )
    }
    case 'object': {
      const fields = objectFields(f)
      const v = (value as Values) ?? {}
      return wrap(
        <div className={styles.nested}>
          <div className={styles.grid}>
            {fields
              .filter((c) => evalWhen(c.when, root, v))
              .map((c) => (
                <FieldRenderer
                  key={c.name}
                  field={c}
                  path={`${path}.${c.name}`}
                  value={v[c.name] ?? fieldDefault(c)}
                  onChange={(nv) => onChange({ ...v, [c.name]: nv })}
                  root={root}
                  errors={errors}
                  readOnly={readOnly}
                  widgets={widgets}
                  dense={dense}
                  styles={styles}
                />
              ))}
          </div>
        </div>
      )
    }
    case 'oneOf': {
      if (!f.oneOf) return null
      const disc = f.oneOf.discriminator
      const v = (value as Values) ?? {}
      const current = (v[disc] as string) ?? Object.keys(f.oneOf.variants)[0]
      const variant = f.oneOf.variants[current]
      const options = Object.entries(f.oneOf.variants).map(([k, vr]) => ({
        label: vr.title ?? k,
        value: k,
      }))
      return wrap(
        <div className={styles.nested}>
          <Field label={disc} noMargin={false}>
            <RadioButtonGroup
              options={options}
              value={current}
              disabled={disabled}
              onChange={(k) =>
                onChange({
                  [disc]: k,
                  ...Object.fromEntries(
                    (f.oneOf?.variants[k]?.fields ?? [])
                      .map((c) => [c.name, fieldDefault(c)])
                      .filter(([, d]) => d !== undefined)
                  ),
                })
              }
            />
          </Field>
          <div className={styles.grid}>
            {(variant?.fields ?? [])
              .filter((c) => evalWhen(c.when, root, v))
              .map((c) => (
                <FieldRenderer
                  key={c.name}
                  field={c}
                  path={`${path}.${c.name}`}
                  value={v[c.name] ?? fieldDefault(c)}
                  onChange={(nv) => onChange({ ...v, [c.name]: nv })}
                  root={root}
                  errors={errors}
                  readOnly={readOnly}
                  widgets={widgets}
                  dense={dense}
                  styles={styles}
                />
              ))}
          </div>
        </div>
      )
    }
    case 'list': {
      const item = listItem(f)
      const arr = Array.isArray(value) ? (value as unknown[]) : []
      const itemKind = item ? fieldKind(item) : 'string'
      const max = f.list?.maxItems ? Number(f.list.maxItems) : undefined
      if (item?.choice) {
        // multi-select of a fixed option set → checkboxes
        const options = item.choice.options.map((o) => ({
          label: o.label ?? String(choiceRaw(o.value)),
          raw: choiceRaw(o.value),
        }))
        return wrap(
          <Stack direction="row" wrap="wrap" gap={2}>
            {options.map((o) => (
              <Checkbox
                key={String(o.raw)}
                label={o.label}
                disabled={disabled}
                value={arr.includes(o.raw)}
                onChange={(e) =>
                  onChange(
                    e.currentTarget.checked ? [...arr, o.raw] : arr.filter((x) => x !== o.raw)
                  )
                }
              />
            ))}
          </Stack>
        )
      }
      return wrap(
        <div>
          {arr.map((it, i) => (
            <div key={i} className={styles.row}>
              <div style={{ flex: 1 }}>
                {itemKind === 'object' || itemKind === 'oneOf' ? (
                  <FieldRenderer
                    field={{ ...(item as SchemaField), title: `#${i + 1}`, description: undefined }}
                    path={`${path}[${i}]`}
                    value={it}
                    onChange={(nv) => onChange(arr.map((x, j) => (j === i ? nv : x)))}
                    root={root}
                    errors={errors}
                    readOnly={readOnly}
                    widgets={widgets}
                    dense
                    styles={styles}
                  />
                ) : (
                  <Input
                    value={it === undefined || it === null ? '' : String(it)}
                    disabled={disabled}
                    invalid={!!errors?.[`${path}[${i}]`]}
                    onChange={(e) =>
                      onChange(
                        arr.map((x, j) =>
                          j === i
                            ? itemKind === 'int64' || itemKind === 'double'
                              ? Number(e.currentTarget.value)
                              : e.currentTarget.value
                            : x
                        )
                      )
                    }
                  />
                )}
              </div>
              {!disabled && (
                <IconButton
                  name="trash-alt"
                  tooltip={t('common.actions.remove')}
                  onClick={() => onChange(arr.filter((_, j) => j !== i))}
                />
              )}
            </div>
          ))}
          {!disabled && (max === undefined || arr.length < max) && (
            <Button
              size="sm"
              variant="secondary"
              icon="plus"
              onClick={() =>
                onChange([
                  ...arr,
                  item ? (fieldDefault(item) ?? (itemKind === 'int64' ? 0 : '')) : '',
                ])
              }
            >
              {t('common.actions.add')}
            </Button>
          )}
        </div>
      )
    }
    case 'map': {
      const obj = (value as Record<string, unknown>) ?? {}
      const entries = Object.entries(obj)
      const vf = f.map?.valueField
      const vkind = vf ? fieldKind(vf) : 'string'
      return wrap(
        <div>
          {entries.map(([k, v], i) => (
            <div key={i} className={styles.row}>
              <Input
                value={k}
                disabled={disabled}
                placeholder={t('common.fields.key')}
                width={20}
                onChange={(e) => {
                  const next: Record<string, unknown> = {}
                  for (const [ek, ev] of entries) next[ek === k ? e.currentTarget.value : ek] = ev
                  onChange(next)
                }}
              />
              <div style={{ flex: 1 }}>
                {vkind === 'object' ? (
                  <FieldRenderer
                    field={{ ...(vf as SchemaField), title: k, description: undefined }}
                    path={`${path}.${k}`}
                    value={v}
                    onChange={(nv) => onChange({ ...obj, [k]: nv })}
                    root={root}
                    errors={errors}
                    readOnly={readOnly}
                    widgets={widgets}
                    dense
                    styles={styles}
                  />
                ) : (
                  <Input
                    value={v === undefined || v === null ? '' : String(v)}
                    disabled={disabled}
                    placeholder={t('common.fields.value')}
                    onChange={(e) =>
                      onChange({
                        ...obj,
                        [k]:
                          vkind === 'int64' || vkind === 'double'
                            ? Number(e.currentTarget.value)
                            : e.currentTarget.value,
                      })
                    }
                  />
                )}
              </div>
              {!disabled && (
                <IconButton
                  name="trash-alt"
                  tooltip={t('common.actions.remove')}
                  onClick={() => {
                    const { [k]: _drop, ...rest } = obj
                    onChange(rest)
                  }}
                />
              )}
            </div>
          ))}
          {!disabled && (
            <Button
              size="sm"
              variant="secondary"
              icon="plus"
              onClick={() =>
                onChange({
                  ...obj,
                  [`key${entries.length + 1}`]: vf ? (fieldDefault(vf) ?? '') : '',
                })
              }
            >
              {t('common.actions.add')}
            </Button>
          )}
        </div>
      )
    }
    case 'ref':
      return wrap(
        <Alert severity="info" title={`${label}: ${f.ref?.name}`}>
          <Text color="secondary">{t('schema.refHint')}</Text>
        </Alert>
      )
    default:
      return wrap(
        <Input
          id={id}
          value={value === undefined ? '' : String(value)}
          disabled={disabled}
          onChange={(e) => onChange(e.currentTarget.value)}
        />
      )
  }
}

export function SchemaFieldSet({ label, children }: { label: string; children: ReactNode }) {
  return <FieldSet label={label}>{children}</FieldSet>
}
