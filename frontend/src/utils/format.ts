const defaultAvatar = 'https://static.productionready.io/images/smiley-cyrus.jpg'

export function avatarUrl(image: string | null | undefined) {
  return image || defaultAvatar
}

export function formatDate(value: string) {
  return new Intl.DateTimeFormat('en', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  }).format(new Date(value))
}

export function splitTags(value: string) {
  return [...new Set(value.split(',').map((tag) => tag.trim()).filter(Boolean))]
}
