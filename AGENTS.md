# Project instructions

## Documentation

Before creating or renaming project documentation, follow the documentation naming
rules in [Quy ước coding](<docs/Lê Phước Thắng - Address Intelligence Platform - Quy ước coding.md>).

- Project design, architecture, domain, foundation, planning and worklog documents
  belong in `docs/` and must be named
  `Lê Phước Thắng - Address Intelligence Platform - <Tên tài liệu>.md`.
- Preserve the exact prefix, Vietnamese accents and ` - ` separators. Check for an
  existing document on the topic before creating another one.
- Keep standard filenames such as `README.md`, `AGENTS.md`, `GEMINI.md` and `SKILL.md`
  where tools or directory conventions require them; data/config/code are exempt.
- Update relative links and references when renaming; verify local targets exist.
  Do not use machine-specific absolute paths or `file://` links.
- Record significant completed work in the project backlog, newest first, using
  `DD/MM/YYYY HH:mm` and an accurate completion scope.

## Commits

Use `type(scope): description` with a concise English description. Each commit
must contain at most 999 added plus deleted lines; split larger changes by purpose.
