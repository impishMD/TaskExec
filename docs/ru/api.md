# HTTP API

[English](../en/api.md) | **Русский**

REST API доступен по `/api`, интерактивный Swagger — по `/swagger/` на вашем экземпляре.
Спецификация [api-docs.yml](../../api-docs.yml) включается в каждую сборку интерфейса.

```sh
curl http://localhost:3000/api/ping
curl -H "Authorization: Bearer $TASKEXEC_API_TOKEN" http://localhost:3000/api/projects
```

Токен создаётся в профиле пользователя. Для сессии используйте `POST /api/auth/login`
с JSON `{"auth":"username","password":"password"}` и полученную cookie.
Не сохраняйте токены в Git и отчётах об ошибках.
