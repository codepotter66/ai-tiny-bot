# Deploy quickref

> Mac → server one-shot deploy. You should not need to log into the server by hand.

[中文文档](README.zh-CN.md)

## Most common: release

```bash
make deploy
```

Local build → save tar → scp → remote update (**default HTTPS self-signed on 443**) → health check.

**workspace**: writable in the container (memory). On redeploy, **live files win**; the package only adds paths missing on the server (e.g. new skills), so online memory / hand-edited persona are kept.

**Online editor**: `https://<server-ip>/demo/workspace.html` (set `TB_WORKSPACE_EDITOR_TOKEN` in `.env`; same token in the page).

After deploy, open on phone: **`https://<server-ip>/demo/`** (trust cert once; open 443 in the security group).

## Ops (no SSH shell required)

| Command | What it does |
|---|---|
| `make deploy` | Release (build + scp + remote apply + health) |
| `make seed-remote` | Register device in the remote agent container |
| `make seed` | Local DB register (local only) |
| `make deploy-health` | Curl remote `/healthz` |
| `make deploy-status` | Remote `docker compose ps` |
| `make deploy-logs` | Remote `docker compose logs -f` |
| `make deploy-restart` | Recreate agent (reload env) |
| `make deploy-stop` | Stop services |

```bash
make seed-remote
make seed-remote DEVICE_ID=tinypal-esp32-01 PAIRING_CODE=ABCD-1234
```

Then reset the ESP32; firmware `config.h` must match `TB_DEVICE_ID` / `TB_PAIRING_CODE`.

## Config

Required in `.env` (no hard-coded defaults in repo):

```bash
DEPLOY_SERVER=user@your.server.ip           # required: SSH target
DEPLOY_SERVER_DIR=/path/to/deploy/root      # required: remote deploy root
TB_PUBLIC_IP=your.server.ip                 # required for https-selfsigned profile
```

## Pipeline

```
Mac: docker save → tar
    ↓ scp
    ↓ scp update-tiny-bot-cloud-agent.sh
    ↓ ssh
Server: update-tiny-bot-cloud-agent.sh --from-tar
      → extract files tar (workspace: live-first + append missing)
      → docker load
      → compose down / up
      → healthz
    ↓ exit code to Mac
```

## First-time prerequisites

1. Docker daemon registry mirror on the server if Hub is blocked (`deploy/daemon.json.cn.example`)
2. SSH key: `ssh-copy-id user@your.server.ip` (matches `DEPLOY_SERVER`)
3. Local `.env` with secrets + `DEPLOY_SERVER` / `DEPLOY_SERVER_DIR` / `TB_PUBLIC_IP`

## Troubleshooting

| Symptom | First step |
|---|---|
| `make deploy` stuck on passphrase | `ssh-copy-id user@your.server.ip` |
| Health check fails | `make deploy-logs` |
| Host unreachable | `make deploy-status` |
| Phone mic blocked | Cannot use plain `http://IP` — see HTTPS below |
| Nuke and redo | `make deploy-stop` then `make deploy` |

## Phone demo needs HTTPS

`getUserMedia` requires **HTTPS** or **localhost**. `http://public-ip:5678/demo/` disables the mic on phones.

### A. Cloudflare quick tunnel

```bash
cd "$DEPLOY_SERVER_DIR/tiny-bot-cloud-agent"
docker compose --profile tunnel up -d
docker compose logs cloudflared
```

Open the printed `https://….trycloudflare.com/demo/`. URL changes on restart.

### B. Self-signed HTTPS

1. Open **443**
2. `docker compose --profile https-selfsigned up -d`
3. Phone: `https://<server-ip>/demo/` (trust cert)

### C. Real domain

Set `TB_PUBLIC_DOMAIN` in `.env`, then `docker compose --profile https up -d`.

See [demo/README.md](../demo/README.md).

## More detail

[../docs/human-docs/05-deployment.md](../docs/human-docs/05-deployment.md) — daemon.json, networking, Dockerfile notes.
