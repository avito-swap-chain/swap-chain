import type { Item } from '../model/types'

/** Что распознавание достаёт из фотографии. Желание и само фото остаются за человеком. */
export type RecognizedItem = Pick<Item, 'title' | 'category' | 'condition'>

/**
 * Пауза перед ответом: за неё состояние «распознаём» успевает стать видимым.
 * Вынесена в параметр, чтобы тесты не ждали реального времени.
 */
const RECOGNIZE_MS = 1700

/**
 * Что заглушка узнаёт: фрагмент имени файла → правдоподобный результат.
 * Ключи покрывают демо-фотографии из `public/mock/items`.
 */
const KNOWN: Record<string, RecognizedItem> = {
  bike: { title: 'Горный велосипед', category: 'Спорт и отдых', condition: 'good' },
  dumbbells: { title: 'Гантели 20 кг', category: 'Спорт и отдых', condition: 'used' },
  scooter: { title: 'Электросамокат', category: 'Транспорт', condition: 'good' },
  monitor24: { title: 'Монитор 24"', category: 'Электроника', condition: 'good' },
  monitor: { title: 'Монитор LG 27" IPS', category: 'Электроника', condition: 'good' },
  console: { title: 'Игровая приставка PlayStation 4', category: 'Электроника', condition: 'used' },
  camera: { title: 'Плёночный фотоаппарат', category: 'Электроника', condition: 'used' },
  keyboard: { title: 'Механическая клавиатура', category: 'Электроника', condition: 'new' },
  watch: { title: 'Умные часы Amazfit GTR', category: 'Электроника', condition: 'good' },
  phone: { title: 'Смартфон', category: 'Электроника', condition: 'good' },
  headphones: { title: 'Наушники', category: 'Аудио', condition: 'good' },
  guitar: { title: 'Акустическая гитара', category: 'Хобби и творчество', condition: 'good' },
  coffee: { title: 'Кофеварка', category: 'Дом и дача', condition: 'good' },
  grinder: { title: 'Кофемолка', category: 'Дом и дача', condition: 'good' },
  lamp: { title: 'Настольная лампа', category: 'Дом и дача', condition: 'good' },
  blanket: { title: 'Плед', category: 'Дом и дача', condition: 'new' },
  beanbag: { title: 'Кресло-мешок', category: 'Дом и дача', condition: 'good' },
}

/**
 * Распознавание вещи по фотографии — второй вход в сервис, публикация нового объявления:
 * там фото загружают с нуля и поля пустые. У главного входа (обмен у уже размещённого
 * объявления) распознавать нечего — название и категория там уже заполнены.
 *
 * ⚠️ Заглушка: модели за этой функцией нет. Результат берётся из имени файла — демо-фото
 * из `public/mock/items` она узнаёт, на остальных честно отказывается, а не выдумывает
 * название. Реальная модель заменит **тело** функции: интерфейс (файл → название, категория,
 * состояние; не узнали — ошибка) рассчитан на настоящий запрос к бэку и меняться не должен.
 *
 * Ответ — подсказка, а не решение за человека: любое поле в форме можно переписать.
 */
export async function recognizeItem(file: File, delayMs = RECOGNIZE_MS): Promise<RecognizedItem> {
  await new Promise((resolve) => setTimeout(resolve, delayMs))

  const name = file.name.toLowerCase()

  // Длинные ключи проверяем первыми: `monitor24.jpg` подходит и под `monitor`.
  const key = Object.keys(KNOWN)
    .sort((a, b) => b.length - a.length)
    .find((candidate) => name.includes(candidate))

  if (!key) throw new Error('Не удалось распознать вещь на фото')

  return KNOWN[key]
}
