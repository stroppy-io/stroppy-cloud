import type { UserRef } from '@api/types'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { useStyles2 } from '@grafana/ui'
import { identicon } from '@helpers/identicon'
import { useState } from 'react'

const getStyles = (theme: GrafanaTheme2) => ({
  image: css({
    display: 'inline-block',
    flexShrink: 0,
    borderRadius: theme.shape.radius.circle,
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    objectFit: 'cover',
    verticalAlign: 'middle',
  }),
  label: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.75) }),
})

// Stable per account, matching the identicon behavior of main-v0 without a remote image service.
export function UserAvatar({ user, size = 24 }: { user: UserRef; size?: number }) {
  const styles = useStyles2(getStyles)
  const [failedUrl, setFailedUrl] = useState<string>()
  const url = user.avatar && /^https?:\/\//.test(user.avatar) ? user.avatar : undefined
  const src = url && url !== failedUrl ? url : identicon(user.id)
  return (
    <img
      className={styles.image}
      src={src}
      alt=""
      width={size}
      height={size}
      onError={url && src === url ? () => setFailedUrl(url) : undefined}
    />
  )
}

export function UserLabel({ user }: { user: UserRef }) {
  const styles = useStyles2(getStyles)
  return (
    <span className={styles.label}>
      <UserAvatar user={user} size={18} />
      <span>{user.display_name ?? user.id}</span>
    </span>
  )
}
