import { useState, type ChangeEvent, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { editProfile, uploadAvatar, userKeys, type Profile } from '@/entities/user'
import { sessionKeys } from '@/shared/model/session'
import { ActionError, Button, Field, Input, Status } from '@/shared/ui'

/**
 * Правка своего профиля: имя и фотография. Отдельной кнопки «загрузить фото» нет —
 * жмут на сам аватар, как в кабинете Авито.
 *
 * Фотография сохраняется сразу после выбора, а имя — по кнопке: выбор файла это уже
 * законченное действие, а имя человек дописывает по буквам.
 */
export function EditProfile({ profile }: { profile: Profile }) {
  const [name, setName] = useState(profile.name)
  const queryClient = useQueryClient()

  // Имя и аватар видны и в шапке, и в кабинете — после правки перечитываем и сессию.
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: userKeys.all })
    queryClient.invalidateQueries({ queryKey: sessionKeys.current() })
  }

  const save = useMutation({
    mutationFn: (patch: Parameters<typeof editProfile>[1]) => editProfile(profile.id, patch),
    onSuccess: refresh,
  })

  const pickPhoto = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    save.mutate({ avatarUrl: await uploadAvatar(file) })
  }

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (name.trim() && name.trim() !== profile.name) save.mutate({ name })
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3.5 border-t border-line pt-4">
      {/* Аватар не повторяем — он стоит крупным над формой. Здесь только действия над ним.
          Пустую строку бэкенд понимает как «убрать фотографию», поэтому предлагаем это
          отдельно и только тогда, когда убирать есть что. */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <label className="cursor-pointer rounded-sm text-[13.5px] font-semibold text-brand outline-offset-4 hover:opacity-80 focus-within:outline-2 focus-within:outline-brand">
          {profile.avatarUrl ? 'Заменить фотографию' : 'Добавить фотографию'}
          <input type="file" accept="image/*" className="sr-only" onChange={pickPhoto} />
        </label>

        {profile.avatarUrl && (
          <button
            type="button"
            onClick={() => save.mutate({ avatarUrl: '' })}
            className="cursor-pointer rounded-sm text-[13.5px] font-semibold text-ink-3 outline-offset-4 hover:text-ink-2 focus-visible:outline-2 focus-visible:outline-brand"
          >
            Убрать фотографию
          </button>
        )}
      </div>

      <Field label="Имя">
        <Input
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Как вас зовут"
          maxLength={100}
          required
        />
      </Field>

      {save.isSuccess && !save.isPending && <Status tone="ok">Сохранено</Status>}
      <ActionError
        error={save.error}
        conflict="Профиль изменился в другом окне — обновите страницу"
      />

      <Button
        type="submit"
        fullWidth
        disabled={save.isPending || !name.trim() || name.trim() === profile.name}
      >
        {save.isPending ? 'Сохраняем…' : 'Сохранить имя'}
      </Button>
    </form>
  )
}
