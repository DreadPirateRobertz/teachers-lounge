import { BattleArena } from './BattleArena'

export default async function BossBattlePage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = await params
  return <BattleArena chapterId={id} />
}
