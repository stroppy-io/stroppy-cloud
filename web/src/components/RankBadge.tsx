import { css, cx } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { type IconSize, useStyles2 } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { ProductIcon } from './ProductIcon'

// A place in a rating: the first three get a trophy or a medal in gold,
// silver and bronze next to the number, the rest just the number.

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    fontFamily: theme.typography.fontFamilyMonospace,
    fontVariantNumeric: 'tabular-nums',
    color: theme.colors.text.secondary,
  }),
  gold: css({ color: theme.visualization.getColorByName('yellow'), fontWeight: 600 }),
  silver: css({ color: theme.colors.text.primary, fontWeight: 600 }),
  bronze: css({ color: theme.visualization.getColorByName('orange'), fontWeight: 600 }),
})

const PODIUM = ['gold', 'silver', 'bronze'] as const

export function RankBadge({
  rank,
  size = 'sm',
  className,
}: {
  rank: number
  size?: IconSize
  className?: string
}) {
  const { t } = useTranslation()
  const styles = useStyles2(getStyles)
  const place = PODIUM[rank - 1]
  return (
    <span className={cx(styles.root, place && styles[place], className)}>
      {place && (
        <ProductIcon
          name={rank === 1 ? 'trophy' : 'medal'}
          size={size}
          title={t(`common.rank.${place}`)}
        />
      )}
      {rank}
    </span>
  )
}
