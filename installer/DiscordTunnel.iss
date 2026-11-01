; Instalador DiscordTunnel — inclui OpenVPN se necessário (uso gratuito RiseupVPN)

#define MyAppName      "DiscordTunnel"
#define MyAppVersion   "1.0.2"
#define MyAppPublisher "DiscordTunnel"
#define MyAppExeName   "DiscordTunnel.exe"
#define MyAppId        "{{B8E4C1A2-9F3D-4A2E-B1C0-DISCORDTUNNEL}"

#define DistDir        "..\dist\pack"
#define OpenVpnVersion "2.7.4"
#define OpenVpnMsi     "OpenVPN-" + OpenVpnVersion + "-I001-amd64.msi"

[Setup]
AppId={#MyAppId}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppSupportURL=https://riseup.net/vpn
AppUpdatesURL=https://riseup.net/vpn/donate
DefaultDirName={autopf}\DiscordTunnel
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=no
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=Output
OutputBaseFilename=DiscordTunnel-Setup-{#MyAppVersion}
Compression=lzma2/ultra
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#MyAppExeName}
CloseApplications=yes
LicenseFile=..\LICENSE.txt
InfoBeforeFile=INFO_BEFORE.txt

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#DistDir}\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\redirector.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\WinDivert.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\WinDivert64.sys"; DestDir: "{app}"; Flags: ignoreversion
Source: "deps\{#OpenVpnMsi}"; DestDir: "{tmp}"; Flags: deleteafterinstall; Check: not OpenVpnInstalled

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Comment: "Discord na RiseupVPN — resto do PC na operadora"
Name: "{group}\Desinstalar {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "msiexec.exe"; Parameters: "/i ""{tmp}\{#OpenVpnMsi}"" /quiet /norestart ADDLOCAL=OpenVPN,Drivers,Drivers.Wintun"; StatusMsg: "Instalando OpenVPN (Wintun)..."; Flags: waituntilterminated; Check: not OpenVpnInstalled
Filename: "{app}\{#MyAppExeName}"; Description: "Abrir {#MyAppName}"; Flags: shellexec postinstall skipifsilent nowait

[Code]
function OpenVpnInstalled: Boolean;
var
  Paths: TArrayOfString;
  i: Integer;
begin
  Paths := ['C:\Program Files\OpenVPN\bin\openvpn.exe',
            'C:\Program Files (x86)\OpenVPN\bin\openvpn.exe'];
  for i := 0 to GetArrayLength(Paths) - 1 do
    if FileExists(Paths[i]) then
    begin
      Result := True;
      Exit;
    end;
  Result := False;
end;

[UninstallDelete]
Type: filesandordirs; Name: "{localappdata}\DiscordTunnel\runtime"
