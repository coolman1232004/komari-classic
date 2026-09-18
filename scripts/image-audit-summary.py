"""Print every finding; fail on HIGH/CRITICAL even if no fix is available."""
import json
from pathlib import Path
blocked = 0
for component in ['server', 'agent']:
    report = json.loads(Path(f'image-audit/{component}.json').read_text())
    findings = [(result.get('Target'), item) for result in report.get('Results', [])
                for item in result.get('Vulnerabilities', [])]
    print(f'{component}: {len(findings)} findings')
    for target, item in findings:
        blocked += item.get('Severity') in {'HIGH', 'CRITICAL'}
        print(json.dumps({key: item.get(key) for key in
                          ['VulnerabilityID', 'PkgName', 'InstalledVersion', 'FixedVersion', 'Severity', 'PrimaryURL']}, sort_keys=True))
if blocked:
    raise SystemExit(f'{blocked} high/critical image findings require review and remediation')
