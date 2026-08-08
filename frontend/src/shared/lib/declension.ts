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

/** Родительный падеж имени: «получаете от Марка», «получаете от Даши». */
export function genitive(name: string): string {
  if (name.endsWith('я')) return swapEnding(name, 'и')
  if (HUSHING_A.test(name)) return swapEnding(name, 'и')
  if (name.endsWith('а')) return swapEnding(name, 'ы')
  if (/[йь]$/.test(name)) return swapEnding(name, 'я')
  if (CONSONANT.test(name)) return `${name}а`
  return name
}
