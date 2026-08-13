import { useRef, useState, type ChangeEvent, type SubmitEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  categoryKeys,
  CONDITIONS,
  CONDITION_LABEL,
  DescriptionField,
  getCategories,
  matchCategory,
  type ItemCondition,
  type ItemDraft,
  type RecognizedItem,
} from '@/entities/item'
import { Banner, Button, IconSparkle, Input, Select } from '@/shared/ui'
import { recognitionPatch } from '../lib/recognitionPatch'
import { ReviewBlock } from './ReviewBlock'

/** Первый шаг публикации: сама вещь, без желания. */
export type ItemFormValues = Omit<ItemDraft, 'wish'>

/**
 * Подписи состояния собраны для строки внутри предложения — «Спорт и отдых · хорошее».
 * Отдельным значением блока то же слово нужно с заглавной.
 */
const capitalize = (text: string) => text[0].toUpperCase() + text.slice(1)

interface ItemReviewProps {
  values: ItemFormValues
  patch: (part: Partial<ItemFormValues>) => void
  /** Что предложила модель. Пропадает, как только предложение приняли. */
  recognized?: RecognizedItem
  onRecognizedUsed: () => void
  /** Другое фото — повод разобрать заново: этим распознавание и полезно. */
  onPhoto: (file: File) => void
  onSubmit: () => void
}

/**
 * Сводка объявления перед публикацией: что записано в каждом поле и карандаш рядом.
 * Не форма из пяти полей подряд — после разбора фотографии заполнено уже почти всё,
 * и человеку нужно проверить и поправить, а не заполнять с нуля.
 */
export function ItemReview({
  values,
  patch,
  recognized,
  onRecognizedUsed,
  onPhoto,
  onSubmit,
}: ItemReviewProps) {
  // Справочник категорий — с бэкенда: по нему же идёт подбор, и свой список тут завести
  // значит выбирать из одного, а искать по другому.
  const { data: categories = [] } = useQuery({
    queryKey: categoryKeys.list(),
    queryFn: getCategories,
  })

  // Названия у модели нет, и на пустом объявлении первым делом нужно оно — открываем сразу.
  const [open, setOpen] = useState<string | null>(values.title ? null : 'Название')
  const photoInput = useRef<HTMLInputElement>(null)

  const toggle = (block: string) => setOpen((prev) => (prev === block ? null : block))

  /** Принять предложение модели: заполняются пустые поля, набранное остаётся как есть. */
  const applyRecognized = () => {
    const suggested = matchCategory(recognized?.category, categories)

    patch({
      ...recognitionPatch(recognized ?? {}, values),
      // Категория подставляется только вместе с идентификатором справочника: одно название
      // без пары в подбор не уедет, а в сводке выглядело бы заполненным полем.
      ...(suggested && !values.categoryId
        ? { category: suggested.name, categoryId: suggested.id }
        : {}),
    })

    onRecognizedUsed()
  }

  const pickPhoto = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) onPhoto(file)
  }

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    onSubmit()
  }

  // Чего не хватает для подбора — в родительном падеже, потому что дальше «не хватает».
  // Название сюда не входит: без него объявление вообще не опубликовать, и говорит
  // об этом заблокированная кнопка, а не совет в баннере.
  const missing = [
    !values.description?.trim() && 'описания',
    !values.categoryId && 'категории',
  ].filter((item): item is string => Boolean(item))

  return (
    <form onSubmit={submit} className="flex flex-1 flex-col gap-4">
      {recognized ? (
        <RecognizedCard recognized={recognized} onApply={applyRecognized} />
      ) : (
        <Banner tone={missing.length > 0 ? 'attention' : 'ok'}>
          {missing.length > 0 ? (
            <>
              <b className="font-bold">Расскажите о вещи чуть больше.</b> Не хватает{' '}
              {missing.join(' и ')} — по ним сервис и ищет обмен.
            </>
          ) : (
            <>
              <b className="font-bold">Всё на месте.</b> Осталось сказать, что хотите взамен.
            </>
          )}
        </Banner>
      )}

      <section className="flex flex-col gap-2 border-b border-line-2 pb-4">
        <h3 className="text-[15px] font-bold">Фото</h3>

        <div className="flex items-center gap-3">
          {values.photoUrl ? (
            <img src={values.photoUrl} alt="" className="size-20 rounded-xl object-cover" />
          ) : (
            <div className="size-20 rounded-xl border-[1.5px] border-dashed border-line" />
          )}

          <Button variant="ghost" onClick={() => photoInput.current?.click()}>
            {values.photoUrl ? 'Заменить фото' : 'Добавить фото'}
          </Button>
        </div>

        <input
          ref={photoInput}
          type="file"
          accept="image/*"
          className="sr-only"
          onChange={pickPhoto}
          aria-label="Фотография вещи"
        />
      </section>

      <ReviewBlock
        label="Название"
        value={values.title}
        missing="Без названия объявление не опубликовать"
        required
        open={open === 'Название'}
        onToggle={() => toggle('Название')}
      >
        <Input
          value={values.title}
          onChange={(event) => patch({ title: event.target.value })}
          placeholder="Например, горный велосипед"
          aria-label="Название"
          required
        />
      </ReviewBlock>

      <ReviewBlock
        label="Категория"
        value={values.category}
        missing="Не выбрана — с ней обмен находится точнее"
        open={open === 'Категория'}
        onToggle={() => toggle('Категория')}
      >
        <Select
          value={values.categoryId ?? ''}
          onChange={(event) => {
            const chosen = categories.find((item) => String(item.id) === event.target.value)
            patch({ category: chosen?.name ?? '', categoryId: chosen?.id })
          }}
          aria-label="Категория"
          disabled={categories.length === 0}
        >
          <option value="" disabled>
            Выберите категорию
          </option>
          {categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </Select>
      </ReviewBlock>

      <ReviewBlock
        label="Состояние"
        value={values.condition && capitalize(CONDITION_LABEL[values.condition])}
        missing="Не указано"
        open={open === 'Состояние'}
        onToggle={() => toggle('Состояние')}
      >
        <Select
          value={values.condition ?? ''}
          onChange={(event) =>
            patch({ condition: (event.target.value || undefined) as ItemCondition })
          }
          aria-label="Состояние"
        >
          {/* Состояние не проставляется само: в обмене оно и есть предмет разговора,
              и «хорошее» по умолчанию было бы обещанием, которого никто не давал. */}
          <option value="">Не указано</option>
          {CONDITIONS.map((condition) => (
            <option key={condition} value={condition}>
              {CONDITION_LABEL[condition]}
            </option>
          ))}
        </Select>
      </ReviewBlock>

      <ReviewBlock
        label="Описание"
        value={values.description}
        missing="Пусто — по описанию сервис и ищет обмен"
        open={open === 'Описание'}
        onToggle={() => toggle('Описание')}
      >
        {/* Подпись у блока уже есть — второй заголовок над полем был бы повтором. */}
        <DescriptionField
          label=""
          value={values.description ?? ''}
          onChange={(description) => patch({ description })}
        />
      </ReviewBlock>

      <div className="mt-auto pt-2">
        <Button type="submit" fullWidth disabled={!values.title.trim()}>
          Далее: что хотите взамен
        </Button>
      </div>
    </form>
  )
}

/**
 * Предложение модели. Результат не переписывает поля сам: разбор фотографии — подсказка,
 * а объявление остаётся тем, что человек о вещи сказал. Поэтому карточка и кнопка,
 * а не молча заполненная сводка.
 */
function RecognizedCard({
  recognized,
  onApply,
}: {
  recognized: RecognizedItem
  onApply: () => void
}) {
  const details = [
    recognized.category,
    recognized.condition && CONDITION_LABEL[recognized.condition],
  ].filter(Boolean)

  return (
    <div className="flex flex-col gap-2 rounded-card border border-line bg-card p-3">
      <span className="flex items-center gap-1.5 font-sans text-xs font-semibold text-ink-2">
        <IconSparkle size={13} className="text-brand" />
        Распознали по фото
      </span>

      {/* Что модель дала, то и показываем: с бэкендом это описание, на моках — название.
          Пусто разом бывает, когда снимок разобрался только до категории. */}
      {recognized.title && <p className="text-[14px] leading-5 font-bold">{recognized.title}</p>}

      <p className="text-[13.5px] leading-5 text-ink">
        {recognized.description ??
          (recognized.title ? 'Описание модель не подобрала' : 'Описание подобрать не удалось')}
      </p>

      {details.length > 0 && <p className="text-[12.5px] text-ink-3">{details.join(' · ')}</p>}

      <Button variant="ghost" onClick={onApply} className="self-start px-3 py-2 text-[13.5px]">
        Заполнить этим
      </Button>

      <span className="text-[12px] text-ink-3">
        {recognized.title
          ? 'Проверьте: модель могла ошибиться'
          : 'Название модель не придумывает — впишите сами'}
      </span>
    </div>
  )
}
