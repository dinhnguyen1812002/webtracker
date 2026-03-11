# Dashboard Templates

This directory contains the templ templates for the Uptime Monitor dashboard.

## Templates

- `layout.templ` - Base layout with navigation, WebSocket connection, and TailwindCSS
- `dashboard.templ` - Main dashboard with monitor overview and statistics
- `monitor_detail.templ` - Detailed monitor view with metrics and recent activity
- `alert_history.templ` - Alert history page with filtering capabilities

## TailwindCSS Styling

The dashboard uses TailwindCSS for styling with the following features:

### Design System
- **Colors**: Custom color palette for status indicators (success, warning, error, info)
- **Typography**: Clean, readable fonts with proper hierarchy
- **Spacing**: Consistent spacing using Tailwind's spacing scale
- **Layout**: Responsive grid system that works on all screen sizes

### Components
- **Status Indicators**: Color-coded dots and badges for monitor status
- **Cards**: Clean card design for monitor items and statistics
- **Navigation**: Responsive navigation bar with connection status
- **Forms**: Styled form elements for filters and inputs
- **Notifications**: Toast-style notifications for real-time alerts

### Responsive Design
- **Mobile First**: Designed for mobile devices first, then enhanced for larger screens
- **Breakpoints**: Uses Tailwind's responsive breakpoints (sm, md, lg, xl)
- **Grid System**: Responsive grid layouts that adapt to screen size
- **Navigation**: Collapsible navigation on smaller screens

### Real-time Features
- **WebSocket Integration**: Real-time updates without page refresh
- **Connection Status**: Visual indicator of WebSocket connection status
- **Live Updates**: Monitor status and metrics update in real-time
- **Notifications**: Toast notifications for alerts and status changes

## Build Process

The templates are compiled using the `templ` tool:

```bash
# Generate Go code from templates
templ generate interface/http/templates/

# Build the application
go build -o bin/uptime-monitor ./cmd/main.go
```

Or use the build script:

```bash
./scripts/build-dashboard.sh
```

## Performance Optimizations

- **CDN Delivery**: TailwindCSS loaded from CDN for fast delivery
- **Minimal JavaScript**: Only essential JavaScript for WebSocket and interactions
- **Efficient Templates**: Compiled templates for fast server-side rendering
- **Caching**: Proper HTTP caching headers for static assets

## Browser Support

The dashboard supports all modern browsers:
- Chrome 60+
- Firefox 60+
- Safari 12+
- Edge 79+

## Accessibility

The dashboard follows web accessibility guidelines:
- **Semantic HTML**: Proper HTML structure and elements
- **ARIA Labels**: Screen reader support for interactive elements
- **Keyboard Navigation**: Full keyboard navigation support
- **Color Contrast**: Sufficient color contrast for readability
- **Focus Indicators**: Clear focus indicators for keyboard users