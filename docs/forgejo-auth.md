# Forgejo Auth Contract

This document records the working Forgejo login path for the homelab stack. Do
not change this contract casually; when login breaks, verify each layer before
changing Authentik, Forgejo, or Google configuration.

## Working Path

Forgejo uses Authentik as its native OIDC provider.

- Public service URL: `https://git.caboose-ai.io`
- Forgejo login source name: `Authentik`
- Authentik provider/application slug: `forgejo`
- Authentik discovery URL:
  `https://auth.caboose-ai.io/application/o/forgejo/.well-known/openid-configuration`
- Forgejo callback URL:
  `https://git.caboose-ai.io/user/oauth2/Authentik/callback`
- OIDC scopes: `openid`, `email`, `profile`
- Authentik provider `sub_mode`: `hashed_user_id`

Google is only an upstream Authentik source. Forgejo should never talk to
Google directly in this stack. The intended login chain is:

1. Browser opens Forgejo.
2. Forgejo redirects to Authentik provider `forgejo`.
3. Authentik authenticates the user, including via Google when selected.
4. Authentik returns an OIDC callback to Forgejo.
5. Forgejo maps the Authentik OIDC subject to the existing local Forgejo user.

## Known Good User Mapping

The current working admin identity is:

- Authentik username: `auth-admin`
- Authentik email: `cxm6467@gmail.com`
- Forgejo username: `auth-admin`
- Forgejo user id: `1`
- Forgejo login source id: `1`
- Authentik OIDC subject:
  `a461b7525880bc529488cc7e62c2ed6b335f046316d90983ced28179faf47d5f`

Forgejo must have this row in `external_login_user`:

```sql
external_id                                                       user_id  login_source_id  provider       email              name        nick_name
----------------------------------------------------------------  -------  ---------------  -------------  -----------------  ----------  ----------
a461b7525880bc529488cc7e62c2ed6b335f046316d90983ced28179faf47d5f  1        1                openidConnect  cxm6467@gmail.com  auth-admin  auth-admin
```

If Authentik is reset and the user's hashed subject changes, recompute the
subject from Authentik and update this mapping intentionally. Do not change
Forgejo to a different OIDC provider, do not point Forgejo directly at Google,
and do not rename the login source unless the installer and smoke tests are
updated at the same time.

## Symptom Triage

`/user/link_account` after the Authentik callback means Authentik login
succeeded and Forgejo could not map the returned OIDC subject to a local
account. This is a Forgejo account-linking issue, not a Google OAuth issue.

Check the Forgejo callback path:

```sh
docker logs --tail 200 forgejo
```

Expected broken pattern before the mapping exists:

```text
GET /user/oauth2/Authentik/callback ... 303 See Other
GET /user/link_account ... 200 OK
```

Check the Forgejo local user:

```sh
docker exec forgejo sh -lc 'sqlite3 -header -column /data/gitea/gitea.db "select id, lower_name, name, email, is_active, login_type, login_source, login_name from user;"'
```

Check the Forgejo OIDC source:

```sh
docker exec forgejo sh -lc 'sqlite3 -header -column /data/gitea/gitea.db "select id, type, name, is_active from login_source;"'
```

Check the external-login binding:

```sh
docker exec forgejo sh -lc 'sqlite3 -header -column /data/gitea/gitea.db "select external_id, user_id, login_source_id, provider, email, name, nick_name from external_login_user;"'
```

Check Authentik's current subject for `auth-admin`:

```sh
docker exec authentik-server sh -lc 'PATH=/ak-root/.venv/bin:$PATH /lifecycle/ak shell -c "from authentik.core.models import User; u=User.objects.get(username=\"auth-admin\"); print(u.uid)"'
```

Check Authentik's provider mode:

```sh
docker exec authentik-server sh -lc 'PATH=/ak-root/.venv/bin:$PATH /lifecycle/ak shell -c "from authentik.providers.oauth2.models import OAuth2Provider; p=OAuth2Provider.objects.get(name=\"forgejo\"); print(p.sub_mode)"'
```

Expected value:

```text
hashed_user_id
```

## Repair Command

Only run this when the subject has been verified from Authentik and the local
Forgejo account is the intended target.

```sh
docker exec forgejo sh -lc "sqlite3 /data/gitea/gitea.db \"begin; insert or replace into external_login_user (external_id, user_id, login_source_id, provider, email, name, nick_name) values ('a461b7525880bc529488cc7e62c2ed6b335f046316d90983ced28179faf47d5f', 1, 1, 'openidConnect', 'cxm6467@gmail.com', 'auth-admin', 'auth-admin'); commit;\""
```

Verify:

```sh
docker exec forgejo sh -lc "sqlite3 -header -column /data/gitea/gitea.db \"select e.external_id, e.login_source_id, u.id, u.name, u.email, u.is_active from external_login_user e join user u on u.id=e.user_id where e.external_id='a461b7525880bc529488cc7e62c2ed6b335f046316d90983ced28179faf47d5f' and e.login_source_id=1;\""
```

Retry login from a clean browser tab:

```text
https://git.caboose-ai.io/user/oauth2/Authentik
```

## Change Rules

- Keep the Forgejo login source named `Authentik`.
- Keep Forgejo using the Authentik `forgejo` OIDC provider.
- Keep Google as an upstream Authentik source only.
- Treat `/user/link_account` as a missing Forgejo external-login mapping unless
  logs prove otherwise.
- Verify with logs and database state before changing installer code.
- If this contract changes, update this document, the Forgejo configurator, and
  browser smoke-test expectations in the same change.
