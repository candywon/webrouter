# Contributing to WebRouter

Thank you for your interest in contributing to WebRouter! This document outlines the process and requirements for contributing.

---

## Contributor License Agreement (CLA)

**All contributors must sign the CLA before their first PR is merged.**

### Why a CLA?

WebRouter uses a dual-licensing model (BSL 1.1 → Apache-2.0 for CE; proprietary for EE). The CLA grants the project maintainers the right to re-license contributions under different terms in the future. This is essential for:

- Switching the CE license after the BSL Change Date
- Including community contributions in the Enterprise Edition
- Protecting the project's long-term legal flexibility

Without a CLA, even a single unsigned contribution can block a license change (this is why HashiCorp required CLA before switching to BSL).

### What you grant

By signing the CLA, you grant the project maintainer (Jianlin Huang):

1. **Copyright license** — perpetual, worldwide, royalty-free right to use, modify, and distribute your contribution
2. **Patent license** — perpetual, worldwide, royalty-free right to practice any patent claims your contribution may involve
3. **Sublicense right** — the right to re-license your contribution under different terms (e.g., Apache-2.0, commercial EULA)

### How to sign

1. Fork the repository
2. Create your feature branch
3. Add your name to the `.claude/cla-signees.md` file in your first PR:
   ```
   - Your Name <your@email.com> — date of signing
   ```
4. By adding your name, you confirm that you have read and agree to the CLA terms below

### CLA Full Text

> **Contributor License Agreement**
>
> By contributing to WebRouter, I agree to the following terms:
>
> 1. I grant Jianlin Huang a perpetual, worldwide, non-exclusive, royalty-free,
>    irrevocable license to reproduce, modify, distribute, sublicense, and
>    otherwise use my Contributions in connection with the WebRouter project,
>    including the right to re-license under any terms Jianlin Huang chooses.
>
> 2. I represent that I am legally entitled to grant the above license.
>    If my employer(s) has rights to intellectual property that I create,
>    I have received permission to make Contributions on behalf of that
>    employer, or my employer has waived such rights.
>
> 3. I represent that each of my Contributions is my original creation
>    (except as noted below). For any Contribution that is not my original
>    creation, I will clearly mark it and identify its source.
>
> 4. I am not aware of any pending or threatened claims, suits, or actions
>    that would affect my ability to grant the rights above.
>
> 5. I understand that my Contributions may be included in both the
>    Community Edition (BSL 1.1 / Apache-2.0) and the Enterprise Edition
>    (proprietary EULA) of WebRouter.

---

## Development Setup

### Prerequisites

- Python 3.8+
- Go 1.21+
- Git

### Setup

```bash
# Clone your fork
git clone https://github.com/<your-username>/webrouter.git
cd webrouter

# Run the install script
bash deploy/install.sh

# Or manually:
pip install -r backend/requirements.txt
cd wr-proxy && make build && cd ..
```

### Running

```bash
# Start all services
python3 backend/start.py start

# Or run Flask in debug mode
FLASK_ENV=development python3 backend/app.py
```

---

## Code Style

### Python (backend)

- Follow PEP 8
- Use 4-space indentation
- Keep functions focused and concise
- Add type hints for new public APIs

### Go (wr-proxy)

- Follow standard Go formatting (`gofmt`)
- Run `go vet` before submitting
- Use `golint` for style suggestions

### JavaScript (frontend SPA)

- Use ES6+ syntax
- Follow the existing class-based page module pattern
- All user-facing strings must use `I18n.t('key', {param: value})` — no hardcoded UI text
- Keep i18n keys in sync between `static/i18n/en.json` and `static/i18n/zh-CN.json`

---

## Pull Request Process

1. **Fork** the repository and create a feature branch from `main`
2. **Make your changes** following the code style guidelines above
3. **Test locally** — ensure the app starts and your changes work
4. **Sign the CLA** — add your name to `.claude/cla-signees.md` if this is your first PR
5. **Submit a PR** with a clear description of what you changed and why
6. **Respond to review feedback** promptly

### PR Title Format

```
type(scope): description

# Examples:
feat(providers): add support for Azure OpenAI provider type
fix(monitor): fix cooldown timer not clearing on reset
i18n(tokens): add missing translation keys for cost alerts
docs: update Quick Start section
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `i18n` | Internationalization |
| `refactor` | Code refactoring |
| `docs` | Documentation |
| `test` | Tests |
| `chore` | Build, CI, maintenance |

---

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include steps to reproduce, expected behavior, and actual behavior
- Mention your OS, Python version, and Go version

---

## Code of Conduct

Be respectful. Disagreement is fine; personal attacks are not. We're all here to make WebRouter better.

---

## Questions?

Open a GitHub Issue or start a Discussion. We're happy to help.
