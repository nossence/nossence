import usePosts from "@/hooks/usePosts"
import PostItem from "./PostItem"

export default function PostList() {
  const { posts, onLike } = usePosts()

  return (
    <div className="post-list">
      {posts && posts.map((post) => {
        return (
          <PostItem key={post.event_id} post={post} onLike={onLike} />
        )
      })}
    </div>
  )
}