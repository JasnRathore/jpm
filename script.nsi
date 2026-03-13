; -------------------------------
; Includes
!include "MUI2.nsh"
!include "x64.nsh"

; -------------------------------
; App Info
!define APP_NAME "JasnPackageManager"
!define APP_VERSION "v0.0.0"
!define APP_EXE "fa.exe"
!define APP_DIR "target"

Name "${APP_NAME}"
OutFile "${APP_NAME}Setup@${APP_VERSION}.exe"

InstallDir "$PROGRAMFILES\${APP_NAME}"
RequestExecutionLevel admin

; -------------------------------
; Icons
!define MUI_ICON "icon.ico"
!define MUI_UNICON "icon.ico"

; -------------------------------
; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_LANGUAGE "English"

; -------------------------------
; Install Section
Section "Install"

	SetOutPath "$INSTDIR"

	; Copy files (same as extracting zip)
	File /r "${APP_DIR}\*"

	; Add to PATH
	ReadRegStr $0 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path"
	StrCpy $0 "$0;$INSTDIR"
	WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path" "$0"

	; Refresh environment
	System::Call 'user32::SendMessageTimeoutA(i 0xffff,i 0x1A,i 0,t "Environment",i 0,i 1000,*i .r0)'

	; Create uninstaller
	WriteUninstaller "$INSTDIR\Uninstall.exe"

	; Add uninstall entry
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "DisplayName" "${APP_NAME}"
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "UninstallString" "$\"$INSTDIR\Uninstall.exe$\""
	WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "DisplayIcon" "$INSTDIR\${APP_EXE}"

SectionEnd

; -------------------------------
; Uninstall Section
Section "Uninstall"

	; Remove install directory
	RMDir /r "$INSTDIR"

	; Remove uninstall registry entry
	DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"

SectionEnd
