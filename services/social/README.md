# Social Login

Social login manages external GitHub and Google Authentik sources. It is represented as a service workspace because it has its own configurator and operational checks.

Google is the enabled login source and uses Authentik's Google provider icon plus email-link matching for existing users. It is intentionally unpromoted so Authentik renders the Google icon instead of a generic "Continue with Google" primary button. GitHub credentials can remain configured for future use, but the managed GitHub source is disabled and unpromoted by default so it is not offered as a login method. The default identification stage is source-only, with local email/password login hidden.
