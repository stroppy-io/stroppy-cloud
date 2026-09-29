import { TenantCreatePage } from '@components/tenants/TenantCreatePage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/tenants/new/')({ component: TenantCreatePage })
