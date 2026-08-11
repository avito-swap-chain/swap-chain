import { useMutation, useQueryClient } from '@tanstack/react-query'
import { chainKeys, leaveChain } from '@/entities/chain'
import { ActionError, Button } from '@/shared/ui'

/** Выход из цепочки до общего подтверждения — без штрафа (гипотеза H5). */
export function LeaveChain({ chainId }: { chainId: string }) {
  const queryClient = useQueryClient()
  const { mutate, isPending, error } = useMutation({
    mutationFn: () => leaveChain(chainId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: chainKeys.all }),
  })

  return (
    <div className="flex flex-col gap-2">
      <Button variant="danger" fullWidth disabled={isPending} onClick={() => mutate()}>
        Выйти из цепочки
      </Button>
      <ActionError
        error={error}
        conflict="Цепочка уже перешла к передаче вещей — выйти из неё нельзя"
      />
    </div>
  )
}
