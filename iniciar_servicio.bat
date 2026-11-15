@echo off
setlocal EnableDelayedExpansion
cd /d "%~dp0"

rem ================================================================
rem  Inicia la impresora fiscal virtual por TCP (modo "serve").
rem  Es el modo que conecta con el spooler SpoolerFiscal:
rem    servidor: printer.transport = tcp
rem             printer.tcp_address = 127.0.0.1:2011
rem
rem  Uso:  iniciar_servicio.bat [modelo] [addr]
rem    modelo: tfhka (por omision) | hasar | bixolon | epson
rem    addr:   127.0.0.1:2011 (por omision)
rem
rem  Ejemplos:
rem    iniciar_servicio.bat
rem    iniciar_servicio.bat tfhka 127.0.0.1:2011
rem    iniciar_servicio.bat hasar 0.0.0.0:2012
rem
rem  El estado persistido se guarda en DATA_DIR (data_serve), para
rem  no mezclarse con el del panel web (data).
rem ================================================================

set "MODELO=%~1"
if "%MODELO%"=="" set "MODELO=tfhka"
set "ADDR=%~2"
if "%ADDR%"=="" set "ADDR=127.0.0.1:2011"
set "DATA_DIR=data_serve"

rem --- compilar solo si falta el binario (si ya existe, se usa el actual;
rem     recompilar con el simulador corriendo falla por el archivo en uso) ---
set "NEED_BUILD=0"
if not exist "%~dp0simuladorfiscal.exe" set "NEED_BUILD=1"
where go >nul 2>nul
if errorlevel 1 (
    if "!NEED_BUILD!"=="1" (
        echo [ERROR] No hay Go en el PATH ni simuladorfiscal.exe compilado.
        pause
        exit /b 1
    )
) else (
    if "!NEED_BUILD!"=="1" (
        echo Compilando simuladorfiscal.exe ...
        go build -o "%~dp0simuladorfiscal.exe" ./cmd/simuladorfiscal
        if errorlevel 1 (
            echo [ERROR] Fallo la compilacion.
            pause
            exit /b 1
        )
    )
)

echo.
echo   Impresora fiscal virtual: %MODELO% en tcp://%ADDR%
echo   Estado: %DATA_DIR%   (Ctrl+C para detener)
echo.
"%~dp0simuladorfiscal.exe" serve --model %MODELO% --addr %ADDR% --data-dir "%DATA_DIR%"
echo.
pause
