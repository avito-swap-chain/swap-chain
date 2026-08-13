import type { ReportReason } from './types'

/**
 * Причины жалобы. Список закрытый и короткий: свободный текст читают люди, а причину —
 * счётчик, и десять почти одинаковых пунктов ничего не считают. Формулировки — про обмен,
 * а не про продажу: у нас нет цены, зато есть вещь, которую отдают незнакомцу через ПВЗ.
 */
export const REPORT_REASON_LABEL: Record<ReportReason, string> = {
  item: 'Вещь не такая, как в объявлении',
  noshow: 'Не сдал вещь в пункт выдачи',
  rude: 'Грубость или оскорбления в переписке',
  fraud: 'Похоже на мошенничество',
  other: 'Другое',
}

export const REPORT_REASONS = Object.keys(REPORT_REASON_LABEL) as ReportReason[]
