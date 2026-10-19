Unicode true
!include "MUI2.nsh"
!include "nsDialogs.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "StrFunc.nsh"
${StrStr}
!macro ExecLogged STEP COMMAND
  StrCpy $LastCommand "${STEP}"
  Push "STEP: ${STEP}"
  Call LogLine
  nsExec::ExecToStack '${COMMAND}'
  Pop $CommandCode
  Pop $CommandOutput
  Push "EXIT CODE: $CommandCode"
  Call LogLine
  Push "OUTPUT: $CommandOutput"
  Call LogLine
!macroend
!macro ExecUnLogged STEP COMMAND
  Push "UNINSTALL STEP: ${STEP}"
  Call un.LogLine
  nsExec::ExecToStack '${COMMAND}'
  Pop $CommandCode
  Pop $CommandOutput
  Push "EXIT CODE: $CommandCode"
  Call un.LogLine
  Push "OUTPUT: $CommandOutput"
  Call un.LogLine
!macroend

!ifndef VERSION
  !define VERSION "dev"
!endif

Name "HRMS Connector"
OutFile "../dist/HRMS-Connector-Setup-${VERSION}.exe"
InstallDir "$PROGRAMFILES64\HRMS Connector"
InstallDirRegKey HKLM "Software\HRMSConnector" "InstallDir"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
ShowInstDetails show
ShowUnInstDetails show

Var Key
Var VerifiedKey
Var Port
Var UIPort
Var OldPort
Var OldUIPort
Var PortProvided
Var UIPortProvided
Var Netfix
Var NetChanged
Var NetWasDHCP
Var ExistingConfig
Var ExistingServiceRunning
Var Dialog
Var KeyControl
Var VerifyResult
Var NetworkLabel
Var NetfixControl
Var PortControl
Var UIPortControl
Var DesktopShortcut
Var DesktopShortcutControl
Var ConfigArgs
Var RestoreNetwork
Var RestoreControl
Var ProgramDataDir
Var LogPath
Var CommandCode
Var CommandOutput
Var FinishWarning
Var PortErrorControl
Var ServiceDetails
Var Retry
Var CheckPortValue
Var CheckPortKind
Var OldPortToCompare
Var FailureStep
Var FailureDetails
Var FailureCommand
Var FailureCode
Var FailureOutput
Var LastCommand
Var PreviousLogPath
Var LogCopyLine

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
Page custom ConnectionPageCreate ConnectionPageLeave
Page custom PortsPageCreate PortsPageLeave
Page custom NetworkPageCreate NetworkPageLeave
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "Open HRMS Connector"
!define MUI_FINISHPAGE_RUN_FUNCTION FinishRun
!define MUI_FINISHPAGE_RUN_TEXT "Open HRMS Connector now"
!define MUI_FINISHPAGE_TEXT "Installation finished. $FinishWarning$\r$\nInstaller log: $LogPath"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
UninstPage custom un.NetworkPageCreate un.NetworkPageLeave
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Function .onInit
  InitPluginsDir
  SetOutPath "$PLUGINSDIR"
  File /oname=connector-verify.exe "../dist/connector.exe"
  ReadEnvStr $0 "ProgramData"
  ${If} $0 == ""
    ReadEnvStr $0 "SystemDrive"
    ${If} $0 == ""
      StrCpy $0 "$WINDIR\..\.."
    ${EndIf}
    StrCpy $0 "$0\ProgramData"
  ${EndIf}
  StrCpy $ProgramDataDir "$0\HRMS Connector"
  Call InitInstallLog
!ifdef NSIS_CONFIG_LOG
  LogSet on
!endif
  Push "Installer started. Destination: $INSTDIR. Log: $LogPath"
  Call LogLine
  StrCpy $Port "17081"
  StrCpy $UIPort "17082"
  StrCpy $PortProvided "0"
  StrCpy $UIPortProvided "0"
  StrCpy $DesktopShortcut "1"
  StrCpy $Netfix "1"
  StrCpy $NetChanged "0"
  StrCpy $NetWasDHCP "0"
  StrCpy $ExistingServiceRunning "0"
  IfFileExists "$ProgramDataDir\config.json" 0 +2
    StrCpy $ExistingConfig "1"
  ${GetParameters} $0
  ${GetOptions} $0 "/KEY=" $Key
  ClearErrors
  ${GetOptions} $0 "/PORT=" $Port
  IfErrors noRelayPort
  StrCpy $PortProvided "1"
noRelayPort:
  ClearErrors
  ${GetOptions} $0 "/UIPORT=" $UIPort
  IfErrors noUIPort
  StrCpy $UIPortProvided "1"
noUIPort:
  ${GetOptions} $0 "/NETFIX=" $Netfix
  ${GetOptions} $0 "/DESKTOP=" $DesktopShortcut
  ${If} $ExistingConfig == "1"
    !insertmacro ExecLogged "read saved relay port" '"$PLUGINSDIR\connector-verify.exe" config-port --kind relay'
    ${If} $CommandCode == "0"
      StrCpy $OldPort $CommandOutput
      ${If} $PortProvided == "0"
        StrCpy $Port $CommandOutput
      ${EndIf}
    ${Else}
      Push "WARNING: could not read the saved relay port. Exit code: $CommandCode. $CommandOutput"
      Call LogLine
      StrCpy $FinishWarning "Saved configuration could not be read; verify the ports before continuing. "
    ${EndIf}
    !insertmacro ExecLogged "read saved UI port" '"$PLUGINSDIR\connector-verify.exe" config-port --kind ui'
    ${If} $CommandCode == "0"
      StrCpy $OldUIPort $CommandOutput
      ${If} $UIPortProvided == "0"
        StrCpy $UIPort $CommandOutput
      ${EndIf}
    ${Else}
      Push "WARNING: could not read the saved UI port. Exit code: $CommandCode. $CommandOutput"
      Call LogLine
      StrCpy $FinishWarning "Saved configuration could not be read; verify the ports before continuing. "
    ${EndIf}
  ${EndIf}
  !insertmacro ExecLogged "detect running existing connector service" '"$SYSDIR\sc.exe" query HRMSConnector'
  ${If} $CommandCode == "0"
    ${StrStr} $R0 $CommandOutput "RUNNING"
    ${If} $R0 != ""
      StrCpy $ExistingServiceRunning "1"
    ${EndIf}
  ${EndIf}
  IfSilent 0 done
  ${If} $Key != ""
    !insertmacro ExecLogged "verify silent-install connection key" '"$PLUGINSDIR\connector-verify.exe" verify-key "$Key"'
    ${If} $CommandCode != "0"
      MessageBox MB_ICONSTOP "Step: verify connection key$\r$\nExit code: $CommandCode$\r$\nOutput: $CommandOutput$\r$\nLog: $LogPath"
      Abort
    ${EndIf}
  ${ElseIf} $ExistingConfig != "1"
    MessageBox MB_ICONSTOP "A new installation requires /KEY=<connection-key>."
    Abort
  ${EndIf}
  Call ValidateSilentPorts
done:
FunctionEnd

Function InitInstallLog
  StrCpy $PreviousLogPath "$LogPath"
  CreateDirectory "$INSTDIR"
  StrCpy $LogPath "$INSTDIR\install.log"
  ClearErrors
  FileOpen $0 "$LogPath" a
  IfErrors logFallback
  FileClose $0
  Goto logReady
logFallback:
  ReadEnvStr $0 "TEMP"
  ${If} $0 == ""
    StrCpy $0 "$PLUGINSDIR"
  ${EndIf}
  StrCpy $LogPath "$0\hrms-connector-install.log"
  ClearErrors
  FileOpen $0 "$LogPath" a
  IfErrors logUnavailable
  FileClose $0
  Goto logReady
logUnavailable:
  StrCpy $LogPath "(unable to create installer log)"
logReady:
  ${If} $PreviousLogPath != ""
  ${AndIf} $PreviousLogPath != "$LogPath"
  ${AndIf} $PreviousLogPath != "(unable to create installer log)"
    ClearErrors
    FileOpen $R0 "$PreviousLogPath" r
    IfErrors logCopyDone
    ClearErrors
    FileOpen $R1 "$LogPath" a
    IfErrors logCopyCloseSource
logCopyLoop:
    ClearErrors
    FileRead $R0 $LogCopyLine
    IfErrors logCopyCloseBoth
    FileWrite $R1 "$LogCopyLine"
    Goto logCopyLoop
logCopyCloseBoth:
    FileClose $R1
logCopyCloseSource:
    FileClose $R0
logCopyDone:
  ${EndIf}
FunctionEnd

Function LogLine
  Exch $0
  DetailPrint "$0"
  ${If} $LogPath != "(unable to create installer log)"
    ClearErrors
    FileOpen $1 "$LogPath" a
    IfErrors logDone
    FileWrite $1 "$0$\r$\n"
    FileClose $1
  ${EndIf}
logDone:
  Pop $0
FunctionEnd

Function DumpLog
  Push $R0
  Push $R1
  Push $R2
  Push $R3
  FindWindow $R0 "#32770" "" $HWNDPARENT
  GetDlgItem $R0 $R0 1016
  SendMessage $R0 ${LB_GETCOUNT} 0 0 $R1
  StrCpy $R2 0
dumpLogLoop:
  IntCmp $R2 $R1 dumpLogDone dumpLogWrite dumpLogDone
dumpLogWrite:
  System::Call 'user32::SendMessage(p $R0, i ${LB_GETTEXT}, p $R2, t .s)'
  Pop $R3
  Push "DETAIL: $R3"
  Call LogLine
  IntOp $R2 $R2 + 1
  Goto dumpLogLoop
dumpLogDone:
  Pop $R3
  Pop $R2
  Pop $R1
  Pop $R0
FunctionEnd

Function un.LogLine
  Exch $0
  DetailPrint "$0"
  ${If} $LogPath != "(unable to create installer log)"
    ClearErrors
    FileOpen $1 "$LogPath" a
    IfErrors unLogDone
    FileWrite $1 "$0$\r$\n"
    FileClose $1
  ${EndIf}
unLogDone:
  Pop $0
FunctionEnd

Function VerifyConnectionKey
  ${NSD_GetText} $KeyControl $Key
  ${If} $Key == ""
    ${NSD_SetText} $VerifyResult "Paste a connection key first."
    Return
  ${EndIf}
  !insertmacro ExecLogged "verify connection key" '"$PLUGINSDIR\connector-verify.exe" verify-key "$Key"'
  ${If} $CommandCode == "0"
    StrCpy $VerifiedKey $Key
    ${NSD_SetText} $VerifyResult "$CommandOutput"
  ${Else}
    StrCpy $VerifiedKey ""
    ${NSD_SetText} $VerifyResult "Step: verify connection key. Exit code: $CommandCode. Output: $CommandOutput"
  ${EndIf}
FunctionEnd

Function ConnectionPageCreate
  IfSilent silent
  Call InitInstallLog
  Push "Connection key page opened. Log: $LogPath"
  Call LogLine
  nsDialogs::Create 1018
  Pop $Dialog
  ${If} $Dialog == error
    MessageBox MB_ICONSTOP "Step: create connection-key page. The installer could not initialize the page.$\r$\nLog: $LogPath"
    Push "ERROR: connection-key dialog could not be initialized."
    Call LogLine
    Abort
  ${EndIf}
  ${NSD_CreateLabel} 0 0 100% 28u "Paste your HRMS connection key / HRMS সংযোগ কী পেস্ট করে Verify নির্বাচন করুন। A new key must be verified before continuing / এগোনোর আগে নতুন কী যাচাই করতে হবে।"
  Pop $0
  ${NSD_CreateText} 0 34u 100% 14u "$Key"
  Pop $KeyControl
  ${NSD_CreateButton} 0 54u 26% 15u "Verify"
  Pop $1
  ${NSD_OnClick} $1 VerifyConnectionKey
  ${NSD_CreateLabel} 0 75u 100% 45u ""
  Pop $VerifyResult
  nsDialogs::Show
  Return
silent:
  Abort
FunctionEnd

Function ConnectionPageLeave
  IfSilent silent
  ${NSD_GetText} $KeyControl $Key
  ${If} $Key == ""
    ${If} $ExistingConfig == "1"
      Return
    ${EndIf}
    MessageBox MB_ICONEXCLAMATION "Verify a connection key before continuing."
    Abort
  ${EndIf}
  ${If} $VerifiedKey != $Key
    MessageBox MB_ICONEXCLAMATION "Verify this connection key before continuing."
    Abort
  ${EndIf}
  Return
silent:
  Return
FunctionEnd

Function CheckInstallerPorts
  StrCpy $CheckPortValue "$Port"
  StrCpy $CheckPortKind "Relay"
  StrCpy $OldPortToCompare "$OldPort"
  Call CheckOnePort
  ${If} $CommandCode != "0"
    Return
  ${EndIf}
  StrCpy $CheckPortValue "$UIPort"
  StrCpy $CheckPortKind "UI"
  StrCpy $OldPortToCompare "$OldUIPort"
  Call CheckOnePort
FunctionEnd

Function CheckOnePort
  ${If} $ExistingServiceRunning == "1"
  ${AndIf} $CheckPortValue == $OldPortToCompare
    StrCpy $CommandCode "0"
    StrCpy $CommandOutput "Saved port unchanged; bind check skipped because the installed service owns it."
    Push "$CheckPortKind port $CheckPortValue unchanged."
    Call LogLine
    Return
  ${EndIf}
  !insertmacro ExecLogged "check port $CheckPortValue" '"$PLUGINSDIR\connector-verify.exe" check-port "$CheckPortValue"'
FunctionEnd

Function ValidateSilentPorts
  IfSilent 0 done
  Call CheckInstallerPorts
  ${If} $CommandCode != "0"
    MessageBox MB_ICONSTOP "Step: validate installer ports. Exit code: $CommandCode$\r$\nOutput: $CommandOutput$\r$\nLog: $LogPath"
    Abort
  ${EndIf}
  IntCmp $Port $UIPort relayInvalid relayValid relayValid
relayInvalid:
  MessageBox MB_ICONSTOP "Step: validate installer ports. Relay and UI ports must be different ($Port and $UIPort). Log: $LogPath"
  Abort
relayValid:
done:
FunctionEnd

Function PortsPageCreate
  IfSilent silent
  nsDialogs::Create 1018
  Pop $Dialog
  ${If} $Dialog == error
    MessageBox MB_ICONSTOP "Step: create ports page. The installer could not initialize the page.$\r$\nLog: $LogPath"
    Push "ERROR: ports dialog could not be initialized."
    Call LogLine
    Abort
  ${EndIf}
  ${NSD_CreateLabel} 0 0 100% 30u "Select available ports (1024–49151). The relay port is entered on every device; the local UI port stays fixed after installation."
  Pop $0
  ${NSD_CreateLabel} 0 35u 35% 14u "Device relay port"
  Pop $0
  ${NSD_CreateNumber} 40% 33u 60% 16u "$Port"
  Pop $PortControl
  ${NSD_CreateLabel} 0 57u 35% 14u "Local UI port (fixed)"
  Pop $0
  ${NSD_CreateNumber} 40% 55u 60% 16u "$UIPort"
  Pop $UIPortControl
  ${NSD_CreateLabel} 0 75u 100% 25u ""
  Pop $PortErrorControl
  ${NSD_CreateCheckbox} 0 105u 100% 20u "Create a Desktop shortcut"
  Pop $DesktopShortcutControl
  ${If} $DesktopShortcut == "1"
    ${NSD_Check} $DesktopShortcutControl
  ${EndIf}
  nsDialogs::Show
  Return
silent:
  Return
FunctionEnd

Function PortsPageLeave
  IfSilent silent
  ${NSD_GetText} $PortControl $Port
  ${NSD_GetText} $UIPortControl $UIPort
  ${NSD_GetState} $DesktopShortcutControl $0
  ${If} $0 == ${BST_CHECKED}
    StrCpy $DesktopShortcut "1"
  ${Else}
    StrCpy $DesktopShortcut "0"
  ${EndIf}
silent:
  Push ""
  Call SetPortPageError
  Push "Validating selected ports: relay=$Port UI=$UIPort"
  Call LogLine
  StrCpy $CheckPortValue "$Port"
  StrCpy $CheckPortKind "Relay"
  StrCpy $OldPortToCompare "$OldPort"
  Call CheckOnePort
  ${If} $CommandCode != "0"
    Push "Relay port $Port (exit $CommandCode): $CommandOutput"
    Call SetPortPageError
    Push "Relay port $Port rejected. Exit code: $CommandCode. $CommandOutput"
    Call LogLine
    Abort
  ${EndIf}
  StrCpy $CheckPortValue "$UIPort"
  StrCpy $CheckPortKind "UI"
  StrCpy $OldPortToCompare "$OldUIPort"
  Call CheckOnePort
  ${If} $CommandCode != "0"
    Push "UI port $UIPort (exit $CommandCode): $CommandOutput"
    Call SetPortPageError
    Push "UI port $UIPort rejected. Exit code: $CommandCode. $CommandOutput"
    Call LogLine
    Abort
  ${EndIf}
  IntCmp $Port $UIPort samePorts distinctPorts distinctPorts
samePorts:
  Push "Relay and UI ports must be different."
  Call SetPortPageError
  Push "Selected ports are identical: $Port"
  Call LogLine
  Abort
distinctPorts:
  Call CheckInstallerPorts
  ${If} $CommandCode != "0"
    Push "$CommandOutput"
    Call SetPortPageError
    Abort
  ${EndIf}
FunctionEnd

Function SetPortPageError
  Exch $0
  IfSilent setPortPageErrorDone
  ${NSD_SetText} $PortErrorControl "$0"
setPortPageErrorDone:
  Pop $0
FunctionEnd

Function CaptureDHCPState
  !insertmacro ExecLogged "read current network state" '"$PLUGINSDIR\connector-verify.exe" netfix --status --json'
  ${If} $CommandCode == "0"
    ${StrStr} $2 $CommandOutput '"dhcp":true'
    ${If} $2 != ""
      StrCpy $NetWasDHCP "1"
    ${EndIf}
  ${EndIf}
FunctionEnd

Function NetworkPageCreate
  IfSilent silent
  nsDialogs::Create 1018
  Pop $Dialog
  ${If} $Dialog == error
    StrCpy $FinishWarning "$FinishWarning Optional network page could not be displayed. "
    Push "Could not create the optional network page."
    Call LogLine
    MessageBox MB_ICONEXCLAMATION "Step: display optional network setup. This page is unavailable; installation will continue without changing the network. Log: $LogPath"
    Abort
  ${EndIf}
  !insertmacro ExecLogged "display current network state" '"$PLUGINSDIR\connector-verify.exe" netfix --status'
  ${NSD_CreateLabel} 0 0 100% 40u "Current network: $CommandOutput"
  Pop $NetworkLabel
  ${NSD_CreateLabel} 0 48u 100% 55u "The current IP address will be kept. The address must not also be assigned to another device by your router. We recommend creating a DHCP reservation for this PC on the router."
  Pop $0
  ${NSD_CreateCheckbox} 0 110u 100% 20u "Keep this PC's IP address fixed. The address will not change."
  Pop $NetfixControl
  ${If} $Netfix == "1"
    ${NSD_Check} $NetfixControl
  ${EndIf}
  nsDialogs::Show
  Return
silent:
  Return
FunctionEnd

Function NetworkPageLeave
  IfSilent silent
  ${NSD_GetState} $NetfixControl $0
  ${If} $0 == ${BST_CHECKED}
    Call CaptureDHCPState
    !insertmacro ExecLogged "optional DHCP-to-static network setup" '"$PLUGINSDIR\connector-verify.exe" netfix --apply'
    ${If} $CommandCode != "0"
      StrCpy $FinishWarning "IP setup was not completed; the current IP was not changed. "
      Push "WARNING: optional network configuration failed. Exit code: $CommandCode. Output: $CommandOutput"
      Call LogLine
    ${ElseIf} $NetWasDHCP == "1"
      StrCpy $NetChanged "1"
    ${EndIf}
  ${EndIf}
  Return
silent:
  ${If} $Netfix == "1"
    Call CaptureDHCPState
    !insertmacro ExecLogged "silent optional DHCP-to-static network setup" '"$PLUGINSDIR\connector-verify.exe" netfix --apply'
    ${If} $CommandCode != "0"
      StrCpy $FinishWarning "IP setup was not completed. "
      Push "WARNING: silent optional network configuration failed. Exit code: $CommandCode. Output: $CommandOutput"
      Call LogLine
    ${ElseIf} $NetWasDHCP == "1"
      StrCpy $NetChanged "1"
    ${EndIf}
  ${EndIf}
FunctionEnd

Function FinishRun
  Push "Opening the local Device Setup page."
  Call LogLine
  ClearErrors
  ExecShell "open" "http://127.0.0.1:$UIPort/setup"
  IfErrors browserOpenFailed
  Return
browserOpenFailed:
  StrCpy $FinishWarning "$FinishWarning Browser could not be opened; use http://127.0.0.1:$UIPort/setup. "
  Push "WARNING: browser open failed with NSIS error."
  Call LogLine
  MessageBox MB_ICONEXCLAMATION "HRMS Connector is installed, but the browser could not be opened. Open http://127.0.0.1:$UIPort/setup manually. Log: $LogPath"
FunctionEnd

Function AbortInstall
  ${If} $FailureCommand == ""
    StrCpy $FailureCommand "$LastCommand"
  ${EndIf}
  ${If} $FailureCode == ""
    StrCpy $FailureCode "$CommandCode"
  ${EndIf}
  ${If} $FailureOutput == ""
    StrCpy $FailureOutput "$CommandOutput"
  ${EndIf}
  Push "FATAL: $FailureStep; command=$FailureCommand; exit code=$FailureCode; output=$FailureOutput; details=$FailureDetails"
  Call LogLine
  Call DumpLog
  MessageBox MB_ICONSTOP "Step: $FailureStep$\r$\nCommand: $FailureCommand$\r$\nExit code: $FailureCode$\r$\nOutput: $FailureOutput$\r$\n$FailureDetails$\r$\nInstaller log: $LogPath"
  Abort
FunctionEnd

Function CleanupExistingService
  !insertmacro ExecLogged "query existing HRMSConnector service" '"$SYSDIR\sc.exe" query HRMSConnector'
  ${If} $CommandCode != "0"
    ${StrStr} $R0 $CommandOutput "1060"
    ${If} $R0 != ""
      Push "No existing HRMSConnector service found."
      Call LogLine
      Goto cleanupKillProcesses
    ${EndIf}
    StrCpy $FailureStep "query existing Windows service"
    StrCpy $FailureDetails "sc.exe query HRMSConnector failed."
    Call AbortInstall
  ${EndIf}
  !insertmacro ExecLogged "stop existing HRMSConnector service" '"$SYSDIR\sc.exe" stop HRMSConnector'
  StrCpy $Retry 0
cleanupWaitStop:
  Sleep 1000
  !insertmacro ExecLogged "verify existing service stopped" '"$SYSDIR\sc.exe" query HRMSConnector'
  ${StrStr} $R0 $CommandOutput "STOPPED"
  ${If} $R0 == ""
    IntOp $Retry $Retry + 1
    IntCmp $Retry 3 cleanupStopFailed cleanupWaitStop cleanupStopFailed
  ${EndIf}
  Goto cleanupKillProcesses
cleanupStopFailed:
  StrCpy $FailureStep "stop existing Windows service"
  StrCpy $FailureCommand "sc.exe query HRMSConnector (after stop request)"
  StrCpy $FailureCode "$CommandCode"
  StrCpy $FailureOutput "$CommandOutput"
  StrCpy $FailureDetails "Service did not reach STOPPED after three checks."
  Call AbortInstall
cleanupKillProcesses:
  !insertmacro ExecLogged "terminate remaining connector process" '"$SYSDIR\taskkill.exe" /F /IM connector.exe /T'
  !insertmacro ExecLogged "terminate any legacy tray process" '"$SYSDIR\taskkill.exe" /F /IM connector-tray.exe /T'
  !insertmacro ExecLogged "delete existing HRMSConnector service" '"$SYSDIR\sc.exe" delete HRMSConnector'
  StrCpy $Retry 0
cleanupWaitDelete:
  Sleep 1000
  !insertmacro ExecLogged "verify existing service deleted" '"$SYSDIR\sc.exe" query HRMSConnector'
  ${If} $CommandCode != "0"
    ${StrStr} $R0 $CommandOutput "1060"
    ${If} $R0 != ""
      Goto cleanupDone
    ${EndIf}
  ${EndIf}
  IntOp $Retry $Retry + 1
  IntCmp $Retry 3 cleanupDeleteFailed cleanupWaitDelete cleanupDeleteFailed
cleanupDeleteFailed:
  StrCpy $FailureStep "delete existing Windows service"
  StrCpy $FailureCommand "sc.exe query HRMSConnector (after delete request)"
  StrCpy $FailureCode "$CommandCode"
  StrCpy $FailureOutput "$CommandOutput"
  StrCpy $FailureDetails "Service remains registered after three delete checks."
  Call AbortInstall
cleanupDone:
  Push "Existing service stopped and removed."
  Call LogLine
FunctionEnd

Function ServiceDiagnostics
  StrCpy $ServiceDetails ""
  !insertmacro ExecLogged "collect service query diagnostics" '"$SYSDIR\sc.exe" query HRMSConnector'
  StrCpy $ServiceDetails "sc query: $CommandOutput"
  !insertmacro ExecLogged "collect service configuration diagnostics" '"$SYSDIR\sc.exe" qc HRMSConnector'
  StrCpy $ServiceDetails "$ServiceDetails$\r$\nsc qc: $CommandOutput"
  !insertmacro ExecLogged "collect recent Windows System event log text" '"$SYSDIR\wevtutil.exe" qe System /c:10 /rd:true /f:text'
  StrCpy $ServiceDetails "$ServiceDetails$\r$\nRecent Service Control Manager events: $CommandOutput"
FunctionEnd

Section "Install"
  Push "Preparing existing service for upgrade."
  Call LogLine
  Call CleanupExistingService

  SetOutPath "$INSTDIR"
  File "../dist/connector.exe"
  CreateDirectory "$ProgramDataDir"
  !insertmacro ExecLogged "grant service data-directory permissions" '"$SYSDIR\icacls.exe" "$ProgramDataDir" /grant "NT AUTHORITY\LOCAL SERVICE:(OI)(CI)M" /T /C'
  ${If} $CommandCode != "0"
    StrCpy $FailureStep "grant LocalService access to the data directory"
    StrCpy $FailureCommand "icacls.exe ProgramData\\HRMS Connector /grant LocalService modify"
    StrCpy $FailureCode "$CommandCode"
    StrCpy $FailureOutput "$CommandOutput"
    StrCpy $FailureDetails "icacls.exe could not grant the service account write access."
    Call AbortInstall
  ${EndIf}

  ${If} $Key != ""
  ${OrIf} $Port != $OldPort
  ${OrIf} $UIPort != $OldUIPort
    StrCpy $ConfigArgs 'configure --port "$Port" --ui-port "$UIPort"'
    ${If} $Key != ""
      StrCpy $ConfigArgs '$ConfigArgs --key "$Key"'
    ${EndIf}
    !insertmacro ExecLogged "save connector configuration" '"$INSTDIR\connector.exe" $ConfigArgs'
    ${If} $CommandCode != "0"
      StrCpy $FailureStep "save connector configuration"
      StrCpy $FailureCommand "connector.exe configure --port $Port --ui-port $UIPort"
      StrCpy $FailureCode "$CommandCode"
      StrCpy $FailureOutput "$CommandOutput"
      StrCpy $FailureDetails "Existing config.json and connection key are retained unless explicitly changed."
      Call AbortInstall
    ${EndIf}
  ${EndIf}

  WriteRegStr HKLM "Software\HRMSConnector" "InstallDir" "$INSTDIR"
  WriteRegDWORD HKLM "Software\HRMSConnector" "NetworkChanged" "$NetChanged"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\HRMSConnector" "DisplayName" "HRMS Connector"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\HRMSConnector" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\HRMSConnector" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\HRMSConnector" "Publisher" "HRMS Connector"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\HRMSConnector" "InstallLocation" "$INSTDIR"
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\HRMSConnector" "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\HRMSConnector" "NoRepair" 1
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ${If} $ExistingConfig == "1"
    !insertmacro ExecLogged "remove previous HRMS Connector firewall rule" '"$SYSDIR\netsh.exe" advfirewall firewall delete rule name="HRMS Connector"'
    ${If} $CommandCode != "0"
      StrCpy $FinishWarning "$FinishWarning Existing firewall rule could not be removed. "
      Push "WARNING: firewall rule cleanup failed. Exit code: $CommandCode. Output: $CommandOutput"
      Call LogLine
    ${EndIf}
  ${EndIf}
  !insertmacro ExecLogged "add HRMS Connector firewall rule" '"$SYSDIR\netsh.exe" advfirewall firewall add rule name="HRMS Connector" dir=in action=allow protocol=TCP localport=$Port profile=any remoteip=LocalSubnet'
  ${If} $CommandCode != "0"
    StrCpy $FinishWarning "$FinishWarning Firewall rule could not be added. "
    Push "WARNING: firewall rule add failed. Exit code: $CommandCode. Output: $CommandOutput"
    Call LogLine
  ${EndIf}
  !insertmacro ExecLogged "install HRMSConnector Windows service" '"$INSTDIR\connector.exe" install'
  ${If} $CommandCode != "0"
    StrCpy $FailureStep "install Windows service"
    StrCpy $FailureCommand "connector.exe install"
    StrCpy $FailureCode "$CommandCode"
    StrCpy $FailureOutput "$CommandOutput"
    Call ServiceDiagnostics
    StrCpy $FailureDetails "$ServiceDetails"
    Call AbortInstall
  ${EndIf}
  !insertmacro ExecLogged "configure delayed automatic service start" '"$SYSDIR\sc.exe" config HRMSConnector start= delayed-auto'
  ${If} $CommandCode != "0"
    StrCpy $FailureStep "configure delayed automatic service start"
    StrCpy $FailureCommand "sc.exe config HRMSConnector start= delayed-auto"
    StrCpy $FailureCode "$CommandCode"
    StrCpy $FailureOutput "$CommandOutput"
    StrCpy $FailureDetails "sc.exe config failed."
    Call AbortInstall
  ${EndIf}
  !insertmacro ExecLogged "configure service recovery actions" '"$SYSDIR\sc.exe" failure HRMSConnector reset= 86400 actions= restart/5000/restart/30000/restart/60000'
  ${If} $CommandCode != "0"
    StrCpy $FinishWarning "$FinishWarning Automatic service recovery actions could not be configured. "
    Push "WARNING: service recovery configuration failed. Exit code: $CommandCode. Output: $CommandOutput"
    Call LogLine
  ${EndIf}
  !insertmacro ExecLogged "start HRMSConnector Windows service" '"$INSTDIR\connector.exe" start'
  ${If} $CommandCode != "0"
    StrCpy $FailureStep "start Windows service"
    StrCpy $FailureCommand "connector.exe start"
    StrCpy $FailureCode "$CommandCode"
    StrCpy $FailureOutput "$CommandOutput"
    Call ServiceDiagnostics
    StrCpy $FailureDetails "$ServiceDetails"
    Call AbortInstall
  ${EndIf}
  !insertmacro ExecLogged "wait up to 30 seconds for the local UI" '"$INSTDIR\connector.exe" wait-ready --timeout 30'
  ${If} $CommandCode != "0"
    StrCpy $FailureStep "wait for local connector UI (30 seconds)"
    StrCpy $FailureCommand "connector.exe wait-ready --timeout 30"
    StrCpy $FailureCode "$CommandCode"
    StrCpy $FailureOutput "$CommandOutput"
    Call ServiceDiagnostics
    StrCpy $FailureDetails "$ServiceDetails"
    Call AbortInstall
  ${EndIf}
  CreateDirectory "$SMPROGRAMS\HRMS Connector"
  !insertmacro ExecLogged "create Start Menu shortcut" '"$INSTDIR\connector.exe" shortcut --path "$SMPROGRAMS\HRMS Connector\HRMS Connector.url" --url "http://127.0.0.1:$UIPort/" --icon "$INSTDIR\connector.exe"'
  ${If} $CommandCode != "0"
    StrCpy $FinishWarning "$FinishWarning Start Menu shortcut could not be created. "
    Push "WARNING: Start Menu shortcut failed. Exit code: $CommandCode. Output: $CommandOutput"
    Call LogLine
  ${EndIf}
  ${If} $DesktopShortcut == "1"
    !insertmacro ExecLogged "create Desktop shortcut" '"$INSTDIR\connector.exe" shortcut --path "$DESKTOP\HRMS Connector.url" --url "http://127.0.0.1:$UIPort/" --icon "$INSTDIR\connector.exe"'
    ${If} $CommandCode != "0"
      StrCpy $FinishWarning "$FinishWarning Desktop shortcut could not be created. "
      Push "WARNING: Desktop shortcut failed. Exit code: $CommandCode. Output: $CommandOutput"
      Call LogLine
    ${EndIf}
  ${EndIf}
SectionEnd

Function un.onInit
  StrCpy $LogPath "$INSTDIR\install.log"
  Push "Uninstaller started."
  Call un.LogLine
  StrCpy $RestoreNetwork "0"
  ReadRegDWORD $RestoreNetwork HKLM "Software\HRMSConnector" "NetworkChanged"
  ${GetParameters} $0
  ${GetOptions} $0 "/RESTORENET=" $1
  IfErrors +2
    StrCpy $RestoreNetwork $1
FunctionEnd

Function un.RestoreClick
  ${NSD_GetState} $RestoreControl $0
  ${If} $0 == ${BST_CHECKED}
    StrCpy $RestoreNetwork "1"
  ${Else}
    StrCpy $RestoreNetwork "0"
  ${EndIf}
FunctionEnd

Function un.NetworkPageCreate
  nsDialogs::Create 1018
  Pop $Dialog
  ${If} $Dialog == error
    Abort
  ${EndIf}
  ${NSD_CreateLabel} 0 0 100% 35u "Restore the adapter to automatic DHCP addressing if this installer changed it."
  Pop $0
  ${NSD_CreateCheckbox} 0 42u 100% 20u "Restore automatic IP (DHCP)"
  Pop $RestoreControl
  ${If} $RestoreNetwork == "1"
    ${NSD_Check} $RestoreControl
  ${EndIf}
  ${NSD_OnClick} $RestoreControl un.RestoreClick
  nsDialogs::Show
FunctionEnd

Function un.NetworkPageLeave
  ${NSD_GetState} $RestoreControl $0
  ${If} $0 == ${BST_CHECKED}
    StrCpy $RestoreNetwork "1"
  ${Else}
    StrCpy $RestoreNetwork "0"
  ${EndIf}
FunctionEnd

Section "Uninstall"
  IfFileExists "$INSTDIR\connector.exe" 0 +3
    !insertmacro ExecUnLogged "stop connector service during uninstall" '"$INSTDIR\connector.exe" stop'
    !insertmacro ExecUnLogged "remove connector service during uninstall" '"$INSTDIR\connector.exe" uninstall'
  !insertmacro ExecUnLogged "remove firewall rule during uninstall" '"$SYSDIR\netsh.exe" advfirewall firewall delete rule name="HRMS Connector"'
  ${If} $RestoreNetwork == "1"
    IfFileExists "$INSTDIR\connector.exe" 0 +2
      !insertmacro ExecUnLogged "restore automatic addressing during uninstall" '"$INSTDIR\connector.exe" netfix --restore'
  ${EndIf}
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\HRMSConnector"
  DeleteRegKey HKLM "Software\HRMSConnector"
  Delete "$DESKTOP\HRMS Connector.url"
  Delete "$SMPROGRAMS\HRMS Connector\HRMS Connector.url"
  RMDir "$SMPROGRAMS\HRMS Connector"
  Delete "$INSTDIR\connector.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
SectionEnd
