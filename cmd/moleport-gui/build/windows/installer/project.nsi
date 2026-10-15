Unicode true

####
## MolePort GUI インストーラー (Wails v2.16.0 の NSIS テンプレートをベースにしたユーザー単位インストーラー)。
##
## テンプレートからの変更点:
## - ユーザー単位インストール (管理者権限不要)。インストール先 $LOCALAPPDATA\Programs\MolePort、
##   アンインストール情報は HKCU、ショートカットも現在のユーザー用。
## - 起動中の moleport-gui.exe がある場合はインストール / アンインストール前に終了を促す。
## - アンインストールで $INSTDIR を再帰削除しない (ユーザーが選んだ任意フォルダを巻き込まないため)。
##
## Template replacements don't work in this file. wails_tools.nsh が INFO_* を wails.json の info から補完する。
## 手動確認: wails build -platform windows/amd64 -tags gui -skipbindings -nsis (-installscope user は不要だが指定しても無害)
####

# wails build -nsis -installscope user を指定しなくてもユーザー単位になるよう既定値を与える。
# (wails_tools.nsh は REQUEST_EXECUTION_LEVEL の既定を admin にするため、include より前に定義する)
!ifndef REQUEST_EXECUTION_LEVEL
  !define REQUEST_EXECUTION_LEVEL "user"
!endif
!ifndef WAILS_INSTALL_SCOPE
  !define WAILS_INSTALL_SCOPE "user"
!endif

!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}" # ユーザー単位のインストール先 (同一パスへ上書きして更新する)。
ShowInstDetails show # This will always show the installation details.

# 起動中の ${PRODUCT_EXECUTABLE} を検出し、終了するまで再試行 / 中止 / 無視をユーザーに選ばせる。
# tasklist は一致するプロセスがあれば CSV 行 ("name","pid",...) を、なければ "INFO: ..." (ロケール依存) を出力するため、
# 先頭文字が二重引用符かどうかだけで判定する。サイレント実行時は確認せず、ファイル使用中エラーに任せる。
!macro MolePort.checkRunning un
Function ${un}CheckRunning
    IfSilent done
  retry:
    nsExec::ExecToStack '"$SYSDIR\tasklist.exe" /FI "IMAGENAME eq ${PRODUCT_EXECUTABLE}" /FO CSV /NH'
    Pop $0 # exit code
    Pop $1 # output
    ${If} $0 != 0
        Goto done
    ${EndIf}
    StrCpy $1 $1 1
    ${If} $1 != '"'
        Goto done
    ${EndIf}
    MessageBox MB_ABORTRETRYIGNORE|MB_ICONEXCLAMATION "${INFO_PRODUCTNAME} is running.$\r$\nClose it (Quit from the tray menu), then click Retry.$\r$\n$\r$\nAbort: cancel. Ignore: continue anyway." IDRETRY retry IDIGNORE done
    Quit
  done:
FunctionEnd
!macroend
!insertmacro MolePort.checkRunning ""
!insertmacro MolePort.checkRunning "un."

Function .onInit
   !insertmacro wails.checkArchitecture
   Call CheckRunning
FunctionEnd

Function un.onInit
   Call un.CheckRunning
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    # インストールしたファイルだけを削除する。$INSTDIR は再帰削除しない (設定ファイル ~/.config/moleport も削除しない)。
    Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller

    RMDir "$INSTDIR"
SectionEnd
