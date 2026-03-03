// Utility functions for generating and working with URL slugs

/**
 * Generate a URL-friendly slug from a workspace name
 */
export function generateWorkspaceSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    // Replace spaces and special characters with hyphens
    .replace(/[^a-z0-9]+/g, '-')
    // Remove leading/trailing hyphens
    .replace(/^-+|-+$/g, '')
    // Collapse multiple hyphens into one
    .replace(/-+/g, '-')
    // Limit length to 50 characters
    .substring(0, 50)
    .replace(/-+$/, '') // Remove trailing hyphen if substring cut in middle of word
}

/**
 * Validate that a slug is URL-safe
 */
export function isValidSlug(slug: string): boolean {
  // Must be 2-50 characters, lowercase letters, numbers, and hyphens only
  // Cannot start or end with hyphen
  const slugRegex = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/
  return slugRegex.test(slug) && slug.length >= 2 && slug.length <= 50
}

/**
 * Ensure slug is unique by adding suffix if needed
 */
export function ensureUniqueSlug(baseSlug: string, existingSlugs: string[]): string {
  let slug = baseSlug
  let counter = 1

  while (existingSlugs.includes(slug)) {
    slug = `${baseSlug}-${counter}`
    counter++
  }

  return slug
}

/**
 * Convert workspace name to display-friendly format
 */
export function slugToDisplayName(slug: string): string {
  return slug
    .split('-')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}

// Examples:
// "Acme Corporation" → "acme-corporation"
// "Product A (Demo)" → "product-a-demo"
// "Brand X - Enterprise" → "brand-x-enterprise"
// "My Company!!!" → "my-company"