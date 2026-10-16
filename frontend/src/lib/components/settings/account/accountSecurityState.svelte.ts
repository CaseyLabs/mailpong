// Reactive state of the account Security tab. AccountSecurityTab.svelte creates
// one AccountSecurityState; the accountSecurity*.ts modules and child
// components take it.
// @ts-ignore - wailsjs path
import { smime, pgp } from '../../../../../wailsjs/go/models'
// @ts-ignore - wailsjs path
import {
  ListSMIMECertificates,
  GetSMIMESignPolicy,
  GetSMIMEEncryptPolicy,
  ListSenderCerts,
  ListPGPKeys,
  GetPGPSignPolicy,
  GetPGPEncryptPolicy,
  ListPGPSenderKeys,
  GetPGPKeyServers,
} from '../../../../../wailsjs/go/app/App'

/** The tab's props, read lazily so they stay reactive. */
export interface AccountSecurityEnv {
  readonly accountId: string
}

export class AccountSecurityState {
  constructor(private readonly env: AccountSecurityEnv) {}

  get accountId() { return this.env.accountId }

  // State
  certificates = $state<smime.Certificate[]>([])
  senderCerts = $state<smime.SenderCert[]>([])
  signPolicy = $state('never')
  encryptPolicy = $state('never')
  loading = $state(true)
  importing = $state(false)

  // Import dialog state
  showImportDialog = $state(false)
  importFilePath = $state('')
  importPassword = $state('')
  importError = $state('')
  showBERConfirmDialog = $state(false)

  // Recipient cert import dialog state
  showRecipientImportDialog = $state(false)
  recipientImportFilePath = $state('')
  recipientImportEmail = $state('')
  recipientImportError = $state('')
  recipientImporting = $state(false)

  // PGP state
  pgpKeys = $state<pgp.Key[]>([])
  pgpSenderKeys = $state<pgp.SenderKey[]>([])
  pgpSignPolicy = $state('never')
  pgpEncryptPolicy = $state('never')
  pgpImporting = $state(false)

  // PGP import dialog state
  showPGPImportDialog = $state(false)
  pgpImportFilePath = $state('')
  pgpImportPassphrase = $state('')
  pgpImportError = $state('')

  // PGP recipient key import dialog state
  showPGPRecipientImportDialog = $state(false)
  pgpRecipientImportFilePath = $state('')
  pgpRecipientImportEmail = $state('')
  pgpRecipientImportError = $state('')
  pgpRecipientImporting = $state(false)

  // Key lookup state (unified WKD + HKP)
  keyLookupEmail = $state('')
  keyLookupLoading = $state(false)

  // Key server state
  keyServers = $state<pgp.KeyServer[]>([])
  newKeyServerURL = $state('')
  addingKeyServer = $state(false)
  keyServersCollapsed = $state(true)

  // Section collapse state (collapsed by default)
  pgpCollapsed = $state(true)
  smimeCollapsed = $state(true)

  async loadData() {
    this.loading = true
    try {
      const [certs, sPolicy, ePolicy, senderCertList, pKeys, pgpSPolicy, pgpEPolicy, pSenderKeys, pKeyServers] = await Promise.all([
        ListSMIMECertificates(this.accountId),
        GetSMIMESignPolicy(this.accountId),
        GetSMIMEEncryptPolicy(this.accountId),
        ListSenderCerts(),
        ListPGPKeys(this.accountId),
        GetPGPSignPolicy(this.accountId),
        GetPGPEncryptPolicy(this.accountId),
        ListPGPSenderKeys(),
        GetPGPKeyServers(),
      ])
      this.certificates = certs || []
      this.signPolicy = sPolicy || 'never'
      this.encryptPolicy = ePolicy || 'never'
      this.senderCerts = senderCertList || []
      this.pgpKeys = pKeys || []
      this.pgpSignPolicy = pgpSPolicy || 'never'
      this.pgpEncryptPolicy = pgpEPolicy || 'never'
      this.pgpSenderKeys = pSenderKeys || []
      this.keyServers = pKeyServers || []
    } catch (err) {
      console.error('Failed to load security data:', err)
    } finally {
      this.loading = false
    }
  }
}
