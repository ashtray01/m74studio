# Сторонние компоненты

- [webview_go](https://github.com/webview/webview_go), commit `6173450d4dd6`, MIT. Go-обвязка встроенного окна.
- [webview](https://github.com/webview/webview), MIT. Включён в указанную версию webview_go.
- Microsoft WebView2 SDK headers из webview_go и Microsoft Edge WebView2 Runtime. Runtime не включён в поставку; используется установленный в Windows экземпляр. Лицензия SDK: `licenses/WebView2-SDK.txt`.
- Go standard library — лицензия Go BSD, `licenses/Go.txt`.

Тексты лицензий webview и Go сохранены в `licenses` вместе с лицензией SDK. GCC/MinGW используются для сборки; применяются соответствующие лицензии инструментальной цепочки и исключения для библиотек времени выполнения. Сам компилятор не распространяется в поставке.

OpenDiag — название программы, записи которой читает M74 Studio. Файлы записей и локальные отчёты не входят в архив приложения.
