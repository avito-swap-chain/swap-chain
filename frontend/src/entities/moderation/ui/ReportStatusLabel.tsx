import type { ReportStatus } from '@/shared/api/generated/model'
import { Status, type StatusTone } from '@/shared/ui'
import { STATUS_LABEL } from '../model/dictionaries'

// Оранжевый — там, где ход за разбирающим. Решённая жалоба уже никуда не движется,
// и цветом её выделять незачем: в очереди важно видеть то, что ещё ждёт.
const TONES: Record<ReportStatus, StatusTone> = {
  open: 'attention',
  resolved: 'muted',
  rejected: 'muted',
}

export function ReportStatusLabel({ status }: { status: ReportStatus }) {
  return <Status tone={TONES[status]}>{STATUS_LABEL[status]}</Status>
}
