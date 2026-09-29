import { Dropdown, Menu, ToolbarButton } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

// Header tenant dropdown: explicit trigger (building icon · name · chevron), menu with all
// memberships (check on the current one, slug · role as description) and "create tenant".
export function TenantSwitcher() {
  const { t } = useTranslation()
  const { slug, me, tenant } = useTenant()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  return (
    <Dropdown
      placement="bottom-end"
      onVisibleChange={setOpen}
      overlay={
        <Menu ariaLabel={t('nav.switchTenant')}>
          <Menu.Group label={t('nav.tenants')}>
            {me.tenants.map((m) => (
              <Menu.Item
                key={m.tenant.id}
                label={m.tenant.name}
                description={`${m.tenant.slug} · ${t(`common.role.${m.role}`)}`}
                icon={m.tenant.slug === slug ? 'check' : 'building'}
                ariaChecked={m.tenant.slug === slug}
                onClick={() => void navigate({ to: '/t/$slug', params: { slug: m.tenant.slug } })}
              />
            ))}
          </Menu.Group>
          <Menu.Divider />
          <Menu.Item
            label={t('nav.createTenant')}
            icon="plus"
            onClick={() => void navigate({ to: '/tenants/new' as never })}
          />
        </Menu>
      }
    >
      <ToolbarButton icon="building" isOpen={open} aria-label={t('nav.switchTenant')}>
        {tenant?.name ?? t('nav.noTenant')}
      </ToolbarButton>
    </Dropdown>
  )
}
