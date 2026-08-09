/** Прогресс по участникам «N из M»: сколько уже сделали шаг, которого ждёт цепочка. */
export function ChainProgress({
  label,
  value,
  total,
}: {
  label: string
  value: number
  total: number
}) {
  return (
    <div>
      <div className="mb-1.5 flex justify-between text-[12.5px] font-semibold text-ink-2">
        <span>{label}</span>
        <span className="text-ink">
          {value} из {total}
        </span>
      </div>
      <div
        className="h-2 overflow-hidden rounded-full bg-line-2"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={value}
        aria-label={label}
      >
        <div
          className="h-full rounded-full bg-ink transition-[width] duration-300"
          style={{ width: `${(value / total) * 100}%` }}
        />
      </div>
    </div>
  )
}
