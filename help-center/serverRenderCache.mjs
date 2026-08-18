import { createHash } from 'node:crypto'
import { createClient } from 'redis'

const redisDataPrefix = 'hc:d:'
const redisTagPrefix = 'hc:t:'

export function renderCacheKey(cacheKey) {
  const digest = createHash('sha256').update(cacheKey).digest('hex')
  return `render:${digest}`
}

export function helpcenterIdentifierTag(identifier) {
  return `hc:host:${String(identifier || '').trim().toLowerCase()}`
}

export async function createSharedRenderCache({
  redisURL,
  ttlSeconds,
  onInvalidate,
  logger = console,
}) {
  if (!redisURL) return null

  const client = createClient({
    url: redisURL,
    disableOfflineQueue: true,
    socket: {
      connectTimeout: 1000,
    },
  })
  const subscriber = client.duplicate()
  client.on('error', (error) => logger.error('help-center render cache error', error))
  subscriber.on('error', (error) => logger.error('help-center render cache subscriber error', error))

  try {
    await Promise.all([client.connect(), subscriber.connect()])
    await subscriber.subscribe('cache:hc:invalidate', (payload) => {
      try {
        const message = JSON.parse(payload)
        const tags = Array.isArray(message.tags) ? message.tags : []
        onInvalidate(tags)
      } catch (error) {
        logger.error('help-center render cache invalidation decode failed', error)
      }
    })
  } catch (error) {
    logger.error('help-center shared render cache unavailable; using local cache', error)
    await Promise.allSettled([client.close(), subscriber.close()])
    return null
  }

  return {
    async get(cacheKey) {
      const logicalKey = renderCacheKey(cacheKey)
      let payload
      try {
        payload = await client.get(`${redisDataPrefix}${logicalKey}`)
      } catch (error) {
        logger.error('help-center shared render cache read failed', error)
        return null
      }
      if (!payload) return null
      try {
        return JSON.parse(payload)
      } catch (error) {
        logger.error('help-center shared render cache decode failed', error)
        await client.del(`${redisDataPrefix}${logicalKey}`)
        return null
      }
    },

    async set(cacheKey, value, identifier) {
      const logicalKey = renderCacheKey(cacheKey)
      const tag = helpcenterIdentifierTag(identifier)
      const transaction = client.multi()
      transaction.set(`${redisDataPrefix}${logicalKey}`, JSON.stringify(value), {
        EX: ttlSeconds,
      })
      transaction.sAdd(`${redisTagPrefix}${tag}`, logicalKey)
      transaction.expire(`${redisTagPrefix}${tag}`, ttlSeconds * 2)
      try {
        await transaction.exec()
      } catch (error) {
        logger.error('help-center shared render cache write failed', error)
      }
    },
  }
}
