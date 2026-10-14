/** Review never sends. Only confirmation on a new Send/Retry action returns a token. */
export class SubmissionReview {
  open = $state(false)
  outcome = $state('unknown')
  #resolve: ((confirmed: boolean) => void) | null = null

  ask(outcome: string): Promise<boolean> {
    this.finish(false)
    this.outcome = outcome
    this.open = true
    return new Promise(resolve => { this.#resolve = resolve })
  }

  finish(confirmed: boolean) {
    this.open = false
    const resolve = this.#resolve
    this.#resolve = null
    resolve?.(confirmed)
  }
}
