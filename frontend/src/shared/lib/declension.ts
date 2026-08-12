// Русский интерфейс склоняет имена: «отдаёте Марку», а не «отдаёте: Марк» — иначе строка
// звучит как поле анкеты. Падеж выводим по окончанию: имена участников приходят с бэка
// обычной строкой, отдельных полей под падежи в модели нет и заводить их незачем.

const CONSONANT = /[бвгджзклмнпрстфхцчшщ]$/
/** После шипящих и заднеязычных в родительном пишется «и», а не «ы»: Даша → Даши, Ольга → Ольги. */
const HUSHING_A = /[жчшщгкх]а$/

const swapEnding = (name: string, ending: string) => `${name.slice(0, -1)}${ending}`

/** Дательный падеж имени: «отдаёте Марку», «отдаёте Даше». Незнакомое окончание не трогаем. */
export function dative(name: string): string {
  if (name.endsWith('ия')) return swapEnding(name, 'и')
  if (/[ая]$/.test(name)) return swapEnding(name, 'е')
  if (/[йь]$/.test(name)) return swapEnding(name, 'ю')
  if (CONSONANT.test(name)) return `${name}у`
  return name
}

/**
 * Творительный падеж имени: «обмен с Марком», «обмен с Катей», «обмен с Дашей».
 * После шипящих у имён на «-а» окончание безударное — «ей», а не «ой».
 */
export function instrumental(name: string): string {
  if (name.endsWith('я')) return swapEnding(name, 'ей')
  if (HUSHING_A.test(name)) return swapEnding(name, 'ей')
  if (name.endsWith('а')) return swapEnding(name, 'ой')
  if (/[йь]$/.test(name)) return swapEnding(name, 'ем')
  if (CONSONANT.test(name)) return `${name}ом`
  return name
}

/** Родительный падеж имени: «получаете от Марка», «получаете от Даши». */
export function genitive(name: string): string {
  if (name.endsWith('я')) return swapEnding(name, 'и')
  if (HUSHING_A.test(name)) return swapEnding(name, 'и')
  if (name.endsWith('а')) return swapEnding(name, 'ы')
  if (/[йь]$/.test(name)) return swapEnding(name, 'я')
  if (CONSONANT.test(name)) return `${name}а`
  return name
}

/** «24 отзыва», «41 отзыв», «17 отзывов» — без склонения подпись выглядит машинной. */
export function reviewsLabel(count: number): string {
  const tail = count % 100
  if (tail > 4 && tail < 21) return `${count} отзывов`

  switch (count % 10) {
    case 1:
      return `${count} отзыв`
    case 2:
    case 3:
    case 4:
      return `${count} отзыва`
    default:
      return `${count} отзывов`
  }
}

/** «1 обмен», «3 обмена», «17 обменов» — та же причина, что и у отзывов. */
export function exchangesLabel(count: number): string {
  const tail = count % 100
  if (tail > 4 && tail < 21) return `${count} завершённых обменов`

  switch (count % 10) {
    case 1:
      return `${count} завершённый обмен`
    case 2:
    case 3:
    case 4:
      return `${count} завершённых обмена`
    default:
      return `${count} завершённых обменов`
  }
}
