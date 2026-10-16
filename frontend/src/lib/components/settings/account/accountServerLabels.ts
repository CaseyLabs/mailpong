import { securityOptions, authMechanismOptions } from '$lib/config/providers'

export function getSecurityLabel(value: string): string {
  return securityOptions.find(opt => opt.value === value)?.label || value
}

export function getAuthMechanismLabel(value: string): string {
  return authMechanismOptions.find(opt => opt.value === value)?.label || value
}
