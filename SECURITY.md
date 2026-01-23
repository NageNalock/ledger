# Security

This project is intended for personal use and trusted deployments.

## Sensitive data

- Never commit `.env`, database files (`*.db`), logs, or uploads.
- Set `JWT_SECRET` to a strong random value (≥ 32 characters).

## AI features

AI-related features may send transaction metadata (merchant, amount, time, remark, categories) to a third-party provider.
Review your compliance requirements before enabling or using AI.

## Reporting security issues

If you discover a security issue, please do not open a public issue.
Instead, contact the maintainer privately.

