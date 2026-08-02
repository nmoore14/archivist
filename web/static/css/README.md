# Archivist styles

Stylesheets load in the order listed below. Keep that order when adding files because later files intentionally refine shared rules from earlier ones.

| File | Responsibility |
| --- | --- |
| `base.css` | Fonts, design tokens, global shell, common controls, cards, forms, and tables |
| `layouts.css` | Primary page layouts, authentication, and core responsive behavior |
| `content.css` | Markdown output, citations, mathematics, and the source drawer |
| `index.css` | Course index statistics, tables, and index settings |
| `workspace.css` | Course discovery, document uploads and progress, notes, and workspace utilities |
| `chat.css` | Course chat layout, sidebar, messages, composer, and keyboard command bar |
| `course-shell.css` | Shared left-sidebar layout for non-chat course pages |
| `search.css` | Global search page, results, and modal |
| `settings.css` | User settings and appearance menu |
| `themes.css` | Dark-theme tokens and component overrides |

Use the existing design tokens in `base.css` instead of introducing isolated colors. Place responsive rules with the component they modify, and put cross-application theme overrides in `themes.css`.

## Typography scale

The root font size in `base.css` controls the entire interface. Body copy starts at `0.75rem`, and every component font size is proportional to that baseline. Change the body baseline and scale the component `rem` values by the same ratio to preserve the existing hierarchy. Use `em` only when text should scale relative to its immediate parent, such as headings inside rendered Markdown.
