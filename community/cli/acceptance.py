#!/usr/bin/env python3
"""Exercise the compiled CLI against an isolated real Compose fixture, not the full app.

Requires Go, Docker/Compose, and a locally available alpine:3.20 image.
Only this test's generated project and volume are removed.
"""
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def run(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)


def port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def main():
    image = subprocess.check_output(['docker', 'image', 'inspect', 'alpine:3.20', '--format', '{{.Id}}'], text=True).strip()
    with tempfile.TemporaryDirectory(prefix='helpin-cli-acceptance-') as temp:
        root = Path(temp)
        binary = root / 'helpin'
        run('go', 'build', '-o', str(binary), '.', cwd=ROOT / 'community/cli', env={**os.environ, 'GOWORK': 'off'})
        source = root / 'helpin-community'
        community = source / 'community'
        community.mkdir(parents=True)
        for name in ('setup.sh', '.env.example', 'apps.example.json'):
            (community / name).write_bytes((ROOT / 'community' / name).read_bytes())
        server = root / 'fixture-api.go'
        server.write_text('''package main
import("encoding/json";"net/http";"os")
func main(){
 if _,err:=os.Stat("/data/persistent");os.IsNotExist(err){os.WriteFile("/data/persistent",[]byte("retained"),0600)}
 http.HandleFunc("/api/auth/config",func(w http.ResponseWriter,r *http.Request){json.NewEncoder(w).Encode(map[string]string{"public_widget_url":os.Getenv("PUBLIC_WIDGET_URL")})})
 if err:=http.ListenAndServe(":8080",nil);err!=nil{panic(err)}
}
''')
        run('go','build','-o',str(community / 'fixture-api'),str(server),env={**os.environ,'CGO_ENABLED':'0','GOOS':'linux'})
        (community / 'compose.yaml').write_text(f'''services:
  helpin-migrate:
    image: {image}
    command: [sh, -c, 'echo fixture migration completed']
  helpin-api:
    image: {image}
    environment:
      PUBLIC_WIDGET_URL: ${{PUBLIC_WIDGET_URL}}
    command: [/fixture-api]
    ports: ['${{BIND_ADDRESS}}:${{DASHBOARD_PORT}}:8080']
    volumes: ['fixture_data:/data', './fixture-api:/fixture-api:ro']
    depends_on:
      helpin-migrate: {{condition: service_completed_successfully}}
    healthcheck:
      test: [CMD, wget, -q, -O, /dev/null, 'http://127.0.0.1:8080/api/auth/config']
      interval: 1s
      retries: 10
volumes:
  fixture_data:
''')
        (source / 'release-evidence').mkdir()
        (source / 'release-evidence/release.json').write_text(json.dumps({'tag': 'community-v0.1.0-test'}))
        archive = root / 'fixture.tar.gz'
        with tarfile.open(archive, 'w:gz') as bundle:
            bundle.add(source, arcname='helpin-community')
        checksum = root / 'fixture.tar.gz.sha256'
        checksum.write_text(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n')
        install = root / 'installation with spaces'
        def cli(command, *args):
            return run(str(binary), command, '--dir', str(install), *args)
        def compose(*args):
            return ['docker', 'compose', '--env-file', str(install / 'community/.env'), '-f', str(install / 'community/compose.yaml'), *args]
        try:
            cli('install', '--yes', '--mode', 'local', '--bundle', str(archive), '--checksum', str(checksum),
                '--port', str(port()), '--storage-port', str(port()), '--help-port', str(port()))
            original = (install / 'community/.env').read_bytes()
            cli('install', '--yes')
            assert original == (install / 'community/.env').read_bytes(), 'repeat install rotated secrets'
            cli('status')
            cli('doctor')
            run(*compose('exec', '-T', 'helpin-api', 'sh', '-c', 'echo operator-data > /data/persistent'))
            cli('configure', '--yes', '--mode', 'local', '--port', str(port()))
            cli('restart')
            cli('doctor')
            value = subprocess.check_output(compose('exec', '-T', 'helpin-api', 'cat', '/data/persistent'), text=True)
            assert value.strip() == 'operator-data', 'restart lost persistent data'
            cli('stop')
            result = subprocess.run([str(binary), 'doctor', '--dir', str(install)], capture_output=True, text=True)
            assert result.returncode != 0, 'doctor passed a stopped installation'
            cli('start')
            value = subprocess.check_output(compose('exec', '-T', 'helpin-api', 'cat', '/data/persistent'), text=True)
            assert value.strip() == 'operator-data', 'stop/start lost persistent data'
            print('CLI Compose acceptance passed: install, repeat install, configure, restart, doctor, stop/start, persistent data.')
        except Exception:
            if (install / 'community/.env').exists():
                subprocess.run(compose('logs', '--tail=30'), check=False)
            raise
        finally:
            if (install / 'community/.env').exists():
                # This randomly located CLI installation has a unique project name.
                run(*compose('down', '-v', '--remove-orphans'))


if __name__ == '__main__':
    main()
