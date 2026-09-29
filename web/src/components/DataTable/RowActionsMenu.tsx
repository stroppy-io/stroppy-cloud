import { css } from '@emotion/css'
import type { GrafanaTheme2, IconName } from '@grafana/data'
import { Dropdown, IconButton, Menu, Text, useStyles2 } from '@grafana/ui'
import { Fragment, useId } from 'react'
import { useTranslation } from 'react-i18next'
import { useOverlayRegistry } from './overlay'

export interface RowAction {
  key: string
  label: string
  icon?: IconName
  description?: string
  // Invalid actions stay visible but disabled, with the reason as the item description.
  disabled?: boolean
  disabledReason?: string
  destructive?: boolean
  // Starts a new visual group (divider before this item).
  group?: boolean
  onClick: () => void
}

const getStyles = (theme: GrafanaTheme2) => ({
  header: css({
    padding: theme.spacing(0.5, 1),
    maxWidth: 280,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    borderBottom: `1px solid ${theme.colors.border.weak}`,
  }),
  wrap: css({ display: 'inline-flex' }),
})

// Kebab menu of a table row. Renders EVERY action for the entity; invalid ones are disabled with
// a reason rather than hidden. Reports open state to the table so auto-refresh pauses.
export function RowActionsMenu({ title, actions }: { title: string; actions: RowAction[] }) {
  const styles = useStyles2(getStyles)
  const { t } = useTranslation()
  const registry = useOverlayRegistry()
  const id = useId()
  const menu = (
    <Menu
      header={
        <div className={styles.header}>
          <Text variant="bodySmall" color="secondary">
            {title}
          </Text>
        </div>
      }
    >
      {actions.map((a) => (
        <Fragment key={a.key}>
          {a.group && <Menu.Divider />}
          <Menu.Item
            label={a.label}
            icon={a.icon}
            description={a.disabled ? (a.disabledReason ?? a.description) : a.description}
            disabled={a.disabled}
            destructive={a.destructive}
            onClick={(e) => {
              e.stopPropagation()
              a.onClick()
            }}
          />
        </Fragment>
      ))}
    </Menu>
  )
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: the click barrier keeps row navigation off
    <span
      className={styles.wrap}
      data-no-row-click
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <Dropdown
        overlay={menu}
        placement="bottom-end"
        onVisibleChange={(open) => registry?.setOpen(id, open)}
      >
        <IconButton name="ellipsis-v" tooltip={t('common.actions.more')} />
      </Dropdown>
    </span>
  )
}
