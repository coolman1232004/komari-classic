"""Print all scanner findings for review, including unfixed vulnerabilities."""
import json
from pathlib import Path
for component in ['server', 'agent']:
    report = json.loads(Path(f'image-audit/{component}.json').read_text())
    findings = [(result.get('Target'), item) for result in report.get('Results', [])
                for item in result.get('Vulnerabilities', [])]
    print(f'{component}: {len(findings)} findings')
    for target, item in findings:
        print(json.dumps({key: item.get(key) for key in
                          ['VulnerabilityID', 'PkgName', 'InstalledVersion', 'FixedVersion', 'Severity', 'PrimaryURL']}, sort_keys=True))
