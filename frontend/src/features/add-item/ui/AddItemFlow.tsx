import { useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import {
  isAborted,
  recognizeItem,
  type RecognitionStage,
  type RecognizedItem,
} from '@/entities/item'
import { ItemReview, type ItemFormValues } from './ItemReview'
import { PhotoIntro } from './PhotoIntro'
import { RecognitionFailed } from './RecognitionFailed'
import { RecognitionProgress } from './RecognitionProgress'

/**
 * Где человек находится внутри первого шага публикации. Разбор фотографии занимает
 * до полуминуты и на это время забирает экран целиком: показывать его строкой над формой
 * значит предлагать заполнять поля, которые вот-вот заполнятся сами.
 */
type Screen = 'photo' | 'analyzing' | 'failed' | 'review'

interface AddItemFlowProps {
  /** Заполненное ранее — чтобы возврат со второго шага не терял ввод. */
  initial?: ItemFormValues
  onSubmit: (values: ItemFormValues) => void
}

/**
 * Первый шаг публикации: фотография → разбор → сводка. Начинается со съёмки, а не с полей:
 * снимок есть у человека в руках, а описание и категорию по нему предложит модель.
 */
export function AddItemFlow({ initial, onSubmit }: AddItemFlowProps) {
  const [values, setValues] = useState<ItemFormValues>(initial ?? { title: '', category: '' })
  // Возврат со второго шага открывает сводку: фотографировать заново человек не собирался.
  const [screen, setScreen] = useState<Screen>(initial ? 'review' : 'photo')
  const [stage, setStage] = useState<RecognitionStage>('upload')
  const [recognized, setRecognized] = useState<RecognizedItem>()
  const waiting = useRef<AbortController | null>(null)

  const patch = (part: Partial<ItemFormValues>) => setValues((prev) => ({ ...prev, ...part }))

  const recognition = useMutation({
    mutationFn: (file: File) => {
      waiting.current = new AbortController()
      return recognizeItem(file, { onStage: setStage, signal: waiting.current.signal })
    },
    onSuccess: (result) => {
      // Фото уже в хранилище бэкенда — при публикации его не надо грузить второй раз.
      if (result.imageUrl) patch({ uploadedUrl: result.imageUrl })
      setRecognized(result)
      setScreen('review')
    },
    // Отмена — не отказ модели: человек сам решил не ждать и заполнит поля руками.
    onError: (error) => setScreen(isAborted(error) ? 'review' : 'failed'),
  })

  /** Новое фото — новый разбор: ради этого распознавание и нужно. */
  const analyze = (file: File) => {
    // Ссылку на прежнее фото освобождаем, иначе замена фото копит их в памяти.
    if (values.photoUrl) URL.revokeObjectURL(values.photoUrl)

    // Сам файл несём дальше: предпросмотр рисуется по `blob:`-ссылке, а бэкенду нужен файл.
    // Прежний адрес в хранилище сбрасываем — он от старого снимка.
    patch({ photoUrl: URL.createObjectURL(file), photoFile: file, uploadedUrl: undefined })
    setRecognized(undefined)
    setStage('upload')
    setScreen('analyzing')
    recognition.mutate(file)
  }

  if (screen === 'analyzing') {
    return (
      <RecognitionProgress
        stage={stage}
        onCancel={() => {
          waiting.current?.abort()
          setScreen('review')
        }}
      />
    )
  }

  if (screen === 'failed') {
    return (
      <RecognitionFailed
        error={recognition.error}
        onRetry={() => setScreen('photo')}
        onManual={() => setScreen('review')}
      />
    )
  }

  if (screen === 'photo') {
    return <PhotoIntro onPick={analyze} onSkip={() => setScreen('review')} />
  }

  return (
    <ItemReview
      values={values}
      patch={patch}
      recognized={recognized}
      onRecognizedUsed={() => setRecognized(undefined)}
      onPhoto={analyze}
      onSubmit={() =>
        onSubmit({ ...values, title: values.title.trim(), description: values.description?.trim() })
      }
    />
  )
}
