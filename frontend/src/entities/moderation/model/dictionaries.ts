import type { AuditAction, ReportReason, ReportStatus } from '@/shared/api/generated/model'

/** Причина жалобы так, как её выбрал пожаловавшийся. */
export const REASON_LABEL: Record<ReportReason, string> = {
  spam: 'Спам или реклама',
  abuse: 'Оскорбления и грубость',
  other: 'Другое',
}

/**
 * Состояние жалобы. `resolved` и `rejected` — не «хорошо» и «плохо», а «нарушение было»
 * и «нарушения нет»: подписи должны говорить именно это, иначе отказ читается как ошибка.
 */
export const STATUS_LABEL: Record<ReportStatus, string> = {
  open: 'В очереди',
  resolved: 'Нарушение подтверждено',
  rejected: 'Нарушения нет',
}

/**
 * Действия в журнале. Записи пишет не только модерация: блокировка и отмена цепочки
 * приходят от действий обычных людей, и в общей ленте они соседствуют с решениями.
 */
export const ACTION_LABEL: Record<AuditAction, string> = {
  REPORT_ASSIGNED: 'жалоба взята в разбор',
  REPORT_RESOLVED: 'нарушение подтверждено',
  REPORT_REJECTED: 'жалоба отклонена',
  USER_BLOCKED: 'пользователь заблокирован',
  CHAIN_CANCELLED: 'цепочка отменена',
  DELIVERY_STATUS_CHANGED: 'статус доставки изменён',
}
