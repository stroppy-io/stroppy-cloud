import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { Link, type LinkProps } from '@tanstack/react-router'
import type { ReactNode } from 'react'

const getStyles = (theme: GrafanaTheme2) => ({
  link: css({
    color: theme.colors.text.link,
    textDecoration: 'none',
    '&:hover': { textDecoration: 'underline' },
    '&:focus-visible': {
      outline: `2px solid ${theme.colors.primary.border}`,
      outlineOffset: 2,
      borderRadius: theme.shape.radius.default,
    },
  }),
  plain: css({
    color: 'inherit',
    textDecoration: 'none',
    '&:hover': { color: theme.colors.text.link },
    '&:focus-visible': {
      outline: `2px solid ${theme.colors.primary.border}`,
      outlineOffset: 2,
      borderRadius: theme.shape.radius.default,
    },
  }),
})

// Typed router link in Grafana text-link styling. `plain` inherits the surrounding text color
// (rows, cards) and only colors on hover.
export function AppLink(
  props: LinkProps & { plain?: boolean; className?: string; children?: ReactNode }
) {
  const { plain, className, ...rest } = props
  const styles = useStyles2(getStyles)
  return <Link {...rest} className={cx(plain ? styles.plain : styles.link, className)} />
}
