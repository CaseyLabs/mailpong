import { GetMailAPIRouteStatus } from '../../../wailsjs/go/app/App'

/** Which accounts currently read and send through a mail API route. */
class MailApiRouteStore {
  /** accountId -> 'gmail-api' | 'graph-api', only while the route is active. */
  active = $state<Record<string, string>>({})

  async refresh(accountId: string) {
    try {
      const status = await GetMailAPIRouteStatus(accountId)
      const next = { ...this.active }
      if (status.state === 'active') next[accountId] = status.route
      else delete next[accountId]
      this.active = next
    } catch {
      // Route state is informational here; the settings section reports errors.
    }
  }

  async refreshAll(accountIds: string[]) {
    await Promise.all(accountIds.map((id) => this.refresh(id)))
  }
}

export const mailApiRoutes = new MailApiRouteStore()

export function mailApiRouteName(route: string | undefined): string {
  switch (route) {
    case 'gmail-api': return 'routeGmail'
    case 'graph-api': return 'routeGraph'
    default: return 'routeProtocol'
  }
}
