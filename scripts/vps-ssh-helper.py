"""VPS SSH helper — runs commands on a VPS via paramiko.

The root password that used to be hardcoded here was leaked (public repo,
2026-03-26 onward) and must be treated as compromised. It is NOT stored in
this file anymore: export VPS_SSH_PASSWORD in your shell instead, and rotate
the password on every machine it was used for (done for the staging VPS on
2026-06-06; the production VPS rotation is tracked separately).
"""
import os
import sys

import paramiko

HOST = os.getenv("VPS_HOST", "103.142.137.232")
USER = "root"
PASS = os.getenv("VPS_SSH_PASSWORD")
if not PASS:
    sys.exit("VPS_SSH_PASSWORD not set — the old hardcoded password was leaked and revoked.")


def run(cmd, timeout=300):
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASS, timeout=15)
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode()
    err = stderr.read().decode()
    code = stdout.channel.recv_exit_status()
    ssh.close()
    return out, err, code


def scp_upload(local_path, remote_path):
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASS, timeout=15)
    sftp = ssh.open_sftp()
    sftp.put(local_path, remote_path)
    sftp.close()
    ssh.close()


if __name__ == "__main__":
    cmd = " ".join(sys.argv[1:]) if len(sys.argv) > 1 else "echo hello"
    out, err, code = run(cmd)
    if out:
        print(out)
    if err:
        print(err, file=sys.stderr)
    sys.exit(code)
