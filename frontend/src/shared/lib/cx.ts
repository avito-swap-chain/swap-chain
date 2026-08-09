export type ClassValue = string | false | null | undefined

export const cx = (...values: ClassValue[]): string => values.filter(Boolean).join(' ')
