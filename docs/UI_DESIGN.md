# UI Design & Architecture

## Design Philosophy

The Uptime Monitor UI follows a **minimalist design aesthetic** with a limited color palette, clear typography, and consistent whitespace.

### Color Palette (3 colors + 1 semantic)

- **Charcoal** (`#1a1a1a`) — primary text
- **Stone** (`#6b7280`) — secondary text, borders, muted elements
- **Teal** (`#0d9488`) — accent, links, primary actions, success states
- **Error** (`#b91c1c`) — failure states and destructive actions (used sparingly)

### Typography & Layout

- Inter for the main font
- Generous whitespace and simple grid layout
- Subtle borders and shadows instead of heavy styling

---

## Technical Architecture

### Go Templ Component Structure

```
interface/http/templates/
├── layout.templ       # Base HTML shell, nav, meta
├── components.templ   # Reusable: StatCard, StatusBadge, StatusDot, EmptyState, SeverityBadge
├── dashboard.templ    # Dashboard page + DashboardData types
├── monitor_list.templ # Monitor list page
├── monitor_detail.templ
├── monitor_form.templ
└── alert_history.templ
```

- **Template inheritance**: Pages use `@Layout("Title") { ... }` for a shared shell.
- **Composition**: Shared UI (stats, status badges, empty states) is factored into `components.templ`.
- **Types**: Structs (`DashboardData`, `MonitorListData`, etc.) live with their templates.

### CSS Structure

- **static/css/app.css** — single stylesheet
- BEM-like naming: `.card`, `.card__header`, `.stat__value`
- CSS custom properties for colors and spacing
- Responsive breakpoints at 640px and 768px

### JavaScript (Minimal)

**WebSocket** (`static/js/app.js`) — needed because Go templ cannot manage live WebSocket updates. It:

- Connects to `/ws` and reconnects with backoff
- Updates status indicator in the nav
- Handles health-check and alert messages
- Renders alert notifications

**Delete monitor** — small inline script in `monitor_detail.templ` for the delete confirmation and `fetch`.

No JavaScript is used for:

- Layout, styling, or navigation
- Form submission
- Data fetching (server-rendered)

### Static Asset Serving

- `/static/*` is served from `./static/`
- CSS: `/static/css/app.css`
- JS: `/static/js/app.js`

---

## Accessibility

- Semantic HTML (`<nav>`, `<main>`, `<article>`, `<time>`)
- `aria-live` on the connection status
- `aria-current` for the current page in breadcrumbs
- `.sr-only` for screen-reader-only text
- Sufficient color contrast and focus styles on form elements
