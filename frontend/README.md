# frontend — заглушка будущего веб-клиента CFU-Board.
#
# API бэкенда задокументировано OpenAPI 3.0:
#   - интерактивно: http://localhost:8080/docs
#   - сырая спека:  http://localhost:8080/docs/openapi.yaml
#                   (файл: ../backend/docs/openapi.yaml)
#
# Рекомендуемый способ генерации типизированного клиента (TypeScript):
#   npx openapi-typescript http://localhost:8080/docs/openapi.yaml -o ./src/api/types.ts
#
# Когда здесь появится приложение, добавьте сервис в ../docker-compose.yml
# и удалите этот файл.
