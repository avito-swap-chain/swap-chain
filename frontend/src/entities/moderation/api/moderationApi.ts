import { unwrap } from '@/shared/api/fetcher'
import {
  assignReport,
  decideReport,
  getAdminFunnelMetrics,
  getAdminReport,
  listAdminAudit,
  listAdminReports,
} from '@/shared/api/generated/endpoints'
import type {
  AuditLogList,
  FunnelMetrics,
  ListAdminReportsParams,
  MessageReport,
  MessageReportList,
  ReportDecision,
  ReportDetail,
} from '@/shared/api/generated/model'

/**
 * Ключи кэша. Очередь зависит от фильтров — иначе переключение «все / без разбирающего»
 * показывало бы прошлый ответ, пока грузится новый.
 */
export const moderationKeys = {
  all: ['moderation'] as const,
  reports: (params: ListAdminReportsParams) => [...moderationKeys.all, 'reports', params] as const,
  report: (id: number) => [...moderationKeys.all, 'report', id] as const,
  audit: () => [...moderationKeys.all, 'audit'] as const,
  funnel: () => [...moderationKeys.all, 'funnel'] as const,
}

/**
 * Снимок воронки: сколько вещей вообще доходит до цепочки, сколько предложений принимают
 * и сколько обменов доводят до конца. Это и есть ответ на вопрос «работает ли идея»,
 * а не украшение админки: длина цепочки в три участника выбрана ради процента совпадений,
 * и проверять его надо цифрами, а не ощущением.
 */
export async function getFunnel(): Promise<FunnelMetrics> {
  return unwrap<FunnelMetrics>(await getAdminFunnelMetrics())
}

/**
 * Мок-режима у модерации нет — по той же причине, что и у доставок ПВЗ: роль `ADMIN` берётся
 * из сессии, а на моках «я» это выбранная персона. Раздел открывается только с бэкендом.
 *
 * Курсор ответа не используем: разбирают текущую очередь, а не листают историю жалоб.
 */
export async function getReports(params: ListAdminReportsParams): Promise<MessageReport[]> {
  const { reports } = unwrap<MessageReportList>(await listAdminReports(params))
  return reports
}

/** Жалоба целиком: сама она, реплика, на которую пожаловались, и вся нить вокруг неё. */
export async function getReport(id: number): Promise<ReportDetail> {
  return unwrap<ReportDetail>(await getAdminReport(id))
}

/**
 * Взять жалобу себе. Повтор для того же администратора бэкенд принимает молча, а занятую
 * кем-то другим отдаёт 409 — двое не разберут одну жалобу дважды.
 */
export async function takeReport(id: number): Promise<MessageReport> {
  return unwrap<MessageReport>(await assignReport(id))
}

/**
 * Решение по жалобе. Оно терминальное и однократное: переписать его нельзя, поэтому
 * комментарий обязателен — в аудите остаётся именно он, а не «решено».
 */
export async function decide(
  id: number,
  decision: ReportDecision,
  comment: string,
): Promise<MessageReport> {
  return unwrap<MessageReport>(await decideReport(id, { decision, comment: comment.trim() }))
}

/** Журнал административных действий. Дописываемый и неизменяемый, новые записи сверху. */
export async function getAudit(): Promise<AuditLogList['entries']> {
  const { entries } = unwrap<AuditLogList>(await listAdminAudit({ limit: 50 }))
  return entries
}
