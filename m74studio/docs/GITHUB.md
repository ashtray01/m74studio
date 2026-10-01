# Публикация на GitHub

Корень Git-репозитория — папка над `m74studio`. Исходники приложения находятся в `m74studio`, workflow — в корневой `.github/workflows`.

## Первый push

Создайте пустой репозиторий на GitHub без автоматически добавленных README и `.gitignore`. Из корня этого проекта выполните, заменив `OWNER` своим именем или организацией:

```powershell
git remote add origin https://github.com/OWNER/m74studio.git
git push -u origin main
```

В GitHub откройте **Actions → Windows build & release**. Для обычной сборки достаточно стандартного `GITHUB_TOKEN`, который GitHub выдаёт автоматически. Секреты проекта не требуются. Если Actions отключены политикой организации, разрешите workflow в настройках репозитория.

## Что выполняет пайплайн

1. Windows Server 2022, Go из `m74studio/go.mod`, MinGW-w64 GCC из MSYS2.
2. Скачивание и проверка Go-модулей.
3. Компиляция ресурсов окна, тесты декодера и экспорта, `go vet`, сборка EXE.
4. Запуск собранного EXE без интерфейса: контрольный пакет → CSV, TXT и JSON. Этот тест работает без личного лога.
5. Упаковка EXE, инструкции, документации и лицензий; вычисление SHA-256.
6. Артефакт `M74Studio-windows-x64` хранится 30 дней.

На pull request токен имеет только `contents: read`. Права `contents: write` выдаются отдельной задаче публикации, которая запускается только после успешной сборки тега `v*`.

## Выпуск версии

Обновите версию интерфейса и `app.rc`, зафиксируйте изменения, затем создайте тег:

```powershell
git tag v1.0.0
git push origin v1.0.0
```

Пайплайн создаст GitHub Release с автоматически сформированными заметками и прикрепит ZIP и `SHA256SUMS.txt`. Повторный запуск того же тега обновит вложения существующего релиза. Сборка не подписана сертификатом Authenticode; Runtime WebView2 в ZIP не включается.

Проверить скачанный архив:

```powershell
Get-FileHash .\M74Studio-windows-x64.zip -Algorithm SHA256
```

Сравните значение с `SHA256SUMS.txt`.

## Локальные файлы

`.gitignore` исключает локальные файлы за пределами проекта, `.tools`, кэш, записи `.log`, экспорты и бинарные сборки. Полный контрольный лог используется локальным тестом при наличии; на GitHub этот тест пропускается, а переносимые проверки пакетов выполняются всегда.

Документация используемых Actions: [checkout](https://github.com/actions/checkout), [setup-go](https://github.com/actions/setup-go), [MSYS2](https://github.com/msys2/setup-msys2), [upload-artifact](https://github.com/actions/upload-artifact), [download-artifact](https://github.com/actions/download-artifact).
