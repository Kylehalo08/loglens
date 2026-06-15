# LogLens VM Operations Cheatsheet

Quick reference for GCP VM stop/start, service wake-up, and deploying local changes.

**Domain (DNS later):** `madhavmaheshwaricreations.in`  
**VM name:** `loglens` | **Zone:** `us-central1-a`  
**Repo on VM:** `/opt/loglens`  
**External IP:** check GCP after every Start (ephemeral — it changes)

---

## URLs (no DNS)

| Service   | URL |
|-----------|-----|
| API       | `http://EXTERNAL_IP:8080` |
| Ingestor  | `http://EXTERNAL_IP:8081` |
| API health| `http://EXTERNAL_IP:8080/health` |

**With DNS (later):**
- `https://api.madhavmaheshwaricreations.in`
- `https://ingest.madhavmaheshwaricreations.in`

---

## 1. Stop VM (not using)

**GCP Console:** Compute Engine → VM instances → select `loglens` → **Stop**

No SSH needed. Data is kept. External IP is released (new IP on next Start).

---

## 2. Start VM (before demo / interview)

**GCP Console:** Compute Engine → VM instances → select `loglens` → **Start**

1. Wait 1–2 min for green checkmark  
2. Copy **External IP** from the table  
3. SSH: **Open in browser window**

---

## 3. Wake up LogLens (after VM Start)

### Quick check (try first)

```bash
curl -s http://127.0.0.1:8080/health
curl -s http://127.0.0.1:8081/health
```

Both should return `{"success":true,...}`. If yes, skip to **Test from Mac**.

### Full wake-up (if health fails)

```bash
cd /opt/loglens/deploy
docker compose -f docker-compose.prod.yml --env-file .env up -d
sleep 15
docker exec loglens-postgres-1 pg_isready -U loglens -d loglens

sudo systemctl start loglens-consumer loglens-api loglens-ingestor
sudo systemctl reload caddy

curl -s http://127.0.0.1:8080/health
curl -s http://127.0.0.1:8081/health
docker compose -f docker-compose.prod.yml ps
```

### Test from Mac

```bash
curl http://EXTERNAL_IP:8080/health
curl http://EXTERNAL_IP:8081/health
```

---

## 4. Push local changes → VM

```
Mac (edit) → git push → GitHub → git pull on VM → rebuild → restart
```

### On Mac

```bash
cd /Users/madhavmaheshwari/loglens
git add .
git commit -m "your message"
git push origin main
```

### On VM (SSH)

```bash
cd /opt/loglens
sudo git pull origin main

# Rebuild (15–30 min on e2-micro)
export PATH=/usr/local/go/bin:$PATH
sudo chown -R $USER:$USER /opt/loglens/bin 2>/dev/null || sudo mkdir -p /opt/loglens/bin && sudo chown $USER:$USER /opt/loglens/bin

go build -buildvcs=false -ldflags="-s -w" -o /opt/loglens/bin/loglens-api ./cmd/api
go build -buildvcs=false -ldflags="-s -w" -o /opt/loglens/bin/loglens-ingestor ./cmd/ingestor
go build -buildvcs=false -ldflags="-s -w" -o /opt/loglens/bin/loglens-consumer ./cmd/consumer
sudo chown loglens:loglens /opt/loglens/bin/loglens-*

sudo systemctl restart loglens-api loglens-ingestor loglens-consumer
curl -s http://127.0.0.1:8080/health
```

### If you changed…

| Changed | Extra command on VM |
|---------|---------------------|
| SQL migrations | `cd /opt/loglens/deploy && ./migrate.sh` |
| docker-compose.prod.yml | `docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d` |
| Caddyfile | `sudo cp deploy/Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy` |
| CORS origins | edit `CORS_ALLOWED_ORIGINS` in `/etc/loglens/env`, then `sudo systemctl restart loglens-api loglens-ingestor` |
| systemd units | `sudo cp deploy/systemd/*.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl restart loglens-*` |
| env secrets | edit `/etc/loglens/env` and `deploy/.env`, then restart services |

### git pull asks for password

Use GitHub **token** (not account password), or make repo public.

---

## 5. Demo API flow (curl)

Replace `EXTERNAL_IP` and `TOKEN` / IDs from responses.

```bash
# Register
curl -s -X POST http://EXTERNAL_IP:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"DemoPass123!","name":"Demo User"}'

# Login
curl -s -X POST http://EXTERNAL_IP:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"DemoPass123!"}'

# Create org
curl -s -X POST http://EXTERNAL_IP:8080/orgs \
  -H "Authorization: Bearer TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Demo Org"}'
```

---

## 6. Hostinger DNS (when ready)

Switch nameservers to **Hostinger** (not dns-parking.com), then add A records → VM External IP:

| Type | Name | Value |
|------|------|-------|
| A | api | EXTERNAL_IP |
| A | ingest | EXTERNAL_IP |
| A | @ | EXTERNAL_IP |
| A | www | EXTERNAL_IP |

Then on VM: `sudo systemctl reload caddy`

Check: `dig +short api.madhavmaheshwaricreations.in`

---

## 7. Important paths & secrets

| What | Path |
|------|------|
| App code | `/opt/loglens` |
| Deploy config | `/opt/loglens/deploy/` |
| Binaries | `/opt/loglens/bin/` |
| Docker env | `/opt/loglens/deploy/.env` (POSTGRES_PASSWORD) |
| App env | `/etc/loglens/env` (never commit) |

---

## 8. Troubleshooting

| Problem | Fix |
|---------|-----|
| Mac can't reach API | GCP firewall: TCP 8080,8081, targets = **All instances** |
| Localhost health OK, Mac timeout | Fix GCP firewall rule |
| Service won't start | `sudo journalctl -u loglens-api -n 40 --no-pager` |
| Postgres auth error | Password in `DATABASE_URL` must match `deploy/.env`; try `%3D` for trailing `=` |
| git pull permission | `sudo git pull` or fix ownership |
| build permission | `sudo chown $USER:$USER /opt/loglens/bin` |
| Out of memory on build | One binary at a time; swap is at `/swapfile` |

### Useful commands

```bash
docker compose -f /opt/loglens/deploy/docker-compose.prod.yml ps
sudo systemctl status loglens-api loglens-ingestor loglens-consumer
sudo journalctl -u loglens-api -f
df -h /
free -h
curl -s ifconfig.me   # show external IP from VM
```

---

## 9. Interview day checklist

- [ ] GCP → **Start** VM  
- [ ] Copy **External IP**  
- [ ] SSH → `curl http://127.0.0.1:8080/health`  
- [ ] Mac → `curl http://EXTERNAL_IP:8080/health`  
- [ ] Register/login test once  
- [ ] After interview → GCP → **Stop** VM  

---

## 10. What auto-starts on boot

| Component | Auto? |
|-----------|-------|
| Docker | Yes |
| Postgres, Redis, Kafka | Yes (`restart: unless-stopped`) |
| loglens-api, ingestor, consumer | Yes (`systemctl enable`) |
| Caddy | Yes |

Usually: Start VM → wait 2 min → health curl is enough.
