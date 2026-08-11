import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PERSONAS, usePersonaStore } from '@/shared/model/persona'
import {
  DEMO_USERS,
  getCurrentUser,
  knownAccounts,
  login,
  logout,
  sessionKeys,
  type CurrentUser,
} from '@/shared/model/session'
import { asset } from '@/shared/lib'
import { isBackendConnected } from '@/shared/config/backend'

/** Строка переключателя: имя в поле, телефон для входа, корона у сотрудника ПВЗ. */
interface SwitchOption {
  name: string
  phone: string
  isAdmin?: boolean
}

const SELECT_CLASS =
  'cursor-pointer rounded-chip border border-line bg-page py-1 pr-1.5 pl-2 font-sans text-[12.5px] font-bold text-ink focus-visible:outline-2 focus-visible:outline-brand'

/**
 * Аватар аккаунта — крайний правый элемент шапки, как у Авито (40px, круглый).
 * Он же показывает, за кого сейчас смотрят: имя в поле дублируется лицом. У пользователя
 * с бэкенда фотографии нет — тогда кружок с инициалом, выдумывать ему лицо незачем.
 *
 * Админа помечаем короной: на демо переключаются между обычным человеком и сотрудником ПВЗ,
 * и по одному имени не понять, почему у одного аккаунта вдруг появились чужие доставки.
 */
function AccountAvatar({ user }: { user: CurrentUser }) {
  const face = user.avatarUrl ? (
    <img
      src={asset(user.avatarUrl)}
      alt=""
      width={40}
      height={40}
      className="size-10 rounded-full object-cover"
    />
  ) : (
    <span
      aria-hidden
      className="grid size-10 place-items-center rounded-full bg-line text-[15px] font-bold text-ink-2"
    >
      {user.name.slice(0, 1).toUpperCase()}
    </span>
  )

  if (!user.isAdmin) return <span className="shrink-0">{face}</span>

  return (
    <span className="relative shrink-0" title="Аккаунт сотрудника ПВЗ">
      {face}
      {/* Корона садится на угол аватара, но не вылезает за ярус шапки: выше ей места нет. */}
      <span aria-hidden className="absolute -top-0.5 -right-1 text-[13px] leading-none">
        👑
      </span>
      <span className="sr-only">Аккаунт сотрудника ПВЗ</span>
    </span>
  )
}

/**
 * Демо-инструмент: посмотреть один и тот же обмен глазами разных участников — иначе
 * цепочку из трёх человек по ссылке не показать.
 *
 * На моках он только меняет, кто «я»: в бизнес-логику не вмешивается, `isMe` и состав
 * кабинета считает api. С бэкендом смотреть чужими глазами можно только по-настоящему —
 * пользователя опознаёт кука, поэтому переключение это выход и вход под другим телефоном.
 */
export function SwitchPersona() {
  const personaId = usePersonaStore((state) => state.personaId)
  const setPersonaId = usePersonaStore((state) => state.setPersonaId)
  const queryClient = useQueryClient()
  const { data: user } = useQuery({ queryKey: sessionKeys.current(), queryFn: getCurrentUser })

  const switchUser = useMutation({
    mutationFn: async (phone: string) => {
      await logout()
      await login(phone)
    },
    // В кеше лежат ответы прошлого пользователя — без сброса экран показал бы чужой кабинет.
    onSettled: () => queryClient.resetQueries(),
  })

  if (!user) return null

  // Список для выбора: засеянные бэкендом демо-пользователи плюс все, кто входил с этого
  // браузера. Раньше своим здесь был только текущий, и после переключения на другого человека
  // зарегистрировавшийся терял себя из виду. Сравниваем по телефону: имена не уникальны,
  // и на них выбор промахивался.
  const options: SwitchOption[] = isBackendConnected
    ? [
        ...knownAccounts().filter(
          (account) => !DEMO_USERS.some((demo) => demo.phone === account.phone),
        ),
        ...DEMO_USERS,
      ]
    : PERSONAS.map((persona) => ({ name: persona.name, phone: persona.id }))

  return (
    <span className="flex items-center gap-2">
      <label className="flex items-center gap-2">
        {/* На узких окнах подпись прячется: в одну строку шапки лезут лок-ап, поиск и аккаунт,
            а смысл переключателя и так виден по имени в поле. Экранным читалкам она остаётся. */}
        <span className="text-[11px] leading-tight text-ink-3 max-sm:sr-only">Смотрю как</span>
        {/* Значение пункта — телефон (на моках его роль играет id персоны), а не имя:
            имена не уникальны и меняются, и выбор по ним промахивался. */}
        <select
          title="Смотрю как"
          value={isBackendConnected ? (user.phone ?? '') : personaId}
          disabled={switchUser.isPending}
          onChange={(event) => {
            if (!isBackendConnected) {
              setPersonaId(event.target.value)
              queryClient.resetQueries()
              return
            }

            switchUser.mutate(event.target.value)
          }}
          className={SELECT_CLASS}
        >
          {options.map((option) => (
            <option key={option.phone || option.name} value={option.phone}>
              {/* Корона и в списке — иначе на демо не видно, под кем открывать админку ПВЗ. */}
              {option.isAdmin ? `${option.name} 👑` : option.name}
            </option>
          ))}
        </select>
      </label>

      {isBackendConnected && (
        <button
          type="button"
          onClick={() => logout().then(() => queryClient.resetQueries())}
          className="cursor-pointer rounded-sm text-[11px] leading-tight text-ink-3 hover:text-ink-2 focus-visible:outline-2 focus-visible:outline-brand"
        >
          Выйти
        </button>
      )}

      <AccountAvatar user={user} />
    </span>
  )
}
