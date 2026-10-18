# Translations

User-facing strings must be translatable.

- Frontend: strings live in `web/src/lib/i18n.ts`. Add the key to `en` and use `t('yourKey')`.
- API (mostly notifications): source strings in `pkg/i18n/lang/en.json`. Use `i18n.T(lang, "your.key", params...)`.
- Edit only `en.json`. Never add a language or translate strings yourself; translations are managed at https://vikunja.io/docs/translations/. If asked to, point the user there.
- Before adding a new string, check if a string with the same value already exists. If it does, use the existing key.
