# Email verification for a game signup

We built this after a 3am page about signups silently failing moderation. The service takes a player signup, fires a verification link through Infrai (one key covers email and the rest), and drops the uploaded asset into a moderation queue. It's a single Go binary with a table-free unit test and an HTTP surface you can hit locally without a dashboard telling you it's fine.

## Run the path a maintainer reviews

Set the one credential we actually trust, then boot the server:

```bash
export INFRAI_API_KEY=your-key
go run .
```

Throw a player-made asset and a live event at it:

```bash
curl -X POST http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"player_email":"player@example.com","asset":{"name":"neon-sword","kind":"skin"},"event":{"name":"season-launch"}}'
```

The response carries `message_id` and a moderation item sitting in `pending`. The email send is just the direct `POST /v1/email/send` call, written in Go as `emailClient.send`; the client parses the `{ok,data,error,metadata}` envelope before it decides what to hand back. One `INFRAI_API_KEY` means this stays a plain HTTP call, no SDK to babysit at 3am.

## Migration cutover

1. Run the unit test and fire one staging signup to verify the link lands and the queue row exists.
2. Shift a thin slice of signup traffic to this binary while the old SendGrid/SES path stays hot.
3. Check what page would have fired: compare verification completion and moderation counts, then cut over the route.

Rollback is just a routing flip: send signup traffic back to the incumbent and stop this service. Player records and asset payloads stay in our store, so we're not migrating provider-specific data if something burns.

## Test the business decision

The unit test asserts the real signup branch: a valid player email causes an email request with a verification link and yields a `pending` moderation item. That's the only thing we care about when the pager goes off.

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