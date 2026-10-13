import { Page } from '@app/Page'
import { css } from '@emotion/css'
import type { GrafanaTheme2 } from '@grafana/data'
import { Button, Stack, Text, useStyles2 } from '@grafana/ui'
import { type Lang, setLang } from '@lib/i18n'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

const getStyles = (theme: GrafanaTheme2) => ({
  lang: css({
    position: 'absolute',
    top: theme.spacing(2),
    right: theme.spacing(2),
    display: 'flex',
    gap: theme.spacing(0.5),
  }),
  card: css({
    width: 380,
    margin: `${theme.spacing(10)} auto 0`,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    padding: theme.spacing(3),
  }),
})

/** RU/EN switch available before the profile (and its preference) exists. */
export function LangSwitch() {
  const { i18n } = useTranslation()
  const styles = useStyles2(getStyles)
  return (
    <div className={styles.lang}>
      {(['ru', 'en'] as Lang[]).map((l) => (
        <Button
          key={l}
          size="sm"
          variant={i18n.language.startsWith(l) ? 'primary' : 'secondary'}
          fill="text"
          onClick={() => setLang(l)}
        >
          {l === 'ru' ? 'Русский' : 'English'}
        </Button>
      ))}
    </div>
  )
}

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
