import { unwrap } from '@/shared/api/fetcher'
import { onVisionResult, whenStreamConnected } from '@/shared/api/events'
import { analyzePhoto, uploadMedia } from '@/shared/api/generated/endpoints'
import type {
  MediaUpload,
  VisionAnalysisAccepted,
  VisionAnalysisResponse,
  VisualQuality,
} from '@/shared/api/generated/model'
import { isBackendConnected } from '@/shared/config/backend'
import { recognize as recognizeMock } from './itemMocks'
import type { ItemCondition, RecognizeOptions, RecognizedItem } from '../model/types'

/**
 * Пять градаций модели против трёх наших. `EXCELLENT` и `GOOD` для человека одно и то же
 * «хорошее»: разницу между «почти новое» и «следы использования» по фото не проверить,
 * а в обмене состояние — предмет разговора, а не точная величина.
 */
const CONDITION: Record<VisualQuality, ItemCondition> = {
  NEW: 'new',
  EXCELLENT: 'good',
  GOOD: 'good',
  FAIR: 'used',
  POOR: 'used',
}

/** Сколько ждём ответ модели. Дольше — человек уже заполнил форму руками. */
const ANALYSIS_TIMEOUT_MS = 30_000

const fromVision = (result: VisionAnalysisResponse): RecognizedItem => ({
  category: result.suggestedCategory,
  condition: CONDITION[result.visualQuality],
  description: result.marketplaceDescription,
  imageUrl: result.imageUrl,
})

/**
 * Распознавание вещи по фотографии — второй вход в сервис, публикация нового объявления:
 * там фото загружают с нуля и поля пустые. У главного входа (обмен у уже размещённого
 * объявления) распознавать нечего — название и категория там уже заполнены.
 *
 * Анализ асинхронный: `POST /vision/analyze` только ставит задачу, а результат приходит
 * событием `vision.analysis.completed` в общий поток. Поэтому сначала дожидаемся, что поток
 * установлен, и только потом запускаем разбор — иначе ответ придёт в никуда, а повторить
 * его сервер не сможет: реплея у потока нет.
 *
 * Ответ — подсказка, а не решение за человека: любое поле в форме можно переписать.
 *
 * О каждом шаге сообщаем наружу: ожидание доходит до полуминуты, и экран всё это время
 * должен говорить, что происходит, а не крутить безымянный индикатор.
 */
export async function recognizeItem(
  file: File,
  { onStage, signal }: RecognizeOptions = {},
): Promise<RecognizedItem> {
  if (!isBackendConnected) return recognizeMock(file, { onStage, signal })

  onStage?.('upload')
  const uploaded = unwrap<MediaUpload>(await uploadMedia({ file }))

  onStage?.('connect')
  const streaming = await whenStreamConnected()
  if (!streaming) throw new Error('Нет связи с сервером — распознавание недоступно')

  onStage?.('analyze')

  // Подписываемся до запроса: анализ короткого фото может завершиться раньше, чем вернётся
  // ответ «принято», и событие тогда пришло бы в пустоту.
  const analysis = waitForResult(uploaded.url, signal)

  unwrap<VisionAnalysisAccepted>(await analyzePhoto({ imageUrl: uploaded.url }))

  return fromVision(await analysis)
}

/** Отменённое ожидание — не сбой: экран просто переходит к ручному заполнению. */
export const isAborted = (error: unknown) => error instanceof Error && error.name === 'AbortError'

/**
 * Ждём событие именно про нашу фотографию: человек мог заменить фото, не дождавшись ответа,
 * и в потоке окажутся два результата подряд — чужой отбрасываем по адресу.
 */
function waitForResult(imageUrl: string, signal?: AbortSignal): Promise<VisionAnalysisResponse> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      done()
      reject(new Error('Распознавание не ответило — заполните поля сами'))
    }, ANALYSIS_TIMEOUT_MS)

    const stop = onVisionResult((result) => {
      if (result instanceof Error) {
        done()
        reject(result)
        return
      }

      if (result.imageUrl !== imageUrl) return

      done()
      resolve(result)
    })

    const cancel = () => {
      done()
      reject(new DOMException('Распознавание отменено', 'AbortError'))
    }

    /** Ждать больше нечего: снимаем таймер, подписку и слушателя отмены разом. */
    function done() {
      clearTimeout(timer)
      stop()
      signal?.removeEventListener('abort', cancel)
    }

    signal?.addEventListener('abort', cancel, { once: true })
  })
}
