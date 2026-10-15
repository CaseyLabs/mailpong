# GOAUTH-5 — Verified Mailven Google client assessment

Owner selected GOAUTH-5 on 2026-10-15 (reopened from deferred). [Roadmap](../../PLAN.md#goauth-5); base `94d16fe`. Assessment only: nothing was registered or submitted to Google. Document review only; Google pages read 2026-10-15.

## Recommendation

**Do not pursue a Mailven-owned verified client now; keep bring-your-own.** Revisit only if user demand justifies a yearly cost and a legal entity or domain. The owner chose this (see Decision).

## Findings

| Question | Finding | Source |
| --- | --- | --- |
| Which scopes are restricted? | `https://mail.google.com/`, `gmail.modify`, `gmail.readonly` and `gmail.compose` are restricted; `gmail.send` is sensitive. Mailven requests `mail.google.com` and `gmail.modify`, so verification is the restricted tier. | [Gmail scopes](https://developers.google.com/gmail/api/auth/scopes) |
| Verification needs | Homepage on a verified domain you own, privacy policy on that domain linked from the consent screen, English demo video of the full OAuth flow, written justification per scope (narrowest scopes). | [Verification help](https://support.google.com/cloud/answer/13464321) |
| Annual cycle | Restricted-scope apps must be reverified and complete a security assessment at least every 12 months from the assessor's Letter of Assessment date. | [Restricted scope verification](https://developers.google.com/identity/protocols/oauth2/production-readiness/restricted-scope-verification) |
| Does CASA apply to a local desktop client? | **Unresolved.** Google's page triggers the assessment for apps that can "access data from or through a third-party server" and when restricted data is stored or transmitted on servers. It has no clause for local-only apps. Mailven has no Mailven server, but Google has not said that exempts it; only Google's review can answer. Treat it as likely required until Google says otherwise. | same |
| Exemptions | Personal use (you or a few people you know), development/testing, and internal Workspace apps need no review. This is the status of the bring-your-own client. | same |
| Unverified limits | An unverified app shows the warning screen and is capped at 100 new users. | [Unverified apps](https://support.google.com/cloud/answer/7454865) |
| Cost | Third-party sources, not Google, put CASA Tier 2 (now AL1) at roughly $500–$1,500 a year with an authorized lab, repeated every year; Tier 3/AL2 manual testing about $3,000–$6,000. Self-scan is reported as no longer an option. Prices conflict across sources and are unconfirmed: [Nylas](https://developer.nylas.com/docs/provider-guides/google/google-verification-security-assessment-guide/), [Switch Labs](https://www.switchlabs.dev/post/casa-tier-2-tier-3-security-review-providers-pricing-and-the-cheapest-option), [ADA FAQ](https://appdefensealliance.dev/masa/faq). | secondary |

## Brand, domain and support

- A bundled client needs a Mailven homepage and privacy policy on a domain the owner verifies in Search Console. The project's domain and legal-entity position is an owner matter; none was assumed here.
- A shared client's secret ships in every binary, and Google quota and any abuse flag then apply to all users at once. Support load rises: review rejections, yearly renewals and a quota outage would affect everyone.
- Bundling reverses the 2026-10-15 decision in PLAN (no bundled client; rejected `cloud-platform` bootstrap).

## Reuse of the Aerion CASA Tier 2 questionnaire

[`docs/history/AERION_CASA_TIER2.md`](../history/AERION_CASA_TIER2.md) is historical and describes upstream Aerion. Its architecture sections (local keyring storage, no server, embedded assets, email isolation) are a useful starting draft, but every answer must be re-checked against Mailven's code, extensions and bundled dependencies before submission, and it cannot be submitted as-is.

## Decision

Owner decision 2026-10-15: option 1. Options were: (1) keep bring-your-own and close GOAUTH-5; (2) ask Google whether a local-only client is exempt, with no submission; (3) approve pursuing verification, which needs a domain, entity and budget decision first.

## Limits

Cost figures are secondary and unconfirmed. Google gave no local-only ruling. No lab was contacted.
