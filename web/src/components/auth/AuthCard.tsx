import { Page } from '@app/Page'
import { LangSwitch } from '@components/auth/LoginPage'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Stack, Text, useStyles2 } from '@grafana/ui'
import type { ReactNode } from 'react'

const getStyles = (theme: GrafanaTheme2) => ({
  card: css({
    width: 380,
    margin: `${theme.spacing(10)} auto 0`,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(3),
  }),
})

/** The frame of every bare auth screen: language switch + a centred card with a title. */
export function AuthCard({ title, children }: { title: string; children: ReactNode }) {
  const styles = useStyles2(getStyles)
  return (
    <Page>
      <LangSwitch />
      <div className={styles.card}>
        <Stack direction="column" gap={2}>
          <Text variant="h4" weight="medium">
            {title}
          </Text>
          {children}
        </Stack>
      </div>
    </Page>
  )
}
