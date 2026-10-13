@echo off
setlocal EnableDelayedExpansion
cd /d "%~dp0"

rem ================================================================ leer .env
set "HOST=127.0.0.1"
set "PORT=8787"
set "DATA_DIR=data"
set "NO_BROWSER=0"
if exist ".env" (
    for /f "usebackq tokens=1,* delims==" %%A in (".env") do (
        if /i "%%A"=="HOST"       set "HOST=%%B"
        if /i "%%A"=="PORT"       set "PORT=%%B"
        if /i "%%A"=="DATA_DIR"   set "DATA_DIR=%%B"
        if /i "%%A"=="NO_BROWSER" set "NO_BROWSER=%%B"
    )
)

rem validar que PORT sea numerico (si no, vuelve a 8080)
echo(%PORT%| findstr /R /C:"^[0-9][0-9]*$" >nul || set "PORT=8080"
set "PREF=!PORT!"

rem ================================================ buscar puerto libre
set /a "TRY=%PORT%"
:scan
if !TRY! GTR 65535 (
    echo [ERROR] No se encontro ningun puerto libre.
    pause
    exit /b 1
)
netstat -ano | findstr /C:":!TRY! " | findstr /C:"LISTENING" >nul 2>&1
if errorlevel 1 goto gotport
if !TRY! EQU !PREF! echo   Puerto !PREF! ocupado, buscando el siguiente...
set /a TRY+=1
goto scan

:gotport
set "PORT=!TRY!"

rem ======================================================== compilar binario
where go >nul 2>&1
if errorlevel 1 (
    if exist "%~dp0simuladorfiscal.exe" (
        echo Go no esta en el PATH; se usa el binario existente.
        goto launch
    )
    echo [ERROR] No hay Go instalado ni simuladorfiscal.exe compilado.
    pause
    exit /b 1
)
echo Compilando simuladorfiscal.exe ...
go build -o "%~dp0simuladorfiscal.exe" ./cmd/simuladorfiscal
if errorlevel 1 (
    echo [ERROR] Fallo la compilacion.
    pause
    exit /b 1
)

rem ============================================================= lanzar panel
:launch
set "EXTRA="
if /i "!NO_BROWSER!"=="1" set "EXTRA= --no-browser"
echo.
echo   Panel SimuladorFiscalGO en http://!HOST!:!PORT!  (del .env: !PREF!, usado: !PORT!)
echo   Ctrl+C detiene el panel.
echo.
"%~dp0simuladorfiscal.exe" panel --host !HOST! --port !PORT! --data-dir "!DATA_DIR!"!EXTRA!
echo.
pause
