import { css } from '@emotion/css'
import { Combobox, type ComboboxOption, InlineField, useStyles2 } from '@grafana/ui'

// One filter of the rating board: a label and a single choice that always
// reads in full in the field — "All" included. Grafana's multi-select squeezes
// its chips to "Post…" and hides the rest behind a counter, so a filter picks
// one value; a link that carries several (older ones) shows them joined.

const ALL = '__all'

/** Field width in Grafana units (8px) that fits the text and the chevron. */
const fitWidth = (text: string, min: number, max: number) =>
  Math.min(max, Math.max(min, Math.ceil(text.length * 0.95) + 5))

const getStyles = () => ({
  // The row's gap spaces the fields; InlineField's own margin would double it.
  field: css({ margin: 0 }),
})

export function RatingFilter({
  id,
  label,
  allLabel,
  options,
  values,
  onChange,
  loading,
  maxWidth = 32,
}: {
  id: string
  label: string
  /** The choice that drops the filter. */
  allLabel: string
  options: ComboboxOption<string>[]
  /** Selected values; none = all. */
  values: readonly string[] | undefined
  onChange: (values: string[] | undefined) => void
  loading?: boolean
  maxWidth?: number
}) {
  const styles = useStyles2(getStyles)
  const all = { label: allLabel, value: ALL }
  const labelOf = (v: string) => options.find((o) => o.value === v)?.label ?? v
  const value: ComboboxOption<string> = !values?.length
    ? all
    : values.length === 1
      ? { label: labelOf(values[0]), value: values[0] }
      : { label: values.map(labelOf).join(', '), value: values.join(',') }
  return (
    <InlineField label={label} htmlFor={id} className={styles.field}>
      <Combobox
        id={id}
        width={fitWidth(value.label ?? '', 12, maxWidth)}
        loading={loading}
        options={[all, ...options]}
        value={value}
        onChange={(o) => onChange(o.value === ALL ? undefined : [o.value])}
      />
    </InlineField>
  )
}
