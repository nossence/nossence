import type { Post, FeedResponse } from "@/interfaces"
import useSWR, { Fetcher } from 'swr'

const fetcher: Fetcher<FeedResponse> = () => fetch(new URL('/api/feed', process.env.NEXT_PUBLIC_API_BASE_URI)).then(res => res.json())

export default function usePosts() {
  const { data: feedResponse } = useSWR(
    '/api/feed',
    fetcher
  )

  const posts: Post[] = feedResponse?.data || []

  const onLike = async (post: Post): Promise<void> => {
    console.log(`Send like event for ${post.event_id} to relay: ${post.relay.join()}`)
  }

  return { posts, onLike }
}