#define AppVersion GetEnv("JHTB_VERSION")
#define AppExe GetEnv("JHTB_EXE")
#define BuildOutput GetEnv("JHTB_OUTPUT")
#if AppVersion == ""
  #error "JHTB_VERSION must come from version.json"
#endif
#if AppExe == ""
  #error "JHTB_EXE must point to the built x64 executable"
#endif
#if BuildOutput == ""
  #error "JHTB_OUTPUT must point to the build output folder"
#endif

[Setup]
AppId={{A69B2E7B-B1CC-4E8E-8A7A-3AFCA819A781}
AppName=嘉豪工具箱
AppVersion={#AppVersion}
AppVerName=嘉豪工具箱 {#AppVersion}
AppPublisher=嘉豪工具箱
AppPublisherURL=https://github.com/stone2013/JiahaoToolbox
AppSupportURL=https://github.com/stone2013/JiahaoToolbox/issues
AppUpdatesURL=https://github.com/stone2013/JiahaoToolbox/releases
DefaultDirName={localappdata}\Programs\JiahaoToolbox
DefaultGroupName=嘉豪工具箱
UsePreviousAppDir=yes
UsePreviousGroup=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.10240
CloseApplications=yes
RestartApplications=no
SetupIconFile=..\resources\app.ico
UninstallDisplayIcon={app}\JiahaoToolbox_v{#AppVersion}.exe
LicenseFile=..\LICENSE
OutputDir={#BuildOutput}
OutputBaseFilename=JiahaoToolbox_v{#AppVersion}_Setup_x64
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
WizardSizePercent=100
Uninstallable=yes
UninstallDisplayName=嘉豪工具箱

[Languages]
Name: "chinesesimp"; MessagesFile: "ChineseSimplified.isl"

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "附加快捷方式："; Flags: unchecked

[Files]
Source: "{#AppExe}"; DestDir: "{app}"; DestName: "JiahaoToolbox_v{#AppVersion}.exe"; Flags: ignoreversion

[Icons]
Name: "{group}\嘉豪工具箱"; Filename: "{app}\JiahaoToolbox_v{#AppVersion}.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\嘉豪工具箱"; Filename: "{app}\JiahaoToolbox_v{#AppVersion}.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\JiahaoToolbox_v{#AppVersion}.exe"; Description: "启动嘉豪工具箱"; Flags: postinstall nowait skipifsilent; Check: IsWebView2RuntimeInstalled

[Code]
const
  WebView2ClientId = '{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  WebView2RuntimeUrl = 'https://developer.microsoft.com/microsoft-edge/webview2/';
var
  RemoveUserData: Boolean;

function IsValidRuntimeVersion(const Value: string): Boolean;
begin
  Result := (Value <> '') and (Value <> '0.0.0.0');
end;

function IsWebView2RuntimeInstalled: Boolean;
var
  Version: string;
begin
  Result := False;
  if RegQueryStringValue(HKLM64, 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\' + WebView2ClientId, 'pv', Version) and IsValidRuntimeVersion(Version) then Result := True;
  if RegQueryStringValue(HKLM32, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\' + WebView2ClientId, 'pv', Version) and IsValidRuntimeVersion(Version) then Result := True;
  if RegQueryStringValue(HKCU, 'Software\Microsoft\EdgeUpdate\Clients\' + WebView2ClientId, 'pv', Version) and IsValidRuntimeVersion(Version) then Result := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ErrorCode: Integer;
begin
  if (CurStep = ssPostInstall) and not IsWebView2RuntimeInstalled then
    if MsgBox('尚未检测到 Microsoft Edge WebView2 Runtime。嘉豪工具箱需要此运行组件才能显示界面。是否打开微软官方页面由你手动安装？安装程序不会下载或安装该组件。', mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES then
      ShellExec('open', WebView2RuntimeUrl, '', '', SW_SHOWNORMAL, ewNoWait, ErrorCode);
end;

function InitializeUninstall: Boolean;
begin
  Result := True;
  RemoveUserData := MsgBox('是否同时删除嘉豪工具箱的个人设置、日志和测速历史？选择“否”会保留这些数据。', mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if (CurUninstallStep = usPostUninstall) and RemoveUserData then begin
    DelTree(ExpandConstant('{app}\JiahaoToolbox_Data'), True, True, True);
    DelTree(ExpandConstant('{userappdata}\JiahaoToolbox'), True, True, True);
  end;
end;
