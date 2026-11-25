# Vite+ Monorepo Starter

A starter for creating a Vite+ monorepo.

## Development

- Check everything is ready:

```bash
vp run ready
```

- Run the tests:

```bash
vp run -r test
```

- Run the browser tests (Playwright, from `apps/web`). They serve the last production build on
  port 4173, or reuse whatever already listens there, so build first and pick a free port when
  another app holds it:

```bash
vp build && E2E_PORT=4185 vp exec playwright test
```

- Build the monorepo:

```bash
vp run -r build
```

- Run the development server:

```bash
vp run dev
```
