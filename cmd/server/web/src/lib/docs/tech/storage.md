# Storage

Damask supports three storage backends for uploaded files.

## Local disk (default)

Files are stored under `DATA_DIR/files/`. No additional configuration needed.

Suitable for single-host deployments. For multi-replica or cloud deployments, use S3.

## S3-compatible

Works with AWS S3, MinIO, Backblaze B2, Cloudflare R2, and any S3-compatible service.

```
STORAGE_BACKEND=s3
S3_ENDPOINT=https://s3.amazonaws.com   # or your compatible endpoint
S3_BUCKET=my-damask-assets
S3_REGION=us-east-1
S3_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE
S3_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

For public-read buckets, set `S3_PUBLIC_URL` to a CDN URL - Damask will redirect thumbnail and file requests there instead of proxying through the server.

## SFTP

```
STORAGE_BACKEND=sftp
SFTP_HOST=files.example.com
SFTP_PORT=22
SFTP_USER=damask
SFTP_KEY=/run/secrets/sftp_key   # path to private key file
SFTP_ROOT=/uploads
```

## Migrations between backends

Damask does not migrate existing files between backends automatically. To switch:

1. Copy all files to the new backend manually
2. Update environment variables
3. Restart - new uploads go to the new backend; old links still resolve (if the old files are accessible)
