---
outline: deep
---

# Multi-Workspace & Team-Ready

Damask is multi-tenant from day one. Whether you run a single personal instance or a shared server for a studio team, every workspace is fully isolated - its own projects, assets, fields, share links, and ingestion sources.

## Workspaces

A workspace is the top-level container for everything in Damask. All data belongs to a workspace: assets, projects, folders, tags, field definitions, share links, ingestion sources, and activity logs.

When you sign up, a workspace is created for you automatically. If you're running a self-hosted instance for a team, the same server can host multiple workspaces - each invisible to the others.

### What is isolated per workspace

| Data | Isolated |
|------|----------|
| Assets and files | ✓ |
| Projects and folders | ✓ |
| Tags | ✓ |
| Custom field definitions | ✓ |
| Share links | ✓ |
| Ingestion sources | ✓ |
| Activity log | ✓ |
| Members and roles | ✓ |

No data crosses workspace boundaries. A user who is a member of two workspaces sees them as completely separate environments.

## Members and roles

### Inviting a team member

Go to **Settings → Members** and click **Invite member**. Enter their email address. Damask sends them an invite link valid for 48 hours.

When they accept, they're added to your workspace with the role you specified. They create a password during the acceptance flow.

### Roles

| Role | What they can do |
|------|-----------------|
| **Owner** | Everything, including deleting the workspace, managing members, and changing billing settings |
| **Editor** | Upload assets, create projects, manage tags, create share links, configure ingestion sources |
| **Viewer** | Browse the library, view assets and metadata, download files. Cannot upload, create, or delete anything |

Role changes take effect immediately. A session already in progress picks up the new role on the next request.

### Removing a member

Go to **Settings → Members** and click **Remove** next to a member. Their session is invalidated immediately. Assets they uploaded remain - they are not deleted with the user.

There must always be at least one **owner** in a workspace. You cannot remove the last owner, or demote yourself from owner if you are the last one.

## Workspace settings

### Workspace name and icon

Go to **Settings → Workspace** to update the workspace name. You can also set a workspace icon by selecting any asset from your library - the icon is version-pinned (a new file version won't silently change the icon).

### Version retention

Configure how many versions to keep per asset (or keep all versions indefinitely). See [Version History](/version-history-audit-log) for details.

### Activity log retention

Configure how many days of activity log events to retain. Default is 365 days. Download events are retained for 30 days by default (they're high-volume and have lower audit value than structural changes).

## Workspace isolation in the API

Every API endpoint is scoped to the authenticated user's workspace. The workspace is read from the Paseto token claim - it is not a URL parameter or a request header.

This means:
- A user cannot access another workspace's assets even if they know an asset ID
- API tokens are workspace-bound by default
- There is no "super-admin" role that can see across workspaces (the server admin TUI is the separate tool for that)

## The admin TUI

For server administrators running a self-hosted Damask instance, a separate `damask-admin` CLI is included in the release. It connects directly to the SQLite database in read-only mode and provides:

- User and workspace overview
- Storage breakdown per workspace
- Recent activity feed
- Job queue health

This is a monitoring tool, not a data management tool. It does not write to the database. Run it on the same machine as the Damask server:

```bash
damask-admin --db /path/to/damask.db
```

See the [Admin TUI](/) documentation for the full keyboard reference.

## Multi-instance deployment

If you need to run Damask for multiple fully separate teams with no shared infrastructure, simply run separate instances - each with its own binary, its own `damask.db`, and its own storage directory. There is no clustering or shared-database mode.

This is often the simplest and most appropriate architecture for an agency running separate instances for different clients.
