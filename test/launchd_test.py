#!/usr/bin/env python3

import plistlib
from pathlib import Path

plist_path = Path(__file__).parents[1] / "init" / "launchd" / "com.github.aclements.perflock.plist"
with plist_path.open("rb") as source:
    job = plistlib.load(source)

assert job["Label"] == "com.github.aclements.perflock"
assert job["ProgramArguments"] == ["/usr/local/bin/perflock", "-daemon"]
assert job["RunAtLoad"] is True
assert job["KeepAlive"] == {"SuccessfulExit": False}
assert job["ProcessType"] == "Background"
assert job["StandardOutPath"] == "/var/log/perflock.log"
assert job["StandardErrorPath"] == "/var/log/perflock.log"
print("PASS: launchd job semantics")
