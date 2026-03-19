
# Miren Labs

Miren Labs is where we ship experimental features that aren't quite ready for prime time. These are capabilities we're actively developing and want to get into your hands early for feedback.

## What to Expect

Labs features are:

- **Experimental** — APIs and behavior may change based on feedback
- **Opt-in** — Disabled by default, you choose when to try them
- **Supported** — We want to hear about bugs and rough edges
- **On a path** — Most labs features are headed toward stable release

## Enabling Labs Features

Labs features are controlled via the `--labs` flag or `MIREN_LABS` environment variable when starting the Miren server.

```bash
# Enable a single labs feature
miren server --labs adminapi

# Enable multiple features
miren server --labs adminapi --labs usersubdomains

# Via environment variable
MIREN_LABS=adminapi miren server

# Multiple features via environment variable (comma-separated)
MIREN_LABS=adminapi,usersubdomains miren server
```

## Giving Feedback

We'd love to hear how labs features work for you:

- **What's working well** — Helps us know we're on the right track
- **What's confusing** — Documentation gaps, unclear behavior
- **What's broken** — Bugs, crashes, unexpected behavior
- **What's missing** — Features that would make it useful for your use case

Reach out via [Discord](https://miren.dev/discord) or open an issue on [GitHub](https://github.com/mirendev/runtime/issues).

## Current Labs Features

Individual labs features are documented alongside their related functionality. Look for the "Labs Feature" callout in the docs.
