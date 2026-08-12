# launchd

Install the binary and system daemon:

```sh
sudo install -m 0755 perflock /usr/local/bin/perflock
sudo install -m 0644 init/launchd/com.github.aclements.perflock.plist \
  /Library/LaunchDaemons/com.github.aclements.perflock.plist
sudo launchctl bootstrap system \
  /Library/LaunchDaemons/com.github.aclements.perflock.plist
```

The daemon runs as root so it can create `/var/run/perflock.socket` and apply
`pmset` power-mode requests. Any local user may connect to the socket.

To uninstall it:

```sh
sudo launchctl bootout system/com.github.aclements.perflock
sudo rm /Library/LaunchDaemons/com.github.aclements.perflock.plist
```
