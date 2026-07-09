---
title: Miren Cloud
description: Central control plane for managing Miren clusters — team access, backups, multi-environment workflows, and custom subdomains.
keywords: [miren cloud, control plane, teams, rbac, backup, multi-environment, organization]
---

# Miren Cloud

Miren Cloud is a central control plane that connects and manages your Miren clusters. While Miren runs fully standalone on your own infrastructure, connecting to Miren Cloud gives you:
- Team management and access control
- Automatic data backup and sync
- Multi-environment workflows
- [Custom subdomains](/miren-cloud/subdomains) for your apps (e.g. `mycluster.run.garden`)

## Miren Server Installation (with Cloud)

When you run `miren server install`, it will automatically register a new cluster to Miren Cloud and redirect you to create your miren.cloud organization and account:

:::note
The install requires systemd at present.
:::

```bash
sudo miren server install
```

By default, you will have full access to your new cluster. Permissions can be tweaked using RBAC rules if needed.

### Miren Server Installation within a Container

If you're on a platform other than Linux (or a Linux platform without systemd available), you can install
the server into a container. This works with either Docker or Podman (auto-detected, or select one with
`--runtime`):

```bash
miren server container install
```

### Install Standalone

To skip cloud registration and run standalone:

```bash
sudo miren server install --without-cloud
```

## Login

Authenticate with miren.cloud:

```bash
miren login
```

This will open a browser window to complete authentication.

## Check Your Identity

See who you're logged in as:

```bash
miren whoami
```

## Register Your Cluster

Connect your local cluster to miren.cloud:

```bash
miren server register -n my-cluster
```

This registers your cluster and enables cloud features.

:::note
By default, servers are already registered when doing `miren server install`.
:::

## View Your Clusters

List all clusters associated with your account:

```bash
miren cluster list
```

## Switch Clusters

If you have multiple clusters, switch between them:

```bash
miren cluster switch my-other-cluster
```
