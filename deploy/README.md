# One-Droplet deployment

This runbook deploys the proven MVP to one DigitalOcean Droplet with PostgreSQL in Docker Compose, Caddy HTTPS, and media in one private DigitalOcean Spaces bucket.

## Before deploying

Manually create and verify:

- One Ubuntu Droplet with Docker Engine and the Docker Compose plugin.
- One domain or subdomain with an `A`/`AAAA` record pointing to the Droplet.
- A DigitalOcean Cloud Firewall allowing SSH from trusted addresses and public TCP 80/443 plus UDP 443. Do not expose PostgreSQL or the API port.
- One private Spaces bucket in the closest region to users.
- Least-privilege Spaces credentials for that bucket.
- Billing and resource alerts.

Configure Spaces CORS to allow the production origin to use `PUT`, `GET`, and `HEAD`, allow the `Content-Type` and `Range` headers, and expose `ETag`, `Accept-Ranges`, `Content-Length`, and `Content-Range`. Do not enable public listing or anonymous object access.

## Configure the server

Clone the private repository on the Droplet, then:

```bash
cd bjj-streaming
cp deploy/.env.production.example deploy/.env.production
chmod 600 deploy/.env.production
editor deploy/.env.production
```

Replace every placeholder. `POSTGRES_PASSWORD` must be strong. Its URL-encoded form must also appear in `DATABASE_URL`. Keep this file only on the server; never commit or paste it into prompts or issue trackers.

Validate and start:

```bash
docker compose --env-file deploy/.env.production -f deploy/compose.production.yaml config --quiet
docker compose --env-file deploy/.env.production -f deploy/compose.production.yaml up -d --build --wait
docker compose --env-file deploy/.env.production -f deploy/compose.production.yaml ps
```

Bootstrap the first administrator once:

```bash
docker compose --env-file deploy/.env.production -f deploy/compose.production.yaml run --rm --entrypoint /bootstrap-admin api --email ADMIN@example.com
```

Verify externally:

```bash
curl --fail https://YOUR_DOMAIN/healthz
curl --fail https://YOUR_DOMAIN/readyz
```

Inspect browser cookies and network traffic. The session cookie must be Secure and HTTP-only, object uploads and playback must use short-lived signed Spaces URLs, and no Spaces credential may reach the browser.

## Operations

View logs without printing environment values:

```bash
docker compose --env-file deploy/.env.production -f deploy/compose.production.yaml logs --tail=200 api caddy db
```

Deploy an update:

```bash
git pull --ff-only
docker compose --env-file deploy/.env.production -f deploy/compose.production.yaml build
docker compose --env-file deploy/.env.production -f deploy/compose.production.yaml up -d --wait
curl --fail https://YOUR_DOMAIN/readyz
```

Migrations run before the API starts. Never edit an applied migration. Review new migrations and take a backup before updating.

## Backup

Create a database backup:

```bash
deploy/backup.sh
```

The command prints a mode-`0600` custom-format dump path under `deploy/backups/`. Copy backups to a separate encrypted location; a backup stored only on the Droplet does not protect against Droplet loss. Spaces objects require a separate retention/copy policy from database backups.

Example daily cron entry:

```text
15 3 * * * cd /srv/bjj-streaming && ./deploy/backup.sh >> /var/log/bjj-backup.log 2>&1
```

Add an explicit retention policy appropriate for the pilot. Do not silently delete backups until that policy is approved.

## Restore

Restoration replaces database contents and briefly stops the API and web containers. Test it first on a separate Droplet or disposable Compose project.

```bash
deploy/restore.sh --confirm deploy/backups/postgres-YYYYMMDDTHHMMSSZ.dump
curl --fail https://YOUR_DOMAIN/readyz
```

After restoration, log in and inspect representative users, videos, progress, and notes. Confirm referenced Spaces objects still exist. A successful script exit alone is not proof of a useful restore.

## Monitoring

For the pilot, use a simple external HTTPS monitor against:

```text
https://YOUR_DOMAIN/healthz
https://YOUR_DOMAIN/readyz
```

Alert on non-`200` responses and certificate expiry. On the Droplet, monitor disk usage, memory, container restart counts, PostgreSQL volume growth, backup success and age, and DigitalOcean bandwidth/storage charges.

Useful checks:

```bash
docker compose --env-file deploy/.env.production -f deploy/compose.production.yaml ps
docker stats --no-stream
df -h
du -sh deploy/backups
```

The login limiter is in process and resets whenever the API restarts. This remains an accepted limitation only for the single-Droplet MVP.

## Required acceptance checks

- Site works over HTTPS from outside the Droplet.
- Only intended ports are reachable; PostgreSQL and API port 8080 are not public.
- Spaces listing and anonymous object reads fail.
- Admin, instructor, and two student pilot accounts complete the core workflow.
- A real database backup is restored and representative data is inspected.
- Uploaded test videos are unquestionably authorized.

Do not call Milestone 4 complete until these checks are performed manually.
