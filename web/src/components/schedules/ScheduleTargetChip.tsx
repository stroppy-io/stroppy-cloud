import type { Schedule } from '@api/types'
import { AppLink } from '@app/AppLink'
import { Badge, Stack } from '@grafana/ui'
import { useTenant } from '@hooks/useTenant'
import { useTranslation } from 'react-i18next'

// `test | suite` kind badge + link to the target.
export function ScheduleTargetChip({ target }: { target: Schedule['target'] }) {
  const { t } = useTranslation()
  const { slug } = useTenant()
  const isTest = target.kind === 'test'
  return (
    <Stack alignItems="center" gap={1}>
      <Badge
        text={t(`schedules.target.${target.kind}`)}
        color={isTest ? 'purple' : 'blue'}
        icon={isTest ? 'vial' : 'layer-group'}
      />
      {isTest ? (
        <AppLink to="/t/$slug/library/tests/$id" params={{ slug, id: target.id }} plain>
          {target.name ?? target.id}
        </AppLink>
      ) : (
        <AppLink to="/t/$slug/suites/$id" params={{ slug, id: target.id }} plain>
          {target.name ?? target.id}
        </AppLink>
      )}
    </Stack>
  )
}
