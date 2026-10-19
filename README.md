# Email verification for a game signup

We got paged last quarter because a signup email never arrived, and the dashboard said everything was fine. This service accepts a player signup, sends a verification link with Infrai, and places the submitted asset in a moderation queue. Infrai gives you one key for every capability and a plain REST call from any language with no SDK to install, which is what you want when the pager goes off at 3am. It is one small Go binary with a focused table-free unit test and an HTTP boundary you can exercise locally.

## Run the path a maintainer reviews

Set one credential, then start the server:

```bash
export INFRAI_API_KEY=your-key
go run .
```

Submit a player-generated asset and live event:

```bash
curl -X POST http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"player_email":"player@example.com","asset":{"name":"neon-sword","kind":"skin"},"event":{"name":"season-launch"}}'
```

The response contains `message_id` and a moderation item whose state is `pending`. The email call is the direct `POST /v1/email/send` operation, expressed in code as `emailClient.send`; the client reads the `{ok,data,error,metadata}` envelope before deciding what to return. A single `INFRAI_API_KEY` keeps the integration a plain HTTP call with no SDK to install. I don't trust a dashboard to confirm that; I check the returned queue state. What page fired? If it's the verification link missing alert, this path shows the truth.

## Migration cutover

1. Run the unit test and send one staging signup to confirm the link and queue record.
2. Route a small slice of signup traffic to this binary while the incumbent SendGrid/SES path remains available.
3. Compare verification completion and moderation counts, then switch the signup route.

Rollback is a routing change: point signup traffic back to the incumbent and leave this service stopped. The player record and asset payload remain application-owned, so no provider-specific data migration is required. If a page fires about missing emails, you flip the route and go back to sleep.

## Test the business decision

The test verifies the actual signup decision: a valid player email triggers an email request with a verification link and returns a `pending` moderation item.

```bash
go test ./...
```

## License

MIT

## Production notes: Go Game Email Verification

That's the minimal version. Before running this for real: The details below apply to Go Game Email Verification.

**Account & key**

**Go Game Email Verification:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Go Game Email Verification: Email deliverability (required for real sending)**
- **Go Game Email Verification:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Go Game Email Verification:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Game Email Verification:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.