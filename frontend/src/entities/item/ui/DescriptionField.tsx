import { Field, Status, Textarea, type StatusTone } from '@/shared/ui'
import { descriptionQuality, type DescriptionQuality } from '../lib/descriptionQuality'

/**
 * Что человек видит вместо порогов длины. Формулировки про подбор, а не про символы:
 * считать буквы — не его работа, ему важно, найдётся ли обмен.
 */
const QUALITY: Record<DescriptionQuality, { tone: StatusTone; label: string }> = {
  short: { tone: 'stop', label: 'Слишком коротко для подбора' },
  fair: { tone: 'attention', label: 'Добавьте деталей — найдётся точнее' },
  good: { tone: 'ok', label: 'Хорошее описание' },
}

interface DescriptionFieldProps {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

/**
 * Описание вещи с оценкой качества. Поле общее для публикации и правки: подсказка нужна
 * в обоих местах одинаково, а бэкенд ищет обмен по `title + description` — это единственное,
 * из чего подбор узнаёт о вещи что-то сверх названия.
 */
export function DescriptionField({ value, onChange, disabled }: DescriptionFieldProps) {
  const quality = value.trim() ? QUALITY[descriptionQuality(value)] : undefined

  return (
    <Field label="Описание">
      <Textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="Что за вещь, как долго у вас, что в комплекте"
        rows={3}
        maxLength={4000}
        disabled={disabled}
      />
      {/* Подсказка появляется только когда есть что оценивать: у пустого поля она читалась бы
          как упрёк за то, что человек ещё не начал. */}
      {quality && <Status tone={quality.tone}>{quality.label}</Status>}
    </Field>
  )
}
