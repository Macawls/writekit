# WriteKit

Publish by conversation. WriteKit is an MCP-native publishing platform — write
and manage your site through Claude, Cursor, or any MCP client, or use the
built-in web dashboard.

## Two ways to run it

**Hosted at [writekit.dev](https://writekit.dev).** Sign in, get a subdomain,
start writing. Paid tiers unlock higher limits, teams, and custom domains.

**Desktop app (coming soon).** Download WriteKit for Windows, macOS, or Linux.
Runs entirely on your machine — no account, no cloud, no subscription. Your
content lives in a local SQLite file; the built-in MCP server lets Claude
Desktop (or any MCP client) read and write to it over `127.0.0.1`.

## License & open-source model

WriteKit is **fully open source under [AGPL-3.0-or-later](LICENSE)**. The
entire codebase is public — including Stripe, team management, admin, and
every feature that runs on the hosted service. The desktop app and the
hosted service build from the same repository.

The AGPL's network-use clause (§13) means that if you modify WriteKit and
run your fork as a network service, you must publish your changes under the
same license. That keeps the hosted-service playing field level; it does
**not** restrict self-hosting, personal use, or modification.

If you just want to write, `writekit.dev` is run by us and paid tiers help
fund development. If you'd rather self-host or run the desktop app, you're
welcome to — every feature is in this repo.
