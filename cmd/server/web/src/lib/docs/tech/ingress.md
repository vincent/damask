# Ingress sources

Ingress sources let Damask automatically pull assets from external systems on a schedule.

## Overview

Configure ingress sources at **Settings → Ingress**. Each source has:

- A **type** (Email, SFTP, S3, Google Drive, Canva, WebDAV)
- A **schedule** (cron expression or interval)
- **Rules** - filter and routing logic (which project/folder incoming files land in)
- A **log** - every ingest attempt is recorded with status and error details

## Email (IMAP)

Damask polls an IMAP mailbox and imports attachments.

```
Type: Email
Host: imap.example.com
Port: 993
User: assets@example.com
Password: ••••••••
```

Attachments from matching emails are ingested as assets.

## SFTP

Polls a remote SFTP directory for new files.

```
Type: SFTP
Host: files.example.com
User: damask
Key: (paste private key)
Path: /incoming
```

After ingestion, files are moved to a `processed/` subdirectory.

## S3

Polls an S3 bucket prefix for new objects.

```
Type: S3
Endpoint: https://s3.amazonaws.com
Bucket: client-uploads
Prefix: incoming/
Access Key / Secret Key: ••••
```

## Google Drive

Connect via **Settings → Integrations → Google Drive**. Then create an ingress source pointing at a specific folder ID.

## Canva

Connect via **Settings → Integrations → Canva**. Ingests exports from linked Canva designs.

## Ingress rules

Rules run in order on each incoming file. Each rule has:

- A **condition** (filename matches pattern, MIME type, sender address, etc.)
- An **action** (assign to project, assign to folder, add tag, skip)

Files that match no rule land in the workspace inbox (no project).

## Manual trigger

Click **Poll now** on any source to trigger an immediate ingest cycle outside the schedule.
