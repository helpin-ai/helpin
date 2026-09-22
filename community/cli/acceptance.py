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
import shutil
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
    image = subprocess.check_output(['docker', 'image', 'inspect', 'alpine:3.20', '--format', '{{index .RepoDigests 0}}'], text=True).strip()
    with tempfile.TemporaryDirectory(prefix='helpin-cli-acceptance-') as temp:
        root = Path(temp).resolve()
        # This fixture runs only a tiny Go HTTP server in Alpine, not Community.
        # Stub RAM and disk capacity for this fixture, not real installation checks.
        # Production resource rejection remains covered by CLI unit tests.
        docker = shutil.which('docker')
        assert docker, 'Docker is required'
        df = shutil.which('df')
        assert df, 'df is required'
        shim_dir = root / 'fixture-bin'
        shim_dir.mkdir()
        shim = shim_dir / 'docker'
        shim.write_text('#!/usr/bin/env python3\n'
                        'import os, sys\n'
                        'if sys.argv[1:] == ["info", "--format", "{{.MemTotal}}"]:\n'
                        '    print(8 * 1024 * 1024 * 1024)\n'
                        'else:\n'
                        f'    os.execv({docker!r}, [{docker!r}, *sys.argv[1:]])\n')
        shim.chmod(0o755)
        disk_shim = shim_dir / 'df'
        disk_shim.write_text('#!/usr/bin/env python3\n'
                             'import os, sys\n'
                             f'if sys.argv[1:] == ["-Pk", {str(root)!r}]:\n'
                             '    print("Filesystem 1024-blocks Used Available Capacity Mounted on")\n'
                             '    print("fixture 41943040 0 41943040 0% /")\n'
                             'else:\n'
                             f'    os.execv({df!r}, [{df!r}, *sys.argv[1:]])\n')
        disk_shim.chmod(0o755)
        os.environ['PATH'] = str(shim_dir) + os.pathsep + os.environ['PATH']
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
    entrypoint: [sh, -c, 'if [ "$$FAIL_MIGRATION" = true ]; then echo migrated-before-failure > /data/persistent; exit 1; fi; echo fixture migration completed']
    environment:
      FAIL_MIGRATION: 'false'
    volumes: ['fixture_data:/data']
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
        def bundle_release(tag, upgrade_from=()):
            (source / 'release-evidence/release.json').write_text(json.dumps({
                'tag': tag, 'upgrade_from': list(upgrade_from),
                'upgrade_evidence': 'https://example.test/isolated-fixture-only' if upgrade_from else ''}))
            sums = [f'{hashlib.sha256(file.read_bytes()).hexdigest()}  {file.relative_to(source)}'
                    for file in sorted(source.rglob('*')) if file.is_file() and file.name != 'SHA256SUMS']
            (source / 'SHA256SUMS').write_text('\n'.join(sums) + '\n')
            archive = root / f'helpin-{tag}.tar.gz'
            with tarfile.open(archive, 'w:gz') as bundle:
                bundle.add(source, arcname='helpin-community')
            checksum = archive.with_name(archive.name + '.sha256')
            checksum.write_text(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n')
            return archive, checksum
        archive, checksum = bundle_release('community-v0.1.0-test')
        install = root / 'installation with spaces'
        def cli(command, *args):
            return run(str(binary), command, '--dir', str(install), *args)
        def compose(*args):
            return ['docker', 'compose', '--env-file', str(install / 'community/.env'), '-f', str(install / 'community/compose.yaml'), *args]
        restored = root / 'restored'
        recovered = root / 'recovered'
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
            run(*compose('exec', '-T', 'helpin-api', 'sh', '-c',
                         'mkdir /data/private; echo owned-data > /data/private/file; chmod 700 /data/private; chown -R 1000:1000 /data/private; ln -s private/file /data/relative-link'))
            snapshot = root / 'snapshot'
            cli('backup', '--yes', '--backup', str(snapshot))
            assert (snapshot / 'backup.json').is_file()
            cli('doctor')
            cli('stop')
            run(str(binary), 'restore', '--dir', str(restored), '--backup', str(snapshot), '--yes')
            def restored_compose(directory, *args):
                return ['docker', 'compose', '--env-file', str(directory / 'community/.env'), '-f', str(directory / 'community/compose.yaml'), *args]
            value = subprocess.check_output(restored_compose(restored, 'exec', '-T', 'helpin-api', 'cat', '/data/persistent'), text=True)
            assert value.strip() == 'operator-data', 'backup/restore lost data'
            ownership = subprocess.check_output(restored_compose(restored, 'exec', '-T', 'helpin-api', 'stat', '-c', '%u:%g:%a', '/data/private'), text=True)
            assert ownership.strip() == '1000:1000:700', 'restore lost volume ownership or permissions'
            linked = subprocess.check_output(restored_compose(restored, 'exec', '-T', 'helpin-api', 'cat', '/data/relative-link'), text=True)
            assert linked.strip() == 'owned-data', 'restore lost contained symlink'
            def values(directory):
                return dict(line.split('=', 1) for line in (directory / 'community/.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
            source_values, restored_values = values(install), values(restored)
            assert source_values.pop('COMPOSE_PROJECT_NAME') != restored_values.pop('COMPOSE_PROJECT_NAME')
            assert source_values == restored_values, 'restored secrets or settings changed'
            run(str(binary), 'stop', '--dir', str(restored))
            cli('start')
            with (community / '.env.example').open('a') as file:
                file.write('\nNEW_RELEASE_SETTING=retained-default\n')
            target, target_sum = bundle_release('community-v0.2.0-test', ['community-v0.1.0-test'])
            cli('upgrade', '--yes', '--bundle', str(target), '--checksum', str(target_sum), '--backup', str(root / 'upgrade-snapshot'))
            assert values(install)['JWT_SECRET'] == source_values['JWT_SECRET'], 'upgrade rotated a key'
            assert values(install)['NEW_RELEASE_SETTING'] == 'retained-default'
            value = subprocess.check_output(compose('exec', '-T', 'helpin-api', 'cat', '/data/persistent'), text=True)
            assert value.strip() == 'operator-data', 'upgrade lost data'
            incompatible, incompatible_sum = bundle_release('community-v0.3.0-test')
            result = subprocess.run([str(binary), 'upgrade', '--dir', str(install), '--yes', '--bundle', str(incompatible), '--checksum', str(incompatible_sum)], capture_output=True, text=True)
            assert result.returncode != 0 and 'does not declare a tested upgrade' in result.stderr, result.stderr
            cli('doctor')
            (community / 'compose.yaml').write_text((community / 'compose.yaml').read_text().replace("FAIL_MIGRATION: 'false'", "FAIL_MIGRATION: 'true'"))
            failing, failing_sum = bundle_release('community-v0.3.0-test', ['community-v0.2.0-test'])
            recovery_backup = root / 'recovery-backup'
            result = subprocess.run([str(binary), 'upgrade', '--dir', str(install), '--yes', '--bundle', str(failing), '--checksum', str(failing_sum), '--backup', str(recovery_backup)], capture_output=True, text=True)
            assert result.returncode != 0 and 'recovery backup' in result.stderr, result.stderr
            assert not subprocess.check_output(compose('ps', '--status', 'running', '-q'), text=True).strip(), 'failed upgrade left writers running'
            run(str(binary), 'restore', '--dir', str(recovered), '--backup', str(recovery_backup), '--yes')
            value = subprocess.check_output(restored_compose(recovered, 'exec', '-T', 'helpin-api', 'cat', '/data/persistent'), text=True)
            assert value.strip() == 'operator-data', 'recovery retained the failed migration mutation'
            assert values(recovered)['JWT_SECRET'] == source_values['JWT_SECRET'], 'recovery lost encryption keys'
            assert json.loads((recovered / 'release-evidence/release.json').read_text())['tag'] == 'community-v0.2.0-test'
            print('CLI Compose acceptance passed: lifecycle, backup/restore, upgrade, incompatible release refusal, failed-migration recovery, persistent data and keys.')
        except Exception:
            if (install / 'community/.env').exists():
                subprocess.run(compose('logs', '--tail=30'), check=False)
            raise
        finally:
            for directory in (install, restored, recovered):
                if (directory / 'community/.env').exists():
                    # All directories/projects are unique to this invocation.
                    run('docker', 'compose', '--env-file', str(directory / 'community/.env'),
                        '-f', str(directory / 'community/compose.yaml'), 'down', '-v', '--remove-orphans')


if __name__ == '__main__':
    main()
