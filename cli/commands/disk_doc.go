package commands

const diskSectionDescription = `Commands for managing Miren disks. These commands are primarily used for troubleshooting and advanced operations.

### Disk Status Values

| Status | Description |
|--------|-------------|
| ` + "`" + `provisioning` + "`" + ` | Disk is being created and storage is being allocated |
| ` + "`" + `provisioned` + "`" + ` | Disk is ready and available for lease |
| ` + "`" + `attached` + "`" + ` | Disk has an active lease and is mounted |
| ` + "`" + `detached` + "`" + ` | Disk was previously attached but lease was released |
| ` + "`" + `deleting` + "`" + ` | Disk is marked for deletion |
| ` + "`" + `error` + "`" + ` | Disk encountered an error during provisioning |

### Lease Status Values

| Status | Description |
|--------|-------------|
| ` + "`" + `pending` + "`" + ` | Lease is waiting to acquire the disk |
| ` + "`" + `bound` + "`" + ` | Lease is active and disk is mounted |
| ` + "`" + `released` + "`" + ` | Lease has been released, cleanup pending |
| ` + "`" + `failed` + "`" + ` | Lease failed to acquire or mount the disk |

### Troubleshooting

**Disk stuck in "provisioning":**
Check server logs for storage backend errors:
` + "```" + `bash
miren debug disk status -i <disk-id>
` + "```" + `

**Lease stuck in "pending":**
The disk may not be provisioned yet, or another lease may have the disk:
` + "```" + `bash
miren debug disk lease-list -d <disk-id>
` + "```" + `

**App won't start due to disk timeout:**
Increase the ` + "`" + `lease_timeout` + "`" + ` in your app configuration, or check if another app has an active lease on the disk.`

const diskCreateDescription = `Disks are normally created automatically when referenced from an app.toml. This command exists to test manual disk creation only.`

const diskDeleteDescription = `:::warning
This is a dangerous command. Only disks without bound leases should be deleted. This marks the disk for deletion. The disk controller will clean up the underlying storage. Ensure no apps are using the disk before deletion.
:::`
