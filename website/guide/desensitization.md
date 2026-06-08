---
title: Desensitization
description: Privacy protection through automatic PII detection and masking
---

# Desensitization

## Overview

WebRouter's desensitization engine automatically detects and masks personally identifiable information (PII) in API requests before they reach upstream providers. The original data is restored in the response, so the upstream never sees sensitive information.

## Built-in Rules

| Rule | Category | Detection Method |
|------|----------|------------------|
| Phone Numbers | `PHONE` | Regex: Chinese mobile numbers |
| Names | `NAME` | Regex: Chinese name patterns |
| Addresses | `ADDRESS` | Regex: Address keywords |
| ID Cards | `IDCARD` | Pattern matching |
| Emails | `EMAIL` | Regex |

## How It Works

1. **Before forwarding** — wr-proxy scans the request body for PII patterns
2. **Masking** — matched content is replaced with placeholders (`[PHONE_1]`, `[NAME_2]`, etc.)
3. **Forward** — the masked request is sent to the upstream provider
4. **Response** — the provider responds normally
5. **Restore** — wr-proxy restores the original values in the response before sending to the client

## Custom Rules

You can add custom desensitization rules:

| Field | Description |
|-------|-------------|
| Name | Rule identifier |
| Type | `regex` (currently supported) |
| Pattern | Regular expression |
| Category | PHONE, NAME, ADDRESS, EMAIL, CUSTOM |
| Level | `standard` or `strict` |

## Configuration

Desensitization is enabled per-token. When creating or editing a token, toggle the **Desensitization** switch to enable PII stripping for that token's requests.