export interface Envelope<T = unknown> {
  code: number
  message?: string
  data?: T
}

export type Role = 'user' | 'admin'
