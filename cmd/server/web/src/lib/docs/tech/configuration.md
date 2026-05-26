# Configuration

Damask is configured via environment variables. All variables are optional unless marked **required**.

## Core

| Variable     | Default        | Description                                                    |
| ------------ | -------------- | -------------------------------------------------------------- |
| `DATA_DIR`   | `./data`       | Directory for the SQLite database and local file storage       |
| `BASE_URL`   | **required**   | Public URL of your instance (e.g. `https://dam.example.com`)   |
| `PORT`       | `8080`         | HTTP listen port                                               |
| `JWT_SECRET` | auto-generated | HMAC secret for JWT signing - set a stable value in production |

## Registration

| Variable             | Default | Description                             |
| -------------------- | ------- | --------------------------------------- |
| `ALLOW_REGISTRATION` | `true`  | Allow new users to self-register        |
| `INVITE_ONLY`        | `false` | Require an invitation token to register |

## Email (SMTP)

| Variable        | Default | Description                    |
| --------------- | ------- | ------------------------------ |
| `SMTP_HOST`     | -       | SMTP server hostname           |
| `SMTP_PORT`     | `587`   | SMTP port                      |
| `SMTP_USER`     | -       | SMTP username                  |
| `SMTP_PASSWORD` | -       | SMTP password                  |
| `SMTP_FROM`     | -       | From address for outgoing mail |

## Storage

See the [Storage guide](storage) for S3 and SFTP options.

## Telemetry

Damask collects anonymous usage statistics by default. Set `TELEMETRY=false` to opt out.
