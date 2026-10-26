/** Adds value to the list, or removes it when present. */
export function toggleListValue<T extends string>(list: T[], value: T): T[] {
  return list.includes(value) ? list.filter((item) => item !== value) : [...list, value]
}
