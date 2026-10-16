// Types shared by the account form and its modules.

// OAuth credentials to pass to parent
export interface OAuthCredentials {
  provider: string
  microsoftAccountType?: string
  accessToken: string
  refreshToken: string
  expiresIn: number
}
