import { useQueryClient } from '@tanstack/react-query'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'

/**
 * Демо-инструмент: посмотреть один и тот же обмен глазами разных участников —
 * иначе цепочку из четырёх человек по ссылке не показать. В продукте на этом месте
 * был бы аккаунт, поэтому в бизнес-логику переключатель не вмешивается: он только
 * меняет, кто «я», а `isMe` и состав кабинета считает api.
 */
export function SwitchPersona() {
  const personaId = usePersonaStore((state) => state.personaId)
  const setPersonaId = usePersonaStore((state) => state.setPersonaId)
  const queryClient = useQueryClient()
  const persona = PERSONAS.find((item) => item.id === personaId) ?? PERSONAS[0]

  return (
    <label className="flex items-center gap-2">
      {/* На узких окнах подпись прячется: в одну строку шапки лезут лок-ап, поиск и аккаунт,
          а смысл переключателя и так виден по имени в поле. Экранным читалкам она остаётся. */}
      <span className="text-[11px] leading-tight text-ink-3 max-sm:sr-only">Смотрю как</span>
      <select
        title="Смотрю как"
        value={personaId}
        onChange={(event) => {
          setPersonaId(event.target.value)
          // В кеше лежат ответы прошлой персоны — без сброса экран показал бы чужой кабинет.
          queryClient.resetQueries()
        }}
        className="cursor-pointer rounded-chip border border-line bg-page py-1 pr-1.5 pl-2 font-sans text-[12.5px] font-bold text-ink focus-visible:outline-2 focus-visible:outline-brand"
      >
        {PERSONAS.map((persona) => (
          <option key={persona.id} value={persona.id}>
            {persona.name}
          </option>
        ))}
      </select>

      {/* Аватар аккаунта — крайний правый элемент шапки, как у Авито (40px, круглый).
          Он же показывает, за кого сейчас смотрят: имя в поле дублируется лицом. */}
      <img
        src={persona.avatarUrl}
        alt=""
        width={40}
        height={40}
        className="size-10 shrink-0 rounded-full object-cover"
      />
    </label>
  )
}
