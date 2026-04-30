!include "MUI2.nsh"
!include "x64.nsh"

!define APPNAME "OngoPlayer"
!define COMPANYNAME "Dlcuy22"
!define DESCRIPTION "Dead simple Music Player that just works"
!define MUI_ICON "./favicon.ico"
Name "${APPNAME}"
OutFile "..\build\OngoPlayer_Installer.exe"
InstallDir "$PROGRAMFILES64\${APPNAME}"
RequestExecutionLevel admin

!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "Install"
    ${If} ${RunningX64}
        setregview 64
    ${EndIf}

    SetOutPath "$INSTDIR"

    File /r "..\build\win\*.*"

    WriteUninstaller "$INSTDIR\Uninstall.exe"
    CreateShortcut "$SMPROGRAMS\${APPNAME}.lnk" "$INSTDIR\OngoPlayer.exe"
    CreateShortcut "$DESKTOP\${APPNAME}.lnk" "$INSTDIR\OngoPlayer.exe"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "DisplayName" "${APPNAME}"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}" "UninstallString" "$\"$INSTDIR\Uninstall.exe$\""
SectionEnd

Section "Uninstall"

    Delete "$SMPROGRAMS\${APPNAME}.lnk"
    Delete "$DESKTOP\${APPNAME}.lnk"

    RMDir /r "$INSTDIR"

    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"

SectionEnd