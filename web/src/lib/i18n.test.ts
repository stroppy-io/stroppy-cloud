import { t as grafanaT } from '@grafana/i18n'
import { afterAll, describe, expect, it } from 'vitest'
import { i18n } from './i18n'

describe('grafana built-in strings', () => {
  const initial = i18n.language
  afterAll(() => i18n.changeLanguage(initial))

  it('resolves grafana-ui keys to Russian on the shared instance', () => {
    expect(i18n.t('grafana-ui.select.no-options-label', { lng: 'ru' })).toBe('Ничего не найдено')
    expect(i18n.t('time-picker.absolute.title', { lng: 'ru' })).toBe(
      'Абсолютный временной диапазон'
    )
    expect(i18n.t('refresh-picker.off-option.label', { lng: 'ru' })).toBe('Выкл.')
    expect(i18n.t('clipboard-button.inline-toast.success', { lng: 'ru' })).toBe('Скопировано')
  })

  it('pluralizes grafana-data ranges in Russian', () => {
    // Grafana's ru strings keep a no-break space between the number and the unit.
    const hours = (count: number) =>
      i18n
        .t('grafana-data.datetime.rangeutils.lastNHours', { lng: 'ru', count })
        .replace(/\u00a0/g, ' ')
    expect(hours(1)).toBe('Последний 1 час')
    expect(hours(3)).toBe('Последние 3 часа')
    expect(hours(5)).toBe('Последние 5 часов')
  })

  it('routes @grafana/i18n t() through our instance and follows changeLanguage', async () => {
    await i18n.changeLanguage('ru')
    expect(grafanaT('grafana-ui.modal.close-tooltip', 'Close')).toBe('Закрыть')
    expect(grafanaT('time-picker.range-content.apply-button', 'Apply time range')).toBe(
      'Применить временной диапазон'
    )
    await i18n.changeLanguage('en')
    expect(grafanaT('grafana-ui.modal.close-tooltip', 'Close')).toBe('Close')
  })
})
