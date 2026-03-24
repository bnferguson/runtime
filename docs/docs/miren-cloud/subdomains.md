
# Subdomains

Every Miren cluster needs a way for the outside world to reach it. You can always bring your own domain and point DNS at your server, but if you'd rather skip the DNS busywork, Miren Cloud can give you a subdomain that just works — claim a name like `mycluster.run.garden`, assign it to your cluster, and you're live.

## Available Base Domains

You can claim subdomains under two base domains:

| Domain | Notes |
|--------|-------|
| `run.garden` | Great for running your [garden server](https://miren.dev/blog/garden-server) |
| `miren.app` | `.app` TLD — browsers enforce HTTPS automatically |

Both come with wildcard DNS, so once you claim `mycluster.run.garden`, requests to `*.mycluster.run.garden` are routed to your cluster too. This is handy for giving each app its own hostname or building multi-tenant setups.

## Claiming a Subdomain

Head to your organization in [miren.cloud](https://miren.cloud), find the **Subdomains** section, and click **Claim Subdomain**.

![Claim subdomain dialog](/img/miren-cloud/claim-subdomain.png)

Names need to be 3–63 characters, lowercase alphanumeric with hyphens, and start/end with a letter or number. A handful of common names (`www`, `api`, `admin`, etc.) are reserved.

## Assigning to a Cluster

Once you've claimed a subdomain, click on it and choose **Assign to Cluster**. Pick one of your active clusters and Miren takes care of the rest — CNAME and wildcard DNS records are provisioned automatically. Propagation usually takes a few minutes.

## Routing Traffic to Your App

With the subdomain assigned to your cluster, the last step is telling Miren which app should handle the traffic:

```bash
miren route set mycluster.run.garden myapp
```

You can also route wildcard subdomains to an app:

```bash
miren route set '*.mycluster.run.garden' myapp
```

See [Traffic Routing](/traffic-routing) for more on how routes work.

## Good to Know

**HTTPS on `.app` domains** — The `.app` TLD is on browser HSTS preload lists, so all `.app` domains require HTTPS. Miren provisions TLS certificates automatically via [Let's Encrypt](/tls), so this works out of the box.

**Wildcard DNS** — When you assign `mycluster.run.garden`, Miren provisions both `mycluster.run.garden` and `*.mycluster.run.garden` pointing at your cluster. Your apps can handle sub-subdomains (e.g. for per-tenant routing) without any additional DNS setup.

**DNS propagation** — Records are provisioned automatically on assignment. It typically takes 1–5 minutes, though in rare cases it can take up to an hour.

**Limits** — During Developer Preview, each organization can claim up to 10 subdomains.
