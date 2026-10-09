# HTTP API

**English** | [Русский](../ru/api.md)

The REST API is served under `/api`. Interactive Swagger documentation is available
at `/swagger/` on your TaskExec instance. [api-docs.yml](../../api-docs.yml) is the source
specification and is bundled into every frontend build.

```sh
curl http://localhost:3000/api/ping
curl -H "Authorization: Bearer $TASKEXEC_API_TOKEN" http://localhost:3000/api/projects
```

Create API tokens in your user profile. Session authentication uses `POST /api/auth/login`
with JSON `{"auth":"username","password":"password"}` and the returned cookie.
Do not commit tokens or include credentials in bug reports.
