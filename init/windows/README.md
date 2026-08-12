# Windows service

Build and install from an elevated PowerShell:

```powershell
Go build -o perflock.exe ./cmd/perflock
.\init\windows\install.ps1
```

The installer copies the binary to `%ProgramFiles%\perflock\perflock.exe`,
registers the `perflock` Windows service, and starts it. The service listens on
`\\.\pipe\perflock`.

The named-pipe ACL grants full access to LocalSystem and administrators. Other
authenticated local users may read and write the pipe. Remote pipe clients are
rejected.

To remove the service from an elevated PowerShell:

```powershell
Stop-Service perflock
sc.exe delete perflock
Remove-Item "$env:ProgramFiles\perflock\perflock.exe"
```
