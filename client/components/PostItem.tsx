import { Post } from "@/interfaces"
import { formatDistanceToNow } from "date-fns"

type PostItemProps = {
  post: Post
  onLike: (post: Post) => Promise<void>
}

export default function PostItem({ post, onLike }: PostItemProps) {
  return (
    <div className="post-item">
      <div className="post-item-author">
        { post.pubkey }
      </div>
      <div className="post-item-main">
        <p>{ post.kind === 30023 ? post.summary : post.content }</p>
        <p>Created at: { formatDistanceToNow(post.created_at * 1000, { addSuffix: true }) }</p>
      </div>
      <div className="post-item-footer">
        <p>Likes: { post.like }</p>
        <button className="post-item-like" aria-label="Like" onClick={() => onLike(post)}>
          👍
        </button>
      </div>
    </div>
  )
}