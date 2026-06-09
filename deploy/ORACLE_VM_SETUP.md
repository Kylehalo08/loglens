# LogLens production deploy on Oracle Cloud (free)

Deploy the full stack — Postgres, Redis, Redpanda (Kafka), API, ingestor, consumer — on one **Oracle Cloud Always Free** ARM VM with **Caddy** HTTPS and your Hostinger domain.

**Domain:** `madhavmaheshwaricreations.in`  
**URLs after deploy:**
- `https://api.madhavmaheshwaricreations.in` — main API + WebSocket
- `https://ingest.madhavmaheshwaricreations.in` — log ingestion
- `https://madhavmaheshwaricreations.in/health` — redirects to API health

---

## Part 1 — Oracle Cloud VM

### 1.1 Create account and VM

1. Sign up at [cloud.oracle.com](https://cloud.oracle.com) (credit card for verification; stay in Always Free).
2. **Compute → Instances → Create instance**
3. Settings:
   - **Name:** `loglens`
   - **Image:** Ubuntu 22.04 or 24.04
   - **Shape:** `VM.Standard.A1.Flex` (Ampere, Always Free)
   - **OCPU:** 2, **Memory:** 12 GB (plenty for demo; minimum ~4 GB works)
   - **Boot volume:** 50 GB
   - **Networking:** assign a **public IPv4**
   - **SSH key:** paste your public key (`cat ~/.ssh/id_ed25519.pub`)

4. Click **Create**.

5. Note the **public IP** (e.g. `150.136.x.x`).

### 1.2 Open firewall (Oracle + Ubuntu)

**Oracle VCN security list** (Networking → VCN → Security Lists → Ingress):

| Source | Protocol | Port |
|--------|----------|------|
| 0.0.0.0/0 | TCP | 22 |
| 0.0.0.0/0 | TCP | 80 |
| 0.0.0.0/0 | TCP | 443 |

**On the VM** (after SSH):

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y ufw
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw --force enable
```

### 1.3 SSH into the VM

```bash
ssh ubuntu@YOUR_VM_PUBLIC_IP
```

---

## Part 2 — Hostinger DNS

In Hostinger → Domains → `madhavmaheshwaricreations.in` → DNS:

| Type | Name | Points to | TTL |
|------|------|-----------|-----|
| A | `api` | `YOUR_VM_PUBLIC_IP` | 300 |
| A | `ingest` | `YOUR_VM_PUBLIC_IP` | 300 |
| A | `@` | `YOUR_VM_PUBLIC_IP` | 300 |
| A | `www` | `YOUR_VM_PUBLIC_IP` | 300 |

Wait 5–30 minutes for propagation. Check:

```bash
dig +short api.madhavmaheshwaricreations.in
```

---

## Part 3 — Install dependencies on VM

```bash
# Docker
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker ubuntu

# Docker Compose plugin (usually included with get.docker.com)
docker compose version

# Go 1.22+
sudo apt install -y golang-go git
go version   # if < 1.22, install from https://go.dev/dl/

# Caddy
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install -y caddy
```

Log out and back in so `docker` group applies:

```bash
exit
ssh ubuntu@YOUR_VM_PUBLIC_IP
```

---

## Part 4 — Deploy LogLens

### 4.1 Clone and layout

```bash
sudo mkdir -p /opt/loglens /etc/loglens
sudo git clone https://github.com/Kylehalo08/loglens.git /opt/loglens
sudo chown -R ubuntu:ubuntu /opt/loglens
cd /opt/loglens
```

If you use a private fork, clone that URL instead.

### 4.2 Create `loglens` system user

```bash
sudo useradd --system --home /opt/loglens --shell /usr/sbin/nologin loglens
sudo chown -R loglens:loglens /opt/loglens
```

### 4.3 Secrets and env files

Generate passwords:

```bash
openssl rand -base64 32   # POSTGRES_PASSWORD + DATABASE_URL
openssl rand -base64 48   # JWT_SECRET
```

**Docker compose env** (`deploy/.env`):

```bash
cat > /opt/loglens/deploy/.env <<'EOF'
POSTGRES_PASSWORD=PASTE_POSTGRES_PASSWORD_HERE
EOF
chmod 600 /opt/loglens/deploy/.env
```

**App env** (`/etc/loglens/env`):

```bash
sudo cp /opt/loglens/deploy/env.production.example /etc/loglens/env
sudo nano /etc/loglens/env
```

Set:
- `POSTGRES_PASSWORD` (same as deploy/.env)
- `DATABASE_URL=postgres://loglens:YOUR_PASSWORD@127.0.0.1:5432/loglens?sslmode=disable`
- `JWT_SECRET` (long random string)

```bash
sudo chmod 600 /etc/loglens/env
sudo chown root:root /etc/loglens/env
```

### 4.4 Start infrastructure

```bash
cd /opt/loglens/deploy
docker compose -f docker-compose.prod.yml --env-file .env up -d
docker compose -f docker-compose.prod.yml ps
```

Wait until Postgres is healthy:

```bash
docker exec loglens-postgres-1 pg_isready -U loglens -d loglens
```

### 4.5 Run migrations

```bash
chmod +x /opt/loglens/deploy/migrate.sh
/opt/loglens/deploy/migrate.sh
```

### 4.6 Build Go binaries

```bash
chmod +x /opt/loglens/deploy/build.sh
/opt/loglens/deploy/build.sh /opt/loglens/bin
sudo chown loglens:loglens /opt/loglens/bin/loglens-*
```

### 4.7 Install systemd units

```bash
sudo cp /opt/loglens/deploy/systemd/*.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now loglens-consumer loglens-api loglens-ingestor
sudo systemctl status loglens-api loglens-ingestor loglens-consumer
```

### 4.8 Configure Caddy (HTTPS)

```bash
sudo cp /opt/loglens/deploy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy
sudo systemctl status caddy
```

Caddy will obtain Let's Encrypt certificates automatically once DNS points to this VM.

---

## Part 5 — Verify

```bash
# Local (on VM)
curl -s http://127.0.0.1:8080/health
curl -s http://127.0.0.1:8081/health

# Public HTTPS (from your laptop)
curl -s https://api.madhavmaheshwaricreations.in/health
curl -s https://ingest.madhavmaheshwaricreations.in/health
```

Register a user:

```bash
curl -s -X POST https://api.madhavmaheshwaricreations.in/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"DemoPass123!","name":"Demo User"}'
```

---

## Part 6 — Interview demo flow

1. **Register / login** → get JWT access token  
2. **Create org** → `POST /orgs`  
3. **Create service** → `POST /orgs/:id/services`  
4. **Generate API key** → `POST .../api-keys`  
5. **Ingest log** → `POST https://ingest.../v1/logs` with `Authorization: Bearer <api_key>`  
6. **Search** → `GET /orgs/:id/logs/search?severity=ERROR`  
7. **Live stream** → WebSocket `wss://api.../orgs/:id/services/:serviceId/logs/stream`

SDK ingest URL env:

```bash
export INGESTOR_URL=https://ingest.madhavmaheshwaricreations.in
```

---

## Operations cheat sheet

| Task | Command |
|------|---------|
| View API logs | `sudo journalctl -u loglens-api -f` |
| Restart API | `sudo systemctl restart loglens-api` |
| Restart infra | `cd /opt/loglens/deploy && docker compose -f docker-compose.prod.yml restart` |
| Pull updates | `cd /opt/loglens && sudo -u loglens git pull && ./deploy/build.sh /opt/loglens/bin && sudo systemctl restart loglens-api loglens-ingestor loglens-consumer` |
| Disk usage | `docker system df` |

---

## Troubleshooting

**Caddy won't get SSL cert**
- DNS not propagated → wait, then `sudo systemctl reload caddy`
- Port 80 blocked → check Oracle security list + `sudo ufw status`

**Kafka connection refused**
- Ensure `KAFKA_BROKERS=127.0.0.1:9092` in `/etc/loglens/env`
- `docker compose -f deploy/docker-compose.prod.yml logs kafka`

**API starts before Postgres**
- `sudo systemctl restart loglens-api` after docker is healthy

**Out of memory**
- Lower Redpanda memory in `docker-compose.prod.yml` (`512M`)
- Reduce VM services or use 6 GB RAM shape

---

## Security notes (demo)

- Postgres / Redis / Kafka are bound to `127.0.0.1` only
- Change default passwords before sharing the demo URL widely
- For a public demo, consider rate limiting at Caddy or disabling open registration
