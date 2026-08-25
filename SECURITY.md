# Security Policy

## Supported versions

During pre-v0 development, only the current `main` branch is actively supported.

After versioned releases begin, supported versions will be documented here.

## Reporting a vulnerability

Do **not** publish credentials, exploit details, or sensitive vulnerability information in a public issue.

Preferred path:

1. Use GitHub's private vulnerability reporting / Security Advisory flow if it is enabled for this repository.
2. If private reporting is not available, open a minimal public issue stating that you need a private security contact. Do not include exploit details or secrets.

## Credential safety

Agent Council is designed to operate with subscription-authenticated CLI tools. Reports involving unexpected use of metered API credentials, credential leakage, environment inheritance, or auth-mode confusion are treated as security-sensitive.
