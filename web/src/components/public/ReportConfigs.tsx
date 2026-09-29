import type { SharedRun } from '@api/types'
import { CollapsableSection, Text } from '@grafana/ui'
import { useTranslation } from 'react-i18next'
import { Pre, ReportSection, SubTitle, useReportStyles } from './ReportPrimitives'

export function ReportConfigs({ run, index }: { run: SharedRun; index: number }) {
  const { t } = useTranslation()
  const rs = useReportStyles()
  const roles = Object.entries(run.configs ?? {})
  return (
    <ReportSection
      id="configs"
      index={index}
      title={t('public.configs.title')}
      hint={t('public.configs.hint')}
    >
      {roles.length === 0 ? (
        <span className={rs.muted}>{t('public.configs.empty')}</span>
      ) : (
        roles.map(([role, files]) => (
          <div key={role}>
            <SubTitle>{role}</SubTitle>
            {Object.entries(files).map(([file, text]) => (
              <CollapsableSection
                key={file}
                label={
                  <Text variant="body" weight="medium">
                    {file}
                  </Text>
                }
                isOpen
              >
                <Pre>{text}</Pre>
              </CollapsableSection>
            ))}
          </div>
        ))
      )}
    </ReportSection>
  )
}
