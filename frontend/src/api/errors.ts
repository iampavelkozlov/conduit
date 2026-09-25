type ErrorResponse = {
  errors?: Record<string, string[]>
}

export class ApiError extends Error {
  readonly messages: string[]
  readonly status: number

  constructor(error: unknown, status: number) {
    const messages = readMessages(error)
    super(messages[0] ?? 'Something went wrong. Please try again.')
    this.name = 'ApiError'
    this.messages = messages
    this.status = status
  }
}

function readMessages(error: unknown): string[] {
  if (!isErrorResponse(error)) return ['Something went wrong. Please try again.']

  return Object.entries(error.errors ?? {}).flatMap(([field, messages]) =>
    messages.map((message) => `${field} ${message}`),
  )
}

function isErrorResponse(value: unknown): value is ErrorResponse {
  return typeof value === 'object' && value !== null && 'errors' in value
}

export function errorMessages(error: unknown) {
  return error instanceof ApiError ? error.messages : ['Unable to reach the server.']
}
