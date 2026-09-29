import { ProfilePage } from '@components/me/ProfilePage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/me/profile')({ component: ProfilePage })
