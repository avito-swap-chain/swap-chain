/** Экран снимаем при 430×800 — те же пропорции держим в кадре. */
export const SHOT_WIDTH = 560
export const SHOT_HEIGHT = Math.round((800 * SHOT_WIDTH) / 430)

/** Строка состояния — часть корпуса, а не наложение поверх: контент под ней не теряется. */
export const STATUS_BAR_HEIGHT = 44
