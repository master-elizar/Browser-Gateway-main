# Stage 8 — Minimal admin

## Delivered

- Admin users: list / create / patch (role, active, password) / delete
- Admin sessions: list all non-destroyed sessions + force stop
- Settings: GET/PUT limits, retention budget, audit toggles, registration flag
- Audit: paginated list with type/user/session/time filters
- Frontend admin pages wired (EN/RU)
- Version `0.8.0` / stage `8`

## Verify

1. Login as SUPER_ADMIN
2. Admin → Users: create a USER, deactivate, re-activate
3. Admin → Settings: change a limit, Save
4. Admin → Audit: see `auth.login` / `admin.settings.update`
5. Admin → Sessions: stop a running session if any
