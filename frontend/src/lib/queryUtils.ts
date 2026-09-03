export function unwrap<T>(response: { data: T | null; error: string | null }): T {
  if (response.error) {
    throw new Error(response.error)
  }
  return response.data as T
}

export function unwrapRequired<T>(
  response: { data: T | null; error: string | null; status?: number },
  requestName: string,
): T {
  if (response.error) {
    throw new Error(response.error)
  }
  if (response.data == null) {
    const status = response.status ? ` (HTTP ${response.status})` : ''
    throw new Error(`${requestName} returned no data${status}`)
  }
  return response.data
}

export function nextCursor(
  page: { next_cursor?: string | null } | null | undefined,
): string | undefined {
  return page?.next_cursor ?? undefined
}
