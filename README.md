# WindowsThemeTray

Утилита для Windows 10/11, которая живёт в системном трее и переключает тему Windows **Light ↔ Dark одним кликом**. Заходить в *Settings → Personalization → Colors* больше не нужно.

- **Левый клик** по иконке переключает тему.
- **Ctrl+Alt+D** переключает тему из любого приложения.
- **Правый клик** открывает меню: `☀ Light`, `🌙 Dark`, `Toggle theme`, `Start with Windows`, `Exit`.
- Иконка показывает текущий режим (солнце или луна), tooltip: `Windows Theme: Light` / `Windows Theme: Dark`.

Это один `.exe` размером около 1.9 МБ. Права администратора и какой-либо runtime не нужны, консольного окна нет.

## Сборка

Нужен Go 1.22+.

```powershell
go mod tidy
go build -ldflags="-H=windowsgui" -o WindowsThemeTray.exe .
```

Минимальный production-бинарник: без отладочной информации, без локальных путей внутри exe.

```powershell
go build -trimpath -ldflags "-H=windowsgui -s -w" -o WindowsThemeTray.exe .
```

То же самое делает скрипт (дополнительно запускает `go vet`):

```powershell
.\build.ps1          # сборка
.\build.ps1 -Icons   # перегенерировать иконки и собрать
```

> В Windows PowerShell 5.1 лучше писать `-ldflags "..."` через пробел. Форма `-ldflags="... ..."` со значением, содержащим пробелы, иногда разбирается неправильно.

Иконки `assets/light.ico` и `assets/dark.ico` уже лежат в репозитории и встраиваются в exe через `go:embed`. Если вы поменяли рисунок в `tools/genicons`, перегенерируйте их командой `go generate` (или `go run ./tools/genicons`).

GitHub Actions (`.github/workflows/build.yml`) собирает exe на каждый push/PR и выкладывает его как artifact. Для тега `v*` создаётся релиз с приложенным exe.

## Автозапуск

Включается пунктом **Start with Windows** в меню. Программа пишет в

```
HKCU\Software\Microsoft\Windows\CurrentVersion\Run
    WindowsThemeTray = "C:\путь\к\WindowsThemeTray.exe"
```

Нюансы:

- Галочка стоит, только если запись указывает на **текущий** exe и не отключена в *Task Manager → Startup apps*. Если exe переместили, включите пункт заново, и путь обновится.
- Если автозапуск отключили в Task Manager, повторное включение через меню снимает этот запрет.
- Пока автозапуск включён, не перемещайте exe. Либо сначала выключите пункт, переместите файл и включите снова.

Вручную можно так: положить ярлык на exe в `shell:startup`.

## Как это работает

Тема хранится в `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`:

| Значение               | Что задаёт                         | 1     | 0    |
|------------------------|------------------------------------|-------|------|
| `AppsUseLightTheme`    | тема приложений                    | Light | Dark |
| `SystemUsesLightTheme` | панель задач, Пуск, Центр уведомлений | Light | Dark |

При переключении записываются оба значения. Затем рассылается `WM_SETTINGCHANGE` с параметром `"ImmersiveColorSet"`, как это делает приложение Settings, и открытые программы сразу перечитывают тему. Текущий режим определяется по `AppsUseLightTheme`. Если значения или ключа нет, считается Light (это значение Windows по умолчанию).

### Архитектура

```
main.go     single-instance mutex, DPI awareness, цикл сообщений
tray.go     скрытое окно, wndProc, иконка в трее, контекстное меню
theme.go    чтение/запись темы в реестре, broadcast WM_SETTINGCHANGE
startup.go  автозапуск (Run + StartupApproved)
hotkey.go   RegisterHotKey(Ctrl+Alt+D)
icon.go     встроенные .ico → HICON нужного размера
winapi.go   объявления WinAPI-функций, констант и структур
tools/genicons  генератор assets/*.ico (только stdlib)
```

Единственная зависимость — `golang.org/x/sys`. Трей реализован напрямую через `Shell_NotifyIconW`. Готовые библиотеки (`getlantern/systray` и форки) на Windows открывают меню и по левому клику, а глобальный hotkey всё равно требует собственного окна и цикла сообщений. Поэтому всё событийное — клики по иконке, `WM_HOTKEY`, `WM_SETTINGCHANGE`, `TaskbarCreated` — приходит в одно скрытое окно и обрабатывается в одном потоке, заблокированном через `runtime.LockOSThread`.

### Какие типичные проблемы трей-приложений учтены

- **Консоль**: exe собирается с `-H=windowsgui` (подсистема GUI).
- **Explorer перезапустился, иконка пропала**: программа ловит `TaskbarCreated` и добавляет иконку заново. Иконки при этом перезагружаются, потому что то же сообщение приходит после смены DPI.
- **Explorer ещё не готов при автозапуске**: если `NIM_ADD` не удался, программа повторяет попытку по таймеру раз в 2 секунды. Если `NIM_ADD` вернул таймаут, но иконка на самом деле добавилась, используется `NIM_MODIFY`.
- **Окно для broadcast-сообщений**: используется скрытое top-level окно, а не message-only, иначе `TaskbarCreated` и `WM_SETTINGCHANGE` не приходят.
- **Меню не закрывается кликом мимо**: перед `TrackPopupMenuEx` вызывается `SetForegroundWindow`, после него отправляется `WM_NULL`.
- **Нет tooltip в NOTIFYICON_VERSION_4**: выставлен `NIF_SHOWTIP`.
- **Иконку привязывают к GUID**: программа идентифицирует иконку по `uID`. При GUID `Shell_NotifyIcon` отказывает, если exe перемещён.
- **Размытая иконка на HiDPI**: процесс объявлен Per-Monitor-V2 DPI aware, ICO содержит размеры 16–256 px, и выбирается подходящий под `SM_CXSMICON`.
- **Тему поменяли в Settings**: по `WM_SETTINGCHANGE`/`ImmersiveColorSet` иконка синхронизируется с реестром. Перед каждым переключением реестр перечитывается заново.
- **Hotkey занят другим приложением**: появляется уведомление, а клик по иконке продолжает работать.
- **Второй экземпляр**: именованный mutex `Local\WindowsThemeTray-SingleInstance`, отдельный на каждую пользовательскую сессию. Второй экземпляр завершается сразу.
- **Корректный выход**: `Exit` → `DestroyWindow` → в `WM_DESTROY` удаляются иконка и hotkey, вызывается `PostQuitMessage`. После выхода в трее не остаётся «призрачной» иконки.

### Ограничения

- Двойной клик — это два клика, поэтому тема переключится дважды.
- `Ctrl+Alt` — это то же, что AltGr на некоторых раскладках. Пока программа запущена, сочетание AltGr+D перехватывается.
- Контекстное меню всегда светлое: тёмные меню Win32 возможны только через недокументированный API uxtheme, а он здесь сознательно не используется.
