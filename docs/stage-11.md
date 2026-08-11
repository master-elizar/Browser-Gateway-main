# Stage 11 — First-run setup + in-app updates

## Delivered

- Install prints a **one-time setup key** (no default admin password file)
- `/setup` creates the first SUPER_ADMIN with that key; login/register blocked until then
- Cannot delete / demote the last SUPER_ADMIN (already enforced)
- Settings → Updates: checks GitHub **releases** and **main** commit; **Check now** shows errors/loading
- Apply update writes `/opt/browser-gateway/data/update.requested`; systemd path unit runs `scripts/apply-update.sh`
- Host stores `/opt/browser-gateway/data/installed.commit` so commit diffs work after first update

## Ops

```bash
systemctl status browser-gateway-update.path
# emergency host update (bypasses UI):
sudo bash /opt/browser-gateway/scripts/apply-update.sh
```

## Note

Older 0.12.0 builds only understood GitHub Releases. Publish `v0.12.1+` so those installs can unlock **Update now**, then newer builds track `main` by commit SHA.

If **Update now** stays disabled with “waiting for host apply”, the marker at `/opt/browser-gateway/data/update.requested` is stuck (apply failed). Use **Clear pending** in the UI, or:

```bash
sudo rm -f /opt/browser-gateway/data/update.requested
sudo systemctl restart browser-gateway-update.path
sudo bash /opt/browser-gateway/scripts/apply-update.sh
```
