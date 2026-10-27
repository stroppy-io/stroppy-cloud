import type { SharedRun } from '@api/types'
import { ReportSection } from '@components/report/ReportPrimitives'
import { RunSetup } from '@components/report/RunSetup'
import { topologyText } from '@helpers/topology'
import { useTranslation } from 'react-i18next'

export function ReportSetup({
  run,
  index,
  tabbed,
}: {
  run: SharedRun
  index: number
  tabbed?: boolean
}) {
  const { t } = useTranslation()
  const db = run.database
  return (
    <ReportSection
      id="setup"
      index={index}
      tabbed={tabbed}
      title={t('public.setup.title')}
      hint={t('public.setup.hint')}
    >
      <RunSetup
        kind={db?.kind ?? run.summary.db_kind}
        version={db?.version ?? run.summary.db_version}
        image={db?.image}
        params={db?.params as Record<string, unknown> | undefined}
        topology={topologyText(run.summary.topology)}
        provider={run.summary.provider_kind}
        machines={run.machines}
        tuning={run.tuning}
        segments={run.workload?.segments}
      />
    </ReportSection>
  )
}
