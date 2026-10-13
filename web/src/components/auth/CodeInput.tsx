import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'

// The six-digit code from the mail, as one large field: digits only, pasted
// codes are cleaned, and the form is submitted by itself once all six are in.

export const CODE_LENGTH = 6

const getStyles = (theme: GrafanaTheme2) => ({
  input: css({
    width: '100%',
    height: 64,
    padding: theme.spacing(0, 2),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.medium}`,
    background: theme.components.input.background,
    color: theme.colors.text.primary,
    fontFamily: theme.typography.fontFamilyMonospace,
    fontSize: 30,
    letterSpacing: '0.6em',
    textAlign: 'center',
    textIndent: '0.6em',
    '&::placeholder': { color: theme.colors.text.disabled },
    '&:focus': {
      outline: `2px solid ${theme.colors.primary.main}`,
      outlineOffset: -1,
      borderColor: theme.colors.primary.border,
    },
  }),
  invalid: css({ borderColor: theme.colors.error.border }),
})

export function CodeInput({
  id,
  value,
  onChange,
  onBlur,
  onComplete,
  invalid,
  autoFocus,
}: {
  id: string
  value: string
  onChange: (v: string) => void
  onBlur?: () => void
  onComplete?: (v: string) => void
  invalid?: boolean
  autoFocus?: boolean
}) {
  const styles = useStyles2(getStyles)
  return (
    <input
      id={id}
      className={cx(styles.input, invalid && styles.invalid)}
      // biome-ignore lint/a11y/noAutofocus: the code is the only thing on the screen
      autoFocus={autoFocus}
      inputMode="numeric"
      autoComplete="one-time-code"
      maxLength={CODE_LENGTH}
      placeholder={'•'.repeat(CODE_LENGTH)}
      aria-invalid={invalid}
      value={value}
      onBlur={onBlur}
      onChange={(e) => {
        const digits = e.currentTarget.value.replace(/\D/g, '').slice(0, CODE_LENGTH)
        onChange(digits)
        if (digits.length === CODE_LENGTH) onComplete?.(digits)
      }}
    />
  )
}
