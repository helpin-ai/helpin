export function unwrap<T>(response: { data: T | null; error: string | null }): T {
  if (response.error) {
    throw new Error(response.error)
  }
  return response.data as T
}
