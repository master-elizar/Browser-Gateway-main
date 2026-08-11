# Stage 4 — Authentication

## Delivered

- Local users with **argon2id** password hashes
- JWT **access** + rotating **refresh** tokens (refresh hashes stored in Postgres)
- Endpoints: `register`, `login`, `refresh`, `logout`, `me`
- First user / bootstrap env → `SUPER_ADMIN`
- Public registration closed after first user (`allowRegistration=false`)
- Auth middleware on protected routes
- Admin: `GET/POST /api/admin/users`
- Frontend login/register + admin users UI
- Audit events: `auth.register`, `auth.login`, `auth.refresh`

## Bootstrap (deployed)

```
BOOTSTRAP_ADMIN_EMAIL=admin@browser-gateway.local
BOOTSTRAP_ADMIN_PASSWORD=ChangeMeAdmin123!
```

Change after first login.

## Verify

```bash
curl -sk https://<host>/api/version
curl -sk -X POST https://<host>/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@browser-gateway.local","password":"ChangeMeAdmin123!"}'
```
