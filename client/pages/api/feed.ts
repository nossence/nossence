// Next.js API route support: https://nextjs.org/docs/api-routes/introduction
import type { NextApiRequest, NextApiResponse } from 'next'
import type { FeedResponse } from '@/interfaces'

export default function handler(
  req: NextApiRequest,
  res: NextApiResponse<FeedResponse>
) {
  res.status(200).json({
    "data": [
      {
        "kind": 30023,
        "content": "Long-form text note: Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.",
        "summary": "Long-form text note: Lorem Ipsum is simply dummy text of the printing and typesetting industry.", // only for long-form content
        "title": "What is Lorem Ipsum?", // only for long-form content
        "image": "https://picsum.photos/id/400/300", // only for long-form content
        "pubkey": "",
        "created_at": 1677835006,
        "like": 101,
        "repost": 102,
        "reply": 103,
        "zap": 104,
        "relay": ["wss://nos.lol"],
        "event_id": "4123d9d8914810eda63a18f7da6f19cb7acd8915ad07474f70c0a3d4cc8eff04"
      },
      {
        "kind": 1,
        "content": "The quick brown fox jumps over the lazy dog. 1234567890",
        "summary": "", // only for long-form content
        "title": "", // only for long-form content
        "image": "", // only for long-form content
        "pubkey": "",
        "created_at": 1677735006,
        "like": 201,
        "repost": 202,
        "reply": 203,
        "zap": 204,
        "relay": ["wss://relay.damus.io"],
        "event_id": "4123d9d8914810eda63a18f7da6f19cb7acd8915ad07474f70c0a3d4cc8eff03"
      }
    ]
  })
}
