# Security Policy

molla is intended to operate as a public URL-shortening service. Authentication, redirect correctness, abuse controls, and cloud configuration are security-sensitive.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability.

Use one of these private channels:

- [GitHub private vulnerability reporting](https://github.com/SumonMSelim/molla/security/advisories/new) (preferred)
- [sumonmselim@gmail.com](mailto:sumonmselim@gmail.com)

Include a description, reproduction steps, affected components, and potential impact. Remove API keys, credentials, personal data, and active malicious URLs from reports.

You can expect an acknowledgment within a few days. Please allow a reasonable remediation window before public disclosure.

## Supported versions

molla is under active development and has no supported release yet. Once releases begin, only the latest release will receive security fixes until a broader support policy is published.

## Priority areas

Reports are especially useful when they involve:

- authentication or authorization bypass;
- exposure of API keys, signing material, or user data;
- redirects that bypass deletion, expiry, or abuse takedown;
- injection, request smuggling, cache poisoning, or unsafe URL handling;
- denial-of-service or rate-limit bypass;
- AWS IAM, storage, encryption, network, or infrastructure misconfiguration;
- dependency or build-pipeline compromise.
