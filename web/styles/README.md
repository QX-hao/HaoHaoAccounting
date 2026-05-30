# Web Styles

Global CSS is split by responsibility so visual maintenance does not continue in one oversized file.

## Files

- `tokens.css`: color, shadow, and surface variables.
- `base.css`: reset, document, form, table, and loading primitives.
- `layout.css`: app shell, sidebar, content, page heading, and grid layout.
- `components.css`: reusable cards, buttons, lists, pills, charts, messages, and transaction controls.
- `pages/login.css`: login-specific page styles.
- `responsive.css`: shared responsive rules.

Keep new feature-specific styles close to their page section when possible, and move repeated visual patterns back into `components.css`.
