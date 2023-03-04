export type Post = {
  kind: ShortTexNoteKind | LongFormContentKind
  content: string
  summary: string // only for long-form content
  title: string // only for long-form content
  image: string // only for long-form content
  pubkey: string
  created_at: number
  like: number
  repost: number
  reply: number
  zap: number
  relay: string[]
  event_id: string
}

export type FeedResponse = {
  data: Post[]
}

export type LongFormContentKind = 30023
export type ShortTexNoteKind = 1